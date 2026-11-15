package main

import (
	"testing"

	"github.com/google/cel-go/cel"
)

// mustEnv 与 poc 求值同源的 CEL 环境（事实校验用：生成的表达式必须能编译）。
func mustEnv(t *testing.T) *cel.Env {
	t.Helper()
	env, err := newCelEnv()
	if err != nil {
		t.Fatal(err)
	}
	return env
}

// assertCompiles 表达式必须在 lib 求值环境编译通过。
func assertCompiles(t *testing.T, env *cel.Env, expr string) {
	t.Helper()
	if _, iss := env.Compile(expr); iss.Err() != nil {
		t.Fatalf("compile %q: %v", expr, iss.Err())
	}
}

// TestToCEL 表单四类条件 + 嵌套组的序列化与编译可过（BL-POC-004）。
func TestToCEL(t *testing.T) {
	env := mustEnv(t)

	cases := []struct {
		name string
		tree *Node
		want string
	}{
		{"状态码", &Node{Target: "status", Op: "eq", Value: "200"}, "response.status == 200"},
		{"体包含", &Node{Target: "body", Op: "contains", Value: "root"}, `response.body.bcontains(b"root")`},
		{"体正则", &Node{Target: "body", Op: "regex", Value: `admin.*pass`}, `"admin.*pass".bmatches(response.body)`},
		{"header包含", &Node{Target: "header", Key: "Content-Type", Op: "contains", Value: "json"},
			`response.headers["Content-Type"].contains("json")`},
		{"AND组合", &Node{Logic: "and", Conditions: []*Node{
			{Target: "status", Op: "eq", Value: "200"},
			{Target: "body", Op: "contains", Value: "ok"},
		}}, `(response.status == 200 && response.body.bcontains(b"ok"))`},
		{"嵌套OR", &Node{Logic: "and", Conditions: []*Node{
			{Target: "status", Op: "eq", Value: "200"},
			{Logic: "or", Conditions: []*Node{
				{Target: "body", Op: "contains", Value: "a"},
				{Target: "body", Op: "regex", Value: "b"},
			}},
		}}, `(response.status == 200 && (response.body.bcontains(b"a") || "b".bmatches(response.body)))`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := c.tree.ToCEL()
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Fatalf("want %q got %q", c.want, got)
			}
			assertCompiles(t, env, got)
		})
	}

	// 非法输入必须报错（落盘前拦截的依据）
	for _, bad := range []*Node{
		{Target: "status", Op: "eq", Value: "abc"},
		{Target: "header", Op: "contains", Value: "x"}, // 缺 header 名
		{Target: "title", Op: "contains", Value: "x"},  // 求值环境无 title
		{Logic: "xor", Conditions: []*Node{{Target: "status", Op: "eq", Value: "200"}}},
		{Logic: "and"}, // 空组
	} {
		if _, err := bad.ToCEL(); err == nil {
			t.Fatalf("want error for %+v", bad)
		}
	}
}

// TestParsePredicate round-trip（树→CEL→树等价）与子集外表达式返回 nil。
func TestParsePredicate(t *testing.T) {
	env := mustEnv(t)

	trees := []*Node{
		{Target: "status", Op: "eq", Value: "200"},
		{Target: "body", Op: "contains", Value: "若依"},
		{Target: "header", Key: "Server", Op: "contains", Value: "nginx"},
		{Logic: "and", Conditions: []*Node{
			{Target: "status", Op: "eq", Value: "200"},
			{Logic: "or", Conditions: []*Node{
				{Target: "body", Op: "contains", Value: "a"},
				{Target: "body", Op: "regex", Value: `v\d+\.`},
			}},
		}},
	}
	for _, tr := range trees {
		expr, err := tr.ToCEL()
		if err != nil {
			t.Fatal(err)
		}
		back := ParsePredicate(expr, env)
		if back == nil {
			t.Fatalf("round-trip nil for %q", expr)
		}
		again, err := back.ToCEL()
		if err != nil {
			t.Fatal(err)
		}
		if again != expr {
			t.Fatalf("round-trip mismatch: %q vs %q", expr, again)
		}
	}

	// 子集外/非法：返回 nil（前端切高级源码模式）
	for _, src := range []string{
		`response.url.scheme == "http"`,                          // 表单不支持的 select
		`response.status == 200 && response.body.bcontains(b"a"`, // 语法错误
		`response.status != 200`,                                 // 不支持的操作符
		`payload == reverse.url`,                                 // 变量引用（多步利用链）
		``,
	} {
		if got := ParsePredicate(src, env); got != nil {
			t.Fatalf("want nil for %q got %+v", src, got)
		}
	}
}
