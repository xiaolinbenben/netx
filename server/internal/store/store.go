package store

import (
	"database/sql"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

// DB 是 SQLite 连接的薄封装。
type DB struct {
	*sql.DB
}

// Open 打开（必要时创建）SQLite 数据库文件。
func Open(path string) (*DB, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, fmt.Errorf("创建数据库目录失败: %w", err)
		}
	}

	dsn := fmt.Sprintf(
		"file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)",
		path,
	)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}
	// SQLite 单写者，连接数限制为 1，从根上避免写冲突
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}
	return &DB{DB: db}, nil
}

// Migrate 按 PRAGMA user_version 依次执行尚未应用的迁移文件。
func Migrate(db *sql.DB, fsys fs.FS) error {
	files, err := fs.Glob(fsys, "migrations/*.sql")
	if err != nil {
		return fmt.Errorf("读取迁移文件失败: %w", err)
	}
	sort.Strings(files)

	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("读取数据库版本失败: %w", err)
	}

	for _, name := range files {
		next, err := migrationVersion(name)
		if err != nil {
			return err
		}
		if next <= version {
			continue
		}
		content, err := fs.ReadFile(fsys, name)
		if err != nil {
			return fmt.Errorf("读取迁移文件 %s 失败: %w", name, err)
		}
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("开启迁移事务失败: %w", err)
		}
		if _, err := tx.Exec(string(content)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("执行迁移 %s 失败: %w", name, err)
		}
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", next)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("更新数据库版本失败: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("提交迁移 %s 失败: %w", name, err)
		}
		version = next
	}
	return nil
}

func migrationVersion(name string) (int, error) {
	base := filepath.Base(name)
	prefix, _, found := strings.Cut(base, "_")
	if !found {
		return 0, fmt.Errorf("迁移文件名不符合 0001_init.sql 格式: %s", base)
	}
	version, err := strconv.Atoi(prefix)
	if err != nil {
		return 0, fmt.Errorf("迁移文件版本号不合法: %s", base)
	}
	return version, nil
}
