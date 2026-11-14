package Plugins

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/killmonday/fscanx/PocScan/lib"
	"github.com/killmonday/fscanx/common"
)

// ---------- 测试基础设施 ----------

// setTestDirPaths 覆盖字典：先消费 sync.Once，再直接改包级切片，
// 使后续 loadDirPaths 调用不再触发重载。
func setTestDirPaths(paths []string) {
	loadDirPaths()
	dirPaths = paths
	dirPathsErr = nil
}

// resetDirSchedForTest 重置全局调度器（done 通道 close 后不可复用，须整体重建）
// 与惰性 client（其 Timeout 按创建时点的 WebTimeout 固化，测试改超时后须重建）。
// 调用前提：旧调度器已完成排空（done 已关闭，其 worker 已退出）。
func resetDirSchedForTest() {
	dirSched = &dirScheduler{
		reg:    make(map[string]struct{}),
		taskCh: make(chan dirTask, dirWorkerNum),
		wake:   make(chan struct{}, 1),
		done:   make(chan struct{}),
	}
	dirClientOnce = sync.Once{}
}

func dirTestSetup(t *testing.T, paths []string) {
	t.Helper()
	common.DirScanEnable = true
	common.WebTimeout = 5 // 测试进程未跑 flag，零值会使 client 无超时
	t.Cleanup(func() {
		common.DirScanEnable = false
		common.WebTimeout = 0
	})
	setTestDirPaths(paths)
	resetDirSchedForTest()
}

// registerForTest 测试专用注册：启动调度器但不启动真 worker——
// 调度器行为测试用 mock 消费者，若经 RegisterSite 会同时冒出 16 个真 worker
// 对虚构域名发起 DNS 解析（无超时）抢走任务，导致排空死等。
func registerForTest(url string, seeds map[string]struct{}) {
	dirSched.mu.Lock()
	defer dirSched.mu.Unlock()
	if _, ok := dirSched.reg[url]; ok {
		return
	}
	dirSched.reg[url] = struct{}{}
	dirSched.sites = append(dirSched.sites, &dirSite{url: url, seeds: seeds, seen: map[string]struct{}{}})
	if !dirSched.started {
		dirSched.started = true
		go dirSched.run()
	}
	dirSched.wakeup()
}

// fingerPage 返回可被指纹引擎识别的响应（python SimpleHTTP 目录列表风格，
// MEM-003 输出一致性测试实证其可识别）。title 可定制用于差集场景。
func fingerPage(title string) (int, string) {
	body := fmt.Sprintf(`<!DOCTYPE HTML><html><head><meta http-equiv="Content-Type" content="text/html; charset=utf-8"><title>%s</title></head><body><hr><a href="../">../</a><hr></body></html>`, title)
	return 200, body
}

// probePresent 用空 seeds 的探针站点跑一次请求，取该 URL 响应的实际呈现 key
// （自适应：不依赖指纹库具体产品名，对指纹库演进稳健）。
func probePresent(t *testing.T, client *http.Client, baseURL, path string) string {
	t.Helper()
	s := &dirSite{url: baseURL, seeds: map[string]struct{}{}, seen: map[string]struct{}{}}
	runDirTask(client, dirTask{site: s, path: path})
	if len(s.hits) != 1 {
		t.Fatalf("probe %s: want 1 hit, got %d (页面应可识别指纹)", path, len(s.hits))
	}
	return s.hits[0].presentKey
}

// ---------- presentKey ----------

func TestPresentKeySorted(t *testing.T) {
	if presentKey([]string{"b", "a"}, "t") != presentKey([]string{"a", "b"}, "t") {
		t.Fatal("指纹顺序不应影响呈现 key")
	}
	if presentKey([]string{"a"}, "t1") == presentKey([]string{"a"}, "t2") {
		t.Fatal("不同标题应为不同呈现")
	}
	if presentKey([]string{"a"}, "t") == presentKey([]string{"a", "b"}, "t") {
		t.Fatal("不同指纹集应为不同呈现")
	}
}

// ---------- 调度器：交错与单站点在途≤1 ----------

func TestSchedulerSingleInflightAndInterleave(t *testing.T) {
	dirTestSetup(t, []string{"/p1", "/p2", "/p3"})

	registerForTest("http://site-a.test", nil)
	registerForTest("http://site-b.test", nil)

	var mu sync.Mutex
	inflight := map[string]int{}
	maxInflight := map[string]int{}
	var order []string

	consumerDone := make(chan struct{})
	go func() {
		defer close(consumerDone)
		for task := range dirSched.taskCh {
			site := task.site.url
			mu.Lock()
			inflight[site]++
			if inflight[site] > maxInflight[site] {
				maxInflight[site] = inflight[site]
			}
			order = append(order, site)
			mu.Unlock()
			time.Sleep(2 * time.Millisecond) // 模拟网络耗时，制造多站点在途窗口
			mu.Lock()
			inflight[site]--
			mu.Unlock()
			dirSched.complete(task.site)
		}
	}()

	SealAndDrain()
	<-consumerDone

	for site, m := range maxInflight {
		if m > 1 {
			t.Fatalf("site %s 在途达到 %d，违反单站点在途≤1", site, m)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if len(order) != 6 {
		t.Fatalf("应发射 2 站点 × 3 路径 = 6 任务，got %d", len(order))
	}
	if order[0] == order[1] {
		t.Fatalf("前两个任务落在同一站点 %v，未交错", order[:2])
	}
}

// ---------- 调度器：全忙时阻塞等待（不退出） ----------

func TestSchedulerWaitsWhenAllBusy(t *testing.T) {
	dirTestSetup(t, []string{"/p1", "/p2"})

	registerForTest("http://busy.test", nil)

	block := make(chan struct{})
	consumerDone := make(chan struct{})
	go func() {
		defer close(consumerDone)
		for task := range dirSched.taskCh {
			<-block // 阻塞第一个任务，站点持续在途
			dirSched.complete(task.site)
		}
	}()

	time.Sleep(50 * time.Millisecond)
	select {
	case <-dirSched.done:
		close(block)
		t.Fatal("调度器在未 Seal 时退出了（应阻塞等待而非因暂时无活退出）")
	default:
	}
	close(block)
	SealAndDrain()
	<-consumerDone
}

// ---------- 停机协议：Seal 后拒绝注册 ----------

func TestSealRejectsNewSites(t *testing.T) {
	dirTestSetup(t, []string{"/p1"})
	dirSched.mu.Lock()
	dirSched.sealed = true // 手动置位，等价 SealAndDrain 的 Seal 阶段（避免真任务排空依赖网络）
	dirSched.mu.Unlock()

	RegisterSite("http://late.test", nil) // sealed → 应被拒绝（含"不启动子系统"）
	dirSched.mu.Lock()
	defer dirSched.mu.Unlock()
	if len(dirSched.sites) != 0 {
		t.Fatalf("Seal 后注册应被拒绝，got %d sites", len(dirSched.sites))
	}
	if dirSched.started {
		t.Fatal("Seal 后注册不应启动 worker/调度器")
	}
}

// ---------- 停机协议：排空中途 Seal 不丢任务 ----------

func TestSealDuringInflightKeepsAllTasks(t *testing.T) {
	dirTestSetup(t, []string{"/p1", "/p2", "/p3", "/p4"})
	registerForTest("http://mid.test", nil)

	var processed int
	var mu sync.Mutex
	consumerDone := make(chan struct{})
	sealDone := make(chan struct{})
	go func() { // Seal 异步触发：同步在消费者内调用会等自己 complete 而死锁
		defer close(sealDone)
		SealAndDrain()
	}()
	go func() {
		defer close(consumerDone)
		for task := range dirSched.taskCh {
			mu.Lock()
			processed++
			doSeal := processed == 2
			mu.Unlock()
			if doSeal {
				dirSched.mu.Lock() // 读 sealed 须持调度器锁（与 SealAndDrain 的写互斥，race 安全）
				sealed := dirSched.sealed
				dirSched.mu.Unlock()
				if !sealed {
					t.Error("Seal 应已置位")
				}
			}
			dirSched.complete(task.site)
		}
	}()
	<-sealDone
	<-consumerDone

	mu.Lock()
	defer mu.Unlock()
	if processed != 4 {
		t.Fatalf("Seal 与排空并发时应处理全部 4 任务，got %d", processed)
	}
}

// ---------- worker panic-safe：在途计数正确递减、可排空 ----------

func TestWorkerPanicStillDrains(t *testing.T) {
	dirTestSetup(t, []string{"/ok1", "/boom", "/ok2"})
	registerForTest("http://panic.test", nil)

	consumerDone := make(chan struct{})
	go func() {
		defer close(consumerDone)
		for task := range dirSched.taskCh {
			func() {
				defer func() {
					if r := recover(); r != nil {
						// 注入的 panic：模拟 dirWorker 的 recover 行为
					}
				}()
				defer dirSched.complete(task.site) // panic 也必须递减在途（panic-safe）
				if task.path == "/boom" {
					panic("injected worker panic")
				}
			}()
		}
	}()
	SealAndDrain()
	<-consumerDone

	dirSched.mu.Lock()
	defer dirSched.mu.Unlock()
	for _, s := range dirSched.sites {
		if !s.drained() {
			t.Fatalf("panic 后站点未排空：inflight=%d idx=%d", s.inflight, s.idx)
		}
	}
}

// ---------- 判定矩阵（runDirTask 直测） ----------

func TestRunDirTaskVerdicts(t *testing.T) {
	mux := http.NewServeMux()
	// 有指纹可识别页（目录列表风格）；标题各异构造差集场景
	mux.HandleFunc("/landing", func(w http.ResponseWriter, r *http.Request) {
		code, body := fingerPage("Landing")
		w.Header().Set("Server", "SimpleHTTP/0.6 Python/3.11.0")
		w.WriteHeader(code)
		fmt.Fprint(w, body)
	})
	mux.HandleFunc("/sub/login", func(w http.ResponseWriter, r *http.Request) {
		code, body := fingerPage("SubLogin")
		w.Header().Set("Server", "SimpleHTTP/0.6 Python/3.11.0")
		w.WriteHeader(code)
		fmt.Fprint(w, body)
	})
	mux.HandleFunc("/hit", func(w http.ResponseWriter, r *http.Request) {
		code, body := fingerPage("Hit")
		w.Header().Set("Server", "SimpleHTTP/0.6 Python/3.11.0")
		w.WriteHeader(code)
		fmt.Fprint(w, body)
	})
	mux.HandleFunc("/plain", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "just plain text, no fingerprint here")
	})
	mux.HandleFunc("/gone", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	// 403 屏蔽（真机噪声反馈）：响应头带可识别指纹也应零输出
	mux.HandleFunc("/denied", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "SimpleHTTP/0.6 Python/3.11.0")
		http.Error(w, "Forbidden", http.StatusForbidden)
	})
	// 401 同属 4xx 全屏蔽（用户决策：仅极少数产品可识别，价值低）
	mux.HandleFunc("/auth401", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "SimpleHTTP/0.6 Python/3.11.0")
		w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	})
	// 同前缀跳转：/sub → /sub/login（应跟随并识别）
	mux.HandleFunc("/sub", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/sub/login", http.StatusFound)
	})
	// 跨前缀跳转：/rand → /landing（软 404 形态，不应跟随）。
	// Server 头故意可被指纹引擎识别：复刻真机误报条件——302 响应本身的头
	// （Location/Server）被匹配出指纹（BL-DIR-001 真机 thinkphp 误报），断言仍零输出。
	mux.HandleFunc("/rand", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "SimpleHTTP/0.6 Python/3.11.0")
		http.Redirect(w, r, "/landing", http.StatusFound)
	})
	// 跨 host：/ext → 第二个 server（同样带可识别指纹的 302 响应头）
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		code, body := fingerPage("OtherHost")
		w.Header().Set("Server", "SimpleHTTP/0.6 Python/3.11.0")
		w.WriteHeader(code)
		fmt.Fprint(w, body)
	}))
	defer other.Close()
	mux.HandleFunc("/ext", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "SimpleHTTP/0.6 Python/3.11.0")
		http.Redirect(w, r, other.URL+"/x", http.StatusFound)
	})
	// 同指纹不同标题（同产品后台救回场景）
	mux.HandleFunc("/wp-admin", func(w http.ResponseWriter, r *http.Request) {
		code, body := fingerPage("AdminLogin")
		w.Header().Set("Server", "SimpleHTTP/0.6 Python/3.11.0")
		w.WriteHeader(code)
		fmt.Fprint(w, body)
	})
	// 空标题集成用例：无 title、指纹与首页同源（Server 头）→ 无新指纹 → 丢
	mux.HandleFunc("/nt-same", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "SimpleHTTP/0.6 Python/3.11.0")
		fmt.Fprint(w, `<html><body>nothing here</body></html>`)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	common.WebTimeout = 5 // ReadRawWithSize 内部用全局 WebTimeout 建 ctx，零值会立即超时只读出头
	t.Cleanup(func() { common.WebTimeout = 0 })
	client := &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= dirRedirectLimit {
				return http.ErrUseLastResponse
			}
			orig := via[0].URL
			if orig.Host != req.URL.Host {
				return http.ErrUseLastResponse
			}
			base := strings.TrimSuffix(orig.Path, "/")
			if base == "" {
				return http.ErrUseLastResponse
			}
			if req.URL.Path != base && !strings.HasPrefix(req.URL.Path, base+"/") {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	// 自适应 seeds：先探测首页/落地页的真实呈现（不依赖指纹库具体产品名）
	landingKey := probePresent(t, client, srv.URL, "/landing")
	homeKey := probePresent(t, client, srv.URL, "/hit") // 与首页同指纹不同标题场景用
	seeds := map[string]struct{}{landingKey: {}, homeKey: {}}

	run := func(path string) *dirSite {
		t.Helper()
		s := &dirSite{url: srv.URL, seeds: seeds, seedFps: seedFingerprintUnion(seeds), seen: map[string]struct{}{}}
		runDirTask(client, dirTask{site: s, path: path})
		return s
	}

	if s := run("/gone"); len(s.hits) != 0 {
		t.Error("404 路径应零命中")
	}
	if s := run("/denied"); len(s.hits) != 0 {
		t.Error("403 路径应零命中（用户决策屏蔽，即使响应头带可识别指纹）")
	}
	if s := run("/auth401"); len(s.hits) != 0 {
		t.Error("401 路径应零命中（4xx 全屏蔽，即使带 WWW-Authenticate 特征）")
	}
	if s := run("/plain"); len(s.hits) != 0 {
		t.Error("无指纹 200 应零命中")
	}
	if s := run("/rand"); len(s.hits) != 0 {
		t.Error("跨前缀 302（软404）应零命中：不跟随则 302 响应无指纹")
	}
	if s := run("/ext"); len(s.hits) != 0 {
		t.Error("跨 host 302 应零命中")
	}
	if s := run("/wp-admin"); len(s.hits) != 1 {
		t.Errorf("同指纹不同标题（后台救回）应 1 命中，got %d", len(s.hits))
	} else if !strings.Contains(s.hits[0].url, "/wp-admin") {
		t.Errorf("命中 url 应为原始请求路径，got %s", s.hits[0].url)
	}
	if s := run("/sub"); len(s.hits) != 1 {
		t.Errorf("同前缀 302 跟随后应 1 命中，got %d", len(s.hits))
	} else if !strings.Contains(s.hits[0].url, "/sub") {
		t.Errorf("命中 url 应保留原始路径 /sub，got %s", s.hits[0].url)
	}

	// 空标题集成验证（自适应 seeds；"空标题+新指纹"分支由 TestSiteAdmit 纯函数覆盖——
	// 指纹引擎对同一 body 的识别数量不稳定（实测抖动），不适合构造指纹差集成用例）
	ntKey := probePresent(t, client, srv.URL, "/nt-same") // (FA, None)
	ntFps := strings.Split(strings.SplitN(ntKey, "\x00", 2)[0], "\x1f")
	ntSeeds := map[string]struct{}{presentKey(ntFps, "Home"): {}} // 模拟首页：同指纹、有真实标题
	ntSite := &dirSite{url: srv.URL, seeds: ntSeeds, seedFps: seedFingerprintUnion(ntSeeds), seen: map[string]struct{}{}}
	runDirTask(client, dirTask{site: ntSite, path: "/nt-same"})
	if len(ntSite.hits) != 0 {
		t.Errorf("空标题且指纹全为首页已见应零命中，got %d", len(ntSite.hits))
	}
}

// ---------- 站点内聚合去重 ----------

func TestAggregatorDedupKeepsFirstURL(t *testing.T) {
	mux := http.NewServeMux()
	page := func(w http.ResponseWriter, r *http.Request) {
		code, body := fingerPage("Same")
		w.Header().Set("Server", "SimpleHTTP/0.6 Python/3.11.0")
		w.WriteHeader(code)
		fmt.Fprint(w, body)
	}
	mux.HandleFunc("/first", page)
	mux.HandleFunc("/second", page)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	s := &dirSite{url: srv.URL, seeds: map[string]struct{}{}, seen: map[string]struct{}{}}
	client := &http.Client{Timeout: 5 * time.Second}
	runDirTask(client, dirTask{site: s, path: "/first"})
	runDirTask(client, dirTask{site: s, path: "/second"})

	if len(s.hits) != 1 {
		t.Fatalf("同呈现应去重为 1 条，got %d", len(s.hits))
	}
	if !strings.Contains(s.hits[0].url, "/first") {
		t.Fatalf("去重后应保留首个命中路径，got %s", s.hits[0].url)
	}
}

// ---------- -dirf 外部字典覆盖 + 3 条只发 3 请求 ----------

func TestDirDictFileOverride(t *testing.T) {
	common.DirScanEnable = true
	t.Cleanup(func() { common.DirScanEnable = false })

	var mu sync.Mutex
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests++
		mu.Unlock()
		http.NotFound(w, r) // 全 404：只验证请求数
	}))
	defer srv.Close()

	dirFile := filepath.Join(t.TempDir(), "dirs.txt")
	if err := os.WriteFile(dirFile, []byte("/one\r\n/two\n\n/three  \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	common.DirDictFile = dirFile
	t.Cleanup(func() { common.DirDictFile = "" })

	dirPathsOnce = sync.Once{} // 重置惰性加载以验证 -dirf 加载路径
	loadDirPaths()
	if len(dirPaths) != 3 {
		t.Fatalf("-dirf 应加载 3 条（含 CRLF/空白清洗），got %d: %v", len(dirPaths), dirPaths)
	}

	resetDirSchedForTest()
	RegisterSite(srv.URL, nil)
	SealAndDrain()

	mu.Lock()
	defer mu.Unlock()
	if requests != 3 {
		t.Fatalf("3 条字典应恰好发 3 个请求，got %d", requests)
	}
}

// ---------- 站点熔断：连续失败放弃剩余路径 ----------

func TestSiteAbortOnConsecutiveFailures(t *testing.T) {
	dirTestSetup(t, []string{"/p1", "/p2", "/p3", "/p4", "/p5", "/p6", "/p7", "/p8"})

	newSite := func() *dirSite {
		return &dirSite{url: "http://down.test", seeds: map[string]struct{}{}, seedFps: map[string]struct{}{}, seen: map[string]struct{}{}}
	}
	// 直调计数器验证熔断逻辑（不真发请求：Windows 对未监听端口是 drop，
	// 走网络会烧满超时拖慢测试；runDirTask→noteSiteFail 接线由代码覆盖）
	s := newSite()
	for i := 0; i < dirAbortThresh; i++ {
		dirSched.noteSiteFail(s)
	}
	if s.idx != len(dirPaths) {
		t.Fatalf("连续 %d 次失败应熔断（idx 跳到顶 %d），got idx=%d", dirAbortThresh, len(dirPaths), s.idx)
	}
	if !s.drained() {
		t.Fatalf("熔断站点应 drained（inflight=%d）", s.inflight)
	}

	// 成功响应清零计数：失败 1 次 → OK → 再失败 阈值-1 次不应熔断
	s2 := newSite()
	dirSched.noteSiteFail(s2)
	dirSched.noteSiteOK(s2)
	for i := 0; i < dirAbortThresh-1; i++ {
		dirSched.noteSiteFail(s2)
	}
	if s2.idx != 0 {
		t.Fatalf("OK 清零后 %d 次失败不应熔断（阈值 %d），idx=%d", dirAbortThresh-1, dirAbortThresh, s2.idx)
	}
}

// ---------- 呈现判定纯函数（隔离指纹引擎的输出不稳定性） ----------

func TestSiteAdmit(t *testing.T) {
	// 首页基准：指纹 {A}，呈现 (A,"Home")
	seeds := map[string]struct{}{presentKey([]string{"A"}, "Home"): {}}
	s := &dirSite{seeds: seeds, seedFps: map[string]struct{}{"A": {}}, seen: map[string]struct{}{}}

	cases := []struct {
		name  string
		fps   []string
		title string
		want  bool
	}{
		{"非空标题+新标题(同产品后台救回)", []string{"A"}, "Admin", true},
		{"非空标题+首页呈现重合", []string{"A"}, "Home", false},
		{"非空标题+新指纹", []string{"A", "B"}, "Home", true},
		{"空标题+指纹全为首页已见", []string{"A"}, "None", false},
		{"空标题+有新指纹", []string{"A", "B"}, "None", true},
		{"空标题+全新指纹", []string{"C"}, "None", true},
	}
	for _, c := range cases {
		if got := s.admit(c.fps, c.title); got != c.want {
			t.Errorf("%s: admit(%v,%q)=%v, want %v", c.name, c.fps, c.title, got, c.want)
		}
	}
}

// ---------- 注册门槛：根路径 body 黑洞的站点不进目录扫描 ----------

func TestWebScanWorkerRegistersOnlyHealthySites(t *testing.T) {
	common.DirScanEnable = true
	common.WebTimeout = 1 // 缩短超时：挂起站点 ~1s 即放弃
	lib.Inithttp()
	t.Cleanup(func() {
		common.DirScanEnable = false
		common.WebTimeout = 0
	})
	setTestDirPaths([]string{"/p1"})
	resetDirSchedForTest()

	// 健康站点：完整响应
	healthy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		code, body := fingerPage("Healthy")
		w.Header().Set("Server", "SimpleHTTP/0.6 Python/3.11.0")
		w.WriteHeader(code)
		fmt.Fprint(w, body)
	}))
	defer healthy.Close()

	// 半死站点：响应头即达、body 挂起到超时（不 Close 它——挂起 handler 会阻塞
	// httptest.Close，泄漏一个测试协程到进程结束可接受）
	blackhole := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "SimpleHTTP/0.6 Python/3.11.0")
		w.WriteHeader(200)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		<-make(chan struct{})
	}))

	for _, srv := range []*httptest.Server{healthy, blackhole} {
		func() {
			defer func() { _ = recover() }()
			host, port, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
			_ = WebScanWorker(&common.HostInfo{Host: host, Ports: port})
		}()
	}

	dirSched.mu.Lock()
	registered := len(dirSched.sites)
	firstURL := ""
	if registered > 0 {
		firstURL = dirSched.sites[0].url
	}
	dirSched.mu.Unlock()

	// 必须收尾：本测试经真 RegisterSite 启动了 worker/调度器/进度循环，
	// 不 Seal 会让残留 goroutine 与下个测试的 resetDirSchedForTest 构成 data race
	SealAndDrain()

	if registered != 1 {
		t.Fatalf("应只注册健康站点 1 个（body 黑洞站点不注册），got %d", registered)
	}
	if firstURL != healthy.URL {
		t.Fatalf("注册的应是健康站点 %s，got %s", healthy.URL, firstURL)
	}
}

// ---------- 慢站点画像行与连续慢熔断 ----------

func TestSlowSiteLine(t *testing.T) {
	// 排队假象：墙钟跨度 71s 但纯路径耗时 7s（130 站点均摊的正常形态）——不打
	queued := &dirSite{startedAt: time.Now().Add(-71 * time.Second), idx: 70, totalDur: 7 * time.Second}
	if got := queued.slowSiteLine(); got != "" {
		t.Errorf("纯路径耗时 7s（阈值 %v）的排队假象不应打画像行，got %q", dirSlowSiteReport, got)
	}
	slow := &dirSite{
		url:       "http://slow.test",
		startedAt: time.Now().Add(-360 * time.Second),
		idx:       70,
		totalDur:  357 * time.Second,
		maxPath:   5400 * time.Millisecond,
		slowPaths: 70,
		pathFails: 1,
	}
	got := slow.slowSiteLine()
	if got == "" {
		t.Fatal("纯路径耗时 357s 应打画像行")
	}
	// 形态判读要素：avg 5.1s（慢成功型，真机 208 实证形态）
	for _, want := range []string{"slow.test", "70 paths", "357s", "avg 5.1s", "5.4s", "70 slow", "1 failed"} {
		if !strings.Contains(got, want) {
			t.Errorf("画像行应含 %q: %s", want, got)
		}
	}
}

func TestSiteAbortOnConsecutiveSlowPaths(t *testing.T) {
	dirTestSetup(t, []string{"/p1", "/p2", "/p3", "/p4", "/p5", "/p6"})

	// 连续 3 条慢路径（>dirSlowPathMark）触发熔断，idx 跳到顶
	s := &dirSite{url: "http://slowalive.test"}
	for i := 0; i < dirAbortThresh; i++ {
		dirSched.notePathDone(s, 3*time.Second)
	}
	if s.idx != len(dirPaths) {
		t.Fatalf("连续 %d 条慢路径应熔断（idx 跳到顶 %d），got %d", dirAbortThresh, len(dirPaths), s.idx)
	}

	// 慢快交替不熔断：慢 2 条 → 快 1 条清零 → 再慢 2 条，未达连续阈值
	s2 := &dirSite{url: "http://jitter.test"}
	dirSched.notePathDone(s2, 3*time.Second)
	dirSched.notePathDone(s2, 3*time.Second)
	dirSched.notePathDone(s2, 100*time.Millisecond)
	dirSched.notePathDone(s2, 3*time.Second)
	dirSched.notePathDone(s2, 3*time.Second)
	if s2.idx != 0 {
		t.Fatalf("慢快交替（连续未达 %d）不应熔断，idx=%d", dirAbortThresh, s2.idx)
	}
}

// ---------- 集成：完整流程（RegisterSite 自启动 worker + SealAndDrain 收尾） ----------

func TestDirScanEndToEnd(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.NotFound(w, r) // 站点根 404 不影响目录扫描（注册由 WebScan 层决定）
			return
		}
		code, body := fingerPage("Found")
		w.Header().Set("Server", "SimpleHTTP/0.6 Python/3.11.0")
		w.WriteHeader(code)
		fmt.Fprint(w, body)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	dirTestSetup(t, []string{"/aaa", "/bbb", "/ccc", "/ddd"})

	RegisterSite(srv.URL, nil) // 4 路径全部命中且呈现相同 → 去重为 1
	SealAndDrain()

	dirSched.mu.Lock()
	defer dirSched.mu.Unlock()
	if dirSched.totalFinds != 1 {
		t.Fatalf("4 同呈现路径应聚合为 1 发现，got %d", dirSched.totalFinds)
	}
	if len(dirSched.reg) != 1 {
		t.Fatalf("应注册 1 站点，got %d", len(dirSched.reg))
	}
	for _, s := range dirSched.sites {
		if s.hits != nil || s.seeds != nil || s.seen != nil {
			t.Fatal("站点排空后聚合器应已释放（置 nil）")
		}
	}
}
