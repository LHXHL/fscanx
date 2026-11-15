// rdpshot: RDP 截屏探针(BL-RDP-002), 截图落 OutputDir 后人工/视觉校验黑块
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
	outdir := "./rdpshots"
	if len(os.Args) >= 4 {
		host, user, pass = os.Args[1], os.Args[2], os.Args[3]
	}
	if len(os.Args) >= 5 {
		outdir = os.Args[4]
	}
	var protocol uint32 = x224.PROTOCOL_HYBRID
	if len(os.Args) >= 6 && os.Args[5] == "ssl" {
		protocol = x224.PROTOCOL_SSL
	}
	_ = os.MkdirAll(outdir, 0o755)

	login.OutputDir = outdir
	login.LogLever = glog.INFO // NewClient 会用该包变量覆盖全局级别
	glog.SetLogger(log.New(os.Stdout, "", log.Ltime))

	fmt.Printf("=== shot probe %s user=%q out=%s protocol=%d ===\n", host, user, outdir, protocol)
	flag, err := login.RdpConn(host, "", user, pass, 6, protocol)
	fmt.Printf("SHOT_RESULT: flag=%v err=%v\n", flag, err)
}
