//go:build windows

package Plugins

import (
	"io"
	"os"
)

// loadIPDBFile Windows 直接 ReadAll：syscall.Mmap 在 windows 的 Go 上不存在（编译期
// 失败，BL-MEM-003 独立审查实测）；开发机场景内存不敏感，全量读入保持兼容。
func loadIPDBFile(f *os.File) ([]byte, error) {
	return io.ReadAll(f)
}
