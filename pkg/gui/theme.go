package gui

import (
	"image/color"
	"os"
	"path/filepath"
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// CJKTheme 包含中文支持的主题
type CJKTheme struct {
	regularFont fyne.Resource
	boldFont    fyne.Resource
}

// NewCJKTheme 创建支持中文的主题
func NewCJKTheme() *CJKTheme {
	t := &CJKTheme{}
	t.loadSystemFonts()
	return t
}

// loadSystemFonts 加载系统中文字体
func (t *CJKTheme) loadSystemFonts() {
	if runtime.GOOS != "windows" {
		return
	}

	// Windows 系统中文字体路径
	fontPaths := []string{
		// 微软雅黑
		filepath.Join(os.Getenv("WINDIR"), "Fonts", "msyh.ttc"),
		filepath.Join(os.Getenv("WINDIR"), "Fonts", "msyhbd.ttc"),
		// 微软雅黑 (备选路径)
		`C:\Windows\Fonts\msyh.ttc`,
		`C:\Windows\Fonts\msyhbd.ttc`,
	}

	for i, fontPath := range fontPaths {
		data, err := os.ReadFile(fontPath)
		if err != nil {
			continue
		}
		res := &fyne.StaticResource{
			StaticName:    filepath.Base(fontPath),
			StaticContent: data,
		}
		if i%2 == 0 {
			t.regularFont = res
		} else {
			t.boldFont = res
		}
		if t.regularFont != nil && t.boldFont != nil {
			break
		}
	}

	// 如果没找到粗体，用常规字体代替
	if t.boldFont == nil && t.regularFont != nil {
		t.boldFont = t.regularFont
	}
}

// Font 返回字体资源
func (t *CJKTheme) Font(style fyne.TextStyle) fyne.Resource {
	if style.Monospace {
		return theme.DefaultTheme().Font(style)
	}
	if style.Bold {
		if t.boldFont != nil {
			return t.boldFont
		}
	}
	if t.regularFont != nil {
		return t.regularFont
	}
	return theme.DefaultTheme().Font(style)
}

// 以下方法委托给默认主题

func (t *CJKTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	return theme.DefaultTheme().Color(name, variant)
}

func (t *CJKTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (t *CJKTheme) Size(name fyne.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(name)
}
