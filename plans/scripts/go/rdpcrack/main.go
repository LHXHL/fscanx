// rdpcrack: RDP NLA 爆破式探针(BL-RDP-001), 走主程序同款 login.RdpCrack + HYBRID(NLA) 协议,
// 正确口令应 true、错误口令应 false——双对照裁定 NLA 认证正确性
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/killmonday/fscanx/mylib/grdp/glog"
	"github.com/killmonday/fscanx/mylib/grdp/login"
	"github.com/killmonday/fscanx/mylib/grdp/protocol/x224"
)

func main() {
	host := "192.168.111.76:3389"
	user := "alice"
	pass := "baidu.com@123"
	if len(os.Args) >= 4 {
		host, user, pass = os.Args[1], os.Args[2], os.Args[3]
	}
	var protocol uint32 = x224.PROTOCOL_HYBRID // NLA(CredSSP/NTLMv2) 路径
	if len(os.Args) >= 5 && os.Args[4] == "ssl" {
		protocol = x224.PROTOCOL_SSL
	}

	// 主程序爆破为静默日志, 探针开 INFO 观察判定链
	glog.SetLevel(glog.INFO)
	glog.SetLogger(log.New(os.Stdout, "", log.Ltime))

	fmt.Printf("=== crack probe %s user=%s protocol=%d ===\n", host, user, protocol)
	flag, err := login.RdpCrack(host, "", user, pass, 5, protocol)
	fmt.Printf("CRACK_RESULT: flag=%v err=%v\n", flag, err)
}
