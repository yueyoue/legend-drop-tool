package parser

import (
	"fmt"
	"time"
)

// EngineType 引擎类型
type EngineType int

const (
	EngineUnknown EngineType = iota
	EngineHERO
	EngineGOM
	EngineGEE
	EngineBLUE
)

func (e EngineType) String() string {
	switch e {
	case EngineHERO:
		return "HERO"
	case EngineGOM:
		return "GOM"
	case EngineGEE:
		return "GEE"
	case EngineBLUE:
		return "BLUE"
	default:
		return "未知"
	}
}

// DropGroup 掉落组（对应#CHILD结构）
type DropGroup struct {
	ProbabilityNumerator   int         // 组概率分子（#CHILD 1/2 中的1）
	ProbabilityDenominator int         // 组概率分母（#CHILD 1/2 中的2）
	IsRandom               bool        // RANDOM标志：组内只随机选一个
	Items                  []GroupItem // 组内条目（物品或嵌套子组）
}

// GroupItem 组内条目（可以是物品或嵌套子组）
type GroupItem struct {
	Entry    *DropEntry // 原始掉落条目（非nil=普通物品）
	SubGroup *DropGroup // 嵌套子组（非nil=嵌套#CHILD）
}

// Probability 返回组进入概率
func (g *DropGroup) Probability() float64 {
	if g.ProbabilityDenominator == 0 {
		return 1
	}
	return float64(g.ProbabilityNumerator) / float64(g.ProbabilityDenominator)
}

// DropEntry 单条掉落配置
type DropEntry struct {
	LineNumber             int
	Depth                  int    // 嵌套深度 (0=顶层, 1=CHILD内, 2=嵌套CHILD内)
	ProbabilityNumerator   int    // 概率分子 (通常为1)
	ProbabilityDenominator int    // 概率分母 (如100，表示1/100)
	ItemName               string // 物品名称
	Quantity               int    // 掉落数量，默认1
	IsComment              bool   // 是否被注释
	IsCallRef              bool   // 是否为#CALL引用
	CallPath               string // #CALL引用的文件路径
	CallLabel              string // #CALL引用的@标签
	IsChildStart           bool   // 是否为#CHILD开始行
	IsChildEnd             bool   // 是否为#CHILD结束括号
	ChildProbability       string // #CHILD的概率 (如 "1/1")
	ChildRandom            bool   // #CHILD是否带RANDOM
	IsCaseStart            bool   // 是否为#CASE开始
	IsIfStart              bool   // 是否为#IF开始
	CaseExpression         string // #CASE/#IF的条件表达式
	HasTrigger             bool   // 是否有|@触发
	TriggerName            string // 触发名称
	RawLine                string // 原始行内容
}

// Probability 返回浮点概率
func (d *DropEntry) Probability() float64 {
	if d.ProbabilityDenominator == 0 {
		return 0
	}
	return float64(d.ProbabilityNumerator) / float64(d.ProbabilityDenominator)
}

// ProbabilityStr 返回概率显示字符串
func (d *DropEntry) ProbabilityStr() string {
	if d.IsCallRef {
		return "#CALL"
	}
	if d.IsChildStart {
		s := "#CHILD " + d.ChildProbability
		if d.ChildRandom {
			s += " RANDOM"
		}
		return s
	}
	if d.IsCaseStart {
		return "#CASE"
	}
	if d.IsIfStart {
		return "#IF"
	}
	if d.ProbabilityDenominator == 1 && d.ProbabilityNumerator == 1 {
		return "1/1(必掉)"
	}
	return fmt.Sprintf("%d/%d", d.ProbabilityNumerator, d.ProbabilityDenominator)
}

// IsEditable 是否可编辑的掉落条目
func (d *DropEntry) IsEditable() bool {
	return !d.IsComment && !d.IsCallRef && !d.IsChildStart && !d.IsChildEnd &&
		!d.IsCaseStart && !d.IsIfStart && d.ProbabilityDenominator > 0
}

// MonsterDropFile 怪物爆率文件
type MonsterDropFile struct {
	FilePath    string
	MonsterName string
	Engine      EngineType
	Entries     []*DropEntry
	RawContent  string
	ParsedAt    time.Time
}

// ParseResult 解析结果
type ParseResult struct {
	File     *MonsterDropFile
	Errors   []ParseError
	Warnings []string
}

// ParseError 解析错误
type ParseError struct {
	Line    int
	Message string
	RawLine string
}

func (e ParseError) Error() string {
	return fmt.Sprintf("第%d行: %s (内容: %s)", e.Line, e.Message, e.RawLine)
}
