//go:build fyne

package ui

import (
	"path/filepath"
	"slices"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"odm/internal/engine"
)

// categoryEditor is the category part of Options → Save to: pick a
// category, edit its file types and folder, add or delete categories, or go
// back to the built-in set. Changes stay in defs and dirs until OK.
type categoryEditor struct {
	win    fyne.Window
	defs   []engine.CategoryDef
	dirs   map[string]string
	folder *widget.Entry // the folder of the selected category
	base   *widget.Entry // the default download folder, for the placeholder

	sel                      *widget.Select
	exts                     *widget.Entry
	note                     *widget.Label
	newBtn, delBtn, resetBtn *widget.Button
	cur                      string
}

func newCategoryEditor(win fyne.Window, defs []engine.CategoryDef, dirs map[string]string, folder, base *widget.Entry) *categoryEditor {
	e := &categoryEditor{win: win, defs: defs, dirs: dirs, folder: folder, base: base}
	e.exts = widget.NewEntry()
	e.exts.SetPlaceHolder("mp4 mkv avi")
	e.note = widget.NewLabel("")
	e.note.Wrapping = fyne.TextWrapWord
	e.note.Importance = widget.LowImportance
	e.sel = widget.NewSelect(e.names(), func(c string) {
		e.store()
		e.load(c)
	})
	e.newBtn = widget.NewButtonWithIcon(T("New…"), theme.ContentAddIcon(), e.add)
	e.delBtn = widget.NewButtonWithIcon(T("Delete"), theme.DeleteIcon(), e.remove)
	e.resetBtn = widget.NewButton(T("Defaults"), e.reset)
	e.sel.SetSelected(e.defs[0].Name)
	return e
}

func (e *categoryEditor) names() []string {
	out := make([]string, len(e.defs))
	for i, d := range e.defs {
		out[i] = d.Name
	}
	return out
}

func (e *categoryEditor) index(name string) int {
	return slices.IndexFunc(e.defs, func(d engine.CategoryDef) bool { return strings.EqualFold(d.Name, name) })
}

// has reports whether a category of that name exists in the edited list.
func (e *categoryEditor) has(name string) bool { return e.index(name) >= 0 }

// store keeps what is typed for the selected category.
func (e *categoryEditor) store() {
	if e.cur == "" {
		return
	}
	e.dirs[e.cur] = strings.TrimSpace(e.folder.Text)
	if i := e.index(e.cur); i >= 0 && e.cur != engine.OtherCategory {
		e.defs[i].Exts = strings.Fields(strings.ReplaceAll(e.exts.Text, ",", " "))
	}
}

func (e *categoryEditor) load(name string) {
	e.cur = name
	e.folder.SetText(e.dirs[name])
	e.folder.SetPlaceHolder(filepath.Join(e.base.Text, name))
	if name == engine.OtherCategory {
		e.exts.SetText("")
		e.exts.Disable()
		e.delBtn.Disable()
		e.note.SetText(T("Other takes every file the categories above do not claim."))
		return
	}
	e.exts.Enable()
	e.delBtn.Enable()
	if i := e.index(name); i >= 0 {
		e.exts.SetText(strings.Join(e.defs[i].Exts, " "))
	}
	e.note.SetText(T("Separate extensions with spaces. An extension listed in two categories belongs to the first."))
}

func (e *categoryEditor) refresh(selectName string) {
	e.sel.Options = e.names()
	e.sel.Refresh()
	e.sel.SetSelected(selectName)
}

func (e *categoryEditor) add() {
	name := widget.NewEntry()
	name.SetPlaceHolder(T("Comics"))
	dialog.ShowForm(T("New category"), T("Add"), T("Cancel"),
		[]*widget.FormItem{widget.NewFormItem(T("Name"), name)}, func(ok bool) {
			if !ok {
				return
			}
			if msg := e.addNamed(name.Text); msg != "" {
				dialog.ShowInformation(T("New category"), msg, e.win)
				return
			}
			e.win.Canvas().Focus(e.exts)
		}, e.win)
}

// addNamed adds a category and selects it, or says why it cannot.
func (e *categoryEditor) addNamed(name string) string {
	n := strings.TrimSpace(name)
	switch {
	case n == "":
		return T("Enter a name.")
	case e.has(n):
		return Tf("There is already a category named %s.", n)
	case strings.ContainsAny(n, `\/:*?"<>|`):
		return `A category name is also a folder name, so it cannot contain \ / : * ? " < > |.`
	}
	e.store()
	// New categories go before Other, which stays last.
	e.defs = slices.Insert(e.defs, len(e.defs)-1, engine.CategoryDef{Name: n})
	e.refresh(n)
	return ""
}

func (e *categoryEditor) remove() {
	i := e.index(e.cur)
	if i < 0 || e.cur == engine.OtherCategory {
		return
	}
	delete(e.dirs, e.cur)
	e.defs = slices.Delete(e.defs, i, i+1)
	e.cur = "" // nothing left to store for it
	e.refresh(e.defs[0].Name)
}

func (e *categoryEditor) reset() {
	e.store()
	e.defs = engine.DefaultCategories()
	e.cur = ""
	e.refresh(e.defs[0].Name)
}
