// client_test.go
package client

import (
	"fmt"
	"testing"
	"time"
)

func TestClient(t *testing.T) {
	// 手工联调冒烟测试: 依赖外部 RDP 服务器, 不可达时跳过(原实现在登录失败后继续使用客户端导致 panic)
	c := NewClient("113.243.29.13:3389", "administrator", "Jhadmin123", TC_RDP, nil)
	err := c.Login()
	if err != nil {
		t.Skipf("外部 RDP 服务器不可达, 跳过: %v", err)
	}
	c.OnReady(func() {
		fmt.Println("ready")
	})
	time.Sleep(100 * time.Second)
}
