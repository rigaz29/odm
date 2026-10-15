//go:build fyne

package ui

import (
	"image/color"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"odm/internal/engine"
)

/* ---------- toolbar button ---------- */

// toolButton is an icon-above-label toolbar button with a hover highlight,
// matching the classic download-manager toolbar.
type toolButton struct {
	widget.BaseWidget
	icon     fyne.Resource
	text     string
	onTap    func()
	hover    bool
	disabled bool
}

// setEnabled greys the button out when its action does not apply to the
// current selection, as classic download-manager toolbars do.
func (b *toolButton) setEnabled(on bool) {
	if b.disabled == !on {
		return
	}
	b.disabled = !on
	if b.disabled {
		b.hover = false
	}
	b.Refresh()
}

func newToolButton(icon fyne.Resource, text string, onTap func()) *toolButton {
	b := &toolButton{icon: icon, text: text, onTap: onTap}
	b.ExtendBaseWidget(b)
	return b
}

func (b *toolButton) Tapped(*fyne.PointEvent) {
	if b.onTap != nil && !b.disabled {
		b.onTap()
	}
}

// MouseIn, MouseMoved and MouseOut make the button desktop.Hoverable; with
// *fyne.PointEvent arguments they were never called and nothing lit up.
func (b *toolButton) MouseIn(*desktop.MouseEvent) {
	if !b.disabled {
		b.hover = true
		b.Refresh()
	}
}

func (b *toolButton) MouseMoved(*desktop.MouseEvent) {}

func (b *toolButton) MouseOut() { b.hover = false; b.Refresh() }

type toolButtonRenderer struct {
	btn     *toolButton
	bg      *canvas.Rectangle
	icon    *canvas.Image
	label   *canvas.Text
	objects []fyne.CanvasObject
}

func (b *toolButton) CreateRenderer() fyne.WidgetRenderer {
	r := &toolButtonRenderer{btn: b}
	r.bg = canvas.NewRectangle(color.Transparent)
	r.bg.CornerRadius = 6
	r.icon = canvas.NewImageFromResource(b.icon)
	r.icon.FillMode = canvas.ImageFillContain
	r.icon.ScaleMode = canvas.ImageScaleSmooth
	r.label = canvas.NewText(b.text, theme.ForegroundColor())
	r.label.Alignment = fyne.TextAlignCenter
	r.label.TextSize = theme.TextSize()
	r.objects = []fyne.CanvasObject{r.bg, r.icon, r.label}
	r.Refresh()
	return r
}

func (r *toolButtonRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	const pad, iconSize = float32(4), float32(30)
	r.icon.Move(fyne.NewPos(size.Width/2-iconSize/2, pad))
	r.icon.Resize(fyne.NewSize(iconSize, iconSize))
	top := pad + iconSize + 3
	r.label.Move(fyne.NewPos(0, top))
	r.label.Resize(fyne.NewSize(size.Width, size.Height-top-pad))
}

func (r *toolButtonRenderer) MinSize() fyne.Size {
	w := fyne.MeasureText(r.btn.text, theme.TextSize(), fyne.TextStyle{}).Width + 14
	if w < 64 {
		w = 64
	}
	return fyne.NewSize(w, 62)
}

func (r *toolButtonRenderer) Refresh() {
	switch {
	case r.btn.disabled:
		r.bg.FillColor = color.Transparent
		r.label.Color = theme.DisabledColor()
	case r.btn.hover:
		r.bg.FillColor = theme.HoverColor()
		r.label.Color = theme.ForegroundColor()
	default:
		r.bg.FillColor = color.Transparent
		r.label.Color = theme.ForegroundColor()
	}
	r.icon.Resource = r.btn.icon
	if r.btn.disabled {
		r.icon.Resource = theme.NewDisabledResource(r.btn.icon)
	}
	r.label.Text = r.btn.text
	r.bg.Refresh()
	r.icon.Refresh()
	r.label.Refresh()
}

func (r *toolButtonRenderer) Objects() []fyne.CanvasObject { return r.objects }

func (r *toolButtonRenderer) Destroy() {}

/* ---------- table cell ---------- */

// cellInset is the text inset shared by cells and headers so they line up.
const cellInset = float32(5)

// rowHeight is the fixed height of a list row. Cells draw their own text
// rather than wrapping widget.Label: the label's padding comes from whichever
// theme the cell resolves when the table happens to create it, which made
// row heights differ between rows and broke the ruled grid.
func rowHeight() float32 {
	return fyne.MeasureText("Ag", theme.TextSize(), fyne.TextStyle{}).Height + 6
}

// tableCell is one list cell: selection background, optional file-type icon
// and a single line of text cut with an ellipsis to fit the column.
type tableCell struct {
	widget.BaseWidget
	bg      *canvas.Rectangle
	icon    *canvas.Image
	text    *canvas.Text
	full    string
	hasIcon bool
}

func newTableCell() *tableCell {
	c := &tableCell{
		bg:   canvas.NewRectangle(color.Transparent),
		icon: canvas.NewImageFromResource(theme.FileIcon()),
		text: canvas.NewText("", theme.ForegroundColor()),
	}
	c.icon.FillMode = canvas.ImageFillContain
	c.text.TextSize = theme.TextSize()
	c.ExtendBaseWidget(c)
	return c
}

func (c *tableCell) setText(s string) {
	if c.full != s {
		c.full = s
		c.Refresh()
	}
}

func (c *tableCell) setIcon(res fyne.Resource) {
	if c.icon.Resource != res {
		c.icon.Resource = res
		c.icon.Refresh()
	}
}

func (c *tableCell) showIcon(on bool) {
	if c.hasIcon != on {
		c.hasIcon = on
		c.Refresh()
	}
}

// setSelected paints the whole row: widget.Table only highlights the one
// clicked cell, so every cell of the selected row draws its own background.
func (c *tableCell) setSelected(on bool) {
	want := color.Color(color.Transparent)
	if on {
		want = odmPalette.accentSoft
	}
	if c.bg.FillColor != want {
		c.bg.FillColor = want
		c.bg.Refresh()
	}
}

func (c *tableCell) CreateRenderer() fyne.WidgetRenderer { return &cellRenderer{c: c} }

type cellRenderer struct{ c *tableCell }

func (r *cellRenderer) Layout(size fyne.Size) {
	c := r.c
	c.bg.Resize(size)
	x := cellInset
	const iconSize = float32(16)
	if c.hasIcon {
		c.icon.Move(fyne.NewPos(x, (size.Height-iconSize)/2))
		c.icon.Resize(fyne.NewSize(iconSize, iconSize))
		x += iconSize + 5
	}
	c.text.Text = ellipsize(c.full, size.Width-x-cellInset, c.text.TextSize)
	th := c.text.MinSize().Height
	c.text.Move(fyne.NewPos(x, (size.Height-th)/2))
	c.text.Resize(fyne.NewSize(size.Width-x-cellInset, th))
}

func (r *cellRenderer) MinSize() fyne.Size { return fyne.NewSize(20, rowHeight()) }

func (r *cellRenderer) Refresh() {
	r.c.text.Color = theme.ForegroundColor()
	r.Layout(r.c.Size())
	r.c.text.Refresh()
	canvas.Refresh(r.c)
}

func (r *cellRenderer) Objects() []fyne.CanvasObject {
	if r.c.hasIcon {
		return []fyne.CanvasObject{r.c.bg, r.c.icon, r.c.text}
	}
	return []fyne.CanvasObject{r.c.bg, r.c.text}
}

func (r *cellRenderer) Destroy() {}

// ellipsize shortens s with a trailing "…" so it fits in width.
func ellipsize(s string, width, size float32) string {
	if s == "" || width <= 0 {
		return ""
	}
	measure := func(t string) float32 { return fyne.MeasureText(t, size, fyne.TextStyle{}).Width }
	if measure(s) <= width {
		return s
	}
	runes := []rune(s)
	lo, hi := 0, len(runes)
	for lo < hi { // longest prefix that still fits with the ellipsis
		mid := (lo + hi + 1) / 2
		if measure(string(runes[:mid])+"…") <= width {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	if lo == 0 {
		return ""
	}
	return string(runes[:lo]) + "…"
}

/* ---------- column header ---------- */

// headerLabel is a compact, clickable table column header.
type headerLabel struct {
	widget.BaseWidget
	app   *application
	col   int
	text  string
	hover bool
}

func newHeaderLabel(app *application) *headerLabel {
	h := &headerLabel{app: app}
	h.ExtendBaseWidget(h)
	return h
}

// SetText updates the header caption.
func (h *headerLabel) SetText(s string) {
	if h.text == s {
		return
	}
	h.text = s
	h.Refresh()
}

func (h *headerLabel) Tapped(*fyne.PointEvent) {
	if h.app != nil {
		h.app.sortBy(h.col)
	}
}

func (h *headerLabel) MouseIn(*desktop.MouseEvent) { h.hover = true; h.Refresh() }

func (h *headerLabel) MouseMoved(*desktop.MouseEvent) {}

func (h *headerLabel) MouseOut() { h.hover = false; h.Refresh() }

type headerRenderer struct {
	h       *headerLabel
	text    *canvas.Text
	bg      *canvas.Rectangle
	line    *canvas.Rectangle
	objects []fyne.CanvasObject
}

func (h *headerLabel) CreateRenderer() fyne.WidgetRenderer {
	// Regular weight at body size, like a native list-view header.
	text := canvas.NewText(h.text, theme.ForegroundColor())
	text.TextSize = theme.TextSize()
	bg := canvas.NewRectangle(color.Transparent)
	line := canvas.NewRectangle(odmPalette.border)
	line.SetMinSize(fyne.NewSize(1, 1))
	return &headerRenderer{h: h, text: text, bg: bg, line: line, objects: []fyne.CanvasObject{bg, text, line}}
}

func (r *headerRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	pad := cellInset
	// Size the text box to its real line height and centre it; stretching it
	// to the full row height pushed the glyphs below the header and clipped them.
	th := r.text.MinSize().Height
	r.text.Move(fyne.NewPos(pad, (size.Height-th)/2))
	r.text.Resize(fyne.NewSize(size.Width-pad*2, th))
	r.line.Move(fyne.NewPos(size.Width-1, 0))
	r.line.Resize(fyne.NewSize(1, size.Height))
}

func (r *headerRenderer) MinSize() fyne.Size {
	ts := fyne.MeasureText(r.h.text, theme.TextSize(), fyne.TextStyle{})
	// widget.Table sizes every row to max(cell, header) MinSize, so a taller
	// header would stretch all the data rows too.
	return fyne.NewSize(ts.Width+cellInset*2, rowHeight())
}

func (r *headerRenderer) Refresh() {
	r.text.Text = r.h.text
	if r.h.hover {
		r.bg.FillColor = theme.HoverColor()
	} else {
		r.bg.FillColor = color.Transparent
	}
	r.bg.Refresh()
	r.text.Refresh()
}

func (r *headerRenderer) Objects() []fyne.CanvasObject { return r.objects }

func (r *headerRenderer) Destroy() {}

/* ---------- sidebar indent ---------- */

// indentSpacer reserves horizontal space so nested sidebar rows line up.
type indentSpacer struct {
	widget.BaseWidget
	w float32
}

func newIndentSpacer() *indentSpacer {
	s := &indentSpacer{}
	s.ExtendBaseWidget(s)
	return s
}

func (s *indentSpacer) set(w float32) {
	if s.w == w {
		return
	}
	s.w = w
	s.Refresh()
}

func (s *indentSpacer) MinSize() fyne.Size { return fyne.NewSize(s.w, 1) }

func (s *indentSpacer) CreateRenderer() fyne.WidgetRenderer { return &spacerRenderer{s: s} }

type spacerRenderer struct{ s *indentSpacer }

func (r *spacerRenderer) Layout(fyne.Size) {}

func (r *spacerRenderer) MinSize() fyne.Size { return r.s.MinSize() }

func (r *spacerRenderer) Refresh() {}

func (r *spacerRenderer) Objects() []fyne.CanvasObject { return nil }

func (r *spacerRenderer) Destroy() {}

/* ---------- connection strip ---------- */

// chunkStrip draws one bar per connection, positioned and sized in proportion
// to its byte range ("a per-connection map").
type chunkStrip struct {
	widget.BaseWidget
	mu     sync.Mutex
	chunks []engine.ChunkInfo
	total  int64
}

func newChunkStrip() *chunkStrip {
	s := &chunkStrip{}
	s.ExtendBaseWidget(s)
	return s
}

func (s *chunkStrip) set(chunks []engine.ChunkInfo, total int64) {
	s.mu.Lock()
	s.chunks = chunks
	s.total = total
	s.mu.Unlock()
	s.Refresh()
}

func (s *chunkStrip) CreateRenderer() fyne.WidgetRenderer {
	r := &stripRenderer{strip: s}
	r.rebuild()
	return r
}

type stripRenderer struct {
	strip   *chunkStrip
	objects []fyne.CanvasObject
}

func (r *stripRenderer) rebuild() {
	r.strip.mu.Lock()
	n := len(r.strip.chunks)
	r.strip.mu.Unlock()
	if n < 1 {
		n = 1
	}
	r.objects = make([]fyne.CanvasObject, 0, n*2)
	for i := 0; i < n; i++ {
		bg := canvas.NewRectangle(odmPalette.scrollTrack)
		bg.CornerRadius = 2
		fill := canvas.NewRectangle(odmPalette.accent)
		fill.CornerRadius = 2
		r.objects = append(r.objects, bg, fill)
	}
}

func (r *stripRenderer) Layout(size fyne.Size) {
	r.strip.mu.Lock()
	chunks := append([]engine.ChunkInfo(nil), r.strip.chunks...)
	total := r.strip.total
	r.strip.mu.Unlock()
	if total <= 0 {
		total = 1
	}
	for i, c := range chunks {
		if i*2+1 >= len(r.objects) {
			break
		}
		bg := r.objects[i*2].(*canvas.Rectangle)
		fill := r.objects[i*2+1].(*canvas.Rectangle)
		x := size.Width * float32(float64(c.Start)/float64(total))
		w := size.Width * float32(float64(c.End-c.Start+1)/float64(total))
		if w < 1 {
			w = 1
		}
		bg.Move(fyne.NewPos(x, 0))
		bg.Resize(fyne.NewSize(w, size.Height))

		frac := 0.0
		if span := float64(c.End - c.Start + 1); span > 0 {
			frac = float64(c.Done) / span
		}
		if frac >= 1 {
			fill.FillColor = odmPalette.success
		} else {
			fill.FillColor = odmPalette.accent
		}
		fill.Move(fyne.NewPos(x, 0))
		fill.Resize(fyne.NewSize(w*float32(frac), size.Height))
	}
}

func (r *stripRenderer) MinSize() fyne.Size { return fyne.NewSize(200, 14) }

// Refresh rebuilds the bars when the chunk count changes and always re-lays
// them out: rebuilt rectangles start at zero size, and fill widths move with
// progress, so skipping Layout left the strip blank.
func (r *stripRenderer) Refresh() {
	r.strip.mu.Lock()
	need := len(r.strip.chunks)
	r.strip.mu.Unlock()
	if need*2 != len(r.objects) {
		r.rebuild()
	}
	r.Layout(r.strip.Size())
	for _, o := range r.objects {
		o.Refresh()
	}
}

func (r *stripRenderer) Objects() []fyne.CanvasObject { return r.objects }

func (r *stripRenderer) Destroy() {}

/* ---------- grid backdrop ---------- */

// gridBackdrop continues the table's grid below the last row, so the empty
// part of the list shows ruled rows and column lines like a report view.
// It sits behind the table; where rows exist the table's own separators
// cover the same positions.
type gridBackdrop struct {
	widget.BaseWidget
	app *application
}

func newGridBackdrop(a *application) *gridBackdrop {
	g := &gridBackdrop{app: a}
	g.ExtendBaseWidget(g)
	return g
}

func (g *gridBackdrop) CreateRenderer() fyne.WidgetRenderer { return &gridRenderer{g: g} }

type gridRenderer struct {
	g       *gridBackdrop
	objects []fyne.CanvasObject
	size    fyne.Size
}

func (r *gridRenderer) Layout(size fyne.Size) {
	r.size = size
	a := r.g.app
	a.mu.Lock()
	rows, rowH, headH := len(a.items), a.rowH, a.headH
	widths := append([]float32(nil), a.colWidths...)
	a.mu.Unlock()
	if rowH <= 0 {
		r.objects = nil
		return
	}
	// widget.Table spaces cells by its theme's Padding and centres a 1px
	// divider in that gap; mirror that so the lines continue seamlessly.
	gap := a.table.Theme().Size(theme.SizeNamePadding)
	sep := theme.SeparatorThicknessSize()
	off := (gap - sep) / 2
	var objs []fyne.CanvasObject
	line := func(x, y, w, h float32) {
		l := canvas.NewRectangle(odmPalette.border)
		l.Move(fyne.NewPos(x, y))
		l.Resize(fyne.NewSize(w, h))
		objs = append(objs, l)
	}
	top := headH + gap + float32(rows)*(rowH+gap)
	for y := top - gap + off; y < size.Height; y += rowH + gap {
		line(0, y, size.Width, sep)
	}
	// Column lines only when every column fits: once the table scrolls
	// sideways its offset is unknown here and the lines would drift.
	full := float32(0)
	for _, w := range widths {
		full += w + gap
	}
	if full <= size.Width+gap {
		x := float32(0)
		for _, w := range widths {
			x += w
			line(x+off, top, sep, size.Height-top)
			x += gap
		}
	}
	r.objects = objs
}

func (r *gridRenderer) MinSize() fyne.Size { return fyne.NewSize(0, 0) }

func (r *gridRenderer) Refresh() {
	r.Layout(r.g.Size())
	canvas.Refresh(r.g)
}

func (r *gridRenderer) Objects() []fyne.CanvasObject { return r.objects }

func (r *gridRenderer) Destroy() {}

/* ---------- tappable icon ---------- */

// tapIcon is a small clickable icon, used for the expand arrows in the
// category tree so toggling a node does not also change the filter.
type tapIcon struct {
	widget.Icon
	onTap func()
}

func newTapIcon() *tapIcon {
	t := &tapIcon{}
	t.ExtendBaseWidget(t)
	return t
}

func (t *tapIcon) Tapped(*fyne.PointEvent) {
	if t.onTap != nil {
		t.onTap()
	}
}

/* ---------- panel frame ---------- */

// framed draws a thin border around a panel, like the sunken frames that
// separate the category tree from the list.
func framed(content fyne.CanvasObject) fyne.CanvasObject {
	border := canvas.NewRectangle(color.Transparent)
	border.StrokeColor = odmPalette.border
	border.StrokeWidth = 1
	return container.NewStack(border, container.New(layout.NewCustomPaddedLayout(1, 1, 1, 1), content))
}

var (
	_ desktop.Hoverable = (*toolButton)(nil)
	_ desktop.Hoverable = (*headerLabel)(nil)
)
