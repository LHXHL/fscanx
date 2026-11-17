package main

// 产品册存储层（BL-POC-009）：SQLite 主存"外置层" + catalog.yaml 文件快照双轨。
//
// 分层：出厂产品册 = PocScan embed（mylib/finger/resources/catalog.yaml，编译期）；外置层 =
// 本 DB（catalog.db，二进制旁）。快照文件 = 二进制旁 catalog.yaml（与出厂文件同名同格式、
// 顶层条目列表）——fscanx 扫描器 main 早期经 Plugins.InitEngineAliases 读取：alias 段注入
// 识别引擎归一表、pocs 段并入绑定合并（同标准名整条覆盖出厂、异名追加）。
//
// 同步机制（同 binds_db 模式）：写 → DB 单条事务 → 整文件重生成（temp+rename）→ meta 记
// sha256；读写前 absorbCatalogFile 对齐——哈希 ≠ meta = 被手改 → 全量吸收 → 重生成；文件缺
// 失按 DB 现状重生成；坏 yaml 显式报错。
//
// 墓碑语义（同 BL-POC-005）：出厂条目的删除落 {alias 保留, pocs:[]} 墓碑——绑定失效（扫描
// 器合并空 pocs = 无绑定）而别名归一保持（识别显示统一不应随绑定删除而破坏）；合并视图过滤
// 墓碑不展示；重新添加同名条目 = 自然复活。幂等：墓碑不可被二次删除移除。

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"

	"github.com/killmonday/fscanx/PocScan"
	"gopkg.in/yaml.v2"
	_ "modernc.org/sqlite"
)

const metaKeyCatalogFileHash = "catalog_file_hash"

// openCatalogDB 打开（必要时建表）catalog.db。
func openCatalogDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	for _, q := range []string{
		`CREATE TABLE IF NOT EXISTS entries (
			name    TEXT PRIMARY KEY,
			vendor  TEXT NOT NULL DEFAULT '',
			product TEXT NOT NULL DEFAULT '',
			alias   TEXT NOT NULL DEFAULT 'null',
			pocs    TEXT NOT NULL DEFAULT 'null'
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

// dbListEntries 读 DB 外置层全量条目（name 序——展示与 flush 顺序确定性）。
func dbListEntries(db *sql.DB) ([]PocScan.CatalogEntry, error) {
	rows, err := db.Query(`SELECT name, vendor, product, alias, pocs FROM entries ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PocScan.CatalogEntry
	for rows.Next() {
		var e PocScan.CatalogEntry
		var aliasJSON, pocsJSON string
		if err := rows.Scan(&e.Name, &e.Vendor, &e.Product, &aliasJSON, &pocsJSON); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(aliasJSON), &e.Alias); err != nil {
			return nil, fmt.Errorf("条目 %s 的 alias 数据损坏: %w", e.Name, err)
		}
		if err := json.Unmarshal([]byte(pocsJSON), &e.Pocs); err != nil {
			return nil, fmt.Errorf("条目 %s 的 pocs 数据损坏: %w", e.Name, err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// dbUpsertEntry 写入（整条覆盖语义；墓碑行同键覆写 = 复活）。
func dbUpsertEntry(tx *sql.Tx, e PocScan.CatalogEntry) error {
	aliasJSON, err := json.Marshal(e.Alias)
	if err != nil {
		return err
	}
	pocsJSON, err := json.Marshal(e.Pocs)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO entries (name, vendor, product, alias, pocs) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET vendor = excluded.vendor, product = excluded.product,
		alias = excluded.alias, pocs = excluded.pocs`, e.Name, e.Vendor, e.Product, string(aliasJSON), string(pocsJSON))
	return err
}

// isOuterEntry 条目是否在外置 DB 层。
func isOuterEntry(db *sql.DB, name string) (bool, error) {
	var dummy string
	err := db.QueryRow(`SELECT name FROM entries WHERE name = ?`, name).Scan(&dummy)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// isBuiltInEntry 条目是否在出厂产品册（embed）。
func isBuiltInEntry(name string) bool {
	full, err := PocScan.FullCatalog()
	if err != nil {
		return false
	}
	for _, e := range full {
		if e.Name == name {
			return true
		}
	}
	return false
}

// builtInEntry 取出厂条目（不存在返回 ok=false）。
func builtInEntry(name string) (PocScan.CatalogEntry, bool) {
	full, err := PocScan.FullCatalog()
	if err != nil {
		return PocScan.CatalogEntry{}, false
	}
	for _, e := range full {
		if e.Name == name {
			return e, true
		}
	}
	return PocScan.CatalogEntry{}, false
}

// catalogMutate 单条变更事务 + 提交后 flush（binds dbMutate 同模式，注释见彼处）。
func catalogMutate(db *sql.DB, yamlPath string, fn func(tx *sql.Tx) error) error {
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
	return flushCatalogFile(db, yamlPath)
}

// flushCatalogFile DB 外置层 → catalog.yaml（顶层条目列表，temp+rename 原子写）→ meta 记哈希。
func flushCatalogFile(db *sql.DB, yamlPath string) error {
	outer, err := dbListEntries(db)
	if err != nil {
		return err
	}
	out, err := yaml.Marshal(outer)
	if err != nil {
		return err
	}
	if err := os.WriteFile(yamlPath+".tmp", out, 0644); err != nil {
		return err
	}
	if err := os.Rename(yamlPath+".tmp", yamlPath); err != nil {
		return err
	}
	return setMeta(db, metaKeyCatalogFileHash, fileHash(out))
}

// absorbCatalogFile 读写前的手改吸收（binds absorbBindsFile 同模式）。
func absorbCatalogFile(db *sql.DB, yamlPath string) error {
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		if os.IsNotExist(err) {
			outer, err := dbListEntries(db)
			if err != nil {
				return err
			}
			if len(outer) > 0 {
				return flushCatalogFile(db, yamlPath)
			}
			return nil
		}
		return err
	}
	if getMeta(db, metaKeyCatalogFileHash) == fileHash(data) {
		return nil // 未被手改，DB 即现状
	}
	entries, err := PocScan.ParseCatalogEntries(data)
	if err != nil {
		return fmt.Errorf("catalog.yaml 解析失败（手改内容有误，请修正或删除该文件后重试）: %w", err)
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM entries`); err != nil {
		_ = tx.Rollback()
		return err
	}
	for _, e := range entries {
		if e.Name == "" {
			continue
		}
		if err := dbUpsertEntry(tx, e); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("吸收条目 %s 失败: %w", e.Name, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return flushCatalogFile(db, yamlPath)
}

// migrateLegacyBinds 存量迁移（BL-POC-009 验收：迁移前后对同一目标的定向发包集合一致）：
// catalog.db 为空且存在遗留数据源时执行一次——
//   1. binds.db（BL-POC-005 SQLite）：外置行转 entries；墓碑行（空 pocs）转为出厂别名保留
//      + 空 pocs 的墓碑（绑定失效语义不丢）；迁移后改名 binds.db.migrated 防重复迁移。
//   2. binds.yaml（手工外置层）：catalog.yaml 缺失时按旧格式吸收转为 entries。
// 迁移保持绑定效果不变：旧 product 键在新体系按原名立条（识别输出原名时 Contains 相等命中，
// 行为与迁移前一致）；归一不明的条目别名区为空 → 合并视图带警示标记（missing-alias）。
func migrateLegacyBinds(catalogDB *sql.DB, catalogYamlPath, bindsDBPath, bindsYamlPath string) error {
	empty, err := catalogDBEmpty(catalogDB)
	if err != nil {
		return err
	}
	if !empty {
		return nil // 已有数据（非首次启动），不迁移
	}

	// 数据源 1：binds.yaml 手工文件（catalog.yaml 缺失时才吸收——catalog.yaml 已是权威外置层）
	if _, err := os.Stat(bindsYamlPath); err == nil {
		if _, err := os.Stat(catalogYamlPath); os.IsNotExist(err) {
			if err := migrateBindsYaml(catalogDB, bindsYamlPath); err != nil {
				return err
			}
		}
	}

	// 数据源 2：binds.db SQLite
	if _, err := os.Stat(bindsDBPath); err == nil {
		if err := migrateBindsDBData(catalogDB, bindsDBPath); err != nil {
			return err
		}
		if err := os.Rename(bindsDBPath, bindsDBPath+".migrated"); err != nil {
			return fmt.Errorf("迁移后重命名 binds.db 失败: %w", err)
		}
	}

	// 有任何迁移写入才需要 flush（无遗留数据源 = 纯出厂起步，无事可做）
	outer, err := dbListEntries(catalogDB)
	if err != nil {
		return err
	}
	if len(outer) > 0 {
		return flushCatalogFile(catalogDB, catalogYamlPath)
	}
	return nil
}

func catalogDBEmpty(db *sql.DB) (bool, error) {
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM entries`).Scan(&n); err != nil {
		return false, err
	}
	return n == 0, nil
}

// isTombstoneRow 行是否为出厂条目的墓碑（pocs 空 + 名字在出厂册——出厂删除标记）。
func isTombstoneRow(tx *sql.Tx, name string) (bool, error) {
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
	return len(p) == 0 && isBuiltInEntry(name), nil
}

// atomicWrite temp+rename 原子写（内置源写回用；同 binds/finger flush 模式）。
func atomicWrite(path string, data []byte) error {
	if err := os.WriteFile(path+".tmp", data, 0644); err != nil {
		return err
	}
	return os.Rename(path+".tmp", path)
}

// migrateBindsYaml 旧 binds.yaml（{binds:[{product,pocs}]}）→ entries。
func migrateBindsYaml(catalogDB *sql.DB, bindsYamlPath string) error {
	data, err := os.ReadFile(bindsYamlPath)
	if err != nil {
		return err
	}
	binds, err := PocScan.LoadBindsFromBytes(data)
	if err != nil {
		// 旧文件损坏：不阻塞启动（外置层缺失 = 仅出厂兜底），显式提示
		fmt.Printf("[!] 存量 binds.yaml 解析失败，跳过迁移: %v\n", err)
		return nil
	}
	return insertLegacyBinds(catalogDB, binds)
}

// migrateBindsDBData 旧 binds.db 的 binds 表 → entries。
func migrateBindsDBData(catalogDB *sql.DB, bindsDBPath string) error {
	oldDB, err := sql.Open("sqlite", bindsDBPath)
	if err != nil {
		return err
	}
	defer oldDB.Close()
	rows, err := oldDB.Query(`SELECT product, pocs FROM binds ORDER BY product`)
	if err != nil {
		// 无 binds 表 = 空库，无事可做
		return nil
	}
	defer rows.Close()
	var binds []PocScan.Bind
	for rows.Next() {
		var b PocScan.Bind
		var pocsJSON string
		if err := rows.Scan(&b.Product, &pocsJSON); err != nil {
			return err
		}
		if err := json.Unmarshal([]byte(pocsJSON), &b.Pocs); err != nil {
			return fmt.Errorf("存量 binds.db 产品 %s 数据损坏: %w", b.Product, err)
		}
		binds = append(binds, b)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return insertLegacyBinds(catalogDB, binds)
}

// insertLegacyBinds 存量绑定行转产品册条目：普通行按原名立条（别名区空 → 合并视图警示
// missing-alias）；墓碑行（空 pocs）转出厂别名保留 + 空 pocs（绑定失效语义保持）。
func insertLegacyBinds(catalogDB *sql.DB, binds []PocScan.Bind) error {
	tx, err := catalogDB.Begin()
	if err != nil {
		return err
	}
	for _, b := range binds {
		e := PocScan.CatalogEntry{Name: b.Product, Pocs: b.Pocs}
		if len(b.Pocs) == 0 {
			// 墓碑迁移：出厂别名保留（归一不因绑定删除而破坏——BL-POC-009 墓碑语义）
			if bi, ok := builtInEntry(b.Product); ok {
				e.Vendor, e.Product, e.Alias = bi.Vendor, bi.Product, bi.Alias
			}
		}
		if err := dbUpsertEntry(tx, e); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("迁移条目 %s 失败: %w", b.Product, err)
		}
	}
	return tx.Commit()
}
