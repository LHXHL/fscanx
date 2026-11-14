package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/killmonday/fscanx/Plugins"
	"github.com/killmonday/fscanx/common"
	//"net/http"
	_ "net/http/pprof"
)

func main() {
	var Info common.HostInfo
	start := time.Now()
	common.Flag(&Info)
	common.Parse(&Info)

	// 检查是否有 --msc 参数（通过flag）
	if common.MasscanMode {
		var err error
		if common.AutoScanBigCidr { // -msc -auto 组合（BL-MSC-004）：探活产物作为 masscan 目标
			err = Plugins.ScanWithMasscanAuto(Info)
		} else {
			err = Plugins.ScanWithMasscan(Info.Host)
		}
		if err == nil {
			common.LogSuccess("[*] scan done! cost: %s\n", time.Since(start))
		}
		common.LogWG.Wait() //等待所有日志打印和写入文件等等事件
		close(common.Results)
		if err != nil {
			if errors.Is(err, Plugins.ErrMasscanInterrupted) {
				os.Exit(130)
			}
			fmt.Fprintln(os.Stderr, "[-] masscan mode failed:", err)
			os.Exit(1)
		}
		return
	}

	// 检查是否有 --std 参数（通过flag）
	if common.ScanWithStdInput {
		Plugins.ScanFromStdin()
		common.LogSuccess("[*] scan done! cost: %s\n", time.Since(start))
		common.LogWG.Wait() //等待所有日志打印和写入文件等等事件
		close(common.Results)
		return
	}
	Plugins.Scan(Info)
	common.LogSuccess("[*] scan done! cost: %s\n", time.Since(start))
	common.LogWG.Wait() //等待所有日志打印和写入文件等等事件
	close(common.Results)
}
