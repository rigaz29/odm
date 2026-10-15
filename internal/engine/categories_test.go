package engine

import (
	"context"
	"net/http"
	"path/filepath"
	"slices"
	"testing"
)

func TestNormalizeCategories(t *testing.T) {
	got := normalizeCategories([]CategoryDef{
		{Name: " Comics ", Exts: []string{".CBZ", "cbr", "cbz", " "}},
		{Name: "comics", Exts: []string{"pdf"}},        // same name: dropped
		{Name: "Other", Exts: []string{"bin"}},         // Other goes last, empty
		{Name: "Books", Exts: []string{"epub", "cbr"}}, // cbr already Comics'
		{Name: "", Exts: []string{"x"}},
		{Name: "bad/name"},
	})
	want := []CategoryDef{
		{Name: "Comics", Exts: []string{"cbr", "cbz"}},
		{Name: "Books", Exts: []string{"epub"}},
		{Name: OtherCategory},
	}
	if !sameCategories(got, want) {
		t.Fatalf("normalised to %+v", got)
	}
	if normalizeCategories(DefaultCategories()) != nil {
		t.Fatal("the built-in list should normalise to nil")
	}
	if normalizeCategories(nil) != nil {
		t.Fatal("nil should stay nil")
	}
}

func TestCustomCategoriesApply(t *testing.T) {
	t.Cleanup(func() { setCategories(nil) })
	defs := DefaultCategories()
	// Drop Video, add Comics before Other.
	defs = slices.DeleteFunc(defs, func(d CategoryDef) bool { return d.Name == "Video" })
	defs = slices.Insert(defs, len(defs)-1, CategoryDef{Name: "Comics", Exts: []string{"cbz"}})
	m := NewManager(Settings{Dir: t.TempDir(), CategoryDefs: defs}, "")

	if got := Category("issue-01.cbz"); got != "Comics" {
		t.Fatalf("cbz filed under %q", got)
	}
	if got := Category("movie.mp4"); got != OtherCategory {
		t.Fatalf("mp4 with Video gone filed under %q", got)
	}
	names := Categories()
	if names[len(names)-2] != "Comics" || names[len(names)-1] != OtherCategory {
		t.Fatalf("order %v", names)
	}

	// A download added for Video, which no longer exists, is shown and filed
	// by its file name instead.
	info := m.Create(AddOptions{URL: "http://h/clip.cbz", FileName: "clip.cbz", Category: "Video"})
	if info.Category != "Comics" {
		t.Fatalf("download with a removed category shows %q", info.Category)
	}

	// Back to the built-in set.
	next := m.Settings()
	next.CategoryDefs = nil
	m.UpdateSettings(next)
	if got := Category("movie.mp4"); got != "Video" {
		t.Fatalf("after reset mp4 is %q", got)
	}
}

func TestCategoryFolderFollowsCustomCategories(t *testing.T) {
	t.Cleanup(func() { setCategories(nil) })
	defs := append([]CategoryDef{{Name: "Comics", Exts: []string{"cbz"}}}, DefaultCategories()...)
	dir := t.TempDir()
	m := NewManager(Settings{Dir: dir, Categories: true, CategoryDefs: defs}, "")
	data := randomData(64 << 10)
	srv := httpServer(t, func(w http.ResponseWriter, r *http.Request) { serveRange(w, r, data) })
	info := m.Create(AddOptions{URL: srv.URL + "/issue.cbz"})
	d := m.get(info.ID)
	d.Start(context.Background())
	if got := d.Info(); got.Dir != filepath.Join(dir, "Comics") {
		t.Fatalf("saved to %q, want the Comics folder", got.Dir)
	}
}
