package favicon

import "testing"

// TestHashMatchFocus 验证 focus 指纹经 md5/mmh3 哈希命中后 IsFocus 置位，非 focus 不置位（BL-FOC-005）
func TestHashMatchFocus(t *testing.T) {
	engine := NewFavicons()
	engine.Md5Fingers["d41d8cd98f00b204e9800998ecf8427e"] = "FocusProduct"
	engine.Mmh3Fingers["116323821"] = "PlainProduct"
	engine.FocusNames["FocusProduct"] = true

	md5Frame := engine.HashMatch("d41d8cd98f00b204e9800998ecf8427e", "")
	if md5Frame == nil || md5Frame.Name != "FocusProduct" {
		t.Fatalf("md5 路径未命中：%v", md5Frame)
	}
	if !md5Frame.IsFocus {
		t.Error("md5 路径 focus 指纹命中后 IsFocus 应为 true")
	}

	// mmh3 路径（md5 不命中时走 mmh3）
	mmh3Frame := engine.HashMatch("no-such-md5", "116323821")
	if mmh3Frame == nil || mmh3Frame.Name != "PlainProduct" {
		t.Fatalf("mmh3 路径未命中：%v", mmh3Frame)
	}
	if mmh3Frame.IsFocus {
		t.Error("非 focus 指纹命中后 IsFocus 应为 false")
	}
}
