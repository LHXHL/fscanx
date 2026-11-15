//go:build ignore

// gen_binds_seed 生成 PocScan/binds-seed.yaml 内置种子（BL-POC-003）。
// 数据源：fscan 原版 PocDatas 硬编码表（rules.go，本脚本硬编码其 14 条 alias 快照）
// + 内置 poc 库（PocScan/pocs/*.yml 的 name 字段）按"名字含 alias"分组。
// 维护口径：内置库新增 poc 后重跑 `go run plans/scripts/gen_binds_seed.go` 刷新种子。
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var aliases = []string{
	"seeyon", "weaver", "tongda", "landray", "thinkphp", "nexus", "qizhi",
	"weaver-ebridge", "weblogic", "zabbix", "vmware", "jboss", "yongyou",
	"coremail", "kingsoft",
}

func main() {
	dir := "PocScan/pocs"
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read pocs dir:", err)
		os.Exit(1)
	}
	re := regexp.MustCompile(`(?m)^name:\s*(\S+)`)
	byAlias := map[string][]string{}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".yaml") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		m := re.FindSubmatch(raw)
		if m == nil {
			continue
		}
		pocName := string(m[1])
		for _, alias := range aliases {
			if strings.Contains(strings.ToLower(pocName), alias) {
				byAlias[alias] = append(byAlias[alias], pocName)
			}
		}
	}

	var b strings.Builder
	b.WriteString("# BL-POC-003 内置绑定种子：由 plans/scripts/gen_binds_seed.go 从内置 poc 库生成快照。\n")
	b.WriteString("# product 为 fingers 引擎输出产品名形态；外置 binds.yaml 同 product 覆盖、异名追加。\n")
	b.WriteString("binds:\n")
	for _, alias := range aliases {
		pocs := byAlias[alias]
		if len(pocs) == 0 {
			continue
		}
		sort.Strings(pocs)
		fmt.Fprintf(&b, "  - product: %s\n    pocs:\n", alias)
		for _, p := range pocs {
			fmt.Fprintf(&b, "      - %s\n", p)
		}
	}
	if err := os.WriteFile("PocScan/binds-seed.yaml", []byte(b.String()), 0644); err != nil {
		fmt.Fprintln(os.Stderr, "write seed:", err)
		os.Exit(1)
	}
	total := 0
	for _, v := range byAlias {
		total += len(v)
	}
	fmt.Printf("binds-seed.yaml written: %d products, %d bind entries\n", len(byAlias), total)
}
