package nla_test

import (
	"encoding/hex"
	"testing"

	"github.com/killmonday/fscanx/mylib/grdp/protocol/nla"
)

func TestEncodeDERTRequest(t *testing.T) {
	// BL-RDP-001 裁定: 旧期望(32 字节)源自上游无 Version 字段的旧实现; 现序列化含 NTLMSSP Version
	// 字段(+8 字节 DER 包装), 已被 Win11 真机 NLA 认证成功实证接受(见 09 backlog 状态记录)
	ntlm := nla.NewNTLMv2("", "", "")
	result := nla.EncodeDERTRequest([]nla.Message{ntlm.GetNegotiateMessage()}, []byte(""), []byte(""))
	if hex.EncodeToString(result) != "3037a003020102a130302e302ca02a04284e544c4d535350000100000035820860000000000000000000000000000000000000000000000000" {
		t.Error("not equal")
	}
}
