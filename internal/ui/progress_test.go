//go:build fyne

package ui

import (
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"

	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"

	"odm/internal/engine"
)

func TestNoticeCompletions(t *testing.T) {
	mgr := engine.NewManager(engine.Settings{Dir: t.TempDir()}, "")
	a := &application{mgr: mgr, progressWindows: map[string]*progressView{}, selectedRow: -1}
	a.fyneApp = test.NewTempApp(t)
	base := len(a.fyneApp.Driver().AllWindows()) // the test driver starts with one
	wins := func() int { return len(a.fyneApp.Driver().AllWindows()) - base }

	old := engine.Info{ID: "old", Status: engine.StatusComplete, FileName: "a"}
	run := engine.Info{ID: "run", Status: engine.StatusDownloading, FileName: "b"}
	a.noticeCompletions([]engine.Info{old, run})
	if wins() != 0 {
		t.Fatalf("first refresh opened %d windows for already-finished downloads", wins())
	}
	closed := false
	pw := a.fyneApp.NewWindow("progress")
	pw.SetOnClosed(func() { closed = true })
	a.progressWindows["run"] = &progressView{win: pw}

	run.Status = engine.StatusComplete
	a.noticeCompletions([]engine.Info{old, run})
	if !closed {
		t.Fatal("progress window not closed on completion")
	}
	if wins() != 1 {
		t.Fatalf("windows = %d, want exactly the completion dialog", wins())
	}
	a.noticeCompletions([]engine.Info{old, run})
	if wins() != 1 {
		t.Fatalf("completion dialog shown again on a later refresh (%d windows)", wins())
	}
}

func TestSelectionFollowsDownloadsAcrossResort(t *testing.T) {
	mgr := engine.NewManager(engine.Settings{Dir: t.TempDir()}, "")
	now := time.Now()
	var states []engine.State
	for i, name := range []string{"d.zip", "b.zip", "a.zip", "c.zip"} {
		states = append(states, engine.State{ID: name, URL: "http://x/" + name, FileName: name,
			Status: engine.StatusPaused, Order: i + 1, CreatedAt: now, UpdatedAt: now})
	}
	mgr.Restore(states)
	a := &application{mgr: mgr, progressWindows: map[string]*progressView{}, selectedRow: -1, sortCol: -1}
	a.fyneApp = test.NewTempApp(t)
	a.fyneApp.Settings().SetTheme(newTheme())
	a.win = a.fyneApp.NewWindow("t")
	a.build()
	a.refresh()

	ids := func() []string {
		var out []string
		for _, d := range a.selectedItems() {
			out = append(out, d.ID)
		}
		return out
	}
	// List order (insertion): d, b, a, c
	a.clickRow(1, 0) // b
	a.clickRow(3, fyne.KeyModifierShift)
	if got := strings.Join(ids(), ","); got != "b.zip,a.zip,c.zip" {
		t.Fatalf("shift range = %s", got)
	}
	a.clickRow(0, fyne.KeyModifierShortcutDefault) // add d, focus moves to d
	if got := strings.Join(ids(), ","); got != "d.zip,b.zip,a.zip,c.zip" {
		t.Fatalf("ctrl add = %s", got)
	}
	a.clickRow(2, fyne.KeyModifierShortcutDefault) // remove a, focus a
	focus, _ := a.current()

	a.sortBy(colName) // re-sort: a, b, c, d
	if got := strings.Join(ids(), ","); got != "b.zip,c.zip,d.zip" {
		t.Fatalf("selection after sort = %s, want the same downloads", got)
	}
	if cur, ok := a.current(); !ok || cur.ID != focus.ID {
		t.Fatalf("focus moved from %s to %+v after sort", focus.ID, cur)
	}
	a.selectAll()
	if len(ids()) != 4 {
		t.Fatalf("select all = %v", ids())
	}
}

// The 1 s refresh must not clear the hover highlight in the sidebar: Fyne's
// List.Refresh resets it on every visible row, so an unchanged sidebar is
// not refreshed. Checked on pixels: the hovered row keeps its colour.
func TestSidebarKeepsHoverOnRefresh(t *testing.T) {
	mgr := engine.NewManager(engine.Settings{Dir: t.TempDir()}, "")
	a := &application{mgr: mgr, progressWindows: map[string]*progressView{}, selectedRow: -1, sortCol: -1}
	a.fyneApp = test.NewTempApp(t)
	a.fyneApp.Settings().SetTheme(newTheme())
	a.win = a.fyneApp.NewWindow("t")
	a.build()
	a.win.Resize(fyne.NewSize(700, 420))
	a.refresh()
	base := a.fyneApp.Driver().AbsolutePositionForObject(a.cats)
	row := base.Add(fyne.NewPos(120, 30)) // second row, right of its label
	test.MoveMouse(a.win.Canvas(), row)
	at := func() (uint32, uint32, uint32) {
		img := a.win.Canvas().Capture()
		s := a.win.Canvas().Scale()
		r, g, b, _ := img.At(int(row.X*s), int(row.Y*s)).RGBA()
		return r, g, b
	}
	r1, g1, b1 := at()
	other := base.Add(fyne.NewPos(120, 120))
	img := a.win.Canvas().Capture()
	s := a.win.Canvas().Scale()
	r0, g0, b0, _ := img.At(int(other.X*s), int(other.Y*s)).RGBA()
	if r1 == r0 && g1 == g0 && b1 == b0 {
		t.Fatal("no hover highlight to begin with")
	}
	a.refresh()
	if r2, g2, b2 := at(); r2 != r1 || g2 != g1 || b2 != b1 {
		t.Fatalf("hover highlight lost on refresh: %v,%v,%v -> %v,%v,%v", r1, g1, b1, r2, g2, b2)
	}
}

func TestSameEntries(t *testing.T) {
	a := []sidebarEntry{{text: "Video (1)", icon: theme.MediaVideoIcon()}, {text: "Audio"}}
	b := []sidebarEntry{{text: "Video (1)", icon: theme.MediaVideoIcon()}, {text: "Audio"}}
	if !sameEntries(a, b) {
		t.Fatal("equal lists reported as different")
	}
	b[0].text = "Video (2)"
	if sameEntries(a, b) {
		t.Fatal("a changed count went unnoticed")
	}
	b[0].text, b[0].icon = "Video (1)", theme.MediaMusicIcon()
	if sameEntries(a, b) {
		t.Fatal("a changed icon went unnoticed")
	}
}

func newListApp(t *testing.T, names ...string) *application {
	t.Helper()
	mgr := engine.NewManager(engine.Settings{Dir: t.TempDir()}, "")
	now := time.Now()
	var states []engine.State
	for i, name := range names {
		states = append(states, engine.State{ID: name, URL: "http://x/" + name, FileName: name,
			Status: engine.StatusPaused, Order: i + 1, CreatedAt: now, UpdatedAt: now})
	}
	mgr.Restore(states)
	a := &application{mgr: mgr, progressWindows: map[string]*progressView{}, selectedRow: -1, sortCol: -1}
	a.fyneApp = test.NewTempApp(t)
	a.fyneApp.Settings().SetTheme(newTheme())
	a.win = a.fyneApp.NewWindow("t")
	a.build()
	a.win.Resize(fyne.NewSize(1000, 400))
	a.refresh()
	return a
}

// The pointer highlights its whole row, not the one cell under it.
func TestListHoverWholeRow(t *testing.T) {
	a := newListApp(t, "a.zip", "b.zip", "c.zip")
	d := a.fyneApp.Driver()
	base := d.AbsolutePositionForObject(a.table)
	a.mu.Lock()
	headH, rowH := a.headH, a.rowH
	a.mu.Unlock()
	gap := a.table.Theme().Size(theme.SizeNamePadding)
	rowY := func(r int) float32 { return headH + gap + float32(r)*(rowH+gap) + rowH/2 }
	pixel := func(x, y float32) [3]uint32 {
		img := a.win.Canvas().Capture()
		s := a.win.Canvas().Scale()
		r, g, b, _ := img.At(int((base.X+x)*s), int((base.Y+y)*s)).RGBA()
		return [3]uint32{r, g, b}
	}
	far := a.colWidths[0] + a.colWidths[1] + a.colWidths[2] + 40 // inside the fourth column
	before := pixel(far, rowY(1))
	test.MoveMouse(a.win.Canvas(), base.Add(fyne.NewPos(30, rowY(1)))) // over the name cell of row 1
	if a.hoverRow != 1 {
		t.Fatalf("hoverRow = %d", a.hoverRow)
	}
	if got := pixel(far, rowY(1)); got == before {
		t.Fatal("another cell of the hovered row is not highlighted")
	}
	if got, want := pixel(far, rowY(2)), before; got != want {
		t.Fatalf("a row not under the pointer is highlighted: %v vs %v", got, want)
	}
}

// A toolbar button lights up under the pointer and looks pushed in while
// it is held.
func TestToolbarHoverAndPress(t *testing.T) {
	a := newListApp(t, "a.zip")
	btn := a.tools["options"]
	pos := a.fyneApp.Driver().AbsolutePositionForObject(btn).Add(fyne.NewPos(btn.Size().Width/2, 10))
	test.MoveMouse(a.win.Canvas(), pos)
	if !btn.hover {
		t.Fatal("toolbar button not hovered by the pointer")
	}
	btn.MouseDown(&desktop.MouseEvent{Button: desktop.MouseButtonPrimary})
	if !btn.pressed {
		t.Fatal("no pressed state")
	}
	btn.MouseUp(&desktop.MouseEvent{Button: desktop.MouseButtonPrimary})
	if btn.pressed {
		t.Fatal("pressed state stuck after release")
	}
	a.tools["delete"].setEnabled(false)
	a.tools["delete"].MouseDown(&desktop.MouseEvent{Button: desktop.MouseButtonPrimary})
	if a.tools["delete"].pressed {
		t.Fatal("a disabled button looks pressed")
	}
}
