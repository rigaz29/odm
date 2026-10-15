package engine

import (
	"bytes"
	"context"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

// noRangeServer never honours Range. The first transfer request (one
// without a Range header) stops after cut bytes; later ones send it all.
func noRangeServer(t *testing.T, data []byte, cut int) (*httptest.Server, *int64) {
	t.Helper()
	var transfers int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		if r.Header.Get("Range") != "" { // the probe
			_, _ = w.Write(data)
			return
		}
		if atomic.AddInt64(&transfers, 1) == 1 && cut > 0 {
			_, _ = w.Write(data[:cut])
			return
		}
		_, _ = w.Write(data)
	}))
	t.Cleanup(srv.Close)
	return srv, &transfers
}

// randomData does not repeat, so bytes written at a wrong offset show up.
func randomData(n int) []byte {
	b := make([]byte, n)
	rand.New(rand.NewSource(int64(n))).Read(b)
	return b
}

func TestNoRangeRetryStartsFromFirstByte(t *testing.T) {
	data := randomData(300 << 10)
	srv, transfers := noRangeServer(t, data, 100<<10)
	dir := t.TempDir()
	d := New("nr1", srv.URL+"/norange.bin", dir, 4, nil, 0, srv.Client())
	d.Start(context.Background())
	info := d.Info()
	if info.Status != StatusComplete {
		t.Fatalf("status %s (%s)", info.Status, info.Error)
	}
	if n := atomic.LoadInt64(transfers); n < 2 {
		t.Fatalf("%d transfer requests, want a retry", n)
	}
	got, err := os.ReadFile(filepath.Join(dir, info.FileName))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Fatal("the retried file does not match the source")
	}
}

func TestNoRangeResumeStartsFromFirstByte(t *testing.T) {
	data := randomData(200 << 10)
	srv, _ := noRangeServer(t, data, 0)
	dir := t.TempDir()
	const half = 100 << 10
	tmp := filepath.Join(dir, "resume.bin.odmpart")
	if err := os.WriteFile(tmp, data[:half], 0o644); err != nil {
		t.Fatal(err)
	}
	st := State{
		ID: "nr2", URL: srv.URL + "/resume.bin", FinalURL: srv.URL + "/resume.bin",
		FileName: "resume.bin", Dir: dir, Connections: 4, Total: int64(len(data)),
		RangeOK: false, Status: StatusPaused, CreatedAt: time.Now(),
		Chunks: []*Chunk{{ID: 0, Start: 0, End: int64(len(data)) - 1, Done: half}},
	}
	d := Restore(st, srv.Client())
	d.Start(context.Background())
	info := d.Info()
	if info.Status != StatusComplete {
		t.Fatalf("status %s (%s)", info.Status, info.Error)
	}
	got, err := os.ReadFile(filepath.Join(dir, "resume.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Fatal("the resumed file does not match the source")
	}
}
