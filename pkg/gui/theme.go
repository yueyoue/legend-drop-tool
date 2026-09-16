package gui

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"

	"github.com/yueyoue/legend-drop-tool/fonts"
)

// 传奇爆率工具深色主题配色（来自 theme-demo.html）
var (
	colorBgPrimary   = color.NRGBA{R: 15, G: 17, B: 23, A: 255}   // #0f1117
	colorBgSecondary = color.NRGBA{R: 22, G: 24, B: 34, A: 255}   // #161822
	colorBgTertiary  = color.NRGBA{R: 28, G: 31, B: 46, A: 255}   // #1c1f2e
	colorBgCard      = color.NRGBA{R: 30, G: 34, B: 53, A: 255}   // #1e2235
	colorBgHover     = color.NRGBA{R: 37, G: 42, B: 58, A: 255}   // #252a3a
	colorBgActive    = color.NRGBA{R: 42, G: 48, B: 72, A: 255}   // #2a3048
	colorBorder      = color.NRGBA{R: 42, G: 46, B: 66, A: 255}   // #2a2e42
	colorBorderLight = color.NRGBA{R: 53, G: 58, B: 82, A: 255}   // #353a52
	colorTextPrimary = color.NRGBA{R: 228, G: 230, B: 240, A: 255} // #e4e6f0
	colorTextSecond  = color.NRGBA{R: 139, G: 144, B: 168, A: 255} // #8b90a8
	colorTextMuted   = color.NRGBA{R: 90, G: 95, B: 120, A: 255}  // #5a5f78
	colorAccent      = color.NRGBA{R: 212, G: 168, B: 67, A: 255}  // #d4a843
	colorAccentHover = color.NRGBA{R: 224, G: 185, B: 79, A: 255}  // #e0b94f
	colorAccentDim   = color.NRGBA{R: 212, G: 168, B: 67, A: 38}   // rgba(212,168,67,0.15)
	colorDanger      = color.NRGBA{R: 224, G: 85, B: 85, A: 255}   // #e05555
	colorDangerDim   = color.NRGBA{R: 224, G: 85, B: 85, A: 38}    // rgba(224,85,85,0.15)
	colorSuccess     = color.NRGBA{R: 78, G: 203, B: 113, A: 255}  // #4ecb71
	colorSuccessDim  = color.NRGBA{R: 78, G: 203, B: 113, A: 38}   // rgba(78,203,113,0.15)
	colorInfo        = color.NRGBA{R: 91, G: 157, B: 240, A: 255}  // #5b9df0
	colorPurple      = color.NRGBA{R: 167, G: 139, B: 250, A: 255} // #a78bfa
)

// DarkTheme 传奇爆率工具深色主题
type DarkTheme struct {
	regularFont fyne.Resource
	boldFont    fyne.Resource
}

// NewCJKTheme 创建深色主题（保留函数名兼容）
func NewCJKTheme() *DarkTheme {
	t := &DarkTheme{}
	t.loadFont()
	return t
}

// loadFont 加载中文字体
func (t *DarkTheme) loadFont() {
	if len(fonts.ChineseFont) > 1024 {
		t.regularFont = fyne.NewStaticResource("WenQuanYiMicroHei.ttf", fonts.ChineseFont)
		t.boldFont = t.regularFont
		fmt.Printf("[DarkTheme] 使用嵌入字体: WenQuanYiMicroHei.ttf (%d bytes)\n", len(fonts.ChineseFont))
		return
	}

	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)
	localFonts := []string{
		filepath.Join(exeDir, "msyh.ttc"), filepath.Join(exeDir, "msyh.ttf"),
		filepath.Join(exeDir, "simhei.ttf"), filepath.Join(exeDir, "WenQuanYiMicroHei.ttf"),
		filepath.Join(exeDir, "fonts", "msyh.ttc"), filepath.Join(exeDir, "fonts", "msyh.ttf"),
		filepath.Join(exeDir, "fonts", "simhei.ttf"), filepath.Join(exeDir, "fonts", "WenQuanYiMicroHei.ttf"),
	}
	for _, fontPath := range localFonts {
		if data, err := os.ReadFile(fontPath); err == nil && len(data) > 1024 {
			t.regularFont = fyne.NewStaticResource(filepath.Base(fontPath), data)
			t.boldFont = t.regularFont
			fmt.Printf("[DarkTheme] 本地字体: %s (%d bytes)\n", fontPath, len(data))
			return
		}
	}

	if runtime.GOOS == "windows" {
		fontDirs := []string{}
		for _, envVar := range []string{"WINDIR", "windir", "SystemRoot"} {
			if v := os.Getenv(envVar); v != "" {
				fontDirs = append(fontDirs, filepath.Join(v, "Fonts"))
			}
		}
		fontDirs = append(fontDirs, `C:\Windows\Fonts`, `D:\Windows\Fonts`)
		for _, dir := range fontDirs {
			for _, name := range []string{"msyh.ttc", "msyhbd.ttc", "simhei.ttf", "simsun.ttc"} {
				fontPath := filepath.Join(dir, name)
				if data, err := os.ReadFile(fontPath); err == nil && len(data) > 1024 {
					t.regularFont = fyne.NewStaticResource(name, data)
					t.boldFont = t.regularFont
					fmt.Printf("[DarkTheme] 系统字体: %s (%d bytes)\n", fontPath, len(data))
					return
				}
			}
		}
	}
}

// Font 返回字体资源
func (t *DarkTheme) Font(style fyne.TextStyle) fyne.Resource {
	if style.Monospace {
		if t.regularFont != nil {
			return t.regularFont
		}
		return theme.DefaultTheme().Font(style)
	}
	if style.Bold && t.boldFont != nil {
		return t.boldFont
	}
	if t.regularFont != nil {
		return t.regularFont
	}
	return theme.DefaultTheme().Font(style)
}

// Color 返回深色主题颜色
func (t *DarkTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	// 背景色
	case theme.ColorNameBackground:
		return colorBgPrimary
	case theme.ColorNameHeaderBackground:
		return colorBgSecondary
	case theme.ColorNameOverlayBackground:
		return colorBgCard

	// 前景/按钮
	case theme.ColorNameForeground:
		return colorTextPrimary
	case theme.ColorNameForegroundOnPrimary:
		return color.NRGBA{R: 0, G: 0, B: 0, A: 255} // 按钮上的文字用黑色
	case theme.ColorNamePrimary:
		return colorAccent
	case theme.ColorNameHover:
		return colorBgHover
	case theme.ColorNamePressed:
		return colorBgActive
	case theme.ColorNameFocus:
		return colorAccent
	case theme.ColorNameSelection:
		return colorAccentDim

	// 输入框
	case theme.ColorNameInputBackground:
		return colorBgTertiary
	case theme.ColorNameInputBorder:
		return colorBorder
	case theme.ColorNamePlaceHolder:
		return colorTextMuted
	case theme.ColorNameDisabled:
		return colorTextMuted
	case theme.ColorNameDisabledButton:
		return colorBgTertiary

	// 分隔线/阴影
	case theme.ColorNameSeparator:
		return colorBorder
	case theme.ColorNameShadow:
		return color.NRGBA{R: 0, G: 0, B: 0, A: 60}

	// 表格
	case theme.ColorNameTableBackground:
		return colorBgPrimary
	case theme.ColorNameTableRow:
		return colorBgSecondary
	case theme.ColorNameTableHeader:
		return colorBgTertiary

	// 按钮
	case theme.ColorNameButton:
		return colorBgTertiary

	// 成功/错误/警告
	case theme.ColorNameSuccess:
		return colorSuccess
	case theme.ColorNameError:
		return colorDanger
	case theme.ColorNameWarning:
		return colorAccent

	// 滚动条
	case theme.ColorNameScrollBar:
		return colorBorder
	}

	// 其他未映射的颜色走深色主题默认值
	return theme.DefaultTheme().Color(name, theme.VariantDark)
}

// Icon 返回图标资源
func (t *DarkTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

// Size 返回尺寸配置（紧凑布局 + 小圆角）
func (t *DarkTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNameInnerPadding:
		return 4 // 内边距（默认8→4，更紧凑）
	case theme.SizeNamePadding:
		return 4 // 通用间距（默认4）
	case theme.SizeNameText:
		return 13 // 文字大小
	case theme.SizeNameHeadingText:
		return 15
	case theme.SizeNameSubHeadingText:
		return 13
	case theme.SizeNameCaptionText:
		return 11
	case theme.SizeNameInlineIcon:
		return 16
	case theme.SizeNameScrollBar:
		return 6 // 滚动条宽度
	case theme.SizeNameScrollBarSmall:
		return 4
	case theme.SizeNameSeparator:
		return 1 // 分隔线粗细
	case theme.SizeNameWindowButtonSize:
		return 28
	case theme.SizeNameTitleBarHeight:
		return 30
	}
	return theme.DefaultTheme().Size(name)
}