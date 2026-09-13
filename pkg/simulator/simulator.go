package simulator

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/yueyoue/legend-drop-tool/pkg/parser"
)

// SimConfig 模拟配置
type SimConfig struct {
	DurationHours    float64 // 模拟时长(小时)
	KillRatio        float64 // 击杀比例 0~1
	RefreshInterval  float64 // 怪物刷新间隔(秒)
	RefreshCount     int     // 每次刷新数量
	MapRateModifier  float64 // 地图爆率修正系数
}

// DefaultConfig 默认配置
func DefaultConfig() SimConfig {
	return SimConfig{
		DurationHours:   1,
		KillRatio:       0.6,
		RefreshInterval: 60,
		RefreshCount:    10,
		MapRateModifier: 1.0,
	}
}

// DropResult 单次掉落结果
type DropResult struct {
	ItemName string
	Quantity int
}

// MonsterSimResult 单怪物模拟结果
type MonsterSimResult struct {
	MonsterName  string
	TotalKills   int64
	TotalDrops   int64
	EmptyDrops   int64
	ItemStats    map[string]*ItemStat
}

// ItemStat 物品统计
type ItemStat struct {
	ItemName   string
	DropCount  int64
	TotalQty   int64
	Probability float64
}

// SimResult 完整模拟结果
type SimResult struct {
	Config       SimConfig
	MonsterStats map[string]*MonsterSimResult
	TotalKills   int64
	TotalDrops   int64
	TotalEmpty   int64
	Duration     time.Duration
}

// DropRate 返回总掉落率
func (r *SimResult) DropRate() float64 {
	if r.TotalKills == 0 {
		return 0
	}
	return float64(r.TotalDrops) / float64(r.TotalKills)
}

// EmptyRate 返回空爆率
func (r *SimResult) EmptyRate() float64 {
	if r.TotalKills == 0 {
		return 0
	}
	return float64(r.TotalEmpty) / float64(r.TotalKills)
}

// Simulator 爆率模拟器
type Simulator struct {
	rng *rand.Rand
}

// New 创建模拟器
func New() *Simulator {
	return &Simulator{
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Simulate 模拟单个怪物的掉落
func (s *Simulator) Simulate(file *parser.MonsterDropFile, config SimConfig) *MonsterSimResult {
	result := &MonsterSimResult{
		MonsterName: file.MonsterName,
		ItemStats:   make(map[string]*ItemStat),
	}

	// 计算总击杀数
	totalDurationSec := config.DurationHours * 3600
	totalRefreshes := totalDurationSec / config.RefreshInterval
	totalMonsters := int64(totalRefreshes) * int64(config.RefreshCount)
	totalKills := int64(float64(totalMonsters) * config.KillRatio)

	if totalKills <= 0 {
		totalKills = 1
	}

	result.TotalKills = totalKills

	// 收集有效的掉落条目，并预计算概率
	type dropProb struct {
		entry *parser.DropEntry
		prob  float64
	}
	var validEntries []dropProb
	for _, entry := range file.Entries {
		if entry.IsComment || entry.IsCallRef || entry.ProbabilityDenominator <= 0 {
			continue
		}
		denominator := float64(entry.ProbabilityDenominator) / config.MapRateModifier
		if denominator < 1 {
			denominator = 1
		}
		prob := float64(entry.ProbabilityNumerator) / denominator
		validEntries = append(validEntries, dropProb{entry: entry, prob: prob})
	}

	if len(validEntries) == 0 {
		result.EmptyDrops = totalKills
		return result
	}

	// 模拟每次击杀（批量处理，减少分支判断）
	for i := int64(0); i < totalKills; i++ {
		dropped := false
		for _, dp := range validEntries {
			if s.rng.Float64() < dp.prob {
				dropped = true
				result.TotalDrops++

				stat, exists := result.ItemStats[dp.entry.ItemName]
				if !exists {
					stat = &ItemStat{
						ItemName:    dp.entry.ItemName,
						Probability: dp.entry.Probability(),
					}
					result.ItemStats[dp.entry.ItemName] = stat
				}
				stat.DropCount++
				stat.TotalQty += int64(dp.entry.Quantity)
			}
		}
		if !dropped {
			result.EmptyDrops++
		}
	}

	return result
}

// SimulateAll 模拟所有怪物
func (s *Simulator) SimulateAll(files []*parser.MonsterDropFile, config SimConfig) *SimResult {
	start := time.Now()
	result := &SimResult{
		Config:       config,
		MonsterStats: make(map[string]*MonsterSimResult),
	}

	for _, file := range files {
		monsterResult := s.Simulate(file, config)
		result.MonsterStats[file.MonsterName] = monsterResult
		result.TotalKills += monsterResult.TotalKills
		result.TotalDrops += monsterResult.TotalDrops
		result.TotalEmpty += monsterResult.EmptyDrops
	}

	result.Duration = time.Since(start)
	return result
}

// FormatResult 格式化模拟结果为文本
func FormatResult(result *SimResult) string {
	var sb string
	sb += fmt.Sprintf("========== 爆率模拟结果 ==========\n")
	sb += fmt.Sprintf("模拟时长: %.1f小时 | 击杀比例: %.0f%%\n", result.Config.DurationHours, result.Config.KillRatio*100)
	sb += fmt.Sprintf("执行耗时: %v\n", result.Duration)
	sb += fmt.Sprintf("----------------------------------\n")
	sb += fmt.Sprintf("总击杀数: %d\n", result.TotalKills)
	sb += fmt.Sprintf("总掉落数: %d\n", result.TotalDrops)
	sb += fmt.Sprintf("空爆次数: %d\n", result.TotalEmpty)
	sb += fmt.Sprintf("总掉落率: %.2f%%\n", result.DropRate()*100)
	sb += fmt.Sprintf("空爆概率: %.2f%%\n", result.EmptyRate()*100)
	sb += fmt.Sprintf("平均每怪掉落数: %.2f\n", float64(result.TotalDrops)/float64(max(result.TotalKills, 1)))
	sb += fmt.Sprintf("\n")

	for name, ms := range result.MonsterStats {
		sb += fmt.Sprintf("【%s】击杀:%d 掉落:%d 空爆率:%.1f%%\n",
			name, ms.TotalKills, ms.TotalDrops,
			float64(ms.EmptyDrops)/float64(max(ms.TotalKills, 1))*100)
		for itemName, stat := range ms.ItemStats {
			actualRate := float64(stat.DropCount) / float64(max(ms.TotalKills, 1)) * 100
			sb += fmt.Sprintf("  ├ %s: 掉落%d次 (实际%.3f%% 配置%.4f%%)\n",
				itemName, stat.DropCount, actualRate, stat.Probability*100)
		}
	}
	return sb
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
