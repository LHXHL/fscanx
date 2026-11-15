// Package packlib 将指纹库明文 JSON 重新打包为 go:embed 引用的 .gz 文件（BL-POC-004
// 从 mylib/finger/cmd/pack 提取的共享包）：cmd/pack 命令与 fscanx-web 的"重打包"按钮
// 共同引用，保证产物字节级等价（同一确定性 gzip 实现）。
//
// 工作流：编辑 mylib/finger/resources 下的指纹明文 → 打包（本包 Run）→ go build .
//
// 仅维护 resources 根目录的 embed 真源（embed.go 引用处）；工作副本目录内的
// 历史 .gz 无代码引用，不在打包范围。
package packlib

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// pairs 为"明文源（工作副本目录）→ gz 目标（resources 根，embed 真源）"映射。
var pairs = []struct{ Src, Dst string }{
	{Src: filepath.Join("fingers_http.json", "fingers_http.json"), Dst: "fingers_http.json.gz"},
	{Src: filepath.Join("goby.json", "goby.json"), Dst: "goby.json.gz"},
}

// Run 打包全部指纹明文到 embed 真源。repoRoot 为项目根目录（含 mylib/）；
// 打包前逐文件校验 JSON 合法性，防止把改坏的指纹库压进二进制、运行时才暴露。
func Run(repoRoot string) (results []Result, err error) {
	for _, p := range pairs {
		src := filepath.Join(repoRoot, "mylib", "finger", "resources", p.Src)
		dst := filepath.Join(repoRoot, "mylib", "finger", "resources", p.Dst)

		raw, err := os.ReadFile(src)
		if err != nil {
			return results, fmt.Errorf("读取明文失败 %s: %w", src, err)
		}
		if !json.Valid(raw) {
			return results, fmt.Errorf("明文不是合法 JSON，已阻止打包: %s", src)
		}
		gz, err := GzipBytes(raw)
		if err != nil {
			return results, fmt.Errorf("压缩失败 %s: %w", src, err)
		}
		if err := os.WriteFile(dst, gz, 0644); err != nil {
			return results, fmt.Errorf("写入 gz 失败 %s: %w", dst, err)
		}
		results = append(results, Result{Src: src, Dst: dst, Raw: len(raw), Gz: len(gz)})
	}
	return results, nil
}

// Result 单文件打包结果。
type Result struct {
	Src    string
	Dst    string
	Raw    int
	Gz     int
}

// GzipBytes 以确定性参数压缩：ModTime 保持零值（gzip 头 mtime 字段写 0）、
// 级别 BestCompression——明文内容不变则产物字节逐位一致（与原 cmd/pack 同实现，
// 字节等价验收的前提），git diff 只反映真实变更。
func GzipBytes(raw []byte) ([]byte, error) {
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
