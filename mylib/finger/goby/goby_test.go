package goby

import (
	"encoding/json"
	"testing"
)

// newTestFinger 构造最小可匹配的 goby 指纹: 单条件 label=a, logic 为 label 布尔表达式("a")
func newTestFinger(name string, focus bool, feature string) *GobyFinger {
	return &GobyFinger{
		Logic: "a",
		Name:  name,
		Focus: focus,
		Rule: []gobyRule{
			{Label: "a", Feature: feature, IsEquel: true},
		},
	}
}

// TestFocusMatch 验证 focus 指纹命中后 IsFocus 置位, 非 focus 指纹不置位
func TestFocusMatch(t *testing.T) {
	engine := &GobyEngine{Fingers: []*GobyFinger{
		newTestFinger("Apache-Shiro", true, "rememberme=deleteme"),
		newTestFinger("nginx", false, "nginx"),
	}}
	if err := engine.Compile(); err != nil {
		t.Fatalf("compile: %v", err)
	}

	content := []byte("HTTP/1.1 200 OK\r\nSet-Cookie: rememberme=deleteme\r\nServer: nginx\r\n\r\n")
	frames := engine.WebMatch(content)

	shiro := frames["apache-shiro"] // Frameworks.Add 会将 name 转小写
	if shiro == nil {
		t.Fatal("focus 指纹未命中")
	}
	if !shiro.IsFocus {
		t.Errorf("focus 指纹命中后 IsFocus 应为 true, got false")
	}

	nginx := frames["nginx"]
	if nginx == nil {
		t.Fatal("普通指纹未命中")
	}
	if nginx.IsFocus {
		t.Errorf("无 focus 字段指纹 IsFocus 应为 false, got true")
	}
}

// TestFocusJSONField 验证 JSON 数据中 focus 字段的解析与旧格式(无 focus 字段)兼容
func TestFocusJSONField(t *testing.T) {
	var fingers []*GobyFinger
	data := `[
		{"name":"Apache-Shiro","focus":true,"logic":"a","rule":[{"label":"a","feature":"rememberme","is_equal":true}]},
		{"name":"nginx","logic":"a","rule":[{"label":"a","feature":"nginx","is_equal":true}]}
	]`
	if err := json.Unmarshal([]byte(data), &fingers); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(fingers) != 2 {
		t.Fatalf("expect 2 fingers, got %d", len(fingers))
	}
	if !fingers[0].Focus {
		t.Errorf(`"focus": true 条目解析后 Focus 应为 true`)
	}
	if fingers[1].Focus {
		t.Errorf("无 focus 字段条目(旧格式)解析后 Focus 应为 false")
	}
}
