package main

// 指纹条目 CRUD（BL-POC-006）：SQLite 主存（finger_db.go）+ 明文 JSON 快照双轨，
// 按页面接口命名规范四端点（/api/finger_list、/api/finger_add、/api/finger_edit、
// /api/finger_delete；复制复用 _copy）。整文 GET/PUT /api/finger 退役。
// 表单序列化按所在源 schema（键序=canonical 键序，缩进=所在源契约）；超 schema 条目
// 由前端降级原文编辑（raw 模式内容即所写）。产品名全集与 pack 管线仍直接消费明文
// （重生成保证两者读到的一直是 DB 的最新快照）。
//
// 条目身份 = (file, id)——SQLite rowid。name 不构成身份（fingers_http 实测 30 组重名）。

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/killmonday/fscanx/mylib/finger/packlib"
)

var (
	errNameRequired = errors.New("name 不能为空")
	errUnknownFile  = errors.New("未知指纹文件")
)

// fingerFiles 明文名 → 工作副本路径映射（与 packlib.pairs 同源约定）。
var fingerFiles = map[string]string{
	"goby":         filepath.Join("mylib", "finger", "resources", "goby.json", "goby.json"),
	"fingers_http": filepath.Join("mylib", "finger", "resources", "fingers_http.json", "fingers_http.json"),
}

func (s *server) fingerPath(file string) (string, bool) {
	rel, ok := fingerFiles[file]
	if !ok {
		return "", false
	}
	return filepath.Join(s.repoRoot, rel), true
}

// ---- 表单序列化 schema（两源各自 canonical 键序与缩进；超 schema 形态走 raw 模式）----

type gobyRuleItem struct {
	Label   string `json:"label"`
	Feature string `json:"feature"`
	IsEqual bool   `json:"is_equal"`
}

type gobyEntrySchema struct {
	Name  string         `json:"name"`
	Focus bool           `json:"focus,omitempty"`
	Logic string         `json:"logic"`
	Rule  []gobyRuleItem `json:"rule"`
}

type fingersFavicon struct {
	Md5  string   `json:"md5,omitempty"`
	Mmh3 []string `json:"mmh3,omitempty"`
}

type fingersRuleItem struct {
	Favicon *fingersFavicon `json:"favicon,omitempty"`
}

type fingersEntrySchema struct {
	Focus bool              `json:"focus,omitempty"`
	Link  string            `json:"link,omitempty"`
	Name  string            `json:"name"`
	Rule  []fingersRuleItem `json:"rule,omitempty"`
	Tag   []string          `json:"tag,omitempty"`
}

// serializeFingerEntry 表单结构 → 元素 JSON 文本。缩进契约按所在源实测格式：
// goby 元素 { 在 0 列、字段 2 空格（prefix="", indent="  "）；fingers_http 元素 { 在
// 4 空格、字段 8 空格（prefix="    ", indent="    "）。首行均无缩进——行首缩进由模板
// 间隙提供，与原文件元素形态一致。纵深防御：form 模式拒绝 schema 外键（UI 已按判据
// 降级原文模式，直打 API 也拦，防静默剥字段后返回 saved）。
func serializeFingerEntry(file string, entry json.RawMessage) (string, error) {
	var schemaKeys []string
	switch file {
	case "goby":
		schemaKeys = []string{"name", "focus", "logic", "rule"}
	case "fingers_http":
		schemaKeys = []string{"focus", "link", "name", "rule", "tag"}
	default:
		return "", errUnknownFile
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(entry, &probe); err != nil {
		return "", err
	}
	keyOK := map[string]bool{}
	for _, k := range schemaKeys {
		keyOK[k] = true
	}
	for k := range probe {
		if !keyOK[k] {
			return "", fmt.Errorf("条目含表单 schema 外字段 %q，请用原文模式编辑", k)
		}
	}
	prefix, indent := "", ""
	switch file {
	case "goby":
		prefix, indent = "", "  "
		var e gobyEntrySchema
		if err := json.Unmarshal(entry, &e); err != nil {
			return "", err
		}
		if strings.TrimSpace(e.Name) == "" {
			return "", errNameRequired
		}
		b, err := json.MarshalIndent(e, prefix, indent)
		return string(b), err
	case "fingers_http":
		prefix, indent = "    ", "    "
		var e fingersEntrySchema
		if err := json.Unmarshal(entry, &e); err != nil {
			return "", err
		}
		if strings.TrimSpace(e.Name) == "" {
			return "", errNameRequired
		}
		b, err := json.MarshalIndent(e, prefix, indent)
		return string(b), err
	}
	return "", errUnknownFile
}

// ---- 端点 ----

// fingerList 条目列表：搜索（name LIKE）+ 分页；行含 raw 供前端结构化回填/降级判定。
func (s *server) fingerList(w http.ResponseWriter, r *http.Request) {
	file := r.URL.Query().Get("file")
	path, ok := s.fingerPath(file)
	if !ok {
		httpErrorf(w, "未知指纹文件: %s", file)
		return
	}
	if err := absorbFingerFile(s.fingerDB, path, file); err != nil {
		httpErrorf(w, "%v", err)
		return
	}
	keyword := strings.TrimSpace(r.URL.Query().Get("keyword"))
	page, size := atoiDefault(r.URL.Query().Get("page"), 1), atoiDefault(r.URL.Query().Get("size"), 20)
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	var total int
	like := "%" + keyword + "%"
	if err := s.fingerDB.QueryRow(`SELECT COUNT(*) FROM finger_entries WHERE file = ? AND name LIKE ?`, file, like).Scan(&total); err != nil {
		httpErrorf(w, "查询失败: %v", err)
		return
	}
	rows, err := s.fingerDB.Query(
		`SELECT id, name, raw FROM finger_entries WHERE file = ? AND name LIKE ? ORDER BY id LIMIT ? OFFSET ?`,
		file, like, size, (page-1)*size)
	if err != nil {
		httpErrorf(w, "查询失败: %v", err)
		return
	}
	defer rows.Close()
	items := make([]map[string]interface{}, 0, size)
	for rows.Next() {
		var id int64
		var name, raw string
		if err := rows.Scan(&id, &name, &raw); err != nil {
			httpErrorf(w, "查询失败: %v", err)
			return
		}
		items = append(items, map[string]interface{}{"id": id, "name": name, "raw": raw})
	}
	writeJSON(w, map[string]interface{}{"total": total, "page": page, "size": size, "items": items})
}

// fingerAdd 新增条目（表单结构 → 所在源序列化 → 追加到文件尾部）。
func (s *server) fingerAdd(w http.ResponseWriter, r *http.Request) {
	var req struct {
		File  string          `json:"file"`
		Entry json.RawMessage `json:"entry"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	path, ok := s.fingerPath(req.File)
	if !ok {
		httpErrorf(w, "未知指纹文件: %s", req.File)
		return
	}
	if err := absorbFingerFile(s.fingerDB, path, req.File); err != nil {
		httpErrorf(w, "%v", err)
		return
	}
	raw, err := serializeFingerEntry(req.File, req.Entry)
	if err != nil {
		httpErrorf(w, "条目序列化失败: %v", err)
		return
	}
	var probe struct {
		Name string `json:"name"`
	}
	_ = json.Unmarshal(req.Entry, &probe)
	if err := fingerAdd(s.fingerDB, path, req.File, probe.Name, raw); err != nil {
		httpErrorf(w, "%v", err)
		return
	}
	writeJSON(w, map[string]string{"status": "saved"})
}

// fingerEdit 编辑条目：mode=form 表单结构重序列化 / mode=raw 内容即所写（校验 JSON
// 对象 + name 非空）。重名条目按 id 各自独立。
func (s *server) fingerEdit(w http.ResponseWriter, r *http.Request) {
	var req struct {
		File  string          `json:"file"`
		ID    int64           `json:"id"`
		Mode  string          `json:"mode"` // form | raw
		Entry json.RawMessage `json:"entry"`
		Text  string          `json:"text"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	path, ok := s.fingerPath(req.File)
	if !ok {
		httpErrorf(w, "未知指纹文件: %s", req.File)
		return
	}
	// 编辑前吸收对齐（同 add/copy/delete）：漏掉会把未吸收的手改随重生成静默覆写掉
	if err := absorbFingerFile(s.fingerDB, path, req.File); err != nil {
		httpErrorf(w, "%v", err)
		return
	}
	var name, raw string
	switch req.Mode {
	case "form":
		if req.ID <= 0 {
			httpErrorf(w, "id 不能为空")
			return
		}
		text, err := serializeFingerEntry(req.File, req.Entry)
		if err != nil {
			httpErrorf(w, "条目序列化失败: %v", err)
			return
		}
		raw = text
		var probe struct {
			Name string `json:"name"`
		}
		_ = json.Unmarshal(req.Entry, &probe)
		name = probe.Name
	case "raw":
		var probe map[string]interface{}
		if !json.Valid([]byte(req.Text)) || json.Unmarshal([]byte(req.Text), &probe) != nil {
			httpErrorf(w, "内容不是合法 JSON 对象，已阻止写入")
			return
		}
		n, _ := probe["name"].(string)
		if strings.TrimSpace(n) == "" {
			httpErrorf(w, "name 不能为空")
			return
		}
		name, raw = n, req.Text
	default:
		httpErrorf(w, "mode 须为 form 或 raw")
		return
	}
	if err := fingerEditRaw(s.fingerDB, path, req.File, req.ID, name, raw); err != nil {
		httpErrorf(w, "%v", err)
		return
	}
	writeJSON(w, map[string]string{"status": "saved"})
}

// fingerCopy 复制条目：原文字节克隆为新行（同名允许——重名本就常态，用户随后自行改）。
func (s *server) fingerCopy(w http.ResponseWriter, r *http.Request) {
	var req struct {
		File string `json:"file"`
		ID   int64  `json:"id"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	path, ok := s.fingerPath(req.File)
	if !ok {
		httpErrorf(w, "未知指纹文件: %s", req.File)
		return
	}
	if err := absorbFingerFile(s.fingerDB, path, req.File); err != nil {
		httpErrorf(w, "%v", err)
		return
	}
	e, err := fingerGet(s.fingerDB, req.File, req.ID)
	if err != nil {
		httpErrorf(w, "条目不存在（可能已被删除）")
		return
	}
	if err := fingerAdd(s.fingerDB, path, req.File, e.Name, e.Raw); err != nil {
		httpErrorf(w, "%v", err)
		return
	}
	writeJSON(w, map[string]string{"status": "copied"})
}

// fingerDelete 删除条目（删行即消失；手改明文加回会被吸收为新条目——尊重手改）。
func (s *server) fingerDelete(w http.ResponseWriter, r *http.Request) {
	var req struct {
		File string `json:"file"`
		ID   int64  `json:"id"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	path, ok := s.fingerPath(req.File)
	if !ok {
		httpErrorf(w, "未知指纹文件: %s", req.File)
		return
	}
	if err := absorbFingerFile(s.fingerDB, path, req.File); err != nil {
		httpErrorf(w, "%v", err)
		return
	}
	if err := fingerDelete(s.fingerDB, path, req.File, req.ID); err != nil {
		httpErrorf(w, "%v", err)
		return
	}
	writeJSON(w, map[string]string{"status": "deleted"})
}

// fingerWrite 已并入 fingerAdd（单调用方不抽骨架）。

func (s *server) postPack(w http.ResponseWriter, r *http.Request) {
	results, err := packlib.Run(s.repoRoot)
	if err != nil {
		httpErrorf(w, "打包失败: %v", err)
		return
	}
	out := make([]map[string]interface{}, 0, len(results))
	for _, res := range results {
		out = append(out, map[string]interface{}{
			"src": res.Src, "dst": res.Dst, "raw": res.Raw, "gz": res.Gz,
		})
	}
	writeJSON(w, map[string]interface{}{"status": "packed", "results": out})
}

// listProducts 提取两指纹明文的产品名全集（绑定编辑器下拉数据源；
// 键=引擎数据源 name 字段，与 binds.product 同命名空间）。
func (s *server) listProducts(w http.ResponseWriter, r *http.Request) {
	set := s.fingerprintNameSet()
	products := make([]string, 0, len(set))
	for n := range set {
		products = append(products, n)
	}
	sort.Strings(products)
	writeJSON(w, map[string]interface{}{"products": products})
}

// fingerprintNameSet 两指纹明文的 name 全集（产品册"自标准名"合法性校验用——
// 名字在全集内 = 识别引擎会原样输出它，空别名条目也非死条目）。
func (s *server) fingerprintNameSet() map[string]bool {
	set := map[string]bool{}
	for name := range fingerFiles {
		path, _ := s.fingerPath(name)
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var arr []map[string]interface{}
		if json.Unmarshal(raw, &arr) != nil {
			continue
		}
		for _, m := range arr {
			if n, ok := m["name"].(string); ok && n != "" {
				set[n] = true
			}
		}
	}
	return set
}

// ---- 助手（api_binds.go 等共用）----

func sprintf(format string, args ...interface{}) string { return fmt.Sprintf(format, args...) }

// atoiDefault 查询参数整数解析（非法/为空取默认值）。
func atoiDefault(s string, def int) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return def
}
