// pack 将指纹库明文 JSON 重新打包为 go:embed 引用的 .gz 文件（BL-POC-004 起为薄壳，
// 核心逻辑在 mylib/finger/packlib 共享包——与 fscanx-web 的"重打包"按钮同一实现）：
//
//	编辑 mylib/finger/resources 下的指纹明文 → go run ./mylib/finger/cmd/pack → go build .
package main

import (
	"fmt"
	"os"

	"github.com/killmonday/fscanx/mylib/finger/packlib"
)

func main() {
	// resDir 相对项目根，请在项目根目录运行本工具
	results, err := packlib.Run(".")
	for _, r := range results {
		fmt.Printf("✅ %s → %s (%d → %d bytes, %.1f%%)\n",
			r.Src, r.Dst, r.Raw, r.Gz, float64(r.Gz)*100/float64(r.Raw))
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ %v\n", err)
		os.Exit(1)
	}
	fmt.Println("打包完成，执行 go build . 生效")
}
