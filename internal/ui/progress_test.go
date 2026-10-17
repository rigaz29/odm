//go:build fyne

package ui

import (
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"

	"fyne.io/fyne/v2/test"

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
