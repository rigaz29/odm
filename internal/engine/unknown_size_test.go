package engine

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// chunkedServer sends data without a Content-Length (chunked encoding),
// ignoring Range, as a generated archive would. The first transfer stops
// part way when cut is above 0.
func chunkedServer(t *testing.T, data []byte, cut int) (string, *int64) {
	t.Helper()
	var transfers int64
	srv := httpServer(t, func(w http.ResponseWriter, r *http.Request) {
		fl := w.(http.Flusher)
		body := data
		if r.Header.Get("Range") == "" && atomic.AddInt64(&transfers, 1) == 1 && cut > 0 {
			_, _ = w.Write(data[:cut])
			fl.Flush()
			// Hijack and drop the connection: the chunked body has no end.
			if hj, ok := w.(http.Hijacker); ok {
				c, _, _ := hj.Hijack()
				_ = c.Close()
			}
			return
		}
		for len(body) > 0 {
			n := 32 << 10
			if n > len(body) {
				n = len(body)
			}
			_, _ = w.Write(body[:n])
			fl.Flush() // chunked: no Content-Length is sent
			body = body[n:]
		}
	})
	return srv.URL, &transfers
}

func TestDownloadWithoutContentLength(t *testing.T) {
	data := randomData(300 << 10)
	base, _ := chunkedServer(t, data, 0)
	p, err := Inspect(context.Background(), http.DefaultClient, base+"/archive.zip", nil)
	if err != nil {
		t.Fatalf("probe refused a response without a size: %v", err)
	}
	if p.Total != 0 || p.RangeOK {
		t.Fatalf("probe: total %d, ranges %v; want unknown size on one stream", p.Total, p.RangeOK)
	}

	dir := t.TempDir()
	d := New("u1", base+"/archive.zip", dir, 8, nil, 0, http.DefaultClient)
	d.Start(context.Background())
	info := d.Info()
	if info.Status != StatusComplete {
		t.Fatalf("status %s (%s)", info.Status, info.Error)
	}
	if info.Total != int64(len(data)) {
		t.Fatalf("size after the download %d, want %d", info.Total, len(data))
	}
	got, _ := os.ReadFile(filepath.Join(dir, info.FileName))
	if !bytes.Equal(got, data) {
		t.Fatal("file does not match")
	}
}

func TestDownloadWithoutContentLengthRetriesACutBody(t *testing.T) {
	data := randomData(300 << 10)
	base, transfers := chunkedServer(t, data, 100<<10)
	dir := t.TempDir()
	d := New("u2", base+"/archive.zip", dir, 4, nil, 0, http.DefaultClient)
	d.retryBase = time.Millisecond
	d.Start(context.Background())
	info := d.Info()
	if info.Status != StatusComplete {
		t.Fatalf("status %s (%s)", info.Status, info.Error)
	}
	if n := atomic.LoadInt64(transfers); n < 2 {
		t.Fatalf("%d transfers; a cut body must be fetched again", n)
	}
	got, _ := os.ReadFile(filepath.Join(dir, info.FileName))
	if !bytes.Equal(got, data) {
		t.Fatalf("file does not match: %d bytes, want %d", len(got), len(data))
	}
}

func TestOpenEndedChunk(t *testing.T) {
	c := &Chunk{Start: 0, End: -1, Done: 5000}
	if c.Complete() || c.Remaining() != 0 || c.Size() != 0 {
		t.Fatalf("open chunk: complete %v, remaining %d, size %d", c.Complete(), c.Remaining(), c.Size())
	}
	if _, ok := Split([]*Chunk{c}, MinChunkSize); ok {
		t.Fatal("an open-ended chunk was split")
	}
}
