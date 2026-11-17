package PocScan

import (
	"embed"
	"fmt"
	"github.com/killmonday/fscanx/PocScan/lib"
	"github.com/killmonday/fscanx/common"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

//go:embed pocs
var Pocs embed.FS
var once sync.Once
var AllPocs []*lib.Poc

func PocScan(info *common.HostInfo) {
	once.Do(initpoc)
	var pocinfo = common.Pocinfo
	buf := strings.Split(info.Url, "/")
	pocinfo.Target = strings.Join(buf[:3], "/")

	// BL-POC-003 终态三态：all=全量（filterPoc("") 返回全部）；显式名=过滤（大小写不敏感）；
	// 空=指纹智能定向——指纹产品名查 binds 定向，并上 poc yaml 自身 always: true 标记
	// （无条件全打型，不依赖指纹），无命中不打且不提示（all 是唯一全量入口）。
	if strings.EqualFold(pocinfo.PocName, "all") {
		pocinfo.PocName = ""
		Execute(pocinfo)
		return
	}
	if pocinfo.PocName != "" {
		Execute(pocinfo)
		return
	}
	names := boundPocNames(info.Frames)
	seen := make(map[string]bool, len(names))
	for _, n := range names {
		seen[n] = true
	}
	for _, n := range alwaysPocNames() {
		if !seen[n] {
			seen[n] = true
			names = append(names, n)
		}
	}
	// 绑定名单路径按名精确索引取 poc（BL-POC-009）：修 filterPoc Contains 子串扩张的
	// 重复执行（名字互为前缀的两个 poc 会被同带出来）；AllPocs 经 pocCollector 同名
	// 去重（外置覆盖内置），name 唯一，map 索引安全。-pocname/all 路径的 filterPoc
	// Contains 模糊语义保持不变（BL-POC-001 既有承诺）。
	pocIndex := make(map[string]*lib.Poc, len(AllPocs))
	for _, p := range AllPocs {
		pocIndex[p.Name] = p
	}
	for _, name := range names {
		p, ok := pocIndex[name]
		if !ok {
			continue // 绑定名单引用了不存在的 poc 名（外置 binds 手写笔误），静默跳过
		}
		req := buildPocRequest(pocinfo.Target)
		if req == nil {
			return
		}
		lib.CheckMultiPoc(req, []*lib.Poc{p}, common.PocNum)
	}
}

// alwaysPocNames 返回 yaml 标记 always: true 的 poc 名单（无条件全打型，如 .git 目录探测）。
func alwaysPocNames() (names []string) {
	for _, p := range AllPocs {
		if p.Always {
			names = append(names, p.Name)
		}
	}
	return
}

func Execute(PocInfo common.PocInfo) {
	req := buildPocRequest(PocInfo.Target)
	if req == nil {
		return
	}
	pocs := filterPoc(PocInfo.PocName)
	lib.CheckMultiPoc(req, pocs, common.PocNum)
}

// buildPocRequest 构造 POC 基准请求（Target 已由 PocScan 入口剥 path 到 scheme://host，
// rule.Path 直接作为全路径拼接）。头设置原样自 Execute 提取（BL-POC-009 绑定名单精确
// 索引路径共用）。
func buildPocRequest(target string) *http.Request {
	req, err := http.NewRequest("GET", target, nil)
	if err != nil {
		common.LogError(fmt.Sprintf("[-] webpocinit %v %v", target, err))
		return nil
	}
	req.Header.Set("User-agent", common.UserAgent)
	req.Header.Set("Accept", common.Accept)
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	if common.Cookie != "" {
		req.Header.Set("Cookie", common.Cookie)
	}
	return req
}

// initpoc 三层合并加载 poc（BL-POC-002）：内置 embed → 二进制目录 pocs/ → -pocpath 追加。
// 同名 poc 后加载覆盖先加载（外置修正内置），替换保位保证顺序稳定；绑定关系一并加载。
func initpoc() {
	c := newPocCollector()
	if loadEmbeddedPocs(c) == 0 {
		fmt.Println("[-] init poc error: no embedded poc loaded")
	}
	if dir := exePocsDir(); dir != "" {
		// 约定目录不存在是正常态（多数用户无外置 poc），静默跳过不报错；
		// 显式 -pocpath 的错误仍由 loadPocsFromDir 打印（打错路径需要可见）。
		if fi, statErr := os.Stat(dir); statErr == nil && fi.IsDir() {
			if n := loadPocsFromDir(dir, c); n > 0 {
				fmt.Printf("[+] load %d pocs from %s\n", n, dir)
			}
		}
	}
	if common.PocPath != "" {
		fmt.Println("[+] load poc from " + common.PocPath)
		if n := loadPocsFromDir(common.PocPath, c); n == 0 {
			fmt.Printf("[-] no poc loaded from %s\n", common.PocPath)
		}
	}
	AllPocs = c.list()
	loadBindsFile()
}

// pocCollector 按 poc 名去重的有序收集器；add 同名替换保位（先内置后外置，外置覆盖内置版本）。
type pocCollector struct {
	pocs []*lib.Poc
	idx  map[string]int
}

func newPocCollector() *pocCollector {
	return &pocCollector{idx: make(map[string]int)}
}

func (c *pocCollector) add(p *lib.Poc) {
	if p == nil || p.Name == "" {
		return
	}
	if i, ok := c.idx[p.Name]; ok {
		c.pocs[i] = p
		return
	}
	c.idx[p.Name] = len(c.pocs)
	c.pocs = append(c.pocs, p)
}

func (c *pocCollector) list() []*lib.Poc { return c.pocs }

// loadEmbeddedPocs 加载编译期内置 poc 库，返回加载数量。
func loadEmbeddedPocs(c *pocCollector) (count int) {
	entries, err := Pocs.ReadDir("pocs")
	if err != nil {
		fmt.Printf("[-] init poc error: %v\n", err)
		return 0
	}
	for _, one := range entries {
		path := one.Name()
		if strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml") {
			if poc, _ := lib.LoadPoc(path, Pocs); poc != nil {
				c.add(poc)
				count++
			}
		}
	}
	return
}

// exePocsDir 返回二进制所在目录下的 pocs/（约定目录，存在与否由调用方判断）；推导失败返回空串。
// 用二进制目录而非工作目录：内网工具目录习惯（exe 与 pocs/ 同放），cd 到别处跑不影响加载。
func exePocsDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Join(filepath.Dir(exe), "pocs")
}

// loadPocsFromDir 递归加载目录下全部 .yml/.yaml，返回加载数量。
func loadPocsFromDir(dir string, c *pocCollector) (count int) {
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return err
		}
		if !info.IsDir() && (strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml")) {
			if poc, _ := lib.LoadPocbyPath(path); poc != nil {
				c.add(poc)
				count++
			}
		}
		return nil
	})
	if err != nil {
		fmt.Printf("[-] init poc error: %v\n", err)
	}
	return
}

func filterPoc(pocname string) (pocs []*lib.Poc) {
	if pocname == "" {
		return AllPocs
	}
	pocname = strings.ToLower(pocname) // BL-POC-001：大小写不敏感过滤
	for _, poc := range AllPocs {
		if strings.Contains(strings.ToLower(poc.Name), pocname) {
			pocs = append(pocs, poc)
		}
	}
	return
}
