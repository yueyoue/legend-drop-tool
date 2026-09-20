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
	// 996 引擎使用 cfg_equip + cfg_item 表
	// 支持 .xlsx 格式，.xls 需先另存为 .xlsx
	dir := filepath.Dir(r.filePath)

	// 尝试两种文件名
	baseNames := []string{"cfg_equip", "cfg_item"}
	exts := []string{".xlsx", ".xls"}

	var allItems []ItemInfo

	for _, base := range baseNames {
		for _, ext := range exts {
			fPath := filepath.Join(dir, base+ext)
			items, err := r.readSheet(fPath)
			if err == nil {
				allItems = append(allItems, items...)
				break // 找到一个就够了
			}
		}
	}

	// 也尝试用户直接指定的文件
	items, err := r.readSheet(r.filePath)
	if err == nil {
		allItems = append(allItems, items...)
	}

	if len(allItems) == 0 {
		return nil, &DBError{Msg: "未找到物品数据。请确认 cfg_equip.xlsx / cfg_item.xlsx 在同一目录，或将 .xls 另存为 .xlsx 格式"}
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
	if err != nil || len(rows) == 0 {
		return nil, fmt.Errorf("读取工作表失败")
	}

	// 找到 Name 列和 Idx 列
	nameCol := -1
	idxCol := -1
	for i, cell := range rows[0] {
		cellLower := strings.ToLower(strings.TrimSpace(cell))
		if cellLower == "name" || cellLower == "物品名称" {
			nameCol = i
		}
		if cellLower == "idx" || cellLower == "id" || cellLower == "编号" {
			idxCol = i
		}
	}
	if nameCol < 0 {
		// 默认第2列为 Name
		if len(rows[0]) >= 2 {
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