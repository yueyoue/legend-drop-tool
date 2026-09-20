package db

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/xuri/excelize/v2"
)

type excelReader struct {
	filePath string
}

func newExcelReader(cfg DBConfig) (Reader, error) {
	if cfg.FilePath == "" {
		return nil, &DBError{Msg: "Excel 文件路径不能为空"}
	}
	return &excelReader{filePath: cfg.FilePath}, nil
}

func (r *excelReader) ReadItems() ([]ItemInfo, error) {
	// 996 引擎使用 cfg_equip.xls（装备表）和 cfg_item.xls（道具表）
	// 两个表都对应原 StdItems.DB
	dir := filepath.Dir(r.filePath)
	files := []string{
		filepath.Join(dir, "cfg_equip.xls"),
		filepath.Join(dir, "cfg_item.xls"),
	}

	var allItems []ItemInfo
	idx := 0

	for _, f := range files {
		items, err := r.readSheet(f)
		if err != nil {
			continue // 跳过不存在的文件
		}
		allItems = append(allItems, items...)
	}

	if len(allItems) == 0 {
		return nil, &DBError{Msg: "未找到物品数据，请确认 Excel 文件路径正确"}
	}
	return allItems, nil
}

func (r *excelReader) readSheet(filePath string) ([]ItemInfo, error) {
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("Excel 文件没有工作表")
	}

	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, err
	}

	// 找到 Name 列的索引
	nameCol := -1
	idxCol := -1
	if len(rows) > 0 {
		for i, cell := range rows[0] {
			cellLower := strings.ToLower(strings.TrimSpace(cell))
			if cellLower == "name" || cellLower == "物品名称" {
				nameCol = i
			}
			if cellLower == "idx" || cellLower == "id" || cellLower == "编号" {
				idxCol = i
			}
		}
	}

	if nameCol < 0 {
		// 默认第2列为 Name（第1列为 Idx）
		if len(rows) > 0 && len(rows[0]) >= 2 {
			idxCol = 0
			nameCol = 1
		} else {
			return nil, fmt.Errorf("无法识别 Name 列")
		}
	}

	var items []ItemInfo
	for i, row := range rows {
		if i == 0 {
			continue // 跳过表头
		}
		if nameCol >= len(row) {
			continue
		}
		name := strings.TrimSpace(row[nameCol])
		if name == "" {
			continue
		}
		item := ItemInfo{Idx: i, Name: name}
		if idxCol >= 0 && idxCol < len(row) {
			fmt.Sscanf(row[idxCol], "%d", &item.Idx)
		}
		items = append(items, item)
	}
	return items, nil
}

func (r *excelReader) Close() error {
	return nil
}