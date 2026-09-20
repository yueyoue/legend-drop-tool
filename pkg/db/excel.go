package db

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/extrame/xls"
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
	// 996 引擎使用 cfg_equip + cfg_item
	dir := filepath.Dir(r.filePath)
	ext := strings.ToLower(filepath.Ext(r.filePath))

	// 根据用户选择的文件决定读取方式
	if ext == ".xls" {
		return r.readXls(dir)
	}
	return r.readXlsx(dir)
}

// readXls 读取 .xls 格式 (Excel 97-2003, BIFF8)
func (r *excelReader) readXls(dir string) ([]ItemInfo, error) {
	fileNames := []string{"cfg_equip.xls", "cfg_item.xls"}
	var allItems []ItemInfo

	for _, fname := range fileNames {
		fPath := filepath.Join(dir, fname)
		wb, err := xls.Open(fPath, "utf-8")
		if err != nil {
			continue // 文件不存在则跳过
		}
		numSheets := wb.NumSheets()
		if numSheets == 0 {
			continue
		}
		sheet := wb.GetSheet(0)
		if sheet == nil {
			continue
		}

		// 找到 Name 列和 Idx 列
		nameCol := -1
		idxCol := -1
		if sheet.MaxRow > 0 {
			headerRow := sheet.Row(0)
			if headerRow != nil {
				for i := 0; i < headerRow.LastCol(); i++ {
					cell := strings.ToLower(strings.TrimSpace(headerRow.Col(i)))
					if cell == "name" || cell == "物品名称" {
						nameCol = i
					}
					if cell == "idx" || cell == "id" || cell == "编号" {
						idxCol = i
					}
				}
			}
		}
		if nameCol < 0 {
			nameCol = 1
			idxCol = 0
		}

		for i := 1; i <= int(sheet.MaxRow); i++ {
			row := sheet.Row(i)
			if row == nil {
				continue
			}
			name := strings.TrimSpace(row.Col(nameCol))
			if name == "" {
				continue
			}
			item := ItemInfo{Idx: i, Name: name}
			if idxCol >= 0 {
				fmt.Sscanf(row.Col(idxCol), "%d", &item.Idx)
			}
			allItems = append(allItems, item)
		}
	}

	if len(allItems) == 0 {
		return nil, &DBError{Msg: "未找到物品数据，请确认 cfg_equip.xls / cfg_item.xls 在同一目录"}
	}
	return allItems, nil
}

// readXlsx 读取 .xlsx 格式
func (r *excelReader) readXlsx(dir string) ([]ItemInfo, error) {
	fileNames := []string{"cfg_equip.xlsx", "cfg_item.xlsx"}
	var allItems []ItemInfo

	for _, fname := range fileNames {
		fPath := filepath.Join(dir, fname)
		f, err := excelize.OpenFile(fPath)
		if err != nil {
			continue
		}
		defer f.Close()

		sheets := f.GetSheetList()
		if len(sheets) == 0 {
			continue
		}
		rows, err := f.GetRows(sheets[0])
		if err != nil || len(rows) == 0 {
			continue
		}

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
			nameCol = 1
			idxCol = 0
		}

		for i, row := range rows {
			if i == 0 {
				continue
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
			allItems = append(allItems, item)
		}
	}

	if len(allItems) == 0 {
		return nil, &DBError{Msg: "未找到物品数据，请确认 cfg_equip.xlsx / cfg_item.xlsx 在同一目录"}
	}
	return allItems, nil
}

func (r *excelReader) Close() error {
	return nil
}