package gonmap

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/killmonday/fscanx/mylib/finger/common"
	"github.com/killmonday/fscanx/mylib/finger/resources"
)

func TestScan(t *testing.T) {
	// nmap probes 数据未嵌入时跳过(974150e 内存优化裁剪, 主程序仅启用 fingers+goby)
	if len(resources.NmapServiceProbesData) == 0 {
		t.Skip("nmap 引擎数据未嵌入(974150e 内存裁剪), 跳过")
	}

	host := "127.0.0.1" // 使用本地地址避免网络问题
	engine, err := NewNmapEngine()
	if err != nil {
		t.Fatal(err)
	}
	sender := common.NewServiceSender(3 * time.Second)

	testPorts := []int{22, 80, 138, 135, 3389, 1080, 443, 445, 1521}
	for _, port := range testPorts {
		result := engine.ServiceMatch(host, strconv.Itoa(port), 3, sender, nil)
		if result != nil && result.Framework != nil {
			fmt.Printf("Port %d: %s\n", port, result.Framework.String())
		} else {
			fmt.Printf("Port %d: no service detected\n", port)
		}
	}
}
