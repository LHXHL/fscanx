package main

// binds 读写与通用 JSON 助手（BL-POC-004）。
// GET 返回内置种子+外置合并后的生效视图；PUT 覆盖写外置 binds.yaml
// （写前做 marshal→parse round-trip 校验，坏数据落不了盘）。

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/killmonday/fscanx/PocScan"
	"gopkg.in/yaml.v2"
)

// bindsHandler 同 path 的方法分发。
func (s *server) bindsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.getBinds(w, r)
	case http.MethodPut:
		s.putBinds(w, r)
	default:
		w.Header().Set("Allow", "GET, PUT")
		http.Error(w, "方法不允许: "+r.Method, http.StatusMethodNotAllowed)
	}
}

func (s *server) getBinds(w http.ResponseWriter, r *http.Request) {
	seed, err := PocScan.SeedBinds()
	if err != nil {
		http.Error(w, "内置种子解析失败: "+err.Error(), 500)
		return
	}
	outer, err := PocScan.LoadBinds(s.bindsPath)
	if err != nil {
		http.Error(w, "外置 binds 解析失败: "+err.Error(), 500)
		return
	}
	merged := seed
	if outer != nil {
		merged = PocScan.MergeBinds(seed, outer)
	}
	writeJSON(w, map[string]interface{}{
		"binds":        merged,
		"externalPath": s.bindsPath,
		"hasExternal":  outer != nil,
	})
}

func (s *server) putBinds(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Binds []PocScan.Bind `json:"binds"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	for i, b := range req.Binds {
		if b.Product == "" || len(b.Pocs) == 0 {
			httpErrorf(w, 400, "第 %d 条绑定缺少 product 或 pocs", i+1)
			return
		}
	}
	// round-trip 校验：marshal 出的 yaml 必须能被扫描器按同 schema 解析回来
	out, err := yaml.Marshal(map[string]interface{}{"binds": req.Binds})
	if err != nil {
		httpErrorf(w, 500, "binds 序列化失败: %v", err)
		return
	}
	if _, err := PocScan.LoadBindsFromBytes(out); err != nil {
		httpErrorf(w, 500, "binds round-trip 校验失败: %v", err)
		return
	}
	if err := writeFile(s.bindsPath, out); err != nil {
		httpErrorf(w, 500, "写盘失败: %v", err)
		return
	}
	writeJSON(w, map[string]string{"status": "saved", "path": s.bindsPath})
}

// ---- 通用助手 ----

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	// 前端 @vea/request 拦截器契约：{code, data, message}，SUCCESS_CODE=0（BL-POC-004 前端重做）
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"code": 0, "data": v})
}

func readJSON(w http.ResponseWriter, r *http.Request, v interface{}) bool {
	body, err := io.ReadAll(io.LimitReader(r.Body, 10<<20))
	if err != nil {
		http.Error(w, "读取请求失败: "+err.Error(), 400)
		return false
	}
	if err := json.Unmarshal(body, v); err != nil {
		http.Error(w, "JSON 解析失败: "+err.Error(), 400)
		return false
	}
	return true
}

func httpErrorf(w http.ResponseWriter, code int, format string, args ...interface{}) {
	http.Error(w, "fscanx-web: "+sprintf(format, args...), code)
}
