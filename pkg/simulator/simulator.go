package simulator

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"time"

	"github.com/yueyoue/legend-drop-tool/pkg/parser"
)

// ==================== 配置 ====================

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

	// 保底机制
	PityEnabled    bool    // 是否启用保底
	PityThreshold  int     // 连续多少次空击杀后触发保底
	PityItemFilter []string // 保底作用的物品列表（空=全部物品）

	// 多次模拟
	RunCount int // 模拟轮数（1=单次，>1=多次取统计）
}

// DefaultConfig 默认配置
func DefaultConfig() SimConfig {
	return SimConfig{
		DurationHours:   1,
		KillRatio:       0.6,
		RefreshInterval: 60,
		RefreshCount:    10,
		MapRateModifier: 1.0,
		PityEnabled:     false,
		PityThreshold:   100,
		RunCount:        1,
	}
}

// ==================== 统计数据 ====================

// ItemStat 物品统计
type ItemStat struct {
	ItemName    string
	DropCount   int64
	TotalQty    int64
	Probability float64
	Maps        map[string]bool
	Monsters    map[string]bool
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

// SimResult 单次模拟结果
type SimResult struct {
	Config       SimConfig
	ItemStats    []*ItemStat
	MapStats     []*MapStat
	MonsterStats []*MonsterStat
	TotalKills   int64
	TotalDrops   int64
	TotalEmpty   int64
	PityTriggered int64 // 保底触发次数
	Duration     time.Duration

	ItemMonsterDrops map[string]map[string]int64
	ItemMapDrops     map[string]map[string]int64
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

// MultiSimResult 多次模拟统计结果
type MultiSimResult struct {
	Config    SimConfig
	Runs      []*SimResult // 每次模拟的结果
	RunCount  int
	Duration  time.Duration

	// 统计汇总
	AvgTotalKills  float64
	AvgTotalDrops  float64
	AvgEmptyRate   float64
	MinTotalDrops  int64
	MaxTotalDrops  int64
	StdDevDrops    float64
	AvgItemStats   []*AvgItemStat // 物品维度统计
}

// AvgItemStat 物品多次模拟的平均统计
type AvgItemStat struct {
	ItemName     string
	AvgDropCount float64
	MinDropCount int64
	MaxDropCount int64
	StdDev       float64
}

// ==================== 模拟器 ====================

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

// ==================== 多次模拟入口 ====================

// SimulateAll 模拟所有怪物，支持多次运行
func (s *Simulator) SimulateAll(
	files []*parser.MonsterDropFile,
	monGenEntries []*parser.MonGenEntry,
	config SimConfig,
) *MultiSimResult {
	start := time.Now()

	if config.RunCount <= 1 {
		config.RunCount = 1
	}

	// 构建掉落组树（只构建一次）
	groupTrees := make(map[string][]parser.GroupItem)
	for _, file := range files {
		groupTrees[file.MonsterName] = parser.BuildDropGroups(file.Entries)
	}

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

	// 构建过滤集合
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
	pityItemSet := make(map[string]bool)
	for _, name := range config.PityItemFilter {
		pityItemSet[name] = true
	}

	// 预计算每个怪物在每个地图的击杀数
	type monsterMapKills struct {
		monsterName string
		mapName     string
		kills       int64
	}
	var allMonsterMapKills []monsterMapKills

	for _, file := range files {
		monsterName := file.MonsterName
		if len(monsterFilterSet) > 0 && !monsterFilterSet[monsterName] {
			continue
		}
		mapInfos, hasMapInfo := monsterMapLookup[monsterName]
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
		if hasMapInfo {
			for _, mi := range mapInfos {
				if len(mapFilterSet) > 0 && !mapFilterSet[mi.mapName] {
					continue
				}
				totalDurationSec := config.DurationHours * 3600
				totalRefreshes := totalDurationSec / mi.refreshSec
				mapKills := int64(totalRefreshes) * int64(mi.count)
				mapKills = int64(float64(mapKills) * config.KillRatio)
				if mapKills > 0 {
					allMonsterMapKills = append(allMonsterMapKills, monsterMapKills{
						monsterName: monsterName, mapName: mi.mapName, kills: mapKills,
					})
				}
			}
		} else {
			totalDurationSec := config.DurationHours * 3600
			totalRefreshes := totalDurationSec / config.RefreshInterval
			totalMonsters := int64(totalRefreshes) * int64(config.RefreshCount)
			totalKills := int64(float64(totalMonsters) * config.KillRatio)
			if totalKills <= 0 {
				totalKills = 1
			}
			allMonsterMapKills = append(allMonsterMapKills, monsterMapKills{
				monsterName: monsterName, mapName: "未知地图", kills: totalKills,
			})
		}
	}

	// 预构建过滤后的掉落组
	filteredGroupTrees := make(map[string][]parser.GroupItem)
	for name, tree := range groupTrees {
		if len(itemFilterSet) > 0 {
			filteredGroupTrees[name] = filterGroupsByItems(tree, itemFilterSet)
		} else {
			filteredGroupTrees[name] = tree
		}
	}

	// 多次模拟
	multi := &MultiSimResult{Config: config, RunCount: config.RunCount}
	multi.Runs = make([]*SimResult, config.RunCount)

	for run := 0; run < config.RunCount; run++ {
		multi.Runs[run] = s.runSingleSimulation(
			files, filteredGroupTrees, allMonsterMapKills, config, monsterFilterSet, pityItemSet,
		)
	}

	// 计算统计汇总
	multi.Duration = time.Since(start)
	s.calculateMultiStats(multi)
	return multi
}

// filterGroupsByItems 过滤掉落组，只保留指定物品
func filterGroupsByItems(items []parser.GroupItem, filterSet map[string]bool) []parser.GroupItem {
	var result []parser.GroupItem
	for _, item := range items {
		if item.Entry != nil {
			if filterSet[item.Entry.ItemName] {
				result = append(result, item)
			}
		} else if item.SubGroup != nil {
			filtered := filterGroupsByItems(item.SubGroup.Items, filterSet)
			if len(filtered) > 0 {
				newGroup := *item.SubGroup
				newGroup.Items = filtered
				result = append(result, parser.GroupItem{SubGroup: &newGroup})
			}
		}
	}
	return result
}

// ==================== 单次模拟 ====================

// runSingleSimulation 执行一次完整模拟
func (s *Simulator) runSingleSimulation(
	files []*parser.MonsterDropFile,
	groupTrees map[string][]parser.GroupItem,
	allMonsterMapKills []monsterMapKills,
	config SimConfig,
	monsterFilterSet map[string]bool,
	pityItemSet map[string]bool,
) *SimResult {
	itemAgg := make(map[string]*ItemStat)
	mapAgg := make(map[string]*MapStat)
	monsterAgg := make(map[string]*MonsterStat)
	itemMonsterDrops := make(map[string]map[string]int64)
	itemMapDrops := make(map[string]map[string]int64)

	// 保底计数器：连续空击杀次数
	var pityCounter int

	for _, mmk := range allMonsterMapKills {
		monsterName := mmk.monsterName
		mapName := mmk.mapName
		kills := mmk.kills

		// 确保怪物统计存在
		if monsterAgg[monsterName] == nil {
			monsterAgg[monsterName] = &MonsterStat{MonsterName: monsterName}
		}
		monsterAgg[monsterName].KillCount += kills

		// 确保地图统计存在
		if mapAgg[mapName] == nil {
			mapAgg[mapName] = &MapStat{MapName: mapName}
		}
		mapAgg[mapName].MonsterCnt += kills

		// 获取该怪物的掉落组树
		tree, hasTree := groupTrees[monsterName]
		if !hasTree || len(tree) == 0 {
			// 无掉落配置，全部为空击杀
			continue
		}

		// 逐次模拟击杀
		for i := int64(0); i < kills; i++ {
			dropped := s.simulateKill(tree, itemAgg, mapAgg, monsterAgg,
				itemMonsterDrops, itemMapDrops, monsterName, mapName, config)

			if dropped {
				pityCounter = 0
			} else {
				pityCounter++
				// 保底触发
				if config.PityEnabled && pityCounter >= config.PityThreshold {
					// 保底：强制掉落一个可掉落的物品
					if s.pityDrop(tree, pityItemSet, itemAgg, mapAgg, monsterAgg,
						itemMonsterDrops, itemMapDrops, monsterName, mapName) {
						pityCounter = 0
					}
				}
			}
		}
	}

	// 汇总结果
	result := &SimResult{Config: config}
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
	result.TotalEmpty = result.TotalKills - s.countNonEmptyKills(result)
	result.ItemMonsterDrops = itemMonsterDrops
	result.ItemMapDrops = itemMapDrops
	return result
}

// simulateKill 模拟一次击杀的掉落，返回是否有物品掉落
func (s *Simulator) simulateKill(
	tree []parser.GroupItem,
	itemAgg map[string]*ItemStat,
	mapAgg map[string]*MapStat,
	monsterAgg map[string]*MonsterStat,
	itemMonsterDrops map[string]map[string]int64,
	itemMapDrops map[string]map[string]int64,
	monsterName, mapName string,
	config SimConfig,
) bool {
	dropped := false
	droppedItems := s.rollGroups(tree, config.MapRateModifier)
	for _, di := range droppedItems {
		dropped = true
		s.recordDrop(di.itemName, di.quantity, di.prob,
			itemAgg, mapAgg, monsterAgg,
			itemMonsterDrops, itemMapDrops,
			monsterName, mapName)
	}
	return dropped
}

// droppedItem 一次击杀中掉落的物品
type droppedItem struct {
	itemName string
	quantity int
	prob     float64
}

// rollGroups 递归roll掉落组树，返回本次击杀掉落的所有物品
func (s *Simulator) rollGroups(tree []parser.GroupItem, mapRateMod float64) []droppedItem {
	var result []droppedItem
	for _, item := range tree {
		if item.Entry != nil {
			// 普通掉落条目
			e := item.Entry
			denominator := float64(e.ProbabilityDenominator) / mapRateMod
			if denominator < 1 {
				denominator = 1
			}
			prob := float64(e.ProbabilityNumerator) / denominator
			if s.rng.Float64() < prob {
				result = append(result, droppedItem{
					itemName: e.ItemName,
					quantity: e.Quantity,
					prob:     prob,
				})
			}
		} else if item.SubGroup != nil {
			// 掉落组
			g := item.SubGroup
			// 先roll是否进入组
			groupProb := g.Probability()
			if groupProb < 1 && s.rng.Float64() >= groupProb {
				continue // 未进入组
			}
			if g.IsRandom {
				// RANDOM模式：组内只选一个
				// 先收集所有可能掉落的条目及其概率
				type candidate struct {
					items []droppedItem
					prob  float64
				}
				var candidates []candidate
				for _, sub := range g.Items {
					if sub.Entry != nil {
						e := sub.Entry
						denominator := float64(e.ProbabilityDenominator) / mapRateMod
						if denominator < 1 {
							denominator = 1
						}
						prob := float64(e.ProbabilityNumerator) / denominator
						candidates = append(candidates, candidate{
							items: []droppedItem{{itemName: e.ItemName, quantity: e.Quantity, prob: prob}},
							prob:  prob,
						})
					} else if sub.SubGroup != nil {
						// 嵌套子组：roll子组
						subDrops := s.rollGroups([]parser.GroupItem{sub}, mapRateMod)
						if len(subDrops) > 0 {
							candidates = append(candidates, candidate{items: subDrops, prob: 1.0})
						}
					}
				}
				if len(candidates) > 0 {
					// 按概率加权随机选择一个
					idx := s.weightedSelect(candidates)
					if idx >= 0 {
						result = append(result, candidates[idx].items...)
					}
				}
			} else {
				// 非RANDOM模式：每个条目独立roll
				subDrops := s.rollGroups(g.Items, mapRateMod)
				result = append(result, subDrops...)
			}
		}
	}
	return result
}

// weightedSelect 按概率加权选择一个候选
func (s *Simulator) weightedSelect(candidates []struct {
	items []droppedItem
	prob  float64
}) int {
	totalProb := 0.0
	for _, c := range candidates {
		totalProb += c.prob
	}
	if totalProb <= 0 {
		return -1
	}
	r := s.rng.Float64() * totalProb
	cumulative := 0.0
	for i, c := range candidates {
		cumulative += c.prob
		if r < cumulative {
			return i
		}
	}
	return len(candidates) - 1
}

// recordDrop 记录一次掉落
func (s *Simulator) recordDrop(
	itemName string, quantity int, prob float64,
	itemAgg map[string]*ItemStat,
	mapAgg map[string]*MapStat,
	monsterAgg map[string]*MonsterStat,
	itemMonsterDrops map[string]map[string]int64,
	itemMapDrops map[string]map[string]int64,
	monsterName, mapName string,
) {
	if itemAgg[itemName] == nil {
		itemAgg[itemName] = &ItemStat{
			ItemName:    itemName,
			Probability: prob,
			Maps:        make(map[string]bool),
			Monsters:    make(map[string]bool),
		}
	}
	itemAgg[itemName].DropCount++
	itemAgg[itemName].TotalQty += int64(quantity)
	itemAgg[itemName].Monsters[monsterName] = true
	itemAgg[itemName].Maps[mapName] = true

	if itemMonsterDrops[itemName] == nil {
		itemMonsterDrops[itemName] = make(map[string]int64)
	}
	itemMonsterDrops[itemName][monsterName]++

	if itemMapDrops[itemName] == nil {
		itemMapDrops[itemName] = make(map[string]int64)
	}
	itemMapDrops[itemName][mapName]++

	monsterAgg[monsterName].DropCount++
	if mapAgg[mapName] != nil {
		mapAgg[mapName].DropCount++
	}
}

// pityDrop 保底掉落：强制掉落一个可掉落的物品
func (s *Simulator) pityDrop(
	tree []parser.GroupItem,
	pityItemSet map[string]bool,
	itemAgg map[string]*ItemStat,
	mapAgg map[string]*MapStat,
	monsterAgg map[string]*MonsterStat,
	itemMonsterDrops map[string]map[string]int64,
	itemMapDrops map[string]map[string]int64,
	monsterName, mapName string,
) bool {
	// 收集所有可掉落的物品名
	var candidates []string
	s.collectItemNames(tree, &candidates)

	if len(candidates) == 0 {
		return false
	}

	// 如果指定了保底物品列表，优先选匹配的
	var target string
	if len(pityItemSet) > 0 {
		for _, name := range candidates {
			if pityItemSet[name] {
				target = name
				break
			}
		}
	}
	if target == "" {
		// 随机选一个
		target = candidates[s.rng.Intn(len(candidates))]
	}

	s.recordDrop(target, 1, 0, itemAgg, mapAgg, monsterAgg,
		itemMonsterDrops, itemMapDrops, monsterName, mapName)
	return true
}

// collectItemNames 收集掉落组树中所有物品名
func (s *Simulator) collectItemNames(tree []parser.GroupItem, names *[]string) {
	for _, item := range tree {
		if item.Entry != nil {
			*names = append(*names, item.Entry.ItemName)
		} else if item.SubGroup != nil {
			s.collectItemNames(item.SubGroup.Items, names)
		}
	}
}

// countNonEmptyKills 估算有掉落的击杀数（用于计算TotalEmpty）
func (s *Simulator) countNonEmptyKills(result *SimResult) int64 {
	// 简化计算：总掉落数即为有掉落的击杀数的最小估计
	// 一次击杀可能掉多个物品，所以实际有掉落的击杀数 ≤ 总掉落数
	// 但精确计算需要在模拟过程中跟踪，这里用总掉落数作为上界
	var totalItemDrops int64
	for _, item := range result.ItemStats {
		totalItemDrops += item.DropCount
	}
	// 每次击杀最多掉一个物品（保守估计）
	// 实际上一次击杀可能掉多个，所以这是下界
	if totalItemDrops > result.TotalKills {
		return result.TotalKills
	}
	return totalItemDrops
}

// ==================== 多次模拟统计 ====================

// calculateMultiStats 计算多次模拟的统计汇总
func (s *Simulator) calculateMultiStats(multi *MultiSimResult) {
	if len(multi.Runs) == 0 {
		return
	}

	n := float64(len(multi.Runs))

	// 基础统计
	for _, run := range multi.Runs {
		multi.AvgTotalKills += float64(run.TotalKills)
		multi.AvgTotalDrops += float64(run.TotalDrops)
		multi.AvgEmptyRate += run.EmptyRate()
	}
	multi.AvgTotalKills /= n
	multi.AvgTotalDrops /= n
	multi.AvgEmptyRate /= n

	// 最大最小值
	multi.MinTotalDrops = multi.Runs[0].TotalDrops
	multi.MaxTotalDrops = multi.Runs[0].TotalDrops
	for _, run := range multi.Runs {
		if run.TotalDrops < multi.MinTotalDrops {
			multi.MinTotalDrops = run.TotalDrops
		}
		if run.TotalDrops > multi.MaxTotalDrops {
			multi.MaxTotalDrops = run.TotalDrops
		}
	}

	// 标准差
	var variance float64
	for _, run := range multi.Runs {
		diff := float64(run.TotalDrops) - multi.AvgTotalDrops
		variance += diff * diff
	}
	variance /= n
	multi.StdDevDrops = math.Sqrt(variance)

	// 物品维度统计
	itemNames := make(map[string]bool)
	for _, run := range multi.Runs {
		for _, item := range run.ItemStats {
			itemNames[item.ItemName] = true
		}
	}

	for itemName := range itemNames {
		avg := &AvgItemStat{ItemName: itemName, MinDropCount: -1}
		var sum float64
		var values []int64

		for _, run := range multi.Runs {
			var cnt int64
			for _, item := range run.ItemStats {
				if item.ItemName == itemName {
					cnt = item.DropCount
					break
				}
			}
			values = append(values, cnt)
			sum += float64(cnt)
			if avg.MinDropCount < 0 || cnt < avg.MinDropCount {
				avg.MinDropCount = cnt
			}
			if cnt > avg.MaxDropCount {
				avg.MaxDropCount = cnt
			}
		}

		avg.AvgDropCount = sum / n
		var v float64
		for _, val := range values {
			d := float64(val) - avg.AvgDropCount
			v += d * d
		}
		avg.StdDev = math.Sqrt(v / n)

		multi.AvgItemStats = append(multi.AvgItemStats, avg)
	}

	sort.Slice(multi.AvgItemStats, func(i, j int) bool {
		return multi.AvgItemStats[i].AvgDropCount > multi.AvgItemStats[j].AvgDropCount
	})
}

// ==================== 格式化输出 ====================

// FormatResult 格式化单次模拟结果为文本
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
	if result.Config.PityEnabled {
		sb += fmt.Sprintf("保底触发: %d次\n", result.PityTriggered)
	}
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

// FormatMultiResult 格式化多次模拟结果
func FormatMultiResult(multi *MultiSimResult) string {
	var sb string
	sb += fmt.Sprintf("========== 多次模拟统计结果 ==========\n")
	sb += fmt.Sprintf("模拟轮数: %d\n", multi.RunCount)
	sb += fmt.Sprintf("模拟时长: %.1f小时 | 击杀比例: %.0f%%\n", multi.Config.DurationHours, multi.Config.KillRatio*100)
	sb += fmt.Sprintf("总耗时: %v\n", multi.Duration)
	sb += fmt.Sprintf("----------------------------------\n")
	sb += fmt.Sprintf("平均击杀: %.0f\n", multi.AvgTotalKills)
	sb += fmt.Sprintf("平均掉落: %.0f (最小:%d 最大:%d 标准差:%.1f)\n",
		multi.AvgTotalDrops, multi.MinTotalDrops, multi.MaxTotalDrops, multi.StdDevDrops)
	sb += fmt.Sprintf("平均空爆率: %.2f%%\n", multi.AvgEmptyRate*100)
	sb += fmt.Sprintf("95%%置信区间: [%.0f, %.0f]\n",
		multi.AvgTotalDrops-1.96*multi.StdDevDrops,
		multi.AvgTotalDrops+1.96*multi.StdDevDrops)
	sb += fmt.Sprintf("\n")

	sb += fmt.Sprintf("----- 物品掉落统计 -----\n")
	for _, item := range multi.AvgItemStats {
		sb += fmt.Sprintf("  %s: 平均%.1f (最小:%d 最大:%d 标准差:%.1f)\n",
			item.ItemName, item.AvgDropCount, item.MinDropCount, item.MaxDropCount, item.StdDev)
	}

	return sb
}
