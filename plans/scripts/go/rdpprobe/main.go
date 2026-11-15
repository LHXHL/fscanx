// rdpprobe: RDP 登录探针(BL-RDP-001), TRACE 级日志观察握手每一步
package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/killmonday/fscanx/mylib/grdp/client"
	"github.com/killmonday/fscanx/mylib/grdp/glog"
)

func main() {
	host := "192.168.111.76:3389"
	user := "alice"
	pass := "baidu.com@123"
	if len(os.Args) >= 4 {
		host, user, pass = os.Args[1], os.Args[2], os.Args[3]
	}

	glog.SetLevel(glog.TRACE) // 握手细节全开
	glog.SetLogger(log.New(os.Stdout, "", log.Ltime))

	fmt.Printf("=== probe %s user=%s ===\n", host, user)
	c := client.NewClient(host, user, pass, client.TC_RDP, nil)

	done := make(chan error, 1)
	go func() { done <- c.Login() }()

	select {
	case err := <-done:
		if err != nil {
			fmt.Println("LOGIN_RESULT: FAIL:", err)
		} else {
			fmt.Println("LOGIN_RESULT: SUCCESS")
		}
	case <-time.After(45 * time.Second):
		fmt.Println("LOGIN_RESULT: TIMEOUT(45s)")
	}
}
