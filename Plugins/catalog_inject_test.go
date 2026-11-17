package Plugins

import (
	"testing"

	"github.com/killmonday/fscanx/mylib/finger/resources"
	"github.com/killmonday/fscanx/PocScan"
)

// TestInitEngineAliasesFrom 外置产品册注入（BL-POC-009）：alias 段进引擎归一表
// （外置写法 → 标准名改口），条目转交 PocScan 外置缓存（pocs 段由 loadBindsFile 合并）。
func TestInitEngineAliasesFrom(t *testing.T) {
	origExternal := PocScan.ExternalCatalogSnapshot()
	t.Cleanup(func() { PocScan.SetExternalCatalog(origExternal) })

	data := []byte(`- name: 泛微 e-cology
  vendor: weaver
  product: e-cology
  alias:
    goby:
      - 泛微-协同办公OA
      - 泛微-测试写法XYZ
  pocs:
    - poc-yaml-demo
- name: 自建产品
  alias:
    fingers:
      - 某指纹原始名ABC
`)
	if err := initEngineAliasesFrom(data); err != nil {
		t.Fatal(err)
	}

	// alias 段已注入归一表：外置新增写法可归一到标准名（查找键与引擎同源 NormalizeString）
	if aliasName, ok := Engine.Aliases.Map["goby"][resources.NormalizeString("泛微-测试写法XYZ")]; !ok || aliasName != "泛微 e-cology" {
		t.Fatalf("外置别名未注入归一表: ok=%v aliasName=%q", ok, aliasName)
	}

	// 整条转交 PocScan 外置缓存
	snap := PocScan.ExternalCatalogSnapshot()
	if len(snap) != 2 {
		t.Fatalf("外置缓存条目数: got %d want 2", len(snap))
	}
	if snap[0].Name != "泛微 e-cology" || len(snap[0].Pocs) != 1 {
		t.Fatalf("外置缓存条目内容不符: %+v", snap[0])
	}

	// 坏 yaml 显式报错
	if err := initEngineAliasesFrom([]byte("[unclosed")); err == nil {
		t.Fatal("坏 yaml 应报错")
	}
}
