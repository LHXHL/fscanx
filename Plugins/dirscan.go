package Plugins

// 指纹导向目录扫描子系统（BL-DIR-001）。
//
// 调度单元是 (站点,路径) 对而非站点：轮询环 + per-site 在途门闩保证任意时刻
// 单站点在途请求 ≤1、多站点请求在时间上交错，避免目录扫描流量集中轰单个站点。
// 停机采用两阶段协议（Seal 关闭注册 → Drain 排空），调度器不因"暂时无活"退出，
// 解决流水线模式下站点渐进注册与收尾的竞争。站点级连续失败熔断（dirAbortThresh）
// 防半死站点 70×超时拖垮收尾。
//
// 判定管线（四道闸，经六轮真机迭代收敛）：状态码 ≥300 全短路（仅 2xx 参与识别，
// 3xx 响应头会被引擎误匹配、4xx/5xx 噪声）→ 302 同源同前缀跟随（发射端省流量）→
// 指纹非空（200 型软 404 免疫）→ 呈现差集（(指纹,标题) 二元组与首页已见呈现
// 完全重合则丢弃，302 型软 404 免疫；空标题退化为纯指纹差；判定与输出解耦，
// 输出带完整指纹）。

import (
	_ "embed"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/killmonday/fscanx/PocScan/lib"
	"github.com/killmonday/fscanx/common"
)

const (
	dirWorkerNum     = 16               // worker 池大小；任务 channel 容量与之相等
	dirRedirectLimit = 3                // 目录请求跟随跳转上限
	dirAbortThresh   = 5                // 站点连续请求失败熔断阈值（防半死站点 70×超时拖垮收尾）
	dirProgressEvery = 30 * time.Second // 进度快照间隔：打印量恒定不刷屏，仅扫描变慢时可见
)

// ---------- 字典 ----------

//go:embed dirlist.txt
var dirListEmbedded []byte

var (
	dirPaths     []string
	dirPathsErr  error
	dirPathsOnce sync.Once
)

// loadDirPaths 惰性加载路径字典：-dirf 外部文件优先，否则内置 embed。
// 行经 TrimSpace（字典为 CRLF 行尾），空行与 # 注释行跳过。
func loadDirPaths() {
	dirPathsOnce.Do(func() {
		data := dirListEmbedded
		if common.DirDictFile != "" {
			b, err := os.ReadFile(common.DirDictFile)
			if err != nil {
				dirPathsErr = err
				// 失败即时提示（Once 天然只提示一次）：避免扫完全部目标后才知道 -dir 未生效
				common.Printf("[-] dirscan disabled: %v\n", err)
				return
			}
			data = b
		}
		dirPaths = nil // Once 可被测试重置，二次执行时防旧字典叠加
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if !strings.HasPrefix(line, "/") {
				line = "/" + line
			}
			dirPaths = append(dirPaths, line)
		}
		if len(dirPaths) == 0 {
			dirPathsErr = errors.New("empty dir dictionary")
			common.Printf("[-] dirscan disabled: %v\n", dirPathsErr) // 与文件读取失败同样即时提示
		}
	})
}

// ---------- 呈现（指纹+标题 二元组）----------

// presentKey 生成呈现的规范化 key。指纹必须排序后拼接：指纹引擎输出顺序
// 非确定（pre-existing，BL-MEM-003 实证），不排序则差集/去重判定失效。
func presentKey(fps []string, title string) string {
	sorted := append([]string(nil), fps...)
	sort.Strings(sorted)
	return strings.Join(sorted, "\x1f") + "\x00" + title
}

// normalizeTitle 标题规范化：空/纯空白 → "None"。首页（DoWebScan）与目录扫描
// （runDirTask）两处必须一致，否则同一页面两侧生成不同呈现 key、差集判定失效。
func normalizeTitle(title string) string {
	if strings.TrimSpace(title) == "" {
		return "None"
	}
	return title
}

// ---------- 站点 ----------

// dirHit 一条有效命中（站点内按呈现去重后）
type dirHit struct {
	presentKey string
	url        string // 站点URL + 首个命中路径
	status     int
	title      string
	fps        string // 完整指纹列表（逗号分隔），与首页 Product 行同构
}

// dirSite 轮询环中的站点条目。seeds 为首页扫描链（初次+跳转+https重试）
// 收集的呈现集合；seedFps 为首页全部指纹元素的并集（空标题路径的判定
// 基准，用户规则 2026-11-14：空标题不算标题差异，退化为指纹差）；
// hits/seen 构成站点聚合器，站点扫完即释放（置 nil）。
type dirSite struct {
	url         string
	idx         int // 下一条待发射路径的下标
	inflight    int // 在途请求计数（>0 时调度器跳过该站点）
	consecFails int // 连续请求失败计数（超时/连接失败/body 黑洞），达阈值熔断
	seeds       map[string]struct{}
	seedFps     map[string]struct{}
	hits        []dirHit
	seen        map[string]struct{}
}

// noteSiteFail 连续失败计数；达 dirAbortThresh 熔断该站点（剩余路径全部跳过）。
// 背景：半死站点（握手成功/头能回/body 黑洞）会通过首页扫描注册进来，其后每条
// 路径烧满 WebTimeout 且单站点在途≤1 串行——最坏 70×9s 纯超时并拖死收尾排空
// （真机实测"部分网段扫描很久"的根因）。熔断只惩罚连续失败，正常站点不受影响。
func (d *dirScheduler) noteSiteFail(s *dirSite) {
	d.mu.Lock()
	s.consecFails++
	abort := s.consecFails >= dirAbortThresh && s.idx < len(dirPaths)
	var skipped int
	if abort {
		skipped = len(dirPaths) - s.idx
		s.idx = len(dirPaths) // 跳过剩余路径；当前任务 complete 归零在途后站点自然 drained
	}
	url := s.url
	fails := s.consecFails
	d.mu.Unlock()
	if abort {
		// 锁外发送（LogSuccess 可能阻塞在 Results 通道）
		common.LogSuccess("[-] dirscan abort %s: %d consecutive failures, skip %d paths", url, fails, skipped)
	}
}

// noteSiteOK 拿到有效响应（含短路丢弃的 404 等——服务器是活的）清零连续失败计数。
func (d *dirScheduler) noteSiteOK(s *dirSite) {
	d.mu.Lock()
	s.consecFails = 0
	d.mu.Unlock()
}

// seedFingerprintUnion 从呈现 key 集合拆出全部指纹元素的并集。
// key 格式见 presentKey（排序指纹 "\x1f" join + "\x00" + 标题），
// 指纹名为产品串不含控制字符，拆分安全。
func seedFingerprintUnion(seeds map[string]struct{}) map[string]struct{} {
	union := make(map[string]struct{})
	for k := range seeds {
		fpPart := k
		if i := strings.IndexByte(k, 0x00); i >= 0 {
			fpPart = k[:i]
		}
		for _, fp := range strings.Split(fpPart, "\x1f") {
			if fp != "" {
				union[fp] = struct{}{}
			}
		}
	}
	return union
}

func (s *dirSite) drained() bool { return s.idx >= len(dirPaths) && s.inflight == 0 }

// ---------- 调度器 ----------

type dirTask struct {
	site *dirSite
	path string
}

// dirScheduler 单 goroutine 轮询环调度器。
// 无可发射任务且未停机时阻塞在 wake 上（不持锁、不自旋），
// 由 worker 完成 / 新注册 / Seal 唤醒。
type dirScheduler struct {
	mu         sync.Mutex
	sites      []*dirSite
	reg        map[string]struct{}
	rr         int
	sealed     bool
	started    bool
	totalFinds int // 累计命中数（flush 时计入，站点 hits 释放后统计仍准确）
	workers    sync.WaitGroup
	taskCh     chan dirTask
	wake       chan struct{}
	done       chan struct{}
}

var dirSched = &dirScheduler{
	reg:    make(map[string]struct{}),
	taskCh: make(chan dirTask, dirWorkerNum),
	wake:   make(chan struct{}, 1),
	done:   make(chan struct{}),
}

// progressLoop 周期性打印进度快照（时间驱动而非百分比里程碑：站点渐进注册、
// 分母不单调，N/M 会误导；时间驱动的打印量恒定——快扫描完全看不到，慢扫描
// 每 30s 一行让使用者知道仍在推进，且仅在有新进展时打印防重复刷屏）。
func (d *dirScheduler) progressLoop() {
	lastDone := -1
	for {
		select {
		case <-d.done:
			return
		case <-time.After(dirProgressEvery):
		}
		d.mu.Lock()
		donePaths, pendingSites, total := 0, 0, len(d.sites)
		for _, s := range d.sites {
			donePaths += s.idx - s.inflight
			if !s.drained() {
				pendingSites++
			}
		}
		findings := d.totalFinds
		changed := donePaths != lastDone
		lastDone = donePaths
		d.mu.Unlock()
		if changed {
			common.LogSuccess("[*] dirscan progress: %d/%d paths done, %d sites pending, %d findings", donePaths, total*len(dirPaths), pendingSites, findings)
		}
	}
}

// RegisterSite 注册站点进入目录扫描轮询环（WebScanWorker 成功返回后调用，
// err 路径不注册）。sealed 后与重复 URL 静默拒绝。首个站点注册时启动子系统。
func RegisterSite(url string, seedPresents map[string]struct{}) {
	if !common.DirScanEnable || url == "" {
		return
	}
	loadDirPaths()
	if dirPathsErr != nil {
		return
	}
	if seedPresents == nil {
		// nil 与"聚合器已释放"不可混淆：注册时统一兜底为空集合
		seedPresents = make(map[string]struct{})
	}
	s := &dirSite{
		url:     url,
		seeds:   seedPresents,
		seedFps: seedFingerprintUnion(seedPresents),
		seen:    make(map[string]struct{}),
	}
	dirSched.mu.Lock()
	if dirSched.sealed {
		dirSched.mu.Unlock()
		return
	}
	if _, ok := dirSched.reg[url]; ok {
		dirSched.mu.Unlock()
		return
	}
	dirSched.reg[url] = struct{}{}
	dirSched.sites = append(dirSched.sites, s)
	firstStart := false
	if !dirSched.started {
		dirSched.started = true
		firstStart = true
		client := dirHTTPClient()
		for i := 0; i < dirWorkerNum; i++ {
			dirSched.workers.Add(1)
			go dirWorker(client)
		}
		go dirSched.run()
		go dirSched.progressLoop()
	}
	nSites := len(dirSched.sites)
	dirSched.wakeup()
	dirSched.mu.Unlock()
	if firstStart {
		// 锁外发送：LogSuccess 可能阻塞在 Results 通道，持锁发送会卡住调度器
		common.LogSuccess("[*] dirscan enabled: %d sites registered, %d paths per site", nSites, len(dirPaths))
	}
}

func (d *dirScheduler) wakeup() {
	select {
	case d.wake <- struct{}{}:
	default:
	}
}

// run 调度主循环：轮询环找"在途==0 且路径未扫完"的站点发射任务；
// 全忙时阻塞等待；sealed 且现存站点全部排空后 close 任务通道收尾。
func (d *dirScheduler) run() {
	defer close(d.done)
	for {
		d.mu.Lock()
		if d.sealed && d.allDrainedLocked() {
			close(d.taskCh)
			d.mu.Unlock()
			return
		}
		var task dirTask
		var ok bool
		n := len(d.sites)
		for i := 0; i < n; i++ {
			d.rr = (d.rr + 1) % n
			s := d.sites[d.rr]
			if s.inflight == 0 && s.idx < len(dirPaths) {
				task = dirTask{site: s, path: dirPaths[s.idx]}
				s.idx++
				s.inflight++
				ok = true
				break
			}
		}
		d.mu.Unlock()
		if ok {
			d.taskCh <- task
			continue
		}
		<-d.wake
	}
}

func (d *dirScheduler) allDrainedLocked() bool {
	for _, s := range d.sites {
		if !s.drained() {
			return false
		}
	}
	return true
}

// SealAndDrain 收尾（各扫描模式末尾调用）：关闭注册，等调度器与全部任务排空，
// 最后输出统计。聚合行在 worker 的 complete 内先于此返回发出（LogWG 顺序契约）。
func SealAndDrain() {
	if !common.DirScanEnable {
		return
	}
	loadDirPaths()
	if dirPathsErr != nil {
		common.LogSuccess("[*] dirscan disabled: %v", dirPathsErr)
		return
	}
	dirSched.mu.Lock()
	dirSched.sealed = true
	started := dirSched.started
	dirSched.wakeup()
	dirSched.mu.Unlock()
	if !started {
		return // 从未有站点注册，无需排空
	}
	<-dirSched.done
	// done 只代表调度器退出（taskCh 已 close）；再等全部 worker 退出——
	// worker 循环体内的 complete（含锁外 LogSuccess）同步先于其返回，
	// Wait 返回即"聚合行全部发出"契约兑现，杜绝晚于 LogWG 归零的 send（reviewer 阻断项）
	dirSched.workers.Wait()
	dirSched.mu.Lock()
	hits := dirSched.totalFinds
	dirSched.mu.Unlock()
	common.LogSuccess("[*] dirscan done: %d sites x %d paths, %d findings", len(dirSched.reg), len(dirPaths), hits)
}

// complete 任务收尾（worker defer 调用，panic-safe）：在途归零、站点排空时
// flush 聚合行并释放聚合器（先锁内构造、后锁外发送，且先于本函数返回——
// 满足"聚合行先于收尾/LogWG 归零"契约）。
func (d *dirScheduler) complete(s *dirSite) {
	d.mu.Lock()
	s.inflight--
	var lines []string
	if s.drained() {
		d.totalFinds += len(s.hits)
		if len(s.hits) > 0 {
			lines = make([]string, 0, len(s.hits))
			for _, h := range s.hits {
				// 与 DoWebScan 输出行同构：[+] Product <url>\t<status>\t(<title>)\t<fps>
				lines = append(lines, fmt.Sprintf("[+] Product %s\t%d\t(%s)\t%s", h.url, h.status, h.title, h.fps))
			}
		}
		s.hits = nil // 聚合器释放：站点级生命周期结束（零命中站点同样释放）
		s.seeds = nil
		s.seen = nil
	}
	d.mu.Unlock()
	for _, l := range lines {
		common.LogSuccess(l)
	}
	d.wakeup()
}

// ---------- worker ----------

func dirWorker(client *http.Client) {
	defer dirSched.workers.Done()
	for task := range dirSched.taskCh {
		func() {
			// defer LIFO：先 complete 后 recover——panic 展开中在途计数仍递减（panic-safe），
			// 随后 recover 吞掉 panic 防止杀死 worker（项目 pool recover 惯例）
			defer func() {
				if r := recover(); r != nil {
					common.Printf("[-] dirscan task panic on %s%s: %v\n", task.site.url, task.path, r)
				}
			}()
			defer dirSched.complete(task.site)
			runDirTask(client, task)
		}()
	}
}

// admit 判定路径呈现能否作为该站点的有效命中（调用方持调度器锁；不含站点内去重）。
// 非空标题走 (指纹,标题) 二元组判定；空标题（"None" 占位）退化为纯指纹差——
// 两条路径分别对应用户 2026-11-14 的两项规则收紧。
func (s *dirSite) admit(fps []string, title string) bool {
	if title == "None" {
		for _, fp := range fps {
			if _, seen := s.seedFps[fp]; !seen {
				return true // 空标题但有任一新指纹
			}
		}
		return false // 空标题且指纹全为首页已见
	}
	_, dup := s.seeds[presentKey(fps, title)]
	return !dup // 与首页呈现完全重合则拒绝
}

// runDirTask 单个 (站点,路径) 探测：读原始响应 → 短路 → 指纹 → 呈现差集 → 命中入聚合器。
func runDirTask(client *http.Client, t dirTask) {
	url := strings.TrimSuffix(t.site.url, "/") + t.path
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return
	}
	req.Header.Set("User-agent", common.UserAgent)
	req.Header.Set("Accept", common.Accept)
	if common.Cookie != "" {
		req.Header.Set("Cookie", common.Cookie)
	}
	req.Header.Set("Connection", "close")
	resp, err := client.Do(req)
	if err != nil {
		dirSched.noteSiteFail(t.site) // 超时/连接失败计入熔断
		return
	}
	defer resp.Body.Close()
	dirSched.noteSiteOK(t.site) // 拿到响应即证明服务器存活，清零计数（含后续短路丢弃的 404 等）
	// 短路丢弃：3xx/4xx/5xx 一律不进指纹引擎（状态码 >= 300 全丢，仅 2xx 参与识别）。
	// 3xx：CheckRedirect 拒绝跟随时拿到的是 302 响应本身，其 Location/Server 头会被引擎
	// 匹配出无关产品（真机实测 Location 含 "thinkphp" 字样即误报）；限跳截断的落地 3xx
	// 同样无有效内容。4xx：403/404 噪声已实证，401 经取舍价值也低（仅极少数产品靠响应头
	// 特征可识别，2026-11-14 用户决策全 4xx 屏蔽）。跟随成功的落地响应状态码才进入判定。
	if resp.StatusCode >= 300 {
		return
	}
	body, err := ReadRawWithSize(resp, 192*1024)
	if len(body) == 0 && err != nil {
		dirSched.noteSiteFail(t.site) // body 黑洞（头回 body 超时）同样计入熔断
		return
	}
	frames, _ := Engine.DetectContent(body)
	products := strings.Split(frames.String(), "||")
	fps := make([]string, 0, len(products))
	for _, p := range products {
		if p = strings.TrimSpace(p); p != "" {
			fps = append(fps, p)
		}
	}
	if len(fps) == 0 {
		return // 无指纹丢弃：200 型软 404 免疫；401/403 无响应头特征亦无输出价值
	}
	title, _, _, _ := ExtractTitleAndICP(body)
	title = normalizeTitle(title)
	key := presentKey(fps, title)
	dirSched.mu.Lock()
	defer dirSched.mu.Unlock()
	if t.site.seeds == nil {
		return // 站点已 flush 释放（迟到任务）
	}
	if !t.site.admit(fps, title) {
		return // 首页呈现重合 / 空标题且无新指纹
	}
	if _, dup := t.site.seen[key]; dup {
		return // 站点内同呈现去重（url 保留首个命中）
	}
	t.site.seen[key] = struct{}{}
	t.site.hits = append(t.site.hits, dirHit{
		presentKey: key,
		url:        url,
		status:     resp.StatusCode,
		title:      title,
		fps:        strings.Join(fps, ", "),
	})
}

// ---------- HTTP client ----------

var (
	dirClientOnce sync.Once
	dirClientInst *http.Client
)

// dirHTTPClient 目录扫描专用 client：共享全局 Transport（代理/TLS 配置一致），
// 自带超时；CheckRedirect 实现"先 host 后 path 前缀"的同源同前缀跟随策略——
// 跳到不同 host（CDN/统一认证域）或非原路径子路径（302 型软 404 的站点级收口
// 跳转）时不跟随，返回最后响应（302 响应体通常无指纹，自然被判定丢弃）。
func dirHTTPClient() *http.Client {
	dirClientOnce.Do(func() {
		tr := http.DefaultTransport
		if lib.Client != nil && lib.Client.Transport != nil {
			tr = lib.Client.Transport
		}
		dirClientInst = &http.Client{
			Transport: tr,
			Timeout:   time.Duration(common.WebTimeout) * time.Second,
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
	})
	return dirClientInst
}
