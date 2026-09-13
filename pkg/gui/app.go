package gui

import (
	_ "embed"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/yueyoue/legend-drop-tool/pkg/auth"
	"github.com/yueyoue/legend-drop-tool/pkg/backup"
	"github.com/yueyoue/legend-drop-tool/pkg/config"
	"github.com/yueyoue/legend-drop-tool/pkg/editor"
	"github.com/yueyoue/legend-drop-tool/pkg/parser"
	"github.com/yueyoue/legend-drop-tool/pkg/simulator"
)

//go:embed icon.png
var iconData []byte

const appTitle = "传奇爆率模拟与修改工具 v1.0.6"

// App 主应用
type App struct {
	fyneApp    fyne.App
	mainWindow fyne.Window
	cfg        *config.AppConfig
	authMgr    *auth.LocalAuth
	editor     *editor.Editor
	simulator  *simulator.Simulator
	backupMgr  *backup.Manager

	currentResults []*parser.ParseResult
	currentEngine  parser.EngineType

	serverPathEntry   *widget.Entry
	engineSelect      *widget.Select
	fileList          *widget.List
	detailTable       *widget.List
	statusLabel       *widget.Label

	// 模拟页面控件
	simMonsterRadio   *widget.RadioGroup
	simItemRadio      *widget.RadioGroup
	simMapRadio       *widget.RadioGroup
	simDurationEntry  *widget.Entry
	simKillRatioEntry *widget.Entry
	simMonsterCount   *widget.Label
	simItemCount      *widget.Label
	simMapCount       *widget.Label
	simResultLabel    *widget.Label

	// 模拟结果 - Table控件
	simItemTable    *widget.Table
	simMapTable     *widget.Table
	simMonsterTable *widget.Table

	// 模拟结果数据
	simResult         *simulator.SimResult
	filteredMapStats  []*simulator.MapStat
	filteredMonStats  []*simulator.MonsterStat

	selectedFileIdx int
	monGenEntries   []*parser.MonGenEntry
	mapInfoLookup   map[string]string

	// 模拟筛选条件
	selectedMonsters []string
	selectedItems    []string
	selectedMaps     []string
}

// New 创建并运行应用
func New() {
	a := app.New()
	if len(iconData) > 0 {
		a.SetIcon(fyne.NewStaticResource("icon.png", iconData))
	}
	customTheme := NewCJKTheme()
	a.Settings().SetTheme(customTheme)

	w := a.NewWindow(appTitle)
	w.Resize(fyne.NewSize(1280, 860))
	if len(iconData) > 0 {
		w.SetIcon(fyne.NewStaticResource("icon.png", iconData))
	}

	cfg, _ := config.Load()
	gui := &App{
		fyneApp:         a,
		mainWindow:      w,
		cfg:             cfg,
		authMgr:         auth.NewLocalAuth(),
		editor:          editor.New(),
		simulator:       simulator.New(),
		selectedFileIdx: -1,
	}

	w.SetContent(gui.buildUI())
	w.ShowAndRun()
}

// buildUI 构建主界面
func (a *App) buildUI() fyne.CanvasObject {
	toolbar := a.buildToolbar()
	leftPanel := a.buildFileListPanel()
	rightPanel := a.buildDetailTabs()
	a.statusLabel = widget.NewLabel("就绪 - 请选择传奇服务端目录")
	statusBar := container.NewHBox(a.statusLabel)

	split := container.NewHSplit(leftPanel, rightPanel)
	split.SetOffset(0.22)
	return container.NewBorder(toolbar, statusBar, nil, nil, split)
}

// buildToolbar 构建顶部工具栏
func (a *App) buildToolbar() fyne.CanvasObject {
	a.serverPathEntry = widget.NewEntry()
	a.serverPathEntry.SetPlaceHolder("选择传奇服务端根目录 (如 D:\\MirServer)")
	a.serverPathEntry.SetText(a.cfg.ServerRoot)

	browseBtn := widget.NewButton("浏览...", a.onBrowseServer)
	loadBtn := widget.NewButton("加载爆率文件", a.onLoadFiles)
	autoDetectBtn := widget.NewButton("自动检测引擎", a.onAutoDetect)

	a.engineSelect = widget.NewSelect(
		[]string{"HERO", "GOM", "GEE", "BLUE", "自动检测"},
		a.onEngineChanged,
	)
	a.engineSelect.SetSelected("自动检测")

	return container.NewVBox(
		container.NewBorder(nil, nil, widget.NewLabel("服务端目录:"), browseBtn, a.serverPathEntry),
		container.NewBorder(nil, nil, widget.NewLabel("引擎类型:"), container.NewHBox(autoDetectBtn, loadBtn), a.engineSelect),
		widget.NewSeparator(),
	)
}

// buildFileListPanel 构建左侧文件列表
func (a *App) buildFileListPanel() fyne.CanvasObject {
	a.fileList = widget.NewList(
		func() int {
			if a.currentResults == nil {
				return 0
			}
			return len(a.currentResults)
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("模板文件名")
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if a.currentResults != nil && id < len(a.currentResults) {
				label := obj.(*widget.Label)
				r := a.currentResults[id]
				count := 0
				for _, e := range r.File.Entries {
					if e.IsEditable() {
						count++
					}
				}
				label.SetText(fmt.Sprintf("%s (%d条)", r.File.MonsterName, count))
			}
		},
	)
	a.fileList.OnSelected = func(id widget.ListItemID) {
		a.selectedFileIdx = id
		a.updateDetailPanel()
	}

	header := widget.NewLabel("怪物列表")
	header.TextStyle = fyne.TextStyle{Bold: true}
	return container.NewBorder(header, nil, nil, nil, a.fileList)
}

// buildDetailTabs 构建右侧详情标签页
func (a *App) buildDetailTabs() fyne.CanvasObject {
	editTab := a.buildEditTab()
	simTab := a.buildSimTab()
	logTab := a.buildLogTab()
	authTab := a.buildAuthTab()

	tabs := container.NewAppTabs(
		container.NewTabItem("爆率修改", editTab),
		container.NewTabItem("爆率模拟", simTab),
		container.NewTabItem("操作日志", logTab),
		container.NewTabItem("授权管理", authTab),
	)
	tabs.SetTabLocation(container.TabLocationTop)
	return tabs
}

// buildEditTab 构建爆率修改页
func (a *App) buildEditTab() fyne.CanvasObject {
	a.detailTable = widget.NewList(
		func() int {
			if a.selectedFileIdx < 0 || a.currentResults == nil || a.selectedFileIdx >= len(a.currentResults) {
				return 0
			}
			return len(a.currentResults[a.selectedFileIdx].File.Entries)
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("掉落条目模板文本内容很长很长很长")
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if a.selectedFileIdx < 0 || a.currentResults == nil {
				return
			}
			file := a.currentResults[a.selectedFileIdx].File
			if id >= len(file.Entries) {
				return
			}
			entry := file.Entries[id]
			label := obj.(*widget.Label)
			indent := strings.Repeat("  ", entry.Depth)

			switch {
			case entry.IsComment:
				label.SetText(fmt.Sprintf("%s📝 %s", indent, strings.TrimSpace(entry.RawLine)))
			case entry.IsCallRef:
				s := fmt.Sprintf("%s📎 #CALL [%s]", indent, entry.CallPath)
				if entry.CallLabel != "" {
					s += " " + entry.CallLabel
				}
				label.SetText(s)
			case entry.IsChildStart:
				s := fmt.Sprintf("%s📦 #CHILD %s", indent, entry.ChildProbability)
				if entry.ChildRandom {
					s += " RANDOM"
				}
				label.SetText(s)
			case entry.IsCaseStart:
				label.SetText(fmt.Sprintf("%s🔀 #CASE %s", indent, entry.CaseExpression))
			case entry.IsIfStart:
				label.SetText(fmt.Sprintf("%s🔀 #IF %s", indent, entry.CaseExpression))
			case entry.IsChildEnd:
				label.SetText(fmt.Sprintf("%s📦 )", indent))
			case entry.IsEditable():
				trigger := ""
				if entry.HasTrigger {
					trigger = fmt.Sprintf(" |%s", entry.TriggerName)
				}
				label.SetText(fmt.Sprintf("%s🎯 %s  %s%s  x%d",
					indent, entry.ProbabilityStr(), entry.ItemName, trigger, entry.Quantity))
			default:
				label.SetText(fmt.Sprintf("%s%s", indent, strings.TrimSpace(entry.RawLine)))
			}
		},
	)
	a.detailTable.OnSelected = func(id widget.ListItemID) {
		a.onEntrySelected(id)
	}

	addBtn := widget.NewButton("新增掉落", a.onAddEntry)
	mulBtn := widget.NewButton("批量倍率调整", a.onBatchMultiply)
	batchSetBtn := widget.NewButton("批量设置概率", a.onBatchSetProb)
	saveBtn := widget.NewButton("保存文件", a.onSaveFile)
	backupBtn := widget.NewButton("备份当前文件", a.onBackupCurrent)
	backupAllBtn := widget.NewButton("备份整个目录", a.onBackupAll)
	detectBtn := widget.NewButton("爆率异常检测", a.onDetectAnomaly)

	btnBar := container.NewHBox(addBtn, mulBtn, batchSetBtn, detectBtn, backupBtn, backupAllBtn, layout.NewSpacer(), saveBtn)
	return container.NewBorder(nil, btnBar, nil, nil, a.detailTable)
}

// buildSimTab 构建爆率模拟页
func (a *App) buildSimTab() fyne.CanvasObject {
	// === 怪物选择区 ===
	a.simMonsterRadio = widget.NewRadioGroup([]string{"所有怪物", "指定怪物"}, nil)
	a.simMonsterRadio.SetSelected("所有怪物")
	a.simMonsterRadio.Horizontal = true
	a.simMonsterCount = widget.NewLabel("(0)")
	monsterSetBtn := widget.NewButton("指定怪物设置", func() { a.showMonsterPicker() })
	monsterSection := container.NewVBox(
		widget.NewLabelWithStyle("怪物", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewHBox(a.simMonsterRadio, a.simMonsterCount, monsterSetBtn),
	)

	// === 物品选择区 ===
	a.simItemRadio = widget.NewRadioGroup([]string{"所有物品", "指定物品"}, nil)
	a.simItemRadio.SetSelected("所有物品")
	a.simItemRadio.Horizontal = true
	a.simItemCount = widget.NewLabel("(0)")
	itemSetBtn := widget.NewButton("指定物品设置", func() { a.showItemPicker() })
	itemSection := container.NewVBox(
		widget.NewLabelWithStyle("物品", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewHBox(a.simItemRadio, a.simItemCount, itemSetBtn),
	)

	// === 地图选择区 ===
	a.simMapRadio = widget.NewRadioGroup([]string{"所有地图", "指定地图"}, nil)
	a.simMapRadio.SetSelected("所有地图")
	a.simMapRadio.Horizontal = true
	a.simMapCount = widget.NewLabel("(0)")
	mapSetBtn := widget.NewButton("指定地图设置", func() { a.showMapPicker() })
	mapSection := container.NewVBox(
		widget.NewLabelWithStyle("指定地图", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewHBox(a.simMapRadio, a.simMapCount, mapSetBtn),
	)

	// === 其它选项 ===
	a.simDurationEntry = widget.NewEntry()
	a.simDurationEntry.SetText("24")
	a.simKillRatioEntry = widget.NewEntry()
	a.simKillRatioEntry.SetText("50")

	otherSection := container.NewVBox(
		widget.NewLabelWithStyle("其它选项", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewGridWithColumns(2,
			widget.NewLabel("模拟运行时间(小时):"), a.simDurationEntry,
			widget.NewLabel("消灭怪物比例(%):"), a.simKillRatioEntry,
		),
	)

	simBtn := widget.NewButton("模拟", a.onRunSimNew)
	simBtn.Importance = widget.HighImportance

	optionArea := container.NewGridWithColumns(4,
		monsterSection, itemSection, mapSection,
		container.NewVBox(otherSection, simBtn),
	)

	// === 掉落物品列表 (Table) ===
	a.simItemTable = widget.NewTable(
		func() (int, int) {
			if a.simResult == nil {
				return 0, 2
			}
			return len(a.simResult.ItemStats) + 1, 2 // +1 for header
		},
		func() fyne.CanvasObject {
			label := widget.NewLabel("")
			label.Alignment = fyne.TextAlignTrailing
			return label
		},
		func(id widget.TableCellID, obj fyne.CanvasObject) {
			label := obj.(*widget.Label)
			if id.Row == 0 {
				// 表头
				label.TextStyle = fyne.TextStyle{Bold: true}
				label.Alignment = fyne.TextAlignCenter
				if id.Col == 0 {
					label.SetText("物品名称")
				} else {
					label.SetText("掉落数量")
				}
				return
			}
			label.TextStyle = fyne.TextStyle{}
			if a.simResult == nil || id.Row-1 >= len(a.simResult.ItemStats) {
				label.SetText("")
				return
			}
			item := a.simResult.ItemStats[id.Row-1]
			if id.Col == 0 {
				label.Alignment = fyne.TextAlignLeading
				label.SetText(item.ItemName)
			} else {
				label.Alignment = fyne.TextAlignTrailing
				label.SetText(formatNumber(item.DropCount))
			}
		},
	)
	a.simItemTable.SetColumnWidth(0, 200)
	a.simItemTable.SetColumnWidth(1, 120)
	a.simItemTable.OnSelected = func(id widget.TableCellID) {
		if id.Row > 0 {
			a.onItemSelected(id.Row - 1)
		}
	}

	// === 掉落地图列表 (Table) ===
	a.simMapTable = widget.NewTable(
		func() (int, int) {
			if a.filteredMapStats == nil {
				if a.simResult == nil {
					return 0, 2
				}
				return len(a.simResult.MapStats) + 1, 2
			}
			return len(a.filteredMapStats) + 1, 2
		},
		func() fyne.CanvasObject {
			label := widget.NewLabel("")
			label.Alignment = fyne.TextAlignTrailing
			return label
		},
		func(id widget.TableCellID, obj fyne.CanvasObject) {
			label := obj.(*widget.Label)
			if id.Row == 0 {
				label.TextStyle = fyne.TextStyle{Bold: true}
				label.Alignment = fyne.TextAlignCenter
				if id.Col == 0 {
					label.SetText("地图名称")
				} else {
					label.SetText("掉落数量")
				}
				return
			}
			label.TextStyle = fyne.TextStyle{}
			data := a.getMapData()
			if id.Row-1 >= len(data) {
				label.SetText("")
				return
			}
			m := data[id.Row-1]
			if id.Col == 0 {
				label.Alignment = fyne.TextAlignLeading
				label.SetText(m.MapName)
			} else {
				label.Alignment = fyne.TextAlignTrailing
				label.SetText(formatNumber(m.DropCount))
			}
		},
	)
	a.simMapTable.SetColumnWidth(0, 200)
	a.simMapTable.SetColumnWidth(1, 120)
	a.simMapTable.OnSelected = func(id widget.TableCellID) {
		if id.Row > 0 {
			a.onMapSelected(id.Row - 1)
		}
	}

	// === 掉落怪物列表 (Table) ===
	a.simMonsterTable = widget.NewTable(
		func() (int, int) {
			if a.filteredMonStats == nil {
				if a.simResult == nil {
					return 0, 3
				}
				return len(a.simResult.MonsterStats) + 1, 3
			}
			return len(a.filteredMonStats) + 1, 3
		},
		func() fyne.CanvasObject {
			label := widget.NewLabel("")
			label.Alignment = fyne.TextAlignTrailing
			return label
		},
		func(id widget.TableCellID, obj fyne.CanvasObject) {
			label := obj.(*widget.Label)
			if id.Row == 0 {
				label.TextStyle = fyne.TextStyle{Bold: true}
				label.Alignment = fyne.TextAlignCenter
				switch id.Col {
				case 0:
					label.SetText("怪物名称")
				case 1:
					label.SetText("刷怪数量")
				case 2:
					label.SetText("掉落数量")
				}
				return
			}
			label.TextStyle = fyne.TextStyle{}
			data := a.getMonData()
			if id.Row-1 >= len(data) {
				label.SetText("")
				return
			}
			ms := data[id.Row-1]
			switch id.Col {
			case 0:
				label.Alignment = fyne.TextAlignLeading
				label.SetText(ms.MonsterName)
			case 1:
				label.Alignment = fyne.TextAlignTrailing
				label.SetText(formatNumber(ms.KillCount))
			case 2:
				label.Alignment = fyne.TextAlignTrailing
				label.SetText(formatNumber(ms.DropCount))
			}
		},
	)
	a.simMonsterTable.SetColumnWidth(0, 160)
	a.simMonsterTable.SetColumnWidth(1, 100)
	a.simMonsterTable.SetColumnWidth(2, 100)

	// 统计摘要
	a.simResultLabel = widget.NewLabel("")
	a.simResultLabel.Wrapping = fyne.TextWrapWord
	exportBtn := widget.NewButton("导出结果", a.onExportSimResult)
	summaryBar := container.NewBorder(nil, nil, nil, exportBtn, a.simResultLabel)

	// 三列结果
	resultGrid := container.NewGridWithColumns(3,
		container.NewBorder(
			widget.NewLabelWithStyle("掉落物品列表", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
			nil, nil, nil, a.simItemTable),
		container.NewBorder(
			widget.NewLabelWithStyle("掉落地图列表", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
			nil, nil, nil, a.simMapTable),
		container.NewBorder(
			widget.NewLabelWithStyle("掉落怪物列表", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
			nil, nil, nil, a.simMonsterTable),
	)

	return container.NewBorder(
		container.NewVBox(optionArea, widget.NewSeparator()),
		container.NewVBox(widget.NewSeparator(), summaryBar),
		nil, nil, resultGrid,
	)
}

// formatNumber 格式化数字（加千位分隔符）
func formatNumber(n int64) string {
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return s
	}
	var result []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}

// getMapData 获取当前显示的地图数据
func (a *App) getMapData() []*simulator.MapStat {
	if a.filteredMapStats != nil {
		return a.filteredMapStats
	}
	if a.simResult != nil {
		return a.simResult.MapStats
	}
	return nil
}

// getMonData 获取当前显示的怪物数据
func (a *App) getMonData() []*simulator.MonsterStat {
	if a.filteredMonStats != nil {
		return a.filteredMonStats
	}
	if a.simResult != nil {
		return a.simResult.MonsterStats
	}
	return nil
}

// onItemSelected 点击物品列表联动：筛选地图和怪物
func (a *App) onItemSelected(row int) {
	if a.simResult == nil || row >= len(a.simResult.ItemStats) {
		return
	}
	selectedItem := a.simResult.ItemStats[row].ItemName

	// 筛选掉落该物品的怪物
	a.filteredMonStats = nil
	if a.currentResults != nil {
		monsterSet := make(map[string]bool)
		for _, r := range a.currentResults {
			for _, entry := range r.File.Entries {
				if entry.IsEditable() && entry.ItemName == selectedItem {
					monsterSet[r.File.MonsterName] = true
					break
				}
			}
		}
		for _, ms := range a.simResult.MonsterStats {
			if monsterSet[ms.MonsterName] {
				a.filteredMonStats = append(a.filteredMonStats, ms)
			}
		}
	}

	// 筛选掉落该物品的地图（通过怪物→地图关联）
	a.filteredMapStats = nil
	mapSet := make(map[string]bool)
	for _, ms := range a.filteredMonStats {
		if a.monGenEntries != nil {
			for _, mg := range a.monGenEntries {
				if mg.MonsterName == ms.MonsterName {
					mapSet[mg.MapName] = true
				}
			}
		}
	}
	if a.simResult != nil {
		for _, m := range a.simResult.MapStats {
			if mapSet[m.MapName] || len(mapSet) == 0 {
				a.filteredMapStats = append(a.filteredMapStats, m)
			}
		}
	}

	// 刷新列表
	a.simMapTable.Refresh()
	a.simMonsterTable.Refresh()
	a.statusLabel.SetText(fmt.Sprintf("已选中物品: %s | 地图:%d个 怪物:%d个", selectedItem, len(a.filteredMapStats), len(a.filteredMonStats)))
}

// onMapSelected 点击地图列表联动：筛选怪物
func (a *App) onMapSelected(row int) {
	data := a.getMapData()
	if data == nil || row >= len(data) {
		return
	}
	selectedMap := data[row].MapName

	// 筛选该地图的怪物
	a.filteredMonStats = nil
	if a.monGenEntries != nil {
		monsterSet := make(map[string]bool)
		for _, mg := range a.monGenEntries {
			if mg.MapName == selectedMap {
				monsterSet[mg.MonsterName] = true
			}
		}
		if a.simResult != nil {
			for _, ms := range a.simResult.MonsterStats {
				if monsterSet[ms.MonsterName] {
					a.filteredMonStats = append(a.filteredMonStats, ms)
				}
			}
		}
	}

	a.simMonsterTable.Refresh()
	a.statusLabel.SetText(fmt.Sprintf("已选中地图: %s | 怪物:%d个", selectedMap, len(a.filteredMonStats)))
}

// buildAuthTab 构建授权管理页
func (a *App) buildAuthTab() fyne.CanvasObject {
	machineID, _ := a.authMgr.GetMachineID()
	info, _ := a.authMgr.CheckLicense()

	machineLabel := widget.NewLabel(fmt.Sprintf("本机机器码: %s", machineID))
	machineLabel.Wrapping = fyne.TextWrapWord

	copyBtn := widget.NewButton("复制机器码", func() {
		a.mainWindow.Clipboard().SetContent(machineID)
		dialog.ShowInformation("已复制", "机器码已复制到剪贴板", a.mainWindow)
	})

	statusText := "未激活"
	if info.IsActive {
		statusText = fmt.Sprintf("已激活 (%s)", info.Type)
	}
	statusLabel := widget.NewLabel(fmt.Sprintf("授权状态: %s", statusText))

	activateEntry := widget.NewEntry()
	activateEntry.SetPlaceHolder("输入激活码")

	activateBtn := widget.NewButton("激活", func() {
		code := strings.TrimSpace(activateEntry.Text)
		if code == "" {
			dialog.ShowError(fmt.Errorf("请输入激活码"), a.mainWindow)
			return
		}
		info, err := a.authMgr.Activate(code)
		if err != nil {
			dialog.ShowError(err, a.mainWindow)
			return
		}
		statusLabel.SetText(fmt.Sprintf("授权状态: 已激活 (%s)", info.Type))
		dialog.ShowInformation("激活成功", "授权已激活，可以使用全部功能", a.mainWindow)
	})

	helpText := widget.NewLabel("授权说明:\n" +
		"1. 复制本机机器码\n" +
		"2. 联系管理员获取激活码\n" +
		"3. 输入激活码完成绑定\n" +
		"4. 一机一码，绑定后不可随意更换设备\n\n" +
		"当前为开发模式，所有功能可用")
	helpText.Wrapping = fyne.TextWrapWord

	return container.NewVBox(
		widget.NewLabel("授权管理"),
		widget.NewSeparator(),
		machineLabel, copyBtn,
		widget.NewSeparator(),
		statusLabel,
		container.NewBorder(nil, nil, nil, activateBtn, activateEntry),
		widget.NewSeparator(),
		helpText,
	)
}

// buildLogTab 构建操作日志页
func (a *App) buildLogTab() fyne.CanvasObject {
	logLabel := widget.NewLabel("操作记录:\n")
	logLabel.Wrapping = fyne.TextWrapWord

	refreshBtn := widget.NewButton("刷新日志", func() {
		var sb strings.Builder
		sb.WriteString("操作记录:\n")
		for _, r := range a.editor.Records() {
			sb.WriteString(fmt.Sprintf("[%s] %s | %s | %s -> %s\n",
				r.Timestamp.Format("15:04:05"),
				r.Action, filepath.Base(r.FilePath),
				r.OldValue, r.NewValue))
		}
		if len(a.editor.Records()) == 0 {
			sb.WriteString("暂无操作记录")
		}
		logLabel.SetText(sb.String())
	})

	scroll := container.NewVScroll(logLabel)
	return container.NewBorder(
		container.NewHBox(widget.NewLabel("操作日志"), layout.NewSpacer(), refreshBtn),
		nil, nil, nil, scroll,
	)
}

// ============ 选择器 ============

func (a *App) showMonsterPicker() {
	if a.currentResults == nil {
		dialog.ShowInformation("提示", "请先加载爆率文件", a.mainWindow)
		return
	}
	var names []string
	for _, r := range a.currentResults {
		names = append(names, r.File.MonsterName)
	}
	a.showMultiPicker("选择怪物", names, func(selected []string) {
		a.selectedMonsters = selected
		a.simMonsterCount.SetText(fmt.Sprintf("(%d)", len(selected)))
	})
}

func (a *App) showItemPicker() {
	if a.currentResults == nil {
		dialog.ShowInformation("提示", "请先加载爆率文件", a.mainWindow)
		return
	}
	itemSet := make(map[string]bool)
	for _, r := range a.currentResults {
		for _, e := range r.File.Entries {
			if e.IsEditable() {
				itemSet[e.ItemName] = true
			}
		}
	}
	var names []string
	for name := range itemSet {
		names = append(names, name)
	}
	sort.Strings(names)
	a.showMultiPicker("选择物品", names, func(selected []string) {
		a.selectedItems = selected
		a.simItemCount.SetText(fmt.Sprintf("(%d)", len(selected)))
	})
}

func (a *App) showMapPicker() {
	if a.monGenEntries == nil {
		dialog.ShowInformation("提示", "未加载MonGen.txt，无法获取地图信息", a.mainWindow)
		return
	}
	mapSet := make(map[string]bool)
	for _, mg := range a.monGenEntries {
		mapSet[mg.MapName] = true
	}
	var names []string
	for mapID := range mapSet {
		names = append(names, mapID)
	}
	sort.Strings(names)
	a.showMultiPicker("选择地图", names, func(selected []string) {
		a.selectedMaps = selected
		a.simMapCount.SetText(fmt.Sprintf("(%d)", len(selected)))
	})
}

func (a *App) showMultiPicker(title string, items []string, onConfirm func([]string)) {
	selected := make(map[string]bool)
	filtered := make([]string, len(items))
	copy(filtered, items)

	searchEntry := widget.NewEntry()
	searchEntry.SetPlaceHolder("输入关键词搜索...")

	list := widget.NewList(
		func() int { return len(filtered) },
		func() fyne.CanvasObject {
			return widget.NewCheck("选项文本", nil)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id >= len(filtered) {
				return
			}
			check := obj.(*widget.Check)
			check.Text = filtered[id]
			check.Checked = selected[filtered[id]]
			check.OnChanged = func(val bool) {
				selected[filtered[id]] = val
			}
			check.Refresh()
		},
	)

	searchEntry.OnChanged = func(query string) {
		query = strings.ToLower(strings.TrimSpace(query))
		if query == "" {
			filtered = make([]string, len(items))
			copy(filtered, items)
		} else {
			filtered = nil
			for _, item := range items {
				if strings.Contains(strings.ToLower(item), query) {
					filtered = append(filtered, item)
				}
			}
		}
		list.Refresh()
	}

	selectAllBtn := widget.NewButton("全选", func() {
		for _, item := range filtered {
			selected[item] = true
		}
		list.Refresh()
	})
	clearAllBtn := widget.NewButton("全不选", func() {
		for _, item := range filtered {
			selected[item] = false
		}
		list.Refresh()
	})

	scroll := container.NewVScroll(list)
	scroll.SetMinSize(fyne.NewSize(500, 450))

	content := container.NewBorder(
		container.NewVBox(searchEntry, container.NewHBox(selectAllBtn, clearAllBtn)),
		nil, nil, nil, scroll,
	)

	dialog.ShowCustomConfirm(title, "确定", "取消", content, func(ok bool) {
		if !ok {
			return
		}
		var result []string
		for name, sel := range selected {
			if sel {
				result = append(result, name)
			}
		}
		onConfirm(result)
	}, a.mainWindow)
}

// ============ 模拟执行 ============

func (a *App) onRunSimNew() {
	if a.currentResults == nil {
		dialog.ShowInformation("提示", "请先加载爆率文件", a.mainWindow)
		return
	}

	duration, err1 := strconv.ParseFloat(a.simDurationEntry.Text, 64)
	killRatioPct, err2 := strconv.ParseFloat(a.simKillRatioEntry.Text, 64)
	if err1 != nil || err2 != nil {
		dialog.ShowError(fmt.Errorf("请输入有效的模拟参数"), a.mainWindow)
		return
	}
	killRatio := killRatioPct / 100.0
	if killRatio < 0 || killRatio > 1 {
		dialog.ShowError(fmt.Errorf("消灭比例必须在0~100之间"), a.mainWindow)
		return
	}

	cfg := simulator.SimConfig{
		DurationHours:   duration,
		KillRatio:       killRatio,
		RefreshInterval: 60,
		RefreshCount:    10,
		MapRateModifier: 1.0,
	}

	if a.simMonsterRadio.Selected == "指定怪物" && len(a.selectedMonsters) > 0 {
		cfg.MonsterFilter = a.selectedMonsters
	}
	if a.simItemRadio.Selected == "指定物品" && len(a.selectedItems) > 0 {
		cfg.ItemFilter = a.selectedItems
	}
	if a.simMapRadio.Selected == "指定地图" && len(a.selectedMaps) > 0 {
		cfg.MapFilter = a.selectedMaps
	}

	// 清除筛选
	a.filteredMapStats = nil
	a.filteredMonStats = nil

	a.statusLabel.SetText("正在模拟...请稍候")

	go func() {
		var files []*parser.MonsterDropFile
		for _, r := range a.currentResults {
			files = append(files, r.File)
		}

		result := a.simulator.SimulateAll(files, a.monGenEntries, cfg)
		a.simResult = result

		a.simItemTable.Refresh()
		a.simMapTable.Refresh()
		a.simMonsterTable.Refresh()

		summary := fmt.Sprintf("总击杀:%d 总掉落:%d 空爆率:%.1f%% 耗时:%v",
			result.TotalKills, result.TotalDrops, result.EmptyRate()*100, result.Duration)
		a.simResultLabel.SetText(summary)
		a.statusLabel.SetText("模拟完成 - " + summary)
	}()
}

// ============ 事件处理 ============

func (a *App) onBrowseServer() {
	dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
		if err != nil || uri == nil {
			return
		}
		path := uri.Path()
		a.serverPathEntry.SetText(path)
		a.cfg.ServerRoot = path
	}, a.mainWindow)
}

func (a *App) onAutoDetect() {
	path := a.serverPathEntry.Text
	if path == "" {
		dialog.ShowError(fmt.Errorf("请先选择服务端目录"), a.mainWindow)
		return
	}
	engine := parser.DetectEngine(path)
	a.currentEngine = engine
	a.engineSelect.SetSelected(engine.String())
	a.statusLabel.SetText(fmt.Sprintf("检测到引擎: %s", engine))
}

func (a *App) onEngineChanged(value string) {
	switch value {
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

func (a *App) onLoadFiles() {
	serverRoot := a.serverPathEntry.Text
	if serverRoot == "" {
		dialog.ShowError(fmt.Errorf("请选择服务端目录"), a.mainWindow)
		return
	}

	monItemsDir := filepath.Join(serverRoot, "Mir200", "Envir", "MonItems")

	results, err := parser.ParseDirectory(monItemsDir, a.currentEngine)
	if err != nil {
		dialog.ShowError(fmt.Errorf("加载爆率文件失败: %v\n路径: %s", err, monItemsDir), a.mainWindow)
		return
	}
	if len(results) == 0 {
		dialog.ShowInformation("提示", "未找到爆率文件(.txt)", a.mainWindow)
		return
	}

	a.currentResults = results
	a.backupMgr = backup.New(serverRoot)
	a.cfg.MonItemsDir = monItemsDir
	a.cfg.ServerRoot = serverRoot
	config.Save(a.cfg)

	monGenPath := filepath.Join(serverRoot, "Mir200", "Envir", "MonGen.txt")
	if monGenEntries, err := parser.ParseMonGen(monGenPath); err == nil && len(monGenEntries) > 0 {
		a.monGenEntries = monGenEntries
	}

	mapInfoPath := filepath.Join(serverRoot, "Mir200", "Envir", "MapInfo.txt")
	if mapInfo, err := parser.ParseMapInfo(mapInfoPath); err == nil {
		a.mapInfoLookup = mapInfo
	}

	totalEntries := 0
	for _, r := range results {
		for _, e := range r.File.Entries {
			if e.IsEditable() {
				totalEntries++
			}
		}
	}

	a.fileList.Refresh()
	a.statusLabel.SetText(fmt.Sprintf("已加载 %d 个怪物文件，共 %d 条掉落配置 | 引擎: %s",
		len(results), totalEntries, a.currentEngine))

	var warnings []string
	for _, r := range results {
		for _, w := range r.Warnings {
			warnings = append(warnings, fmt.Sprintf("[%s] %s", r.File.MonsterName, w))
		}
	}
	if len(warnings) > 0 {
		warningText := fmt.Sprintf("有 %d 条无法识别的行：\n\n", len(warnings))
		maxShow := 100
		if len(warnings) < maxShow {
			maxShow = len(warnings)
		}
		for i := 0; i < maxShow; i++ {
			warningText += warnings[i] + "\n"
		}
		if len(warnings) > 100 {
			warningText += fmt.Sprintf("\n... 还有 %d 条", len(warnings)-100)
		}
		warnLabel := widget.NewLabel(warningText)
		warnLabel.Wrapping = fyne.TextWrapWord
		scroll := container.NewVScroll(warnLabel)
		scroll.SetMinSize(fyne.NewSize(600, 400))
		dialog.ShowCustom("解析提示", "确定", scroll, a.mainWindow)
	}
}

func (a *App) onEntrySelected(id widget.ListItemID) {
	if a.selectedFileIdx < 0 || a.currentResults == nil {
		return
	}
	file := a.currentResults[a.selectedFileIdx].File
	if id >= len(file.Entries) {
		return
	}
	entry := file.Entries[id]
	if !entry.IsEditable() {
		return
	}

	numEntry := widget.NewEntry()
	numEntry.SetText(strconv.Itoa(entry.ProbabilityNumerator))
	denEntry := widget.NewEntry()
	denEntry.SetText(strconv.Itoa(entry.ProbabilityDenominator))
	qtyEntry := widget.NewEntry()
	qtyEntry.SetText(strconv.Itoa(entry.Quantity))

	form := widget.NewForm(
		widget.NewFormItem("物品名称", widget.NewLabel(entry.ItemName)),
		widget.NewFormItem("概率分子", numEntry),
		widget.NewFormItem("概率分母", denEntry),
		widget.NewFormItem("掉落数量", qtyEntry),
	)

	dialog.ShowCustomConfirm("修改掉落配置", "保存", "取消", form, func(ok bool) {
		if !ok {
			return
		}
		num, err1 := strconv.Atoi(numEntry.Text)
		den, err2 := strconv.Atoi(denEntry.Text)
		qty, err3 := strconv.Atoi(qtyEntry.Text)
		if err1 != nil || err2 != nil || err3 != nil || den <= 0 || qty <= 0 {
			dialog.ShowError(fmt.Errorf("请输入有效的正整数"), a.mainWindow)
			return
		}
		a.editor.ModifyEntry(file, entry, num, den)
		a.editor.ModifyQuantity(file, entry, qty)
		a.detailTable.Refresh()
		a.statusLabel.SetText(fmt.Sprintf("已修改 %s 的掉落配置: %s", file.MonsterName, entry.ProbabilityStr()))
	}, a.mainWindow)
}

func (a *App) onAddEntry() {
	if a.selectedFileIdx < 0 || a.currentResults == nil {
		dialog.ShowInformation("提示", "请先选择一个怪物文件", a.mainWindow)
		return
	}

	itemEntry := widget.NewEntry()
	itemEntry.SetPlaceHolder("物品名称")
	numEntry := widget.NewEntry()
	numEntry.SetText("1")
	denEntry := widget.NewEntry()
	denEntry.SetText("100")
	qtyEntry := widget.NewEntry()
	qtyEntry.SetText("1")

	form := widget.NewForm(
		widget.NewFormItem("物品名称", itemEntry),
		widget.NewFormItem("概率分子", numEntry),
		widget.NewFormItem("概率分母", denEntry),
		widget.NewFormItem("掉落数量", qtyEntry),
	)

	dialog.ShowCustomConfirm("新增掉落配置", "添加", "取消", form, func(ok bool) {
		if !ok {
			return
		}
		num, _ := strconv.Atoi(numEntry.Text)
		den, _ := strconv.Atoi(denEntry.Text)
		qty, _ := strconv.Atoi(qtyEntry.Text)
		if den <= 0 || qty <= 0 || itemEntry.Text == "" {
			dialog.ShowError(fmt.Errorf("请填写完整信息"), a.mainWindow)
			return
		}
		file := a.currentResults[a.selectedFileIdx].File
		a.editor.AddEntry(file, itemEntry.Text, num, den, qty)
		a.detailTable.Refresh()
		a.statusLabel.SetText(fmt.Sprintf("已新增掉落: %s", itemEntry.Text))
	}, a.mainWindow)
}

func (a *App) onBatchMultiply() {
	if a.selectedFileIdx < 0 || a.currentResults == nil {
		dialog.ShowInformation("提示", "请先选择一个怪物文件", a.mainWindow)
		return
	}

	mulEntry := widget.NewEntry()
	mulEntry.SetText("2.0")

	form := widget.NewForm(widget.NewFormItem("倍率", mulEntry))
	dialog.ShowCustomConfirm("批量倍率调整", "执行", "取消", form, func(ok bool) {
		if !ok {
			return
		}
		mul, err := strconv.ParseFloat(mulEntry.Text, 64)
		if err != nil || mul <= 0 {
			dialog.ShowError(fmt.Errorf("请输入有效的正数倍率"), a.mainWindow)
			return
		}
		file := a.currentResults[a.selectedFileIdx].File
		count := a.editor.BatchMultiply(file, mul)
		a.detailTable.Refresh()
		a.statusLabel.SetText(fmt.Sprintf("已批量调整 %d 条掉落配置，倍率: %.2f", count, mul))
	}, a.mainWindow)
}

func (a *App) onBatchSetProb() {
	if a.selectedFileIdx < 0 || a.currentResults == nil {
		dialog.ShowInformation("提示", "请先选择一个怪物文件", a.mainWindow)
		return
	}

	numEntry := widget.NewEntry()
	numEntry.SetText("1")
	denEntry := widget.NewEntry()
	denEntry.SetText("100")

	form := widget.NewForm(
		widget.NewFormItem("概率分子", numEntry),
		widget.NewFormItem("概率分母", denEntry),
	)
	dialog.ShowCustomConfirm("批量设置概率", "执行", "取消", form, func(ok bool) {
		if !ok {
			return
		}
		num, _ := strconv.Atoi(numEntry.Text)
		den, _ := strconv.Atoi(denEntry.Text)
		if den <= 0 {
			dialog.ShowError(fmt.Errorf("概率分母必须大于0"), a.mainWindow)
			return
		}
		file := a.currentResults[a.selectedFileIdx].File
		count := a.editor.BatchSetAll(file, num, den)
		a.detailTable.Refresh()
		a.statusLabel.SetText(fmt.Sprintf("已批量设置 %d 条掉落配置为 %d/%d", count, num, den))
	}, a.mainWindow)
}

func (a *App) onSaveFile() {
	if a.selectedFileIdx < 0 || a.currentResults == nil {
		return
	}
	file := a.currentResults[a.selectedFileIdx].File

	if a.cfg.AutoBackup && a.backupMgr != nil {
		backupPath, err := a.backupMgr.BackupFile(file.FilePath)
		if err == nil {
			a.statusLabel.SetText(fmt.Sprintf("已备份至: %s", backupPath))
		}
	}

	a.editor.RebuildRawContent(file)
	if err := a.editor.SaveFile(file); err != nil {
		dialog.ShowError(err, a.mainWindow)
		return
	}
	a.statusLabel.SetText(fmt.Sprintf("已保存: %s", file.FilePath))
	dialog.ShowInformation("保存成功", fmt.Sprintf("文件已保存: %s", file.MonsterName+".txt"), a.mainWindow)
}

func (a *App) onBackupCurrent() {
	if a.selectedFileIdx < 0 || a.currentResults == nil || a.backupMgr == nil {
		return
	}
	file := a.currentResults[a.selectedFileIdx].File
	path, err := a.backupMgr.BackupFile(file.FilePath)
	if err != nil {
		dialog.ShowError(err, a.mainWindow)
		return
	}
	a.statusLabel.SetText(fmt.Sprintf("已备份: %s", path))
	dialog.ShowInformation("备份成功", path, a.mainWindow)
}

func (a *App) onBackupAll() {
	if a.backupMgr == nil {
		dialog.ShowInformation("提示", "请先加载服务端目录", a.mainWindow)
		return
	}
	dir := filepath.Join(a.cfg.ServerRoot, "Mir200", "Envir", "MonItems")
	path, err := a.backupMgr.BackupDirectory(dir)
	if err != nil {
		dialog.ShowError(err, a.mainWindow)
		return
	}
	a.statusLabel.SetText(fmt.Sprintf("已备份整个目录至: %s", path))
	dialog.ShowInformation("备份成功", path, a.mainWindow)
}

func (a *App) onDetectAnomaly() {
	if a.currentResults == nil {
		dialog.ShowInformation("提示", "请先加载爆率文件", a.mainWindow)
		return
	}

	var anomalies []string
	for _, r := range a.currentResults {
		name := r.File.MonsterName
		hasDrop := false
		for _, e := range r.File.Entries {
			if !e.IsEditable() {
				continue
			}
			hasDrop = true
			if e.ProbabilityDenominator <= 1 && e.ProbabilityNumerator >= 1 {
				anomalies = append(anomalies, fmt.Sprintf("[%s] %s 必掉(%s) - 可能是测试配置",
					name, e.ItemName, e.ProbabilityStr()))
			}
			if e.ProbabilityDenominator > 100000 {
				anomalies = append(anomalies, fmt.Sprintf("[%s] %s 极低爆率(%s) - 可能配置错误",
					name, e.ItemName, e.ProbabilityStr()))
			}
		}
		if !hasDrop && len(r.File.Entries) > 0 {
			anomalies = append(anomalies, fmt.Sprintf("[%s] 无有效掉落配置", name))
		}
	}

	if len(anomalies) == 0 {
		dialog.ShowInformation("检测结果", "未发现异常配置", a.mainWindow)
	} else {
		text := strings.Join(anomalies, "\n")
		dialog.ShowInformation("异常检测结果", fmt.Sprintf("发现 %d 个异常:\n%s", len(anomalies), text), a.mainWindow)
	}
}

func (a *App) onExportSimResult() {
	if a.simResult == nil {
		dialog.ShowInformation("提示", "请先运行模拟", a.mainWindow)
		return
	}
	text := simulator.FormatResult(a.simResult)
	dialog.ShowFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil || writer == nil {
			return
		}
		defer writer.Close()
		writer.Write([]byte(text))
		dialog.ShowInformation("导出成功", "模拟结果已保存", a.mainWindow)
	}, a.mainWindow)
}

func (a *App) updateDetailPanel() {
	if a.detailTable != nil {
		a.detailTable.Refresh()
	}
}
