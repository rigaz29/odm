//go:build fyne

package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"odm/internal/engine"
)

// reopen builds a second main window on a's app, as the next run of ODM
// would find it: same preferences, new state.
func reopen(t *testing.T, a *application) *application {
	t.Helper()
	b := &application{mgr: a.mgr, progressWindows: map[string]*progressView{}, selectedRow: -1, sortCol: -1}
	b.fyneApp = a.fyneApp
	b.win = a.fyneApp.NewWindow("ODM")
	b.build()
	b.refresh()
	return b
}

func TestColumnWidths(t *testing.T) {
	all := visibleColumns(nil)
	const gap = 3
	sum := func(w []float32) (s float32) {
		for _, x := range w {
			s += x + gap
		}
		return s
	}
	// Defaults: the fixed columns keep their widths and File Name and
	// Description fill the rest, 60:40.
	w := columnWidths(all, nil, 1200, gap)
	if got := sum(w); got != 1200-2 {
		t.Fatalf("columns fill %v of 1200", got)
	}
	if w[colStatus] != columns[colStatus].width || w[colName] <= w[colDescription] {
		t.Fatalf("default widths %v", w)
	}
	// A dragged width stays; the sharing columns take up the difference.
	w2 := columnWidths(all, map[int]float32{colStatus: 200}, 1200, gap)
	if w2[colStatus] != 200 || sum(w2) != 1200-2 {
		t.Fatalf("dragged status: %v", w2)
	}
	// With File Name dragged, Description alone fills the rest.
	w3 := columnWidths(all, map[int]float32{colName: 300}, 1200, gap)
	if w3[colName] != 300 || sum(w3) != 1200-2 {
		t.Fatalf("dragged name: %v", w3)
	}
	// Too narrow: the sharing columns keep their minimums.
	w4 := columnWidths(all, nil, 500, gap)
	if w4[colName] != minNameWidth || w4[colDescription] != minDescWidth {
		t.Fatalf("narrow: %v", w4)
	}
	// Hidden Description: File Name takes all of the rest.
	vis := visibleColumns(map[int]bool{colDescription: true})
	w5 := columnWidths(vis, nil, 1200, gap)
	if len(w5) != columnCount-1 || sum(w5) != 1200-2 {
		t.Fatalf("no description: %v", w5)
	}
}

func TestHideColumnAndRemember(t *testing.T) {
	a := newTestApp(t, engine.Settings{})
	a.mgr.Create(engine.AddOptions{URL: "http://h/a.bin", FileName: "a.bin", Queue: "Main"})
	a.refresh()

	a.setColumnShown(colQueue, false)
	a.setColumnShown(colName, false) // refused: the name always shows
	if _, cols := a.table.Length(); cols != columnCount-1 {
		t.Fatalf("%d columns after hiding Queue", cols)
	}
	// The second column is now Size.
	if got := a.headerText(a.visibleColumn(1)); got != "Size" {
		t.Fatalf("second column %q", got)
	}
	cell := newTableCell()
	a.updateCell(widget.TableCellID{Row: 0, Col: 1}, cell)
	if cell.full == "Main" {
		t.Fatal("the hidden Queue column still fills the second cell")
	}
	for _, it := range a.columnsMenu.Items {
		if it.Label == "Queue" && it.Checked {
			t.Fatal("View → Columns still checks Queue")
		}
	}

	b := reopen(t, a)
	if b.columnShown(colQueue) || !b.columnShown(colName) {
		t.Fatalf("after a restart: hidden %v", b.hiddenCols)
	}
	b.resetColumns()
	if !b.columnShown(colQueue) {
		t.Fatal("Reset columns left Queue hidden")
	}
	if c := reopen(t, b); !c.columnShown(colQueue) {
		t.Fatal("the reset was not saved")
	}
}

func TestSortIsRemembered(t *testing.T) {
	a := newTestApp(t, engine.Settings{})
	a.sortBy(colSize)
	a.sortBy(colSize) // descending
	b := reopen(t, a)
	if b.sortCol != colSize || b.sortAsc {
		t.Fatalf("after a restart: sort column %d ascending %v", b.sortCol, b.sortAsc)
	}
	if got := b.headerText(colSize); got != "Size ▼" {
		t.Fatalf("header %q", got)
	}
}

// headerFor is the laid-out header cell of col and its absolute left edge.
func headerFor(t *testing.T, a *application, col int) (*headerLabel, float32) {
	t.Helper()
	drv := fyne.CurrentApp().Driver()
	for _, h := range a.headers {
		if pos := drv.AbsolutePositionForObject(h); h.col == col && !pos.IsZero() {
			return h, pos.X
		}
	}
	t.Fatalf("no header on screen for column %d", col)
	return nil, 0
}

func drag(g *headerGrips, from float32, steps ...float32) {
	x := from
	for _, dx := range steps {
		x += dx
		g.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{AbsolutePosition: fyne.NewPos(x, 10)}, Dragged: fyne.NewDelta(dx, 0)})
	}
	g.DragEnd()
}

func TestDragHeaderBorder(t *testing.T) {
	a := newTestApp(t, engine.Settings{})
	a.win.Resize(fyne.NewSize(1400, 600))
	g := newHeaderGrips(a)

	status, left := headerFor(t, a, colStatus)
	border := left + status.Size().Width
	statusW, sizeW := a.columnWidth(colStatus), a.columnWidth(colSize)
	if a.gripAt(border+2) != colStatus || a.gripAt(left+status.Size().Width/2) != -1 {
		t.Fatal("gripAt does not find the Status border")
	}

	// A click in the middle of a header sorts; on its border it does not.
	status.Tapped(&fyne.PointEvent{AbsolutePosition: fyne.NewPos(border-2, 10)})
	if a.sortCol == colStatus {
		t.Fatal("a click on the border sorted")
	}
	status.Tapped(&fyne.PointEvent{AbsolutePosition: fyne.NewPos(left+20, 10)})
	if a.sortCol != colStatus {
		t.Fatal("a click in the middle did not sort")
	}

	// A drag that starts away from a border does nothing.
	drag(g, left+20, 10, 30)
	if a.columnWidth(colStatus) != statusW || a.columnWidth(colSize) != sizeW {
		t.Fatal("a drag in the middle of a header resized a column")
	}
	// Just left of the Status header is the border of Size.
	drag(g, left-1, -5, -10)
	if got := a.columnWidth(colSize); got != sizeW-15 {
		t.Fatalf("size is %v wide, want %v", got, sizeW-15)
	}
	// The Status border, dragged 30px right in two moves.
	status, left = headerFor(t, a, colStatus)
	drag(g, left+status.Size().Width-1, 10, 20)
	if got := a.columnWidth(colStatus); got != statusW+30 {
		t.Fatalf("status is %v wide after the drag, want %v", got, statusW+30)
	}

	b := reopen(t, a)
	b.layoutColumns(1200)
	if got := b.columnWidth(colStatus); got != statusW+30 {
		t.Fatalf("after a restart status is %v wide, want %v", got, statusW+30)
	}
	if got := b.columnWidth(colSize); got != sizeW-15 {
		t.Fatalf("after a restart size is %v wide, want %v", got, sizeW-15)
	}
}
