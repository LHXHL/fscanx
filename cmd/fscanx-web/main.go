// fscanx-web 是 fscanx 的本地维护端（BL-POC-004，独立二进制，扫描器本体不膨胀）：
// poc 表单化编辑（谓词树生成 CEL）、绑定关系编辑、指纹明文编辑与一键重打包。
//
// 启动：fscanx-web [-addr 127.0.0.1:7758] [-root <仓库根>]
//   - poc/binds 读写扫描器部署目录（二进制旁 pocs/ 与 binds.yaml，运行时生效零编译）
//   - 指纹明文与重打包读写仓库根下 mylib/finger/resources（产物供下次 go build）
package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"

	"github.com/google/cel-go/cel"
)

//go:embed all:static
var staticFS embed.FS

// server 维护端状态：目录定位与 CEL 环境。
type server struct {
	repoRoot   string    // fscanx 仓库根（指纹明文、pack 目标）
	pocsDir    string    // 扫描器部署目录的外置 poc 目录（二进制旁 pocs/）
	bindsPath  string    // 外置绑定关系文件（二进制旁 binds.yaml）
	celEnv     *cel.Env  // 与 poc 求值同源的表达式环境（校验/反解析）
}

func main() {
	addr := flag.String("addr", "127.0.0.1:7758", "web 监听地址")
	root := flag.String("root", ".", "fscanx 仓库根目录（含 mylib/，指纹编辑与重打包用）")
	flag.Parse()

	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "定位二进制目录失败:", err)
		os.Exit(1)
	}
	exeDir := filepath.Dir(exe)
	env, err := newCelEnv()
	if err != nil {
		fmt.Fprintln(os.Stderr, "初始化 CEL 环境失败:", err)
		os.Exit(1)
	}
	s := &server{
		repoRoot:  *root,
		pocsDir:   filepath.Join(exeDir, "pocs"),
		bindsPath: filepath.Join(exeDir, "binds.yaml"),
		celEnv:    env,
	}

	static, _ := fs.Sub(staticFS, "static")
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(static)))
	// go1.20 无方法路由模式，同 path 的方法分发在 handler 内做（requireMethod/pocHandler 等）
	mux.HandleFunc("/api/pocs", s.listPocs)
	mux.HandleFunc("/api/poc", s.pocHandler)
	mux.HandleFunc("/api/validate", s.validateCEL)
	mux.HandleFunc("/api/binds", s.bindsHandler)
	mux.HandleFunc("/api/finger", s.fingerHandler)
	mux.HandleFunc("/api/pack", s.postPack)
	mux.HandleFunc("/api/products", s.listProducts)
	mux.HandleFunc("/api/routes", s.listRoutes)

	fmt.Printf("fscanx-web 维护端\n  访问:   http://%s/\n  外置目录: %s\n  仓库根: %s\n", *addr, exeDir, *root)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		fmt.Fprintln(os.Stderr, "启动失败:", err)
		os.Exit(1)
	}
}
