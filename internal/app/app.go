package app

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

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

// ============================================================
// 文件对话框（Wails 前端调用）
// ============================================================

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

// Placeholder 防止空 import
var _ = strconv.Itoa