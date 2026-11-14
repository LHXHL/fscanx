package common

import "testing"

// TestFrameworkString 验证指纹输出形态, 重点关注标记为 ASCII 星号(全终端宽度恒定)
func TestFrameworkString(t *testing.T) {
	focusFrame := func(name, version string) *Framework {
		frame := NewFramework(name, FrameFromFingers)
		frame.IsFocus = true
		if version != "" {
			frame.Version = version
		}
		return frame
	}

	tests := []struct {
		name  string
		frame *Framework
		want  string
	}{
		{"普通指纹", NewFramework("nginx", FrameFromFingers), "[nginx]"},
		{"重点关注指纹", focusFrame("shiro", ""), "[*shiro]"},
		{"重点关注带版本", focusFrame("shiro", "1.2.4"), "[*shiro:1.2.4]"},
		{"版本中的冒号替换为下划线", focusFrame("shiro", "1:2:3"), "[*shiro:1_2_3]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.frame.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}
