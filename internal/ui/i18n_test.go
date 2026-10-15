//go:build fyne

package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"fyne.io/fyne/v2"

	"odm/internal/engine"
)

// sourceKeys is every text the UI passes to T, Tf or markT.
func sourceKeys(t *testing.T) map[string]string {
	t.Helper()
	call := regexp.MustCompile(`\b(?:T|Tf|markT)\(("(?:[^"\\]|\\.)*")`)
	files, _ := filepath.Glob("*.go")
	keys := map[string]string{}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range call.FindAllStringSubmatch(string(src), -1) {
			k, err := strconv.Unquote(m[1])
			if err != nil {
				t.Fatalf("%s: %s: %v", f, m[1], err)
			}
			keys[k] = f
		}
	}
	return keys
}

var verb = regexp.MustCompile(`%[-+# 0]*\d*(?:\.\d+)?[a-zA-Z%]`)

func TestIndonesianCoversTheUI(t *testing.T) {
	id, err := readTranslations("translations/id.json")
	if err != nil {
		t.Fatal(err)
	}
	keys := sourceKeys(t)
	if len(keys) < 300 {
		t.Fatalf("found only %d texts in the sources", len(keys))
	}
	for k, file := range keys {
		v, ok := id[k]
		switch {
		case !ok || v == "":
			t.Errorf("%s: no Indonesian for %q", file, k)
		case strings.Join(verb.FindAllString(k, -1), " ") != strings.Join(verb.FindAllString(v, -1), " "):
			t.Errorf("%q and %q have different verbs", k, v)
		}
	}
	for k := range id {
		if _, used := keys[k]; !used {
			t.Errorf("id.json has %q, which the UI no longer uses", k)
		}
	}
}

func TestFyneTextInIndonesian(t *testing.T) {
	data, err := translationFS.ReadFile("translations/fyne.id.json")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	// The texts Fyne's widgets look up (lang.L and lang.X in Fyne 2.8).
	for _, k := range []string{"Cancel", "OK", "Yes", "No", "Confirm", "Error", "Open", "Save",
		"Copy", "Cut", "Paste", "Select all", "Undo", "Redo", "Quit", "File", "Folder",
		"New Folder", "Create Folder", "Enter filename", "Show Hidden Files", "Advanced",
		"Favourites", "(Parent)", "(Select one)", "file.name", "file.parent", "monday", "sunday.short"} {
		if m[k] == nil {
			t.Errorf("fyne.id.json lacks %q", k)
		}
	}
}

func TestLanguageChoice(t *testing.T) {
	cases := []struct {
		choice string
		system fyne.Locale
		want   string
	}{
		{langSystem, "id-ID", langIndonesian},
		{langSystem, "en-US", langEnglish},
		{langSystem, "fr", langEnglish},
		{langEnglish, "id-ID", langEnglish},
		{langIndonesian, "en-GB", langIndonesian},
		{"", "id", langIndonesian},
		{langSystem, "in_ID", langIndonesian},
	}
	for _, c := range cases {
		if got := resolveLanguage(c.choice, c.system); got != c.want {
			t.Errorf("resolveLanguage(%q, %q) = %q, want %q", c.choice, c.system, got, c.want)
		}
	}
	if languageModes[languageIndex(langIndonesian)] != langIndonesian || languageIndex("xx") != 0 {
		t.Fatal("languageIndex")
	}

	id, err := readTranslations("translations/id.json")
	if err != nil {
		t.Fatal(err)
	}
	translations = id
	defer func() { translations = nil }()
	if T("Add URL") != "Tambah URL" || Tf("%d items", 3) != "3 item" || T("not a key") != "not a key" {
		t.Fatalf("T gives %q, %q, %q", T("Add URL"), Tf("%d items", 3), T("not a key"))
	}
	if got := statusText(engineInfoPaused()); got != "Dijeda · 40%" {
		t.Fatalf("paused status in Indonesian: %q", got)
	}
}

func engineInfoPaused() engine.Info {
	return engine.Info{Status: engine.StatusPaused, Total: 100, Progress: 40}
}

func TestWindowInIndonesian(t *testing.T) {
	id, err := readTranslations("translations/id.json")
	if err != nil {
		t.Fatal(err)
	}
	translations = id
	defer func() { translations = nil }()
	a := newTestApp(t, engine.Settings{})
	a.mgr.Create(engine.AddOptions{URL: "http://h/a.bin", FileName: "a.bin"})
	a.refresh()
	var menus []string
	for _, m := range a.mainMenu.Items {
		menus = append(menus, m.Label)
	}
	if got := strings.Join(menus, " "); got != "Tugas Unduhan Tampilan Bantuan" {
		t.Fatalf("menus %q", got)
	}
	if a.tools["add"].text != "Tambah URL" || a.headerText(colSize) != "Ukuran" || a.status.Text != "1 item" {
		t.Fatalf("toolbar %q, header %q, status %q", a.tools["add"].text, a.headerText(colSize), a.status.Text)
	}
}
