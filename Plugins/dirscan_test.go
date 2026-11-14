package Plugins

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

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
	// 空标题用例（用户规则：空标题不算差异，退化为指纹差）
	// /nt-same：无 title、指纹与首页同源（Server 头）→ 无新指纹 → 丢
	// /nt-new：无 title、body 带目录列表特征文案（directory 类指纹）→ 有新指纹 → 出
	mux.HandleFunc("/nt-same", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "SimpleHTTP/0.6 Python/3.11.0")
		fmt.Fprint(w, `<html><body>nothing here</body></html>`)
	})
	mux.HandleFunc("/nt-new", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "SimpleHTTP/0.6 Python/3.11.0")
		fmt.Fprint(w, `<html><body><h1>Directory listing for /</h1><hr><a href="../">../</a><hr></body></html>`)
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

	// 空标题规则（自适应：先取 /nt-same 的真实指纹集构造首页基准，不依赖指纹库具体产品名）
	ntKey := probePresent(t, client, srv.URL, "/nt-same") // (FA, None)
	ntFps := strings.Split(strings.SplitN(ntKey, "\x00", 2)[0], "\x1f")
	// 前置断言：/nt-new 必须比 /nt-same 多至少一个指纹（目录列表特征文案生效），
	// 若指纹库演进导致失效，此处 Fail 提示重造测试页而不是静默跳过
	nwKey := probePresent(t, client, srv.URL, "/nt-new")
	nwFps := strings.Split(strings.SplitN(nwKey, "\x00", 2)[0], "\x1f")
	if len(nwFps) <= len(ntFps) {
		t.Fatalf("测试页构造失效：/nt-new 指纹(%d) 应多于 /nt-same(%d)，需重造特征 body", len(nwFps), len(ntFps))
	}
	ntSeeds := map[string]struct{}{presentKey(ntFps, "Home"): {}} // 模拟首页：同指纹、有真实标题
	ntRun := func(path string) *dirSite {
		s := &dirSite{url: srv.URL, seeds: ntSeeds, seedFps: seedFingerprintUnion(ntSeeds), seen: map[string]struct{}{}}
		runDirTask(client, dirTask{site: s, path: path})
		return s
	}
	if s := ntRun("/nt-same"); len(s.hits) != 0 {
		t.Errorf("空标题且指纹全为首页已见应零命中，got %d", len(s.hits))
	}
	if s := ntRun("/nt-new"); len(s.hits) != 1 {
		t.Errorf("空标题但有新指纹（目录列表特征）应 1 命中，got %d", len(s.hits))
	} else if s.hits[0].title != "None" {
		t.Errorf("命中标题应为 None 占位，got %q", s.hits[0].title)
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
