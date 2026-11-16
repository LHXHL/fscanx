package main

// 指纹条目存储层（BL-POC-006）：SQLite 主存 + 明文 JSON 快照双轨（与 binds_db.go 同分层，
// 但无墓碑——指纹权威只有 DB 一层，快照由 DB 重生成，不存在"DB 删行后种子复活"路径）。
//
// 字节契约（重生成器核心）：两源明文格式不同且元素间分隔符不统一（goby `, ` 同行、
// fingers_http ",\n    "，尾部另有异形嵌套条目），统一重序列化=全库格式漂移。因此种子
// 阶段按 json.Decoder 逐元素切分原文，保存 {前缀, 逐间隙分隔符, 后缀} 模板与各元素
// 原始字节；重生成 = 前缀 + 依次拼接（未动条目字节原样保留，改动条目按所在源格式契约
// 序列化），保证未动区域逐字节不变。
//
// 同步机制（单向权威 + 手改吸收，同 binds）：写 = DB 单条事务 → 整文件重生成（temp+rename
// 原子写）→ meta 记录 sha256；读 = API 前对齐——文件哈希 ≠ meta = 被手改 → 以文件为准
// 全量吸收进 DB；文件缺失按 DB 重生成防误删；坏 JSON 显式报错（明文保持原状）。

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"

	_ "modernc.org/sqlite"
)

// fingerTemplate 明文文件的拼装模板：前缀（文件头到首元素）、逐间隙分隔符（元素 i 与
// i+1 之间的原始字节，含逗号/换行/缩进）、后缀（末元素到文件尾）。
type fingerTemplate struct {
	Prefix string   `json:"prefix"`
	Gaps   []string `json:"gaps"`
	Suffix string   `json:"suffix"`
}

// fingerEntryMeta 条目表行（raw=元素原始 JSON 字节，name 冗余列供搜索）。
type fingerEntryMeta struct {
	ID   int64
	Name string
	Raw  string
}

// openFingerDB 打开（必要时建表）finger.db。单用户本地端：连接数收敛为 1。
func openFingerDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	for _, q := range []string{
		`CREATE TABLE IF NOT EXISTS finger_entries (
			id   INTEGER PRIMARY KEY AUTOINCREMENT,
			file TEXT NOT NULL,
			name TEXT NOT NULL,
			raw  BLOB NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_finger_entries_file ON finger_entries (file, id)`,
		`CREATE TABLE IF NOT EXISTS meta (
			key   TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
	} {
		if _, err := db.Exec(q); err != nil {
			db.Close()
			return nil, fmt.Errorf("建表失败: %w", err)
		}
	}
	return db, nil
}

// parseFingerArray 把明文 JSON 数组切分为 {模板, 元素原始字节, 各元素 name}。
// 元素边界用自研扫描器（括号深度+字符串感知）逐字节切——json.Decoder 的 InputOffset
// 会在 Decode 后前视越过元素边界（为判断 More 预读分隔符），间隙字节会被吞掉，
// 不可用。未动区域零重排，分隔符原样保留。
func parseFingerArray(data []byte) (fingerTemplate, []fingerEntryMeta, error) {
	p := skipJSONSpace(data, 0)
	if p >= len(data) || data[p] != '[' {
		return fingerTemplate{}, nil, fmt.Errorf("顶层不是 JSON 数组")
	}
	p++ // 越过 '['，从首个元素开始扫描
	var tpl fingerTemplate
	var elems []fingerEntryMeta
	first := true
	for {
		i := skipJSONSpace(data, p)
		if i >= len(data) {
			return fingerTemplate{}, nil, fmt.Errorf("数组未闭合")
		}
		if data[i] == ']' {
			if first {
				// 空数组（手改清空）：'[' 后到 ']' 归前缀，防重生成丢 '['
				tpl.Prefix = string(data[:i])
				tpl.Suffix = string(data[i:])
			} else {
				// 末元素结尾到 ']' 之间的空白（含换行）归 suffix，重生成才能逐字节还原
				tpl.Suffix = string(data[p:])
			}
			break
		}
		if first {
			tpl.Prefix = string(data[:i]) // '[' 及其后空白
			first = false
		} else {
			if data[i] != ',' {
				return fingerTemplate{}, nil, fmt.Errorf("数组元素间缺少逗号（位置 %d）", i)
			}
			// 间隙 = 上一元素结尾到逗号后的空白止（含逗号与换行缩进，原样保留）
			j := skipJSONSpace(data, i+1)
			tpl.Gaps = append(tpl.Gaps, string(data[p:j]))
			i = j
		}
		end, err := scanJSONValue(data, i)
		if err != nil {
			return fingerTemplate{}, nil, fmt.Errorf("元素扫描失败: %w", err)
		}
		raw := data[i:end]
		if !json.Valid(raw) {
			return fingerTemplate{}, nil, fmt.Errorf("元素不是合法 JSON")
		}
		var nameOnly struct {
			Name string `json:"name"`
		}
		_ = json.Unmarshal(raw, &nameOnly)
		elems = append(elems, fingerEntryMeta{Name: nameOnly.Name, Raw: string(raw)})
		p = end
	}
	return tpl, elems, nil
}

// skipJSONSpace 跳过空白（空格/制表/换行/回车）。
func skipJSONSpace(data []byte, i int) int {
	for i < len(data) {
		switch data[i] {
		case ' ', '\t', '\n', '\r':
			i++
		default:
			return i
		}
	}
	return i
}

// scanJSONValue 从值首字节扫描到值结束（深度计数，字符串内不感知括号）。
func scanJSONValue(data []byte, i int) (int, error) {
	depth, inStr, esc := 0, false, false
	for ; i < len(data); i++ {
		c := data[i]
		if inStr {
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == '"':
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{', '[':
			depth++
		case '}', ']':
			depth--
			if depth == 0 {
				return i + 1, nil
			}
		}
	}
	return 0, fmt.Errorf("JSON 值未闭合")
}

// serializeFingerArray 模板 + 元素 → 明文全文（未动元素字节原样，间隙原样）。
// 元素数超过原间隙数（新增/复制追加）时复用末间隙衔接，不越界。
func serializeFingerArray(tpl fingerTemplate, elems []fingerEntryMeta) []byte {
	var b bytes.Buffer
	b.WriteString(tpl.Prefix)
	for i, e := range elems {
		if i > 0 {
			if i-1 < len(tpl.Gaps) {
				b.WriteString(tpl.Gaps[i-1])
			} else if len(tpl.Gaps) > 0 {
				b.WriteString(tpl.Gaps[len(tpl.Gaps)-1])
			}
		}
		b.WriteString(e.Raw)
	}
	b.WriteString(tpl.Suffix)
	return b.Bytes()
}

// fingerMetaKey 模板与哈希的 meta 键。
func fingerMetaKey(file, kind string) string { return "finger_" + kind + "_" + file }

// fingerCountDB 某 file 的条目数。
func fingerCountDB(db *sql.DB, file string) (int, error) {
	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM finger_entries WHERE file = ?`, file).Scan(&n)
	return n, err
}

// seedFingerFile 明文 → DB 全量入库（首启种子与手改吸收共用：文件是最新事实）。
func seedFingerFile(db *sql.DB, file string, data []byte) error {
	tpl, elems, err := parseFingerArray(data)
	if err != nil {
		return fmt.Errorf("%s 解析失败（手改内容有误，请修正该文件后重试）: %w", file, err)
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM finger_entries WHERE file = ?`, file); err != nil {
		_ = tx.Rollback()
		return err
	}
	for _, e := range elems {
		if _, err := tx.Exec(`INSERT INTO finger_entries (file, name, raw) VALUES (?, ?, ?)`, file, e.Name, e.Raw); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	tplJSON, err := json.Marshal(tpl)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := setMetaTx(tx, fingerMetaKey(file, "tpl"), string(tplJSON)); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := setMetaTx(tx, fingerMetaKey(file, "hash"), fileHash(data)); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// absorbFingerFile 读写前的文件对齐检查（同 binds absorbBindsFile 语义）：
// 哈希一致 = DB 即现状；不一致 = 被手改 → 以文件为准全量吸收；文件缺失按 DB 重生成。
func absorbFingerFile(db *sql.DB, path, file string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			n, e := fingerCountDB(db, file)
			if e != nil {
				return e
			}
			if n > 0 {
				return regenerateFingerFile(db, path, file) // 防误删丢数据
			}
			return fmt.Errorf("指纹明文不存在: %s（检查 -root 是否指向仓库根）", path)
		}
		return err
	}
	if getMeta(db, fingerMetaKey(file, "hash")) == fileHash(data) {
		return nil
	}
	if err := seedFingerFile(db, file, data); err != nil {
		return err
	}
	return regenerateFingerFile(db, path, file) // 吸收后重生成统一记录哈希
}

// regenerateFingerFile DB → 明文全文（temp+rename 原子写）→ meta 记录新哈希。
func regenerateFingerFile(db *sql.DB, path, file string) error {
	elems, tpl, err := fingerAllEntries(db, file)
	if err != nil {
		return err
	}
	out := serializeFingerArray(tpl, elems)
	if err := os.WriteFile(path+".tmp", out, 0644); err != nil {
		return err
	}
	if err := os.Rename(path+".tmp", path); err != nil {
		return err
	}
	return setMeta(db, fingerMetaKey(file, "hash"), fileHash(out))
}

// fingerAllEntries 某 file 全部条目（id 序=文件原序）+ 模板。
func fingerAllEntries(db *sql.DB, file string) ([]fingerEntryMeta, fingerTemplate, error) {
	rows, err := db.Query(`SELECT id, name, raw FROM finger_entries WHERE file = ? ORDER BY id`, file)
	if err != nil {
		return nil, fingerTemplate{}, err
	}
	defer rows.Close()
	var elems []fingerEntryMeta
	for rows.Next() {
		var e fingerEntryMeta
		if err := rows.Scan(&e.ID, &e.Name, &e.Raw); err != nil {
			return nil, fingerTemplate{}, err
		}
		elems = append(elems, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fingerTemplate{}, err
	}
	tplJSON := getMeta(db, fingerMetaKey(file, "tpl"))
	var tpl fingerTemplate
	if err := json.Unmarshal([]byte(tplJSON), &tpl); err != nil {
		return nil, fingerTemplate{}, fmt.Errorf("%s 模板数据损坏: %w", file, err)
	}
	return elems, tpl, nil
}

// fingerMutate 单条变更事务 + 提交后重生成文件（语义同 binds dbMutate）。
func fingerMutate(db *sql.DB, path, file string, fn func(tx *sql.Tx) error) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return regenerateFingerFile(db, path, file)
}

// fingerAdd 追加新条目（文件尾部；新元素用末间隙衔接，格式按所在源契约由调用方序列化好）。
func fingerAdd(db *sql.DB, path, file, name, raw string) error {
	return fingerMutate(db, path, file, func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT INTO finger_entries (file, name, raw) VALUES (?, ?, ?)`, file, name, raw)
		return err
	})
}

// fingerEditRaw 按 id 整条替换原始字节（原文编辑模式：内容即所写）。
func fingerEditRaw(db *sql.DB, path, file string, id int64, name, raw string) error {
	return fingerMutate(db, path, file, func(tx *sql.Tx) error {
		res, err := tx.Exec(`UPDATE finger_entries SET name = ?, raw = ? WHERE file = ? AND id = ?`, name, raw, file, id)
		if err != nil {
			return err
		}
		if n, e := res.RowsAffected(); e != nil {
			return e
		} else if n == 0 {
			return fmt.Errorf("条目不存在（可能已被删除或手改吸收重排）")
		}
		return nil
	})
}

// fingerDelete 按 id 删条目（删行即消失——权威单层无复活路径；手改明文加回会被吸收成新条目）。
func fingerDelete(db *sql.DB, path, file string, id int64) error {
	return fingerMutate(db, path, file, func(tx *sql.Tx) error {
		res, err := tx.Exec(`DELETE FROM finger_entries WHERE file = ? AND id = ?`, file, id)
		if err != nil {
			return err
		}
		if n, e := res.RowsAffected(); e != nil {
			return e
		} else if n == 0 {
			return fmt.Errorf("条目不存在")
		}
		return nil
	})
}

// fingerGet 某 file 单条（id 定位；重名条目各自独立）。
func fingerGet(db *sql.DB, file string, id int64) (fingerEntryMeta, error) {
	var e fingerEntryMeta
	err := db.QueryRow(`SELECT id, name, raw FROM finger_entries WHERE file = ? AND id = ?`, file, id).Scan(&e.ID, &e.Name, &e.Raw)
	return e, err
}

// setMetaTx 事务内写 meta（种子/吸收路径用；非事务版 setMeta 在 binds_db.go）。
func setMetaTx(tx *sql.Tx, key, value string) error {
	_, err := tx.Exec(`INSERT INTO meta (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}
