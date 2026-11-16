package main

// 指纹明文读/写（textarea 整文编辑，内容即所写=除目标改动外零格式漂移）、
// 一键重打包（复用 packlib 共享包，与 cmd/pack 同一实现=产物字节等价）、
// 产品名全集（绑定编辑器的下拉数据源）（BL-POC-004）。

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"

	"github.com/killmonday/fscanx/mylib/finger/packlib"
)

// fingerFiles 明文名 → 工作副本路径映射（与 packlib.pairs 同源约定）。
var fingerFiles = map[string]string{
	"goby":         filepath.Join("mylib", "finger", "resources", "goby.json", "goby.json"),
	"fingers_http": filepath.Join("mylib", "finger", "resources", "fingers_http.json", "fingers_http.json"),
}

func (s *server) fingerPath(file string) (string, bool) {
	rel, ok := fingerFiles[file]
	if !ok {
		return "", false
	}
	return filepath.Join(s.repoRoot, rel), true
}

// fingerHandler 同 path 的方法分发。
func (s *server) fingerHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.getFinger(w, r)
	case http.MethodPut:
		s.putFinger(w, r)
	default:
		w.Header().Set("Allow", "GET, PUT")
		http.Error(w, "方法不允许: "+r.Method, http.StatusMethodNotAllowed)
	}
}

func (s *server) getFinger(w http.ResponseWriter, r *http.Request) {
	out := map[string]string{}
	for name := range fingerFiles {
		path, _ := s.fingerPath(name)
		if raw, err := os.ReadFile(path); err == nil {
			out[name] = string(raw)
		}
	}
	writeJSON(w, map[string]interface{}{"files": out})
}

func (s *server) putFinger(w http.ResponseWriter, r *http.Request) {
	var req struct {
		File    string `json:"file"`
		Content string `json:"content"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	path, ok := s.fingerPath(req.File)
	if !ok {
		http.Error(w, "未知指纹文件: "+req.File, 400)
		return
	}
	// 写前校验 JSON 合法性（与 packlib 同防线，改坏的库落不了盘）
	if !json.Valid([]byte(req.Content)) {
		http.Error(w, "内容不是合法 JSON，已阻止写入", 400)
		return
	}
	if err := os.WriteFile(path, []byte(req.Content), 0644); err != nil {
		http.Error(w, "写盘失败: "+err.Error(), 500)
		return
	}
	writeJSON(w, map[string]string{"status": "saved", "path": path})
}

func (s *server) postPack(w http.ResponseWriter, r *http.Request) {
	results, err := packlib.Run(s.repoRoot)
	if err != nil {
		httpErrorf(w, "打包失败: %v", err)
		return
	}
	out := make([]map[string]interface{}, 0, len(results))
	for _, res := range results {
		out = append(out, map[string]interface{}{
			"src": res.Src, "dst": res.Dst, "raw": res.Raw, "gz": res.Gz,
		})
	}
	writeJSON(w, map[string]interface{}{"status": "packed", "results": out})
}

// listProducts 提取两指纹明文的产品名全集（绑定编辑器下拉数据源；
// 键=引擎数据源 name 字段，与 binds.product 同命名空间）。
func (s *server) listProducts(w http.ResponseWriter, r *http.Request) {
	set := map[string]bool{}
	for name := range fingerFiles {
		path, _ := s.fingerPath(name)
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var arr []map[string]interface{}
		if json.Unmarshal(raw, &arr) != nil {
			continue
		}
		for _, m := range arr {
			if n, ok := m["name"].(string); ok && n != "" {
				set[n] = true
			}
		}
	}
	products := make([]string, 0, len(set))
	for n := range set {
		products = append(products, n)
	}
	sort.Strings(products)
	writeJSON(w, map[string]interface{}{"products": products})
}

// ---- 助手（避免与 api_binds 重复 import 纠缠，集中小函数）----

func sprintf(format string, args ...interface{}) string { return fmt.Sprintf(format, args...) }

func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}
