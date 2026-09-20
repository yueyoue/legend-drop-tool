package db

import (
	"database/sql"
	"fmt"

	_ "github.com/microsoft/go-mssqldb"
)

type mssqlReader struct {
	db *sql.DB
}

func newMSSQLReader(cfg DBConfig) (Reader, error) {
	if cfg.Host == "" {
		cfg.Host = "127.0.0.1"
	}
	if cfg.Port == 0 {
		cfg.Port = 1433
	}
	if cfg.DBName == "" {
		return nil, &DBError{Msg: "SQL Server 数据库名不能为空"}
	}
	connStr := fmt.Sprintf("server=%s;port=%d;database=%s;user id=%s;password=%s;encrypt=disable",
		cfg.Host, cfg.Port, cfg.DBName, cfg.User, cfg.Password)
	db, err := sql.Open("sqlserver", connStr)
	if err != nil {
		return nil, fmt.Errorf("连接 SQL Server 数据库失败: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("连接 SQL Server 数据库失败: %w", err)
	}
	return &mssqlReader{db: db}, nil
}

func (r *mssqlReader) ReadItems() ([]ItemInfo, error) {
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

func (r *mssqlReader) Close() error {
	return r.db.Close()
}