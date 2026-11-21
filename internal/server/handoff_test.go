package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"odm/internal/engine"
)

// A download the browser holds is tried before ODM takes it. A file host's
// free link works once, and the browser's own request used it: ODM got
// "410 Gone", after the browser's copy was cancelled, and neither
// downloaded the file. Refused, the browser keeps its download.
func TestHeldHandOffIsTriedFirst(t *testing.T) {
	gone := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "this link was used", http.StatusGone)
	}))
	defer gone.Close()
	file := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "f.zip", time.Time{}, strings.NewReader("data"))
	}))
	defer file.Close()

	mgr := engine.NewManager(engine.Settings{Dir: t.TempDir()}, "")
	s := New(mgr)
	prompted := 0
	s.OnPrompt = func(engine.AddOptions) bool { prompted++; return true }
	h := s.Handler()
	post := func(body string) (int, string) {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/downloads", strings.NewReader(body)))
		return w.Code, w.Body.String()
	}

	if c, body := post(`{"url":"` + gone.URL + `/c1285416347","prompt":true,"check":true}`); c != http.StatusConflict || !strings.Contains(body, "410") {
		t.Fatalf("a link ODM cannot fetch: %d %s, want 409 naming the 410", c, body)
	}
	if prompted != 0 || len(mgr.List()) != 0 {
		t.Fatalf("taken anyway: %d prompts, %d downloads", prompted, len(mgr.List()))
	}
	if c, body := post(`{"url":"` + file.URL + `/f.zip","prompt":true,"check":true}`); c != http.StatusAccepted || prompted != 1 {
		t.Fatalf("a link ODM can fetch: %d %s, %d prompts", c, body, prompted)
	}
	// Without a copy in the browser (its context menu, a script) there is
	// nothing to fall back on: ODM takes the address, and says what fails.
	if c, _ := post(`{"url":"` + gone.URL + `/c1285416348","prompt":true}`); c != http.StatusAccepted || prompted != 2 {
		t.Fatalf("not held: %d, %d prompts", c, prompted)
	}
}
