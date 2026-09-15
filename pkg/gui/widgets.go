package gui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
)

// ── 带背景色的容器（Stack 叠层，不用自定义 Widget）─────────

// BGBox 在内容背后叠一层有颜色的矩形背景
// 用 container.NewStack 实现，保证渲染可靠
func BGBox(bgColor color.Color, content fyne.CanvasObject) fyne.CanvasObject {
	bg := canvas.NewRectangle(bgColor)
	bg.FillColor = bgColor
	return container.NewMax(bg, content)
}

// BGBoxPadded 带内边距的背景容器
func BGBoxPadded(bgColor color.Color, padding float32, content fyne.CanvasObject) fyne.CanvasObject {
	bg := canvas.NewRectangle(bgColor)
	padded := container.NewPadded(content)
	return container.NewStack(bg, padded)
}

// ── 圆角按钮（保留 canvas.Rectangle.CornerRadius）──────────

// RoundedBtn 圆角按钮
type RoundedBtn struct {
	fyne.CanvasObject
	label      string
	bg         *canvas.Rectangle
	textObj    *canvas.Text
	bgColor    color.Color
	textColor  color.Color
	hoverColor color.Color
	onTapped   func()
	hovered    bool
}

// NewRoundedBtn 创建圆角按钮
func NewRoundedBtn(label string, bgColor, textColor, hoverColor color.Color, radius float32, onTapped func()) *RoundedBtn {
	b := &RoundedBtn{
		label:      label,
		bgColor:    bgColor,
		textColor:  textColor,
		hoverColor: hoverColor,
		onTapped:   onTapped,
	}
	b.bg = canvas.NewRectangle(bgColor)
	b.bg.CornerRadius = radius
	b.textObj = canvas.NewText(label, textColor)
	b.textObj.TextStyle = fyne.TextStyle{Bold: true}
	b.textObj.TextSize = 12
	b.textObj.Alignment = fyne.TextAlignCenter

	// 用 Stack 把背景和文字叠在一起，作为底层 CanvasObject
	b.CanvasObject = container.NewStack(b.bg, b.textObj)
	return b
}

// SetText 更新按钮文字
func (b *RoundedBtn) SetText(text string) {
	b.textObj.Text = text
	b.textObj.Refresh()
}