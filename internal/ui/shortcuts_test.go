//go:build fyne

package ui

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"odm/internal/engine"
)

// menuShortcut runs the main-menu item bound to key+mod, as the window does
// before a focused widget sees the keys, and reports whether one was found.
func menuShortcut(a *application, key fyne.KeyName, mod fyne.KeyModifier) bool {
	want := (&desktop.CustomShortcut{KeyName: key, Modifier: mod}).ShortcutName()
	var find func(m *fyne.Menu) bool
	find = func(m *fyne.Menu) bool {
		for _, it := range m.Items {
			if it.Shortcut != nil && it.Shortcut.ShortcutName() == want && it.Action != nil {
				it.Action()
				return true
			}
			if it.ChildMenu != nil && find(it.ChildMenu) {
				return true
			}
		}
		return false
	}
	for _, m := range a.mainMenu.Items {
		if find(m) {
			return true
		}
	}
	return false
}

func TestSpaceTogglesTheSelection(t *testing.T) {
	a := newTestApp(t, engine.Settings{})
	d1 := a.mgr.Create(engine.AddOptions{URL: "http://127.0.0.1:1/a.bin", FileName: "a.bin"})
	d2 := a.mgr.Create(engine.AddOptions{URL: "http://127.0.0.1:1/b.bin", FileName: "b.bin"})
	a.refresh()
	a.selectAll()

	status := func(id string) engine.Status {
		for _, d := range a.mgr.List() {
			if d.ID == id {
				return d.Status
			}
		}
		return ""
	}
	// Both are stopped: Space queues them.
	a.table.TypedKey(&fyne.KeyEvent{Name: fyne.KeySpace})
	a.refresh()
	for _, id := range []string{d1.ID, d2.ID} {
		if s := status(id); s != engine.StatusQueued && s != engine.StatusDownloading && s != engine.StatusError {
			t.Fatalf("after Space %s is %s, want it started", id, s)
		}
	}
	// Started (or failed fast against a closed port and retried): Space
	// again pauses whatever is still running.
	a.mgr.Pause(d1.ID)
	a.mgr.Pause(d2.ID)
	a.refresh()
	if s := status(d1.ID); s != engine.StatusPaused {
		t.Fatalf("setup: %s", s)
	}
	a.table.TypedKey(&fyne.KeyEvent{Name: fyne.KeySpace})
	a.refresh()
	a.table.TypedKey(&fyne.KeyEvent{Name: fyne.KeySpace})
	a.refresh()
	if s := status(d1.ID); s != engine.StatusPaused {
		t.Fatalf("Space twice left %s, want paused", s)
	}
}

// overlayText is the text of the window's top overlay (an open dialog).
func overlayText(a *application) string {
	top := a.win.Canvas().Overlays().Top()
	if top == nil {
		return ""
	}
	var b strings.Builder
	for _, o := range test.LaidOutObjects(top) {
		switch o := o.(type) {
		case *widget.Label:
			b.WriteString(o.Text + "\n")
		case *canvas.Text:
			b.WriteString(o.Text + "\n")
		}
	}
	return b.String()
}

func TestShiftDeleteAsksToDeleteFiles(t *testing.T) {
	a := newTestApp(t, engine.Settings{})
	a.mgr.Create(engine.AddOptions{URL: "http://h/a.bin", FileName: "a.bin"})
	a.refresh()
	a.selectAll()
	a.table.TypedShortcut(&fyne.ShortcutCut{}) // Ctrl+X does nothing here
	if txt := overlayText(a); txt != "" {
		t.Fatalf("Ctrl+X opened %q", txt)
	}
	a.table.TypedShortcut(&fyne.ShortcutCut{Secondary: true})
	if txt := overlayText(a); !strings.Contains(txt, "delete the files") {
		t.Fatalf("Shift+Del asked %q", txt)
	}
}

func TestMenuShortcuts(t *testing.T) {
	a := newTestApp(t, engine.Settings{})
	if !menuShortcut(a, fyne.KeyN, fyne.KeyModifierShortcutDefault) {
		t.Fatal("no menu item has Ctrl+N")
	}
	// Ctrl+A must stay with the focused widget (the search box selects its
	// text), so no menu item may claim it.
	ctrlA := (&fyne.ShortcutSelectAll{}).ShortcutName()
	for _, m := range a.mainMenu.Items {
		for _, it := range m.Items {
			if it.Shortcut != nil && it.Shortcut.ShortcutName() == ctrlA {
				t.Fatalf("%q claims Ctrl+A", it.Label)
			}
		}
	}
}

func TestOneExitItem(t *testing.T) {
	a := newTestApp(t, engine.Settings{})
	quits := 0
	for _, it := range a.mainMenu.Items[0].Items {
		if it.IsQuit {
			quits++
		}
	}
	if quits != 1 {
		t.Fatalf("%d quit items in the Tasks menu, want 1 (or Fyne adds its own)", quits)
	}
}
