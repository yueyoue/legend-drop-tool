package db

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
)

type mysqlReader struct {
	db *sql.DB
}

func newMySQLReader(cfg DBConfig) (Reader, error) {
	if cfg.Host == "" {
		cfg.Host = "127.0.0.1"
	}
	if cfg.Port == 0 {
		cfg.Port = 3306
	}
	if cfg.DBName == "" {
		return nil, &DBError{Msg: "MySQL 数据库名不能为空"}
	}
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("连接 MySQL 数据库失败: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("连接 MySQL 数据库失败: %w", err)
	}
	return &mysqlReader{db: db}, nil
}

func (r *mysqlReader) ReadItems() ([]ItemInfo, error) {
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

func (r *mysqlReader) Close() error {
	return r.db.Close()
}