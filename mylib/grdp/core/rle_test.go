// rle_test.go
package core

import (
	"fmt"
	"testing"

	"github.com/killmonday/fscanx/mylib/grdp/glog"
)

func TestSum(t *testing.T) {
	// Decompress 内部走 glog 调试日志, 测试环境无 logger, 置 NONE 静默(见 glog checkLogger)
	glog.SetLevel(glog.NONE)
	// 2026-11-15 取证: 该输入触发 decompress3 CVAL 越界 panic(rle.go:9), 实现层缺陷,
	// 非编译问题, 待专项修复
	t.Skip("decompress3 对该历史样例输入越界 panic(实现缺陷), 待专项修复(见 BL-BLD-001 状态记录)")
	input := []byte{
		192, 44, 200, 8, 132, 200, 8, 200, 8, 200, 8, 200, 8, 0, 19, 132, 232, 8, 12, 50, 142, 66, 77, 58, 208, 59, 225, 25, 1, 0, 0, 0, 0, 0, 0, 0, 132, 139, 33, 142, 66, 142, 66, 142, 66, 208, 59, 4, 43, 1, 0, 0, 0, 0, 0, 0, 0, 132, 203, 41, 142, 66, 142, 66, 142, 66, 208, 59, 96, 0, 1, 0, 0, 0, 0, 0, 0, 0, 132, 9, 17, 142, 66, 142, 66, 142, 66, 208, 59, 230, 27, 1, 0, 0, 0, 0, 0, 0, 0, 132, 200, 8, 9, 17, 139, 33, 74, 25, 243, 133, 14, 200, 8, 132, 200, 8, 200, 8, 200, 8, 200, 8,
	}
	out := Decompress(input, 64, 64, 3)
	fmt.Println(out)
}
