package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/killmonday/fscanx/PocScan"
)

// newTestCatalogDB 测试用 catalog.db + 快照路径（t.TempDir 自清理）。
func newTestCatalogDB(t *testing.T) (*sql.DB, string) {
	t.Helper()
	dir := t.TempDir()
	db, err := openCatalogDB(filepath.Join(dir, "catalog.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db, filepath.Join(dir, "catalog.yaml")
}

// TestCatalogCRUD 条目 CRUD round-trip：upsert 写入 → DB 与快照读回一致；外置行删除=真删。
func TestCatalogCRUD(t *testing.T) {
	db, yamlPath := newTestCatalogDB(t)
	e := PocScan.CatalogEntry{
		Name: "测试产品X", Vendor: "test", Product: "X",
		Alias: map[string][]string{"goby": {"Test-Product-X"}},
		Pocs:  []string{"poc-a", "poc-b"},
	}
	if err := catalogMutate(db, yamlPath, func(tx *sql.Tx) error { return dbUpsertEntry(tx, e) }); err != nil {
		t.Fatal(err)
	}
	got, err := dbListEntries(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "测试产品X" || len(got[0].Pocs) != 2 || got[0].Alias["goby"][0] != "Test-Product-X" {
		t.Fatalf("round-trip 不一致: %+v", got)
	}
	// 快照已生成且可解析回读（扫描器 ParseCatalogEntries 同口径）
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := PocScan.ParseCatalogEntries(data)
	if err != nil {
		t.Fatalf("快照解析失败: %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "测试产品X" || len(entries[0].Pocs) != 2 {
		t.Fatalf("快照内容不符: %+v", entries)
	}
	// 外置行删除 = 真删
	if err := catalogMutate(db, yamlPath, func(tx *sql.Tx) error {
		_, err := tx.Exec(`DELETE FROM entries WHERE name = ?`, "测试产品X")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if got, _ = dbListEntries(db); len(got) != 0 {
		t.Fatalf("删除后应空: %+v", got)
	}
}

// TestCatalogTombstone 墓碑三件套（BL-POC-005 同语义）：出厂条目删除 = 墓碑落库
// （pocs 空、别名保留——绑定失效而归一不破坏）；墓碑行判定正确；重新添加同名 = 覆写复活。
func TestCatalogTombstone(t *testing.T) {
	db, yamlPath := newTestCatalogDB(t)
	if _, ok := builtInEntry("泛微 e-cology"); !ok {
		t.Fatal("出厂册缺泛微 e-cology 条目")
	}
	// 出厂条目删除 = 落墓碑
	embedBase, _ := PocScan.FullCatalog()
	tomb, err := tombstoneOf(embedBase, "泛微 e-cology")
	if err != nil {
		t.Fatal(err)
	}
	if len(tomb.Pocs) != 0 || len(tomb.Alias) == 0 {
		t.Fatalf("墓碑形态不符（pocs 空别名保留）: %+v", tomb)
	}
	if err := catalogMutate(db, yamlPath, func(tx *sql.Tx) error { return dbUpsertEntry(tx, tomb) }); err != nil {
		t.Fatal(err)
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	tombRow, err := isTombstoneRow(tx, "泛微 e-cology")
	if err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if !tombRow {
		t.Fatal("出厂同名空 pocs 行应判为墓碑")
	}
	// 重新添加同名（完整条目）= 覆写复活
	revive, _ := builtInEntry("泛微 e-cology")
	if err := catalogMutate(db, yamlPath, func(tx *sql.Tx) error { return dbUpsertEntry(tx, revive) }); err != nil {
		t.Fatal(err)
	}
	tx, _ = db.Begin()
	tombRow, err = isTombstoneRow(tx, "泛微 e-cology")
	if err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	tx.Commit()
	if tombRow {
		t.Fatal("同名完整条目覆写后不应再判为墓碑（复活失败）")
	}
}

// TestCatalogAbsorb 手改吸收：手写 catalog.yaml → 哈希失配 → 全量吸收进 DB → 重生成统一格式。
func TestCatalogAbsorb(t *testing.T) {
	db, yamlPath := newTestCatalogDB(t)
	hand := []byte(`- name: 手改产品
  alias:
    goby:
      - Hand-Edited-X
  pocs:
    - poc-hand-1
`)
	if err := os.WriteFile(yamlPath, hand, 0644); err != nil {
		t.Fatal(err)
	}
	if err := absorbCatalogFile(db, yamlPath); err != nil {
		t.Fatal(err)
	}
	got, err := dbListEntries(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "手改产品" || len(got[0].Pocs) != 1 {
		t.Fatalf("手改吸收不符: %+v", got)
	}
	// 吸收后 DB 与文件对齐：再 absorb 应零动作（哈希一致）
	if err := absorbCatalogFile(db, yamlPath); err != nil {
		t.Fatalf("对齐后重复 absorb 应无错: %v", err)
	}
	// 坏 yaml 显式报错（手改坏了必须暴露）
	if err := os.WriteFile(yamlPath, []byte("[unclosed"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := absorbCatalogFile(db, yamlPath); err == nil {
		t.Fatal("坏 yaml 应报错")
	}
}

// TestMigrateLegacyBinds 存量迁移（BL-POC-009 验收）：binds.db（普通行+墓碑行）与
// binds.yaml 两个遗留数据源迁入 entries——普通行原名立条（行为不变）、墓碑行转出厂
// 别名保留 + 空 pocs；迁移后 binds.db 改名防重复。
func TestMigrateLegacyBinds(t *testing.T) {
	dir := t.TempDir()
	cdb, err := openCatalogDB(filepath.Join(dir, "catalog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer cdb.Close()
	catalogYaml := filepath.Join(dir, "catalog.yaml")

	// 遗留 binds.db：普通行（weaver 旧键）+ 墓碑行（出厂同名删除标记）
	legacyPath := filepath.Join(dir, "binds.db")
	ldb, err := openBindsDB(legacyPath)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := ldb.Begin()
	if err != nil {
		t.Fatal(err)
	}
	pj1 := `["poc-yaml-weaver-E-Cology-getSqlData-sqli"]`
	if _, err := tx.Exec(`INSERT INTO binds (product, pocs) VALUES (?, ?)`, "weaver", pj1); err != nil {
		t.Fatal(err)
	}
	pj2 := `[]`
	if _, err := tx.Exec(`INSERT INTO binds (product, pocs) VALUES (?, ?)`, "泛微 e-mobile", pj2); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	ldb.Close()

	// 遗留 binds.yaml：普通行
	bindsYaml := filepath.Join(dir, "binds.yaml")
	if err := os.WriteFile(bindsYaml, []byte("binds:\n  - product: legacy-yaml-product\n    pocs: [poc-y]"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := migrateLegacyBinds(cdb, catalogYaml, legacyPath, bindsYaml); err != nil {
		t.Fatal(err)
	}
	got, err := dbListEntries(cdb)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]PocScan.CatalogEntry{}
	for _, e := range got {
		byName[e.Name] = e
	}
	// 普通行：原名立条（识别输出原名 Contains 命中，行为不变），别名空=missing_alias 警示
	if e, ok := byName["weaver"]; !ok || len(e.Pocs) != 1 || len(e.Alias) != 0 {
		t.Fatalf("binds.db 普通行迁移不符: %+v", byName["weaver"])
	}
	// 墓碑行：出厂别名保留 + pocs 空（绑定失效语义保持）
	if e, ok := byName["泛微 e-mobile"]; !ok || len(e.Pocs) != 0 || len(e.Alias) == 0 {
		t.Fatalf("墓碑行迁移不符（出厂别名应保留）: %+v", byName["泛微 e-mobile"])
	}
	// binds.yaml 行迁入
	if e, ok := byName["legacy-yaml-product"]; !ok || len(e.Pocs) != 1 {
		t.Fatalf("binds.yaml 行迁移不符: %+v", byName["legacy-yaml-product"])
	}
	// 快照已生成；binds.db 已改名防重复迁移
	if _, err := os.Stat(catalogYaml); err != nil {
		t.Fatal("迁移后快照未生成")
	}
	if _, err := os.Stat(legacyPath + ".migrated"); err != nil {
		t.Fatal("binds.db 未改名 .migrated")
	}
	// 幂等：catalog.db 非空后再跑迁移零动作
	if err := migrateLegacyBinds(cdb, catalogYaml, legacyPath, bindsYaml); err != nil {
		t.Fatalf("二次迁移应幂等: %v", err)
	}
}
