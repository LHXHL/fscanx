package main

// binds 存储层（BL-POC-005）：SQLite 主存"外置层" + binds.yaml 文件快照双轨。
//
// 分层决策：DB 只存用户外置条目（= 原 binds.yaml 的内容），内置种子保持 PocScan embed
// 打底，fscanx 扫描器零改动——启动时照旧读 种子 ⊕ binds.yaml。
//
// 同步机制（单向权威 + 手改吸收）：
//   写：UI 增删改 → DB 单条事务 → 整文件重生成（temp+rename 原子写）→ meta 记录文件 sha256
//   读：API 操作前 absorbBindsFile 对齐——文件哈希 ≠ meta = 被手改 → 全量吸收进 DB → 重生成；
//       文件缺失不视为清空，按 DB 现状重生成（防误删丢数据）；坏 yaml 显式报错（手改坏了必须暴露，不能静默吞掉）。

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"github.com/killmonday/fscanx/PocScan"
	"gopkg.in/yaml.v2"
	_ "modernc.org/sqlite"
)

const metaKeyFileHash = "binds_file_hash"

// bindsYamlFile binds.yaml 顶层结构（flush 与解析共用同一 schema）。
type bindsYamlFile struct {
	Bind []PocScan.Bind `yaml:"binds"`
}

// openBindsDB 打开（必要时建表）binds.db。单用户本地端：连接数收敛为 1，无锁竞争窗口。
func openBindsDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	for _, q := range []string{
		`CREATE TABLE IF NOT EXISTS binds (
			product TEXT PRIMARY KEY,
			pocs    TEXT NOT NULL
		)`,
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

// dbListBinds 读 DB 外置层（product 序——展示与 flush 顺序确定性）。
func dbListBinds(db *sql.DB) ([]PocScan.Bind, error) {
	rows, err := db.Query(`SELECT product, pocs FROM binds ORDER BY product`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PocScan.Bind
	for rows.Next() {
		var product, pocsJSON string
		if err := rows.Scan(&product, &pocsJSON); err != nil {
			return nil, err
		}
		var pocs []string
		if err := json.Unmarshal([]byte(pocsJSON), &pocs); err != nil {
			return nil, fmt.Errorf("产品 %s 的 pocs 数据损坏: %w", product, err)
		}
		out = append(out, PocScan.Bind{Product: product, Pocs: pocs})
	}
	return out, rows.Err()
}

// seedPocsOf 产品的内置种子 pocs（种子起源产品的追加基线；无则 nil）。
func seedPocsOf(product string) []string {
	seed, err := PocScan.SeedBinds()
	if err != nil {
		return nil
	}
	for _, b := range seed {
		if b.Product == product {
			return b.Pocs
		}
	}
	return nil
}

// bindsAdd 添加绑定关系（upsert-append 语义）：产品不存在则新建；已存在则把 pocs
// 追加去重后更新——POC 页快捷绑定免读全量，绑定页同名由前端先行拦截、服务端兜底合并不丢数据。
// 种子起源产品：以种子 pocs 为基线追加（外置同名整条覆盖引擎语义下，直接插入会使种子原绑定全部失效）。
func bindsAdd(db *sql.DB, yamlPath, product string, pocs []string) error {
	return dbMutate(db, yamlPath, func(tx *sql.Tx) error {
		var existing string
		err := tx.QueryRow(`SELECT pocs FROM binds WHERE product = ?`, product).Scan(&existing)
		switch {
		case err == sql.ErrNoRows:
			base := pocs
			if isSeedProduct(product) {
				base = appendDedup(seedPocsOf(product), pocs)
			}
			b, e := json.Marshal(base)
			if e != nil {
				return e
			}
			_, err := tx.Exec(`INSERT INTO binds (product, pocs) VALUES (?, ?)`, product, string(b))
			return err
		case err != nil:
			return err
		}
		var cur []string
		if err := json.Unmarshal([]byte(existing), &cur); err != nil {
			return fmt.Errorf("产品「%s」已有条目数据损坏: %w", product, err)
		}
		b, e := json.Marshal(appendDedup(cur, pocs))
		if e != nil {
			return e
		}
		_, err = tx.Exec(`UPDATE binds SET pocs = ? WHERE product = ?`, string(b), product)
		return err
	})
}

// isOuterProduct 产品是否已存在于外置层（DB）。不在且在种子中 = "种子起源行"（只活在合并视图）。
func isOuterProduct(db *sql.DB, product string) (bool, error) {
	var dummy string
	err := db.QueryRow(`SELECT pocs FROM binds WHERE product = ?`, product).Scan(&dummy)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// isSeedProduct 产品是否在内置种子中（种子起源判定用）。
// 种子解析失败时保守返回 false（按"不存在"处理——错误语义退化为"不存在绑定"而非静默放行）。
func isSeedProduct(product string) bool {
	seed, err := PocScan.SeedBinds()
	if err != nil {
		return false
	}
	for _, b := range seed {
		if b.Product == product {
			return true
		}
	}
	return false
}

// bindsEdit 按 original 定位行更新 product 与 pocs；改名时校验新名不与其他条目冲突。
// 种子起源行（original 不在外置层）：落墓碑 {original,[]} 失效种子条目，再写入新条目/覆写
// （引擎 MergeBinds 同名覆盖语义，扫描器零改动；不加墓碑则种子原条目继续生效）。
func bindsEdit(db *sql.DB, yamlPath, original, product string, pocs []string) error {
	inOuter, err := isOuterProduct(db, original)
	if err != nil {
		return err
	}
	if !inOuter && !isSeedProduct(original) {
		return fmt.Errorf("产品「%s」不存在绑定", original)
	}
	originalIsTombstone := false
	if inOuter {
		var pj string
		if err := db.QueryRow(`SELECT pocs FROM binds WHERE product = ?`, original).Scan(&pj); err != nil {
			return err
		}
		var p []string
		if json.Unmarshal([]byte(pj), &p) == nil && len(p) == 0 {
			originalIsTombstone = true
		}
	}
	return dbMutate(db, yamlPath, func(tx *sql.Tx) error {
		pj, err := json.Marshal(pocs)
		if err != nil {
			return err
		}
		if product != original {
			// 改名目标冲突检查：目标为墓碑（视图不可见）时允许覆写
			var existing string
			err := tx.QueryRow(`SELECT pocs FROM binds WHERE product = ?`, product).Scan(&existing)
			if err == nil {
				var ep []string
				if json.Unmarshal([]byte(existing), &ep) == nil && len(ep) > 0 {
					return fmt.Errorf("产品「%s」已存在绑定", product)
				}
			} else if err != sql.ErrNoRows {
				return err
			}
		}
		upsert := `INSERT INTO binds (product, pocs) VALUES (?, ?)
			ON CONFLICT(product) DO UPDATE SET pocs = excluded.pocs`
		// 原行移除：外置实体行删行；种子起源行落墓碑（失效内嵌条目）；
		// 墓碑行不可删（删了种子绑定会复活）——保留，仅当同名编辑时被下方 upsert 覆写复活
		if inOuter && !originalIsTombstone {
			if _, err := tx.Exec(`DELETE FROM binds WHERE product = ?`, original); err != nil {
				return err
			}
		}
		if !inOuter {
			tomb, e := json.Marshal([]string{})
			if e != nil {
				return e
			}
			if _, err := tx.Exec(upsert, original, string(tomb)); err != nil {
				return err
			}
		}
		// 新行写入（同名编辑时 product == original，upsert 即覆写）
		_, err = tx.Exec(upsert, product, string(pj))
		return err
	})
}

// bindsDelete 按产品删除绑定条目。外置行直接删；种子起源行落墓碑 {product,[]}
// （合并视图隐藏 + 引擎绑定失效——种子在内嵌层，外置只能覆盖不能移除）。
// 幂等：重复删除墓碑行直接返回成功（墓碑不可移除——移除会让种子绑定复活）。
func bindsDelete(db *sql.DB, yamlPath, product string) error {
	inOuter, err := isOuterProduct(db, product)
	if err != nil {
		return err
	}
	if !inOuter && !isSeedProduct(product) {
		return fmt.Errorf("产品「%s」不存在绑定", product)
	}
	return dbMutate(db, yamlPath, func(tx *sql.Tx) error {
		if !inOuter {
			tomb, e := json.Marshal([]string{})
			if e != nil {
				return e
			}
			_, err := tx.Exec(`INSERT INTO binds (product, pocs) VALUES (?, ?)
				ON CONFLICT(product) DO UPDATE SET pocs = excluded.pocs`, product, string(tomb))
			return err
		}
		var pj string
		if err := tx.QueryRow(`SELECT pocs FROM binds WHERE product = ?`, product).Scan(&pj); err != nil {
			return err
		}
		var p []string
		if json.Unmarshal([]byte(pj), &p) == nil && len(p) == 0 {
			return nil // 已是墓碑 = 已删除，幂等成功
		}
		res, err := tx.Exec(`DELETE FROM binds WHERE product = ?`, product)
		if err != nil {
			return err
		}
		if n, e := res.RowsAffected(); e == nil && n == 0 {
			return fmt.Errorf("产品「%s」不存在绑定", product)
		}
		return nil
	})
}

// dbMutate 单条变更事务 + 提交后 flush 文件。fn 内为业务校验错误；flush 的 WriteFile/Rename
// 失败时 meta 仍是旧哈希、不会触发哈希对齐自愈——自愈依赖"下一次任意写操作的无条件 flush"
// （每次增删改都会全量重生成）。若用户放弃后续操作，DB 与文件会持续分叉直到下一次写。
func dbMutate(db *sql.DB, yamlPath string, fn func(tx *sql.Tx) error) error {
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
	return flushBindsFile(db, yamlPath)
}

// flushBindsFile DB 外置层 → binds.yaml（temp+rename 原子写）→ meta 记录新哈希。
func flushBindsFile(db *sql.DB, yamlPath string) error {
	outer, err := dbListBinds(db)
	if err != nil {
		return err
	}
	out, err := yaml.Marshal(bindsYamlFile{Bind: outer})
	if err != nil {
		return err
	}
	if err := os.WriteFile(yamlPath+".tmp", out, 0644); err != nil {
		return err
	}
	if err := os.Rename(yamlPath+".tmp", yamlPath); err != nil {
		return err
	}
	return setMeta(db, metaKeyFileHash, fileHash(out))
}

// absorbBindsFile 读写前的文件对齐检查（见文件头注释）。
func absorbBindsFile(db *sql.DB, yamlPath string) error {
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		if os.IsNotExist(err) {
			// 文件缺失：DB 有数据 → 重生成（防误删丢数据）；DB 空 → 纯种子起步，无事可做
			outer, err := dbListBinds(db)
			if err != nil {
				return err
			}
			if len(outer) > 0 {
				return flushBindsFile(db, yamlPath)
			}
			return nil
		}
		return err
	}
	if getMeta(db, metaKeyFileHash) == fileHash(data) {
		return nil // 未被手改，DB 即现状
	}
	outer, err := PocScan.LoadBindsFromBytes(data)
	if err != nil {
		return fmt.Errorf("binds.yaml 解析失败（手改内容有误，请修正或删除该文件后重试）: %w", err)
	}
	// 全量吸收：文件是手改的最新事实，DB 外置层 := 文件内容（增/删/改三种手改统一处理；
	// 手工清空的文件 = 清空外置层，是显式编辑行为，与误删文件区分）。
	// 手改文件含同名重复 product：后者覆盖前者（与 MergeBinds 覆盖语义一致，不因主键冲突锁死接口）
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM binds`); err != nil {
		_ = tx.Rollback()
		return err
	}
	for _, b := range outer {
		pj, e := json.Marshal(b.Pocs)
		if e != nil {
			_ = tx.Rollback()
			return e
		}
		if _, err := tx.Exec(`INSERT INTO binds (product, pocs) VALUES (?, ?)
			ON CONFLICT(product) DO UPDATE SET pocs = excluded.pocs`, b.Product, string(pj)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("吸收产品 %s 失败: %w", b.Product, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return flushBindsFile(db, yamlPath) // 吸收后 flush 统一格式并记录哈希
}

func getMeta(db *sql.DB, key string) string {
	var v string
	_ = db.QueryRow(`SELECT value FROM meta WHERE key = ?`, key).Scan(&v)
	return v
}

func setMeta(db *sql.DB, key, value string) error {
	_, err := db.Exec(`INSERT INTO meta (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

func fileHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// appendDedup 列表合并去重（保持原序，新值追加在尾部）。
func appendDedup(cur, add []string) []string {
	seen := make(map[string]bool, len(cur)+len(add))
	out := make([]string, 0, len(cur)+len(add))
	for _, s := range cur {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	for _, s := range add {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
