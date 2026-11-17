package main

// 产品册 API（BL-POC-009 三视角维护端）：按页面接口命名规范四端点 + 归属判定——
//   GET  /api/catalog_list    合并视图：出厂（embed ⊕ -root 源文件优先，防写回后陈旧基底）
//                             ⊕ DB 外置层，条目带 origin 与 missing_alias 警示；墓碑过滤
//   POST /api/catalog_add     新建标准产品条目——必须挂别名（防死条目铁律），名字已在指纹
//                             输出名全集时允许空别名（自标准名形态）
//   POST /api/catalog_edit    编辑条目（original 定位 + target 双目标：built-in 写回 -root
//                             出厂源文件下次编译生效 / external 落 DB 立即生效）；整条全量写出
//   POST /api/catalog_delete  删除：出厂条目落墓碑（绑定失效、别名归一保留）；外置真删；幂等
//   POST /api/catalog_resolve 批量指纹归属判定（标准产品 /「某标准产品」的别名 / 未归属）
// 错误统一 200 + {code:1,message} 信封；读写前 absorbCatalogFile 对齐手改。

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"

	fingeralias "github.com/killmonday/fscanx/mylib/finger/alias"
	"github.com/killmonday/fscanx/mylib/finger/resources"
	"github.com/killmonday/fscanx/PocScan"
	"gopkg.in/yaml.v2"
)

// catalogEntryView 合并视图条目（前端展示形态）。
type catalogEntryView struct {
	Name         string              `json:"name"`
	Vendor       string              `json:"vendor"`
	Product      string              `json:"product"`
	Alias        map[string][]string `json:"alias"`
	Pocs         []string            `json:"pocs"`
	Origin       string              `json:"origin"`         // built-in | external
	MissingAlias bool                `json:"missing_alias"`  // 别名区为空且非出厂自标准名形态（出厂别名条目都有别名；外置条目空别名 = 可能死条目，警示）
	HasPocs      bool                `json:"has_pocs"`
}

// builtInCatalogBaseline 出厂条目基线：-root 源文件优先（built-in 写回后 embed 仍是旧值，
// 读仓库最新源防陈旧基底二次覆盖——BL-POC-008 getPoc(built-in) 同款决策），embed 兜底。
func (s *server) builtInCatalogBaseline() ([]PocScan.CatalogEntry, error) {
	if s.builtInCatalogPath != "" {
		if data, err := os.ReadFile(s.builtInCatalogPath); err == nil {
			if entries, perr := PocScan.ParseCatalogEntries(data); perr == nil {
				return entries, nil
			}
			// 源文件损坏：回落 embed（写回守卫会防住再次写坏）
		}
	}
	return PocScan.FullCatalog()
}

func (s *server) catalogList(w http.ResponseWriter, r *http.Request) {
	if err := absorbCatalogFile(s.db, s.catalogPath); err != nil {
		httpErrorf(w, "catalog.yaml 对齐失败: %v", err)
		return
	}
	builtIn, err := s.builtInCatalogBaseline()
	if err != nil {
		httpErrorf(w, "出厂产品册解析失败: %v", err)
		return
	}
	outer, err := dbListEntries(s.db)
	if err != nil {
		httpErrorf(w, "读取产品册数据失败: %v", err)
		return
	}
	outerByName := map[string]PocScan.CatalogEntry{}
	builtInNames := map[string]bool{}
	for _, bi := range builtIn {
		builtInNames[bi.Name] = true
	}
	for _, e := range outer {
		outerByName[e.Name] = e
	}
	// 合并视图：出厂打底、外置同名整条覆盖（引擎 MergeBinds 同语义）；墓碑（出厂同名且
	// 外置 pocs 空）= 出厂条目已删除标记，不展示。
	var out []catalogEntryView
	for _, bi := range builtIn {
		if e, ok := outerByName[bi.Name]; ok {
			if len(e.Pocs) == 0 {
				continue // 墓碑：出厂条目已删除
			}
			out = append(out, s.toEntryView(e, "external"))
			continue
		}
		out = append(out, s.toEntryView(bi, "built-in"))
	}
	for _, e := range outer {
		if builtInNames[e.Name] {
			continue // 已在出厂循环输出（覆盖或墓碑）
		}
		if len(e.Pocs) == 0 && len(e.Alias) == 0 {
			continue // 空条目（无别名无名单）无展示价值
		}
		out = append(out, s.toEntryView(e, "external"))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	writeJSON(w, map[string]interface{}{
		"entries":       out,
		"external_path": s.catalogPath,
		"built_in_path": s.builtInCatalogPath,
		"has_external":  len(outer) > 0,
		"build_hint":    "保存到代码源文件的条目需重新编译 fscanx 生效；外挂文件的条目重跑扫描器即生效",
	})
}

// toEntryView 合并视图条目。缺别名警示排除"自标准名"合法形态（名字本身在指纹输出名全集
// ——识别输出原名即标准名，空别名非死条目；对抗式复审：kingsoft/vmware 等出厂自标准名
// 条目曾被误标警示）。
func (s *server) toEntryView(e PocScan.CatalogEntry, origin string) catalogEntryView {
	missing := len(e.Alias) == 0 && !s.fingerprintNameSet()[e.Name]
	return catalogEntryView{
		Name: e.Name, Vendor: e.Vendor, Product: e.Product,
		Alias: e.Alias, Pocs: e.Pocs, Origin: origin,
		MissingAlias: missing, HasPocs: len(e.Pocs) > 0,
	}
}

// catalogWriteReq 写端点共用载体（original 仅 edit；add/edit 均可带 target）。
type catalogWriteReq struct {
	Original string              `json:"original"`
	Target   string              `json:"target"` // built-in | external（默认 external；add 支持——新建直接进代码源文件）
	Entry    PocScan.CatalogEntry `json:"entry"`
}

func (s *server) catalogAdd(w http.ResponseWriter, r *http.Request) {
	var req catalogWriteReq
	if !readJSON(w, r, &req) {
		return
	}
	e := req.Entry
	e.Name = strings.TrimSpace(e.Name)
	if e.Name == "" {
		httpErrorf(w, "标准名（name）不能为空")
		return
	}
	if err := absorbCatalogFile(s.db, s.catalogPath); err != nil {
		httpErrorf(w, "catalog.yaml 对齐失败: %v", err)
		return
	}
	if in, err := isOuterEntry(s.db, e.Name); err != nil {
		httpErrorf(w, "查询条目失败: %v", err)
		return
	} else if in {
		httpErrorf(w, "条目「%s」已存在（外置层），请用编辑", e.Name)
		return
	}
	if s.isBuiltInEntryBase(e.Name) {
		httpErrorf(w, "「%s」是出厂标准产品，请用编辑修改", e.Name)
		return
	}
	// 防死条目铁律：别名区为空时，名字必须已在指纹输出名全集（自标准名形态——识别输出
	// 原名即标准名）；否则识别永远不会输出这个名字，条目永不命中
	if len(e.Alias) == 0 && !s.fingerprintNameSet()[e.Name] {
		httpErrorf(w, "「%s」不在指纹输出名全集且未挂任何别名——识别引擎永远报不出这个名字，绑定不会命中。请至少挂一条真实指纹写法做别名，或改用指纹输出名全集中的写法", e.Name)
		return
	}
	if req.Target == "built-in" {
		s.catalogAddBuiltIn(w, e)
		return
	}
	if err := catalogMutate(s.db, s.catalogPath, func(tx *sql.Tx) error {
		return dbUpsertEntry(tx, e)
	}); err != nil {
		httpErrorf(w, "新建条目失败: %v", err)
		return
	}
	writeJSON(w, map[string]string{"status": "saved", "path": s.catalogPath})
}

// catalogAddBuiltIn 新建条目直接进代码源文件（用户裁决：web 新建的资产可选"下次 go build
// 生效"）：出厂源文本追加（与转正通道同一文本手术/三重防护/备份），不落外挂层——与转正
// 通道一致的窗口期语义（重编译前扫描器不识别该条目，hint 明示）。
func (s *server) catalogAddBuiltIn(w http.ResponseWriter, e PocScan.CatalogEntry) {
	if s.builtInCatalogPath == "" {
		httpErrorf(w, "未配置 -root（仓库根），无法写入代码源文件；可改用外挂保存（立即生效）")
		return
	}
	src, err := os.ReadFile(s.builtInCatalogPath)
	if err != nil {
		httpErrorf(w, "读取源文件失败: %v", err)
		return
	}
	out, err := appendCatalogEntryToText(src, e)
	if err != nil {
		httpErrorf(w, "%v", err)
		return
	}
	entries, perr := PocScan.ParseCatalogEntries(out)
	if perr != nil {
		httpErrorf(w, "写入内容解析校验失败，源文件未改动: %v", perr)
		return
	}
	ne, ok := findEntryByName(entries, e.Name)
	if !ok || !sameCatalogEntry(ne, e) {
		httpErrorf(w, "写入内容校验不符（新条目缺失或内容漂移），源文件未改动")
		return
	}
	if bak, rerr := os.ReadFile(s.builtInCatalogPath); rerr == nil {
		if werr := os.WriteFile(s.builtInCatalogPath+".bak", bak, 0644); werr != nil {
			httpErrorf(w, "写前备份失败（%v），为安全起见本次写入未执行", werr)
			return
		}
	}
	if err := atomicWrite(s.builtInCatalogPath, out); err != nil {
		httpErrorf(w, "写入源文件失败: %v", err)
		return
	}
	writeJSON(w, map[string]string{"status": "saved", "path": s.builtInCatalogPath,
		"hint": "已写入代码源文件，重新编译 fscanx 后生效；在此之前该条目在扫描器中暂不生效"})
}

func (s *server) catalogEdit(w http.ResponseWriter, r *http.Request) {
	var req catalogWriteReq
	if !readJSON(w, r, &req) {
		return
	}
	original := strings.TrimSpace(req.Original)
	e := req.Entry
	e.Name = strings.TrimSpace(e.Name)
	if original == "" || e.Name == "" {
		httpErrorf(w, "original 与 name 不能为空")
		return
	}
	if req.Target == "" {
		req.Target = "external"
	}
	if req.Target != "built-in" && req.Target != "external" {
		httpErrorf(w, "target 仅支持 built-in | external")
		return
	}
	if err := absorbCatalogFile(s.db, s.catalogPath); err != nil {
		httpErrorf(w, "catalog.yaml 对齐失败: %v", err)
		return
	}
	if req.Target == "built-in" {
		s.catalogEditBuiltIn(w, original, e)
		return
	}
	s.catalogEditExternal(w, original, e)
}

// catalogEditBuiltIn 写回 -root 出厂源文件（下次编译生效；BL-POC-008 内置写回同模式）。
// 守卫：-root 已配置、源文件存在、改名不与出厂其他条目冲突；文件级 temp+rename 原子写。
// 两条路径：original 是出厂条目 → **文本级整条替换**（注释/排版/其余条目字节零漂移）；
// original 不是出厂条目（外挂条目转正）→ 文本追加到出厂源，写成功后删外挂同名行
// （条目已进代码源文件，外挂覆盖行冗余，删后合并视图 origin 翻转为内置）。
// 三重防护：逐块 yaml 解析定位（引号/转义/任意合法格式鲁棒）、写回前全文解析校验
// （不过不写）、定位或校验失败一律显式报错保持源文件不动。
func (s *server) catalogEditBuiltIn(w http.ResponseWriter, original string, e PocScan.CatalogEntry) {
	if s.builtInCatalogPath == "" {
		httpErrorf(w, "未配置 -root（仓库根），无法写回代码源文件；可改用外挂保存（立即生效）")
		return
	}
	if _, err := os.Stat(s.builtInCatalogPath); err != nil {
		httpErrorf(w, "产品册源文件不存在（%s），无法写回；可改用外挂保存", s.builtInCatalogPath)
		return
	}
	baseline, err := s.builtInCatalogBaseline()
	if err != nil {
		httpErrorf(w, "出厂产品册解析失败: %v", err)
		return
	}
	src, err := os.ReadFile(s.builtInCatalogPath)
	if err != nil {
		httpErrorf(w, "读取源文件失败: %v", err)
		return
	}
	found := false
	for _, bi := range baseline {
		if bi.Name == original {
			found = true
			continue
		}
		if e.Name != original && bi.Name == e.Name {
			httpErrorf(w, "改名目标「%s」与出厂条目冲突", e.Name)
			return
		}
	}

	var out []byte
	if found {
		out, err = replaceCatalogEntryInText(src, original, e)
	} else {
		out, err = appendCatalogEntryToText(src, e)
	}
	if err != nil {
		httpErrorf(w, "%v", err)
		return
	}
	// 写回前解析校验（三重防护之二）：全文解析不过 = 不写，源文件保持不动
	entries, perr := PocScan.ParseCatalogEntries(out)
	if perr != nil {
		httpErrorf(w, "写回内容解析校验失败，源文件未改动: %v", perr)
		return
	}
	if found {
		ne, ok := findEntryByName(entries, e.Name)
		if !ok || !sameCatalogEntry(ne, e) {
			httpErrorf(w, "写回内容校验不符（目标条目缺失或内容漂移），源文件未改动")
			return
		}
	} else if _, ok := findEntryByName(entries, e.Name); !ok {
		httpErrorf(w, "写回内容校验不符（转正条目未追加），源文件未改动")
		return
	}
	// 写回前滚动备份 .bak（最后一份写回前的原文）——未来任何未预见缺陷的一键恢复通道
	if bak, rerr := os.ReadFile(s.builtInCatalogPath); rerr == nil {
		if werr := os.WriteFile(s.builtInCatalogPath+".bak", bak, 0644); werr != nil {
			httpErrorf(w, "写回前备份失败（%v），为安全起见本次写回未执行", werr)
			return
		}
	}
	if err := atomicWrite(s.builtInCatalogPath, out); err != nil {
		httpErrorf(w, "写回源文件失败: %v", err)
		return
	}
	if !found {
		// 转正收尾：删外挂同名行（外挂层冗余覆盖），合并视图 origin 翻转为内置
		if inOuter, e2 := isOuterEntry(s.db, original); e2 == nil && inOuter {
			if merr := catalogMutate(s.db, s.catalogPath, func(tx *sql.Tx) error {
				_, derr := tx.Exec(`DELETE FROM entries WHERE name = ?`, original)
				return derr
			}); merr != nil {
				writeJSON(w, map[string]string{"status": "saved", "path": s.builtInCatalogPath,
					"hint": "已写入源文件，但外挂同名行清理失败（不影响生效，重编译后以外挂内容覆盖显示）: " + merr.Error()})
				return
			}
		}
		writeJSON(w, map[string]string{"status": "saved", "path": s.builtInCatalogPath,
			"hint": "已写入代码源文件，重新编译 fscanx 后生效；在此之前该条目在扫描器中暂不生效（外挂记录已清理，扫描器仍用编译期内置旧版）"})
		return
	}
	// 内置编辑收尾：若外挂层存在同名墓碑行（此前删除该条目的标记），写回 = 撤销删除意图，
	// 清理墓碑让写回的内置定义可见（非墓碑的外挂覆盖行是合法语义，不动）
	if inOuter, e2 := isOuterEntry(s.db, original); e2 == nil && inOuter {
		if merr := catalogMutate(s.db, s.catalogPath, func(tx *sql.Tx) error {
			tomb, terr := s.isTombstoneRowBase(tx, original)
			if terr != nil {
				return terr
			}
			if !tomb {
				return nil // 非墓碑覆盖行不动
			}
			_, derr := tx.Exec(`DELETE FROM entries WHERE name = ?`, original)
			return derr
		}); merr != nil {
			writeJSON(w, map[string]string{"status": "saved", "path": s.builtInCatalogPath,
				"hint": "已写入源文件，但外挂删除标记清理失败（界面暂被压制显示，重编译后可见）: " + merr.Error()})
			return
		}
	}
	writeJSON(w, map[string]string{"status": "saved", "path": s.builtInCatalogPath, "hint": "已写入代码源文件，重新编译 fscanx 后生效"})
}

// findEntryByName 按名取条目。
func findEntryByName(entries []PocScan.CatalogEntry, name string) (PocScan.CatalogEntry, bool) {
	for _, e := range entries {
		if e.Name == name {
			return e, true
		}
	}
	return PocScan.CatalogEntry{}, false
}

// sameCatalogEntry 内容级相等（alias map 逐键逐值比对——替换后断言用）。
func sameCatalogEntry(a, b PocScan.CatalogEntry) bool {
	if a.Name != b.Name || a.Vendor != b.Vendor || a.Product != b.Product ||
		len(a.Alias) != len(b.Alias) || len(a.Pocs) != len(b.Pocs) {
		return false
	}
	for k, va := range a.Alias {
		vb, ok := b.Alias[k]
		if !ok || len(va) != len(vb) {
			return false
		}
		for i := range va {
			if va[i] != vb[i] {
				return false
			}
		}
	}
	for i := range a.Pocs {
		if a.Pocs[i] != b.Pocs[i] {
			return false
		}
	}
	return true
}

// catalogEditExternal 外置保存（DB 整条覆盖，立即生效）。
func (s *server) catalogEditExternal(w http.ResponseWriter, original string, e PocScan.CatalogEntry) {
	inOuter, err := isOuterEntry(s.db, original)
	if err != nil {
		httpErrorf(w, "查询条目失败: %v", err)
		return
	}
	if !inOuter && !s.isBuiltInEntryBase(original) {
		httpErrorf(w, "条目「%s」不存在", original)
		return
	}
	if e.Name != original {
		if in, err := isOuterEntry(s.db, e.Name); err != nil {
			httpErrorf(w, "查询条目失败: %v", err)
			return
		} else if in {
			httpErrorf(w, "改名目标「%s」已存在（外置层）", e.Name)
			return
		}
	}
	err = catalogMutate(s.db, s.catalogPath, func(tx *sql.Tx) error {
		if e.Name != original {
			if inOuter {
				// 外置原行：普通行删除（被新条目替代）；墓碑行保留（出厂删除标记不可移除，
				// BL-POC-005 同语义——删了出厂绑定会复活）
				tomb, terr := s.isTombstoneRowBase(tx, original)
				if terr != nil {
					return terr
				}
				if !tomb {
					if _, derr := tx.Exec(`DELETE FROM entries WHERE name = ?`, original); derr != nil {
						return derr
					}
				} else if s.isBuiltInEntryBase(original) {
					// 墓碑行改名：墓碑保留之外，若 original 本身是出厂条目，墓碑已在
					// （无需重写）；新条目独立写入
					_ = original
				}
			} else if s.isBuiltInEntryBase(original) {
				// 出厂条目改名（原条目不在外置层）：原出厂条目落墓碑失效 + 新条目写入
				tomb, terr := s.builtInTombstone(original)
				if terr != nil {
					return terr
				}
				if uerr := dbUpsertEntry(tx, tomb); uerr != nil {
					return uerr
				}
			}
		}
		// 同名编辑直接整条 upsert（外置行覆盖 / 出厂条目落外置整条覆盖——整条全量写出铁律）
		return dbUpsertEntry(tx, e)
	})
	if err != nil {
		httpErrorf(w, "编辑条目失败: %v", err)
		return
	}
	writeJSON(w, map[string]string{"status": "saved", "path": s.catalogPath})
}

func (s *server) catalogDelete(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		httpErrorf(w, "name 不能为空")
		return
	}
	if err := absorbCatalogFile(s.db, s.catalogPath); err != nil {
		httpErrorf(w, "catalog.yaml 对齐失败: %v", err)
		return
	}
	inOuter, err := isOuterEntry(s.db, name)
	if err != nil {
		httpErrorf(w, "查询条目失败: %v", err)
		return
	}
	if !inOuter && !s.isBuiltInEntryBase(name) {
		httpErrorf(w, "条目「%s」不存在", name)
		return
	}
	err = catalogMutate(s.db, s.catalogPath, func(tx *sql.Tx) error {
		if inOuter {
			// 墓碑幂等：已是墓碑（出厂条目的删除标记）→ 幂等成功不删（删了出厂绑定会复活）
			var aliasJSON, pocsJSON string
			if err := tx.QueryRow(`SELECT alias, pocs FROM entries WHERE name = ?`, name).Scan(&aliasJSON, &pocsJSON); err != nil {
				return err
			}
			var p []string
			if json.Unmarshal([]byte(pocsJSON), &p) == nil && len(p) == 0 && s.isBuiltInEntryBase(name) {
				return nil
			}
			_, err := tx.Exec(`DELETE FROM entries WHERE name = ?`, name)
			return err
		}
		// 出厂条目删除 = 落墓碑（绑定失效、别名归一保留）
		tomb, e := s.builtInTombstone(name)
		if e != nil {
			return e
		}
		return dbUpsertEntry(tx, tomb)
	})
	if err != nil {
		httpErrorf(w, "删除条目失败: %v", err)
		return
	}
	writeJSON(w, map[string]string{"status": "saved", "path": s.catalogPath})
}

// tombstoneOf 基线条目墓碑：完整条目但 pocs 置空（绑定失效；别名归一保留）。
func tombstoneOf(base []PocScan.CatalogEntry, name string) (PocScan.CatalogEntry, error) {
	for _, e := range base {
		if e.Name == name {
			t := e
			t.Pocs = nil
			return t, nil
		}
	}
	return PocScan.CatalogEntry{}, fmt.Errorf("「%s」不是出厂条目", name)
}

// builtInTombstone 出厂条目墓碑（server 基线：-root 源文件优先）——转正写入源文件的条目
// 在重编译前 embed 不含，但删除/墓碑操作必须认它（对抗式复审：转正条目窗口期删除缺口）。
func (s *server) builtInTombstone(name string) (PocScan.CatalogEntry, error) {
	base, err := s.builtInCatalogBaseline()
	if err != nil {
		return PocScan.CatalogEntry{}, err
	}
	return tombstoneOf(base, name)
}

// isBuiltInEntryBase 出厂条目判定（baseline 基线：-root 源文件优先，embed 兜底）。
func (s *server) isBuiltInEntryBase(name string) bool {
	base, err := s.builtInCatalogBaseline()
	if err != nil {
		return false
	}
	for _, e := range base {
		if e.Name == name {
			return true
		}
	}
	return false
}

// builtInEntryBase 取出厂条目（baseline 基线）。
func (s *server) builtInEntryBase(name string) (PocScan.CatalogEntry, bool) {
	base, err := s.builtInCatalogBaseline()
	if err != nil {
		return PocScan.CatalogEntry{}, false
	}
	for _, e := range base {
		if e.Name == name {
			return e, true
		}
	}
	return PocScan.CatalogEntry{}, false
}

// isTombstoneRowBase 行是否为出厂条目的墓碑（pocs 空 + baseline 判定为出厂条目）。
func (s *server) isTombstoneRowBase(tx *sql.Tx, name string) (bool, error) {
	var pocsJSON string
	err := tx.QueryRow(`SELECT pocs FROM entries WHERE name = ?`, name).Scan(&pocsJSON)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var p []string
	if json.Unmarshal([]byte(pocsJSON), &p) != nil {
		return false, nil
	}
	return len(p) == 0 && s.isBuiltInEntryBase(name), nil
}

// catalogResolve 批量指纹归属判定：对每个 (source, name) 查归一表（出厂 ⊕ 外置别名注入），
// 命中 = 「某标准产品」的别名；未命中但名字本身是标准产品条目 = 标准产品；否则未归属。
func (s *server) catalogResolve(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Items []struct {
			Source string `json:"source"`
			Name   string `json:"name"`
		} `json:"items"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	aliases, err := s.mergedAliases()
	if err != nil {
		httpErrorf(w, "构造归一表失败: %v", err)
		return
	}
	stdNames := map[string]bool{}
	if base, err := s.builtInCatalogBaseline(); err == nil {
		for _, e := range base {
			stdNames[strings.ToLower(e.Name)] = true
		}
	}
	outer, _ := dbListEntries(s.db)
	for _, e := range outer {
		stdNames[strings.ToLower(e.Name)] = true
	}
	type verdict struct {
		Source  string `json:"source"`
		Name    string `json:"name"`
		Owner   string `json:"owner"`
		Relaton string `json:"relation"` // alias-of | self | unassigned
	}
	out := make([]verdict, 0, len(req.Items))
	for _, it := range req.Items {
		v := verdict{Source: it.Source, Name: it.Name, Relaton: "unassigned"}
		// self 优先：名字本身就是标准产品条目名（部分条目标准名也收录在某源别名区——
		// 自收录形态，显示"标准产品"而非指向自己的 alias-of）
		if stdNames[strings.ToLower(it.Name)] {
			v.Relaton = "self"
		} else if a, ok := aliases.Find(it.Source, resources.NormalizeString(it.Name)); ok {
			// 指定源次之（Find 带 blocked 检查，与引擎该源行为一致）
			v.Owner, v.Relaton = a.Name, "alias-of"
		} else if _, a, ok := aliases.FindAny(it.Name); ok {
			// 未命中跨源兜底（写法归属是全局问题——写法可能只收在非当前引擎源，
			// 如 ehole 独有写法在 fingers_http 页查看时不应误报未归属）
			v.Owner, v.Relaton = a.Name, "alias-of"
		}
		out = append(out, v)
	}
	writeJSON(w, map[string]interface{}{"verdicts": out})
}

// mergedAliases 构造"出厂 ⊕ 外置"归一表（归属判定用；每次请求现构造——出厂 50 条目解析
// 微秒级，简单正确优先）。外置条目仅注入非墓碑（墓碑别名保留出厂原样——注意出厂 embed
// 已含别名，AppendAliases 同名覆盖语义与扫描器 InitEngineAliases 一致）。
func (s *server) mergedAliases() (*fingeralias.Aliases, error) {
	aliases, err := fingeralias.NewAliases()
	if err != nil {
		return nil, err
	}
	outer, err := dbListEntries(s.db)
	if err != nil {
		return nil, err
	}
	inject := make([]*fingeralias.Alias, 0, len(outer))
	for _, e := range outer {
		if e.Name == "" {
			continue
		}
		inject = append(inject, &fingeralias.Alias{Name: e.Name, Vendor: e.Vendor, Product: e.Product, AliasMap: e.Alias})
	}
	if len(inject) > 0 {
		aliases.AppendAliases(inject)
	}
	return aliases, nil
}

// renderCatalogEntryText 渲染单个条目文本块（yaml 库序列化——特殊字符转义安全；
// v2 紧凑风格与当前源文件一致）。crlf=true 时行尾适配源文件风格（git autocrlf 物化
// CRLF 的文件写回后不产生混合行尾）。
func renderCatalogEntryText(e PocScan.CatalogEntry, crlf bool) ([]byte, error) {
	b, err := yaml.Marshal([]PocScan.CatalogEntry{e})
	if err != nil {
		return nil, err
	}
	if crlf {
		b = bytes.ReplaceAll(b, []byte("\n"), []byte("\r\n"))
	}
	return b, nil
}

// catalogEntryStartRe 顶层条目起始标记（列首 "- name:"）——块边界定位锚。
var catalogEntryStartRe = regexp.MustCompile(`(?m)^- name:`)

// replaceCatalogEntryInText 源文本中的文本级条目替换：按顶层 "- name:" 边界把源文本切成
// 字节段（头部注释/空行原样保留），逐段 yaml 解析取 Name 匹配 original，命中段的字节区间
// 整段替换为新渲染块——之前与之后的全部原文字节零漂移（注释、排版、未动条目）。
// 逐段解析（而非字符串匹配条目名）对引号/转义/任意合法 yaml 格式鲁棒；定位失败显式报错
// 不写（数据与文本不一致的保护）。
func replaceCatalogEntryInText(src []byte, original string, e PocScan.CatalogEntry) ([]byte, error) {
	locs := catalogEntryStartRe.FindAllIndex(src, -1)
	if len(locs) == 0 {
		return nil, fmt.Errorf("源文件中未找到任何条目起始标记，拒绝写回（文件格式异常？）")
	}
	// 先全量扫描定位同名块：命中 0 = 数据与文本不一致；命中 >1 = 文件存在同名重复条目
	// （手改异常态）——两种情况都拒绝写回（只替换首个会静默残留旧块，全文校验也发现不了）
	matchIdx := -1
	matches := 0
	for i, loc := range locs {
		start := loc[0]
		end := len(src)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		// 块文本顶层是序列（"- name:" 前缀）——解析目标必须是切片取首元素
		var parsed []PocScan.CatalogEntry
		if uerr := yaml.Unmarshal(src[start:end], &parsed); uerr != nil || len(parsed) == 0 {
			continue // 不可解析块跳过（按原文保留，调用方全文解析校验兜底）
		}
		if parsed[0].Name == original {
			matches++
			matchIdx = i
		}
	}
	if matches == 0 {
		return nil, fmt.Errorf("条目「%s」未在源文件文本中定位到（数据与文件不一致）", original)
	}
	if matches > 1 {
		return nil, fmt.Errorf("源文件中存在 %d 个同名条目「%s」（手改异常态），拒绝写回——请先人工清理同名重复", matches, original)
	}
	newBlock, rerr := renderCatalogEntryText(e, bytes.Contains(src, []byte("\r\n")))
	if rerr != nil {
		return nil, fmt.Errorf("渲染条目失败: %w", rerr)
	}
	start := locs[matchIdx][0]
	end := len(src)
	if matchIdx+1 < len(locs) {
		end = locs[matchIdx+1][0]
	}
	var out []byte
	out = append(out, src[:start]...) // 目标块之前的全部原文（注释+未动块）
	out = append(out, newBlock...)
	out = append(out, src[end:]...) // 目标块之后的全部原文
	return out, nil
}

// appendCatalogEntryToText 转正追加：新条目块追加到源文本末尾（缺尾换行先补；行尾适配源文件风格）。
func appendCatalogEntryToText(src []byte, e PocScan.CatalogEntry) ([]byte, error) {
	block, rerr := renderCatalogEntryText(e, bytes.Contains(src, []byte("\r\n")))
	if rerr != nil {
		return nil, fmt.Errorf("渲染条目失败: %w", rerr)
	}
	out := make([]byte, 0, len(src)+len(block)+1)
	out = append(out, src...)
	if len(out) > 0 && out[len(out)-1] != '\n' {
		out = append(out, '\n')
	}
	out = append(out, block...)
	return out, nil
}
