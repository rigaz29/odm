//go:build fyne

package ui

import (
	"image/color"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// The columns of the download list. Which of them show, the widths the
// user dragged and the sort order are kept in the app's preferences.
const (
	colName = iota
	colQueue
	colSize
	colStatus
	colETA
	colRate
	colLastTry
	colDescription
	columnCount
)

// columns describes each column: its header, its name in the column menu
// (both in English, shown through T), the key it is saved under, and its
// default width. The columns without one
// share the space the others leave.
var columns = [columnCount]struct {
	title, name, key string
	width            float32
}{
	colName:        {markT("File Name"), markT("File name"), "name", 0},
	colQueue:       {markT("Q"), markT("Queue"), "queue", 44},
	colSize:        {markT("Size"), markT("Size"), "size", 80},
	colStatus:      {markT("Status"), markT("Status"), "status", 112},
	colETA:         {markT("Time left"), markT("Time left"), "eta", 78},
	colRate:        {markT("Transfer rate"), markT("Transfer rate"), "rate", 100},
	colLastTry:     {markT("Last attempt"), markT("Last attempt"), "lasttry", 100},
	colDescription: {markT("Description"), markT("Description"), "description", 0},
}

// A dragged column stops shrinking at minColumnWidth. Below their minimums
// the sharing columns stop shrinking too and the list scrolls sideways
// rather than squeezing names into slivers.
const (
	minColumnWidth = 36
	minNameWidth   = 180
	minDescWidth   = 110
)

const (
	prefHiddenColumns = "columns.hidden"
	prefColumnWidths  = "columns.widths"
	prefSortColumn    = "sort.column"
	prefSortAscending = "sort.ascending"
)

func columnByKey(key string) int {
	for i, c := range columns {
		if c.key == key {
			return i
		}
	}
	return -1
}

func visibleColumns(hidden map[int]bool) []int {
	var out []int
	for c := 0; c < columnCount; c++ {
		if !hidden[c] {
			out = append(out, c)
		}
	}
	return out
}

// loadColumns restores the shown columns, their widths and the sort order.
func (a *application) loadColumns() {
	p := a.fyneApp.Preferences()
	a.hiddenCols = map[int]bool{}
	for _, k := range p.StringList(prefHiddenColumns) {
		if c := columnByKey(k); c > colName { // File Name always shows
			a.hiddenCols[c] = true
		}
	}
	a.userWidths = map[int]float32{}
	for _, kv := range p.StringList(prefColumnWidths) {
		k, v, _ := strings.Cut(kv, "=")
		w, err := strconv.ParseFloat(v, 32)
		if c := columnByKey(k); c >= 0 && err == nil && w >= minColumnWidth {
			a.userWidths[c] = float32(w)
		}
	}
	a.visibleCols = visibleColumns(a.hiddenCols)
	a.sortCol = columnByKey(p.String(prefSortColumn)) // -1: added order
	a.sortAsc = p.BoolWithFallback(prefSortAscending, true)
}

func (a *application) saveColumns() {
	a.mu.Lock()
	var hidden, widths []string
	for c := range columns {
		if a.hiddenCols[c] {
			hidden = append(hidden, columns[c].key)
		}
		if w, ok := a.userWidths[c]; ok {
			widths = append(widths, columns[c].key+"="+strconv.Itoa(int(w+0.5)))
		}
	}
	sortKey := ""
	if a.sortCol >= 0 {
		sortKey = columns[a.sortCol].key
	}
	asc := a.sortAsc
	a.mu.Unlock()
	p := a.fyneApp.Preferences()
	p.SetStringList(prefHiddenColumns, hidden)
	p.SetStringList(prefColumnWidths, widths)
	p.SetString(prefSortColumn, sortKey)
	p.SetBool(prefSortAscending, asc)
}

// columnWidths gives the width of each shown column in a list total wide,
// with gap between columns. A width the user dragged is kept; File Name and
// Description share what the others leave unless they were dragged too,
// 60:40 when both show.
func columnWidths(visible []int, user map[int]float32, total, gap float32) []float32 {
	widths := make([]float32, len(visible))
	var used float32
	var flex []int // indexes into visible
	for i, c := range visible {
		if w, ok := user[c]; ok {
			widths[i] = w
		} else if columns[c].width > 0 {
			widths[i] = columns[c].width
		} else {
			flex = append(flex, i)
			continue
		}
		used += widths[i]
	}
	// widget.Table spaces columns by its theme's Padding, between columns
	// only: its content is the widths plus (columns-1) gaps. Leaving the
	// gaps out made the row wider than the view (a horizontal scrollbar,
	// the last column clipped); counting one gap too many, plus slack, left
	// a strip after the last column that the header and the selection did
	// not reach. Half a pixel absorbs float rounding.
	slack := total - float32(max(len(visible)-1, 0))*gap - 0.5 - used
	least := func(i int) float32 {
		if visible[i] == colName {
			return minNameWidth
		}
		return minDescWidth
	}
	switch len(flex) {
	case 1:
		widths[flex[0]] = max(slack, least(flex[0]))
	case 2:
		first := max(slack*0.6, least(flex[0]))
		widths[flex[0]] = first
		widths[flex[1]] = max(slack-first, least(flex[1]))
	}
	return widths
}

// layoutColumns sizes the shown columns for a list total wide.
func (a *application) layoutColumns(total float32) {
	if total <= 0 {
		return
	}
	gap := theme.Padding()
	if a.table != nil {
		gap = a.table.Theme().Size(theme.SizeNamePadding)
	}
	a.mu.Lock()
	a.listWidth = total
	widths := columnWidths(a.visibleCols, a.userWidths, total, gap)
	same := len(a.colWidths) == len(widths)
	for i := 0; same && i < len(widths); i++ {
		same = a.colWidths[i] == widths[i]
	}
	if same {
		a.mu.Unlock()
		return
	}
	a.colWidths = widths
	a.mu.Unlock()
	for i, w := range widths {
		a.table.SetColumnWidth(i, w)
	}
	if a.grid != nil {
		a.grid.Refresh()
	}
}

func (a *application) relayoutColumns() {
	a.mu.Lock()
	total := a.listWidth
	a.mu.Unlock()
	a.layoutColumns(total)
}

// visibleColumn is the column shown at position i of the table.
func (a *application) visibleColumn(i int) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	if i < 0 || i >= len(a.visibleCols) {
		return -1
	}
	return a.visibleCols[i]
}

// columnWidth is the width col has now, or 0 when it is hidden.
func (a *application) columnWidth(col int) float32 {
	a.mu.Lock()
	defer a.mu.Unlock()
	for i, c := range a.visibleCols {
		if c == col && i < len(a.colWidths) {
			return a.colWidths[i]
		}
	}
	return 0
}

// columnBefore is the shown column left of col, or -1.
func (a *application) columnBefore(col int) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	for i, c := range a.visibleCols {
		if c == col && i > 0 {
			return a.visibleCols[i-1]
		}
	}
	return -1
}

// resizeColumn gives col the width w, as dragged by the user.
func (a *application) resizeColumn(col int, w float32) {
	a.mu.Lock()
	a.userWidths[col] = max(w, minColumnWidth)
	a.mu.Unlock()
	a.relayoutColumns()
}

func (a *application) columnShown(col int) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return !a.hiddenCols[col]
}

// setColumnShown shows or hides col. File Name always shows.
func (a *application) setColumnShown(col int, on bool) {
	if col <= colName || col >= columnCount {
		return
	}
	a.mu.Lock()
	a.hiddenCols[col] = !on
	a.visibleCols = visibleColumns(a.hiddenCols)
	a.mu.Unlock()
	a.columnsChanged()
}

// resetColumns shows every column at its default width.
func (a *application) resetColumns() {
	a.mu.Lock()
	a.hiddenCols = map[int]bool{}
	a.userWidths = map[int]float32{}
	a.visibleCols = visibleColumns(a.hiddenCols)
	a.mu.Unlock()
	a.columnsChanged()
}

func (a *application) columnsChanged() {
	a.relayoutColumns()
	a.table.Refresh()
	if a.grid != nil {
		a.grid.Refresh()
	}
	a.saveColumns()
	if a.columnsMenu != nil {
		a.columnsMenu.Items = a.columnMenuItems()
		a.columnsMenu.Refresh()
	}
}

// columnMenuItems lists the columns that can be hidden, checked when they
// show, and Reset columns: View → Columns and the header's right-click menu.
func (a *application) columnMenuItems() []*fyne.MenuItem {
	var items []*fyne.MenuItem
	for c := colName + 1; c < columnCount; c++ {
		c := c
		it := fyne.NewMenuItem(T(columns[c].name), func() { a.setColumnShown(c, !a.columnShown(c)) })
		it.Checked = a.columnShown(c)
		items = append(items, it)
	}
	return append(items, fyne.NewMenuItemSeparator(), fyne.NewMenuItem(T("Reset columns"), a.resetColumns))
}

// headerGrip is how far from a column's right border, on either side, a
// drag resizes the column.
const headerGrip = float32(6)

// gripAt is the column whose right border is within reach of the absolute
// x, or -1. The borders come from the header cells the table has laid out,
// so they follow sideways scrolling; a pooled header, off the canvas, has
// its position reported as the origin and is skipped.
func (a *application) gripAt(x float32) int {
	if a.table == nil {
		return -1
	}
	drv := fyne.CurrentApp().Driver()
	left := drv.AbsolutePositionForObject(a.table).X
	right := left + a.table.Size().Width
	best, bestDist := -1, headerGrip
	for _, h := range a.headers {
		w := h.Size().Width
		if h.col < 0 || w <= 0 || !h.Visible() {
			continue
		}
		pos := drv.AbsolutePositionForObject(h)
		border := pos.X + w
		if pos.IsZero() || border <= left || border > right {
			continue
		}
		d := x - border
		if d < 0 {
			d = -d
		}
		if d <= bestDist {
			best, bestDist = h.col, d
		}
	}
	return best
}

// headerGrips lies over the list's header row and resizes a column when a
// drag starts on its border. widget.Table covers its headers with a layer
// of its own that takes every drag, so the headers never see one. The grips
// are neither tappable nor hoverable: clicks and the pointer still reach
// the headers below.
type headerGrips struct {
	widget.BaseWidget
	app *application
	// The drag in progress: the column it resizes (-1 for a drag that did
	// not start on a border), its starting width and the movement since.
	dragging bool
	col      int
	startW   float32
	moved    float32
}

func newHeaderGrips(a *application) *headerGrips {
	g := &headerGrips{app: a, col: -1}
	g.ExtendBaseWidget(g)
	return g
}

func (g *headerGrips) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(canvas.NewRectangle(color.Transparent))
}

func (g *headerGrips) MinSize() fyne.Size { return fyne.NewSize(0, rowHeight()) }

func (g *headerGrips) Cursor() desktop.Cursor {
	if g.col >= 0 || g.app.hoverGrip >= 0 {
		return desktop.HResizeCursor
	}
	return desktop.DefaultCursor
}

// Dragged follows the summed movement rather than the position, which the
// table's own layout shifts while the column changes.
func (g *headerGrips) Dragged(e *fyne.DragEvent) {
	if !g.dragging {
		g.dragging = true
		g.col = g.app.gripAt(e.AbsolutePosition.X - e.Dragged.DX) // where it started
		g.startW, g.moved = g.app.columnWidth(g.col), 0
	}
	if g.col < 0 {
		return
	}
	g.moved += e.Dragged.DX
	g.app.resizeColumn(g.col, g.startW+g.moved)
}

func (g *headerGrips) DragEnd() {
	if g.col >= 0 {
		g.app.saveColumns()
	}
	g.dragging, g.col = false, -1
}

func (a *application) showColumnMenu(pos fyne.Position) {
	widget.ShowPopUpMenuAtPosition(fyne.NewMenu("", a.columnMenuItems()...), a.win.Canvas(), pos)
}
