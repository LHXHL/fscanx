package main

// poc CRUD 与 CEL 校验（BL-POC-004；多步 rules 结构化迭代轮）。
// 列表=内置 embed + 外置 pocs/；编辑返回多步结构化视图（每步 method/path/headers/
// body/search/follow_redirects/continue/expression/谓词树），set/sets 变量与 always
// 标记一并返回；groups 等长尾形态表单不承载（form=false，前端走源码模式）。
// 写回落盘前双重校验：yaml 可解析为 Poc 结构 + 每条 rule 的 expression 编译通过。
// 表单保存的 yaml 渲染：表单管辖字段（name/always/set/sets/rules）从表单值生成，
// 其余顶级段（detail/groups/manual/...）从打开时原文提取拼接——未暴露字段零丢失。

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
	"github.com/killmonday/fscanx/PocScan"
	"github.com/killmonday/fscanx/PocScan/lib"
	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
	"gopkg.in/yaml.v2"
)

// pocItem 列表项。
type pocItem struct {
	Name   string `json:"name"`
	Source string `json:"source"` // built-in | external
	Always bool   `json:"always"` // 无条件全打型标记（列表可视化）
}

// stepView 编辑视图的单步 rule：全字段结构化 + 谓词树（Tree 为 null 表示该步
// expression 超出谓词子集——前端该步降级为 CEL 文本编辑，不再整卡切源码模式）。
type stepView struct {
	Method          string            `json:"method"`
	Path            string            `json:"path"`
	Headers         map[string]string `json:"headers"`
	Body            string            `json:"body"`
	Search          string            `json:"search"`
	FollowRedirects bool              `json:"followRedirects"`
	Continue        bool              `json:"continue"`
	Expression      string            `json:"expression"`
	Tree            *Node             `json:"tree"`
}

// kvView 单值 set 变量（value 是表达式，如 randomInt(...)）。
type kvView struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// kvListsView 列表 sets 变量。
type kvListsView struct {
	Key    string   `json:"key"`
	Values []string `json:"values"`
}

// pocDetail 编辑视图：结构化字段 + yaml 原文。
type pocDetail struct {
	Name   string       `json:"name"`
	Always bool         `json:"always"`
	Set    []kvView     `json:"set"`
	Sets   []kvListsView `json:"sets"`
	Steps  []stepView   `json:"steps"`
	Form   bool         `json:"form"` // 表单可承载（groups 等长尾形态为 false）
	Yaml   string       `json:"yaml"` // 原文（源码模式与段落保真基底）
}

// pocHandler 同 path 的方法分发（go1.20 无方法路由模式）。
func (s *server) pocHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.getPoc(w, r)
	case http.MethodPut:
		s.putPoc(w, r)
	case http.MethodDelete:
		s.deletePoc(w, r)
	default:
		w.Header().Set("Allow", "GET, PUT, DELETE")
		http.Error(w, "方法不允许: "+r.Method, http.StatusMethodNotAllowed)
	}
}

func (s *server) listPocs(w http.ResponseWriter, r *http.Request) {
	items := []pocItem{}
	// 内置：embed 真源，解析出真实 name
	entries, err := PocScan.Pocs.ReadDir("pocs")
	if err == nil {
		for _, e := range entries {
			n := e.Name()
			if strings.HasSuffix(n, ".yml") || strings.HasSuffix(n, ".yaml") {
				if p, _ := lib.LoadPoc(n, PocScan.Pocs); p != nil && p.Name != "" {
					items = append(items, pocItem{Name: p.Name, Source: "built-in", Always: p.Always})
				}
			}
		}
	}
	// 外置：二进制旁 pocs/
	if ext := listExternalPocs(s.pocsDir); len(ext) > 0 {
		items = append(items, ext...)
	}
	writeJSON(w, map[string]interface{}{"pocs": items})
}

// listExternalPocs 解析外置目录全部 poc 的 name。
func listExternalPocs(dir string) (items []pocItem) {
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".yml") || strings.HasSuffix(path, ".yaml") {
			if p, _ := lib.LoadPocbyPath(path); p != nil && p.Name != "" {
				items = append(items, pocItem{Name: p.Name, Source: "external", Always: p.Always})
			}
		}
		return nil
	})
	return
}

// findPocFile 按 name 定位 poc 文件（外置优先——与加载覆盖语义一致），返回 yaml 原文。
func (s *server) findPocFile(name string) (yamlText string, external bool, ok bool) {
	if ext := listExternalPocs(s.pocsDir); len(ext) > 0 {
		_ = filepath.Walk(s.pocsDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info == nil || info.IsDir() {
				return nil
			}
			if p, _ := lib.LoadPocbyPath(path); p != nil && p.Name == name {
				if raw, err := os.ReadFile(path); err == nil {
					yamlText, external, ok = string(raw), true, true
				}
			}
			return nil
		})
		if ok {
			return
		}
	}
	entries, err := PocScan.Pocs.ReadDir("pocs")
	if err != nil {
		return
	}
	for _, e := range entries {
		n := e.Name()
		if !strings.HasSuffix(n, ".yml") && !strings.HasSuffix(n, ".yaml") {
			continue
		}
		if p, _ := lib.LoadPoc(n, PocScan.Pocs); p != nil && p.Name == name {
			if raw, err := PocScan.Pocs.ReadFile("pocs/" + n); err == nil {
				yamlText, external, ok = string(raw), false, true
			}
			return
		}
	}
	return
}

func (s *server) getPoc(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "缺少 name", 400)
		return
	}
	yamlText, _, ok := s.findPocFile(name)
	if !ok {
		http.Error(w, "未找到 poc: "+name, 404)
		return
	}
	writeJSON(w, pocFromYaml(name, yamlText, s))
}

// pocFromYaml yaml 原文 → 结构化编辑视图（全 rules 步骤化 + 谓词树反解析）。
func pocFromYaml(name, yamlText string, s *server) pocDetail {
	var p lib.Poc
	if err := yaml.Unmarshal([]byte(yamlText), &p); err != nil {
		return pocDetail{Name: name, Form: false, Yaml: yamlText}
	}
	d := pocDetail{
		Name:   name,
		Always: p.Always,
		Form:   len(p.Groups) == 0 && len(p.Rules) > 0,
		Yaml:   yamlText,
	}
	for _, kv := range p.Set {
		d.Set = append(d.Set, kvView{Key: kv.Key, Value: kv.Value})
	}
	for _, kv := range p.Sets {
		d.Sets = append(d.Sets, kvListsView{Key: kv.Key, Values: kv.Value})
	}
	for _, rule := range p.Rules {
		d.Steps = append(d.Steps, stepView{
			Method:          rule.Method,
			Path:            rule.Path,
			Headers:         rule.Headers,
			Body:            rule.Body,
			Search:          rule.Search,
			FollowRedirects: rule.FollowRedirects,
			Continue:        rule.Continue,
			Expression:      rule.Expression,
			Tree:            ParsePredicate(rule.Expression, s.celEnv),
		})
	}
	return d
}

// putPocReq 写入请求：表单模式（多步结构化）或源码模式。
type putPocReq struct {
	Mode string `json:"mode"` // form | raw
	Name string `json:"name"`
	// form 模式
	Always   bool          `json:"always"`
	Set      []kvView      `json:"set"`
	Sets     []kvListsView `json:"sets"`
	Steps    []putStep     `json:"steps"`
	BaseYaml string        `json:"baseYaml"` // 打开时原文：detail/groups 等未暴露段保真继承
	// raw 模式
	Yaml string `json:"yaml"`
}

// putStep 表单提交的单步 rule：Tree（谓词子集内）与 Expression（原文兜底）二选一。
type putStep struct {
	Method          string            `json:"method"`
	Path            string            `json:"path"`
	Headers         map[string]string `json:"headers"`
	Body            string            `json:"body"`
	Search          string            `json:"search"`
	FollowRedirects bool              `json:"followRedirects"`
	Continue        bool              `json:"continue"`
	Tree            *Node             `json:"tree"`
	Expression      string            `json:"expression"`
}

func (s *server) putPoc(w http.ResponseWriter, r *http.Request) {
	var req putPocReq
	if !readJSON(w, r, &req) {
		return
	}
	if req.Name == "" {
		http.Error(w, "name 不能为空", 400)
		return
	}
	var yamlText string
	switch req.Mode {
	case "form":
		text, err := s.renderPocYAML(&req)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		yamlText = text
	case "raw":
		yamlText = req.Yaml
	default:
		http.Error(w, "mode 须为 form 或 raw", 400)
		return
	}
	// 落盘前双重校验：可解析为 Poc + name 一致 + 每条 expression 编译通过
	var p lib.Poc
	if err := yaml.Unmarshal([]byte(yamlText), &p); err != nil {
		http.Error(w, "yaml 解析失败: "+err.Error(), 400)
		return
	}
	if p.Name == "" || p.Name != req.Name {
		http.Error(w, "yaml 的 name 为空或与请求 name 不一致", 400)
		return
	}
	if len(p.Rules) == 0 {
		http.Error(w, "rules 为空", 400)
		return
	}
	env, err := s.envWithSetVars(p.Set)
	if err != nil {
		http.Error(w, "构造变量环境失败: "+err.Error(), 500)
		return
	}
	for i, rule := range p.Rules {
		if strings.TrimSpace(rule.Expression) == "" {
			http.Error(w, fmt.Sprintf("第 %d 条 rule 缺少 expression", i+1), 400)
			return
		}
		if _, iss := env.Compile(rule.Expression); iss.Err() != nil {
			http.Error(w, fmt.Sprintf("第 %d 条 rule 的 expression 编译失败: %v", i+1, iss.Err()), 400)
			return
		}
	}
	if err := os.MkdirAll(s.pocsDir, 0755); err != nil {
		http.Error(w, "创建外置目录失败: "+err.Error(), 500)
		return
	}
	if err := os.WriteFile(filepath.Join(s.pocsDir, sanitizeName(req.Name)+".yml"), []byte(yamlText), 0644); err != nil {
		http.Error(w, "写盘失败: "+err.Error(), 500)
		return
	}
	writeJSON(w, map[string]string{"status": "saved", "file": filepath.Join(s.pocsDir, sanitizeName(req.Name)+".yml")})
}

// envWithSetVars 在基础环境上声明 set 变量（与求值端 lib/eval.go 前缀类型规则同源：
// randomInt→int、newReverse→Reverse、其余→string）。含变量引用的 expression 在裸
// 环境下编译必失败（undeclared reference），内置 166/386 poc 用 set——校验必须带声明。
func (s *server) envWithSetVars(set lib.StrMap) (*cel.Env, error) {
	if len(set) == 0 {
		return s.celEnv, nil
	}
	var opts []cel.EnvOption
	for _, kv := range set {
		if kv.Key == "" {
			continue
		}
		var d *exprpb.Decl
		switch {
		case strings.HasPrefix(kv.Value, "randomInt"):
			d = decls.NewIdent(kv.Key, decls.Int, nil)
		case strings.HasPrefix(kv.Value, "newReverse"):
			d = decls.NewIdent(kv.Key, decls.NewObjectType("lib.Reverse"), nil)
		default:
			d = decls.NewIdent(kv.Key, decls.String, nil)
		}
		opts = append(opts, cel.Declarations(d))
	}
	return s.celEnv.Extend(opts...)
}

// renderPocYAML 表单请求 → yaml 文本：表单管辖字段渲染 + BaseYaml 未暴露顶级段保真拼接。
func (s *server) renderPocYAML(req *putPocReq) (string, error) {
	if len(req.Steps) == 0 {
		return "", fmt.Errorf("至少需要一步请求")
	}
	setVars := make(lib.StrMap, 0, len(req.Set))
	for _, kv := range req.Set {
		if kv.Key != "" {
			setVars = append(setVars, lib.StrItem{Key: kv.Key, Value: kv.Value})
		}
	}
	env, err := s.envWithSetVars(setVars)
	if err != nil {
		return "", fmt.Errorf("构造变量环境失败: %v", err)
	}
	var b strings.Builder
	b.WriteString(yamlLine(yaml.MapSlice{{Key: "name", Value: req.Name}}))
	if req.Always {
		b.WriteString("\nalways: true")
	}
	if len(req.Set) > 0 {
		b.WriteString("\nset:")
		for _, kv := range req.Set {
			if kv.Key == "" {
				continue
			}
			b.WriteString("\n  " + yamlLine(yaml.MapSlice{{Key: kv.Key, Value: kv.Value}}))
		}
	}
	if len(req.Sets) > 0 {
		b.WriteString("\nsets:")
		for _, kv := range req.Sets {
			if kv.Key == "" {
				continue
			}
			b.WriteString("\n" + indentLines(yamlLine(yaml.MapSlice{{Key: kv.Key, Value: kv.Values}}), "  "))
		}
	}
	b.WriteString("\nrules:")
	for i, st := range req.Steps {
		expr := strings.TrimSpace(st.Expression)
		if st.Tree != nil {
			generated, err := st.Tree.ToCEL()
			if err != nil {
				return "", fmt.Errorf("第 %d 步条件错误: %v", i+1, err)
			}
			expr = generated
		}
		if expr == "" {
			return "", fmt.Errorf("第 %d 步缺少条件（谓词树或 CEL 表达式）", i+1)
		}
		if _, iss := env.Compile(expr); iss.Err() != nil {
			return "", fmt.Errorf("第 %d 步 expression 编译失败: %v", i+1, iss.Err())
		}
		b.WriteString(strings.TrimSuffix(renderStepYAML(i, st, expr), "\n"))
	}
	// 未暴露顶级段（detail/groups/manual/transport/...）从打开时原文保真继承
	if extra := extraTopLevelSections(req.BaseYaml); extra != "" {
		b.WriteString("\n\n" + extra)
	}
	b.WriteString("\n")
	return b.String(), nil
}

// renderStepYAML 单步 rule → 缩进正确的 yaml 列表项块。
func renderStepYAML(index int, st putStep, expr string) string {
	var b strings.Builder
	b.WriteString("\n  - " + yamlLine(yaml.MapSlice{{Key: "method", Value: stepMethod(st.Method)}}))
	if st.Path != "" {
		b.WriteString("\n    " + yamlLine(yaml.MapSlice{{Key: "path", Value: st.Path}}))
	}
	if len(st.Headers) > 0 {
		b.WriteString("\n    headers:")
		keys := make([]string, 0, len(st.Headers))
		for k := range st.Headers {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if k == "" {
				continue
			}
			b.WriteString("\n      " + yamlLine(yaml.MapSlice{{Key: k, Value: st.Headers[k]}}))
		}
	}
	if st.Body != "" {
		b.WriteString("\n" + indentLines(yamlLine(yaml.MapSlice{{Key: "body", Value: st.Body}}), "    "))
	}
	if st.Search != "" {
		b.WriteString("\n    " + yamlLine(yaml.MapSlice{{Key: "search", Value: st.Search}}))
	}
	if st.FollowRedirects {
		b.WriteString("\n    follow_redirects: true")
	}
	if st.Continue {
		b.WriteString("\n    continue: true")
	}
	b.WriteString("\n" + indentLines(yamlLine(yaml.MapSlice{{Key: "expression", Value: expr}}), "    "))
	return b.String()
}

// stepMethod 步骤请求方法归一化（缺省 GET）。
func stepMethod(m string) string {
	if m = strings.ToUpper(strings.TrimSpace(m)); m == "" {
		return "GET"
	}
	return m
}

// yamlLine 借 yaml.Marshal 生成单个 "key: value" 完整行/块（转义与块格式由库保证，
// 手拼引号在 body/xml/正则等场景必翻车）。
func yamlLine(kv yaml.MapSlice) string {
	b, err := yaml.Marshal(kv)
	if err != nil {
		return ""
	}
	return strings.TrimRight(string(b), "\n")
}

// indentLines 整块缩进（首行与续行一致），列表项字段块用。
func indentLines(block, prefix string) string {
	lines := strings.Split(strings.TrimRight(block, "\n"), "\n")
	for i, ln := range lines {
		lines[i] = prefix + ln
	}
	return strings.Join(lines, "\n")
}

// formOwnedKeys 表单管辖的顶级 key——extraTopLevelSections 提取时排除。
var formOwnedKeys = map[string]bool{
	"name": true, "always": true, "set": true, "sets": true, "rules": true,
}

// topKeyRe 顶级 key 行（缩进 0 的 `key:` 形态）。
var topKeyRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_\-]*:`)

// extraTopLevelSections 提取 yaml 原文中表单不管的顶级段（含段体与注释行）：
// detail/groups/manual/transport 等重渲染无法保证零丢失的字段面，原文拼接保真。
func extraTopLevelSections(yamlText string) string {
	if strings.TrimSpace(yamlText) == "" {
		return ""
	}
	var out []string
	keep := false
	for _, ln := range strings.Split(yamlText, "\n") {
		if topKeyRe.MatchString(ln) {
			key := ln[:strings.IndexByte(ln, ':')]
			keep = !formOwnedKeys[key]
		}
		if keep && strings.TrimSpace(ln) != "" {
			out = append(out, ln)
		}
	}
	return strings.Join(out, "\n")
}

func (s *server) deletePoc(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "缺少 name", 400)
		return
	}
	found := false
	_ = filepath.Walk(s.pocsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if p, _ := lib.LoadPocbyPath(path); p != nil && p.Name == name {
			if err := os.Remove(path); err == nil {
				found = true
			}
		}
		return nil
	})
	if !found {
		http.Error(w, "外置目录未找到该 poc（内置 poc 不可删除）: "+name, 404)
		return
	}
	writeJSON(w, map[string]string{"status": "deleted"})
}

func (s *server) validateCEL(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Expression string `json:"expression"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if _, iss := s.celEnv.Compile(req.Expression); iss.Err() != nil {
		writeJSON(w, map[string]interface{}{"ok": false, "error": iss.Err().Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"ok": true})
}

// sanitizeName poc name → 安全文件名。
func sanitizeName(name string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9_\-]`)
	s := re.ReplaceAllString(strings.TrimPrefix(name, "poc-yaml-"), "-")
	if s == "" {
		s = "unnamed"
	}
	return s
}

// sortNames 供列表排序（保持输出稳定）。
func sortNames(items []pocItem) {
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
}
