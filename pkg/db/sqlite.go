package db

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

type sqliteReader struct {
	db *sql.DB
}

func newSQLiteReader(cfg DBConfig) (Reader, error) {
	if cfg.FilePath == "" {
		return nil, &DBError{Msg: "SQLite 数据库文件路径不能为空"}
	}
	db, err := sql.Open("sqlite3", cfg.FilePath)
	if err != nil {
		return nil, fmt.Errorf("打开 SQLite 数据库失败: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("连接 SQLite 数据库失败: %w", err)
	}
	return &sqliteReader{db: db}, nil
}

func (r *sqliteReader) ReadItems() ([]ItemInfo, error) {
	// 尝试查询 StdItems 表
	rows, err := r.db.Query("SELECT Idx, Name FROM StdItems ORDER BY Idx")
	if err != nil {
		return nil, fmt.Errorf("查询 StdItems 表失败: %w", err)
	}
	defer rows.Close()

	var items []ItemInfo
	for rows.Next() {
		var item ItemInfo
		if err := rows.Scan(&item.Idx, &item.Name); err != nil {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

func (r *sqliteReader) Close() error {
	return r.db.Close()
}