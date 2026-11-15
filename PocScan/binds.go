package PocScan

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"
)

//go:embed binds-seed.yaml
var bindsSeed string

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

// SeedBinds 解析内置种子（导出供 fscanx-web 展示合并基线）。
func SeedBinds() ([]Bind, error) {
	return parseBinds([]byte(bindsSeed))
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
// 内置种子（binds-seed.yaml，原 fscan PocDatas 14 条产品的内置库快照，gen 脚本可重跑刷新）
// 打底，二进制旁 binds.yaml 同 product 整条覆盖、异名追加。种子兜底保证 PocBinds 恒非空
// ——"空 -pocname 智能定向"不会因用户未建 binds.yaml 而整体失效。
func loadBindsFile() {
	seed, err := parseBinds([]byte(bindsSeed))
	if err != nil {
		fmt.Printf("[-] load binds seed error: %v\n", err)
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
