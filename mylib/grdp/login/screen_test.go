package login

import (
	"testing"

	"github.com/killmonday/fscanx/mylib/grdp/glog"
)

// TestBpp 验证位深→每像素字节数映射(FreeRDP 规范: 8→1/15,16→2/24→3/32→4, 未知→0)
// BL-RDP-002: 此前 15bpp 误映射 1、8bpp 无分支
func TestBpp(t *testing.T) {
	// 未知位深用例走 glog.Error, 测试环境无 logger, 置 NONE 静默(见 glog checkLogger)
	glog.SetLevel(glog.NONE)
	cases := []struct {
		bpp  uint16
		want int
	}{
		{8, 1}, {15, 2}, {16, 2}, {24, 3}, {32, 4},
		{4, 0}, {1, 0}, {0, 0},
	}
	for _, c := range cases {
		if got := Bpp(c.bpp); got != c.want {
			t.Errorf("Bpp(%d) = %d, want %d", c.bpp, got, c.want)
		}
	}
}

// TestToRGBA 按原始位深解码像素(15/16bpp 全 1 应为白色, 8bpp 灰度近似, 24bpp BGR 序)
func TestToRGBA(t *testing.T) {
	// 15bpp RGB555: 0x7FFF(大端字节序 0x7F,0xFF) = 三通道满 → 白
	r, g, b, a := ToRGBA(15, 0, []byte{0x7F, 0xFF})
	if r != 255 || g != 255 || b != 255 || a != 255 {
		t.Errorf("ToRGBA(15, white) = %d,%d,%d,%d want 255,255,255,255", r, g, b, a)
	}
	// 16bpp RGB565: 0xFFFF = 白
	r, g, b, _ = ToRGBA(16, 0, []byte{0xFF, 0xFF})
	if r != 255 || g != 255 || b != 255 {
		t.Errorf("ToRGBA(16, white) = %d,%d,%d want 255,255,255", r, g, b)
	}
	// 8bpp 调色板索引灰度近似
	r, g, b, _ = ToRGBA(8, 0, []byte{0x80})
	if r != 0x80 || g != 0x80 || b != 0x80 {
		t.Errorf("ToRGBA(8, 0x80) = %d,%d,%d want 0x80,0x80,0x80", r, g, b)
	}
	// 24bpp BGR 字节序: data[i+2]=R, data[i+1]=G, data[i]=B
	r, g, b, _ = ToRGBA(24, 0, []byte{0x11, 0x22, 0x33})
	if r != 0x33 || g != 0x22 || b != 0x11 {
		t.Errorf("ToRGBA(24, BGR) = %d,%d,%d want 0x33,0x22,0x11", r, g, b)
	}
}
