package app

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/yueyoue/legend-drop-tool/pkg/auth"
	"github.com/yueyoue/legend-drop-tool/pkg/backup"
	"github.com/yueyoue/legend-drop-tool/pkg/config"
	"github.com/yueyoue/legend-drop-tool/pkg/editor"
	"github.com/yueyoue/legend-drop-tool/pkg/parser"
	"github.com/yueyoue/legend-drop-tool/pkg/simulator"
)

// App 后端绑定，所有 public 方法自动暴露给前端
type App struct {
	ctx context.Context

	cfg       *config.AppConfig
	authMgr   *auth.LocalAuth
	editor    *editor.Editor
	simulator *simulator.Simulator
	backupMgr *backup.Manager

	currentResults []*parser.ParseResult
	currentEngine  parser.EngineType
	monGenEntries  []*parser.MonGenEntry
	mapInfoLookup  map[string]string

	// 模拟筛选
	selectedMonsters []string
	selectedItems    []string
	selectedMaps     []string

	// 模拟结果
	simMultiResult *simulator.MultiSimResult
	simResult      *simulator.SimResult
}

func NewApp() *App {
	cfg, _ := config.Load()
	return &App{
		cfg:       cfg,
		editor:    editor.New(),
		simulator: simulator.New(),
		authMgr:   auth.NewLocalAuth(),
	}
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
}

// ============================================================
// 配置
// ============================================================

// GetConfig 获取配置
func (a *App) GetConfig() *config.AppConfig {
	return a.cfg
}

// SaveConfig 保存配置
func (a *App) SaveConfig() error {
	return config.Save(a.cfg)
}

// SetServerPath 设置服务端路径
func (a *App) SetServerPath(path string) {
	a.cfg.ServerRoot = path
	config.Save(a.cfg)
}

// ============================================================
// 引擎检测
// ============================================================

// DetectEngine 自动检测引擎类型
func (a *App) DetectEngine(serverRoot string) string {
	engine := parser.DetectEngine(serverRoot)
	a.currentEngine = engine
	return engine.String()
}

// SetEngine 手动设置引擎类型
func (a *App) SetEngine(engineName string) {
	switch engineName {
	case "HERO":
		a.currentEngine = parser.EngineHERO
	case "GOM":
		a.currentEngine = parser.EngineGOM
	case "GEE":
		a.currentEngine = parser.EngineGEE
	case "BLUE":
		a.currentEngine = parser.EngineBLUE
	default:
		a.currentEngine = parser.EngineUnknown
	}
}

// ============================================================
// 文件加载
// ============================================================

// MonsterInfo 怪物简要信息（传给前端）
type MonsterInfo struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
	Index int    `json:"index"`
}

// LoadFiles 加载爆率文件
func (a *App) LoadFiles(serverRoot string) (*LoadResult, error) {
	if serverRoot == "" {
		return nil, fmt.Errorf("请选择服务端目录")
	}

	monItemsDir := filepath.Join(serverRoot, "Mir200", "Envir", "MonItems")
	results, err := parser.ParseDirectory(monItemsDir, a.currentEngine)
	if err != nil {
		return nil, fmt.Errorf("加载爆率文件失败: %v\n路径: %s", err, monItemsDir)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("未找到爆率文件(.txt)")
	}

	a.currentResults = results
	a.backupMgr = backup.New(serverRoot)
	a.cfg.MonItemsDir = monItemsDir
	a.cfg.ServerRoot = serverRoot
	config.Save(a.cfg)

	// 加载 MonGen
	monGenPath := filepath.Join(serverRoot, "Mir200", "Envir", "MonGen.txt")
	if entries, err := parser.ParseMonGen(monGenPath); err == nil {
		a.monGenEntries = entries
	}

	// 加载 MapInfo
	mapInfoPath := filepath.Join(serverRoot, "Mir200", "Envir", "MapInfo.txt")
	if info, err := parser.ParseMapInfo(mapInfoPath); err == nil {
		a.mapInfoLookup = info
	}

	// 构建怪物列表
	var monsters []MonsterInfo
	totalEntries := 0
	for i, r := range results {
		count := 0
		for _, e := range r.File.Entries {
			if e.IsEditable() {
				count++
			}
		}
		totalEntries += count
		monsters = append(monsters, MonsterInfo{
			Name:  r.File.MonsterName,
			Count: count,
			Index: i,
		})
	}

	// 收集警告
	var warnings []string
	for _, r := range results {
		for _, w := range r.Warnings {
			warnings = append(warnings, fmt.Sprintf("[%s] %s", r.File.MonsterName, w))
		}
	}

	return &LoadResult{
		Monsters:    monsters,
		TotalFiles:  len(results),
		TotalEntries: totalEntries,
		Engine:      a.currentEngine.String(),
		Warnings:    warnings,
	}, nil
}

type LoadResult struct {
	Monsters     []MonsterInfo `json:"monsters"`
	TotalFiles   int           `json:"totalFiles"`
	TotalEntries int           `json:"totalEntries"`
	Engine       string        `json:"engine"`
	Warnings     []string      `json:"warnings"`
}

// ============================================================
// 条目操作
// ============================================================

// EntryInfo 条目信息（传给前端）
type EntryInfo struct {
	Index        int    `json:"index"`
	Depth        int    `json:"depth"`
	IsComment    bool   `json:"isComment"`
	IsCallRef    bool   `json:"isCallRef"`
	IsChildStart bool   `json:"isChildStart"`
	IsChildEnd   bool   `json:"isChildEnd"`
	IsCaseStart  bool   `json:"isCaseStart"`
	IsIfStart    bool   `json:"isIfStart"`
	IsEditable   bool   `json:"isEditable"`
	ProbStr      string `json:"probStr"`
	ProbNum      int    `json:"probNum"`
	ProbDen      int    `json:"probDen"`
	ItemName     string `json:"itemName"`
	Quantity     int    `json:"quantity"`
	HasTrigger   bool   `json:"hasTrigger"`
	TriggerName  string `json:"triggerName"`
	RawLine      string `json:"rawLine"`
	CallPath     string `json:"callPath"`
	CallLabel    string `json:"callLabel"`
	ChildProb    string `json:"childProb"`
	ChildRandom  bool   `json:"childRandom"`
	CaseExpr     string `json:"caseExpr"`
}

// GetEntries 获取指定怪物的条目列表
func (a *App) GetEntries(monsterIndex int) ([]EntryInfo, error) {
	if monsterIndex < 0 || a.currentResults == nil || monsterIndex >= len(a.currentResults) {
		return nil, fmt.Errorf("无效的怪物索引")
	}
	file := a.currentResults[monsterIndex].File
	var entries []EntryInfo
	for i, e := range file.Entries {
		entries = append(entries, EntryInfo{
			Index:        i,
			Depth:        e.Depth,
			IsComment:    e.IsComment,
			IsCallRef:    e.IsCallRef,
			IsChildStart: e.IsChildStart,
			IsChildEnd:   e.IsChildEnd,
			IsCaseStart:  e.IsCaseStart,
			IsIfStart:    e.IsIfStart,
			IsEditable:   e.IsEditable(),
			ProbStr:      e.ProbabilityStr(),
			ProbNum:      e.ProbabilityNumerator,
			ProbDen:      e.ProbabilityDenominator,
			ItemName:     e.ItemName,
			Quantity:     e.Quantity,
			HasTrigger:   e.HasTrigger,
			TriggerName:  e.TriggerName,
			RawLine:      strings.TrimSpace(e.RawLine),
			CallPath:     e.CallPath,
			CallLabel:    e.CallLabel,
			ChildProb:    e.ChildProbability,
			ChildRandom:  e.ChildRandom,
			CaseExpr:     e.CaseExpression,
		})
	}
	return entries, nil
}

// ModifyEntry 修改条目概率
func (a *App) ModifyEntry(monsterIndex, entryIndex, num, den, qty int) error {
	if a.currentResults == nil || monsterIndex < 0 || monsterIndex >= len(a.currentResults) {
		return fmt.Errorf("无效的怪物索引")
	}
	file := a.currentResults[monsterIndex].File
	if entryIndex < 0 || entryIndex >= len(file.Entries) {
		return fmt.Errorf("无效的条目索引")
	}
	entry := file.Entries[entryIndex]
	a.editor.ModifyEntry(file, entry, num, den)
	a.editor.ModifyQuantity(file, entry, qty)
	return nil
}

// AddEntry 新增条目
func (a *App) AddEntry(monsterIndex int, itemName string, num, den, qty int) error {
	if a.currentResults == nil || monsterIndex < 0 || monsterIndex >= len(a.currentResults) {
		return fmt.Errorf("无效的怪物索引")
	}
	file := a.currentResults[monsterIndex].File
	a.editor.AddEntry(file, itemName, num, den, qty)
	return nil
}

// BatchMultiply 批量倍率调整
func (a *App) BatchMultiply(monsterIndex int, multiplier float64) (int, error) {
	if a.currentResults == nil || monsterIndex < 0 || monsterIndex >= len(a.currentResults) {
		return 0, fmt.Errorf("无效的怪物索引")
	}
	file := a.currentResults[monsterIndex].File
	count := a.editor.BatchMultiply(file, multiplier)
	return count, nil
}

// BatchSetProb 批量设置概率
func (a *App) BatchSetProb(monsterIndex, num, den int) (int, error) {
	if a.currentResults == nil || monsterIndex < 0 || monsterIndex >= len(a.currentResults) {
		return 0, fmt.Errorf("无效的怪物索引")
	}
	file := a.currentResults[monsterIndex].File
	count := a.editor.BatchSetAll(file, num, den)
	return count, nil
}

// SaveFile 保存文件
func (a *App) SaveFile(monsterIndex int) error {
	if a.currentResults == nil || monsterIndex < 0 || monsterIndex >= len(a.currentResults) {
		return fmt.Errorf("无效的怪物索引")
	}
	file := a.currentResults[monsterIndex].File

	if a.cfg.AutoBackup && a.backupMgr != nil {
		a.backupMgr.BackupFile(file.FilePath)
	}

	a.editor.RebuildRawContent(file)
	return a.editor.SaveFile(file)
}

// ============================================================
// 备份
// ============================================================

// BackupCurrent 备份当前文件
func (a *App) BackupCurrent(monsterIndex int) (string, error) {
	if a.currentResults == nil || monsterIndex < 0 || monsterIndex >= len(a.currentResults) || a.backupMgr == nil {
		return "", fmt.Errorf("请先加载服务端目录")
	}
	file := a.currentResults[monsterIndex].File
	return a.backupMgr.BackupFile(file.FilePath)
}

// BackupAll 备份整个目录
func (a *App) BackupAll() (string, error) {
	if a.backupMgr == nil {
		return "", fmt.Errorf("请先加载服务端目录")
	}
	dir := filepath.Join(a.cfg.ServerRoot, "Mir200", "Envir", "MonItems")
	return a.backupMgr.BackupDirectory(dir)
}

// ============================================================
// 异常检测
// ============================================================

// AnomalyInfo 异常信息
type AnomalyInfo struct {
	Monster string `json:"monster"`
	Item    string `json:"item"`
	Prob    string `json:"prob"`
	Reason  string `json:"reason"`
}

// DetectAnomaly 爆率异常检测
func (a *App) DetectAnomaly() []AnomalyInfo {
	if a.currentResults == nil {
		return nil
	}
	var anomalies []AnomalyInfo
	for _, r := range a.currentResults {
		name := r.File.MonsterName
		hasDrop := false
		for _, e := range r.File.Entries {
			if !e.IsEditable() {
				continue
			}
			hasDrop = true
			if e.ProbabilityDenominator <= 1 && e.ProbabilityNumerator >= 1 {
				anomalies = append(anomalies, AnomalyInfo{
					Monster: name, Item: e.ItemName, Prob: e.ProbabilityStr(),
					Reason: "必掉 - 可能是测试配置",
				})
			}
			if e.ProbabilityDenominator > 100000 {
				anomalies = append(anomalies, AnomalyInfo{
					Monster: name, Item: e.ItemName, Prob: e.ProbabilityStr(),
					Reason: "极低爆率 - 可能配置错误",
				})
			}
		}
		if !hasDrop && len(r.File.Entries) > 0 {
			anomalies = append(anomalies, AnomalyInfo{
				Monster: name, Reason: "无有效掉落配置",
			})
		}
	}
	return anomalies
}

// ============================================================
// 模拟
// ============================================================

// SimRequest 模拟请求参数
type SimRequest struct {
	DurationHours float64  `json:"durationHours"`
	KillRatioPct  float64  `json:"killRatioPct"`
	PityEnabled   bool     `json:"pityEnabled"`
	PityThreshold int      `json:"pityThreshold"`
	RunCount      int      `json:"runCount"`
	Monsters      []string `json:"monsters"`
	Items         []string `json:"items"`
	Maps          []string `json:"maps"`
}

// SimResponse 模拟结果
type SimResponse struct {
	TotalKills   int64           `json:"totalKills"`
	TotalDrops   int64           `json:"totalDrops"`
	EmptyRate    float64         `json:"emptyRate"`
	Duration     string          `json:"duration"`
	ItemStats    []SimItemStat   `json:"itemStats"`
	MapStats     []SimMapStat    `json:"mapStats"`
	MonsterStats []SimMonsterStat `json:"monsterStats"`
	RunCount     int             `json:"runCount"`
	AvgDrops     float64         `json:"avgDrops"`
	MinDrops     int64           `json:"minDrops"`
	MaxDrops     int64           `json:"maxDrops"`
	AvgEmptyRate float64         `json:"avgEmptyRate"`

	// 级联筛选数据
	ItemMonsterDrops    map[string]map[string]int64            `json:"itemMonsterDrops"`
	ItemMapDrops        map[string]map[string]int64            `json:"itemMapDrops"`
	ItemMonsterMapDrops map[string]map[string]map[string]int64 `json:"itemMonsterMapDrops"`
}

type SimItemStat struct {
	ItemName  string  `json:"itemName"`
	DropCount int64   `json:"dropCount"`
	Prob      float64 `json:"prob"`
}

type SimMapStat struct {
	MapName   string `json:"mapName"`
	DropCount int64  `json:"dropCount"`
}

type SimMonsterStat struct {
	MonsterName string `json:"monsterName"`
	KillCount   int64  `json:"killCount"`
	DropCount   int64  `json:"dropCount"`
}

// RunSimulation 运行模拟
func (a *App) RunSimulation(req SimRequest) (*SimResponse, error) {
	if a.currentResults == nil {
		return nil, fmt.Errorf("请先加载爆率文件")
	}

	killRatio := req.KillRatioPct / 100.0
	if killRatio < 0 || killRatio > 1 {
		return nil, fmt.Errorf("消灭比例必须在0~100之间")
	}
	if req.PityThreshold <= 0 {
		req.PityThreshold = 100
	}
	if req.RunCount <= 0 {
		req.RunCount = 1
	}

	cfg := simulator.SimConfig{
		DurationHours:   req.DurationHours,
		KillRatio:       killRatio,
		RefreshInterval: 60,
		RefreshCount:    10,
		MapRateModifier: 1.0,
		PityEnabled:     req.PityEnabled,
		PityThreshold:   req.PityThreshold,
		RunCount:        req.RunCount,
		MonsterFilter:   req.Monsters,
		ItemFilter:      req.Items,
		MapFilter:       req.Maps,
	}

	var files []*parser.MonsterDropFile
	for _, r := range a.currentResults {
		files = append(files, r.File)
	}

	multiResult := a.simulator.SimulateAll(files, a.monGenEntries, cfg)
	a.simMultiResult = multiResult

	var simResult *simulator.SimResult
	if len(multiResult.Runs) > 0 {
		simResult = multiResult.Runs[0]
		a.simResult = simResult
	}

	// 构建响应
	resp := &SimResponse{
		RunCount: req.RunCount,
		Duration: multiResult.Duration.String(),
	}

	if simResult != nil {
		resp.TotalKills = simResult.TotalKills
		resp.TotalDrops = simResult.TotalDrops
		resp.EmptyRate = simResult.EmptyRate()

		// 物品统计（按掉落数排序）
		for _, s := range simResult.ItemStats {
			resp.ItemStats = append(resp.ItemStats, SimItemStat{
				ItemName:  s.ItemName,
				DropCount: s.DropCount,
				Prob:      s.Probability,
			})
		}
		sort.Slice(resp.ItemStats, func(i, j int) bool {
			return resp.ItemStats[i].DropCount > resp.ItemStats[j].DropCount
		})

		// 地图统计
		for _, s := range simResult.MapStats {
			resp.MapStats = append(resp.MapStats, SimMapStat{
				MapName:   s.MapName,
				DropCount: s.DropCount,
			})
		}
		sort.Slice(resp.MapStats, func(i, j int) bool {
			return resp.MapStats[i].DropCount > resp.MapStats[j].DropCount
		})

		// 怪物统计
		for _, s := range simResult.MonsterStats {
			resp.MonsterStats = append(resp.MonsterStats, SimMonsterStat{
				MonsterName: s.MonsterName,
				KillCount:   s.KillCount,
				DropCount:   s.DropCount,
			})
		}
		sort.Slice(resp.MonsterStats, func(i, j int) bool {
			return resp.MonsterStats[i].DropCount > resp.MonsterStats[j].DropCount
		})
	}

	if req.RunCount > 1 {
		resp.AvgDrops = multiResult.AvgTotalDrops
		resp.MinDrops = multiResult.MinTotalDrops
		resp.MaxDrops = multiResult.MaxTotalDrops
		resp.AvgEmptyRate = multiResult.AvgEmptyRate
	}

	// 级联筛选数据
	if simResult != nil {
		resp.ItemMonsterDrops = simResult.ItemMonsterDrops
		resp.ItemMapDrops = simResult.ItemMapDrops
		resp.ItemMonsterMapDrops = simResult.ItemMonsterMapDrops
	}

	return resp, nil
}

// ExportSimResult 导出模拟结果
func (a *App) ExportSimResult() (string, error) {
	if a.simResult == nil {
		return "", fmt.Errorf("请先运行模拟")
	}
	if a.simMultiResult != nil && a.simMultiResult.RunCount > 1 {
		return simulator.FormatMultiResult(a.simMultiResult), nil
	}
	return simulator.FormatResult(a.simResult), nil
}

// ============================================================
// 授权
// ============================================================

// GetMachineID 获取机器码
func (a *App) GetMachineID() string {
	id, _ := a.authMgr.GetMachineID()
	return id
}

// GetLicenseStatus 获取授权状态
func (a *App) GetLicenseStatus() map[string]interface{} {
	info, _ := a.authMgr.CheckLicense()
	return map[string]interface{}{
		"isActive": info.IsActive,
		"type":     info.Type,
	}
}

// Activate 激活
func (a *App) Activate(code string) (map[string]interface{}, error) {
	if code == "" {
		return nil, fmt.Errorf("请输入激活码")
	}
	info, err := a.authMgr.Activate(code)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"isActive": info.IsActive,
		"type":     info.Type,
	}, nil
}

// ============================================================
// 筛选设置
// ============================================================

// SetMonsterFilter 设置怪物筛选
func (a *App) SetMonsterFilter(monsters []string) {
	a.selectedMonsters = monsters
}

// SetItemFilter 设置物品筛选
func (a *App) SetItemFilter(items []string) {
	a.selectedItems = items
}

// SetMapFilter 设置地图筛选
func (a *App) SetMapFilter(maps []string) {
	a.selectedMaps = maps
}

// GetAllMonsterNames 获取所有怪物名称（用于筛选器）
func (a *App) GetAllMonsterNames() []string {
	if a.currentResults == nil {
		return nil
	}
	var names []string
	for _, r := range a.currentResults {
		names = append(names, r.File.MonsterName)
	}
	return names
}

// GetAllItemNames 获取所有物品名称（用于筛选器）
func (a *App) GetAllItemNames() []string {
	if a.currentResults == nil {
		return nil
	}
	seen := make(map[string]bool)
	var names []string
	for _, r := range a.currentResults {
		for _, e := range r.File.Entries {
			if e.IsEditable() && !seen[e.ItemName] {
				seen[e.ItemName] = true
				names = append(names, e.ItemName)
			}
		}
	}
	sort.Strings(names)
	return names
}

// GetAllMapNames 获取所有地图名称（用于筛选器）
func (a *App) GetAllMapNames() []string {
	if a.monGenEntries == nil && a.currentResults == nil {
		return nil
	}
	seen := make(map[string]bool)
	var names []string

	// 从 MonGen 提取地图名
	for _, e := range a.monGenEntries {
		mapName := strings.TrimSpace(e.MapName)
		if mapName != "" && !seen[mapName] {
			seen[mapName] = true
			displayName := mapName
			if a.mapInfoLookup != nil {
				if desc, ok := a.mapInfoLookup[mapName]; ok && desc != "" {
					displayName = mapName + " (" + desc + ")"
				}
			}
			names = append(names, displayName)
		}
	}

	// 如果 MonGen 没数据，从怪物掉落文件名推断
	if len(names) == 0 {
		for _, r := range a.currentResults {
			mapName := r.File.MonsterName
			if !seen[mapName] {
				seen[mapName] = true
				names = append(names, mapName)
			}
		}
	}

	sort.Strings(names)
	return names
}

// ============================================================
// 文件对话框（Wails 前端调用）
// ============================================================

// SelectDirectory 打开系统目录选择对话框
func (a *App) SelectDirectory() (string, error) {
	path, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择传奇服务端根目录",
	})
	if err != nil {
		return "", fmt.Errorf("打开目录对话框失败: %v", err)
	}
	return path, nil
}

// SelectFile 打开系统文件选择对话框
func (a *App) SelectFile(title string) (string, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: title,
	})
	if err != nil {
		return "", fmt.Errorf("打开文件对话框失败: %v", err)
	}
	return path, nil
}

// FormatEntryDisplay 格式化条目显示文本
func (a *App) FormatEntryDisplay(monsterIndex int) []string {
	if a.currentResults == nil || monsterIndex < 0 || monsterIndex >= len(a.currentResults) {
		return nil
	}
	file := a.currentResults[monsterIndex].File
	var lines []string
	for _, e := range file.Entries {
		indent := strings.Repeat("  ", e.Depth)
		switch {
		case e.IsComment:
			lines = append(lines, indent+"📝 "+strings.TrimSpace(e.RawLine))
		case e.IsCallRef:
			s := indent + "📎 #CALL [" + e.CallPath + "]"
			if e.CallLabel != "" {
				s += " " + e.CallLabel
			}
			lines = append(lines, s)
		case e.IsChildStart:
			s := indent + "📦 #CHILD " + e.ChildProbability
			if e.ChildRandom {
				s += " RANDOM"
			}
			lines = append(lines, s)
		case e.IsChildEnd:
			lines = append(lines, indent+"📦 )")
		case e.IsCaseStart:
			lines = append(lines, indent+"🔀 #CASE "+e.CaseExpression)
		case e.IsIfStart:
			lines = append(lines, indent+"🔀 #IF "+e.CaseExpression)
		case e.IsEditable():
			trigger := ""
			if e.HasTrigger {
				trigger = " |" + e.TriggerName
			}
			lines = append(lines, fmt.Sprintf("%s🎯 %s  %s%s  x%d",
				indent, e.ProbabilityStr(), e.ItemName, trigger, e.Quantity))
		default:
			lines = append(lines, indent+strings.TrimSpace(e.RawLine))
		}
	}
	return lines
}

// ============================================================
// 爆率调配（Rate Tuning）
// ============================================================

// ItemDropSource 单个掉落来源
type ItemDropSource struct {
	MonsterName  string  `json:"monsterName"`
	MonsterIndex int     `json:"monsterIndex"` // currentResults 索引
	EntryIndex   int     `json:"entryIndex"`   // 条目索引
	MapName      string  `json:"mapName"`
	ProbNum      int     `json:"probNum"`
	ProbDen      int     `json:"probDen"`
	ProbStr      string  `json:"probStr"`
	Quantity     int     `json:"quantity"`
	KillPerHour  float64 `json:"killPerHour"`  // 每小时杀怪数
	ExpectHours  float64 `json:"expectHours"`  // 当前期望多少小时出一个
}

// RateAnalysis 物品掉落分析结果
type RateAnalysis struct {
	ItemName     string           `json:"itemName"`
	Sources      []ItemDropSource `json:"sources"`
	TotalKPH     float64          `json:"totalKph"`      // 所有来源总每小时杀怪数
	TotalExpectH float64          `json:"totalExpectH"`  // 综合期望小时
	FromSim      bool             `json:"fromSim"`       // 是否来自模拟数据
}

// RateChange 单条爆率修改建议
type RateChange struct {
	MonsterName  string `json:"monsterName"`
	MonsterIndex int    `json:"monsterIndex"`
	EntryIndex   int    `json:"entryIndex"`
	MapName      string `json:"mapName"`
	OldNum       int    `json:"oldNum"`
	OldDen       int    `json:"oldDen"`
	NewNum       int    `json:"newNum"`
	NewDen       int    `json:"newDen"`
	NewExpectH   float64 `json:"newExpectH"` // 修改后期望小时
}

// TargetRate 目标爆率
type TargetRate struct {
	MapName     string  `json:"mapName"`     // 地图名
	TargetHours float64 `json:"targetHours"` // 目标多少小时出一个
}

// CopyResult 复制结果
type CopyResult struct {
	Success  bool   `json:"success"`
	Modified int    `json:"modified"` // 修改了多少条
	Message  string `json:"message"`
}

// HasSimResult 检查是否有模拟结果
func (a *App) HasSimResult() bool {
	return a.simResult != nil
}

// AnalyzeItemDrops 分析指定物品的所有掉落来源
// 有模拟结果时使用模拟数据（更准确），否则用爆率文件+MonGen计算
func (a *App) AnalyzeItemDrops(itemName string) (*RateAnalysis, error) {
	if a.currentResults == nil {
		return nil, fmt.Errorf("请先加载爆率文件")
	}
	if itemName == "" {
		return nil, fmt.Errorf("请输入物品名称")
	}

	analysis := &RateAnalysis{
		ItemName: itemName,
	}

	// 优先使用模拟数据
	if a.simResult != nil {
		return a.analyzeItemDropsFromSim(itemName, analysis)
	}

	// 纯文件计算
	return a.analyzeItemDropsFromFile(itemName, analysis)
}

// analyzeItemDropsFromSim 使用模拟数据分析
func (a *App) analyzeItemDropsFromSim(itemName string, analysis *RateAnalysis) (*RateAnalysis, error) {
	analysis.FromSim = true

	// 从模拟结果获取数据
	monsterDrops := a.simResult.ItemMonsterDrops[itemName]
	mapDrops := a.simResult.ItemMapDrops[itemName]
	monsterMapDrops := a.simResult.ItemMonsterMapDrops[itemName]

	if len(monsterDrops) == 0 {
		return nil, fmt.Errorf("模拟结果中未找到物品「%s」的掉落数据", itemName)
	}

	// 构建怪物→地图映射
	monsterToMap := make(map[string]string)
	for _, e := range a.monGenEntries {
		if _, ok := monsterDrops[e.MonsterName]; ok {
			monsterToMap[e.MonsterName] = e.MapName
		}
	}

	// 每个怪物的数据
	durationH := a.simResult.Duration.Hours()
	if durationH <= 0 {
		durationH = 1
	}

	for monsterName, dropCount := range monsterDrops {
		mapName := monsterToMap[monsterName]
		if mapName == "" {
			mapName = "未知地图"
		}
		displayMap := mapName
		if a.mapInfoLookup != nil {
			if desc, ok := a.mapInfoLookup[mapName]; ok && desc != "" {
				displayMap = mapName + " (" + desc + ")"
			}
		}

		// 查找对应的爆率文件条目获取爆率
		var probNum, probDen, qty int
		var mi, ei int
		for idx, r := range a.currentResults {
			if r.File.MonsterName == monsterName {
				mi = idx
				for j, e := range r.File.Entries {
					if e.IsEditable() && e.ItemName == itemName {
						probNum = e.ProbabilityNumerator
						probDen = e.ProbabilityDenominator
						qty = e.Quantity
						ei = j
						break
					}
				}
				break
				}
		}

		// 模拟实际每小时掉落数
		simDropsPerHour := float64(dropCount) / durationH
		var expectH float64
		if simDropsPerHour > 0 {
			expectH = 1.0 / simDropsPerHour
		} else {
			expectH = 999999
		}

		// 该怪物在该地图的模拟杀怪数
		var monsterKills int64
		if a.simResult.MonsterStats != nil {
			for _, ms := range a.simResult.MonsterStats {
				if ms.MonsterName == monsterName {
					monsterKills = ms.KillCount
					break
				}
			}
		}
		kph := float64(monsterKills) / durationH

		analysis.Sources = append(analysis.Sources, ItemDropSource{
			MonsterName:  monsterName,
			MonsterIndex: mi,
			EntryIndex:   ei,
			MapName:      displayMap,
			ProbNum:      probNum,
			ProbDen:      probDen,
			ProbStr:      fmt.Sprintf("%d/%d", probNum, probDen),
			Quantity:     qty,
			KillPerHour:  kph,
			ExpectHours:  expectH,
		})

		analysis.TotalKPH += kph
	}

	// 按地图排序
	sort.Slice(analysis.Sources, func(i, j int) bool {
		if analysis.Sources[i].MapName != analysis.Sources[j].MapName {
			return analysis.Sources[i].MapName < analysis.Sources[j].MapName
		}
		return analysis.Sources[i].ExpectHours < analysis.Sources[j].ExpectHours
	})

	// 综合期望
	if mapDrops != nil {
		totalDropCount := int64(0)
		for _, c := range mapDrops {
			totalDropCount += c
		}
		simTotalDropsPerHour := float64(totalDropCount) / durationH
		if simTotalDropsPerHour > 0 {
			analysis.TotalExpectH = 1.0 / simTotalDropsPerHour
		} else {
			analysis.TotalExpectH = 999999
		}
	}

	_ = monsterMapDrops // 保留，后续可用于更细粒度分析
	return analysis, nil
}

// analyzeItemDropsFromFile 使用爆率文件+MonGen计算
func (a *App) analyzeItemDropsFromFile(itemName string, analysis *RateAnalysis) (*RateAnalysis, error) {

	for mi, r := range a.currentResults {
		monsterName := r.File.MonsterName
		for ei, e := range r.File.Entries {
			if !e.IsEditable() {
				continue
			}
			if e.ItemName != itemName {
				continue
			}

			// 从 MonGen 获取该怪物的刷怪信息
			refreshSec, count := findMonGenInfo(a.monGenEntries, monsterName)
			kph := float64(count) * 3600.0 / refreshSec // 每小时杀怪数

			prob := e.Probability()
			var expectH float64
			if prob > 0 && kph > 0 {
				expectH = 1.0 / (kph * prob)
			} else {
				expectH = 999999
			}

			// 查找该怪物所在的地图
			mapName := findMapForMonster(a.monGenEntries, monsterName)
			displayMap := mapName
			if a.mapInfoLookup != nil {
				if desc, ok := a.mapInfoLookup[mapName]; ok && desc != "" {
					displayMap = mapName + " (" + desc + ")"
				}
			}

			analysis.Sources = append(analysis.Sources, ItemDropSource{
				MonsterName:  monsterName,
				MonsterIndex: mi,
				EntryIndex:   ei,
				MapName:      displayMap,
				ProbNum:      e.ProbabilityNumerator,
				ProbDen:      e.ProbabilityDenominator,
				ProbStr:      e.ProbabilityStr(),
				Quantity:     e.Quantity,
				KillPerHour:  kph,
				ExpectHours:  expectH,
			})

			analysis.TotalKPH += kph
		}
	}

	if len(analysis.Sources) == 0 {
		return nil, fmt.Errorf("未找到物品「%s」的掉落配置", itemName)
	}

	// 综合期望：总杀怪速率下出一个的时间
	totalProbRate := 0.0
	for _, s := range analysis.Sources {
		prob := float64(s.ProbNum) / float64(s.ProbDen)
		totalProbRate += s.KillPerHour * prob
	}
	if totalProbRate > 0 {
		analysis.TotalExpectH = 1.0 / totalProbRate
	} else {
		analysis.TotalExpectH = 999999
	}

	return analysis, nil
}

// RecommendRates 根据目标时间推荐爆率修改
// targets: 每个地图的目标小时数
func (a *App) RecommendRates(itemName string, targets []TargetRate) ([]RateChange, error) {
	if a.currentResults == nil {
		return nil, fmt.Errorf("请先加载爆率文件")
	}

	// 构建目标 map
	targetMap := make(map[string]float64)
	for _, t := range targets {
		targetMap[t.MapName] = t.TargetHours
	}

	var changes []RateChange

	for mi, r := range a.currentResults {
		monsterName := r.File.MonsterName
		for ei, e := range r.File.Entries {
			if !e.IsEditable() || e.ItemName != itemName {
				continue
			}

			mapName := findMapForMonster(a.monGenEntries, monsterName)
			displayMap := mapName
			if a.mapInfoLookup != nil {
				if desc, ok := a.mapInfoLookup[mapName]; ok && desc != "" {
					displayMap = mapName + " (" + desc + ")"
				}
			}

			// 匹配目标：先尝试 displayMap，再尝试 mapName
			targetH, ok := targetMap[displayMap]
			if !ok {
				targetH, ok = targetMap[mapName]
			}
			if !ok {
				continue // 此地图无目标，跳过
			}

			refreshSec, count := findMonGenInfo(a.monGenEntries, monsterName)
			kph := float64(count) * 3600.0 / refreshSec

			// 反算推荐分母：
			// expectH = 1 / (kph * num/den)
			// => den = kph * num * targetH
			var newDen int
			if kph > 0 && targetH > 0 {
				rawDen := kph * float64(e.ProbabilityNumerator) * targetH
				newDen = roundToNiceDenominator(rawDen)
			} else {
				newDen = e.ProbabilityDenominator
			}

			newExpectH := 1.0 / (kph * float64(e.ProbabilityNumerator) / float64(newDen))

			changes = append(changes, RateChange{
				MonsterName:  monsterName,
				MonsterIndex: mi,
				EntryIndex:   ei,
				MapName:      displayMap,
				OldNum:       e.ProbabilityNumerator,
				OldDen:       e.ProbabilityDenominator,
				NewNum:       e.ProbabilityNumerator,
				NewDen:       newDen,
				NewExpectH:   newExpectH,
			})
		}
	}

	if len(changes) == 0 {
		return nil, fmt.Errorf("未找到物品「%s」的匹配掉落来源", itemName)
	}

	return changes, nil
}

// ApplyRecommendedRates 应用推荐爆率修改
func (a *App) ApplyRecommendedRates(itemName string, changes []RateChange) (int, error) {
	if a.currentResults == nil {
		return 0, fmt.Errorf("请先加载爆率文件")
	}
	modified := 0
	for _, c := range changes {
		mi := c.MonsterIndex
		if mi < 0 || mi >= len(a.currentResults) {
			continue
		}
		file := a.currentResults[mi].File
		ei := c.EntryIndex
		if ei < 0 || ei >= len(file.Entries) {
			continue
		}
		entry := file.Entries[ei]
		if entry.ItemName != itemName {
			continue
		}
		a.editor.ModifyEntry(file, entry, c.NewNum, c.NewDen)
		modified++
	}
	return modified, nil
}

// CopyRatesToItems 将指定物品的爆率复制到其他物品
// 仅复制可编辑的普通掉落条目（跳过 #CALL/#CHILD 等）
func (a *App) CopyRatesToItems(sourceItem string, targetItems []string) (*CopyResult, error) {
	if a.currentResults == nil {
		return nil, fmt.Errorf("请先加载爆率文件")
	}
	if len(targetItems) == 0 {
		return nil, fmt.Errorf("请选择至少一个目标物品")
	}

	// 收集源物品在每个怪物中的爆率配置
	type sourceRate struct {
		num int
		den int
		qty int
	}
	monsterRates := make(map[string]sourceRate) // monsterName -> rate

	for _, r := range a.currentResults {
		for _, e := range r.File.Entries {
			if e.IsEditable() && e.ItemName == sourceItem {
				monsterRates[r.File.MonsterName] = sourceRate{
					num: e.ProbabilityNumerator,
					den: e.ProbabilityDenominator,
					qty: e.Quantity,
				}
				break
			}
		}
	}

	if len(monsterRates) == 0 {
		return nil, fmt.Errorf("未找到物品「%s」的掉落配置", sourceItem)
	}

	totalModified := 0
	for _, targetItem := range targetItems {
		for _, r := range a.currentResults {
			rate, hasRate := monsterRates[r.File.MonsterName]
			if !hasRate {
				continue
			}

			found := false
			for _, e := range r.File.Entries {
				if e.IsEditable() && e.ItemName == targetItem {
					a.editor.ModifyEntry(r.File, e, rate.num, rate.den)
					a.editor.ModifyQuantity(r.File, e, rate.qty)
					totalModified++
					found = true
					break
				}
			}

			// 如果该怪物没有此物品，新增一条
			if !found {
				a.editor.AddEntry(r.File, targetItem, rate.num, rate.den, rate.qty)
				totalModified++
			}
		}
	}

	return &CopyResult{
		Success:  true,
		Modified: totalModified,
		Message:  fmt.Sprintf("已将「%s」的爆率复制到 %d 个物品，共修改 %d 条", sourceItem, len(targetItems), totalModified),
	}, nil
}

// SaveAllModifiedFiles 保存所有已修改的爆率文件
func (a *App) SaveAllModifiedFiles() (int, error) {
	if a.currentResults == nil {
		return 0, fmt.Errorf("请先加载爆率文件")
	}
	count := 0
	for _, r := range a.currentResults {
		if a.cfg.AutoBackup && a.backupMgr != nil {
			a.backupMgr.BackupFile(r.File.FilePath)
		}
		a.editor.RebuildRawContent(r.File)
		if err := a.editor.SaveFile(r.File); err == nil {
			count++
		}
	}
	return count, nil
}

// findMonGenInfo 从 MonGen 中查找怪物的刷新信息
func findMonGenInfo(entries []*parser.MonGenEntry, monsterName string) (refreshSec float64, count int) {
	for _, e := range entries {
		if strings.EqualFold(e.MonsterName, monsterName) {
			return float64(e.RefreshMinutes) * 60, e.Count
		}
	}
	return 60, 10 // 默认值：60秒刷新，每次10只
}

// findMapForMonster 从 MonGen 中查找怪物所在的地图
func findMapForMonster(entries []*parser.MonGenEntry, monsterName string) string {
	for _, e := range entries {
		if strings.EqualFold(e.MonsterName, monsterName) {
			return e.MapName
		}
	}
	return "未知地图"
}

// GetMonstersOnMap 获取指定地图上的怪物列表（用于模拟结果的地图点击）
func (a *App) GetMonstersOnMap(mapName string) []string {
	if a.monGenEntries == nil {
		return nil
	}
	seen := make(map[string]bool)
	var names []string
	for _, e := range a.monGenEntries {
		m := strings.TrimSpace(e.MapName)
		if m == mapName && !seen[e.MonsterName] {
			seen[e.MonsterName] = true
			names = append(names, e.MonsterName)
		}
	}
	return names
}

// roundToNiceDenominator 将分母取整到"好看"的数字
func roundToNiceDenominator(raw float64) int {
	if raw <= 0 {
		return 1
	}
	// 优先取整到 10/100/500/1000/5000/10000 等
	niceValues := []int{1, 2, 5, 10, 20, 50, 100, 200, 500, 1000, 2000, 5000, 10000, 20000, 50000, 100000, 200000, 500000, 1000000}
	for _, n := range niceValues {
		if float64(n) >= raw*0.8 {
			return n
		}
	}
	// 超大值，取整到万
	return int(math.Ceil(raw/10000)) * 10000
}

// Placeholder 防止空 import
var _ = strconv.Itoa