//go:build linux || darwin

package Plugins

import (
	"io"
	"os"
	"syscall"
)

// loadIPDBFile unix 下优先 mmap 装入 IP 库：[]byte 直接映射文件页缓存，物理内存由 OS
// 按需换入，无归属地查询时 RSS 几乎为零（BL-MEM-003，qqwry.dat 约 27MB）。
// 映射在进程生命周期内常驻，不做 Munmap（OS 随进程回收）。
// mmap 不可用（个别文件系统/权限场景）时回退 io.ReadAll，行为与历史版本一致。
func loadIPDBFile(f *os.File) ([]byte, error) {
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	data, err := syscall.Mmap(int(f.Fd()), 0, int(st.Size()), syscall.PROT_READ, syscall.MAP_PRIVATE)
	if err != nil {
		return io.ReadAll(f)
	}
	return data, nil
}
