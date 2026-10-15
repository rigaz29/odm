package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"odm/internal/engine"
)

const testMaster = `#EXTM3U
#EXT-X-STREAM-INF:BANDWIDTH=800000,RESOLUTION=1280x720,CODECS="avc1.64001f,mp4a.40.2"
720.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=5000000,RESOLUTION=1920x1080,CODECS="avc1.640028,mp4a.40.2"
1080.m3u8
`

const testMedia = "#EXTM3U\n#EXT-X-TARGETDURATION:4\n#EXTINF:4,\na.ts\n#EXTINF:4,\nb.ts\n#EXT-X-ENDLIST\n"

// playlists serves an HLS stream that answers only with the session cookie
// the browser would send.
func playlists(t *testing.T) *httptest.Server {
	t.Helper()
	drm, err := os.ReadFile(filepath.Join("..", "media", "testdata", "hls_drm.m3u8"))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{"/master.m3u8": testMaster, "/1080.m3u8": testMedia, "/720.m3u8": testMedia, "/drm.m3u8": string(drm)}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := files[r.URL.Path]
		if !ok || r.Header.Get("Cookie") != "session=1" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestProbeStreamListsQualities(t *testing.T) {
	srv := playlists(t)
	h := New(engine.NewManager(engine.Settings{Dir: t.TempDir()}, "")).Handler()
	probe := func(path string) (int, map[string]any) {
		body := fmt.Sprintf(`{"url": %q, "headers": {"Cookie": "session=1"}}`, srv.URL+path)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/probe/stream", strings.NewReader(body)))
		var out map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		return w.Code, out
	}

	code, out := probe("/master.m3u8")
	if code != http.StatusOK || out["kind"] != "hls" || out["single"] != nil {
		t.Fatalf("master: %d %v", code, out)
	}
	qs, _ := out["qualities"].([]any)
	if len(qs) != 2 {
		t.Fatalf("qualities %v", out["qualities"])
	}
	first, _ := qs[0].(map[string]any)
	if first["label"] != "1080p · 5.0 Mbps · H.264, AAC" || first["estimate"] != float64(5000000/8*8) {
		t.Fatalf("best quality %v", first)
	}

	if code, out := probe("/720.m3u8"); code != http.StatusOK || out["single"] != true {
		t.Fatalf("one quality: %d %v", code, out)
	}
	if code, out := probe("/drm.m3u8"); code != http.StatusUnprocessableEntity || out["refused"] != true {
		t.Fatalf("DRM: %d %v", code, out)
	}
	if code, out := probe("/missing.m3u8"); code != http.StatusBadGateway || out["refused"] != nil {
		t.Fatalf("missing: %d %v", code, out)
	}
}

func TestAddPinsTheQualityPicked(t *testing.T) {
	srv := playlists(t)
	mgr := engine.NewManager(engine.Settings{Dir: t.TempDir()}, "")
	h := New(mgr).Handler()

	body := fmt.Sprintf(`{"url": %q, "stream": "hls", "quality": "720p · 800 kbps · H.264, AAC",
		"headers": {"Cookie": "session=1"}, "start": false}`, srv.URL+"/master.m3u8")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/downloads", strings.NewReader(body)))
	if w.Code != http.StatusCreated {
		t.Fatalf("add: %d %s", w.Code, w.Body)
	}
	data, err := mgr.Export()
	var states []engine.State
	if err == nil {
		err = json.Unmarshal(data, &states)
	}
	if err != nil || len(states) != 1 || states[0].Stream == nil {
		t.Fatalf("state: %v %+v", err, states)
	}
	if v := states[0].Stream.Variant; !strings.Contains(v, "720.m3u8") {
		t.Fatalf("pinned variant %q, want the 720p one", v)
	}
}

// The window gets the label to select; nothing is probed for it.
func TestPromptCarriesTheQuality(t *testing.T) {
	s := New(engine.NewManager(engine.Settings{Dir: t.TempDir()}, ""))
	var got engine.AddOptions
	s.OnPrompt = func(o engine.AddOptions) bool { got = o; return true }
	body := `{"url": "https://cdn.example/master.m3u8", "stream": "hls", "quality": "720p", "prompt": true}`
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/downloads", strings.NewReader(body)))
	if w.Code != http.StatusAccepted || got.StreamQuality != "720p" || got.Stream == nil {
		t.Fatalf("prompt: %d, options %+v", w.Code, got)
	}
}
