package engine

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A server that builds the file before it answers (an archive or an
// export made on demand) sends no headers for longer than the shared
// client waits for playlists and pages: a file's probe and transfer wait
// longer.
func TestFileWaitsLongerForHeadersThanPlaylists(t *testing.T) {
	oldShared, oldFile := headerTimeout, fileHeaderTimeout
	headerTimeout, fileHeaderTimeout = 100*time.Millisecond, 5*time.Second
	t.Cleanup(func() { headerTimeout, fileHeaderTimeout = oldShared, oldFile })

	data := randomData(64 * 1024)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(400 * time.Millisecond) // building the file
		http.ServeContent(w, r, "export.zip", time.Unix(1700000000, 0), bytes.NewReader(data))
	}))
	defer srv.Close()
	client := buildClient(Settings{Proxy: ProxyConfig{Mode: ProxyNone}})

	// The shared client gives up on it.
	if _, err := Inspect(context.Background(), client, srv.URL+"/export.zip", nil); err == nil {
		t.Fatal("the shared client waited past its header limit")
	}

	dir := t.TempDir()
	d := New("export", srv.URL+"/export.zip", dir, 2, nil, 0, client)
	d.retryBase = time.Millisecond
	t.Cleanup(func() { closePart(d) })
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	d.Start(ctx)
	info := d.Info()
	if info.Status != StatusComplete {
		t.Fatalf("status=%s err=%q", info.Status, info.Error)
	}
	got, _ := os.ReadFile(filepath.Join(dir, info.FileName))
	if !bytes.Equal(got, data) {
		t.Fatal("downloaded bytes do not match")
	}
	if tr := client.Transport.(*http.Transport); tr.ResponseHeaderTimeout != headerTimeout {
		t.Fatalf("the shared client's limit changed: %v", tr.ResponseHeaderTimeout)
	}
}

// A weak ETag is never sent as If-Range: a server like Go's own would
// answer every range with the whole file. Last-Modified stands in, and
// the ranges are honoured.
func TestWeakETagKeepsRanges(t *testing.T) {
	data := randomData(4 * MinChunkSize)
	var partial, whole, weakIfRange int
	var mu = make(chan struct{}, 1)
	mu <- struct{}{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `W/"v1"`)
		rec := &codeRecorder{ResponseWriter: w}
		http.ServeContent(rec, r, "file.bin", time.Unix(1700000000, 0), bytes.NewReader(data))
		if oneByteProbe(r) {
			return
		}
		<-mu
		if r.Header.Get("If-Range") == `W/"v1"` {
			weakIfRange++
		}
		if rec.code == http.StatusPartialContent {
			partial++
		} else {
			whole++
		}
		mu <- struct{}{}
	}))
	defer srv.Close()

	dir := t.TempDir()
	d := New("weak", srv.URL+"/file.bin", dir, 4, nil, 0, srv.Client())
	d.retryBase = time.Millisecond
	t.Cleanup(func() { closePart(d) })
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	d.Start(ctx)
	info := d.Info()
	if info.Status != StatusComplete {
		t.Fatalf("status=%s err=%q", info.Status, info.Error)
	}
	<-mu
	if weakIfRange != 0 || whole != 0 || partial < 2 {
		t.Fatalf("weak If-Range %d, whole answers %d, ranged answers %d", weakIfRange, whole, partial)
	}
	got, _ := os.ReadFile(filepath.Join(dir, info.FileName))
	if !bytes.Equal(got, data) {
		t.Fatal("downloaded bytes do not match")
	}
}

type codeRecorder struct {
	http.ResponseWriter
	code int
}

func (c *codeRecorder) WriteHeader(code int) { c.code = code; c.ResponseWriter.WriteHeader(code) }
