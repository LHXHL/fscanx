package main

// 表单渲染 round-trip 与段落保真测试（BL-POC-004 多步结构化迭代轮）：
// renderPocYAML 产物必须能被 yaml.Unmarshal 解析回等价结构化字段，
// 且 BaseYaml 中未暴露顶级段（detail/groups/manual）逐行保真。

import (
	"strings"
	"testing"

	"github.com/killmonday/fscanx/PocScan/lib"
	"gopkg.in/yaml.v2"
)

func testRenderReq() (*putPocReq, *server) {
	req := &putPocReq{
		Mode:   "form",
		Name:   "poc-yaml-render-test",
		Always: true,
		Set:    []kvView{{Key: "rand", Value: "randomInt(1, 100)"}},
		Steps: []putStep{
			{
				Method: "get",
				Path:   "/first?id={{rand}}",
				Headers: map[string]string{
					"X-Test":  "1",
					"Content-Type": "text/xml",
				},
				FollowRedirects: true,
				Tree: &Node{Logic: "and", Conditions: []*Node{
					{Target: "status", Op: "eq", Value: "200"},
					{Target: "body", Op: "contains", Value: "marker-1"},
				}},
			},
			{
				Method: "POST",
				Path:   "/second",
				Body:   "a=1&b=<xml>'quote'\"dq\"",
				// 超出谓词子集：CEL 原文兜底（bytes/md5 等函数调用）
				Expression: `response.body.bcontains(bytes(md5(string(rand))))`,
			},
		},
		BaseYaml: `name: poc-yaml-render-test
manual: true
rules:
  - method: GET
    path: /old
    expression: response.status == 200
groups:
  g1:
  - method: GET
    path: /g
    expression: response.status == 200
detail:
  author: original-author
  links:
    - https://example.com
`,
	}
	env, err := newCelEnv()
	if err != nil {
		panic(err)
	}
	return req, &server{celEnv: env}
}

func TestRenderPocYAML(t *testing.T) {
	req, s := testRenderReq()
	text, err := s.renderPocYAML(req)
	if err != nil {
		t.Fatal(err)
	}

	var p lib.Poc
	if err := yaml.Unmarshal([]byte(text), &p); err != nil {
		t.Fatalf("rendered yaml not parsable: %v\n---\n%s", err, text)
	}
	if p.Name != "poc-yaml-render-test" || !p.Always {
		t.Fatalf("name/always mismatch: %+v", p)
	}
	if len(p.Set) != 1 || p.Set[0].Key != "rand" || p.Set[0].Value != "randomInt(1, 100)" {
		t.Fatalf("set mismatch: %+v", p.Set)
	}
	if len(p.Rules) != 2 {
		t.Fatalf("want 2 rules got %d\n%s", len(p.Rules), text)
	}
	r1, r2 := p.Rules[0], p.Rules[1]
	if r1.Method != "GET" || r1.Path != "/first?id={{rand}}" || !r1.FollowRedirects {
		t.Fatalf("rule1 mismatch: %+v", r1)
	}
	if r1.Headers["Content-Type"] != "text/xml" || r1.Headers["X-Test"] != "1" {
		t.Fatalf("rule1 headers mismatch: %+v", r1.Headers)
	}
	if r1.Expression != "(response.status == 200 && response.body.bcontains(b\"marker-1\"))" {
		t.Fatalf("rule1 expression mismatch: %q", r1.Expression)
	}
	if r2.Method != "POST" || r2.Body != "a=1&b=<xml>'quote'\"dq\"" {
		t.Fatalf("rule2 mismatch: %+v", r2)
	}
	if r2.Expression != `response.body.bcontains(bytes(md5(string(rand))))` {
		t.Fatalf("rule2 expression mismatch: %q", r2.Expression)
	}
	// 未暴露段保真：groups/manual 完整继承，detail 逐行保留
	if len(p.Groups) != 1 || !strings.Contains(text, "manual: true") {
		t.Fatalf("groups/manual not preserved:\n%s", text)
	}
	if !strings.Contains(text, "author: original-author") || !strings.Contains(text, "- https://example.com") {
		t.Fatalf("detail not preserved:\n%s", text)
	}
	// 渲染产物不应残留旧 rules
	if strings.Contains(text, "/old") {
		t.Fatalf("old rules leaked:\n%s", text)
	}
}

func TestExtraTopLevelSections(t *testing.T) {
	base := `name: x
always: true
set:
  a: b
rules:
  - method: GET
    path: /p
    expression: response.status == 200
transport: http
detail:
  author: me
`
	extra := extraTopLevelSections(base)
	if strings.Contains(extra, "rules") || strings.Contains(extra, "name") || strings.Contains(extra, "set:") {
		t.Fatalf("form-owned keys leaked: %s", extra)
	}
	if !strings.Contains(extra, "transport: http") || !strings.Contains(extra, "author: me") {
		t.Fatalf("extra sections lost: %s", extra)
	}
	if extraTopLevelSections("") != "" {
		t.Fatal("empty base should be empty")
	}
}
