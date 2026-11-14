// Package pack 将指纹库明文 JSON 重新打包为 go:embed 引用的 .gz 文件，
// 替代 gzip 外部命令（Windows 无内置 gzip），实现跨平台纯 Go 维护工作流：
//
//	编辑 mylib/finger/resources 下的指纹明文 → go run ./mylib/finger/cmd/pack → go build .
//
// 仅维护 resources 根目录的 embed 真源（embed.go 引用处）；工作副本目录内的
// 历史 .gz 无代码引用，不在打包范围。
package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// resDir 为 resources 目录相对项目根的位置（请在项目根目录运行本工具）。
const resDir = "mylib" + string(filepath.Separator) + "finger" + string(filepath.Separator) + "resources"

// pairs 为"明文源（工作副本目录）→ gz 目标（resources 根，embed 真源）"映射。
var pairs = []struct{ Src, Dst string }{
	{Src: filepath.Join("fingers_http.json", "fingers_http.json"), Dst: "fingers_http.json.gz"},
	{Src: filepath.Join("goby.json", "goby.json"), Dst: "goby.json.gz"},
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "❌ %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	for _, p := range pairs {
		src := filepath.Join(resDir, p.Src)
		dst := filepath.Join(resDir, p.Dst)

		raw, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("读取明文失败 %s（请在项目根目录运行 go run ./mylib/finger/cmd/pack）: %w", src, err)
		}

		// 打包前校验 JSON 合法性，防止把改坏的指纹库压进二进制、运行时才暴露
		if !json.Valid(raw) {
			return fmt.Errorf("明文不是合法 JSON，已阻止打包: %s", src)
		}

		gz, err := gzipBytes(raw)
		if err != nil {
			return fmt.Errorf("压缩失败 %s: %w", src, err)
		}

		if err := os.WriteFile(dst, gz, 0644); err != nil {
			return fmt.Errorf("写入 gz 失败 %s: %w", dst, err)
		}
		fmt.Printf("✅ %s → %s (%d → %d bytes, %.1f%%)\n",
			src, dst, len(raw), len(gz), float64(len(gz))*100/float64(len(raw)))
	}
	fmt.Println("打包完成，执行 go build . 生效")
	return nil
}

// gzipBytes 以确定性参数压缩：ModTime 保持零值（gzip 头 mtime 字段写 0）、
// 级别 BestCompression——明文内容不变则产物字节逐位一致，git diff 只反映真实变更。
func gzipBytes(raw []byte) ([]byte, error) {
	var buf bytes.Buffer
	w, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(raw); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
