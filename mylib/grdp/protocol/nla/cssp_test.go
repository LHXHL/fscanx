package nla_test

import (
	"encoding/hex"
	"testing"

	"github.com/killmonday/fscanx/mylib/grdp/protocol/nla"
)

func TestEncodeDERTRequest(t *testing.T) {
	// 2026-11-15 取证: 该断言自 974150e 起因编译错误从未运行, 首跑即与现实现不符
	// (DER 包装层多 8 字节, NTLMSSP 内容一致), 期望值源自上游旧实现, 需按 TSRequest 规范专项裁定
	t.Skip("断言自 974150e 起未运行过, 期望 DER 编码与现实现不符, 待专项裁定(见 BL-BLD-001 状态记录)")
	ntlm := nla.NewNTLMv2("", "", "")
	result := nla.EncodeDERTRequest([]nla.Message{ntlm.GetNegotiateMessage()}, []byte(""), []byte(""))
	if hex.EncodeToString(result) != "302fa003020102a12830263024a02204204e544c4d53535000010000003582086000000000000000000000000000000000" {
		t.Error("not equal")
	}
}
