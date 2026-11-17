package Plugins

import (
	"fmt"
	"os"
	"path/filepath"

	fingeralias "github.com/killmonday/fscanx/mylib/finger/alias"
	"github.com/killmonday/fscanx/PocScan"
)

// InitEngineAliases 外置产品册别名注入（BL-POC-009）：main 早期调用——指纹识别前，
// 因为 DoWebScan 的归一输出（Frames）依赖注入后的归一表，等 PocScan.initpoc 再注入就晚了。
// 读二进制旁 catalog.yaml（外置产品册，与出厂 catalog.yaml 同构：标准名+别名+测试名单），
// alias 段经 AppendAliases 注入识别引擎归一表（同标准名整条覆盖），整条转交
// PocScan.SetExternalCatalog 供 loadBindsFile 合并 pocs 段。文件不存在 = 无外置产品册，
// 零动作（存量路径：外置层只做绑定的旧 binds.yaml 由 loadBindsFile 兜底读取）。
func InitEngineAliases() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(exe), "catalog.yaml"))
	if err != nil {
		return // 约定文件不存在是正常态（多数用户无外置产品册），静默跳过
	}
	if err := initEngineAliasesFrom(data); err != nil {
		fmt.Printf("[-] load external catalog.yaml error: %v\n", err)
	}
}

// initEngineAliasesFrom 从产品册字节执行注入（InitEngineAliases 的可测内层）。
// 注入失败返回错误由调用方打印——坏 yaml 需要可见（外置文件被手改错的场景）。
func initEngineAliasesFrom(data []byte) error {
	entries, err := PocScan.ParseCatalogEntries(data)
	if err != nil {
		return err
	}
	inject := make([]*fingeralias.Alias, 0, len(entries))
	for _, e := range entries {
		if e.Name == "" {
			continue
		}
		inject = append(inject, &fingeralias.Alias{
			Name:     e.Name,
			Vendor:   e.Vendor,
			Product:  e.Product,
			AliasMap: e.Alias,
		})
	}
	if len(inject) > 0 {
		Engine.Aliases.AppendAliases(inject)
	}
	PocScan.SetExternalCatalog(entries)
	fmt.Printf("[+] load %d entries from external catalog.yaml\n", len(entries))
	return nil
}
