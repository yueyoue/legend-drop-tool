package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yueyoue/legend-drop-tool/pkg/parser"
)

// EditAction 编辑动作类型
type EditAction string

const (
	ActionModify   EditAction = "修改"
	ActionDelete   EditAction = "删除"
	ActionAdd      EditAction = "新增"
	ActionBatchMul EditAction = "批量倍率"
)

// EditRecord 修改记录
type EditRecord struct {
	Timestamp time.Time
	Action    EditAction
	FilePath  string
	LineNum   int
	OldValue  string
	NewValue  string
}

// Editor 爆率编辑器
type Editor struct {
	records []EditRecord
}

// New 创建编辑器
func New() *Editor {
	return &Editor{}
}

// Records 返回修改记录
func (e *Editor) Records() []EditRecord {
	return e.records
}

// ModifyEntry 修改单条掉落概率
func (e *Editor) ModifyEntry(file *parser.MonsterDropFile, entry *parser.DropEntry, newNumerator, newDenominator int) error {
	if entry.IsCallRef {
		return fmt.Errorf("不支持修改#CALL引用")
	}

	oldValue := entry.ProbabilityStr()
	entry.ProbabilityNumerator = newNumerator
	entry.ProbabilityDenominator = newDenominator

	e.records = append(e.records, EditRecord{
		Timestamp: time.Now(),
		Action:    ActionModify,
		FilePath:  file.FilePath,
		LineNum:   entry.LineNumber,
		OldValue:  oldValue,
		NewValue:  entry.ProbabilityStr(),
	})
	return nil
}

// ModifyQuantity 修改掉落数量
func (e *Editor) ModifyQuantity(file *parser.MonsterDropFile, entry *parser.DropEntry, newQty int) error {
	oldQty := entry.Quantity
	entry.Quantity = newQty

	e.records = append(e.records, EditRecord{
		Timestamp: time.Now(),
		Action:    ActionModify,
		FilePath:  file.FilePath,
		LineNum:   entry.LineNumber,
		OldValue:  fmt.Sprintf("数量:%d", oldQty),
		NewValue:  fmt.Sprintf("数量:%d", newQty),
	})
	return nil
}

// DeleteEntry 删除单条掉落
func (e *Editor) DeleteEntry(file *parser.MonsterDropFile, idx int) error {
	if idx < 0 || idx >= len(file.Entries) {
		return fmt.Errorf("索引越界")
	}
	entry := file.Entries[idx]
	e.records = append(e.records, EditRecord{
		Timestamp: time.Now(),
		Action:    ActionDelete,
		FilePath:  file.FilePath,
		LineNum:   entry.LineNumber,
		OldValue:  entry.RawLine,
	})
	file.Entries = append(file.Entries[:idx], file.Entries[idx+1:]...)
	return nil
}

// AddEntry 新增掉落条目
func (e *Editor) AddEntry(file *parser.MonsterDropFile, itemName string, numerator, denominator, quantity int) {
	entry := &parser.DropEntry{
		LineNumber:             len(file.Entries) + 1,
		ProbabilityNumerator:   numerator,
		ProbabilityDenominator: denominator,
		ItemName:               itemName,
		Quantity:               quantity,
		RawLine:                fmt.Sprintf("%d/%d %s %d", numerator, denominator, itemName, quantity),
	}
	file.Entries = append(file.Entries, entry)

	e.records = append(e.records, EditRecord{
		Timestamp: time.Now(),
		Action:    ActionAdd,
		FilePath:  file.FilePath,
		NewValue:  entry.RawLine,
	})
}

// BatchMultiply 批量倍率调整 (对所有非注释、非CALL的条目)
func (e *Editor) BatchMultiply(file *parser.MonsterDropFile, multiplier float64) int {
	count := 0
	for _, entry := range file.Entries {
		if entry.IsComment || entry.IsCallRef || entry.ProbabilityDenominator == 0 {
			continue
		}
		oldDen := entry.ProbabilityDenominator
		// 分母除以倍率 = 提高爆率
		newDen := int(float64(oldDen) / multiplier)
		if newDen < 1 {
			newDen = 1
		}
		entry.ProbabilityDenominator = newDen
		count++
	}

	e.records = append(e.records, EditRecord{
		Timestamp: time.Now(),
		Action:    ActionBatchMul,
		FilePath:  file.FilePath,
		OldValue:  "全条目",
		NewValue:  fmt.Sprintf("%.2f倍", multiplier),
	})
	return count
}

// BatchSetAll 批量设置所有条目为同一概率
func (e *Editor) BatchSetAll(file *parser.MonsterDropFile, numerator, denominator int) int {
	count := 0
	for _, entry := range file.Entries {
		if entry.IsComment || entry.IsCallRef {
			continue
		}
		entry.ProbabilityNumerator = numerator
		entry.ProbabilityDenominator = denominator
		count++
	}
	return count
}

// FilterByItem 按物品名筛选条目
func (e *Editor) FilterByItem(file *parser.MonsterDropFile, itemName string) []*parser.DropEntry {
	var result []*parser.DropEntry
	for _, entry := range file.Entries {
		if entry.IsComment || entry.IsCallRef {
			continue
		}
		if strings.Contains(entry.ItemName, itemName) {
			result = append(result, entry)
		}
	}
	return result
}

// SaveFile 保存修改后的爆率文件
func (e *Editor) SaveFile(file *parser.MonsterDropFile) error {
	var lines []string
	for _, entry := range file.Entries {
		lines = append(lines, entry.RawLine)
	}
	content := strings.Join(lines, "\r\n") + "\r\n"
	return os.WriteFile(file.FilePath, []byte(content), 0644)
}

// SaveAs 保存为新文件
func (e *Editor) SaveAs(file *parser.MonsterDropFile, newPath string) error {
	var lines []string
	for _, entry := range file.Entries {
		lines = append(lines, entry.RawLine)
	}
	content := strings.Join(lines, "\r\n") + "\r\n"
	dir := filepath.Dir(newPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(newPath, []byte(content), 0644)
}

// RebuildRawContent 根据条目重建原始内容
func (e *Editor) RebuildRawContent(file *parser.MonsterDropFile) {
	var lines []string
	for _, entry := range file.Entries {
		if entry.IsComment {
			lines = append(lines, entry.RawLine)
		} else if entry.IsCallRef {
			lines = append(lines, fmt.Sprintf("#CALL [%s]", entry.CallPath))
		} else {
			line := fmt.Sprintf("%d/%d %s", entry.ProbabilityNumerator, entry.ProbabilityDenominator, entry.ItemName)
			if entry.Quantity > 1 {
				line += fmt.Sprintf(" %d", entry.Quantity)
			}
			lines = append(lines, line)
			entry.RawLine = line
		}
	}
	file.RawContent = strings.Join(lines, "\r\n")
}
