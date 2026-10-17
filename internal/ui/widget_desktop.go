//go:build fyne

package ui

import (
	"image/color"
	"strings"
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
	pressed  bool // the mouse button is down on it
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
		b.hover, b.pressed = false, false
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

func (b *toolButton) MouseOut() { b.hover, b.pressed = false, false; b.Refresh() }

// MouseDown and MouseUp show the button pushed in while it is held, like a
// native toolbar; the action runs on Tapped (released over the button).
func (b *toolButton) MouseDown(e *desktop.MouseEvent) {
	if !b.disabled && e.Button == desktop.MouseButtonPrimary {
		b.pressed = true
		b.Refresh()
	}
}

func (b *toolButton) MouseUp(*desktop.MouseEvent) {
	if b.pressed {
		b.pressed = false
		b.Refresh()
	}
}

type toolButtonRenderer struct {
	btn     *toolButton
	bg      *canvas.Rectangle
	icon    *canvas.Image
	lines   [2]*canvas.Text // the label, on one line or two
	objects []fyne.CanvasObject
}

// toolLabelMax is the widest label kept on one line. A longer one with a
// space, as translations often are, goes on two lines rather than widening
// the toolbar until its last buttons scroll out of the window.
const toolLabelMax = float32(84)

// toolLines splits label at the space nearest its middle when it is wider
// than toolLabelMax.
func toolLines(label string, size float32) []string {
	if !strings.Contains(label, " ") || fyne.MeasureText(label, size, fyne.TextStyle{}).Width <= toolLabelMax {
		return []string{label}
	}
	best, mid := -1, len(label)/2
	for i, r := range label {
		if r == ' ' && (best < 0 || abs(i-mid) < abs(best-mid)) {
			best = i
		}
	}
	return []string{label[:best], label[best+1:]}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func (b *toolButton) CreateRenderer() fyne.WidgetRenderer {
	r := &toolButtonRenderer{btn: b}
	r.bg = canvas.NewRectangle(color.Transparent)
	r.bg.CornerRadius = 6
	r.icon = canvas.NewImageFromResource(b.icon)
	r.icon.FillMode = canvas.ImageFillContain
	r.icon.ScaleMode = canvas.ImageScaleSmooth
	r.objects = []fyne.CanvasObject{r.bg, r.icon}
	for i := range r.lines {
		r.lines[i] = canvas.NewText("", theme.ForegroundColor())
		r.lines[i].Alignment = fyne.TextAlignCenter
		r.lines[i].TextSize = theme.TextSize()
		r.objects = append(r.objects, r.lines[i])
	}
	r.Refresh()
	return r
}

func (r *toolButtonRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	const pad, iconSize = float32(4), float32(30)
	// Held down: the icon shrinks a little and the content sinks a pixel.
	icon, sink := iconSize, float32(0)
	if r.btn.pressed {
		icon, sink = iconSize-2, 1
	}
	r.icon.Move(fyne.NewPos(size.Width/2-icon/2, pad+(iconSize-icon)/2+sink))
	r.icon.Resize(fyne.NewSize(icon, icon))
	top := pad + iconSize + 3 + sink
	lh := r.lines[0].MinSize().Height
	for i, l := range r.lines {
		l.Move(fyne.NewPos(0, top+float32(i)*lh))
		l.Resize(fyne.NewSize(size.Width, lh))
	}
}

func (r *toolButtonRenderer) MinSize() fyne.Size {
	lines := toolLines(r.btn.text, theme.TextSize())
	var w float32 = 64
	for _, l := range lines {
		w = max(w, fyne.MeasureText(l, theme.TextSize(), fyne.TextStyle{}).Width+10)
	}
	lh := fyne.MeasureText("Ag", theme.TextSize(), fyne.TextStyle{}).Height
	return fyne.NewSize(w, 62+float32(len(lines)-1)*lh)
}

func (r *toolButtonRenderer) Refresh() {
	fg := theme.ForegroundColor()
	switch {
	case r.btn.disabled:
		r.bg.FillColor = color.Transparent
		fg = theme.DisabledColor()
	case r.btn.pressed:
		r.bg.FillColor = theme.PressedColor()
	case r.btn.hover:
		r.bg.FillColor = theme.HoverColor()
	default:
		r.bg.FillColor = color.Transparent
	}
	r.icon.Resource = r.btn.icon
	if r.btn.disabled {
		r.icon.Resource = theme.NewDisabledResource(r.btn.icon)
	}
	lines := toolLines(r.btn.text, theme.TextSize())
	for i, l := range r.lines {
		l.Text = ""
		if i < len(lines) {
			l.Text = lines[i]
		}
		l.Color = fg
		l.Refresh()
	}
	r.Layout(r.btn.Size()) // a press moves the icon and the label
	r.bg.Refresh()
	r.icon.Refresh()
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
	// selected paints the selection colour behind the cell.
	selected bool
	// row is the table row the cell shows now (cells are reused as the
	// list scrolls). onSecondary, when set, is told of a right-click on
	// it, so the click can pick its row before a menu opens; onPress of
	// the left button going down, with the modifier keys held then;
	// onDoubleTap of a double click.
	//
	// The cell takes every click on it: Fyne gives a click to the
	// innermost object that handles any kind of click, so once the cell
	// handled right-clicks, left clicks no longer reached the table.
	row         int
	onSecondary func(row int, pos fyne.Position)
	onPress     func(row int, mods fyne.KeyModifier)
	onDoubleTap func(row int)
	// hovered paints the hover colour behind the cell. onHover is told of
	// the pointer entering the cell (its row) or leaving it (-1), so the
	// whole row under the pointer is highlighted, not the one cell Fyne's
	// table marks.
	hovered bool
	onHover func(row int)
}

func (c *tableCell) MouseIn(*desktop.MouseEvent) {
	if c.onHover != nil {
		c.onHover(c.row)
	}
}

func (c *tableCell) MouseMoved(*desktop.MouseEvent) {}

func (c *tableCell) MouseOut() {
	if c.onHover != nil {
		c.onHover(-1)
	}
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
// clicked cell, so every cell of the selected row draws its own background,
// and so does every cell of the row under the pointer (hovered).
func (c *tableCell) setSelected(on, hovered bool) {
	if c.selected != on || c.hovered != hovered {
		c.selected, c.hovered = on, hovered
		c.Refresh()
	}
}

// TappedSecondary passes a right-click on with the cell's row. Without it
// the click reached the table, which knows no row, and the menu acted on
// whatever was selected before.
func (c *tableCell) TappedSecondary(e *fyne.PointEvent) {
	if c.onSecondary != nil {
		c.onSecondary(c.row, e.AbsolutePosition)
	}
}

// MouseDown selects at the press, as Explorer does. A tap is only reported
// after the double-click delay, when Ctrl or Shift may have been let go.
func (c *tableCell) MouseDown(e *desktop.MouseEvent) {
	if c.onPress != nil && e.Button == desktop.MouseButtonPrimary {
		c.onPress(c.row, e.Modifier)
	}
}

func (c *tableCell) MouseUp(*desktop.MouseEvent) {}

func (c *tableCell) DoubleTapped(*fyne.PointEvent) {
	if c.onDoubleTap != nil {
		c.onDoubleTap(c.row)
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
	r.c.bg.FillColor = color.Transparent
	switch {
	case r.c.selected:
		r.c.bg.FillColor = theme.SelectionColor()
	case r.c.hovered:
		r.c.bg.FillColor = theme.HoverColor()
	}
	r.c.bg.Refresh()
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

// headerLabel is a compact table column header: a click sorts by the
// column and a right-click lists the columns to show. Drags on its borders
// are taken by headerGrips, which lies over the header row.
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

// Tapped sorts by the column, unless the click is on a border, where it
// ends a resize.
func (h *headerLabel) Tapped(e *fyne.PointEvent) {
	if h.app != nil && h.app.gripAt(e.AbsolutePosition.X) < 0 {
		h.app.sortBy(h.col)
	}
}

func (h *headerLabel) TappedSecondary(e *fyne.PointEvent) {
	if h.app != nil {
		h.app.showColumnMenu(e.AbsolutePosition)
	}
}

func (h *headerLabel) MouseIn(e *desktop.MouseEvent) {
	h.hover = true
	h.Refresh()
	h.MouseMoved(e)
}

func (h *headerLabel) MouseMoved(e *desktop.MouseEvent) {
	if h.app != nil {
		h.app.hoverGrip = h.app.gripAt(e.AbsolutePosition.X)
	}
}

func (h *headerLabel) MouseOut() {
	h.hover = false
	h.Refresh()
	if h.app != nil {
		h.app.hoverGrip = -1
	}
}

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
	line := canvas.NewRectangle(theme.SeparatorColor())
	line.SetMinSize(fyne.NewSize(1, 1))
	return &headerRenderer{h: h, text: text, bg: bg, line: line, objects: []fyne.CanvasObject{bg, text, line}}
}

func (r *headerRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	pad := cellInset
	// Size the text box to its real line height and centre it; stretching it
	// to the full row height pushed the glyphs below the header and clipped them.
	// Cut with an ellipsis, as the cells are: a long caption (a translation,
	// a narrowed column) would run into the next header.
	r.text.Text = ellipsize(r.h.text, size.Width-pad*2, r.text.TextSize)
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
	r.Layout(r.h.Size()) // sets the text, cut to the width
	r.text.Color = theme.ForegroundColor()
	r.line.FillColor = theme.SeparatorColor()
	r.line.Refresh()
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
		bg := canvas.NewRectangle(theme.Color(theme.ColorNameScrollBarBackground))
		bg.CornerRadius = 2
		fill := canvas.NewRectangle(theme.PrimaryColor())
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
		bg.FillColor = theme.Color(theme.ColorNameScrollBarBackground)
		bg.Move(fyne.NewPos(x, 0))
		bg.Resize(fyne.NewSize(w, size.Height))

		frac := 0.0
		if span := float64(c.End - c.Start + 1); span > 0 {
			frac = float64(c.Done) / span
		}
		if frac >= 1 {
			fill.FillColor = theme.SuccessColor()
		} else {
			fill.FillColor = theme.PrimaryColor()
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
		l := canvas.NewRectangle(theme.SeparatorColor())
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
		// Between columns only, as the table draws them: a line after the
		// last column would stand alone at the right edge.
		x := float32(0)
		for i, w := range widths {
			x += w
			if i < len(widths)-1 {
				line(x+off, top, sep, size.Height-top)
			}
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
	return container.NewStack(newFrameLine(), container.New(layout.NewCustomPaddedLayout(1, 1, 1, 1), content))
}

// frameLine is the border of a framed panel, in the theme's separator
// colour of the moment.
type frameLine struct {
	widget.BaseWidget
}

func newFrameLine() *frameLine {
	f := &frameLine{}
	f.ExtendBaseWidget(f)
	return f
}

func (f *frameLine) CreateRenderer() fyne.WidgetRenderer {
	border := canvas.NewRectangle(color.Transparent)
	border.StrokeWidth = 1
	r := &frameRenderer{border: border}
	r.Refresh()
	return r
}

type frameRenderer struct{ border *canvas.Rectangle }

func (r *frameRenderer) Layout(size fyne.Size) { r.border.Resize(size) }

func (r *frameRenderer) MinSize() fyne.Size { return fyne.NewSize(0, 0) }

func (r *frameRenderer) Refresh() {
	r.border.StrokeColor = theme.SeparatorColor()
	r.border.Refresh()
}

func (r *frameRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.border} }

func (r *frameRenderer) Destroy() {}

var (
	_ desktop.Hoverable      = (*toolButton)(nil)
	_ desktop.Mouseable      = (*toolButton)(nil)
	_ desktop.Hoverable      = (*headerLabel)(nil)
	_ fyne.SecondaryTappable = (*headerLabel)(nil)
	_ desktop.Cursorable     = (*headerGrips)(nil)
	_ fyne.Draggable         = (*headerGrips)(nil)
)

/* ---------- accent swatches ---------- */

// newAccentPicker is a row of round swatches, one per accent colour, with a
// ring around the chosen one and its name beside them. selected reports
// the chosen accent's key.
func newAccentPicker(key string) (row fyne.CanvasObject, selected func() string) {
	if key == "" {
		key = accents[0].key
	}
	name := widget.NewLabel("")
	var swatches []*swatch
	pick := func(k string) {
		key = k
		for _, s := range swatches {
			s.Refresh()
		}
		for _, a := range accents {
			if a.key == k {
				name.SetText(T(a.name))
			}
		}
	}
	objs := make([]fyne.CanvasObject, 0, len(accents)+1)
	for _, a := range accents {
		s := &swatch{key: a.key, color: a.color, chosen: func(k string) bool { return k == key }, pick: pick}
		s.ExtendBaseWidget(s)
		swatches = append(swatches, s)
		objs = append(objs, s)
	}
	pick(key)
	objs = append(objs, name)
	return container.NewHBox(objs...), func() string { return key }
}

// swatch is one round accent colour to click.
type swatch struct {
	widget.BaseWidget
	key    string
	color  color.NRGBA
	chosen func(key string) bool
	pick   func(key string)
}

func (s *swatch) Tapped(*fyne.PointEvent) { s.pick(s.key) }

func (s *swatch) Cursor() desktop.Cursor { return desktop.PointerCursor }

func (s *swatch) CreateRenderer() fyne.WidgetRenderer {
	r := &swatchRenderer{s: s, ring: canvas.NewCircle(color.Transparent), dot: canvas.NewCircle(s.color)}
	r.ring.StrokeWidth = 2
	r.Refresh()
	return r
}

type swatchRenderer struct {
	s         *swatch
	ring, dot *canvas.Circle
}

const swatchSize = float32(24)

func (r *swatchRenderer) Layout(size fyne.Size) {
	d := min(size.Width, size.Height)
	off := fyne.NewPos((size.Width-d)/2, (size.Height-d)/2)
	r.ring.Move(off.AddXY(1, 1))
	r.ring.Resize(fyne.NewSize(d-2, d-2))
	r.dot.Move(off.AddXY(5, 5))
	r.dot.Resize(fyne.NewSize(d-10, d-10))
}

func (r *swatchRenderer) MinSize() fyne.Size { return fyne.NewSize(swatchSize, swatchSize) }

func (r *swatchRenderer) Refresh() {
	r.ring.StrokeColor = color.Transparent
	if r.s.chosen(r.s.key) {
		r.ring.StrokeColor = theme.ForegroundColor()
	}
	r.ring.Refresh()
	r.dot.Refresh()
}

func (r *swatchRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.ring, r.dot} }

func (r *swatchRenderer) Destroy() {}

/* ---------- sidebar row ---------- */

// Sidebar row metrics. Fixed rather than taken from the theme: the rows
// used to follow the theme's padding (through a compact theme override),
// and some rows were laid out with a different theme than others, so
// they sat a few pixels apart and moved when the pointer touched them.
const (
	sideRowHeight = 20
	sideArrowW    = 16
	sideIconW     = 16
	sideGap       = 6
	sideLeft      = 4
)

// sidebarRowLayout places a sidebar row's arrow, icon and name at fixed
// offsets from its indent, and shortens the name to fit.
type sidebarRowLayout struct {
	indent float32
	full   string // the name before shortening
}

func (l *sidebarRowLayout) MinSize([]fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(sideLeft+sideArrowW+sideIconW+sideGap+40, sideRowHeight)
}

func (l *sidebarRowLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	arrow, icon, text := objs[0], objs[1], objs[2].(*canvas.Text)
	x := sideLeft + l.indent
	place := func(o fyne.CanvasObject, x, w float32) {
		o.Move(fyne.NewPos(x, (size.Height-w)/2))
		o.Resize(fyne.NewSize(w, w))
	}
	place(arrow, x, 14)
	x += sideArrowW
	place(icon, x, sideIconW)
	x += sideIconW + sideGap
	text.Text = fitText(l.full, size.Width-x-4, text.TextSize, text.TextStyle)
	ts := fyne.MeasureText(text.Text, text.TextSize, text.TextStyle)
	text.Move(fyne.NewPos(x, (size.Height-ts.Height)/2))
	text.Resize(ts)
	text.Refresh()
}

// fitText shortens s with "…" until it fits in w.
func fitText(s string, w, size float32, style fyne.TextStyle) string {
	if w <= 0 || fyne.MeasureText(s, size, style).Width <= w {
		return s
	}
	r := []rune(s)
	for len(r) > 0 {
		r = r[:len(r)-1]
		if c := string(r) + "…"; fyne.MeasureText(c, size, style).Width <= w {
			return c
		}
	}
	return "…"
}

// newSidebarRow is the list's item template: arrow, icon, name.
func newSidebarRow() *fyne.Container {
	text := canvas.NewText("", theme.Color(theme.ColorNameForeground))
	text.TextSize = theme.TextSize()
	return container.New(&sidebarRowLayout{}, newTapIcon(), widget.NewIcon(nil), text)
}
