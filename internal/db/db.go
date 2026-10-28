// Package db 打开代理所使用的内嵌式 SQLite 数据库。它使用纯 Go 的
// modernc.org/sqlite 驱动，因此二进制可以在不依赖 CGO 的情况下构建。
package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // 注册 "sqlite" 驱动
)

// Open 准备磁盘上的数据库：按需创建父目录，并应用并发 MQTT 负载
// 所需的 pragma 设置。
func Open(path string) (*sql.DB, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create db dir: %w", err)
		}
	}

	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)", path)
	handle, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// SQLite 串行化写入；单连接可避免 "database is locked"。
	handle.SetMaxOpenConns(1)
	if err := handle.Ping(); err != nil {
		_ = handle.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	return handle, nil
}
