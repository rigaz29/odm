//go:build fyne

package ui

import (
	"slices"
	"testing"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"odm/internal/engine"
)

func TestCategoryEditor(t *testing.T) {
	app := test.NewTempApp(t)
	w := app.NewWindow("Options")
	folder, base := widget.NewEntry(), widget.NewEntry()
	base.SetText("D:/dl")
	dirs := map[string]string{"Video": "V:/movies", "Audio": "A:/music"}
	e := newCategoryEditor(w, engine.DefaultCategories(), dirs, folder, base)

	if e.sel.Selected != "Video" || folder.Text != "V:/movies" {
		t.Fatalf("first category %q with folder %q", e.sel.Selected, folder.Text)
	}

	// Edit Video's types, then switch: the edit is kept.
	e.exts.SetText("mp4, mkv")
	e.sel.SetSelected(engine.OtherCategory)
	if got := e.defs[e.index("Video")].Exts; !slices.Equal(got, []string{"mp4", "mkv"}) {
		t.Fatalf("Video types kept as %v", got)
	}
	if !e.exts.Disabled() || !e.delBtn.Disabled() {
		t.Fatal("Other's types and Delete should be disabled")
	}

	// Add a category: it goes before Other and is selected.
	if msg := e.addNamed("Comics"); msg != "" {
		t.Fatal(msg)
	}
	if n := len(e.defs); e.defs[n-2].Name != "Comics" || e.defs[n-1].Name != engine.OtherCategory || e.sel.Selected != "Comics" {
		t.Fatalf("after add: %v, selected %q", e.names(), e.sel.Selected)
	}
	if msg := e.addNamed("comics"); msg == "" {
		t.Fatal("a second Comics was accepted")
	}
	if msg := e.addNamed("a/b"); msg == "" {
		t.Fatal("a name with a slash was accepted")
	}

	// Delete Audio: gone with its folder.
	e.sel.SetSelected("Audio")
	e.remove()
	if e.has("Audio") || dirs["Audio"] != "" {
		t.Fatalf("Audio still there: %v, folder %q", e.names(), dirs["Audio"])
	}

	// Defaults bring the built-in list back.
	e.reset()
	if !e.has("Audio") || e.has("Comics") {
		t.Fatalf("after Defaults: %v", e.names())
	}
}
