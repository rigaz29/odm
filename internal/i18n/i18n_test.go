package i18n_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"odm/internal/browserreg"
	"odm/internal/i18n"
)

// keysIn returns every text the files under dir pass to one of the calls in
// pattern, mapped to the file it was last found in. pattern's first group is
// the text as a double-quoted literal, which Go and TypeScript spell alike.
func keysIn(t *testing.T, dir string, pattern *regexp.Regexp, want func(path string) bool) map[string]string {
	t.Helper()
	keys := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "node_modules" || d.Name() == "bindings" {
				return filepath.SkipDir
			}
			return nil
		}
		if !want(path) {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range pattern.FindAllStringSubmatch(string(src), -1) {
			k, err := strconv.Unquote(m[1])
			if err != nil {
				t.Fatalf("%s: %s: %v", path, m[1], err)
			}
			keys[k] = filepath.ToSlash(path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return keys
}

// fyneKeys is every text the Fyne UI passes to T, Tf or markT, and the ones
// it shows through browserreg.SummaryWith(…, T).
func fyneKeys(t *testing.T) map[string]string {
	call := regexp.MustCompile(`\b(?:T|Tf|markT)\(("(?:[^"\\]|\\.)*")`)
	keys := keysIn(t, "../ui", call, func(p string) bool {
		return filepath.Dir(p) == filepath.Clean("../ui") && strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go")
	})
	for _, k := range browserreg.SummaryTexts {
		keys[k] = "browserreg"
	}
	return keys
}

// pageKeys is every text the new UI's page passes to t or tf, or marks
// with markT, and the ones its Go side translates with tr (the tray, the
// notifications).
func pageKeys(t *testing.T) map[string]string {
	call := regexp.MustCompile(`\b(?:t|tf|markT)\(\s*("(?:[^"\\]|\\.)*")`)
	keys := keysIn(t, "../gui/frontend/src", call, func(p string) bool {
		return (strings.HasSuffix(p, ".ts") || strings.HasSuffix(p, ".tsx")) && !strings.HasSuffix(p, ".test.ts")
	})
	tr := regexp.MustCompile(`\btr\(("(?:[^"\\]|\\.)*")`)
	for k, f := range keysIn(t, "../gui", tr, func(p string) bool {
		return filepath.Dir(p) == filepath.Clean("../gui") && strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go")
	}) {
		keys[k] = f
	}
	return keys
}

var verb = regexp.MustCompile(`%[-+# 0]*\d*(?:\.\d+)?[a-zA-Z%]`)

func TestIndonesianCoversBothUIs(t *testing.T) {
	id, err := i18n.Indonesian()
	if err != nil {
		t.Fatal(err)
	}
	fyne, page := fyneKeys(t), pageKeys(t)
	if len(fyne) < 300 || len(page) < 50 {
		t.Fatalf("found only %d texts in the Fyne UI and %d in the page", len(fyne), len(page))
	}
	used := map[string]bool{}
	for _, keys := range []map[string]string{fyne, page} {
		for k, file := range keys {
			used[k] = true
			v, ok := id[k]
			switch {
			case !ok || v == "":
				t.Errorf("%s: no Indonesian for %q", file, k)
			case strings.Join(verb.FindAllString(k, -1), " ") != strings.Join(verb.FindAllString(v, -1), " "):
				t.Errorf("%q and %q have different verbs", k, v)
			}
		}
	}
	for k := range id {
		if !used[k] {
			t.Errorf("id.json has %q, which neither UI uses", k)
		}
	}
}

func TestIndonesianIsShared(t *testing.T) {
	a, err := i18n.Indonesian()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := i18n.Indonesian()
	if a["Add URL"] != "Tambah URL" || len(a) != len(b) {
		t.Fatalf("Indonesian() gives %q, %d and %d texts", a["Add URL"], len(a), len(b))
	}
}
