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

// loadFont 加载中文字体，优先使用嵌入字体，其次本地文件，最后系统字体
func (t *CJKTheme) loadFont() {
	// 1. 优先使用编译时嵌入的字体
	if len(fonts.NotoSansSC) > 1024 {
		t.regularFont = fyne.NewStaticResource("NotoSansSC.ttf", fonts.NotoSansSC)
		t.boldFont = t.regularFont
		fmt.Printf("[CJKTheme] 使用嵌入字体: NotoSansSC.ttf (%d bytes)\n", len(fonts.NotoSansSC))
		return
	}

	// 2. 尝试从exe同目录加载字体文件
	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)

	localFonts := []string{
		filepath.Join(exeDir, "msyh.ttc"),
		filepath.Join(exeDir, "msyh.ttf"),
		filepath.Join(exeDir, "simhei.ttf"),
		filepath.Join(exeDir, "NotoSansSC.ttf"),
		filepath.Join(exeDir, "fonts", "msyh.ttc"),
		filepath.Join(exeDir, "fonts", "msyh.ttf"),
		filepath.Join(exeDir, "fonts", "simhei.ttf"),
		filepath.Join(exeDir, "fonts", "NotoSansSC.ttf"),
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
				data, err := os.ReadFile(fontPath)
				if err == nil && len(data) > 1024 {
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
