package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"odm/internal/engine"
)

func TestProbeSiteWithoutYtDlp(t *testing.T) {
	mgr := engine.NewManager(engine.Settings{Dir: t.TempDir(), YtDlpPath: filepath.Join(t.TempDir(), "missing")}, "")
	t.Cleanup(func() { mgr.UpdateSettings(engine.Settings{Dir: t.TempDir()}) })
	h := New(mgr).Handler()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/probe/site", strings.NewReader(`{"url": "https://www.youtube.com/watch?v=abc"}`)))
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if w.Code != http.StatusServiceUnavailable || out["missing"] != true {
		t.Fatalf("%d %v", w.Code, out)
	}

	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/probe/site", strings.NewReader(`{"url": "file:///etc/passwd"}`)))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("a file address: %d", w.Code)
	}
}

func TestAddSitePage(t *testing.T) {
	mgr := engine.NewManager(engine.Settings{Dir: t.TempDir()}, "")
	h := New(mgr).Handler()
	body := `{"url": "https://www.youtube.com/watch?v=abc", "site": true, "format": "136+140", "estimate": 99,
		"file_name": "A talk.mp4", "headers": {"Cookie": "SID=secret"}, "start": false}`
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/downloads", strings.NewReader(body)))
	var info engine.Info
	_ = json.Unmarshal(w.Body.Bytes(), &info)
	if w.Code != http.StatusCreated || !info.Site || info.FileName != "A talk.mp4" {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	data, _ := mgr.Export()
	if !strings.Contains(string(data), `"format": "136+140"`) || strings.Contains(string(data), "secret") {
		t.Fatalf("state:\n%s", data)
	}

	both := `{"url": "https://www.youtube.com/watch?v=abc", "site": true, "stream": "hls"}`
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/downloads", strings.NewReader(both)))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("a stream and a site at once: %d", w.Code)
	}
}
