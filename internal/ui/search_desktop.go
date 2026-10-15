//go:build fyne

package ui

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"odm/internal/engine"
)

// searchEntry is the filter box next to the toolbar. Escape clears it and
// hands the keyboard back to the list.
type searchEntry struct {
	widget.Entry
	onEscape func()
}

func newSearchEntry(onChanged func(string), onEscape func()) *searchEntry {
	e := &searchEntry{onEscape: onEscape}
	e.ExtendBaseWidget(e)
	e.SetPlaceHolder("Search downloads (Ctrl+F)")
	e.OnChanged = onChanged
	clear := widget.NewButtonWithIcon("", theme.CancelIcon(), func() { e.SetText("") })
	clear.Importance = widget.LowImportance
	e.ActionItem = clear
	return e
}

func (e *searchEntry) TypedKey(ev *fyne.KeyEvent) {
	if ev.Name == fyne.KeyEscape {
		e.SetText("")
		if e.onEscape != nil {
			e.onEscape()
		}
		return
	}
	e.Entry.TypedKey(ev)
}

// searchTerms splits a query into lower-case words. Every word has to match,
// so "ubuntu iso" finds "ubuntu-24.04-desktop-amd64.iso".
func searchTerms(q string) []string {
	return strings.Fields(strings.ToLower(q))
}

// matchesSearch reports whether every term occurs in the download's file
// name, address or description.
func matchesSearch(d engine.Info, terms []string) bool {
	if len(terms) == 0 {
		return true
	}
	hay := strings.ToLower(d.FileName + "\n" + d.URL + "\n" + d.Description)
	for _, t := range terms {
		if !strings.Contains(hay, t) {
			return false
		}
	}
	return true
}

// focusSearch moves the keyboard to the search box with its text selected,
// so typing replaces the previous query.
func (a *application) focusSearch() {
	if a.search == nil {
		return
	}
	a.win.Canvas().Focus(a.search)
	a.search.TypedShortcut(&fyne.ShortcutSelectAll{})
}

func (a *application) setSearch(q string) {
	a.mu.Lock()
	a.query = searchTerms(q)
	a.mu.Unlock()
	a.refresh()
}
