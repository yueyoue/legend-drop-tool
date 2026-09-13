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
	// 1/100 物品名称 [数量]
	reSimpleDrop = regexp.MustCompile(`^(\d+)/(\d+)\s+(.+?)(?:\s+(\d+))?\s*$`)
	// #CALL [路径]
	reCallRef = regexp.MustCompile(`(?i)^#CALL\s+\[(.+?)\]`)
	// @标签
	reLabel = regexp.MustCompile(`^(\[.+?\])`)
	// GOM #CHILD
	reChild = regexp.MustCompile(`(?i)^#CHILD\s+`)
	// GOM RANDOM
	reRandom = regexp.MustCompile(`(?i)^RANDOM\s*\(`)
	// 注释行
	reComment = regexp.MustCompile(`^\s*[;]`)
	// 空行
	reEmpty = regexp.MustCompile(`^\s*$`)
	// HERO引擎格式: 怪物名 物品名 概率分母
	reHEROFormat = regexp.MustCompile(`^(.+?)\s+(.+?)\s+(\d+)\s*$`)
)

// ParseFile 解析单个爆率文件
func ParseFile(filePath string, engine EngineType) (*ParseResult, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}

	content := string(data)
	monsterName := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))

	mdf := &MonsterDropFile{
		FilePath:    filePath,
		MonsterName: monsterName,
		Engine:      engine,
		RawContent:  content,
		ParsedAt:    time.Now(),
	}

	result := &ParseResult{File: mdf}

	scanner := bufio.NewScanner(strings.NewReader(content))
	lineNum := 0

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
				IsComment:  true,
				RawLine:    line,
			}
			mdf.Entries = append(mdf.Entries, entry)
			continue
		}

		// GOM/GEE #CALL引用
		if match := reCallRef.FindStringSubmatch(trimmed); match != nil {
			entry := &DropEntry{
				LineNumber: lineNum,
				IsCallRef:  true,
				CallPath:   match[1],
				RawLine:    line,
			}
			mdf.Entries = append(mdf.Entries, entry)
			continue
		}

		// GOM/GEE #CHILD 或 RANDOM 标记 (作为特殊条目记录)
		if reChild.MatchString(trimmed) || reRandom.MatchString(trimmed) || reLabel.MatchString(trimmed) {
			entry := &DropEntry{
				LineNumber: lineNum,
				IsComment:  false,
				ItemName:   trimmed,
				RawLine:    line,
			}
			mdf.Entries = append(mdf.Entries, entry)
			continue
		}

		// 标准掉落格式: 1/100 物品名称 [数量]
		if match := reSimpleDrop.FindStringSubmatch(trimmed); match != nil {
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
				LineNumber:             lineNum,
				ProbabilityNumerator:   num,
				ProbabilityDenominator: den,
				ItemName:               itemName,
				Quantity:               qty,
				RawLine:                line,
			}
			mdf.Entries = append(mdf.Entries, entry)
			continue
		}

		// HERO引擎备选格式: 物品名 概率分母
		if match := reHEROFormat.FindStringSubmatch(trimmed); match != nil {
			den, err := strconv.Atoi(match[3])
			if err == nil && den > 0 {
				entry := &DropEntry{
					LineNumber:             lineNum,
					ProbabilityNumerator:   1,
					ProbabilityDenominator: den,
					ItemName:               strings.TrimSpace(match[2]),
					Quantity:               1,
					RawLine:                line,
				}
				mdf.Entries = append(mdf.Entries, entry)
				continue
			}
		}

		// 无法识别的行
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("第%d行: 无法识别格式: %s", lineNum, trimmed))
	}

	return result, nil
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
	// 检查引擎控制器文件名
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

	// 检查M2Server.exe的特征
	m2Path := filepath.Join(serverRoot, "Mir200", "M2Server.exe")
	if _, err := os.Stat(m2Path); err == nil {
		// 通过配置文件特征判断
		if _, err := os.Stat(filepath.Join(serverRoot, "Mir200", "Envir", "GlobalDropRate.txt")); err == nil {
			return EngineHERO
		}
		return EngineGOM // 默认GOM
	}

	return EngineUnknown
}
