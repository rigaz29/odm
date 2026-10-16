package gui

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"odm/internal/engine"
)

// fileServer serves files by path with Range support, and records the
// Cookie header of the last request.
func fileServer(t *testing.T, files map[string]string) (*httptest.Server, func() string) {
	t.Helper()
	var mu sync.Mutex
	cookie := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		cookie = r.Header.Get("Cookie")
		mu.Unlock()
		body, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		http.ServeContent(w, r, filepath.Base(r.URL.Path), time.Time{}, bytes.NewReader([]byte(body)))
	}))
	t.Cleanup(srv.Close)
	return srv, func() string {
		mu.Lock()
		defer mu.Unlock()
		return cookie
	}
}

func TestProbeTellsWhatAnAddressIs(t *testing.T) {
	srv, _ := fileServer(t, map[string]string{
		"/files/report.pdf": strings.Repeat("p", 5000),
		"/files/pack.zip":   strings.Repeat("z", 3000),
	})
	svc, _ := newTestService(t)

	if r := svc.Probe("  ", ""); r.Card.State != "idle" || r.Kind != "" {
		t.Fatalf("empty: %+v", r)
	}
	if r := svc.Probe("ftp://x/y", ""); r.Card.State != "warn" || r.Kind != "" {
		t.Fatalf("ftp: %+v", r)
	}
	r := svc.Probe(srv.URL+"/files/report.pdf ", "")
	if r.Kind != "file" || r.Card.State != "file" || r.FileName != "report.pdf" || r.Category != "Documents" ||
		r.Card.Size != 5000 || !r.Card.Resumable || r.Card.Host != "127.0.0.1" || r.Zip || r.URL != srv.URL+"/files/report.pdf" {
		t.Fatalf("file: %+v", r)
	}
	if r := svc.Probe(srv.URL+"/files/pack.zip", ""); !r.Zip {
		t.Fatalf("a ZIP on a server that sends parts offers no preview: %+v", r)
	}
	if r := svc.Probe(srv.URL+"/missing.bin", ""); r.Kind != "" || r.Card.State != "problem" || r.Card.Error == "" {
		t.Fatalf("missing: %+v", r)
	}
}

func TestProbeAndCreateAStream(t *testing.T) {
	pl := func(p string) string {
		return "#EXTM3U\n#EXT-X-TARGETDURATION:2\n#EXTINF:2,\n" + p + "1.ts\n#EXTINF:2,\n" + p + "2.ts\n#EXT-X-ENDLIST\n"
	}
	srv, _ := fileServer(t, map[string]string{
		"/m.m3u8": "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=900000,RESOLUTION=1920x1080\nhi.m3u8\n" +
			"#EXT-X-STREAM-INF:BANDWIDTH=100000,RESOLUTION=640x360\nlo.m3u8\n",
		"/hi.m3u8": pl("hi"), "/lo.m3u8": pl("lo"),
	})
	svc, mgr := newTestService(t)
	r := svc.Probe(srv.URL+"/m.m3u8", "")
	if r.Kind != "stream" || r.Card.State != "stream" || r.Card.Format != "HLS" || !r.Video || len(r.Qualities) != 2 {
		t.Fatalf("stream: %+v", r)
	}
	res, err := svc.Create(CreateRequest{URL: srv.URL + "/m.m3u8", FileName: "talk.mp4", Quality: 1, Category: "Video"})
	if err != nil || res.ID == "" {
		t.Fatalf("%+v %v", res, err)
	}
	d, _ := mgr.Get(res.ID)
	if d.Stream != "hls" || d.FileName != "talk.mp4" || d.Category != "Video" || d.Status == engine.StatusDownloading {
		t.Fatalf("download %+v", d)
	}
}

func TestCreateAsksBeforeAddingAnAddressAgain(t *testing.T) {
	srv, _ := fileServer(t, map[string]string{"/a.bin": "aaaa"})
	svc, mgr := newTestService(t)
	dir := t.TempDir()
	req := CreateRequest{URL: srv.URL + "/a.bin", Folder: dir, FileName: "a.bin", Description: " note ", Queue: engine.DefaultQueue}
	first, err := svc.Create(req)
	if err != nil || first.ID == "" || first.Duplicate != nil {
		t.Fatalf("%+v %v", first, err)
	}
	d, _ := mgr.Get(first.ID)
	if d.Dir != dir || d.Description != "note" || d.Connections != mgr.Settings().Connections {
		t.Fatalf("download %+v", d)
	}
	again, err := svc.Create(req)
	if err != nil || again.ID != "" || again.Duplicate == nil || again.Duplicate.ID != first.ID {
		t.Fatalf("the same address again: %+v %v", again, err)
	}
	req.Again = true
	if third, err := svc.Create(req); err != nil || third.ID == "" || third.ID == first.ID {
		t.Fatalf("again, confirmed: %+v %v", third, err)
	}
	if _, err := svc.Create(CreateRequest{URL: srv.URL + "/b.bin", Checksum: "not a hash"}); err == nil {
		t.Fatal("a checksum ODM cannot read was accepted")
	}
	if _, err := svc.Create(CreateRequest{URL: " "}); err == nil {
		t.Fatal("an empty address was added")
	}
}

func TestPrefillCarriesTheBrowsersHeaders(t *testing.T) {
	srv, cookie := fileServer(t, map[string]string{"/v.bin": "vvvv"})
	svc, mgr := newTestService(t)
	token := svc.keepPrefill(engine.AddOptions{URL: srv.URL + "/v.bin", FileName: "v.bin", Description: "from the browser",
		Headers: map[string]string{"Cookie": "session=1"}})
	if p := svc.Prefill(token); p.URL != srv.URL+"/v.bin" || p.FileName != "v.bin" || p.Description != "from the browser" || p.Site || p.Stream {
		t.Fatalf("prefill %+v", p)
	}
	svc.Probe(srv.URL+"/v.bin", token)
	if got := cookie(); got != "session=1" {
		t.Fatalf("the check sent cookie %q", got)
	}
	res, err := svc.Create(CreateRequest{Token: token, URL: srv.URL + "/v.bin", FileName: "v.bin"})
	if err != nil || res.ID == "" {
		t.Fatalf("%+v %v", res, err)
	}
	if p := svc.Prefill(token); p.URL != "" {
		t.Fatal("the hand-off was kept after its download was added")
	}
	if _, ok := mgr.Get(res.ID); !ok {
		t.Fatal("no download")
	}
}

// stubWindows records what the service asked to open.
type stubWindows struct {
	mu   sync.Mutex
	asks []string
}

func (w *stubWindows) note(s string) {
	w.mu.Lock()
	w.asks = append(w.asks, s)
	w.mu.Unlock()
}
func (w *stubWindows) openAdd(token string)   { w.note("add " + token) }
func (w *stubWindows) openProgress(id string) { w.note("progress " + id) }
func (w *stubWindows) closeProgress(id string) {
	w.note("close " + id)
}
func (w *stubWindows) openComplete(id string)           { w.note("complete " + id) }
func (w *stubWindows) openOptions(tab string)           { w.note("options " + tab) }
func (w *stubWindows) pickFolder(current string) string { return current + `\picked` }
func (w *stubWindows) pickFile(current string) string   { return current + `.exe` }
func (w *stubWindows) got() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return strings.Join(w.asks, ", ")
}

func TestFinishedDownloadsGetTheCompleteWindow(t *testing.T) {
	svc, mgr := newTestService(t)
	w := &stubWindows{}
	svc.windows = w
	at := func(st engine.Status) []engine.Info {
		return []engine.Info{{ID: "a", Status: st}, {ID: "b", Status: engine.StatusComplete}}
	}
	svc.noticeFinished(at(engine.StatusComplete)) // the first look only learns
	svc.noticeFinished(at(engine.StatusDownloading))
	svc.noticeFinished(at(engine.StatusComplete))
	if got := w.got(); got != "close a, complete a" {
		t.Fatalf("asked %q", got)
	}
	svc.SetCompleteDialog(false)
	if !mgr.Settings().NoCompleteDialog {
		t.Fatal("the setting stayed on")
	}
	svc.noticeFinished(at(engine.StatusPaused))
	svc.noticeFinished(at(engine.StatusComplete))
	if got := w.got(); got != "close a, complete a, close a" {
		t.Fatalf("asked %q with the complete window off", got)
	}
	if got := svc.PickFolder(`C:\x`); got != `C:\x\picked` {
		t.Fatalf("PickFolder gave %q", got)
	}
}

func TestStartingOpensTheProgressWindow(t *testing.T) {
	srv, _ := fileServer(t, map[string]string{"/s.bin": "ssss"})
	svc, mgr := newTestService(t)
	w := &stubWindows{}
	svc.windows = w
	res, err := svc.Create(CreateRequest{URL: srv.URL + "/s.bin", FileName: "s.bin", Folder: t.TempDir(), Start: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { mgr.Pause(res.ID) })
	if got := w.got(); got != "progress "+res.ID {
		t.Fatalf("asked %q", got)
	}
	later, _ := svc.Create(CreateRequest{URL: srv.URL + "/s.bin", FileName: "s2.bin", Again: true})
	if got := w.got(); strings.Contains(got, later.ID) {
		t.Fatalf("Download Later opened a window: %q", got)
	}
}

func TestDetail(t *testing.T) {
	svc, mgr := newTestService(t)
	dir := t.TempDir()
	d := mgr.Create(engine.AddOptions{URL: "http://127.0.0.1:9/f.iso", FileName: "f.iso", Dir: dir})
	svc.SetDownloadLimit(d.ID, 200<<10)
	svc.SetOpenWhenDone(d.ID, true)
	got, err := svc.Detail(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Item.ID != d.ID || got.Path != filepath.Join(dir, "f.iso") || got.SpeedLimit != 200<<10 || !got.OpenWhenDone || !got.CompleteDialog {
		t.Fatalf("detail %+v", got)
	}
	svc.SetDownloadLimit(d.ID, -5)
	if got, _ := svc.Detail(d.ID); got.SpeedLimit != 0 {
		t.Fatalf("limit %d after lifting it", got.SpeedLimit)
	}
	if _, err := svc.Detail("nope"); err == nil {
		t.Fatal("an unknown download has a detail")
	}
}
