//go:build fyne

package ui

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"odm/internal/engine"
)

// formOf finds the widget.Form in the New Download content.
func formOf(o fyne.CanvasObject) *widget.Form {
	switch v := o.(type) {
	case *widget.Form:
		return v
	case *fyne.Container:
		for _, c := range v.Objects {
			if f := formOf(c); f != nil {
				return f
			}
		}
	}
	return nil
}

// formEntry returns the first entry in the form row labelled label.
func formEntry(t *testing.T, content fyne.CanvasObject, label string) *widget.Entry {
	t.Helper()
	form := formOf(content)
	if form == nil {
		t.Fatal("no form in the New Download content")
	}
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
	for _, it := range form.Items {
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

// cardText is the text of the probe card under the New Download form.
func cardText(content fyne.CanvasObject) string {
	var b strings.Builder
	for _, o := range test.LaidOutObjects(content.(*fyne.Container).Objects[1]) {
		if l, ok := o.(*widget.Label); ok && l.Text != "" {
			b.WriteString(l.Text + " | ")
		}
	}
	return b.String()
}

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for end := time.Now().Add(10 * time.Second); time.Now().Before(end); time.Sleep(20 * time.Millisecond) {
		if ok() {
			return
		}
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestAddFormProbeCard(t *testing.T) {
	data := bytes.Repeat([]byte("odm"), 1000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/missing.pdf" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		http.ServeContent(w, r, "report.pdf", time.Time{}, bytes.NewReader(data))
	}))
	defer srv.Close()

	a := newTestApp(t, engine.Settings{})
	content, _ := a.newAddForm(a.win, engine.AddOptions{}, func() {}, nil)
	if txt := cardText(content); !strings.Contains(txt, "No address yet") {
		t.Fatalf("empty form card: %q", txt)
	}
	url := formEntry(t, content, "URL")

	url.SetText(srv.URL + "/report.pdf")
	waitFor(t, "the card to show the file", func() bool { return strings.Contains(cardText(content), "report.pdf") })
	txt := cardText(content)
	for _, want := range []string{"report.pdf", humanBytes(int64(len(data))), "application/pdf", "127.0.0.1", "Resumable"} {
		if !strings.Contains(txt, want) {
			t.Fatalf("card %q lacks %q", txt, want)
		}
	}

	url.SetText(srv.URL + "/missing.pdf")
	waitFor(t, "the card to show the failure", func() bool { return strings.Contains(cardText(content), "could not be checked") })

	url.SetText("ftp://host/file.zip")
	if txt := cardText(content); !strings.Contains(txt, "Not checked") {
		t.Fatalf("ftp card: %q", txt)
	}
}
