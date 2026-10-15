package engine

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// versionedServer serves one file whose content and ETag can be swapped,
// counting full transfers (range requests other than the one-byte probe).
type versionedServer struct {
	mu        sync.Mutex
	data      []byte
	etag      string
	transfers int64
	url       string
}

func newVersionedServer(t *testing.T, data []byte, etag string) *versionedServer {
	v := &versionedServer{data: data, etag: etag}
	srv := httpServer(t, func(w http.ResponseWriter, r *http.Request) {
		v.mu.Lock()
		data, etag := v.data, v.etag
		v.mu.Unlock()
		if rng := r.Header.Get("Range"); rng != "bytes=0-0" {
			atomic.AddInt64(&v.transfers, 1)
		}
		w.Header().Set("ETag", etag)
		serveRange(w, r, data)
	})
	v.url = srv.URL + "/report.pdf"
	return v
}

func (v *versionedServer) set(data []byte, etag string) {
	v.mu.Lock()
	v.data, v.etag = data, etag
	v.mu.Unlock()
}

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func syncManager(t *testing.T, sync bool) (*Manager, string) {
	t.Helper()
	dir := t.TempDir()
	m := NewManager(Settings{Dir: dir}, "")
	m.settings.Schedules[DefaultQueue] = QueueSchedule{Sync: sync, Days: allDays(), Parallel: 1}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	m.Begin(ctx)
	return m, dir
}

func TestSyncQueueFetchesAChangedFileAgain(t *testing.T) {
	v1, v2 := randomData(200<<10), randomData(300<<10)
	srv := newVersionedServer(t, v1, `"v1"`)
	m, dir := syncManager(t, true)
	info := m.Create(AddOptions{URL: srv.url, Start: true})
	waitFor(t, "the first download", func() bool { return m.get(info.ID).status() == StatusComplete })
	path := filepath.Join(dir, "report.pdf")

	srv.set(v2, `"v2"`)
	m.StartQueue(DefaultQueue)
	// Read the file only once ODM is done with it: on Windows a file held
	// open cannot be replaced at that moment.
	waitFor(t, "the file to be fetched again", func() bool {
		in := m.get(info.ID).Info()
		return in.Total == int64(len(v2)) && in.Status == StatusComplete
	})
	if got, _ := os.ReadFile(path); !bytes.Equal(got, v2) {
		t.Fatal("the local file was not replaced with the new version")
	}
	if fi, err := os.Stat(filepath.Join(dir, "report (2).pdf")); err == nil {
		t.Fatalf("a second copy was made instead of replacing: %v", fi.Name())
	}
	if got := m.get(info.ID).Info(); got.FileName != "report.pdf" || got.Total != int64(len(v2)) {
		t.Fatalf("after sync: %q, %d bytes", got.FileName, got.Total)
	}
	waitFor(t, "the queue to finish", func() bool { return !m.QueueRunning(DefaultQueue) })
}

func TestSyncQueueLeavesAnUnchangedFile(t *testing.T) {
	data := randomData(128 << 10)
	srv := newVersionedServer(t, data, `"same"`)
	m, _ := syncManager(t, true)
	info := m.Create(AddOptions{URL: srv.url, Start: true})
	waitFor(t, "the download", func() bool { return m.get(info.ID).status() == StatusComplete })
	before := atomic.LoadInt64(&srv.transfers)

	m.StartQueue(DefaultQueue)
	waitFor(t, "the check", func() bool { return !m.QueueRunning(DefaultQueue) })
	if n := atomic.LoadInt64(&srv.transfers) - before; n != 0 {
		t.Fatalf("an unchanged file was fetched again (%d transfers)", n)
	}
	if s := m.get(info.ID).status(); s != StatusComplete {
		t.Fatalf("status %s", s)
	}
}

func TestOrdinaryQueueDoesNotCheckFinishedFiles(t *testing.T) {
	srv := newVersionedServer(t, randomData(64<<10), `"v1"`)
	m, _ := syncManager(t, false)
	info := m.Create(AddOptions{URL: srv.url, Start: true})
	waitFor(t, "the download", func() bool { return m.get(info.ID).status() == StatusComplete })
	srv.set(randomData(80<<10), `"v2"`)
	m.StartQueue(DefaultQueue)
	time.Sleep(200 * time.Millisecond)
	if got := m.get(info.ID).Info(); got.Status != StatusComplete || got.Total != 64<<10 {
		t.Fatalf("an ordinary queue touched a finished file: %s, %d bytes", got.Status, got.Total)
	}
}

func TestServerFileChanged(t *testing.T) {
	cases := []struct {
		etag, mod string
		total     int64
		p         Probe
		want      bool
	}{
		{`"a"`, "", 10, Probe{ETag: `"a"`, Total: 99}, false}, // ETag decides
		{`"a"`, "", 10, Probe{ETag: `"b"`, Total: 10}, true},
		{"", "Mon", 10, Probe{LastModified: "Tue", Total: 10}, true},
		{"", "Mon", 10, Probe{LastModified: "Mon", Total: 10}, false},
		{"", "", 10, Probe{Total: 11}, true},
		{"", "", 10, Probe{Total: 10}, false},
		{"", "", 10, Probe{}, false}, // nothing to compare: unchanged
	}
	for i, c := range cases {
		p := c.p
		if got := serverFileChanged(c.etag, c.mod, c.total, &p); got != c.want {
			t.Errorf("case %d: %v, want %v", i, got, c.want)
		}
	}
}

// A destination held open for a moment (an antivirus, a player with the old
// copy) used to fail the download at the final rename on Windows.
func TestFinishWaitsForAFileHeldOpen(t *testing.T) {
	dir := t.TempDir()
	final := filepath.Join(dir, "clip.mp4")
	if err := os.WriteFile(final, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(final+".odmpart", []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	held, err := os.Open(final)
	if err != nil {
		t.Fatal(err)
	}
	go func() { time.Sleep(600 * time.Millisecond); held.Close() }()

	d := &Download{Dir: dir, FileName: "clip.mp4", replace: true}
	if err := d.finish(); err != nil {
		t.Fatalf("finish gave up on a briefly held file: %v", err)
	}
	if got, _ := os.ReadFile(final); string(got) != "new" {
		t.Fatalf("final file holds %q", got)
	}
}
