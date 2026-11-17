// fscanx-web 是 fscanx 的本地维护端（BL-POC-004，独立二进制，扫描器本体不膨胀）：
// poc 表单化编辑（谓词树生成 CEL）与内置源写回（BL-POC-008）、绑定关系编辑、
// 指纹条目 CRUD（SQLite 主存 + 明文快照，BL-POC-006）与一键重打包。
//
// 启动：fscanx-web [-addr 127.0.0.1:7758] [-root <仓库根>]
//   - poc/binds 读写扫描器部署目录（二进制旁 pocs/ 与 binds.yaml，运行时生效零编译）
//   - 内置 POC 可选写回内置源（-root 下 PocScan/pocs/，下次 go build 生效，BL-POC-008）
//   - 指纹明文与重打包读写仓库根下 mylib/finger/resources（产物供下次 go build）
package main

import (
	"database/sql"
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

// server 维护端状态：目录定位、binds/产品册/指纹存储与 CEL 环境。
type server struct {
	repoRoot          string   // fscanx 仓库根（指纹明文、pack 目标、出厂产品册内置源）
	pocsDir           string   // 扫描器部署目录的外置 poc 目录（二进制旁 pocs/）
	bindsPath         string   // 旧外置绑定文件（binds.yaml，存量迁移源，BL-POC-009 起不再更新）
	bindsDBPath       string   // 旧 binds.db 路径（迁移源）
	catalogPath       string   // 外置产品册文件（二进制旁 catalog.yaml，扫描器消费的快照）
	builtInCatalogPath string  // 出厂产品册内置源（-root 下 mylib/finger/resources/catalog.yaml，写回目标）
	db                *sql.DB  // 产品册外置层主存储（SQLite，二进制旁 catalog.db，见 catalog_db.go，BL-POC-009）
	fingerDB          *sql.DB  // 指纹条目主存储（SQLite，二进制旁 finger.db，见 finger_db.go，BL-POC-006）
	celEnv            *cel.Env // 与 poc 求值同源的表达式环境（校验/反解析）
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
		repoRoot:           *root,
		pocsDir:            filepath.Join(exeDir, "pocs"),
		bindsPath:          filepath.Join(exeDir, "binds.yaml"),
		bindsDBPath:        filepath.Join(exeDir, "binds.db"),
		catalogPath:        filepath.Join(exeDir, "catalog.yaml"),
		builtInCatalogPath: filepath.Join(*root, "mylib", "finger", "resources", "catalog.yaml"),
		celEnv:             env,
	}
	cdb, err := openCatalogDB(filepath.Join(exeDir, "catalog.db"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "初始化产品册数据库失败:", err)
		os.Exit(1)
	}
	s.db = cdb
	defer cdb.Close()
	// 存量迁移（BL-POC-009）：binds.db / binds.yaml → 产品册 entries，一次性、幂等防护
	if err := migrateLegacyBinds(cdb, s.catalogPath, s.bindsDBPath, s.bindsPath); err != nil {
		fmt.Fprintln(os.Stderr, "存量绑定迁移失败:", err)
		os.Exit(1)
	}

	fdb, err := openFingerDB(filepath.Join(exeDir, "finger.db"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "初始化指纹数据库失败:", err)
		os.Exit(1)
	}
	s.fingerDB = fdb
	defer fdb.Close()

	static, _ := fs.Sub(staticFS, "static")
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(static)))
	// go1.20 无方法路由模式，同 path 的方法分发在 handler 内做（requireMethod/pocHandler 等）
	mux.HandleFunc("/api/pocs", s.listPocs)
	mux.HandleFunc("/api/poc", s.pocHandler)
	mux.HandleFunc("/api/validate", s.validateCEL)
	// 产品册 API（BL-POC-009 三视角；旧 binds_* 端点随绑定页退役，存量数据经 migrateLegacyBinds 迁移）
	mux.HandleFunc("/api/catalog_list", s.catalogList)
	mux.HandleFunc("/api/catalog_add", s.catalogAdd)
	mux.HandleFunc("/api/catalog_edit", s.catalogEdit)
	mux.HandleFunc("/api/catalog_delete", s.catalogDelete)
	mux.HandleFunc("/api/catalog_resolve", s.catalogResolve)
	mux.HandleFunc("/api/finger_list", s.fingerList)
	mux.HandleFunc("/api/finger_add", s.fingerAdd)
	mux.HandleFunc("/api/finger_edit", s.fingerEdit)
	mux.HandleFunc("/api/finger_copy", s.fingerCopy)
	mux.HandleFunc("/api/finger_delete", s.fingerDelete)
	mux.HandleFunc("/api/pack", s.postPack)
	mux.HandleFunc("/api/products", s.listProducts)
	mux.HandleFunc("/api/routes", s.listRoutes)
	mux.HandleFunc("/api/user/login", s.userLogin)
	mux.HandleFunc("/api/user/logout", s.userLogout)

	fmt.Printf("fscanx-web 维护端\n  访问:   http://%s/ （默认账号 %s / %s）\n  外置目录: %s\n  仓库根: %s\n", *addr, authUsername, authPassword, exeDir, *root)
	if err := http.ListenAndServe(*addr, s.authMiddleware(mux)); err != nil {
		fmt.Fprintln(os.Stderr, "启动失败:", err)
		os.Exit(1)
	}
}
