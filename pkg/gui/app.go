package gui

import (
	_ "embed"
	"fmt"
	"image/color"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
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

const appTitle = "传奇爆率模拟与修改工具 v1.0.9"

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
	simPityCheck      *widget.Check
	simPityEntry      *widget.Entry
	simRunCountEntry  *widget.Entry
	simMonsterCount   *widget.Label
	simItemCount      *widget.Label
	simMapCount       *widget.Label
	simResultLabel    *widget.Label
	simTrackerLabel   *widget.Label // 稀有物品追踪

	// 模拟结果 - Table控件
	simItemTable    *widget.Table
	simMapTable     *widget.Table
	simMonsterTable *widget.Table

	// 模拟结果 - 搜索控件
	simItemSearch    *widget.Entry
	simMapSearch     *widget.Entry
	simMonsterSearch *widget.Entry

	// 搜索过滤后的数据
	filteredItemStats  []*simulator.ItemStat

	// 模拟结果数据
	simMultiResult    *simulator.MultiSimResult
	simResult         *simulator.SimResult // 当前显示的单次结果
	filteredMapStats  []*simulator.MapStat
	filteredMonStats  []*simulator.MonsterStat

	// 固定容量数组（防止Table因数据长度变化重置滚动位置）
	mapDisplayData    []*simulator.MapStat
	monDisplayData    []*simulator.MonsterStat
	mapDisplayCount   int
	monDisplayCount   int

	selectedFileIdx int
	monGenEntries   []*parser.MonGenEntry
	mapInfoLookup   map[string]string

	// 模拟筛选条件
	selectedMonsters []string
	selectedItems    []string
	selectedMaps     []string

	// 当前选中的物品名称（用于地图→怪物的级联筛选）
	selectedItemName string
	pitySection      fyne.CanvasObject // 保底设置面板
	selectedMonsterIdx int // 怪物表选中行索引

	// 布局切换
	mainStack     *fyne.Container
	splitLayout   fyne.CanvasObject
	simFullWidth  fyne.CanvasObject
}

// New 创建并运行应用
func New() {
	cfg, _ := config.Load()

	a := app.New()
	if len(iconData) > 0 {
		a.SetIcon(fyne.NewStaticResource("icon.png", iconData))
	}
	// 根据配置设置主题变体
	themeVariant := theme.VariantDark
	if cfg.Theme == "light" {
		themeVariant = theme.VariantLight
	}
	customTheme := NewCJKTheme(themeVariant)
	a.Settings().SetTheme(customTheme)

	w := a.NewWindow(appTitle)
	w.Resize(fyne.NewSize(1280, 860))
	if len(iconData) > 0 {
		w.SetIcon(fyne.NewStaticResource("icon.png", iconData))
	}

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
	statusBar := NewCard(container.NewHBox(a.statusLabel), colorBgSecondary, 0, 6)

	// 可拖动分割条（爆率修改等页面使用）
	splitLayout := newDragDivider(leftPanel, rightPanel, 0.25)

	// 爆率模拟页面全宽布局（不含左侧怪物列表）
	simFullWidth := a.buildSimTabFull()

	// Stack 容器用于切换布局
	mainStack := container.NewStack(splitLayout, simFullWidth)
	simFullWidth.Hide()
	a.mainStack = mainStack
	a.splitLayout = splitLayout
	a.simFullWidth = simFullWidth

	return container.NewBorder(toolbar, statusBar, nil, nil, mainStack)
}

// buildToolbar 构建顶部工具栏
func (a *App) buildToolbar() fyne.CanvasObject {
	a.serverPathEntry = widget.NewEntry()
	a.serverPathEntry.SetPlaceHolder("选择传奇服务端根目录 (如 D:\\MirServer)")
	a.serverPathEntry.SetText(a.cfg.ServerRoot)

	browseBtn := NewRoundedBtn("浏览...", colorBgTertiary, colorTextPrimary, colorBgHover, 6, a.onBrowseServer)
	autoDetectBtn := NewRoundedBtn("🔍 自动检测", colorBgTertiary, colorTextPrimary, colorBgHover, 6, a.onAutoDetect)
	loadBtn := NewRoundedBtn("📂 加载爆率文件", colorAccent, color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xff}, colorAccentHover, 6, a.onLoadFiles)

	a.engineSelect = widget.NewSelect(
		[]string{"HERO", "GOM", "GEE", "BLUE", "自动检测"},
		a.onEngineChanged,
	)
	a.engineSelect.SetSelected("自动检测")

	row1 := container.NewBorder(nil, nil, widget.NewLabel("服务端目录:"), browseBtn, a.serverPathEntry)
	row2 := container.NewBorder(nil, nil, widget.NewLabel("引擎类型:"), container.NewHBox(autoDetectBtn, loadBtn), a.engineSelect)

	return NewCard(
		container.NewVBox(row1, row2),
		colorBgSecondary, 0, 10,
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

	header := widget.NewLabelWithStyle("👹 怪物列表", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	scroll := container.NewVScroll(a.fileList)
	return container.NewBorder(NewCard(header, colorBgTertiary, 0, 8), nil, nil, nil, scroll)
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

	// 切换Tab时控制布局：爆率模拟页面隐藏左侧怪物列表
	tabs.OnChanged = func(tab *container.TabItem) {
		if a.mainStack == nil {
			return
		}
		if tab.Text == "爆率模拟" {
			a.splitLayout.Hide()
			a.simFullWidth.Show()
		} else {
			a.simFullWidth.Hide()
			a.splitLayout.Show()
		}
	}

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

	addBtn := NewRoundedBtn("➕ 新增掉落", colorBgTertiary, colorTextPrimary, colorBgHover, 6, a.onAddEntry)
	mulBtn := NewRoundedBtn("✖️ 批量倍率", colorBgTertiary, colorTextPrimary, colorBgHover, 6, a.onBatchMultiply)
	batchSetBtn := NewRoundedBtn("🎯 批量概率", colorBgTertiary, colorTextPrimary, colorBgHover, 6, a.onBatchSetProb)
	detectBtn := NewRoundedBtn("⚠️ 异常检测", colorBgTertiary, colorTextPrimary, colorBgHover, 6, a.onDetectAnomaly)
	backupBtn := NewRoundedBtn("💾 备份文件", colorBgTertiary, colorTextPrimary, colorBgHover, 6, a.onBackupCurrent)
	backupAllBtn := NewRoundedBtn("📦 备份目录", colorBgTertiary, colorTextPrimary, colorBgHover, 6, a.onBackupAll)
	saveBtn := NewRoundedBtn("💾 保存", colorAccent, color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xff}, colorAccentHover, 6, a.onSaveFile)

	btnBar := container.NewHBox(addBtn, mulBtn, batchSetBtn, detectBtn, backupBtn, backupAllBtn, layout.NewSpacer(), saveBtn)
	return container.NewBorder(nil, NewCard(btnBar, colorBgSecondary, 0, 6), nil, nil, a.detailTable)
}

// buildSimTab 构建爆率模拟页
// buildSimTab 构建爆率模拟页（占位，实际使用 buildSimTabFull）
func (a *App) buildSimTab() fyne.CanvasObject {
	return widget.NewLabel("")
}

// buildSimTabFull 构建爆率模拟页全宽布局（无左侧怪物列表）
func (a *App) buildSimTabFull() fyne.CanvasObject {
	// === Row 1: 筛选条件（怪物/物品/地图） ===

	// 怪物选择
	a.simMonsterRadio = widget.NewRadioGroup([]string{"所有怪物", "指定怪物"}, nil)
	a.simMonsterRadio.SetSelected("所有怪物")
	a.simMonsterRadio.Horizontal = true
	a.simMonsterCount = widget.NewLabel("(0)")
	monsterSetBtn := widget.NewButton("指定怪物设置", func() { a.showMonsterPicker() })
	monsterSetBtn.Hidden = true
	a.simMonsterRadio.OnChanged = func(val string) {
		if val == "指定怪物" {
			monsterSetBtn.Hidden = false
		} else {
			monsterSetBtn.Hidden = true
		}
		monsterSetBtn.Refresh()
	}
	monsterSection := container.NewVBox(
		widget.NewLabelWithStyle("怪物", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewHBox(a.simMonsterRadio, a.simMonsterCount, monsterSetBtn),
	)

	// 物品选择
	a.simItemRadio = widget.NewRadioGroup([]string{"所有物品", "指定物品"}, nil)
	a.simItemRadio.SetSelected("所有物品")
	a.simItemRadio.Horizontal = true
	a.simItemCount = widget.NewLabel("(0)")
	itemSetBtn := widget.NewButton("指定物品设置", func() { a.showItemPicker() })
	itemSetBtn.Hidden = true
	a.simItemRadio.OnChanged = func(val string) {
		if val == "指定物品" {
			itemSetBtn.Hidden = false
		} else {
			itemSetBtn.Hidden = true
		}
		itemSetBtn.Refresh()
	}
	itemSection := container.NewVBox(
		widget.NewLabelWithStyle("物品", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewHBox(a.simItemRadio, a.simItemCount, itemSetBtn),
	)

	// 地图选择
	a.simMapRadio = widget.NewRadioGroup([]string{"所有地图", "指定地图"}, nil)
	a.simMapRadio.SetSelected("所有地图")
	a.simMapRadio.Horizontal = true
	a.simMapCount = widget.NewLabel("(0)")
	mapSetBtn := widget.NewButton("指定地图设置", func() { a.showMapPicker() })
	mapSetBtn.Hidden = true
	a.simMapRadio.OnChanged = func(val string) {
		if val == "指定地图" {
			mapSetBtn.Hidden = false
		} else {
			mapSetBtn.Hidden = true
		}
		mapSetBtn.Refresh()
	}
	mapSection := container.NewVBox(
		widget.NewLabelWithStyle("地图", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewHBox(a.simMapRadio, a.simMapCount, mapSetBtn),
	)

	filterRow := container.NewGridWithColumns(3, monsterSection, itemSection, mapSection)

	// === Row 2: 模拟参数 ===
	a.simDurationEntry = widget.NewEntry()
	a.simDurationEntry.SetText("24")
	a.simKillRatioEntry = widget.NewEntry()
	a.simKillRatioEntry.SetText("50")
	a.simPityCheck = widget.NewCheck("保底", nil)
	a.simPityEntry = widget.NewEntry()
	a.simPityEntry.SetText("100")
	a.simRunCountEntry = widget.NewEntry()
	a.simRunCountEntry.SetText("1")

	simBtn := NewRoundedBtn("▶ 开始模拟", colorAccent, color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xff}, colorAccentHover, 6, a.onRunSimNew)

	paramRow := container.NewHBox(
		widget.NewLabel("模拟时长(h):"), a.simDurationEntry,
		widget.NewLabel("消灭比例(%):"), a.simKillRatioEntry,
		a.simPityCheck, widget.NewLabel("连续空击杀:"), a.simPityEntry,
		widget.NewLabel("模拟轮数:"), a.simRunCountEntry,
		layout.NewSpacer(),
		simBtn,
	)

	configArea := container.NewVBox(
		NewCard(filterRow, colorBgSecondary, 0, 10),
		NewCard(paramRow, colorBgSecondary, 0, 8),
	)

	// === 掉落物品列表 (Table) ===
	a.simItemSearch = widget.NewEntry()
	a.simItemSearch.SetPlaceHolder("搜索物品名称...")
	a.simItemSearch.OnChanged = func(query string) {
		a.filterItemTable(query)
	}

	a.simItemTable = widget.NewTable(
		func() (int, int) {
			if a.simResult == nil {
				return 0, 2
			}
			return len(a.filteredItemStats) + 1, 2
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
					label.SetText("物品名称")
				} else {
					label.SetText("掉落数量")
				}
				return
			}
			label.TextStyle = fyne.TextStyle{}
			if id.Row-1 >= len(a.filteredItemStats) {
				label.SetText("")
				return
			}
			item := a.filteredItemStats[id.Row-1]
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
		if id.Row > 0 && id.Row-1 < len(a.filteredItemStats) {
			selectedItem := a.filteredItemStats[id.Row-1]
			for i, item := range a.simResult.ItemStats {
				if item == selectedItem {
					a.onItemSelected(i)
					break
				}
			}
		}
	}

	// === 掉落地图列表 (Table) ===
	a.simMapTable = widget.NewTable(
		func() (int, int) {
			return a.mapDisplayCount + 1, 2
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
			idx := id.Row - 1
			if idx >= a.mapDisplayCount || idx >= len(a.mapDisplayData) || a.mapDisplayData[idx] == nil {
				label.SetText("")
				return
			}
			m := a.mapDisplayData[idx]
			if id.Col == 0 {
				label.Alignment = fyne.TextAlignLeading
				label.SetText(a.formatMapDisplay(m.MapName))
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
			return a.monDisplayCount + 1, 3
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
			idx := id.Row - 1
			if idx >= a.monDisplayCount || idx >= len(a.monDisplayData) || a.monDisplayData[idx] == nil {
				label.SetText("")
				return
			}
			ms := a.monDisplayData[idx]
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
	a.simMonsterTable.OnSelected = func(id widget.TableCellID) {
		if id.Row > 0 {
			a.selectedMonsterIdx = id.Row - 1
		}
	}

	// 统计摘要
	a.simResultLabel = widget.NewLabel("")
	a.simResultLabel.Wrapping = fyne.TextWrapWord
	exportBtn := NewRoundedBtn("📤 导出结果", colorBgTertiary, colorTextPrimary, colorBgHover, 6, a.onExportSimResult)
	summaryBar := NewCard(container.NewBorder(nil, nil, nil, exportBtn, a.simResultLabel), colorBgSecondary, 0, 8)

	// 三列结果（带搜索）
	a.simMapSearch = widget.NewEntry()
	a.simMapSearch.SetPlaceHolder("搜索地图...")
	a.simMapSearch.OnChanged = func(q string) {
		a.filterDisplayDataBySearch()
	}
	a.simMonsterSearch = widget.NewEntry()
	a.simMonsterSearch.SetPlaceHolder("搜索怪物...")
	a.simMonsterSearch.OnChanged = func(q string) {
		a.filterDisplayDataBySearch()
	}
	openFileBtn := NewRoundedBtn("📂 打开文件", colorBgTertiary, colorTextPrimary, colorBgHover, 6, func() {
		a.openSelectedMonsterFile()
	})
	monsterSearchBar := container.NewBorder(nil, nil, nil, openFileBtn, a.simMonsterSearch)

	resultGrid := container.NewGridWithColumns(3,
		container.NewBorder(
			widget.NewLabelWithStyle("掉落物品列表", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
			a.simItemSearch, nil, nil, a.simItemTable),
		container.NewBorder(
			widget.NewLabelWithStyle("掉落地图列表", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
			a.simMapSearch, nil, nil, a.simMapTable),
		container.NewBorder(
			widget.NewLabelWithStyle("掉落怪物列表", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
			monsterSearchBar, nil, nil, a.simMonsterTable),
	)

	// 稀有物品追踪面板
	a.simTrackerLabel = widget.NewLabel("")
	a.simTrackerLabel.Wrapping = fyne.TextWrapWord
	trackerScroll := container.NewVScroll(a.simTrackerLabel)
	trackerScroll.SetMinSize(fyne.NewSize(0, 80))
	trackerPanel := container.NewBorder(
		widget.NewLabelWithStyle("稀有物品追踪", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		nil, nil, nil, trackerScroll,
	)

	return container.NewBorder(
		configArea,
		container.NewVBox(widget.NewSeparator(), summaryBar),
		nil, nil,
		container.NewVSplit(resultGrid, trackerPanel),
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

// resolveMapName 将地图编号解析为地图名称
func (a *App) resolveMapName(mapID string) string {
	if a.mapInfoLookup != nil {
		// 精确匹配
		if name, ok := a.mapInfoLookup[mapID]; ok && name != "" {
			return name
		}
		// 小写匹配（MonGen.txt中可能用小写）
		if name, ok := a.mapInfoLookup[strings.ToLower(mapID)]; ok && name != "" {
			return name
		}
	}
	return mapID
}

// formatMapDisplay 格式化地图显示名称（编号+名称）
func (a *App) formatMapDisplay(mapID string) string {
	name := a.resolveMapName(mapID)
	if name != mapID {
		return fmt.Sprintf("%s (%s)", mapID, name)
	}
	return mapID
}

// refreshDisplayData 刷新地图和怪物的显示数据
// 使用固定容量策略：数组只增不缩，通过count控制实际显示行数
// 这样Table的Length回调返回值不会因筛选而变化，避免Fyne重置滚动位置
func (a *App) refreshDisplayData(mapData []*simulator.MapStat, monData []*simulator.MonsterStat) {
	// 地图数据
	a.mapDisplayCount = len(mapData)
	for len(a.mapDisplayData) < a.mapDisplayCount {
		a.mapDisplayData = append(a.mapDisplayData, nil)
	}
	copy(a.mapDisplayData, mapData)
	for i := a.mapDisplayCount; i < len(a.mapDisplayData); i++ {
		a.mapDisplayData[i] = nil
	}

	// 怪物数据
	a.monDisplayCount = len(monData)
	for len(a.monDisplayData) < a.monDisplayCount {
		a.monDisplayData = append(a.monDisplayData, nil)
	}
	copy(a.monDisplayData, monData)
	for i := a.monDisplayCount; i < len(a.monDisplayData); i++ {
		a.monDisplayData[i] = nil
	}
}

// onItemSelected 点击物品列表联动：筛选地图和怪物
// 使用模拟器的ItemMonsterDrops/ItemMapDrops获取物品级别的准确掉落数
func (a *App) onItemSelected(row int) {
	if a.simResult == nil || row >= len(a.simResult.ItemStats) {
		return
	}
	selectedItem := a.simResult.ItemStats[row].ItemName
	a.selectedItemName = selectedItem // 记录当前选中的物品

	// 从模拟结果中获取该物品的怪物掉落数和地图掉落数
	monsterDrops := a.simResult.ItemMonsterDrops[selectedItem]
	mapDrops := a.simResult.ItemMapDrops[selectedItem]

	// 构建掉落该物品的怪物列表（带物品级别的掉落数）
	a.filteredMonStats = nil
	if monsterDrops != nil {
		for monsterName, dropCnt := range monsterDrops {
			// 从全局MonsterStats中找到击杀数
			var killCnt int64
			for _, ms := range a.simResult.MonsterStats {
				if ms.MonsterName == monsterName {
					killCnt = ms.KillCount
					break
				}
			}
			a.filteredMonStats = append(a.filteredMonStats, &simulator.MonsterStat{
				MonsterName: monsterName,
				KillCount:   killCnt,
				DropCount:   dropCnt, // 物品级别的掉落数，不是总掉落数
			})
		}
		sort.Slice(a.filteredMonStats, func(i, j int) bool {
			return a.filteredMonStats[i].DropCount > a.filteredMonStats[j].DropCount
		})
	}

	// 构建掉落该物品的地图列表（带物品级别的掉落数）
	a.filteredMapStats = nil
	if mapDrops != nil {
		for mapName, dropCnt := range mapDrops {
			// 从全局MapStats中找到刷怪数
			var monCnt int64
			for _, m := range a.simResult.MapStats {
				if m.MapName == mapName {
					monCnt = m.MonsterCnt
					break
				}
			}
			a.filteredMapStats = append(a.filteredMapStats, &simulator.MapStat{
				MapName:    mapName,
				MonsterCnt: monCnt,
				DropCount:  dropCnt, // 物品级别的掉落数
			})
		}
		sort.Slice(a.filteredMapStats, func(i, j int) bool {
			return a.filteredMapStats[i].DropCount > a.filteredMapStats[j].DropCount
		})
	}

	// 刷新显示数据
	a.refreshDisplayData(a.filteredMapStats, a.filteredMonStats)
	a.simMapTable.Refresh()
	a.simMonsterTable.Refresh()
	a.statusLabel.SetText(fmt.Sprintf("已选中物品: %s | 地图:%d个 怪物:%d个", selectedItem, a.mapDisplayCount, a.monDisplayCount))

	// 更新稀有物品追踪面板
	a.updateTrackerPanel(selectedItem)
}

// onMapSelected 点击地图列表联动：筛选怪物
// 当已选中物品时，使用物品级别的掉落数；否则使用全局数据
func (a *App) onMapSelected(row int) {
	if row >= a.mapDisplayCount || row >= len(a.mapDisplayData) || a.mapDisplayData[row] == nil {
		return
	}
	selectedMapID := a.mapDisplayData[row].MapName
	selectedMapDisplay := a.formatMapDisplay(selectedMapID)

	// 使用当前选中的物品（从onItemSelected记录）
	selectedItemName := a.selectedItemName

	// 筛选该地图的怪物
	a.filteredMonStats = nil

	if selectedItemName != "" && a.simResult.ItemMonsterMapDrops[selectedItemName] != nil {
		// 物品筛选模式：使用物品→怪物→地图级别的掉落数
		monsterMapDrops := a.simResult.ItemMonsterMapDrops[selectedItemName]
		for monsterName, mapDrops := range monsterMapDrops {
			// 取该怪物在选中地图上的掉落数
			dropCnt, hasDrop := mapDrops[selectedMapID]
			if !hasDrop {
				continue
			}
			var killCnt int64
			for _, ms := range a.simResult.MonsterStats {
				if ms.MonsterName == monsterName {
					killCnt = ms.KillCount
					break
				}
			}
			a.filteredMonStats = append(a.filteredMonStats, &simulator.MonsterStat{
				MonsterName: monsterName,
				KillCount:   killCnt,
				DropCount:   dropCnt,
			})
		}
	} else {
		// 全局模式
		if a.monGenEntries != nil {
			monsterSet := make(map[string]bool)
			for _, mg := range a.monGenEntries {
				if mg.MapName == selectedMapID {
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
	}

	sort.Slice(a.filteredMonStats, func(i, j int) bool {
		return a.filteredMonStats[i].DropCount > a.filteredMonStats[j].DropCount
	})

	// 只刷新怪物显示数据，保持地图滚动位置不变
	a.monDisplayCount = len(a.filteredMonStats)
	for len(a.monDisplayData) < a.monDisplayCount {
		a.monDisplayData = append(a.monDisplayData, nil)
	}
	copy(a.monDisplayData, a.filteredMonStats)
	for i := a.monDisplayCount; i < len(a.monDisplayData); i++ {
		a.monDisplayData[i] = nil
	}
	a.simMonsterTable.Refresh()
	a.statusLabel.SetText(fmt.Sprintf("已选中地图: %s | 怪物:%d个", selectedMapDisplay, a.monDisplayCount))
}

// openSelectedMonsterFile 打开选中怪物的爆率文件
func (a *App) openSelectedMonsterFile() {
	if a.currentResults == nil {
		return
	}
	idx := a.selectedMonsterIdx
	if idx < 0 || idx >= a.monDisplayCount || idx >= len(a.monDisplayData) || a.monDisplayData[idx] == nil {
		dialog.ShowInformation("提示", "请先在怪物列表中选择一个怪物", a.mainWindow)
		return
	}
	monsterName := a.monDisplayData[idx].MonsterName

	// 在已加载的结果中查找对应的文件
	for i, r := range a.currentResults {
		if r.File.MonsterName == monsterName {
			a.selectedFileIdx = i
			a.fileList.Select(i)
			a.updateDetailPanel()
			a.statusLabel.SetText(fmt.Sprintf("已打开怪物文件: %s", monsterName))
			return
		}
	}
	dialog.ShowInformation("提示", fmt.Sprintf("未找到怪物 [%s] 的爆率文件", monsterName), a.mainWindow)
}

// updateTrackerPanel 更新稀有物品追踪面板
func (a *App) updateTrackerPanel(itemName string) {
	if a.simResult == nil || a.simResult.ItemTrackers == nil || a.simTrackerLabel == nil {
		a.simTrackerLabel.SetText("暂无追踪数据")
		return
	}

	t, exists := a.simResult.ItemTrackers[itemName]
	if !exists {
		a.simTrackerLabel.SetText(fmt.Sprintf("物品 [%s] 在本次模拟中未掉落，无追踪数据", itemName))
		return
	}

	// 计算统计信息
	totalKills := a.simResult.TotalKills
	if totalKills == 0 {
		totalKills = 1
	}

	// 期望间隔 = 总击杀 / 掉落次数
	expectedInterval := float64(totalKills) / float64(t.TotalDrops)

	// 平均实际间隔
	avgInterval := 0.0
	if len(t.Intervals) > 0 {
		var sum int64
		for _, v := range t.Intervals {
			sum += v
		}
		avgInterval = float64(sum) / float64(len(t.Intervals))
	}

	// 预计出货时间（小时）
	durationHours := a.simResult.Config.DurationHours
	if durationHours == 0 {
		durationHours = 1
	}
	killsPerHour := float64(totalKills) / durationHours
	expectedHours := expectedInterval / killsPerHour

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("物品: %s\n", itemName))
	sb.WriteString(fmt.Sprintf("掉落次数: %d\n", t.TotalDrops))
	sb.WriteString(fmt.Sprintf("期望间隔: %.0f 次击杀\n", expectedInterval))
	if len(t.Intervals) > 0 {
		sb.WriteString(fmt.Sprintf("实际平均间隔: %.0f 次击杀\n", avgInterval))
	}
	sb.WriteString(fmt.Sprintf("最长干旱期: %d 次击杀\n", t.MaxDrought))
	sb.WriteString(fmt.Sprintf("预计出货时间: %.1f 小时\n", expectedHours))
	if t.FirstDropKill > 0 {
		sb.WriteString(fmt.Sprintf("首次掉落: 第 %d 次击杀\n", t.FirstDropKill))
	}
	if t.LastDropKill > 0 {
		sb.WriteString(fmt.Sprintf("最后掉落: 第 %d 次击杀\n", t.LastDropKill))
	}

	a.simTrackerLabel.SetText(sb.String())
}

// filterItemTable 根据搜索关键词过滤物品列表
func (a *App) filterItemTable(query string) {
	if a.simResult == nil {
		return
	}
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		a.filteredItemStats = a.simResult.ItemStats
	} else {
		a.filteredItemStats = nil
		for _, item := range a.simResult.ItemStats {
			if strings.Contains(strings.ToLower(item.ItemName), query) {
				a.filteredItemStats = append(a.filteredItemStats, item)
			}
		}
	}
	a.simItemTable.Refresh()
}

// filterDisplayDataBySearch 根据搜索关键词过滤地图和怪物显示数据
func (a *App) filterDisplayDataBySearch() {
	if a.simResult == nil {
		return
	}

	// 获取当前完整数据源（不受搜索影响的原始数据）
	var mapSource []*simulator.MapStat
	if a.filteredMapStats != nil {
		mapSource = a.filteredMapStats
	} else {
		mapSource = a.simResult.MapStats
	}
	var monSource []*simulator.MonsterStat
	if a.filteredMonStats != nil {
		monSource = a.filteredMonStats
	} else {
		monSource = a.simResult.MonsterStats
	}

	// 过滤地图
	mapQuery := ""
	if a.simMapSearch != nil {
		mapQuery = strings.ToLower(strings.TrimSpace(a.simMapSearch.Text))
	}
	var filteredMaps []*simulator.MapStat
	if mapQuery == "" {
		filteredMaps = mapSource
	} else {
		for _, m := range mapSource {
			if strings.Contains(strings.ToLower(a.formatMapDisplay(m.MapName)), mapQuery) {
				filteredMaps = append(filteredMaps, m)
			}
		}
	}

	// 过滤怪物
	monQuery := ""
	if a.simMonsterSearch != nil {
		monQuery = strings.ToLower(strings.TrimSpace(a.simMonsterSearch.Text))
	}
	var filteredMons []*simulator.MonsterStat
	if monQuery == "" {
		filteredMons = monSource
	} else {
		for _, ms := range monSource {
			if strings.Contains(strings.ToLower(ms.MonsterName), monQuery) {
				filteredMons = append(filteredMons, ms)
			}
		}
	}

	a.refreshDisplayData(filteredMaps, filteredMons)
	a.simMapTable.Refresh()
	a.simMonsterTable.Refresh()
}

// buildAuthTab 构建授权管理页
func (a *App) buildAuthTab() fyne.CanvasObject {
	machineID, _ := a.authMgr.GetMachineID()
	info, _ := a.authMgr.CheckLicense()

	machineLabel := widget.NewLabel(fmt.Sprintf("本机机器码: %s", machineID))
	machineLabel.Wrapping = fyne.TextWrapWord

	copyBtn := NewRoundedBtn("📋 复制机器码", colorBgTertiary, colorTextPrimary, colorBgHover, 6, func() {
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

	activateBtn := NewRoundedBtn("激活", colorAccent, color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xff}, colorAccentHover, 6, func() {
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

	helpText := widget.NewLabel("1. 复制本机机器码\n2. 联系管理员获取激活码\n3. 输入激活码完成绑定\n4. 一机一码，绑定后不可随意更换设备")
	helpText.Wrapping = fyne.TextWrapWord

	cardMachine := NewCard(
		container.NewVBox(
			widget.NewLabelWithStyle("💻 设备信息", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			container.NewBorder(nil, nil, nil, copyBtn, machineLabel),
		),
		colorBgCard, 8, 12,
	)
	cardActivate := NewCard(
		container.NewVBox(
			widget.NewLabelWithStyle("🔑 授权状态", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			statusLabel,
			container.NewBorder(nil, nil, nil, activateBtn, activateEntry),
		),
		colorBgCard, 8, 12,
	)
	cardHelp := NewCard(
		container.NewVBox(
			widget.NewLabelWithStyle("📖 授权说明", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			helpText,
			widget.NewLabelWithStyle("当前为开发模式，所有功能可用", fyne.TextAlignLeading, fyne.TextStyle{Italic: true}),
		),
		colorBgCard, 8, 12,
	)

	return container.NewVBox(cardMachine, cardActivate, cardHelp)
}

// buildLogTab 构建操作日志页
func (a *App) buildLogTab() fyne.CanvasObject {
	logLabel := widget.NewLabel("操作记录:\n")
	logLabel.Wrapping = fyne.TextWrapWord

	refreshBtn := NewRoundedBtn("🔄 刷新日志", colorBgTertiary, colorTextPrimary, colorBgHover, 6, func() {
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
		names = append(names, a.formatMapDisplay(mapID))
	}
	sort.Strings(names)
	a.showMultiPicker("选择地图", names, func(selected []string) {
		// 提取地图编号（去掉括号中的名称部分）
		var mapIDs []string
		for _, s := range selected {
			if idx := strings.Index(s, " ("); idx > 0 {
				mapIDs = append(mapIDs, s[:idx])
			} else {
				mapIDs = append(mapIDs, s)
			}
		}
		a.selectedMaps = mapIDs
		a.simMapCount.SetText(fmt.Sprintf("(%d)", len(mapIDs)))
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

	pityThreshold, _ := strconv.Atoi(a.simPityEntry.Text)
	if pityThreshold <= 0 {
		pityThreshold = 100
	}
	runCount, _ := strconv.Atoi(a.simRunCountEntry.Text)
	if runCount <= 0 {
		runCount = 1
	}

	cfg := simulator.SimConfig{
		DurationHours:   duration,
		KillRatio:       killRatio,
		RefreshInterval: 60,
		RefreshCount:    10,
		MapRateModifier: 1.0,
		PityEnabled:     a.simPityCheck.Checked,
		PityThreshold:   pityThreshold,
		RunCount:        runCount,
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
	a.selectedItemName = ""
	a.mapDisplayCount = 0
	a.monDisplayCount = 0

	a.statusLabel.SetText("正在模拟...请稍候")

	go func() {
		var files []*parser.MonsterDropFile
		for _, r := range a.currentResults {
			files = append(files, r.File)
		}

		multiResult := a.simulator.SimulateAll(files, a.monGenEntries, cfg)
		a.simMultiResult = multiResult
		// 使用第一次模拟的结果作为默认显示
		if len(multiResult.Runs) > 0 {
			a.simResult = multiResult.Runs[0]
			a.filteredItemStats = a.simResult.ItemStats
		}

		a.refreshDisplayData(a.simResult.MapStats, a.simResult.MonsterStats)
		a.simItemTable.Refresh()
		a.simMapTable.Refresh()
		a.simMonsterTable.Refresh()

		var summary string
		if runCount > 1 {
			summary = fmt.Sprintf("模拟%d轮 平均掉落:%.0f (最小:%d 最大:%d) 空爆率:%.1f%% 耗时:%v",
				runCount, multiResult.AvgTotalDrops, multiResult.MinTotalDrops,
				multiResult.MaxTotalDrops, multiResult.AvgEmptyRate*100, multiResult.Duration)
		} else {
			summary = fmt.Sprintf("总击杀:%d 总掉落:%d 空爆率:%.1f%% 耗时:%v",
				a.simResult.TotalKills, a.simResult.TotalDrops,
				a.simResult.EmptyRate()*100, a.simResult.Duration)
		}
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
	var text string
	if a.simMultiResult != nil && a.simMultiResult.RunCount > 1 {
		text = simulator.FormatMultiResult(a.simMultiResult)
	} else {
		text = simulator.FormatResult(a.simResult)
	}
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

// dragDivider 自定义可拖动分割条容器
type dragDivider struct {
	widget.BaseWidget
	left, right fyne.CanvasObject
	offset      float64 // 0.0 ~ 1.0
	dragging    bool
}

func newDragDivider(left, right fyne.CanvasObject, offset float64) *dragDivider {
	d := &dragDivider{left: left, right: right, offset: offset}
	d.ExtendBaseWidget(d)
	return d
}

func (d *dragDivider) CreateRenderer() fyne.WidgetRenderer {
	separator := widget.NewSeparator()
	separator.Resize(fyne.NewSize(4, 0))
	return &dragDividerRenderer{divider: d, separator: separator}
}

func (d *dragDivider) Dragged(ev *fyne.DragEvent) {
	d.dragging = true
	size := d.Size()
	if size.Width > 0 {
		newOffset := float64(ev.Position.X) / float64(size.Width)
		if newOffset < 0.1 {
			newOffset = 0.1
		}
		if newOffset > 0.9 {
			newOffset = 0.9
		}
		d.offset = newOffset
		d.Refresh()
	}
}

func (d *dragDivider) DragEnd() {
	d.dragging = false
}

type dragDividerRenderer struct {
	divider   *dragDivider
	separator *widget.Separator
}

func (r *dragDividerRenderer) Layout(size fyne.Size) {
	sepWidth := float32(4)
	leftWidth := float32(float64(size.Width)*r.divider.offset) - sepWidth/2
	if leftWidth < 0 {
		leftWidth = 0
	}
	rightWidth := size.Width - leftWidth - sepWidth
	if rightWidth < 0 {
		rightWidth = 0
	}

	r.divider.left.Move(fyne.NewPos(0, 0))
	r.divider.left.Resize(fyne.NewSize(leftWidth, size.Height))

	r.separator.Move(fyne.NewPos(leftWidth, 0))
	r.separator.Resize(fyne.NewSize(sepWidth, size.Height))

	r.divider.right.Move(fyne.NewPos(leftWidth+sepWidth, 0))
	r.divider.right.Resize(fyne.NewSize(rightWidth, size.Height))
}

func (r *dragDividerRenderer) MinSize() fyne.Size {
	leftMin := r.divider.left.MinSize()
	rightMin := r.divider.right.MinSize()
	return fyne.NewSize(leftMin.Width+rightMin.Width+4,
		fyne.Max(leftMin.Height, rightMin.Height))
}

func (r *dragDividerRenderer) Refresh() {
	r.Layout(r.divider.Size())
}

func (r *dragDividerRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.divider.left, r.separator, r.divider.right}
}

func (r *dragDividerRenderer) Destroy() {}
