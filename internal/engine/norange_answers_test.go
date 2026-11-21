package engine

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

// Without ranges, a short answer (a page saying the link expired) is not
// progress: each attempt starts again at the first byte. It is retried
// with the usual backoff and limit, not asked for again at once for ever.
func TestNoRangeShortAnswerIsRetriedWithALimit(t *testing.T) {
	full := randomData(64 << 10)
	short := []byte("<html><body>This link has expired.</body></html>")
	var transfers atomic.Int64
	srv := httpServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") != "" { // the probe: no ranges, the whole file
			w.Header().Set("Content-Length", strconv.Itoa(len(full)))
			_, _ = w.Write(full)
			return
		}
		transfers.Add(1)
		w.Header().Set("Content-Length", strconv.Itoa(len(short)))
		_, _ = w.Write(short)
	})
	d := New("short200", srv.URL+"/get.php", t.TempDir(), 4, nil, 0, srv.Client())
	t.Cleanup(func() { closePart(d) })
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	d.Start(ctx)
	if n := transfers.Load(); n > 2*MaxRetries {
		t.Fatalf("%d whole-file requests in 2 s (status %s)", n, d.Info().Status)
	}
	if info := d.Info(); info.Status == StatusComplete {
		t.Fatal("a page in place of the file finished the download")
	}
}

// Without ranges, an answer of another length and validator is another
// file: it is not cut to the old size and kept as complete.
func TestNoRangeAnswerOfAnotherFileIsNotCut(t *testing.T) {
	v1 := randomData(64 << 10)
	v2 := randomData(96 << 10) // the file changes right after the probe
	var probed atomic.Bool
	srv := httpServer(t, func(w http.ResponseWriter, r *http.Request) {
		body, etag := v1, `"v1"`
		if probed.Load() {
			body, etag = v2, `"v2"`
		}
		if r.Header.Get("Range") != "" {
			probed.Store(true)
		}
		w.Header().Set("ETag", etag)
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		_, _ = w.Write(body)
	})
	dir := t.TempDir()
	d := New("grew", srv.URL+"/report.csv", dir, 4, nil, 0, srv.Client())
	d.retryBase = time.Millisecond
	t.Cleanup(func() { closePart(d) })
	d.Start(context.Background())
	info := d.Info()
	got, _ := os.ReadFile(filepath.Join(dir, info.FileName))
	if info.Status == StatusComplete && !bytes.Equal(got, v2) {
		t.Fatalf("complete with %d bytes; the file on the server has %d", len(got), len(v2))
	}
}
