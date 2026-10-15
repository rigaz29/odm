//go:build fyne

package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"odm/internal/engine"
)

func TestMatchesSearch(t *testing.T) {
	d := engine.Info{
		FileName:    "ubuntu-24.04-desktop-amd64.iso",
		URL:         "https://releases.example.com/24.04/ubuntu.iso",
		Description: "Work laptop",
	}
	cases := []struct {
		q    string
		want bool
	}{
		{"", true},
		{"ubuntu", true},
		{"UBUNTU ISO", true},       // case does not matter, every word must occur
		{"releases.example", true}, // the address counts
		{"laptop", true},           // and the description
		{"ubuntu fedora", false},
		{"debian", false},
	}
	for _, c := range cases {
		if got := matchesSearch(d, searchTerms(c.q)); got != c.want {
			t.Errorf("search %q = %v, want %v", c.q, got, c.want)
		}
	}
}

func TestDisplayName(t *testing.T) {
	cases := []struct {
		d    engine.Info
		want string
	}{
		{engine.Info{FileName: "a.zip", URL: "http://h/b.zip"}, "a.zip"},
		{engine.Info{URL: "http://h/dir/missing-file.zip?x=1"}, "missing-file.zip"},
		{engine.Info{URL: "http://h/"}, "h"},
		{engine.Info{URL: "not a url\x7f"}, "not a url\x7f"},
	}
	for _, c := range cases {
		if got := displayName(c.d); got != c.want {
			t.Errorf("displayName(%+v) = %q, want %q", c.d, got, c.want)
		}
	}
}

func TestSearchFiltersTheList(t *testing.T) {
	mgr := engine.NewManager(engine.Settings{Dir: t.TempDir()}, "")
	mgr.Create(engine.AddOptions{URL: "http://h/report-2026.pdf", FileName: "report-2026.pdf"})
	mgr.Create(engine.AddOptions{URL: "http://h/holiday.mp4", FileName: "holiday.mp4"})
	mgr.Create(engine.AddOptions{URL: "http://h/report-2025.pdf", FileName: "report-2025.pdf"})

	a := &application{mgr: mgr, progressWindows: map[string]*progressView{}, selectedRow: -1, sortCol: -1}
	a.fyneApp = test.NewTempApp(t)
	a.fyneApp.Settings().SetTheme(newTheme()) // knows the toolbar's colours
	a.win = a.fyneApp.NewWindow("ODM")
	a.build()

	a.search.SetText("report")
	if n := len(a.items); n != 2 {
		t.Fatalf("%d rows for %q, want 2", n, "report")
	}
	if got := a.status.Text; got != "2 of 3 items match the search" {
		t.Fatalf("status %q", got)
	}

	// Escape clears the search and shows every download again.
	a.win.Canvas().Focus(a.search)
	a.search.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
	if a.search.Text != "" || len(a.items) != 3 {
		t.Fatalf("after Escape: text %q, %d rows", a.search.Text, len(a.items))
	}

	// Ctrl+F puts the keyboard in the search box.
	a.win.Canvas().Unfocus()
	if !menuShortcut(a, fyne.KeyF, fyne.KeyModifierShortcutDefault) {
		t.Fatal("no menu item has Ctrl+F")
	}
	if a.win.Canvas().Focused() != a.search {
		t.Fatal("Ctrl+F did not focus the search box")
	}
}
