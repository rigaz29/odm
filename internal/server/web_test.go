package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"odm/internal/engine"
)

// TestWebAssets checks that every script and stylesheet the page loads is
// embedded, and that every element id the script looks up exists.
func TestWebAssets(t *testing.T) {
	h := New(engine.NewManager(engine.Settings{Dir: t.TempDir()}, "")).Handler()
	get := func(path string) string {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusOK {
			t.Fatalf("GET %s: %d", path, w.Code)
		}
		b, _ := io.ReadAll(w.Body)
		return string(b)
	}
	page := get("/")
	for _, m := range regexp.MustCompile(`(?:src|href)="(/[^"]+)"`).FindAllStringSubmatch(page, -1) {
		get(m[1])
	}
	script := get("/app.js")
	for _, m := range regexp.MustCompile(`\$\("([A-Za-z]+)"\)`).FindAllStringSubmatch(script, -1) {
		if !strings.Contains(page, `id="`+m[1]+`"`) {
			t.Errorf("app.js looks up #%s, which index.html lacks", m[1])
		}
	}
	for _, m := range regexp.MustCompile(`data-icon="([A-Za-z]+)"`).FindAllStringSubmatch(page, -1) {
		if !strings.Contains(script, "\n  "+m[1]+": [") {
			t.Errorf("index.html uses icon %q, which app.js lacks", m[1])
		}
	}
}
