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

// ── 自定义深色主题色板（与 HTML demo 一致）─────────────────────

var (
	// 背景层级
	colorBgPrimary   = color.NRGBA{R: 0x0f, G: 0x11, B: 0x17, A: 0xff} // #0f1117
	colorBgSecondary = color.NRGBA{R: 0x16, G: 0x18, B: 0x22, A: 0xff} // #161822
	colorBgTertiary  = color.NRGBA{R: 0x1c, G: 0x1f, B: 0x2e, A: 0xff} // #1c1f2e
	colorBgCard      = color.NRGBA{R: 0x1e, G: 0x22, B: 0x35, A: 0xff} // #1e2235
	colorBgHover     = color.NRGBA{R: 0x25, G: 0x2a, B: 0x3a, A: 0xff} // #252a3a
	colorBgActive    = color.NRGBA{R: 0x2a, G: 0x30, B: 0x48, A: 0xff} // #2a3048

	// 边框
	colorBorder      = color.NRGBA{R: 0x2a, G: 0x2e, B: 0x42, A: 0xff} // #2a2e42
	colorBorderLight = color.NRGBA{R: 0x35, G: 0x3a, B: 0x52, A: 0xff} // #353a52

	// 文字
	colorTextPrimary   = color.NRGBA{R: 0xe4, G: 0xe6, B: 0xf0, A: 0xff} // #e4e6f0
	colorTextSecondary = color.NRGBA{R: 0x8b, G: 0x90, B: 0xa8, A: 0xff} // #8b90a8
	colorTextMuted     = color.NRGBA{R: 0x5a, G: 0x5f, B: 0x78, A: 0xff} // #5a5f78

	// 强调色
	colorAccent     = color.NRGBA{R: 0xd4, G: 0xa8, B: 0x43, A: 0xff} // #d4a843 金色
	colorAccentDim  = color.NRGBA{R: 0xd4, G: 0xa8, B: 0x43, A: 0x26} // 半透明金
	colorError      = color.NRGBA{R: 0xe0, G: 0x55, B: 0x55, A: 0xff} // #e05555
	colorSuccess    = color.NRGBA{R: 0x4e, G: 0xcb, B: 0x71, A: 0xff} // #4ecb71
	colorInfo       = color.NRGBA{R: 0x5b, G: 0x9d, B: 0xf0, A: 0xff} // #5b9df0

	// 交互
	colorPressed = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x18} // 按下高亮
	colorScroll  = color.NRGBA{R: 0x2a, G: 0x2e, B: 0x42, A: 0x99} // 滚动条
	colorShadow  = color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x66} // 阴影
)

// ── 浅色主题色板（保留系统默认即可，这里做兜底）─────────────────

var lightFallback = map[fyne.ThemeColorName]color.Color{
	theme.ColorNameBackground:     color.NRGBA{R: 0xf5, G: 0xf5, B: 0xf5, A: 0xff},
	theme.ColorNameForeground:     color.NRGBA{R: 0x21, G: 0x21, B: 0x21, A: 0xff},
	theme.ColorNamePrimary:        color.NRGBA{R: 0x21, G: 0x96, B: 0xf3, A: 0xff},
	theme.ColorNameInputBackground: color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
	theme.ColorNameInputBorder:    color.NRGBA{R: 0xe0, G: 0xe0, B: 0xe0, A: 0xff},
	theme.ColorNamePlaceHolder:    color.NRGBA{R: 0x88, G: 0x88, B: 0x88, A: 0xff},
	theme.ColorNameSeparator:      color.NRGBA{R: 0xe0, G: 0xe0, B: 0xe0, A: 0xff},
}

// ── CJKTheme ─────────────────────────────────────────────────

// CJKTheme 包含中文支持 + 自定义深色/浅色主题
type CJKTheme struct {
	regularFont fyne.Resource
	boldFont    fyne.Resource
	variant     fyne.ThemeVariant
}

// NewCJKTheme 创建支持中文的主题
func NewCJKTheme(variant fyne.ThemeVariant) *CJKTheme {
	t := &CJKTheme{variant: variant}
	t.loadFont()
	return t
}

// SetVariant 动态切换主题变体
func (t *CJKTheme) SetVariant(variant fyne.ThemeVariant) {
	t.variant = variant
}

// ── 字体加载 ─────────────────────────────────────────────────

func (t *CJKTheme) loadFont() {
	// 1. 优先使用编译时嵌入的字体
	if len(fonts.ChineseFont) > 1024 {
		t.regularFont = fyne.NewStaticResource("WenQuanYiMicroHei.ttf", fonts.ChineseFont)
		t.boldFont = t.regularFont
		fmt.Printf("[CJKTheme] 使用嵌入字体: WenQuanYiMicroHei.ttf (%d bytes)\n", len(fonts.ChineseFont))
		return
	}

	// 2. 从 exe 同目录加载
	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)
	localFonts := []string{
		filepath.Join(exeDir, "msyh.ttc"),
		filepath.Join(exeDir, "msyh.ttf"),
		filepath.Join(exeDir, "simhei.ttf"),
		filepath.Join(exeDir, "WenQuanYiMicroHei.ttf"),
		filepath.Join(exeDir, "fonts", "msyh.ttc"),
		filepath.Join(exeDir, "fonts", "msyh.ttf"),
		filepath.Join(exeDir, "fonts", "simhei.ttf"),
		filepath.Join(exeDir, "fonts", "WenQuanYiMicroHei.ttf"),
	}
	for _, fontPath := range localFonts {
		if data, err := os.ReadFile(fontPath); err == nil && len(data) > 1024 {
			t.regularFont = fyne.NewStaticResource(filepath.Base(fontPath), data)
			t.boldFont = t.regularFont
			fmt.Printf("[CJKTheme] 本地字体: %s (%d bytes)\n", fontPath, len(data))
			return
		}
	}

	// 3. Windows 系统字体
	if runtime.GOOS == "windows" {
		fontDirs := []string{}
		for _, envVar := range []string{"WINDIR", "windir", "SystemRoot"} {
			if v := os.Getenv(envVar); v != "" {
				fontDirs = append(fontDirs, filepath.Join(v, "Fonts"))
			}
		}
		fontDirs = append(fontDirs, `C:\Windows\Fonts`, `D:\Windows\Fonts`)
		fontNames := []string{"msyh.ttc", "msyhbd.ttc", "simhei.ttf", "simsun.ttc"}
		for _, dir := range fontDirs {
			for _, name := range fontNames {
				fontPath := filepath.Join(dir, name)
				if data, err := os.ReadFile(fontPath); err == nil && len(data) > 1024 {
					t.regularFont = fyne.NewStaticResource(name, data)
					t.boldFont = t.regularFont
					fmt.Printf("[CJKTheme] 系统字体: %s (%d bytes)\n", fontPath, len(data))
					return
				}
			}
		}
	}
	fmt.Println("[CJKTheme] 警告: 未找到中文字体,中文可能显示为方块")
}

// ── fyne.Theme 接口实现 ──────────────────────────────────────

// Font 返回字体资源
func (t *CJKTheme) Font(style fyne.TextStyle) fyne.Resource {
	if style.Monospace {
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

// Color 返回自定义主题颜色
func (t *CJKTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	// 深色主题
	if t.variant == theme.VariantDark {
		return darkColor(name)
	}
	// 浅色主题：优先用自定义色板，兜底用 Fyne 默认
	if c, ok := lightFallback[name]; ok {
		return c
	}
	return theme.DefaultTheme().Color(name, theme.VariantLight)
}

func (t *CJKTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (t *CJKTheme) Size(name fyne.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(name)
}

// ── 深色色板映射 ─────────────────────────────────────────────

func darkColor(name fyne.ThemeColorName) color.Color {
	switch name {
	// 背景
	case theme.ColorNameBackground:
		return colorBgPrimary
	case theme.ColorNameHeaderBackground:
		return colorBgSecondary
	case theme.ColorNameButton:
		return colorBgTertiary
	case theme.ColorNameDisabledButton:
		return colorBgTertiary
	case theme.ColorNameInputBackground:
		return colorBgTertiary
	case theme.ColorNameMenuBackground:
		return colorBgCard
	case theme.ColorNameOverlayBackground:
		return colorBgCard

	// 前景 / 文字
	case theme.ColorNameForeground:
		return colorTextPrimary
	case theme.ColorNameDisabled:
		return colorTextMuted
	case theme.ColorNamePlaceHolder:
		return colorTextMuted

	// 强调
	case theme.ColorNamePrimary:
		return colorAccent
	case theme.ColorNameHyperlink:
		return colorInfo
	case theme.ColorNameFocus:
		return colorAccentDim
	case theme.ColorNameSelection:
		return colorAccentDim
	case theme.ColorNameSuccess:
		return colorSuccess
	case theme.ColorNameError:
		return colorError
	case theme.ColorNameWarning:
		return colorAccent

	// 交互
	case theme.ColorNameHover:
		return colorBgHover
	case theme.ColorNamePressed:
		return colorPressed

	// 边框 / 分隔
	case theme.ColorNameInputBorder:
		return colorBorder
	case theme.ColorNameSeparator:
		return colorBorder

	// 其它
	case theme.ColorNameScrollBar:
		return colorScroll
	case theme.ColorNameShadow:
		return colorShadow
	}

	return colorBgPrimary
}