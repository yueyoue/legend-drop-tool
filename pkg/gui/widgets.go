package gui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// ── 圆角卡片容器 ────────────────────────────────────────────

// Card 是一个带圆角背景 + 内边距的容器
type Card struct {
	widget.BaseWidget
	content  fyne.CanvasObject
	bgColor  color.Color
	radius   float32
	padding  float32
}

// NewCard 创建圆角卡片
func NewCard(content fyne.CanvasObject, bgColor color.Color, radius, padding float32) *Card {
	c := &Card{
		content: content,
		bgColor: bgColor,
		radius:  radius,
		padding: padding,
	}
	c.ExtendBaseWidget(c)
	return c
}

func (c *Card) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(c.bgColor)
	bg.CornerRadius = c.radius
	return &cardRenderer{
		card: c,
		bg:   bg,
		objs: []fyne.CanvasObject{bg, c.content},
	}
}

type cardRenderer struct {
	card *Card
	bg   *canvas.Rectangle
	objs []fyne.CanvasObject
}

func (r *cardRenderer) Layout(size fyne.Size) {
	pad := r.card.padding
	r.bg.Resize(size)
	r.card.content.Resize(fyne.NewSize(size.Width-2*pad, size.Height-2*pad))
	r.card.content.Move(fyne.NewPos(pad, pad))
}

func (r *cardRenderer) MinSize() fyne.Size {
	inner := r.card.content.MinSize()
	pad := r.card.padding
	return fyne.NewSize(inner.Width+2*pad, inner.Height+2*pad)
}

func (r *cardRenderer) Objects() []fyne.CanvasObject {
	return r.objs
}

func (r *cardRenderer) Refresh() {
	r.bg.FillColor = r.card.bgColor
	r.bg.CornerRadius = r.card.radius
	r.bg.Refresh()
	r.Layout(r.card.Size())
}

func (r *cardRenderer) Destroy() {}

// ── 带标题的卡片 ────────────────────────────────────────────

// TitledCard 带标题头的卡片
type TitledCard struct {
	widget.BaseWidget
	title    string
	content  fyne.CanvasObject
	bgColor  color.Color
	hdrColor color.Color
	titleColor color.Color
	radius   float32
	padding  float32
}

// NewTitledCard 创建带标题的卡片
func NewTitledCard(title string, content fyne.CanvasObject, bgColor, hdrColor, titleColor color.Color, radius, padding float32) *TitledCard {
	tc := &TitledCard{
		title:      title,
		content:    content,
		bgColor:    bgColor,
		hdrColor:   hdrColor,
		titleColor: titleColor,
		radius:     radius,
		padding:    padding,
	}
	tc.ExtendBaseWidget(tc)
	return tc
}

func (tc *TitledCard) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(tc.bgColor)
	bg.CornerRadius = tc.radius

	titleLabel := widget.NewLabel(tc.title)
	titleLabel.TextStyle = fyne.TextStyle{Bold: true}

	titleBg := canvas.NewRectangle(tc.hdrColor)
	titleBg.CornerRadius = tc.radius

	titleBar := container.NewStack(titleBg, titleLabel)
	content := container.NewBorder(titleBar, nil, nil, nil, tc.content)

	return &cardRenderer{
		card: &Card{content: content, padding: 0},
		bg:   bg,
		objs: []fyne.CanvasObject{bg, content},
	}
}

// ── 圆角按钮（用 Canvas 绘制）────────────────────────────────

// RoundedBtn 圆角按钮
type RoundedBtn struct {
	widget.BaseWidget
	label      string
	bgColor    color.Color
	textColor  color.Color
	hoverColor color.Color
	radius     float32
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
		radius:     radius,
		onTapped:   onTapped,
	}
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
	bg := canvas.NewRectangle(b.bgColor)
	bg.CornerRadius = b.radius
	label := canvas.NewText(b.label, b.textColor)
	label.TextStyle = fyne.TextStyle{Bold: true}
	label.TextSize = 12
	label.Alignment = fyne.TextAlignCenter
	return &roundedBtnRenderer{btn: b, bg: bg, label: label, objs: []fyne.CanvasObject{bg, label}}
}

type roundedBtnRenderer struct {
	btn   *RoundedBtn
	bg    *canvas.Rectangle
	label *canvas.Text
	objs  []fyne.CanvasObject
}

func (r *roundedBtnRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	r.label.Resize(size)
	pad := (size.Height - float32(r.label.TextSize)) / 2
	r.label.Move(fyne.NewPos(0, pad-2))
}

func (r *roundedBtnRenderer) MinSize() fyne.Size {
	return fyne.NewSize(80, 30)
}

func (r *roundedBtnRenderer) Objects() []fyne.CanvasObject {
	return r.objs
}

func (r *roundedBtnRenderer) Refresh() {
	if r.btn.hovered {
		r.bg.FillColor = r.btn.hoverColor
	} else {
		r.bg.FillColor = r.btn.bgColor
	}
	r.label.Color = r.btn.textColor
	r.bg.Refresh()
	r.label.Refresh()
	r.Layout(r.btn.Size())
}

func (r *roundedBtnRenderer) Destroy() {}

// ── 辅助：快速创建 HBox 卡片行 ──────────────────────────────

// CardHBox 水平排列多个子元素，整体包在一个圆角背景里
func CardHBox(bgColor color.Color, radius, padding float32, objects ...fyne.CanvasObject) *Card {
	inner := container.NewHBox(objects...)
	return NewCard(inner, bgColor, radius, padding)
}

// CardVBox 垂直排列多个子元素，整体包在一个圆角背景里
func CardVBox(bgColor color.Color, radius, padding float32, objects ...fyne.CanvasObject) *Card {
	inner := container.NewVBox(objects...)
	return NewCard(inner, bgColor, radius, padding)
}