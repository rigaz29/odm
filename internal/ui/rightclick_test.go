//go:build fyne

package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"odm/internal/engine"
)

// rightClick right-clicks the list's cell for row as the pointer would.
func rightClick(t *testing.T, a *application, row int) {
	t.Helper()
	c, ok := a.table.CreateCell().(*tableCell)
	if !ok {
		t.Fatal("the list's cells are not tableCells")
	}
	a.updateCell(widget.TableCellID{Row: row, Col: 0}, c)
	c.TappedSecondary(&fyne.PointEvent{AbsolutePosition: fyne.NewPos(20, 60)})
}

func selectedIDs(a *application) map[string]bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := map[string]bool{}
	for id := range a.selected {
		out[id] = true
	}
	return out
}

// A right-click works on the row under the pointer at once: it used to
// open the menu for whatever was selected before, so a row had to be
// clicked first.
func TestRightClickPicksTheRow(t *testing.T) {
	a := newTestApp(t, engine.Settings{})
	for _, name := range []string{"a.zip", "b.zip", "c.zip"} {
		a.mgr.Create(engine.AddOptions{URL: "http://h/" + name, FileName: name})
	}
	a.refresh()
	if len(a.items) != 3 {
		t.Fatalf("%d rows", len(a.items))
	}
	id := func(row int) string { return a.items[row].ID }

	a.clickRow(0, 0)
	rightClick(t, a, 2)
	if sel := selectedIDs(a); len(sel) != 1 || !sel[id(2)] {
		t.Fatalf("selection after right-clicking an unselected row: %v", sel)
	}
	if d, ok := a.current(); !ok || d.ID != id(2) {
		t.Fatalf("current row: %+v", d)
	}
	if a.win.Canvas().Overlays().Top() == nil {
		t.Fatal("no menu opened")
	}
	a.win.Canvas().Overlays().Top().Hide()

	// Within a selection of several, the selection stays for the menu.
	a.clickRow(0, 0)
	a.clickRow(1, fyne.KeyModifierShortcutDefault)
	rightClick(t, a, 1)
	if sel := selectedIDs(a); len(sel) != 2 || !sel[id(0)] || !sel[id(1)] {
		t.Fatalf("selection after right-clicking within it: %v", sel)
	}
	if d, ok := a.current(); !ok || d.ID != id(1) {
		t.Fatalf("current row: %+v", d)
	}
}
