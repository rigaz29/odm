//go:build fyne

package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
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

// The cells take left clicks too: once they handled right-clicks, Fyne gave
// them every click, and left clicks no longer selected anything.
var (
	_ desktop.Mouseable      = (*tableCell)(nil)
	_ fyne.DoubleTappable    = (*tableCell)(nil)
	_ fyne.SecondaryTappable = (*tableCell)(nil)
)

// leftPress presses the left button on the list's cell for row, with the
// modifier keys mods held.
func leftPress(t *testing.T, a *application, row int, mods fyne.KeyModifier) *tableCell {
	t.Helper()
	c := a.table.CreateCell().(*tableCell)
	a.updateCell(widget.TableCellID{Row: row, Col: 1}, c)
	c.MouseDown(&desktop.MouseEvent{Button: desktop.MouseButtonPrimary, Modifier: mods})
	return c
}

func TestLeftClickSelects(t *testing.T) {
	a := newTestApp(t, engine.Settings{})
	for _, name := range []string{"a.zip", "b.zip", "c.zip", "d.zip", "e.zip"} {
		a.mgr.Create(engine.AddOptions{URL: "http://h/" + name, FileName: name})
	}
	a.refresh()
	id := func(row int) string { return a.items[row].ID }
	want := func(step string, rows ...int) {
		t.Helper()
		sel := selectedIDs(a)
		ok := len(sel) == len(rows)
		for _, r := range rows {
			ok = ok && sel[id(r)]
		}
		if !ok {
			t.Fatalf("%s: selected %v, want rows %v", step, sel, rows)
		}
	}
	ctrl, shift := fyne.KeyModifierShortcutDefault, fyne.KeyModifierShift

	leftPress(t, a, 1, 0)
	want("click", 1)
	if d, ok := a.current(); !ok || d.ID != id(1) {
		t.Fatalf("current row: %+v", d)
	}
	if a.win.Canvas().Focused() != a.table {
		t.Fatal("the list did not take the keyboard focus")
	}
	leftPress(t, a, 3, ctrl)
	want("Ctrl+click", 1, 3)
	leftPress(t, a, 1, ctrl)
	want("Ctrl+click on a selected row", 3)

	leftPress(t, a, 0, 0)
	leftPress(t, a, 2, shift)
	want("Shift+click", 0, 1, 2)
	leftPress(t, a, 1, shift)
	want("Shift+click from the same anchor", 0, 1)

	leftPress(t, a, 4, ctrl)
	leftPress(t, a, 2, ctrl|shift)
	want("Ctrl+Shift+click", 0, 1, 2, 3, 4)

	// The right button only readies the menu (see TestRightClickPicksTheRow).
	c := leftPress(t, a, 0, 0)
	c.MouseDown(&desktop.MouseEvent{Button: desktop.MouseButtonSecondary})
	want("right button down", 0)

	// A double click opens the row's progress window.
	c = leftPress(t, a, 3, 0)
	c.DoubleTapped(&fyne.PointEvent{})
	if a.progressWindows[id(3)] == nil {
		t.Fatal("no progress window after a double click")
	}
}
