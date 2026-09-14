package simulator

import (
	"fmt"
	"math/rand"
	"sort"
	"time"

	"github.com/yueyoue/legend-drop-tool/pkg/parser"
)

// SimConfig 模拟配置
type SimConfig struct {
	DurationHours   float64  // 模拟时长(小时)
	KillRatio       float64  // 击杀比例 0~1
	RefreshInterval float64  // 怪物刷新间隔(秒)
	RefreshCount    int      // 每次刷新数量
	MapRateModifier float64  // 地图爆率修正系数
	MonsterFilter   []string // 指定怪物列表（空=全部）
	ItemFilter      []string // 指定物品列表（空=全部）
	MapFilter       []string // 指定地图列表（空=全部）
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

// ItemStat 物品统计
type ItemStat struct {
	ItemName    string
	DropCount   int64
	TotalQty    int64
	Probability float64
	Maps        map[string]bool  // 掉落该物品的地图
	Monsters    map[string]bool  // 掉落该物品的怪物
}

// MapStat 地图统计
type MapStat struct {
	MapName    string
	MonsterCnt int64
	DropCount  int64
}

// MonsterStat 怪物统计
type MonsterStat struct {
	MonsterName string
	KillCount   int64
	DropCount   int64
}

// SimResult 完整模拟结果
type SimResult struct {
	Config       SimConfig
	ItemStats    []*ItemStat   // 按掉落数量排序
	MapStats     []*MapStat    // 按掉落数量排序
	MonsterStats []*MonsterStat // 按掉落数量排序
	TotalKills   int64
	TotalDrops   int64
	TotalEmpty   int64
	Duration     time.Duration

	// 物品级别的怪物/地图掉落数跟踪
	// ItemMonsterDrops[物品名][怪物名] = 该怪物掉落该物品的次数
	ItemMonsterDrops map[string]map[string]int64
	// ItemMapDrops[物品名][地图名] = 该地图掉落该物品的次数
	ItemMapDrops map[string]map[string]int64
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

// monsterMapInfo 怪物与地图的关联信息
type monsterMapInfo struct {
	monsterName string
	mapName     string
	refreshSec  float64
	count       int
}

// SimulateAll 模拟所有怪物，支持按怪物/物品/地图筛选
func (s *Simulator) SimulateAll(
	files []*parser.MonsterDropFile,
	monGenEntries []*parser.MonGenEntry,
	config SimConfig,
) *SimResult {
	start := time.Now()

	// 构建怪物→地图映射
	monsterMapLookup := make(map[string][]monsterMapInfo)
	if monGenEntries != nil {
		for _, mg := range monGenEntries {
			info := monsterMapInfo{
				monsterName: mg.MonsterName,
				mapName:     mg.MapName,
				refreshSec:  float64(mg.RefreshMinutes) * 60,
				count:       mg.Count,
			}
			monsterMapLookup[mg.MonsterName] = append(monsterMapLookup[mg.MonsterName], info)
		}
	}

	// 构建怪物过滤集合
	monsterFilterSet := make(map[string]bool)
	for _, name := range config.MonsterFilter {
		monsterFilterSet[name] = true
	}
	itemFilterSet := make(map[string]bool)
	for _, name := range config.ItemFilter {
		itemFilterSet[name] = true
	}
	mapFilterSet := make(map[string]bool)
	for _, name := range config.MapFilter {
		mapFilterSet[name] = true
	}

	// 按物品汇总
	itemAgg := make(map[string]*ItemStat)
	// 按地图汇总
	mapAgg := make(map[string]*MapStat)
	// 按怪物汇总
	monsterAgg := make(map[string]*MonsterStat)
	// 物品→怪物掉落数
	itemMonsterDrops := make(map[string]map[string]int64)
	// 物品→地图掉落数
	itemMapDrops := make(map[string]map[string]int64)

	for _, file := range files {
		monsterName := file.MonsterName

		// 怪物筛选
		if len(monsterFilterSet) > 0 && !monsterFilterSet[monsterName] {
			continue
		}

		// 获取该怪物的地图信息
		mapInfos, hasMapInfo := monsterMapLookup[monsterName]

		// 地图筛选
		if len(mapFilterSet) > 0 {
			if !hasMapInfo {
				continue
			}
			found := false
			for _, mi := range mapInfos {
				if mapFilterSet[mi.mapName] {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// 计算该怪物的击杀数
		var totalKills int64
		if hasMapInfo && len(mapInfos) > 0 {
			for _, mi := range mapInfos {
				// 如果有地图筛选，只计算匹配的地图
				if len(mapFilterSet) > 0 && !mapFilterSet[mi.mapName] {
					continue
				}
				totalDurationSec := config.DurationHours * 3600
				totalRefreshes := totalDurationSec / mi.refreshSec
				mapKills := int64(totalRefreshes) * int64(mi.count)
				mapKills = int64(float64(mapKills) * config.KillRatio)
				if mapKills > 0 {
					totalKills += mapKills

					// 记录地图统计
					if mapAgg[mi.mapName] == nil {
						mapAgg[mi.mapName] = &MapStat{MapName: mi.mapName}
					}
					mapAgg[mi.mapName].MonsterCnt += mapKills
				}
			}
		} else {
			// 没有MonGen信息，使用默认配置
			totalDurationSec := config.DurationHours * 3600
			totalRefreshes := totalDurationSec / config.RefreshInterval
			totalMonsters := int64(totalRefreshes) * int64(config.RefreshCount)
			totalKills = int64(float64(totalMonsters) * config.KillRatio)

			// 记录到"未知地图"
			if mapAgg["未知地图"] == nil {
				mapAgg["未知地图"] = &MapStat{MapName: "未知地图"}
			}
			mapAgg["未知地图"].MonsterCnt += totalKills
		}

		if totalKills <= 0 {
			totalKills = 1
		}

		// 收集有效掉落条目
		type dropProb struct {
			entry *parser.DropEntry
			prob  float64
		}
		var validEntries []dropProb
		for _, entry := range file.Entries {
			if entry.IsComment || entry.IsCallRef || entry.ProbabilityDenominator <= 0 {
				continue
			}
			// 物品筛选
			if len(itemFilterSet) > 0 && !itemFilterSet[entry.ItemName] {
				continue
			}
			denominator := float64(entry.ProbabilityDenominator) / config.MapRateModifier
			if denominator < 1 {
				denominator = 1
			}
			prob := float64(entry.ProbabilityNumerator) / denominator
			validEntries = append(validEntries, dropProb{entry: entry, prob: prob})
		}

		// 记录怪物统计
		if monsterAgg[monsterName] == nil {
			monsterAgg[monsterName] = &MonsterStat{MonsterName: monsterName}
		}
		monsterAgg[monsterName].KillCount += totalKills

		// 模拟掉落
		for i := int64(0); i < totalKills; i++ {
			for _, dp := range validEntries {
				if s.rng.Float64() < dp.prob {
					itemName := dp.entry.ItemName
					// 物品统计
					if itemAgg[itemName] == nil {
						itemAgg[itemName] = &ItemStat{
							ItemName:    itemName,
							Probability: dp.entry.Probability(),
							Maps:        make(map[string]bool),
							Monsters:    make(map[string]bool),
						}
					}
					itemAgg[itemName].DropCount++
					itemAgg[itemName].TotalQty += int64(dp.entry.Quantity)
					itemAgg[itemName].Monsters[monsterName] = true

					// 物品→怪物掉落数跟踪
					if itemMonsterDrops[itemName] == nil {
						itemMonsterDrops[itemName] = make(map[string]int64)
					}
					itemMonsterDrops[itemName][monsterName]++

					if hasMapInfo {
						for _, mi := range mapInfos {
							if len(mapFilterSet) > 0 && !mapFilterSet[mi.mapName] {
								continue
							}
							itemAgg[itemName].Maps[mi.mapName] = true
							// 物品→地图掉落数跟踪
							if itemMapDrops[itemName] == nil {
								itemMapDrops[itemName] = make(map[string]int64)
							}
							itemMapDrops[itemName][mi.mapName]++
						}
					} else {
						itemAgg[itemName].Maps["未知地图"] = true
						if itemMapDrops[itemName] == nil {
							itemMapDrops[itemName] = make(map[string]int64)
						}
						itemMapDrops[itemName]["未知地图"]++
					}

					// 怪物掉落统计
					monsterAgg[monsterName].DropCount++

					// 地图掉落统计
					if hasMapInfo {
						for _, mi := range mapInfos {
							if len(mapFilterSet) > 0 && !mapFilterSet[mi.mapName] {
								continue
							}
							if mapAgg[mi.mapName] != nil {
								mapAgg[mi.mapName].DropCount++
							}
						}
					} else {
						if mapAgg["未知地图"] != nil {
							mapAgg["未知地图"].DropCount++
						}
					}
				}
			}
		}
	}

	// 转换为排序切片
	result := &SimResult{
		Config: config,
	}

	for _, v := range itemAgg {
		result.ItemStats = append(result.ItemStats, v)
		result.TotalDrops += v.DropCount
	}
	sort.Slice(result.ItemStats, func(i, j int) bool {
		return result.ItemStats[i].DropCount > result.ItemStats[j].DropCount
	})

	for _, v := range mapAgg {
		result.MapStats = append(result.MapStats, v)
	}
	sort.Slice(result.MapStats, func(i, j int) bool {
		return result.MapStats[i].DropCount > result.MapStats[j].DropCount
	})

	for _, v := range monsterAgg {
		result.MonsterStats = append(result.MonsterStats, v)
		result.TotalKills += v.KillCount
	}
	sort.Slice(result.MonsterStats, func(i, j int) bool {
		return result.MonsterStats[i].DropCount > result.MonsterStats[j].DropCount
	})

	result.ItemMonsterDrops = itemMonsterDrops
	result.ItemMapDrops = itemMapDrops
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

	sb += fmt.Sprintf("----- 掉落物品列表 -----\n")
	for _, item := range result.ItemStats {
		sb += fmt.Sprintf("  %s: 掉落%d次 (数量%d)\n", item.ItemName, item.DropCount, item.TotalQty)
	}

	sb += fmt.Sprintf("\n----- 掉落地图列表 -----\n")
	for _, m := range result.MapStats {
		sb += fmt.Sprintf("  %s: 刷怪%d 掉落%d\n", m.MapName, m.MonsterCnt, m.DropCount)
	}

	sb += fmt.Sprintf("\n----- 掉落怪物列表 -----\n")
	for _, ms := range result.MonsterStats {
		sb += fmt.Sprintf("  %s: 击杀%d 掉落%d\n", ms.MonsterName, ms.KillCount, ms.DropCount)
	}

	return sb
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
