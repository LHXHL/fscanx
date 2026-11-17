//go:build ignore

// verify_catalog_merge 一次性验证（BL-POC-009 对抗式复审 A 项）：出厂 catalog.yaml 合并后，
// 旧 aliases.yaml 的归一行为零回归。口径 = 旧表真实加载语义：按 yaml 条目顺序模拟 Compile
// （NormalizeString 归一键，后写覆盖——同键冲突条目文件序后者赢），对旧表每个 (引擎源, 写法)
// 比对新表 Map 的归一结果；删除项（gen_catalog.go staleAliases 白名单）放行。
// 运行：git show HEAD:mylib/finger/resources/aliases.yaml > plans/old-aliases.tmp.yaml
//
//	go run plans/scripts/gen_catalog.go
//	go run plans/scripts/verify_catalog_merge.go
package main

import (
	"fmt"
	"os"
	"strings"

	fingers "github.com/killmonday/fscanx/mylib/finger"
	"github.com/killmonday/fscanx/mylib/finger/resources"
	"gopkg.in/yaml.v3"
)

type oldEntry struct {
	Name  string              `yaml:"name"`
	Alias map[string][]string `yaml:"alias"`
}

// staleDeleted 已裁决删除的失效别名（gen_catalog.go staleAliases 展开白名单）。
var staleDeleted = map[string]bool{
	"goby|泛微-移动办公OA": true,
}

func main() {
	raw, err := os.ReadFile("plans/old-aliases.tmp.yaml")
	if err != nil {
		fmt.Fprintln(os.Stderr, "read old aliases:", err)
		os.Exit(1)
	}
	var old []oldEntry
	if err := yaml.Unmarshal(raw, &old); err != nil {
		fmt.Fprintln(os.Stderr, "parse old aliases:", err)
		os.Exit(1)
	}
	// 模拟旧表 Compile 语义：按条目顺序写 Map，后写覆盖（同归一键冲突时文件序后者赢）
	oldMap := map[string]map[string]string{} // engine → normKey → ToLower(aliasName)
	for _, e := range old {
		want := strings.ToLower(e.Name)
		for engine, names := range e.Alias {
			if oldMap[engine] == nil {
				oldMap[engine] = map[string]string{}
			}
			for _, n := range names {
				oldMap[engine][resources.NormalizeString(n)] = want
			}
		}
	}
	eng, err := fingers.NewEngine("fingers", "goby")
	if err != nil {
		fmt.Fprintln(os.Stderr, "new engine:", err)
		os.Exit(1)
	}
	total, bad := 0, 0
	for _, e := range old {
		for engine, names := range e.Alias {
			for _, n := range names {
				key := engine + "|" + n
				if staleDeleted[key] {
					continue // 预期删除（指向指纹库不存在的写法），白名单放行
				}
				total++
				want, wok := oldMap[engine][resources.NormalizeString(n)]
				got, gok := eng.Aliases.Map[engine][resources.NormalizeString(n)]
				if wok != gok || want != got {
					bad++
					fmt.Printf("[MISMATCH] %s/%s → old %q(ok=%v), new %q(ok=%v)\n", engine, n, want, wok, got, gok)
				}
			}
		}
	}
	fmt.Printf("旧表归一行为零回归核验（含同键冲突覆盖语义）：%d 条映射，%d 条不符\n", total, bad)
	if bad > 0 {
		os.Exit(1)
	}
}
