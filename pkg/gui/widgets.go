package gui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// ── 带背景色的容器（用 canvas.Rectangle 做背景）────────────

// BGStack 在内容背后叠一层有颜色的矩形背景
type BGStack struct {
	widget.BaseWidget
	bgColor color.Color
	content fyne.CanvasObject
}

func NewBGStack(bgColor color.Color, content fyne.CanvasObject) *BGStack {
	s := &BGStack{bgColor: bgColor, content: content}
	s.ExtendBaseWidget(s)
	return s
}

func (s *BGStack) SetBGColor(c color.Color) {
	s.bgColor = c
	s.Refresh()
}

func (s *BGStack) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(s.bgColor)
	return &bgStackRenderer{s: s, bg: bg, objs: []fyne.CanvasObject{bg, s.content}}
}

type bgStackRenderer struct {
	s    *BGStack
	bg   *canvas.Rectangle
	objs []fyne.CanvasObject
}

func (r *bgStackRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	r.s.content.Resize(size)
	r.s.content.Move(fyne.NewPos(0, 0))
}

func (r *bgStackRenderer) MinSize() fyne.Size {
	return r.s.content.MinSize()
}

func (r *bgStackRenderer) Objects() []fyne.CanvasObject {
	return r.objs
}

func (r *bgStackRenderer) Refresh() {
	r.bg.FillColor = r.s.bgColor
	canvas.Refresh(r.bg)
	r.Layout(r.s.Size())
}

func (r *bgStackRenderer) Destroy() {}

// ── 圆角按钮 ────────────────────────────────────────────────

type RoundedBtn struct {
	widget.BaseWidget
	label      string
	bg         *canvas.Rectangle
	textObj    *canvas.Text
	bgColor    color.Color
	textColor  color.Color
	hoverColor color.Color
	onTapped   func()
	hovered    bool
}

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
	b.ExtendBaseWidget(b)
	return b
}

func (b *RoundedBtn) Tapped(_ *fyne.PointEvent) {
	if b.onTapped != nil {
		b.onTapped()
	}
}

func (b *RoundedBtn) MouseIn(_ *fyne.PointEvent) {
	b.hovered = true
	b.Refresh()
}

func (b *RoundedBtn) MouseOut() {
	b.hovered = false
	b.Refresh()
}

func (b *RoundedBtn) MouseMoved(_ *fyne.PointEvent) {}

func (b *RoundedBtn) CreateRenderer() fyne.WidgetRenderer {
	return &roundedBtnRenderer{btn: b, objs: []fyne.CanvasObject{b.bg, b.textObj}}
}

type roundedBtnRenderer struct {
	btn  *RoundedBtn
	objs []fyne.CanvasObject
}

func (r *roundedBtnRenderer) Layout(size fyne.Size) {
	r.btn.bg.Resize(size)
	r.btn.textObj.Resize(size)
	pad := (size.Height - float32(r.btn.textObj.TextSize)) / 2
	r.btn.textObj.Move(fyne.NewPos(0, pad-2))
}

func (r *roundedBtnRenderer) MinSize() fyne.Size {
	return fyne.NewSize(80, 30)
}

func (r *roundedBtnRenderer) Objects() []fyne.CanvasObject {
	return r.objs
}

func (r *roundedBtnRenderer) Refresh() {
	if r.btn.hovered {
		r.btn.bg.FillColor = r.btn.hoverColor
	} else {
		r.btn.bg.FillColor = r.btn.bgColor
	}
	r.btn.textObj.Color = r.btn.textColor
	canvas.Refresh(r.btn.bg)
	canvas.Refresh(r.btn.textObj)
	r.Layout(r.btn.Size())
}

func (r *roundedBtnRenderer) Destroy() {}

// ── 辅助：快速创建带背景的 HBox/VBox ─────────────────────────

func CardHBox(bgColor color.Color, padding float32, objects ...fyne.CanvasObject) fyne.CanvasObject {
	inner := container.NewHBox(objects...)
	if padding > 0 {
		inner = container.NewPadded(inner)
	}
	return NewBGStack(bgColor, inner)
}

func CardVBox(bgColor color.Color, padding float32, objects ...fyne.CanvasObject) fyne.CanvasObject {
	inner := container.NewVBox(objects...)
	if padding > 0 {
		inner = container.NewPadded(inner)
	}
	return NewBGStack(bgColor, inner)
}