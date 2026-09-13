package gui

import (
	"fmt"
	"path/filepath"
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

const appTitle = "传奇爆率模拟与修改工具 v1.0"

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
	simResultLabel    *widget.Label

	simDurationEntry  *widget.Entry
	simKillRatioEntry *widget.Entry
	simRefreshEntry   *widget.Entry
	simCountEntry     *widget.Entry

	selectedFileIdx int
}

// New 创建并运行应用
func New() {
	a := app.New()
	w := a.NewWindow(appTitle)
	w.Resize(fyne.NewSize(1200, 800))

	cfg, _ := config.Load()

	gui := &App{
		fyneApp:           a,
		mainWindow:        w,
		cfg:               cfg,
		authMgr:           auth.NewLocalAuth(),
		editor:            editor.New(),
		simulator:         simulator.New(),
		selectedFileIdx:   -1,
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
	split.Offset = 0.25

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
		container.NewHBox(
			widget.NewLabel("服务端目录:"),
			a.serverPathEntry,
			browseBtn,
		),
		container.NewHBox(
			widget.NewLabel("引擎类型:"),
			a.engineSelect,
			autoDetectBtn,
			layout.NewSpacer(),
			loadBtn,
		),
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
				name := r.File.MonsterName
				count := 0
				for _, e := range r.File.Entries {
					if e.IsEditable() {
						count++
					}
				}
				label.SetText(fmt.Sprintf("%s (%d条)", name, count))
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
	a.simDurationEntry = widget.NewEntry()
	a.simDurationEntry.SetText("1")
	a.simKillRatioEntry = widget.NewEntry()
	a.simKillRatioEntry.SetText("0.6")
	a.simRefreshEntry = widget.NewEntry()
	a.simRefreshEntry.SetText("60")
	a.simCountEntry = widget.NewEntry()
	a.simCountEntry.SetText("10")

	simBtn := widget.NewButton("模拟当前怪物", a.onRunSimulation)
	simAllBtn := widget.NewButton("模拟全部怪物", a.onRunSimulationAll)
	exportBtn := widget.NewButton("导出结果", a.onExportSimResult)

	a.simResultLabel = widget.NewLabel("")
	a.simResultLabel.Wrapping = fyne.TextWrapWord

	form := container.NewVBox(
		widget.NewLabel("模拟参数配置"),
		widget.NewSeparator(),
		container.NewGridWithColumns(2,
			widget.NewLabel("模拟时长(小时):"), a.simDurationEntry,
			widget.NewLabel("击杀比例(0~1):"), a.simKillRatioEntry,
			widget.NewLabel("刷新间隔(秒):"), a.simRefreshEntry,
			widget.NewLabel("每次刷新数量:"), a.simCountEntry,
		),
		widget.NewSeparator(),
		container.NewHBox(simBtn, simAllBtn, exportBtn),
		widget.NewSeparator(),
		widget.NewLabel("模拟结果:"),
	)

	scrollResult := container.NewVScroll(a.simResultLabel)
	scrollResult.SetMinSize(fyne.NewSize(0, 400))
	return container.NewBorder(form, nil, nil, nil, scrollResult)
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
		container.NewHBox(activateEntry, activateBtn),
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

	{
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
			warnings = append(warnings, r.Warnings...)
		}
		if len(warnings) > 0 {
			dialog.ShowInformation("解析提示",
				fmt.Sprintf("有 %d 条无法识别的行，请检查格式", len(warnings)),
				a.mainWindow)
		}
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
		dialog.ShowInfo("提示", "请先选择一个怪物文件", a.mainWindow)
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
		dialog.ShowInfo("提示", "请先选择一个怪物文件", a.mainWindow)
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
		dialog.ShowInfo("提示", "请先选择一个怪物文件", a.mainWindow)
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
		dialog.ShowInfo("提示", "请先加载服务端目录", a.mainWindow)
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
		dialog.ShowInfo("提示", "请先加载爆率文件", a.mainWindow)
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

func (a *App) onRunSimulation() {
	if a.selectedFileIdx < 0 || a.currentResults == nil {
		dialog.ShowInfo("提示", "请先选择一个怪物文件", a.mainWindow)
		return
	}
	result := a.runSim(false)
	if result != nil {
		a.simResultLabel.SetText(simulator.FormatResult(result))
	}
}

func (a *App) onRunSimulationAll() {
	if a.currentResults == nil {
		dialog.ShowInfo("提示", "请先加载爆率文件", a.mainWindow)
		return
	}
	result := a.runSim(true)
	if result != nil {
		a.simResultLabel.SetText(simulator.FormatResult(result))
	}
}

func (a *App) runSim(all bool) *simulator.SimResult {
	duration, err1 := strconv.ParseFloat(a.simDurationEntry.Text, 64)
	killRatio, err2 := strconv.ParseFloat(a.simKillRatioEntry.Text, 64)
	refreshInterval, err3 := strconv.ParseFloat(a.simRefreshEntry.Text, 64)
	refreshCount, err4 := strconv.Atoi(a.simCountEntry.Text)

	if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
		dialog.ShowError(fmt.Errorf("请输入有效的模拟参数"), a.mainWindow)
		return nil
	}
	if killRatio < 0 || killRatio > 1 {
		dialog.ShowError(fmt.Errorf("击杀比例必须在0~1之间"), a.mainWindow)
		return nil
	}

	cfg := simulator.SimConfig{
		DurationHours:   duration,
		KillRatio:       killRatio,
		RefreshInterval: refreshInterval,
		RefreshCount:    refreshCount,
		MapRateModifier: 1.0,
	}

	a.statusLabel.SetText("正在模拟...")

	if all {
		var files []*parser.MonsterDropFile
		for _, r := range a.currentResults {
			files = append(files, r.File)
		}
		result := a.simulator.SimulateAll(files, cfg)
		a.statusLabel.SetText(fmt.Sprintf("模拟完成 - 总击杀:%d 总掉落:%d 耗时:%v",
			result.TotalKills, result.TotalDrops, result.Duration))
		return result
	}

	file := a.currentResults[a.selectedFileIdx].File
	result := a.simulator.Simulate(file, cfg)
	a.statusLabel.SetText(fmt.Sprintf("模拟完成 - %s 击杀:%d 掉落:%d",
		file.MonsterName, result.TotalKills, result.TotalDrops))

	return &simulator.SimResult{
		Config:       cfg,
		MonsterStats: map[string]*simulator.MonsterSimResult{file.MonsterName: result},
		TotalKills:   result.TotalKills,
		TotalDrops:   result.TotalDrops,
		TotalEmpty:   result.EmptyDrops,
	}
}

func (a *App) onExportSimResult() {
	text := a.simResultLabel.Text
	if text == "" {
		dialog.ShowInfo("提示", "请先运行模拟", a.mainWindow)
		return
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
