package PocScan

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/killmonday/fscanx/mylib/finger/resources"
	"gopkg.in/yaml.v2"
)

// Bind 单条指纹→poc 绑定关系（BL-POC-002 数据结构）。
type Bind struct {
	Product string   `yaml:"product" json:"product"` // fingers 引擎输出产品名（goby/fingers_http/favicon 三源合并后的输出）
	Pocs    []string `yaml:"pocs" json:"pocs"`       // poc 的 yaml name 字段列表（无条件全打型已迁至 poc yaml 自身 always 标记）
}

// PocBinds 全局绑定关系：内置种子打底 + 外置 binds.yaml 覆盖合并（loadBindsFile）。
var PocBinds []Bind

type bindsFile struct {
	Bind []Bind `yaml:"binds"`
}

// parseBinds 解析 binds.yaml 内容。
func parseBinds(data []byte) ([]Bind, error) {
	var f bindsFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	return f.Bind, nil
}

// LoadBinds 解析 binds.yaml 文件；文件不存在返回 nil,nil（正常态：尚未建立任何外置绑定）。
func LoadBinds(path string) ([]Bind, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return parseBinds(data)
}

// LoadBindsFromBytes 解析 binds yaml 字节（fscanx-web 落盘前 round-trip 校验用）。
func LoadBindsFromBytes(data []byte) ([]Bind, error) {
	return parseBinds(data)
}

// CatalogEntry 外置产品册条目（BL-POC-009）：与出厂 catalog.yaml 同构——标准名 + 别名
// （各指纹源写法归一到标准名）+ 测试名单。alias 字段注入识别引擎归一表，pocs 段并入
// 绑定合并。
type CatalogEntry struct {
	Name    string              `yaml:"name"`
	Vendor  string              `yaml:"vendor,omitempty"`
	Product string              `yaml:"product,omitempty"`
	Alias   map[string][]string `yaml:"alias,omitempty"`
	Pocs    []string            `yaml:"pocs,omitempty"`
}

// ParseCatalogEntries 解析产品册 yaml 字节（顶层列表；导出供 Plugins 外置注入复用）。
func ParseCatalogEntries(data []byte) ([]CatalogEntry, error) {
	var entries []CatalogEntry
	if err := yaml.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// externalCatalog 二进制旁外置产品册缓存（Plugins.InitEngineAliases 在 main 早期填充，
// loadBindsFile 消费 pocs 段）。nil = 无外置产品册（loadBindsFile 退回旧 binds.yaml 兼容）。
var externalCatalog []CatalogEntry

// SetExternalCatalog 设置外置产品册缓存（导出供 Plugins 注入）。
func SetExternalCatalog(entries []CatalogEntry) {
	externalCatalog = entries
}

// ExternalCatalogSnapshot 读取外置产品册缓存快照（测试与诊断用，导出访问器）。
func ExternalCatalogSnapshot() []CatalogEntry {
	return externalCatalog
}

// FullCatalog 解析出厂产品册全量条目（含无 pocs 的纯别名条目）——fscanx-web 产品册视图、
// 指纹归属判定与 built-in 写回的目标基线（BL-POC-009）。
func FullCatalog() ([]CatalogEntry, error) {
	return ParseCatalogEntries(resources.CatalogData)
}

// SeedBinds 解析内置绑定兜底名单（导出供 fscanx-web 展示合并基线）。
func SeedBinds() ([]Bind, error) {
	return catalogSeed()
}

// catalogSeed 从出厂产品册（mylib/finger/resources/catalog.yaml，BL-POC-009 合并别名表
// 与预置名单的单一出厂文件）提取绑定兜底名单：按 name+pocs 投影独立解析——alias 字段
// 由宽松解析忽略（与 alias 包对 pocs 字段的忽略互为镜像，识别引擎解析零改动）。
func catalogSeed() ([]Bind, error) {
	var entries []struct {
		Name string   `yaml:"name"`
		Pocs []string `yaml:"pocs"`
	}
	if err := yaml.Unmarshal(resources.CatalogData, &entries); err != nil {
		return nil, err
	}
	var binds []Bind
	for _, e := range entries {
		if len(e.Pocs) == 0 {
			continue
		}
		binds = append(binds, Bind{Product: e.Name, Pocs: e.Pocs})
	}
	return binds, nil
}

// MergeBinds 两层合并（纯函数，可单测）：seed 打底，outer 同 product 整条覆盖、异名追加。
func MergeBinds(seed, outer []Bind) []Bind {
	merged := make([]Bind, len(seed))
	copy(merged, seed)
	byProduct := make(map[string]int, len(merged))
	for i, b := range merged {
		byProduct[b.Product] = i
	}
	for _, ob := range outer {
		if i, ok := byProduct[ob.Product]; ok {
			merged[i] = ob
		} else {
			byProduct[ob.Product] = len(merged)
			merged = append(merged, ob)
		}
	}
	return merged
}

// loadBindsFile 加载绑定关系到 PocBinds（BL-POC-003 两层合并，与 poc 加载同模式）：
// 出厂产品册（catalog.yaml 的 pocs 段，BL-POC-009 起为合并后的单一出厂文件）
// 打底，二进制旁 binds.yaml 同 product 整条覆盖、异名追加。出厂兜底保证 PocBinds 恒非空
// ——"空 -pocname 智能定向"不会因用户未建 binds.yaml 而整体失效。
func loadBindsFile() {
	seed, err := catalogSeed()
	if err != nil {
		fmt.Printf("[-] load catalog binds error: %v\n", err)
		return
	}
	if externalCatalog != nil {
		// BL-POC-009：外置产品册（二进制旁 catalog.yaml）的 pocs 段并入绑定合并
		// （MergeBinds 同 name 整条覆盖、异名追加）；alias 段已在 main 早期经
		// Plugins.InitEngineAliases 注入归一表（指纹识别前的硬时序）。
		var asBinds []Bind
		for _, e := range externalCatalog {
			if e.Name == "" {
				continue
			}
			asBinds = append(asBinds, Bind{Product: e.Name, Pocs: e.Pocs})
		}
		PocBinds = MergeBinds(seed, asBinds)
		return
	}
	exe, err := os.Executable()
	if err != nil {
		PocBinds = seed
		return
	}
	path := filepath.Join(filepath.Dir(exe), "binds.yaml")
	outer, err := LoadBinds(path)
	if err != nil {
		fmt.Printf("[-] load binds error: %v\n", err)
		PocBinds = seed
		return
	}
	if outer != nil {
		PocBinds = MergeBinds(seed, outer)
		fmt.Printf("[+] load %d binds from %s\n", len(outer), path)
		return
	}
	PocBinds = seed
}

// boundPocNames 按指纹产品名查绑定关系，返回定向执行的 poc 名集合。
// 无任何命中返回空——调用方不打且不提示（BL-POC-003 终态语义，-pocname all 是唯一全量入口；
// 无条件全打型在 poc yaml 自身 always 标记，由 alwaysPocNames 单独收集）。
func boundPocNames(frames []string) (names []string) {
	seen := make(map[string]bool)
	add := func(list []string) {
		for _, p := range list {
			if p != "" && !seen[p] {
				seen[p] = true
				names = append(names, p)
			}
		}
	}
	for _, b := range PocBinds {
		if b.Product == "" {
			continue
		}
		product := strings.ToLower(b.Product)
		for _, f := range frames {
			if strings.Contains(strings.ToLower(f), product) {
				add(b.Pocs)
				break
			}
		}
	}
	return
}
