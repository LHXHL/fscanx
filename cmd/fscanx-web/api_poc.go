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
	Source string `json:"source"`         // built-in | external
	Always bool   `json:"always"`         // 无条件全打型标记（列表可视化）
	File   string `json:"file,omitempty"` // 文件名（内置=embed 真实文件名，内置源写回定位用）
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
	Name   string        `json:"name"`
	Source string        `json:"source"` // 本次定位来源 built-in | external（编辑上下文）
	File   string        `json:"file"`   // 文件名（内置=embed 真实文件名，内置源写回定位）
	Always bool          `json:"always"`
	Set    []kvView      `json:"set"`
	Sets   []kvListsView `json:"sets"`
	Steps  []stepView    `json:"steps"`
	Form   bool          `json:"form"` // 表单可承载（groups 等长尾形态为 false）
	Yaml   string        `json:"yaml"` // 原文（源码模式与段落保真基底）
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
					items = append(items, pocItem{Name: p.Name, Source: "built-in", Always: p.Always, File: n})
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
				items = append(items, pocItem{Name: p.Name, Source: "external", Always: p.Always, File: info.Name()})
			}
		}
		return nil
	})
	return
}

// findPocFile 按 name+source 定位 poc 文件，返回 yaml 原文、文件名与实际来源。
// source=external 只查外置；source=built-in 仓库内置源文件优先（repoRoot/PocScan/pocs，
// 那是下次编译的真实内容——写回后编辑器重载必须拿到新源而非 embed 旧版，防以陈旧
// 基底二次保存覆盖写回成果）、embed 兜底；source 为空保持旧行为（外置优先——与加载
// 覆盖语义一致）。
func (s *server) findPocFile(name, source string) (yamlText string, file string, src string, ok bool) {
	if source != "built-in" {
		_ = filepath.Walk(s.pocsDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info == nil || info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".yml") && !strings.HasSuffix(path, ".yaml") {
				return nil
			}
			if p, _ := lib.LoadPocbyPath(path); p != nil && p.Name == name {
				if raw, err := os.ReadFile(path); err == nil {
					yamlText, file, src, ok = string(raw), info.Name(), "external", true
				}
			}
			return nil
		})
		if ok || source == "external" {
			return
		}
	}
	embedFile, embedText, found := embedPocByName(name)
	if !found {
		return
	}
	file, src = embedFile, "built-in"
	if s.repoRoot != "" {
		if raw, err := os.ReadFile(s.builtinSourcePath(embedFile)); err == nil {
			var cur lib.Poc
			// 内容同一性：-root 指向其他版本检出时同文件名可能装的是别的 POC，
			// 解析不出同名 POC 就回落 embed，不给编辑器贴错标签
			if yaml.Unmarshal(raw, &cur) == nil && cur.Name == name {
				yamlText, ok = string(raw), true
				return
			}
		}
	}
	yamlText, ok = embedText, true
	return
}

// builtinSourcePath 内置源目录（-root 下 PocScan/pocs/）某文件的完整路径——读取与写回
// 共用同一拼装，防两处路径漂移。
func (s *server) builtinSourcePath(file string) string {
	return filepath.Join(s.repoRoot, "PocScan", "pocs", file)
}

// embedPocByName 在内置 embed 库按 name 查找，返回真实文件名与原文（写回内置源时
// 服务端解析目标文件名——不信任客户端传路径，name 已在落盘前与 yaml 校验一致）。
func embedPocByName(name string) (file, yamlText string, ok bool) {
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
				return n, string(raw), true
			}
			return
		}
	}
	return
}

func (s *server) getPoc(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		httpErrorf(w, "缺少 name")
		return
	}
	yamlText, file, src, ok := s.findPocFile(name, r.URL.Query().Get("source"))
	if !ok {
		httpErrorf(w, "未找到 poc: %s（外置行可能已被外部删除）", name)
		return
	}
	d := pocFromYaml(name, yamlText, s)
	d.Source, d.File = src, file
	writeJSON(w, d)
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
	Mode   string `json:"mode"`   // form | raw
	Target string `json:"target"` // 保存目标：built-in（写回内置源，下次编译生效）| external（默认，外置覆盖立即生效）
	Name   string `json:"name"`
	File   string `json:"file"` // 打开时的文件名（target=built-in 时作改名守卫：与 name 解析出的文件不一致即拒）
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
		httpErrorf(w, "name 不能为空")
		return
	}
	var yamlText string
	switch req.Mode {
	case "form":
		text, err := s.renderPocYAML(&req)
		if err != nil {
			httpErrorf(w, "%s", err.Error())
			return
		}
		yamlText = text
	case "raw":
		yamlText = req.Yaml
	default:
		httpErrorf(w, "mode 须为 form 或 raw")
		return
	}
	// 落盘前双重校验：可解析为 Poc + name 一致 + 每条 expression 编译通过
	var p lib.Poc
	if err := yaml.Unmarshal([]byte(yamlText), &p); err != nil {
		httpErrorf(w, "yaml 解析失败: %v", err)
		return
	}
	if p.Name == "" || p.Name != req.Name {
		httpErrorf(w, "yaml 的 name 为空或与请求 name 不一致")
		return
	}
	// groups 形态（31/386，无顶层 rules）同样放行保存——只拦两种形态皆空
	if len(p.Rules) == 0 && len(p.Groups) == 0 {
		httpErrorf(w, "rules/groups 均为空")
		return
	}
	env, err := s.envWithSetVars(p.Set)
	if err != nil {
		httpErrorf(w, "构造变量环境失败: %v", err)
		return
	}
	for i, rule := range p.Rules {
		if strings.TrimSpace(rule.Expression) == "" {
			httpErrorf(w, "第 %d 条 rule 缺少 expression", i+1)
			return
		}
		if _, iss := env.Compile(rule.Expression); iss.Err() != nil {
			httpErrorf(w, "第 %d 条 rule 的 expression 编译失败: %v", i+1, iss.Err())
			return
		}
	}
	// groups 内 rule 同样过编译校验（保持"落盘前校验"语义完整）
	for _, gi := range p.Groups {
		for i, rule := range gi.Value {
			if strings.TrimSpace(rule.Expression) == "" {
				httpErrorf(w, "groups %s 第 %d 条 rule 缺少 expression", gi.Key, i+1)
				return
			}
			if _, iss := env.Compile(rule.Expression); iss.Err() != nil {
				httpErrorf(w, "groups %s 第 %d 条 rule 的 expression 编译失败: %v", gi.Key, i+1, iss.Err())
				return
			}
		}
	}
	// 保存目标：external=外置覆盖（默认，立即生效）；built-in=写回内置源（下次编译生效）。
	// 内置目标文件名由 embed 库按（已与 yaml 校验一致的）name 服务端解析，且必须已存在——
	// 防 -root 指向其他检出时凭空造文件。错误走 httpErrorf 信封（BL-POC-005 已验证：
	// 非 2xx 纯文本被前端 axios 丢弃，指导性提示到不了用户）。
	var targetPath string
	switch req.Target {
	case "", "external":
		if err := os.MkdirAll(s.pocsDir, 0755); err != nil {
			httpErrorf(w, "创建外置目录失败: %v", err)
			return
		}
		targetPath = filepath.Join(s.pocsDir, sanitizeName(req.Name)+".yml")
	case "built-in":
		if s.repoRoot == "" {
			httpErrorf(w, "未配置 -root，无法写回内置源，仅可保存到外置 pocs/")
			return
		}
		embedFile, _, found := embedPocByName(p.Name)
		if found {
			// embed 已有该名：编辑既有内置 POC——改名守卫（打开 A 文件却写进 B 的内置源文件 → 拒）
			if req.File != "" && req.File != embedFile {
				httpErrorf(w, "检测到重命名（%s → %s），既有内置源写回不支持改名，请保存到外置 pocs/", req.File, embedFile)
				return
			}
			targetPath = s.builtinSourcePath(embedFile)
			if _, err := os.Stat(targetPath); err != nil {
				httpErrorf(w, "内置源文件不存在: %s（-root 未指向本仓库或版本不一致），仅可保存到外置 pocs/", targetPath)
				return
			}
			// 内容同一性守卫：-root 指向其他版本检出时同文件名可能装的是别的 POC，覆盖即改错文件
			if raw, err := os.ReadFile(targetPath); err == nil {
				var cur lib.Poc
				if err := yaml.Unmarshal(raw, &cur); err != nil || cur.Name == "" || cur.Name != p.Name {
					httpErrorf(w, "内置源文件 %s 当前内容解析异常或为其他 POC（name=%q），为避免覆盖错误文件已拒绝；可保存到外置 pocs/", targetPath, cur.Name)
					return
				}
			}
		} else {
			// embed 无此名：新建 / 外挂转内置 / 改名转内置——目标按 name 解析仓库源文件：
			// 不存在 = 创建（下次编译随版本分发）；存在且 name 一致 = 编辑未编译的仓库文件
			//（getPoc built-in 读仓库最新源同语义）；存在但 name 不一致 = 拒绝（防覆盖错文件）
			targetPath = s.builtinSourcePath(sanitizeName(p.Name) + ".yml")
			if raw, err := os.ReadFile(targetPath); err == nil {
				var cur lib.Poc
				if err := yaml.Unmarshal(raw, &cur); err != nil || cur.Name == "" || cur.Name != p.Name {
					httpErrorf(w, "内置源目录已存在同名文件 %s 且内容是其他 POC（name=%q），为避免覆盖错误文件已拒绝；可保存到外置 pocs/", targetPath, cur.Name)
					return
				}
			}
		}
	default:
		httpErrorf(w, "target 须为 built-in 或 external")
		return
	}
	// temp+rename 原子写（防中途崩溃留撕裂 yaml 进 embed/加载；与 BL-POC-006 指纹快照同约定）
	tmp := targetPath + ".fscanx-web.tmp"
	if err := os.WriteFile(tmp, []byte(yamlText), 0644); err != nil {
		httpErrorf(w, "写盘失败: %v", err)
		return
	}
	if err := os.Rename(tmp, targetPath); err != nil {
		_ = os.Remove(tmp)
		httpErrorf(w, "落盘失败: %v", err)
		return
	}
	writeJSON(w, map[string]string{"status": "saved", "file": targetPath, "target": req.Target})
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
		httpErrorf(w, "缺少 name")
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
		httpErrorf(w, "外置目录未找到该 poc（内置 poc 不可删除）: %s", name)
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
