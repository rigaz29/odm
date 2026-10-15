//go:build fyne

package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"odm/internal/engine"
)

// formEntry returns the first entry in the form row labelled label.
func formEntry(t *testing.T, form fyne.CanvasObject, label string) *widget.Entry {
	t.Helper()
	var find func(o fyne.CanvasObject) *widget.Entry
	find = func(o fyne.CanvasObject) *widget.Entry {
		switch v := o.(type) {
		case *widget.Entry:
			return v
		case *fyne.Container:
			for _, c := range v.Objects {
				if e := find(c); e != nil {
					return e
				}
			}
		}
		return nil
	}
	for _, it := range form.(*widget.Form).Items {
		if it.Text == label {
			if e := find(it.Widget); e != nil {
				return e
			}
		}
	}
	t.Fatalf("no entry in the %q row", label)
	return nil
}

func TestAddFormRemembersTheSaveFolder(t *testing.T) {
	def, last := t.TempDir(), t.TempDir()
	a := newTestApp(t, engine.Settings{Dir: def, LastDir: last})
	form, buttons := a.newAddForm(a.win, engine.AddOptions{}, func() {}, nil)
	folder := formEntry(t, form, "Save folder")
	if folder.Text != last {
		t.Fatalf("offers %q, want the folder used last time %q", folder.Text, last)
	}

	// Add a download to another folder; the next window offers that one.
	other := t.TempDir()
	folder.SetText(other)
	formEntry(t, form, "URL").SetText("ftp://h/file.zip") // not http: no probe runs
	test.Tap(buttons[0].(*widget.Button))                 // Download Later
	if got := a.mgr.Settings().LastDir; got != other {
		t.Fatalf("remembered %q, want %q", got, other)
	}
	if list := a.mgr.List(); len(list) != 1 || list[0].Dir != other {
		t.Fatalf("download not saved to the chosen folder: %+v", list)
	}
	next, _ := a.newAddForm(a.win, engine.AddOptions{}, func() {}, nil)
	if got := formEntry(t, next, "Save folder").Text; got != other {
		t.Fatalf("next window offers %q, want %q", got, other)
	}
}

func TestAddFormOffersTheDefaultWhenNotRemembering(t *testing.T) {
	def := t.TempDir()
	a := newTestApp(t, engine.Settings{Dir: def, LastDir: t.TempDir(), NoRememberDir: true})
	form, _ := a.newAddForm(a.win, engine.AddOptions{}, func() {}, nil)
	if got := formEntry(t, form, "Save folder").Text; got != def {
		t.Fatalf("offers %q, want the default %q", got, def)
	}
}
