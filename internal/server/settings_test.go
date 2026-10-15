package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"odm/internal/engine"
)

func TestPutSettingsKeepsWhatTheBodyLeavesOut(t *testing.T) {
	mgr := engine.NewManager(engine.Settings{
		Dir:       t.TempDir(),
		Queues:    []string{"Main", "Night"},
		Logins:    []engine.Login{{Host: "files.example", User: "me", Pass: "pw"}},
		Proxy:     engine.ProxyConfig{Mode: engine.ProxyHTTP, Host: "proxy.example", Port: 3128},
		Schedules: map[string]engine.QueueSchedule{"Night": {Retries: 7}},
	}, "")
	h := New(mgr).Handler()
	for _, method := range []string{http.MethodPut, http.MethodPatch} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(method, "/api/settings",
			strings.NewReader(`{"connections": 4, "speed_limit": 102400, "category_list": ["ignored"]}`)))
		if w.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", method, w.Code, w.Body)
		}
		st := mgr.Settings()
		if st.Connections != 4 || st.SpeedLimit != 102400 {
			t.Fatalf("%s: the sent fields were not applied: %d connections, limit %d", method, st.Connections, st.SpeedLimit)
		}
		if len(st.Queues) != 2 || len(st.Logins) != 1 || st.Proxy.Host != "proxy.example" || st.Schedules["Night"].Retries != 7 {
			t.Fatalf("%s: fields the body left out were lost: %+v", method, st)
		}
	}
}
