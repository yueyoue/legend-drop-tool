package parser

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	// 传统格式: 1/100 物品名称 [数量]
	reSimpleDrop = regexp.MustCompile(`^(\d+)/(\d+)\s+(.+?)(?:\s+(\d+))?\s*$`)
	// #CALL [路径] 或 #CALL [路径] @标签
	reCallRef = regexp.MustCompile(`(?i)^#CALL\s+\[(.+?)\](?:\s+(\S+))?`)
	// #CHILD 开始: #CHILD 1/1 RANDOM 或 #CHILD 1/2
	reChildStart = regexp.MustCompile(`(?i)^#CHILD\s+(\d+/\d+)\s*(RANDOM)?`)
	// #CASE: #CASE N10|1 RANDOM 或 #CASE M10
	reCaseStart = regexp.MustCompile(`(?i)^#CASE\s+(.+?)(?:\s+(RANDOM))?\s*$`)
	// #IF: #IF [N20 > 100, N20 < 110|1] RANDOM
	reIfStart = regexp.MustCompile(`(?i)^#IF\s+(\[.+?\])(?:\s+(RANDOM))?\s*$`)
	// 触发字段: 物品名|@触发名
	reTrigger = regexp.MustCompile(`^(.+?)\|(@\S+)$`)
	// 注释行: ;开头
	reComment = regexp.MustCompile(`^\s*;`)
	// 空行
	reEmpty = regexp.MustCompile(`^\s*$`)
	// 单独的 ( 或 )
	reParenOpen  = regexp.MustCompile(`^\s*\(\s*$`)
	reParenClose = regexp.MustCompile(`^\s*\)\s*$`)
	// 缩进的掉落条目 (CHILD内的子条目通常有缩进或制表符)
	reIndentedDrop = regexp.MustCompile(`^\s+(\d+)/(\d+)\s+(.+?)(?:\s+(\d+))?\s*$`)
)

// ParseFile 解析单个爆率文件
func ParseFile(filePath string, engine EngineType) (*ParseResult, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}

	// 处理BOM
	content := string(data)
	if len(content) >= 3 && content[0] == 0xEF && content[1] == 0xBB && content[2] == 0xBF {
		content = content[3:]
	}

	monsterName := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))

	mdf := &MonsterDropFile{
		FilePath:    filePath,
		MonsterName: monsterName,
		Engine:      engine,
		RawContent:  content,
		ParsedAt:    time.Now(),
	}

	result := &ParseResult{File: mdf}

	// 如果引擎未知，尝试自动检测
	if engine == EngineUnknown {
		mdf.Engine = detectEngineFromContent(content)
	}

	scanner := bufio.NewScanner(strings.NewReader(content))
	lineNum := 0
	depth := 0 // 括号嵌套深度

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// 空行跳过
		if reEmpty.MatchString(trimmed) {
			continue
		}

		// 注释行
		if reComment.MatchString(trimmed) {
			entry := &DropEntry{
				LineNumber: lineNum,
				Depth:      depth,
				IsComment:  true,
				RawLine:    line,
			}
			mdf.Entries = append(mdf.Entries, entry)
			continue
		}

		// #CALL引用
		if match := reCallRef.FindStringSubmatch(trimmed); match != nil {
			entry := &DropEntry{
				LineNumber: lineNum,
				Depth:      depth,
				IsCallRef:  true,
				CallPath:   strings.TrimSpace(match[1]),
				RawLine:    line,
			}
			if len(match) > 2 && match[2] != "" {
				entry.CallLabel = match[2]
			}
			mdf.Entries = append(mdf.Entries, entry)
			continue
		}

		// #CHILD开始
		if match := reChildStart.FindStringSubmatch(trimmed); match != nil {
			entry := &DropEntry{
				LineNumber:       lineNum,
				Depth:            depth,
				IsChildStart:     true,
				ChildProbability: match[1],
				ChildRandom:      len(match) > 2 && strings.ToUpper(match[2]) == "RANDOM",
				RawLine:          line,
			}
			mdf.Entries = append(mdf.Entries, entry)
			depth++
			continue
		}

		// #CASE开始
		if match := reCaseStart.FindStringSubmatch(trimmed); match != nil {
			entry := &DropEntry{
				LineNumber:     lineNum,
				Depth:          depth,
				IsCaseStart:    true,
				CaseExpression: strings.TrimSpace(match[1]),
				ChildRandom:    len(match) > 2 && strings.ToUpper(match[2]) == "RANDOM",
				RawLine:        line,
			}
			mdf.Entries = append(mdf.Entries, entry)
			depth++
			continue
		}

		// #IF开始
		if match := reIfStart.FindStringSubmatch(trimmed); match != nil {
			entry := &DropEntry{
				LineNumber:     lineNum,
				Depth:          depth,
				IsIfStart:      true,
				CaseExpression: strings.TrimSpace(match[1]),
				ChildRandom:    len(match) > 2 && strings.ToUpper(match[2]) == "RANDOM",
				RawLine:        line,
			}
			mdf.Entries = append(mdf.Entries, entry)
			depth++
			continue
		}

		// 单独的 ( — CHILD/块开始
		if reParenOpen.MatchString(trimmed) {
			entry := &DropEntry{
				LineNumber: lineNum,
				Depth:      depth,
				RawLine:    line,
			}
			mdf.Entries = append(mdf.Entries, entry)
			depth++
			continue
		}

		// 单独的 ) — CHILD/块结束
		if reParenClose.MatchString(trimmed) {
			if depth > 0 {
				depth--
			}
			entry := &DropEntry{
				LineNumber: lineNum,
				Depth:      depth,
				IsChildEnd: true,
				RawLine:    line,
			}
			mdf.Entries = append(mdf.Entries, entry)
			continue
		}

		// #CASE的数值条件行 (如 100 或 101)
		if depth > 0 {
			if val, err := strconv.Atoi(trimmed); err == nil && val > 0 {
				// 这是 #CASE 的条件值行，作为特殊条目记录
				entry := &DropEntry{
					LineNumber:     lineNum,
					Depth:          depth,
					CaseExpression: fmt.Sprintf("=%d", val),
					RawLine:        line,
				}
				mdf.Entries = append(mdf.Entries, entry)
				continue
			}
		}

		// 掉落条目 (支持缩进和非缩进)
		if match := findDropMatch(trimmed, line); match != nil {
			mdf.Entries = append(mdf.Entries, match)
			continue
		}

		// 无法识别的行
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("第%d行: 无法识别格式: %s", lineNum, trimmed))
	}

	return result, nil
}

// findDropMatch 尝试匹配掉落条目格式
func findDropMatch(trimmed, rawLine string) *DropEntry {
	// 尝试带缩进的格式
	if match := reIndentedDrop.FindStringSubmatch(rawLine); match != nil {
		return buildDropEntry(match, rawLine, 0)
	}
	// 尝试标准格式
	if match := reSimpleDrop.FindStringSubmatch(trimmed); match != nil {
		return buildDropEntry(match, rawLine, 0)
	}
	return nil
}

// buildDropEntry 从正则匹配构建DropEntry
func buildDropEntry(match []string, rawLine string, depth int) *DropEntry {
	num, _ := strconv.Atoi(match[1])
	den, _ := strconv.Atoi(match[2])
	itemName := strings.TrimSpace(match[3])
	qty := 1
	if match[4] != "" {
		qty, _ = strconv.Atoi(match[4])
	}
	if qty <= 0 {
		qty = 1
	}

	entry := &DropEntry{
		LineNumber:             0,
		Depth:                  depth,
		ProbabilityNumerator:   num,
		ProbabilityDenominator: den,
		ItemName:               itemName,
		Quantity:               qty,
		RawLine:                rawLine,
	}

	// 检查触发字段 |@xxx
	if triggerMatch := reTrigger.FindStringSubmatch(itemName); triggerMatch != nil {
		entry.ItemName = strings.TrimSpace(triggerMatch[1])
		entry.TriggerName = triggerMatch[2]
		entry.HasTrigger = true
	}

	return entry
}

// detectEngineFromContent 通过内容特征检测引擎
func detectEngineFromContent(content string) EngineType {
	upper := strings.ToUpper(content)
	if strings.Contains(upper, "#CHILD") || strings.Contains(upper, "#CALL") || strings.Contains(upper, "#CASE") || strings.Contains(upper, "#IF") {
		return EngineGOM
	}
	return EngineHERO
}

// ParseDirectory 解析整个MonItems目录
func ParseDirectory(dirPath string, engine EngineType) ([]*ParseResult, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("读取目录失败: %w", err)
	}

	var results []*ParseResult
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".txt" {
			continue
		}
		fullPath := filepath.Join(dirPath, entry.Name())
		result, err := ParseFile(fullPath, engine)
		if err != nil {
			result = &ParseResult{
				File: &MonsterDropFile{
					FilePath:    fullPath,
					MonsterName: strings.TrimSuffix(entry.Name(), ext),
				},
				Errors: []ParseError{{Line: 0, Message: err.Error()}},
			}
		}
		results = append(results, result)
	}
	return results, nil
}

// DetectEngine 自动检测引擎类型
func DetectEngine(serverRoot string) EngineType {
	controllers := map[string]EngineType{
		"HERO引擎控制器.exe": EngineHERO,
		"GOM引擎控制器.exe":  EngineGOM,
		"GEE引擎控制器.exe":  EngineGEE,
		"BLUE引擎控制器.exe": EngineBLUE,
	}
	for name, engine := range controllers {
		if _, err := os.Stat(filepath.Join(serverRoot, name)); err == nil {
			return engine
		}
	}

	m2Path := filepath.Join(serverRoot, "Mir200", "M2Server.exe")
	if _, err := os.Stat(m2Path); err == nil {
		if _, err := os.Stat(filepath.Join(serverRoot, "Mir200", "Envir", "GlobalDropRate.txt")); err == nil {
			return EngineHERO
		}
		return EngineGOM
	}

	return EngineUnknown
}

// ResolveCallPath 解析#CALL引用的完整路径
func ResolveCallPath(callPath, monItemsDir string) string {
	callPath = strings.ReplaceAll(callPath, "\\", string(os.PathSeparator))
	callPath = strings.ReplaceAll(callPath, "/", string(os.PathSeparator))

	if filepath.IsAbs(callPath) {
		return callPath
	}

	// 相对于MonItems目录
	return filepath.Join(monItemsDir, callPath)
}
