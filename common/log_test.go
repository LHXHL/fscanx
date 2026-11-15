package common

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/fatih/color"
)

// captureColor 重定向 fatih/color 输出到缓冲并强制保留色码（BL-FOC-006）：
// go test 下 stdout 非 TTY，库自动置 NoColor=true 剥掉全部 ANSI 码，断言会空转。
func captureColor(t *testing.T, fn func()) string {
	t.Helper()
	oldOut, oldNo := color.Output, color.NoColor
	color.Output = &bytes.Buffer{}
	color.NoColor = false
	os.Unsetenv("NO_COLOR")
	defer func() {
		color.Output, color.NoColor = oldOut, oldNo
	}()
	fn()
	return color.Output.(*bytes.Buffer).String()
}

// 无 focus 的指纹段必须走原 Println 路径，输出与旧渲染逐字节一致（Contains 门控零变化）
func TestPrintFpsTailNoFocus(t *testing.T) {
	white := color.New(color.FgWhite)
	red := color.New(color.FgRed)
	got := captureColor(t, func() {
		printFpsTail("[php], [nginx], [Cert:cn]", white, red)
	})
	if want := "\x1b[37m[php], [nginx], [Cert:cn]\n\x1b[0m"; got != want {
		t.Fatalf("无 focus 段输出与旧渲染不一致:\n got %q\nwant %q", got, want)
	}
}

// focus token（[* 前缀）被红码精确包裹，普通 token 与分隔符保持底色
func TestPrintFpsTailFocusRed(t *testing.T) {
	white := color.New(color.FgWhite)
	red := color.New(color.FgRed)
	got := captureColor(t, func() {
		printFpsTail("[php], [*若依-RuoYi], [Cert:cn]", white, red)
	})
	if !strings.Contains(got, "\x1b[31m[*若依-RuoYi]\x1b[0m") {
		t.Fatalf("focus token 未被红色码包裹: %q", got)
	}
	if n := strings.Count(got, "\x1b[31m"); n != 1 {
		t.Fatalf("红码出现 %d 次，期望 1: %q", n, got)
	}
	if !strings.Contains(got, "\x1b[37m[php]\x1b[0m") || !strings.Contains(got, "\x1b[37m[Cert:cn]\x1b[0m") {
		t.Fatalf("普通 token 底色被改变: %q", got)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Fatalf("行尾缺换行: %q", got)
	}
}

// focus 在段首/段尾边界（含 cyan 交替分支、带版本形态）
func TestPrintFpsTailFocusEdges(t *testing.T) {
	cyan := color.New(color.FgCyan)
	red := color.New(color.FgRed)
	got := captureColor(t, func() {
		printFpsTail("[*shiro:1.2.4], [nginx]", cyan, red)
	})
	if !strings.HasPrefix(got, "\x1b[31m[*shiro:1.2.4]\x1b[0m") {
		t.Fatalf("段首 focus token 未红: %q", got)
	}
	got = captureColor(t, func() {
		printFpsTail("[nginx], [*若依管理系统]", cyan, red)
	})
	if !strings.HasSuffix(got, "\x1b[31m[*若依管理系统]\x1b[0m\n") {
		t.Fatalf("段尾 focus token 未红: %q", got)
	}
}

// 3 段 DC banner 行（portscan.go:201）：Join(slice[3:]) 得空串，走原路径不 panic
func TestPrintFpsTailEmptySeg(t *testing.T) {
	white := color.New(color.FgWhite)
	red := color.New(color.FgRed)
	got := captureColor(t, func() {
		printFpsTail("", white, red)
	})
	if want := "\x1b[37m\n\x1b[0m"; got != want {
		t.Fatalf("空段输出异常:\n got %q\nwant %q", got, want)
	}
}
