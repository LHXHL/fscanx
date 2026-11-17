package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/killmonday/fscanx/PocScan"
)

// newTestServer catalog API handler 测试用 server（临时目录，出厂基线走 embed）。
func newTestServer(t *testing.T) (*server, string) {
	t.Helper()
	dir := t.TempDir()
	db, err := openCatalogDB(filepath.Join(dir, "catalog.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	s := &server{
		db:                 db,
		catalogPath:        filepath.Join(dir, "catalog.yaml"),
		builtInCatalogPath: "", // 空 = 出厂基线走 embed（真出厂册）
		repoRoot:           dir, // 无指纹明文 → fingerprintNameSet 空 → 空 alias 必拒
	}
	return s, dir
}

// TestCatalogAddDeadEntryGuard 防死条目铁律（BL-POC-009）：空别名 + 名字不在指纹输出名
// 全集 → 拒绝（识别永远报不出 = 绑定永不命中）；挂别名后放行。
func TestCatalogAddDeadEntryGuard(t *testing.T) {
	s, _ := newTestServer(t)
	post := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/api/catalog_add", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		s.catalogAdd(rec, req)
		return rec
	}
	// 幽灵条目：无别名 + 名字不可能是识别输出 → 必拒
	rec := post(`{"entry":{"name":"幽灵产品XYZ"}}`)
	if !strings.Contains(rec.Body.String(), `"code":1`) {
		t.Fatalf("死条目未被拒绝: %s", rec.Body.String())
	}
	// 挂别名 → 放行
	rec = post(`{"entry":{"name":"测试产品Y","alias":{"goby":["Test-Y"]},"pocs":["poc-y"]}}`)
	if strings.Contains(rec.Body.String(), `"code":1`) {
		t.Fatalf("挂别名条目被误拒: %s", rec.Body.String())
	}
	// 同名重复 → 拒绝（提示用编辑）
	rec = post(`{"entry":{"name":"测试产品Y","alias":{"goby":["Test-Y2"]},"pocs":["poc-y2"]}}`)
	if !strings.Contains(rec.Body.String(), `"code":1`) {
		t.Fatalf("同名未拦截: %s", rec.Body.String())
	}
}

// TestCatalogListMergeAndTombstone 合并视图：出厂打底、外置覆盖显示、墓碑过滤（出厂条目
// 删除后从视图消失）、外置独立条目显示 external 来源。
func TestCatalogListMergeAndTombstone(t *testing.T) {
	s, _ := newTestServer(t)
	// 墓碑：出厂条目删除标记（出厂别名保留 + pocs 空）
	tomb, err := s.builtInTombstone("泛微 e-mobile")
	if err != nil {
		t.Fatal(err)
	}
	if err := catalogMutate(s.db, s.catalogPath, func(tx *sql.Tx) error { return dbUpsertEntry(tx, tomb) }); err != nil {
		t.Fatal(err)
	}
	// 外置独立条目
	ext := PocScan.CatalogEntry{Name: "外置独立产品", Alias: map[string][]string{"goby": {"Ext-X"}}, Pocs: []string{"poc-x"}}
	if err := catalogMutate(s.db, s.catalogPath, func(tx *sql.Tx) error { return dbUpsertEntry(tx, ext) }); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	s.catalogList(rec, httptest.NewRequest("GET", "/api/catalog_list", nil))
	body := rec.Body.String()
	if strings.Contains(body, "泛微 e-mobile") {
		t.Fatal("墓碑条目不应出现在合并视图")
	}
	if !strings.Contains(body, "泛微 e-cology") || !strings.Contains(body, `"origin":"built-in"`) {
		t.Fatal("出厂条目缺失或来源标注错误")
	}
	if !strings.Contains(body, "外置独立产品") || !strings.Contains(body, `"origin":"external"`) {
		t.Fatal("外置条目缺失或来源标注错误")
	}
}

// TestCatalogResolve 归属判定三态：出厂别名写法 → alias-of；条目名 → self；陌生名 → unassigned。
func TestCatalogResolve(t *testing.T) {
	s, _ := newTestServer(t)
	body := `{"items":[
		{"source":"goby","name":"泛微-协同办公OA"},
		{"source":"goby","name":"泛微 e-cology"},
		{"source":"goby","name":"完全陌生的名字QRS"}
	]}`
	req := httptest.NewRequest("POST", "/api/catalog_resolve", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.catalogResolve(rec, req)
	out := rec.Body.String()
	if !strings.Contains(out, `"alias-of"`) || !strings.Contains(out, "泛微 e-cology") {
		t.Fatalf("别名归属判定失败: %s", out)
	}
	if !strings.Contains(out, `"self"`) {
		t.Fatalf("标准产品自归属失败: %s", out)
	}
	if !strings.Contains(out, `"unassigned"`) {
		t.Fatalf("未归属判定失败: %s", out)
	}
}

// newBuiltInSource 带注释的出厂源文件 fixture（注释+空行+两条目，写回零漂移断言的基准）。
func newBuiltInSource(t *testing.T, dir string) *server {
	t.Helper()
	src := `# 产品册头部注释（写回后必须原样保留）。
# 第二行注释。
- name: 条目甲
  alias:
    goby:
    - A-写法
  pocs:
  - poc-yaml-a
- name: 条目乙
  alias:
    fingers:
    - B-写法
  pocs:
  - poc-yaml-b
`
	path := filepath.Join(dir, "catalog_src.yaml")
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	s, _ := newTestServer(t)
	s.builtInCatalogPath = path
	return s
}

// TestCatalogEditBuiltInPreservesComments 写回内置源文本手术回归（BL-POC-009 用户裁决：
// 稳定性/鲁棒性/高容错）：替换目标条目后头部注释与未动条目字节零漂移、目标更新、全文解析通过。
func TestCatalogEditBuiltInPreservesComments(t *testing.T) {
	s := newBuiltInSource(t, t.TempDir())
	before, err := os.ReadFile(s.builtInCatalogPath)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/api/catalog_edit",
		strings.NewReader(`{"original":"条目甲","target":"built-in","entry":{"name":"条目甲","alias":{"goby":["A-写法","新增写法"]},"pocs":["poc-yaml-a"]}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.catalogEdit(rec, req)
	if strings.Contains(rec.Body.String(), `"code":1`) {
		t.Fatalf("写回被拒: %s", rec.Body.String())
	}
	after, err := os.ReadFile(s.builtInCatalogPath)
	if err != nil {
		t.Fatal(err)
	}
	// 注释原样保留
	if !strings.HasPrefix(string(after), "# 产品册头部注释") {
		t.Fatal("头部注释丢失")
	}
	// 未动条目「条目乙」整块字节零漂移
	if !strings.Contains(string(after), "- name: 条目乙\n  alias:\n    fingers:\n    - B-写法\n  pocs:\n  - poc-yaml-b\n") {
		t.Fatalf("未动条目字节漂移:\n%s", string(after))
	}
	// 目标条目更新（新别名在）
	if !strings.Contains(string(after), "新增写法") {
		t.Fatal("目标条目未更新")
	}
	// 全文解析仍通过且条目数不变
	entries, perr := PocScan.ParseCatalogEntries(after)
	if perr != nil || len(entries) != 2 {
		t.Fatalf("写回后解析失败或条目数漂移: %v n=%d", perr, len(entries))
	}
	_ = before
}

// TestCatalogEditBuiltInPromotesExternal 外挂条目转正（BL-POC-009 用户裁决补通道）：
// 外挂条目 target=built-in → 出厂源追加 + 外挂同名行删除（origin 翻转内置）。
func TestCatalogEditBuiltInPromotesExternal(t *testing.T) {
	s := newBuiltInSource(t, t.TempDir())
	// 先落一个外挂条目
	reqAdd := httptest.NewRequest("POST", "/api/catalog_add",
		strings.NewReader(`{"entry":{"name":"外挂产品Q","alias":{"goby":["Q-写法"]},"pocs":["poc-yaml-q"]}}`))
	reqAdd.Header.Set("Content-Type", "application/json")
	recAdd := httptest.NewRecorder()
	s.catalogAdd(recAdd, reqAdd)
	if strings.Contains(recAdd.Body.String(), `"code":1`) {
		t.Fatalf("外挂条目创建失败: %s", recAdd.Body.String())
	}
	// 转正
	req := httptest.NewRequest("POST", "/api/catalog_edit",
		strings.NewReader(`{"original":"外挂产品Q","target":"built-in","entry":{"name":"外挂产品Q","alias":{"goby":["Q-写法"]},"pocs":["poc-yaml-q"]}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.catalogEdit(rec, req)
	if strings.Contains(rec.Body.String(), `"code":1`) {
		t.Fatalf("转正被拒: %s", rec.Body.String())
	}
	// 源文件已追加
	after, _ := os.ReadFile(s.builtInCatalogPath)
	if !strings.Contains(string(after), "- name: 外挂产品Q") {
		t.Fatalf("转正条目未追加到源文件:\n%s", string(after))
	}
	if !strings.HasPrefix(string(after), "# 产品册头部注释") {
		t.Fatal("转正后头部注释丢失")
	}
	// 外挂同名行已删
	outer, err := dbListEntries(s.db)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range outer {
		if e.Name == "外挂产品Q" {
			t.Fatal("转正后外挂同名行未删除（外挂覆盖会遮蔽内置定义）")
		}
	}
}

// TestCatalogEditBuiltInRejectsConflict 转正撞名守卫：改名目标与出厂条目冲突 → 拒绝且源文件字节不动。
func TestCatalogEditBuiltInRejectsConflict(t *testing.T) {
	s := newBuiltInSource(t, t.TempDir())
	before, _ := os.ReadFile(s.builtInCatalogPath)
	req := httptest.NewRequest("POST", "/api/catalog_edit",
		strings.NewReader(`{"original":"幽灵外挂Z","target":"built-in","entry":{"name":"条目甲","alias":{"goby":["Z"]},"pocs":[]}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.catalogEdit(rec, req)
	if !strings.Contains(rec.Body.String(), `"code":1`) {
		t.Fatalf("撞名转正未被拒绝: %s", rec.Body.String())
	}
	after, _ := os.ReadFile(s.builtInCatalogPath)
	if string(before) != string(after) {
		t.Fatal("拒绝写回后源文件字节被改动")
	}
}

// ---- 对抗式复审轮（用户指令：业务逻辑正确/鲁棒/幂等）----

// TestCatalogEditBuiltInRejectsDuplicateBlocks 同名重复块防御：源文件被手改成两个同名条目
// （异常态）→ 拒绝写回且文件字节不动（只替换首个会静默残留旧块，全文校验发现不了）。
func TestCatalogEditBuiltInRejectsDuplicateBlocks(t *testing.T) {
	s := newBuiltInSource(t, t.TempDir())
	// 手工制造同名重复（追加一个同名块）
	src, _ := os.ReadFile(s.builtInCatalogPath)
	dup := append(src, []byte("- name: 条目甲\n  alias:\n    goby:\n    - 重复写法\n")...)
	os.WriteFile(s.builtInCatalogPath, dup, 0644)
	before, _ := os.ReadFile(s.builtInCatalogPath)
	req := httptest.NewRequest("POST", "/api/catalog_edit",
		strings.NewReader(`{"original":"条目甲","target":"built-in","entry":{"name":"条目甲","alias":{"goby":["新写法"]},"pocs":[]}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.catalogEdit(rec, req)
	if !strings.Contains(rec.Body.String(), `"code":1`) {
		t.Fatalf("同名重复块未被拒绝: %s", rec.Body.String())
	}
	after, _ := os.ReadFile(s.builtInCatalogPath)
	if string(before) != string(after) {
		t.Fatal("拒绝写回后源文件字节被改动")
	}
}

// TestCatalogEditBuiltInCRLFLineEndings 行尾适配（git autocrlf 物化 CRLF 前科）：CRLF 源文件
// 写回后新块行尾与源文件一致（无混合行尾），未动部分字节零漂移。
func TestCatalogEditBuiltInCRLFLineEndings(t *testing.T) {
	s := newBuiltInSource(t, t.TempDir())
	src, _ := os.ReadFile(s.builtInCatalogPath)
	crlfSrc := bytes.ReplaceAll(src, []byte("\n"), []byte("\r\n"))
	os.WriteFile(s.builtInCatalogPath, crlfSrc, 0644)
	req := httptest.NewRequest("POST", "/api/catalog_edit",
		strings.NewReader(`{"original":"条目甲","target":"built-in","entry":{"name":"条目甲","alias":{"goby":["A-写法"]},"pocs":["poc-yaml-a"]}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.catalogEdit(rec, req)
	if strings.Contains(rec.Body.String(), `"code":1`) {
		t.Fatalf("CRLF 文件写回被拒: %s", rec.Body.String())
	}
	after, _ := os.ReadFile(s.builtInCatalogPath)
	// 新块内容行尾为 CRLF
	if !strings.Contains(string(after), "- name: 条目甲\r\n") {
		t.Fatal("写回块行尾未适配 CRLF（产生混合行尾）")
	}
	// 未动条目保持 CRLF 原样
	if !strings.Contains(string(after), "- name: 条目乙\r\n") {
		t.Fatal("未动条目行尾漂移")
	}
}

// TestCatalogEditBuiltInClearsTombstone 墓碑压制解除：出厂条目被删（外挂墓碑）后写回内置源
// = 撤销删除意图 → 外挂墓碑行清理，写回的内置定义在合并视图可见（不被墓碑压制）。
func TestCatalogEditBuiltInClearsTombstone(t *testing.T) {
	s := newBuiltInSource(t, t.TempDir())
	// 删除出厂条目（落墓碑：别名保留 pocs 空）
	reqDel := httptest.NewRequest("POST", "/api/catalog_delete", strings.NewReader(`{"name":"条目甲"}`))
	reqDel.Header.Set("Content-Type", "application/json")
	recDel := httptest.NewRecorder()
	s.catalogDelete(recDel, reqDel)
	if strings.Contains(recDel.Body.String(), `"code":1`) {
		t.Fatalf("删除失败: %s", recDel.Body.String())
	}
	// 合并视图确认被墓碑过滤
	d := listEntriesView(t, s)
	for _, e := range d {
		if e.Name == "条目甲" {
			t.Fatal("墓碑未过滤，前置态不成立")
		}
	}
	// 写回内置源（撤销删除意图）
	req := httptest.NewRequest("POST", "/api/catalog_edit",
		strings.NewReader(`{"original":"条目甲","target":"built-in","entry":{"name":"条目甲","alias":{"goby":["A-写法"]},"pocs":["poc-yaml-a"]}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.catalogEdit(rec, req)
	if strings.Contains(rec.Body.String(), `"code":1`) {
		t.Fatalf("写回被拒: %s", rec.Body.String())
	}
	// 外挂墓碑已清理 → 合并视图可见内置定义
	outer, err := dbListEntries(s.db)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range outer {
		if e.Name == "条目甲" {
			t.Fatal("墓碑行未清理（写回的内置定义被压制不可见）")
		}
	}
	d2 := listEntriesView(t, s)
	found := false
	for _, e := range d2 {
		if e.Name == "条目甲" && e.Origin == "built-in" {
			found = true
		}
	}
	if !found {
		t.Fatal("写回后内置条目在合并视图不可见")
	}
}

// TestCatalogEditBuiltInPromoteIdempotent 转正幂等：首次转正（追加+外挂清理）后再次以
// target=built-in 编辑同名条目 = 编辑内置（替换语义），文件不产生重复条目。
func TestCatalogEditBuiltInPromoteIdempotent(t *testing.T) {
	s := newBuiltInSource(t, t.TempDir())
	post := func(body string) {
		req := httptest.NewRequest("POST", "/api/catalog_edit", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		s.catalogEdit(rec, req)
		if strings.Contains(rec.Body.String(), `"code":1`) {
			t.Fatalf("编辑被拒: %s", rec.Body.String())
		}
	}
	// 首次：外挂条目转正
	reqAdd := httptest.NewRequest("POST", "/api/catalog_add",
		strings.NewReader(`{"entry":{"name":"转正幂等P","alias":{"goby":["P-写法"]},"pocs":["poc-yaml-p"]}}`))
	reqAdd.Header.Set("Content-Type", "application/json")
	recAdd := httptest.NewRecorder()
	s.catalogAdd(recAdd, reqAdd)
	if strings.Contains(recAdd.Body.String(), `"code":1`) {
		t.Fatalf("外挂创建失败: %s", recAdd.Body.String())
	}
	post(`{"original":"转正幂等P","target":"built-in","entry":{"name":"转正幂等P","alias":{"goby":["P-写法"]},"pocs":["poc-yaml-p"]}}`)
	after1, _ := os.ReadFile(s.builtInCatalogPath)
	// 二次：同名 target=built-in 再保存（内容微调）→ 应走替换语义而非再追加
	post(`{"original":"转正幂等P","target":"built-in","entry":{"name":"转正幂等P","alias":{"goby":["P-写法","P-改"]},"pocs":["poc-yaml-p"]}}`)
	after2, _ := os.ReadFile(s.builtInCatalogPath)
	// 条目不重复
	if strings.Count(string(after2), "- name: 转正幂等P") != 1 {
		t.Fatalf("二次保存后条目重复（幂等破坏）:\n%s", string(after2))
	}
	if !strings.Contains(string(after2), "P-改") {
		t.Fatal("二次保存内容未更新")
	}
	_ = after1
}

// listEntriesView 直调 catalogList 取合并视图条目（测试辅助）。
func listEntriesView(t *testing.T, s *server) []catalogEntryView {
	t.Helper()
	req := httptest.NewRequest("GET", "/api/catalog_list", nil)
	rec := httptest.NewRecorder()
	s.catalogList(rec, req)
	var resp struct {
		Code int `json:"code"`
		Data struct {
			Entries []catalogEntryView `json:"entries"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析合并视图失败: %v body=%s", err, rec.Body.String())
	}
	if resp.Code != 0 {
		t.Fatalf("catalog_list 失败: %s", rec.Body.String())
	}
	return resp.Data.Entries
}

// TestToEntryViewSelfStdNoWarning 缺别名警示的"自标准名"排除（对抗式复审⑥）：别名区为空
// 但名字在指纹输出名全集（repoRoot 有指纹明文）→ 不误标 missing_alias；不在全集 → 标警示。
func TestToEntryViewSelfStdNoWarning(t *testing.T) {
	dir := t.TempDir()
	// 造指纹明文（fingerprintNameSet 读 repoRoot 相对路径结构）
	gobyDir := filepath.Join(dir, "mylib", "finger", "resources", "goby.json")
	if err := os.MkdirAll(gobyDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gobyDir, "goby.json"), []byte(`[{"name":"自标准名条目X"}]`), 0644); err != nil {
		t.Fatal(err)
	}
	db, err := openCatalogDB(filepath.Join(dir, "catalog.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	s := &server{db: db, catalogPath: filepath.Join(dir, "catalog.yaml"), repoRoot: dir}

	inSet := s.toEntryView(PocScan.CatalogEntry{Name: "自标准名条目X"}, "built-in")
	if inSet.MissingAlias {
		t.Fatal("自标准名条目（名字在指纹全集）被误标缺别名警示")
	}
	notInSet := s.toEntryView(PocScan.CatalogEntry{Name: "幽灵条目Z"}, "external")
	if !notInSet.MissingAlias {
		t.Fatal("全集外空别名条目未标警示（死条目警示失效）")
	}
}

// TestCatalogEditBuiltInBackup 写回前滚动备份：写回成功后 <源文件>.bak 内容 = 写回前原文
// （未来任何未预见缺陷的一键恢复通道）。
func TestCatalogEditBuiltInBackup(t *testing.T) {
	s := newBuiltInSource(t, t.TempDir())
	before, _ := os.ReadFile(s.builtInCatalogPath)
	req := httptest.NewRequest("POST", "/api/catalog_edit",
		strings.NewReader(`{"original":"条目甲","target":"built-in","entry":{"name":"条目甲","alias":{"goby":["A-写法"]},"pocs":["poc-yaml-a"]}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.catalogEdit(rec, req)
	if strings.Contains(rec.Body.String(), `"code":1`) {
		t.Fatalf("写回被拒: %s", rec.Body.String())
	}
	bak, err := os.ReadFile(s.builtInCatalogPath + ".bak")
	if err != nil {
		t.Fatalf("写回后备份文件缺失: %v", err)
	}
	if string(bak) != string(before) {
		t.Fatal("备份内容与写回前原文不一致")
	}
}

// TestCatalogAddBuiltIn 新建直接进代码源文件（用户裁决）：target=built-in → 出厂源追加 +
// 注释保留 + 原文前缀零漂移；外挂层不落行。
func TestCatalogAddBuiltIn(t *testing.T) {
	s := newBuiltInSource(t, t.TempDir())
	before, _ := os.ReadFile(s.builtInCatalogPath)
	req := httptest.NewRequest("POST", "/api/catalog_add",
		strings.NewReader(`{"target":"built-in","entry":{"name":"新建内置产品N","alias":{"goby":["N-写法"]},"pocs":["poc-yaml-n"]}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.catalogAdd(rec, req)
	if strings.Contains(rec.Body.String(), `"code":1`) {
		t.Fatalf("新建写入内置源被拒: %s", rec.Body.String())
	}
	after, _ := os.ReadFile(s.builtInCatalogPath)
	if !strings.HasPrefix(string(after), "# 产品册头部注释") {
		t.Fatal("头部注释丢失")
	}
	if !strings.HasPrefix(string(after), string(before)) {
		t.Fatal("原文被改动（应仅尾部追加）")
	}
	if !strings.Contains(string(after), "- name: 新建内置产品N") {
		t.Fatal("新条目未追加")
	}
	outer, err := dbListEntries(s.db)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range outer {
		if e.Name == "新建内置产品N" {
			t.Fatal("target=built-in 的新建不应落外挂层")
		}
	}
}
