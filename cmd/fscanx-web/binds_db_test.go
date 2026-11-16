package main

// binds 存储层单测（BL-POC-005）：CRUD + 文件同步（flush/吸收/缺失重生成/坏文件报错）。

import (
	"database/sql"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/killmonday/fscanx/PocScan"
)

// newBindsTestEnv 独立临时目录的 DB + yaml 路径。
func newBindsTestEnv(t *testing.T) (*sql.DB, string) {
	t.Helper()
	dir := t.TempDir()
	db, err := openBindsDB(filepath.Join(dir, "binds.db"))
	if err != nil {
		t.Fatalf("openBindsDB: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db, filepath.Join(dir, "binds.yaml")
}

func mustListProducts(t *testing.T, db *sql.DB) []string {
	t.Helper()
	binds, err := dbListBinds(db)
	if err != nil {
		t.Fatalf("dbListBinds: %v", err)
	}
	out := make([]string, 0, len(binds))
	for _, b := range binds {
		out = append(out, b.Product)
	}
	sort.Strings(out)
	return out
}

func mustListPocs(t *testing.T, db *sql.DB, product string) []string {
	t.Helper()
	binds, err := dbListBinds(db)
	if err != nil {
		t.Fatalf("dbListBinds: %v", err)
	}
	for _, b := range binds {
		if b.Product == product {
			return b.Pocs
		}
	}
	return nil
}

func TestOpenBindsDB_FreshEmpty(t *testing.T) {
	db, _ := newBindsTestEnv(t)
	if got := mustListProducts(t, db); len(got) != 0 {
		t.Fatalf("新库应无外置条目，got %v", got)
	}
}

func TestAddFlushFileContent(t *testing.T) {
	db, yp := newBindsTestEnv(t)
	if err := bindsAdd(db, yp, "WordPress", []string{"poc-a", "poc-b"}); err != nil {
		t.Fatalf("bindsAdd: %v", err)
	}
	// 文件落盘且能被扫描器同 schema 解析回来
	outer, err := PocScan.LoadBinds(yp)
	if err != nil {
		t.Fatalf("LoadBinds: %v", err)
	}
	if len(outer) != 1 || outer[0].Product != "WordPress" || len(outer[0].Pocs) != 2 {
		t.Fatalf("文件内容不符: %+v", outer)
	}
	// meta 哈希已记录（未被手改时 absorb 应为 no-op）
	data, _ := os.ReadFile(yp)
	if getMeta(db, metaKeyFileHash) != fileHash(data) {
		t.Fatal("flush 后 meta 哈希应与文件一致")
	}
}

func TestAddUpsertAppendDedup(t *testing.T) {
	db, yp := newBindsTestEnv(t)
	if err := bindsAdd(db, yp, "A", []string{"p1", "p2"}); err != nil {
		t.Fatalf("bindsAdd1: %v", err)
	}
	if err := bindsAdd(db, yp, "A", []string{"p2", "p3", ""}); err != nil {
		t.Fatalf("bindsAdd2: %v", err)
	}
	got := mustListPocs(t, db, "A")
	if len(got) != 3 || got[0] != "p1" || got[1] != "p2" || got[2] != "p3" {
		t.Fatalf("追加去重不符: %v", got)
	}
	if got := mustListProducts(t, db); len(got) != 1 {
		t.Fatalf("同名应只一条: %v", got)
	}
}

func TestEditRenameAndConflict(t *testing.T) {
	db, yp := newBindsTestEnv(t)
	if err := bindsAdd(db, yp, "A", []string{"p1"}); err != nil {
		t.Fatalf("bindsAdd A: %v", err)
	}
	if err := bindsAdd(db, yp, "B", []string{"p2"}); err != nil {
		t.Fatalf("bindsAdd B: %v", err)
	}
	// 改名到已存在的 B → 报错
	if err := bindsEdit(db, yp, "A", "B", []string{"p1"}); err == nil {
		t.Fatal("改名到已存在产品应报错")
	}
	// 改名到新名 + 更新 pocs
	if err := bindsEdit(db, yp, "A", "A2", []string{"p9"}); err != nil {
		t.Fatalf("bindsEdit 改名: %v", err)
	}
	prods := mustListProducts(t, db)
	if len(prods) != 2 || prods[0] != "A2" || prods[1] != "B" {
		t.Fatalf("改名结果不符: %v", prods)
	}
	if pocs := mustListPocs(t, db, "A2"); len(pocs) != 1 || pocs[0] != "p9" {
		t.Fatalf("编辑后 pocs 不符: %v", pocs)
	}
}

func TestDeleteAndMissing(t *testing.T) {
	db, yp := newBindsTestEnv(t)
	_ = bindsAdd(db, yp, "A", []string{"p1"})
	if err := bindsDelete(db, yp, "A"); err != nil {
		t.Fatalf("bindsDelete: %v", err)
	}
	if got := mustListProducts(t, db); len(got) != 0 {
		t.Fatalf("删除后应为空: %v", got)
	}
	if err := bindsDelete(db, yp, "A"); err == nil {
		t.Fatal("删除不存在的条目应报错")
	}
}

func TestAbsorbHandEdit(t *testing.T) {
	db, yp := newBindsTestEnv(t)
	_ = bindsAdd(db, yp, "A", []string{"p1"})
	_ = bindsAdd(db, yp, "B", []string{"p2"})
	// 手改：改 A 的 pocs、加 C、删 B
	hand := "binds:\n  - product: A\n    pocs: [p1-changed]\n  - product: C\n    pocs: [p3]\n"
	if err := os.WriteFile(yp, []byte(hand), 0644); err != nil {
		t.Fatalf("手写文件: %v", err)
	}
	if err := absorbBindsFile(db, yp); err != nil {
		t.Fatalf("absorb: %v", err)
	}
	prods := mustListProducts(t, db)
	if len(prods) != 2 || prods[0] != "A" || prods[1] != "C" {
		t.Fatalf("吸收后产品集不符: %v", prods)
	}
	if pocs := mustListPocs(t, db, "A"); len(pocs) != 1 || pocs[0] != "p1-changed" {
		t.Fatalf("吸收后 A 的 pocs 不符: %v", pocs)
	}
	// 吸收后再次 absorb 应为 no-op（哈希已对齐）
	if err := absorbBindsFile(db, yp); err != nil {
		t.Fatalf("二次 absorb: %v", err)
	}
	if got := mustListProducts(t, db); len(got) != 2 {
		t.Fatalf("no-op absorb 不应改数据: %v", got)
	}
}

func TestAbsorbMissingFileRegenerate(t *testing.T) {
	db, yp := newBindsTestEnv(t)
	_ = bindsAdd(db, yp, "A", []string{"p1"})
	if err := os.Remove(yp); err != nil {
		t.Fatalf("删除文件: %v", err)
	}
	// 文件缺失 + DB 有数据 → 按 DB 重生成，不视为清空
	if err := absorbBindsFile(db, yp); err != nil {
		t.Fatalf("absorb: %v", err)
	}
	outer, err := PocScan.LoadBinds(yp)
	if err != nil {
		t.Fatalf("重生成后文件应可解析: %v", err)
	}
	if len(outer) != 1 || outer[0].Product != "A" {
		t.Fatalf("重生成内容不符: %+v", outer)
	}
}

func TestAbsorbBadYamlError(t *testing.T) {
	db, yp := newBindsTestEnv(t)
	_ = bindsAdd(db, yp, "A", []string{"p1"})
	if err := os.WriteFile(yp, []byte(":::not yaml{{{"), 0644); err != nil {
		t.Fatalf("写坏文件: %v", err)
	}
	if err := absorbBindsFile(db, yp); err == nil {
		t.Fatal("坏 yaml 应显式报错而非静默吞掉")
	}
	// 报错后 DB 数据不变
	if got := mustListProducts(t, db); len(got) != 1 || got[0] != "A" {
		t.Fatalf("报错后 DB 不应变: %v", got)
	}
}

func TestAbsorbEmptyFileClears(t *testing.T) {
	db, yp := newBindsTestEnv(t)
	_ = bindsAdd(db, yp, "A", []string{"p1"})
	// 手工清空文件（显式编辑行为）→ 吸收后 DB 外置层清空
	if err := os.WriteFile(yp, []byte(""), 0644); err != nil {
		t.Fatalf("写空文件: %v", err)
	}
	if err := absorbBindsFile(db, yp); err != nil {
		t.Fatalf("absorb: %v", err)
	}
	if got := mustListProducts(t, db); len(got) != 0 {
		t.Fatalf("手工清空应吸收为空: %v", got)
	}
}

func TestAppendDedup(t *testing.T) {
	got := appendDedup([]string{"a", "b", ""}, []string{"b", "c", ""})
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Fatalf("appendDedup 不符: %v", got)
	}
}

// 回归：delete 请求只带 product（无 pocs）必须成功——曾经复用写校验把 delete 拦成 400。
func TestBindsDeleteHandler_NoPocsRequired(t *testing.T) {
	db, yp := newBindsTestEnv(t)
	_ = bindsAdd(db, yp, "A", []string{"p1"})
	s := &server{db: db, bindsPath: yp}
	req := httptest.NewRequest("POST", "/api/binds_delete", strings.NewReader(`{"product":"A"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.bindsDelete(rec, req)
	if rec.Code != 200 {
		t.Fatalf("delete 应 200，got %d: %s", rec.Code, rec.Body.String())
	}
	if got := mustListProducts(t, db); len(got) != 0 {
		t.Fatalf("删除后应无条目: %v", got)
	}
}

// ---- 种子起源行语义（墓碑机制）：编辑/删除合并视图中的种子条目必须真实生效 ----

// seedName 取内置种子第一个产品名（动态适配种子内容）。
func seedName(t *testing.T) string {
	t.Helper()
	seed, err := PocScan.SeedBinds()
	if err != nil || len(seed) == 0 {
		t.Skip("内置种子不可用")
	}
	return seed[0].Product
}

// mergedShown 合并视图（种子 ⊕ 外置，过滤墓碑）的产品→pocs 映射。
func mergedShown(t *testing.T, db *sql.DB) map[string][]string {
	t.Helper()
	seed, err := PocScan.SeedBinds()
	if err != nil {
		t.Fatalf("SeedBinds: %v", err)
	}
	outer, err := dbListBinds(db)
	if err != nil {
		t.Fatalf("dbListBinds: %v", err)
	}
	merged := PocScan.MergeBinds(seed, outer)
	out := make(map[string][]string)
	for _, b := range merged {
		if len(b.Pocs) > 0 {
			out[b.Product] = b.Pocs
		}
	}
	return out
}

func TestEditSeedOriginRow_OverrideSameName(t *testing.T) {
	db, yp := newBindsTestEnv(t)
	name := seedName(t)
	if ok, _ := isOuterProduct(db, name); ok {
		t.Fatalf("%s 不应在外置层", name)
	}
	if err := bindsEdit(db, yp, name, name, []string{"poc-overridden"}); err != nil {
		t.Fatalf("bindsEdit 种子行: %v", err)
	}
	view := mergedShown(t, db)
	pocs := view[name]
	if len(pocs) != 1 || pocs[0] != "poc-overridden" {
		t.Fatalf("种子行编辑后合并视图应为覆写 pocs: %v", pocs)
	}
}

func TestEditSeedOriginRow_RenameTombstones(t *testing.T) {
	db, yp := newBindsTestEnv(t)
	name := seedName(t)
	if err := bindsEdit(db, yp, name, name+"-x", []string{"poc-new"}); err != nil {
		t.Fatalf("bindsEdit 改名: %v", err)
	}
	view := mergedShown(t, db)
	if _, ok := view[name]; ok {
		t.Fatal("改名后原种子名应被墓碑隐藏")
	}
	if pocs := view[name+"-x"]; len(pocs) != 1 || pocs[0] != "poc-new" {
		t.Fatalf("新名条目应出现: %v", pocs)
	}
}

func TestDeleteSeedOriginRow_TombstoneHides(t *testing.T) {
	db, yp := newBindsTestEnv(t)
	name := seedName(t)
	if err := bindsDelete(db, yp, name); err != nil {
		t.Fatalf("种子行删除不应报错: %v", err)
	}
	view := mergedShown(t, db)
	if _, ok := view[name]; ok {
		t.Fatal("删除后种子条目应从合并视图隐藏")
	}
	// tombstone 落盘文件中 pocs 为空数组（引擎消费语义：同名覆盖为空=失效）
	outer, err := PocScan.LoadBinds(yp)
	if err != nil {
		t.Fatalf("LoadBinds: %v", err)
	}
	found := false
	for _, b := range outer {
		if b.Product == name {
			found = true
			if len(b.Pocs) != 0 {
				t.Fatalf("墓碑应为空 pocs: %+v", b)
			}
		}
	}
	if !found {
		t.Fatal("墓碑应落盘 binds.yaml")
	}
}

func TestAddOnTombstonedProductRevives(t *testing.T) {
	db, yp := newBindsTestEnv(t)
	name := seedName(t)
	_ = bindsDelete(db, yp, name)
	if err := bindsAdd(db, yp, name, []string{"poc-revive"}); err != nil {
		t.Fatalf("bindsAdd: %v", err)
	}
	view := mergedShown(t, db)
	pocs := view[name]
	if len(pocs) != 1 || pocs[0] != "poc-revive" {
		t.Fatalf("墓碑产品重新添加应复活: %v", pocs)
	}
}

// 端到端：bindsList handler 对种子行删除后不再返回该条目。
func TestBindsListHandler_HidesTombstonedSeedRow(t *testing.T) {
	db, yp := newBindsTestEnv(t)
	name := seedName(t)
	_ = bindsDelete(db, yp, name)
	s := &server{db: db, bindsPath: yp}
	req := httptest.NewRequest("GET", "/api/binds_list", nil)
	rec := httptest.NewRecorder()
	s.bindsList(rec, req)
	if rec.Code != 200 {
		t.Fatalf("list 应 200: %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), `"`+name+`"`) {
		t.Fatalf("合并视图不应含墓碑条目 %s: %s", name, rec.Body.String())
	}
}

// 回归（reviewer HIGH）：重复删除墓碑行必须幂等成功且保留墓碑——
// 若墓碑被删，种子绑定会复活（MergeBinds 回落种子行）。
func TestDeleteTombstoneIdempotent(t *testing.T) {
	db, yp := newBindsTestEnv(t)
	name := seedName(t)
	if err := bindsDelete(db, yp, name); err != nil {
		t.Fatalf("首次删除: %v", err)
	}
	if err := bindsDelete(db, yp, name); err != nil {
		t.Fatalf("重复删除应幂等成功: %v", err)
	}
	if pocs := mustListPocs(t, db, name); len(pocs) != 0 {
		t.Fatalf("墓碑必须保留: %v", pocs)
	}
	if _, ok := mergedShown(t, db)[name]; ok {
		t.Fatal("墓碑行重复删除后仍应隐藏")
	}
}

// 回归（reviewer HIGH 同族）：墓碑行作 original 改名时墓碑必须保留（否则种子复活），
// 新名条目正常生效。
func TestEditTombstoneOriginRenameKeepsTombstone(t *testing.T) {
	db, yp := newBindsTestEnv(t)
	name := seedName(t)
	_ = bindsDelete(db, yp, name)
	if err := bindsEdit(db, yp, name, name+"-x", []string{"poc-after-tombstone"}); err != nil {
		t.Fatalf("墓碑行改名: %v", err)
	}
	if pocs := mustListPocs(t, db, name); len(pocs) != 0 {
		t.Fatalf("墓碑必须保留: %v", pocs)
	}
	view := mergedShown(t, db)
	if _, ok := view[name]; ok {
		t.Fatal("种子行应保持隐藏")
	}
	if pocs := view[name+"-x"]; len(pocs) != 1 || pocs[0] != "poc-after-tombstone" {
		t.Fatalf("新名条目应生效: %v", pocs)
	}
}

// 回归（reviewer MEDIUM）：快捷绑定到种子起源产品应以种子 pocs 为基线追加，
// 而非单 poc 条目整条覆盖种子绑定。
func TestAddSeedProductAppendsSeedPocs(t *testing.T) {
	db, yp := newBindsTestEnv(t)
	name := seedName(t)
	seedN := len(seedPocsOf(name))
	if seedN == 0 {
		t.Skip("种子首产品无 pocs")
	}
	if err := bindsAdd(db, yp, name, []string{"poc-quick-bind"}); err != nil {
		t.Fatalf("bindsAdd: %v", err)
	}
	view := mergedShown(t, db)
	pocs := view[name]
	if len(pocs) != seedN+1 || pocs[seedN] != "poc-quick-bind" {
		t.Fatalf("种子产品追加应保留原 pocs: seedN=%d got=%v", seedN, pocs)
	}
}
