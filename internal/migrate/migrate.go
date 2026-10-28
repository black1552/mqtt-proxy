// Package migrate 实现一个小型、自包含的 SQL 迁移工具。迁移文件会内嵌到
// 二进制中，使单个可执行文件即可自行搭建（或回滚）数据库结构，无需外部依赖。
package migrate

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

//go:embed sql/*.sql
var sqlFS embed.FS

// 文件名格式：<version>_<name>.<up|down>.sql
var fileRE = regexp.MustCompile(`^(\d+)_(.+)\.(up|down)\.sql$`)

// Migration 是由数字版本号标识的一对 up/down 脚本。
type Migration struct {
	Version int
	Name    string
	Up      string
	Down    string
}

// Migrator 针对 *sql.DB 运行迁移。
type Migrator struct {
	db *sql.DB
}

// New 返回一个绑定到 db 的 Migrator。
func New(db *sql.DB) *Migrator { return &Migrator{db: db} }

// load 将每个内嵌脚本解析为按版本排序的迁移列表。
func load() ([]Migration, error) {
	entries, err := fs.ReadDir(sqlFS, "sql")
	if err != nil {
		return nil, err
	}
	byVersion := map[int]*Migration{}
	for _, e := range entries {
		m := fileRE.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		version, err := strconv.Atoi(m[1])
		if err != nil {
			return nil, fmt.Errorf("bad version in %s: %w", e.Name(), err)
		}
		body, err := sqlFS.ReadFile("sql/" + e.Name())
		if err != nil {
			return nil, err
		}
		mig := byVersion[version]
		if mig == nil {
			mig = &Migration{Version: version, Name: m[2]}
			byVersion[version] = mig
		}
		if m[3] == "up" {
			mig.Up = string(body)
		} else {
			mig.Down = string(body)
		}
	}
	out := make([]Migration, 0, len(byVersion))
	for _, mig := range byVersion {
		if mig.Up == "" {
			return nil, fmt.Errorf("migration %04d missing up script", mig.Version)
		}
		out = append(out, *mig)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}

// ensureVersionTable 在缺失时创建版本记录表。
func (m *Migrator) ensureVersionTable() error {
	_, err := m.db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
        version    INTEGER PRIMARY KEY,
        name       TEXT NOT NULL,
        applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
    )`)
	return err
}

// Version 返回已应用的最高迁移版本号（无迁移时为 0）。
func (m *Migrator) Version() (int, error) {
	if err := m.ensureVersionTable(); err != nil {
		return 0, err
	}
	var v sql.NullInt64
	err := m.db.QueryRow(`SELECT MAX(version) FROM schema_migrations`).Scan(&v)
	if err != nil && err != sql.ErrNoRows {
		return 0, err
	}
	if !v.Valid {
		return 0, nil
	}
	return int(v.Int64), nil
}

// Pending 返回尚未应用的迁移。
func (m *Migrator) Pending() ([]Migration, error) {
	migrations, err := load()
	if err != nil {
		return nil, err
	}
	cur, err := m.Version()
	if err != nil {
		return nil, err
	}
	var pending []Migration
	for _, mig := range migrations {
		if mig.Version > cur {
			pending = append(pending, mig)
		}
	}
	return pending, nil
}

// Up 应用所有待处理迁移，每个都在独立事务中执行。
func (m *Migrator) Up() ([]int, error) {
	pending, err := m.Pending()
	if err != nil {
		return nil, err
	}
	var applied []int
	for _, mig := range pending {
		if err := m.applyUp(mig); err != nil {
			return applied, fmt.Errorf("apply up %04d_%s: %w", mig.Version, mig.Name, err)
		}
		applied = append(applied, mig.Version)
	}
	return applied, nil
}

func (m *Migrator) applyUp(mig Migration) error {
	tx, err := m.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(mig.Up); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO schema_migrations (version, name) VALUES (?, ?)`, mig.Version, mig.Name); err != nil {
		return err
	}
	return tx.Commit()
}

// Down 回滚最近一次已应用的迁移。
func (m *Migrator) Down() (int, error) {
	cur, err := m.Version()
	if err != nil {
		return 0, err
	}
	if cur == 0 {
		return 0, nil
	}
	migrations, err := load()
	if err != nil {
		return 0, err
	}
	for i := len(migrations) - 1; i >= 0; i-- {
		mig := migrations[i]
		if mig.Version != cur {
			continue
		}
		if strings.TrimSpace(mig.Down) == "" {
			return 0, fmt.Errorf("migration %04d_%s has no down script", mig.Version, mig.Name)
		}
		tx, err := m.db.Begin()
		if err != nil {
			return 0, err
		}
		defer func() { _ = tx.Rollback() }()
		if _, err := tx.Exec(mig.Down); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(`DELETE FROM schema_migrations WHERE version = ?`, mig.Version); err != nil {
			return 0, err
		}
		if err := tx.Commit(); err != nil {
			return 0, err
		}
		return mig.Version, nil
	}
	return 0, fmt.Errorf("version %d not found in scripts", cur)
}

// Force 在不执行脚本的情况下标记数据库版本号，适用于接管一个
// 已完成迁移的数据库。
func (m *Migrator) Force(version int) error {
	if err := m.ensureVersionTable(); err != nil {
		return err
	}
	if version == 0 {
		_, err := m.db.Exec(`DELETE FROM schema_migrations`)
		return err
	}
	migrations, err := load()
	if err != nil {
		return err
	}
	name := "forced"
	for _, mig := range migrations {
		if mig.Version == version {
			name = mig.Name
			break
		}
	}
	_, err = m.db.Exec(`INSERT INTO schema_migrations (version, name) VALUES (?, ?)
        ON CONFLICT(version) DO UPDATE SET name = excluded.name`, version, name)
	return err
}

// All 返回所有已知迁移，按版本排序。
func (m *Migrator) All() ([]Migration, error) { return load() }
