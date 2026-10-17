package server

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"odm/internal/engine"
	"odm/internal/ipc"
)

// A request the native host relays names its browser; ODM remembers each
// browser once, and ignores what is not a browser's name.
func TestRelayedRequestNamesBrowser(t *testing.T) {
	mgr := engine.NewManager(engine.Settings{Dir: t.TempDir()}, "")
	s := New(mgr)
	s.OnBrowser = mgr.NoteExtension
	h := s.Handler()
	get := func(browser string) {
		r := httptest.NewRequest(http.MethodGet, "/api/config", nil)
		if browser != "" {
			r.Header.Set(ipc.BrowserHeader, browser)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("config: %d", w.Code)
		}
	}
	get("")
	get("edge")
	get("firefox")
	get("edge")
	get("../../etc")
	if got := mgr.Settings().ExtensionBrowsers; !slices.Equal(got, []string{"edge", "firefox"}) {
		t.Fatalf("browsers: %v", got)
	}
}
