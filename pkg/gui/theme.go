package gui

import (
	_ "embed"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

//go:embed ../../assets/fonts/msyh.ttc
var embeddedFont []byte

// CJKTheme 包含中文支持的主题
type CJKTheme struct {
	regularFont fyne.Resource
	boldFont    fyne.Resource
}

// NewCJKTheme 创建支持中文的主题
func NewCJKTheme() *CJKTheme {
	t := &CJKTheme{}
	t.loadFont()
	return t
}

// loadFont 加载中文字体
func (t *CJKTheme) loadFont() {
	// 优先使用嵌入的字体
	if len(embeddedFont) > 1024 {
		t.regularFont = fyne.NewStaticResource("msyh.ttc", embeddedFont)
		t.boldFont = t.regularFont
		fmt.Printf("[CJKTheme] 使用嵌入字体 (%d bytes)\n", len(embeddedFont))
		return
	}

	// 备选：从系统加载
	if runtime.GOOS != "windows" {
		return
	}

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
			data, err := os.ReadFile(fontPath)
			if err != nil || len(data) < 1024 {
				continue
			}
			res := fyne.NewStaticResource(name, data)
			if t.regularFont == nil {
				t.regularFont = res
				fmt.Printf("[CJKTheme] 系统字体: %s (%d bytes)\n", fontPath, len(data))
			}
			if t.regularFont != nil {
				t.boldFont = t.regularFont
				return
			}
		}
	}

	fmt.Println("[CJKTheme] 警告: 未找到中文字体")
}

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

func (t *CJKTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	return theme.DefaultTheme().Color(name, variant)
}

func (t *CJKTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (t *CJKTheme) Size(name fyne.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(name)
}
