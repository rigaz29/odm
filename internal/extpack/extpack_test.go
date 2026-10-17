package extpack

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"odm/extension"
	"odm/internal/engine"
)

func TestManifestVersion(t *testing.T) {
	cases := map[string]string{"v0.2.0": "0.2.0", "0.2.0-rc1": "0.2.0", "v1.2.3.4": "1.2.3.4", "v2": "2"}
	for in, want := range cases {
		if got, err := ManifestVersion(in); err != nil || got != want {
			t.Errorf("ManifestVersion(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "latest", "v1.x", "1.2.3.4.5"} {
		if _, err := ManifestVersion(bad); err == nil {
			t.Errorf("ManifestVersion(%q) accepted", bad)
		}
	}
}

func readZip(t *testing.T, path string) (map[string]any, []string) {
	t.Helper()
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	var names []string
	var man map[string]any
	for _, f := range zr.File {
		names = append(names, f.Name)
		if f.Name == "manifest.json" {
			r, _ := f.Open()
			if err := json.NewDecoder(r).Decode(&man); err != nil {
				t.Fatal(err)
			}
			r.Close()
		}
	}
	sort.Strings(names)
	return man, names
}

// Builds from the real extension/ folder, so a manifest change that breaks
// either browser fails here rather than in the release workflow.
func TestBuildFromRepoExtension(t *testing.T) {
	out := t.TempDir()
	archives, err := Build(filepath.Join("..", "..", "extension"), out, "0.9.1")
	if err != nil {
		t.Fatal(err)
	}

	cm, cfiles := readZip(t, archives[Chromium])
	if cm["version"] != "0.9.1" || cm["key"] == nil || cm["browser_specific_settings"] != nil {
		t.Fatalf("chromium manifest: version=%v key?=%v bss=%v", cm["version"], cm["key"] != nil, cm["browser_specific_settings"])
	}
	if bg := cm["background"].(map[string]any); bg["service_worker"] != "background.js" || bg["scripts"] != nil {
		t.Fatalf("chromium background: %v", bg)
	}

	fm, ffiles := readZip(t, archives[Firefox])
	if fm["version"] != "0.9.1" || fm["key"] != nil {
		t.Fatalf("firefox manifest keeps key or wrong version: %v", fm["version"])
	}
	bg := fm["background"].(map[string]any)
	if bg["service_worker"] != nil || len(bg["scripts"].([]any)) != 1 {
		t.Fatalf("firefox background: %v", bg)
	}
	gecko := fm["browser_specific_settings"].(map[string]any)["gecko"].(map[string]any)
	if gecko["id"] != "odm@rigaz29.github.io" {
		t.Fatalf("gecko id %v", gecko["id"])
	}

	for _, files := range [][]string{cfiles, ffiles} {
		has := map[string]bool{}
		for _, f := range files {
			has[f] = true
		}
		for _, want := range []string{"manifest.json", "background.js", "content.js", "popup.html", "popup.js"} {
			if !has[want] {
				t.Fatalf("archive misses %s: %v", want, files)
			}
		}
		for _, f := range files {
			if f == "README.md" || f == "files.go" || filepath.Dir(f) == "native" || filepath.Dir(f) == "test" {
				t.Fatalf("archive ships %s, which is not part of the extension", f)
			}
		}
	}
}

// The extension's fallback type list must match ODM's built-in list, or the
// takeover would change depending on whether ODM was reachable.
func TestExtensionDefaultTypesMatchODM(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "extension", "background.js"))
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?s)const DEFAULT_TYPES = \[(.*?)\];`).FindSubmatch(src)
	if m == nil {
		t.Fatal("DEFAULT_TYPES not found in background.js")
	}
	got := regexp.MustCompile(`"([a-z0-9]+)"`).FindAllSubmatch(m[1], -1)
	var js []string
	for _, g := range got {
		js = append(js, string(g[1]))
	}
	want := engine.DefaultFileTypes()
	if strings.Join(js, ",") != strings.Join(want, ",") {
		t.Fatalf("background.js DEFAULT_TYPES differ from engine.DefaultFileTypes:\n js:  %v\n odm: %v", js, want)
	}
}

// Unpack writes ODM's embedded copy of the extension as "Load unpacked"
// reads it, and on later calls touches only what changed.
func TestUnpack(t *testing.T) {
	dir := t.TempDir()
	changed, err := Unpack(extension.Files, Chromium, dir, "0.9.2")
	if err != nil || !changed {
		t.Fatalf("first unpack: changed=%v err=%v", changed, err)
	}
	var m map[string]any
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	if m["version"] != "0.9.2" || m["key"] == nil || m["browser_specific_settings"] != nil {
		t.Fatalf("manifest: %v", m)
	}
	for _, f := range []string{"background.js", "content.js", "medialist.js", "popup.html", "popup.js"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Fatalf("missing %s: %v", f, err)
		}
	}
	if changed, err := Unpack(extension.Files, Chromium, dir, "0.9.2"); err != nil || changed {
		t.Fatalf("unchanged unpack: changed=%v err=%v", changed, err)
	}

	stray := filepath.Join(dir, "old.js")
	if err := os.WriteFile(stray, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "popup.js"), []byte("edited"), 0o644); err != nil {
		t.Fatal(err)
	}
	if changed, err := Unpack(extension.Files, Chromium, dir, "0.9.2"); err != nil || !changed {
		t.Fatalf("repair: changed=%v err=%v", changed, err)
	}
	if _, err := os.Stat(stray); !os.IsNotExist(err) {
		t.Fatalf("a file no longer in the extension stayed: %v", err)
	}
	want, _ := extension.Files.ReadFile("popup.js")
	if got, _ := os.ReadFile(filepath.Join(dir, "popup.js")); string(got) != string(want) {
		t.Fatal("an edited file was not put back")
	}
}
