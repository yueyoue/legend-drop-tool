package db

import (
	"database/sql"
	"fmt"
	"path/filepath"

	_ "github.com/alexbrainman/odbc"
)

type odbcReader struct {
	db *sql.DB
}

// newODBCReader 创建 ODBC 读取器（用于 BDE/Paradox 和 Access/MDB）
// driverName: ODBC 驱动名称
func newODBCReader(cfg DBConfig, driverName string) (Reader, error) {
	if cfg.FilePath == "" {
		return nil, &DBError{Msg: "数据库文件路径不能为空"}
	}

	var connStr string
	switch cfg.Type {
	case DBTypeBDE:
		// BDE/Paradox: 连接到文件所在目录
		dir := filepath.Dir(cfg.FilePath)
		connStr = fmt.Sprintf("Driver={%s};DBQ=%s;DefaultDir=%s;", driverName, cfg.FilePath, dir)
	case DBTypeAccess:
		connStr = fmt.Sprintf("Driver={%s};DBQ=%s;", driverName, cfg.FilePath)
	default:
		return nil, &DBError{Msg: "不支持的 ODBC 数据库类型"}
	}

	db, err := sql.Open("odbc", connStr)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败（请确认已安装 %s 驱动）: %w", driverName, err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("连接数据库失败（请确认已安装 %s 驱动）: %w", driverName, err)
	}
	return &odbcReader{db: db}, nil
}

func (r *odbcReader) ReadItems() ([]ItemInfo, error) {
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

func (r *odbcReader) Close() error {
	return r.db.Close()
}