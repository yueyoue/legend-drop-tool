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
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
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

// decodeToUTF8 将文件字节解码为UTF-8字符串
// 自动检测UTF-8/GBK/GB18030编码
func decodeToUTF8(data []byte) string {
	// 1. 去除BOM
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		data = data[3:] // UTF-8 BOM
	} else if len(data) >= 2 && data[0] == 0xFF && data[1] == 0xFE {
		data = data[2:] // UTF-16 LE BOM
	} else if len(data) >= 2 && data[0] == 0xFE && data[1] == 0xFF {
		data = data[2:] // UTF-16 BE BOM
	}

	// 2. 如果是合法UTF-8，直接返回
	if utf8.Valid(data) {
		return string(data)
	}

	// 3. 使用 golang.org/x/text 解码 GBK/GB18030
	// GB18030 是 GBK 的超集，用 GB18030 解码器可以兼容两者
	decoder := simplifiedchinese.GB18030.NewDecoder()
	result, _, err := transform.Bytes(decoder, data)
	if err != nil {
		// GB18030 解码失败，尝试 GBK
		decoder = simplifiedchinese.GBK.NewDecoder()
		result, _, err = transform.Bytes(decoder, data)
		if err != nil {
			// 都失败了，回退到替换无效字节
			return strings.ToValidUTF8(string(data), "�")
		}
	}

	return string(result)
}

// ParseFile 解析单个爆率文件
func ParseFile(filePath string, engine EngineType) (*ParseResult, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}

	// 智能解码：自动检测UTF-8/GBK并转换
	content := decodeToUTF8(data)

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

// ReparseFromRaw 从原始文本重新解析条目（用于文本编辑器保存后重建条目列表）
func ReparseFromRaw(content string) []*DropEntry {
	var entries []*DropEntry
	scanner := bufio.NewScanner(strings.NewReader(content))
	lineNum := 0
	depth := 0

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
			entries = append(entries, &DropEntry{
				LineNumber: lineNum, Depth: depth, IsComment: true, RawLine: line,
			})
			continue
		}

		// #CALL引用
		if match := reCallRef.FindStringSubmatch(trimmed); match != nil {
			e := &DropEntry{
				LineNumber: lineNum, Depth: depth, IsCallRef: true,
				CallPath: strings.TrimSpace(match[1]), RawLine: line,
			}
			if len(match) > 2 && match[2] != "" {
				e.CallLabel = match[2]
			}
			entries = append(entries, e)
			continue
		}

		// #CHILD开始
		if match := reChildStart.FindStringSubmatch(trimmed); match != nil {
			entries = append(entries, &DropEntry{
				LineNumber: lineNum, Depth: depth, IsChildStart: true,
				ChildProbability: match[1],
				ChildRandom: len(match) > 2 && strings.ToUpper(match[2]) == "RANDOM",
				RawLine: line,
			})
			depth++
			continue
		}

		// #CASE开始
		if match := reCaseStart.FindStringSubmatch(trimmed); match != nil {
			entries = append(entries, &DropEntry{
				LineNumber: lineNum, Depth: depth, IsCaseStart: true,
				CaseExpression: strings.TrimSpace(match[1]),
				ChildRandom: len(match) > 2 && strings.ToUpper(match[2]) == "RANDOM",
				RawLine: line,
			})
			depth++
			continue
		}

		// #IF开始
		if match := reIfStart.FindStringSubmatch(trimmed); match != nil {
			entries = append(entries, &DropEntry{
				LineNumber: lineNum, Depth: depth, IsIfStart: true,
				CaseExpression: strings.TrimSpace(match[1]),
				ChildRandom: len(match) > 2 && strings.ToUpper(match[2]) == "RANDOM",
				RawLine: line,
			})
			depth++
			continue
		}

		// 单独的 (
		if reParenOpen.MatchString(trimmed) {
			entries = append(entries, &DropEntry{LineNumber: lineNum, Depth: depth, RawLine: line})
			depth++
			continue
		}

		// 单独的 )
		if reParenClose.MatchString(trimmed) {
			if depth > 0 {
				depth--
			}
			entries = append(entries, &DropEntry{LineNumber: lineNum, Depth: depth, IsChildEnd: true, RawLine: line})
			continue
		}

		// #CASE 条件值行
		if depth > 0 {
			if val, err := strconv.Atoi(trimmed); err == nil && val > 0 {
				entries = append(entries, &DropEntry{
					LineNumber: lineNum, Depth: depth,
					CaseExpression: fmt.Sprintf("=%d", val), RawLine: line,
				})
				continue
			}
		}

		// 掉落条目
		if match := findDropMatch(trimmed, line); match != nil {
			match.LineNumber = lineNum
			match.Depth = depth
			entries = append(entries, match)
			continue
		}
	}
	return entries
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

// MonGenEntry 刷怪配置条目
type MonGenEntry struct {
	MapName         string // 地图名
	X               int    // X坐标
	Y               int    // Y坐标
	MonsterName     string // 怪物名
	Range           int    // 刷新范围
	Count           int    // 刷新数量
	RefreshMinutes  int    // 刷新间隔(分钟)
	RawLine         string
}

// ParseMonGen 解析 MonGen.txt 刷怪文件
// 格式: 地图号 X Y 怪物名 范围 数量 刷新间隔(分钟)
func ParseMonGen(filePath string) ([]*MonGenEntry, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取MonGen文件失败: %w", err)
	}

	content := decodeToUTF8(data)
	scanner := bufio.NewScanner(strings.NewReader(content))

	var entries []*MonGenEntry
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ";") {
			continue
		}

		// 用空白字符分割
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}

		entry := &MonGenEntry{
			MapName:     fields[0],
			MonsterName: fields[3],
			RawLine:     line,
		}

		entry.X, _ = strconv.Atoi(fields[1])
		entry.Y, _ = strconv.Atoi(fields[2])
		entry.Range, _ = strconv.Atoi(fields[4])
		entry.Count, _ = strconv.Atoi(fields[5])
		if len(fields) > 6 {
			entry.RefreshMinutes, _ = strconv.Atoi(fields[6])
		}
		if entry.RefreshMinutes <= 0 {
			entry.RefreshMinutes = 5 // 默认5分钟
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

// FindMonGenForMonster 从MonGen条目中查找指定怪物的配置
func FindMonGenForMonster(monGenEntries []*MonGenEntry, monsterName string) (refreshSec float64, count int) {
	for _, entry := range monGenEntries {
		if strings.EqualFold(entry.MonsterName, monsterName) {
			return float64(entry.RefreshMinutes) * 60, entry.Count
		}
	}
	return 60, 10 // 默认值
}

// ParseMapInfo 解析 MapInfo.txt 获取地图编号到名称的映射
// 标准格式: [地图编号 地图名称] 标记...
// 例如: [0122 盟重省] DAY
//       [newren|0139 比奇省] SAFE NODROPITEM
//       [0 比奇省] ALLOWUSEMYSHOP ONKILLMON
func ParseMapInfo(filePath string) (map[string]string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	content := decodeToUTF8(data)
	scanner := bufio.NewScanner(strings.NewReader(content))
	result := make(map[string]string)

	reBracket := regexp.MustCompile(`^\[(.+?)\]`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ";") {
			continue
		}

		// 匹配 [...] 中的内容
		match := reBracket.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		inner := strings.TrimSpace(match[1])

		// 按空格分割，第一部分是地图编号，第二部分是地图名称
		fields := strings.Fields(inner)
		if len(fields) < 2 {
			continue
		}

		mapID := fields[0]
		mapName := fields[1]

		// 处理 B113A|B102 这种格式，取 | 前面的部分（MonGen.txt中使用的别名）
		if idx := strings.Index(mapID, "|"); idx > 0 {
			mapID = mapID[:idx]
		}

		if mapID != "" && mapName != "" {
			// 存储原始ID
			result[mapID] = mapName
			// 同时存储小写版本（MonGen.txt中可能用小写）
			result[strings.ToLower(mapID)] = mapName
		}
	}

	return result, nil
}

// BuildDropGroups 从扁平条目列表构建层级掉落组树
// 将#CHILD/#IF/#CASE结构转换为DropGroup树，用于模拟器正确处理
func BuildDropGroups(entries []*DropEntry) []GroupItem {
	var result []GroupItem
	i := 0

	for i < len(entries) {
		e := entries[i]

		// 跳过注释、空行、CHILD结束括号
		if e.IsComment || e.IsChildEnd {
			i++
			continue
		}

		// #CALL引用
		if e.IsCallRef {
			result = append(result, GroupItem{Entry: e})
			i++
			continue
		}

		// #CHILD开始
		if e.IsChildStart {
			group := &DropGroup{
				IsRandom: e.ChildRandom,
			}
			// 解析组概率
			if parts := strings.SplitN(e.ChildProbability, "/", 2); len(parts) == 2 {
				group.ProbabilityNumerator, _ = strconv.Atoi(parts[0])
				group.ProbabilityDenominator, _ = strconv.Atoi(parts[1])
			}
			// 收集组内条目（找匹配的结束括号）
			depth := 1
			i++
			var childEntries []*DropEntry
			for i < len(entries) && depth > 0 {
				ce := entries[i]
				if ce.IsChildStart || ce.IsCaseStart || ce.IsIfStart {
					depth++
				}
				if ce.IsChildEnd {
					depth--
					if depth == 0 {
						break
					}
				}
				childEntries = append(childEntries, ce)
				i++
			}
			// 递归构建子组
			group.Items = BuildDropGroups(childEntries)
			result = append(result, GroupItem{SubGroup: group})
			i++ // 跳过结束括号
			continue
		}

		// #CASE/#IF开始（当作RANDOM组处理）
		if e.IsCaseStart || e.IsIfStart {
			group := &DropGroup{
				ProbabilityNumerator:   1,
				ProbabilityDenominator: 1,
				IsRandom:               e.ChildRandom,
			}
			depth := 1
			i++
			var childEntries []*DropEntry
			for i < len(entries) && depth > 0 {
				ce := entries[i]
				if ce.IsChildStart || ce.IsCaseStart || ce.IsIfStart {
					depth++
				}
				if ce.IsChildEnd {
					depth--
					if depth == 0 {
						break
					}
				}
				childEntries = append(childEntries, ce)
				i++
			}
			group.Items = BuildDropGroups(childEntries)
			result = append(result, GroupItem{SubGroup: group})
			i++
			continue
		}

		// 普通掉落条目
		if e.IsEditable() {
			result = append(result, GroupItem{Entry: e})
		}
		i++
	}

	return result
}
