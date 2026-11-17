package main

// binds API（BL-POC-005 SQLite 化）——**BL-POC-009 起路由退役**（绑定页升级为标准产品页，
// catalog_* 端点承接；binds.db/binds.yaml 存量经 migrateLegacyBinds 一次性迁移）。
// handler 与存储函数保留：binds_db_test 历史回归继续覆盖（墓碑幂等/吸收/种子行语义等
// 机制在 catalog 侧有对应实现，此侧测试作为同族机制的参照基线）。
// 错误统一 200 + {code:1, message} 信封（见 httpErrorf）；读写前 absorbBindsFile 对齐手改。

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/killmonday/fscanx/PocScan"
)

// bindsList 生效视图：种子打底 + DB 外置层覆盖合并（仅展示；消费端在扫描器内同规则合并）。
func (s *server) bindsList(w http.ResponseWriter, r *http.Request) {
	if err := absorbBindsFile(s.db, s.bindsPath); err != nil {
		httpErrorf(w, "binds.yaml 对齐失败: %v", err)
		return
	}
	seed, err := PocScan.SeedBinds()
	if err != nil {
		httpErrorf(w, "内置种子解析失败: %v", err)
		return
	}
	outer, err := dbListBinds(s.db)
	if err != nil {
		httpErrorf(w, "读取绑定数据失败: %v", err)
		return
	}
	merged := seed
	if len(outer) > 0 {
		merged = PocScan.MergeBinds(seed, outer)
	}
	// 过滤墓碑条目（空 pocs = 删除种子绑定/失效标记，仅落盘供引擎消费，不进展示视图）
	shown := merged[:0]
	for _, b := range merged {
		if len(b.Pocs) > 0 {
			shown = append(shown, b)
		}
	}
	writeJSON(w, map[string]interface{}{
		"binds":        shown,
		"externalPath": s.bindsPath,
		"hasExternal":  len(outer) > 0,
	})
}

// bindsAddReq 三个写端点共用载体（original 仅 edit 使用）。
type bindsWriteReq struct {
	Original string   `json:"original"`
	Product  string   `json:"product"`
	Pocs     []string `json:"pocs"`
}

func (s *server) bindsAdd(w http.ResponseWriter, r *http.Request) {
	req, ok := readBindsWriteReq(w, r, false)
	if !ok {
		return
	}
	if err := absorbBindsFile(s.db, s.bindsPath); err != nil {
		httpErrorf(w, "binds.yaml 对齐失败: %v", err)
		return
	}
	if err := bindsAdd(s.db, s.bindsPath, req.Product, req.Pocs); err != nil {
		httpErrorf(w, "添加绑定失败: %v", err)
		return
	}
	writeJSON(w, map[string]string{"status": "saved", "path": s.bindsPath})
}

func (s *server) bindsEdit(w http.ResponseWriter, r *http.Request) {
	req, ok := readBindsWriteReq(w, r, true)
	if !ok {
		return
	}
	if err := absorbBindsFile(s.db, s.bindsPath); err != nil {
		httpErrorf(w, "binds.yaml 对齐失败: %v", err)
		return
	}
	if err := bindsEdit(s.db, s.bindsPath, req.Original, req.Product, req.Pocs); err != nil {
		httpErrorf(w, "编辑绑定失败: %v", err)
		return
	}
	writeJSON(w, map[string]string{"status": "saved", "path": s.bindsPath})
}

func (s *server) bindsDelete(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Product string `json:"product"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	req.Product = strings.TrimSpace(req.Product)
	if req.Product == "" {
		httpErrorf(w, "product 不能为空")
		return
	}
	if err := absorbBindsFile(s.db, s.bindsPath); err != nil {
		httpErrorf(w, "binds.yaml 对齐失败: %v", err)
		return
	}
	if err := bindsDelete(s.db, s.bindsPath, req.Product); err != nil {
		httpErrorf(w, "删除绑定失败: %v", err)
		return
	}
	writeJSON(w, map[string]string{"status": "saved", "path": s.bindsPath})
}

// readBindsWriteReq 解析并校验写请求：product 非空、pocs 清洗后非空；edit 还需 original 非空。
func readBindsWriteReq(w http.ResponseWriter, r *http.Request, needOriginal bool) (*bindsWriteReq, bool) {
	var req bindsWriteReq
	if !readJSON(w, r, &req) {
		return nil, false
	}
	req.Product = strings.TrimSpace(req.Product)
	req.Original = strings.TrimSpace(req.Original)
	pocs := make([]string, 0, len(req.Pocs))
	seen := make(map[string]bool, len(req.Pocs))
	for _, p := range req.Pocs {
		p = strings.TrimSpace(p)
		if p != "" && !seen[p] {
			seen[p] = true
			pocs = append(pocs, p)
		}
	}
	req.Pocs = pocs
	if req.Product == "" {
		httpErrorf(w, "product 不能为空")
		return nil, false
	}
	if len(req.Pocs) == 0 {
		httpErrorf(w, "pocs 至少一项")
		return nil, false
	}
	if needOriginal && req.Original == "" {
		httpErrorf(w, "original（编辑前的产品名）不能为空")
		return nil, false
	}
	return &req, true
}

// ---- 通用助手 ----

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	// 前端 @vea/request 拦截器契约：{code, data, message}，SUCCESS_CODE=0（BL-POC-004 前端重做）
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"code": 0, "data": v})
}

func readJSON(w http.ResponseWriter, r *http.Request, v interface{}) bool {
	// 写接口要求 JSON Content-Type：跨站无预检请求（text/plain）被挡下，浏览器侧变为预检请求即失败
	if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		httpErrorf(w, "Content-Type 必须为 application/json")
		return false
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 10<<20))
	if err != nil {
		httpErrorf(w, "读取请求失败: %v", err)
		return false
	}
	if err := json.Unmarshal(body, v); err != nil {
		httpErrorf(w, "JSON 解析失败: %v", err)
		return false
	}
	return true
}

// httpErrorf 业务/请求错误统一 200 + {code:1, message} 信封——前端 @vea/request 拦截器对
// code!=0 会 throw Error(message) 并经 onError 展示，指导性错误文本因此可达用户
// （非 2xx 纯文本会被 axios 丢弃成 "Request failed with status code 400"）。
func httpErrorf(w http.ResponseWriter, format string, args ...interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"code": 1, "message": "fscanx-web: " + sprintf(format, args...)})
}
