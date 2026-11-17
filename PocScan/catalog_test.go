package PocScan

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/killmonday/fscanx/common"
	"github.com/killmonday/fscanx/PocScan/lib"
)

// TestLoadBindsExternalCatalog 外置产品册分支（BL-POC-009）：externalCatalog 非空时
// loadBindsFile 走产品册合并（pocs 段投影 + MergeBinds 同 name 覆盖/异名追加），
// 不再读旧 binds.yaml。
func TestLoadBindsExternalCatalog(t *testing.T) {
	origExternal, origBinds := externalCatalog, PocBinds
	t.Cleanup(func() {
		externalCatalog, PocBinds = origExternal, origBinds
	})
	SetExternalCatalog([]CatalogEntry{
		{Name: "泛微 e-cology", Pocs: []string{"poc-external-override"}}, // 同 name 覆盖出厂
		{Name: "外置新产品", Pocs: []string{"poc-external-new"}},          // 异名追加
	})
	loadBindsFile()
	byProduct := map[string][]string{}
	for _, b := range PocBinds {
		byProduct[b.Product] = b.Pocs
	}
	if got := byProduct["泛微 e-cology"]; len(got) != 1 || got[0] != "poc-external-override" {
		t.Fatalf("外置同 name 未整条覆盖出厂 pocs: %v", got)
	}
	if got := byProduct["外置新产品"]; len(got) != 1 || got[0] != "poc-external-new" {
		t.Fatalf("外置异名未追加: %v", got)
	}
	if got := byProduct["thinkphp"]; len(got) == 0 {
		t.Fatal("出厂兜底条目丢失（seed 打底失效）")
	}
}

// TestCatalogSeedWeaverSplit 出厂产品册 weaver/yongyou 拆分语义（BL-POC-009）：
// 英文旧键退役，pocs 按路径特征实证拆到各子产品标准名条目。
func TestCatalogSeedWeaverSplit(t *testing.T) {
	binds, err := catalogSeed()
	if err != nil {
		t.Fatal(err)
	}
	byProduct := map[string][]string{}
	for _, b := range binds {
		byProduct[b.Product] = b.Pocs
		for _, stale := range []string{"weaver", "weaver-ebridge", "yongyou", "seeyon", "tongda", "landray"} {
			if b.Product == stale {
				t.Fatalf("英文旧键残留（应为中文标准名或自标准名）: %s", b.Product)
			}
		}
	}
	want := map[string][]string{
		"泛微 e-cology": {"poc-yaml-weaver-E-Cology-getSqlData-sqli", "poc-yaml-weaver-oa-arbitrary-file-upload"},
		"泛微 云桥e-bridge":  {"poc-yaml-weaver-ebridge-file-read"},
		"泛微 e-office":     {"poc-yaml-weaver-oa-eoffice-v9-upload-getshell"},
		"用友 U8 OA":        {"poc-yaml-yongyou-u8-oa-sqli"},
	}
	for std, pocs := range want {
		got, ok := byProduct[std]
		if !ok {
			t.Fatalf("产品册缺条目: %s", std)
		}
		if len(got) != len(pocs) {
			t.Fatalf("%s pocs 数不符: got %v want %v", std, got, pocs)
		}
		set := map[string]bool{}
		for _, p := range got {
			set[p] = true
		}
		for _, p := range pocs {
			if !set[p] {
				t.Fatalf("%s 缺 poc %s: got %v", std, p, got)
			}
		}
	}
	// 反向约束：e-cology 条目不得混入其它子产品的 poc（防"一锅端"原状）
	for _, p := range byProduct["泛微 e-cology"] {
		if p == "poc-yaml-weaver-ebridge-file-read" || p == "poc-yaml-weaver-oa-eoffice-v9-upload-getshell" {
			t.Fatalf("e-cology 条目混入子产品 poc: %s", p)
		}
	}
}

// TestBoundPocExactIndex 绑定名单精确索引（BL-POC-009）：名字互为前缀的两个 poc
// 只执行被绑定的那个——filterPoc Contains 子串扩张会把前缀同名 poc 一并带出（双跑），
// 精确索引路径必须只发被绑定 poc 的请求。
func TestBoundPocExactIndex(t *testing.T) {
	var hits int32
	pathA := "/exact-index-marker-a"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == pathA {
			atomic.AddInt32(&hits, 1)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	origPocs, origPocInfo, origCookie := AllPocs, common.Pocinfo, common.Cookie
	t.Cleanup(func() {
		AllPocs, common.Pocinfo, common.Cookie = origPocs, origPocInfo, origCookie
	})
	// HTTP client 是显式初始化（Inithttp 由扫描器入口调用，非包 init）——测试进程必须
	// 自行初始化，否则 Client 为 nil，executePoc nil deref panic 被 CheckMultiPoc 的
	// goroutine 级 recover 吞掉、wg.Done 跳过 → wg.Wait 死锁（探针轮实证）。
	lib.Inithttp()
	common.Pocinfo = common.PocInfo{} // 空 PocName = 智能定向
	common.Cookie = ""
	// PocNum 默认值由 flag.Parse 赋值（测试进程不跑）→ 零值 0 = CheckMultiPoc 零 worker
	// → tasks 发送永久阻塞（goroutine dump 实证）。测试显式设 1。
	origPocNum := common.PocNum
	common.PocNum = 1
	t.Cleanup(func() { common.PocNum = origPocNum })

	oldBinds := PocBinds
	t.Cleanup(func() { PocBinds = oldBinds })

	info := &common.HostInfo{Url: srv.URL, Frames: []string{"poc-yaml-exact-short"}}
	// 首调触发 once.Do(initpoc)（加载真实产品册 binds 对测试 frame 无命中，零请求）——
	// 若先设 PocBinds 再调 PocScan，loadBindsFile 会把测试数据覆盖回真实产品册（探针轮实证）
	PocScan(info)
	// 名字互为前缀：绑定名单只引用 short，精确索引不得带出 short-suffix
	PocBinds = []Bind{{Product: "poc-yaml-exact-short", Pocs: []string{"poc-yaml-exact-short"}}}
	AllPocs = []*lib.Poc{
		{Name: "poc-yaml-exact-short", Rules: []lib.Rules{{
			Method: "GET", Path: pathA,
			Expression: "response.status == 200",
		}}},
		{Name: "poc-yaml-exact-short-suffix", Rules: []lib.Rules{{
			Method: "GET", Path: "/exact-index-marker-b",
			Expression: "response.status == 200",
		}}},
	}
	PocScan(info)

	if hits != 1 {
		t.Fatalf("绑定名单应只发 1 次被绑定 poc 请求, got %d（Contains 子串扩张会双跑/带出 suffix）", hits)
	}
}

// TestWeaverSampleEndToEnd 泛微样例端到端（BL-POC-009 验收第一条）：出厂产品册绑定下，
// frame=「泛微 e-cology」定向发出 e-cology 条目的测试包，且【不发出】其它子产品
// （云桥/e-office）的测试包——反向约束防"一锅端"原状蒙混过验。
func TestWeaverSampleEndToEnd(t *testing.T) {
	var ecoHits, bridgeHits, officeHits int32
	markers := map[string]*int32{
		"/weaver-eco-marker":    &ecoHits,
		"/weaver-bridge-marker": &bridgeHits,
		"/weaver-office-marker": &officeHits,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h, ok := markers[r.URL.Path]; ok {
			atomic.AddInt32(h, 1)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	origPocs, origPocInfo, origCookie := AllPocs, common.Pocinfo, common.Cookie
	t.Cleanup(func() { AllPocs, common.Pocinfo, common.Cookie = origPocs, origPocInfo, origCookie })
	lib.Inithttp() // 显式初始化（非包 init），否则 Client nil panic → wg 死锁（探针轮实证）
	common.Pocinfo = common.PocInfo{}
	common.Cookie = ""
	origPocNum := common.PocNum
	common.PocNum = 1
	t.Cleanup(func() { common.PocNum = origPocNum })

	// 显式初始化（两个全局单次/惰性机制的时序坑，单跑/合跑/全包三轮实证）：
	// ① once.Do(initpoc) 全局只跑一次——必须在 AllPocs 覆盖前耗掉，否则单跑时首调
	//    PocScan 触发 initpoc 把探测 poc 覆盖回真实库致零发包；
	// ② 前序测试 cleanup 恢复的 PocBinds 可能是其捕获值（首个触发 initpoc 的测试捕获
	//    为 nil），once 已耗无法重载——直调 loadBindsFile（非 once 保护，只动 PocBinds）
	//    强制刷新为真实出厂绑定。
	once.Do(initpoc)
	loadBindsFile()
	AllPocs = []*lib.Poc{
		{Name: "poc-yaml-weaver-E-Cology-getSqlData-sqli", Rules: []lib.Rules{{Method: "GET", Path: "/weaver-eco-marker", Expression: "response.status == 200"}}},
		{Name: "poc-yaml-weaver-oa-arbitrary-file-upload", Rules: []lib.Rules{{Method: "GET", Path: "/weaver-eco-marker", Expression: "response.status == 200"}}},
		{Name: "poc-yaml-weaver-ebridge-file-read", Rules: []lib.Rules{{Method: "GET", Path: "/weaver-bridge-marker", Expression: "response.status == 200"}}},
		{Name: "poc-yaml-weaver-oa-eoffice-v9-upload-getshell", Rules: []lib.Rules{{Method: "GET", Path: "/weaver-office-marker", Expression: "response.status == 200"}}},
	}
	PocScan(&common.HostInfo{Url: srv.URL, Frames: []string{"泛微 e-cology"}})

	if atomic.LoadInt32(&ecoHits) == 0 {
		t.Fatal("出厂绑定未生效：frame=泛微 e-cology 应定向发出 e-cology 条目测试包（改前 weaver 键零发包）")
	}
	if atomic.LoadInt32(&bridgeHits) != 0 || atomic.LoadInt32(&officeHits) != 0 {
		t.Fatalf("子产品隔离失效：e-cology 目标不应发出云桥(%d)/e-office(%d)测试包", atomic.LoadInt32(&bridgeHits), atomic.LoadInt32(&officeHits))
	}
}

// TestPocNameSubstringGuard -pocname 模糊语义护栏（BL-POC-009 铁律 4）：精确索引仅限
// 绑定名单路径，显式 -pocname <子串> 的 filterPoc Contains 语义保持不变（BL-POC-001 承诺）
// ——与 TestBoundPocExactIndex 形成对照：同名前缀字段绑定路径发 1 次、-pocname 路径命中 2 个。
func TestPocNameSubstringGuard(t *testing.T) {
	origPocs := AllPocs
	t.Cleanup(func() { AllPocs = origPocs })
	AllPocs = []*lib.Poc{
		{Name: "poc-yaml-exact-short"},
		{Name: "poc-yaml-exact-short-suffix"},
		{Name: "poc-yaml-unrelated"},
	}
	got := filterPoc("exact-short")
	if len(got) != 2 {
		t.Fatalf("-pocname 子串过滤应命中 2 个（含前缀同名），got %d——Contains 模糊语义被破坏", len(got))
	}
}

// TestMigratedBindEquivalence 迁移行为等价（BL-POC-009 验收：迁移前后对同一目标的定向
// 发包集合一致）：同一组绑定数据，旧体系直通 PocBinds 与迁移投影形态（CatalogEntry
// name/pocs → Bind 投影，即 loadBindsFile 外置分支语义）在 httptest 端到端发包集合相等。
func TestMigratedBindEquivalence(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/migrate-equiv-marker" {
			atomic.AddInt32(&hits, 1)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	origPocs, origPocInfo, origCookie := AllPocs, common.Pocinfo, common.Cookie
	t.Cleanup(func() { AllPocs, common.Pocinfo, common.Cookie = origPocs, origPocInfo, origCookie })
	lib.Inithttp()
	common.Pocinfo = common.PocInfo{}
	common.Cookie = ""
	origPocNum := common.PocNum
	common.PocNum = 1
	t.Cleanup(func() { common.PocNum = origPocNum })

	// once 可能未触发（单跑本测试时）：先耗掉 initpoc（它会覆盖 AllPocs 与 PocBinds），
	// 再覆盖 AllPocs / 设测试绑定——顺序反了探测 poc 会被真实库覆盖致零发包（单跑实证）
	once.Do(initpoc)

	AllPocs = []*lib.Poc{
		{Name: "poc-yaml-legacy-bind-poc", Rules: []lib.Rules{{Method: "GET", Path: "/migrate-equiv-marker", Expression: "response.status == 200"}}},
	}

	// 旧体系绑定行（外置 binds.yaml 语义：product 旧键 + pocs）
	legacyBinds := []Bind{{Product: "legacy-product-key", Pocs: []string{"poc-yaml-legacy-bind-poc"}}}
	// 迁移投影形态：insertLegacyBinds 原名立条（CatalogEntry）→ loadBindsFile 投影回 Bind
	migratedBinds := []Bind{}
	for _, e := range []CatalogEntry{{Name: "legacy-product-key", Pocs: []string{"poc-yaml-legacy-bind-poc"}}} {
		migratedBinds = append(migratedBinds, Bind{Product: e.Name, Pocs: e.Pocs})
	}

	oldBinds := PocBinds
	t.Cleanup(func() { PocBinds = oldBinds })

	PocBinds = legacyBinds
	PocScan(&common.HostInfo{Url: srv.URL, Frames: []string{"legacy-product-key"}})
	oldCount := atomic.LoadInt32(&hits)

	PocBinds = migratedBinds
	PocScan(&common.HostInfo{Url: srv.URL, Frames: []string{"legacy-product-key"}})
	migratedCount := atomic.LoadInt32(&hits) - oldCount

	if oldCount == 0 || oldCount != migratedCount {
		t.Fatalf("迁移前后发包集合不一致: old=%d migrated=%d", oldCount, migratedCount)
	}
}
