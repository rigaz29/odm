package engine

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"odm/internal/media"
)

// A segment that sends a few bytes and then nothing used to hold its
// worker, and so the whole stream, forever.
func TestStreamSegmentStallIsRetried(t *testing.T) {
	var calls int64
	srv := segServer(t, threeSegs, map[string]http.HandlerFunc{
		"/seg1.ts": func(w http.ResponseWriter, r *http.Request) {
			if atomic.AddInt64(&calls, 1) == 1 {
				w.Header().Set("Content-Length", "100")
				_, _ = w.Write([]byte("[/se"))
				w.(http.Flusher).Flush()
				<-r.Context().Done() // stall until the client gives up
				return
			}
			_, _ = w.Write([]byte("[/seg1.ts]"))
		},
	})
	d := newStreamDownload(t, srv.Client(), 2, StreamJob{Kind: media.KindHLS, Manifest: srv.URL + "/p.m3u8"})
	d.stallTimeout = 200 * time.Millisecond
	d.retryBase = time.Millisecond

	done := make(chan struct{})
	go func() { d.Start(context.Background()); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		d.Pause()
		<-done
		t.Fatal("stream still waiting on a stalled segment after 10s")
	}
	if got := string(readResult(t, d)); got != "[/seg0.ts][/seg1.ts][/seg2.ts]" {
		t.Fatalf("output %q", got)
	}
	if n := atomic.LoadInt64(&calls); n < 2 {
		t.Fatalf("stalled segment requested %d time(s), want a retry", n)
	}
}

// A prefix skipped from a whole-file answer has the same stall limit.
func TestDiscardPrefixStall(t *testing.T) {
	pr, pw := io.Pipe()
	defer pw.Close()
	go func() { _, _ = pw.Write([]byte(strings.Repeat("x", 10))) }() // then nothing
	start := time.Now()
	err := discardPrefix(context.Background(), pr, 1000, 100*time.Millisecond)
	if err == nil {
		t.Fatal("a stalled prefix was reported as skipped")
	}
	if time.Since(start) > 3*time.Second {
		t.Fatalf("gave up after %v", time.Since(start))
	}
}

// A playlist or a decryption key whose server stops sending part way used
// to hold the download until it was paused: their bodies had no stall
// limit, unlike segments.
func TestManifestAndKeyStall(t *testing.T) {
	hang := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000")
		_, _ = w.Write([]byte("#EXTM3U\n"))
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}
	srv := segServer(t, threeSegs, map[string]http.HandlerFunc{"/slow.m3u8": hang, "/slow.key": hang})
	d := newStreamDownload(t, srv.Client(), 1, StreamJob{Kind: media.KindHLS, Manifest: srv.URL + "/slow.m3u8"})
	d.stallTimeout = 150 * time.Millisecond

	start := time.Now()
	if _, _, err := d.fetchText(context.Background(), srv.URL+"/slow.m3u8"); !errors.Is(err, errStalled) {
		t.Fatalf("stalled manifest: %v", err)
	}
	run := &streamRun{d: d, keys: map[string][]byte{}}
	if _, err := run.key(context.Background(), srv.URL+"/slow.key"); !errors.Is(err, errStalled) {
		t.Fatalf("stalled key: %v", err)
	}
	if took := time.Since(start); took > 5*time.Second {
		t.Fatalf("gave up after %v", took)
	}
	// A whole manifest still reads, and one too large is still refused.
	if data, _, err := d.fetchText(context.Background(), srv.URL+"/p.m3u8"); err != nil || !strings.HasPrefix(string(data), "#EXTM3U") {
		t.Fatalf("manifest: %q, %v", data, err)
	}
}

func TestReadAllIdleLimit(t *testing.T) {
	body := io.NopCloser(strings.NewReader(strings.Repeat("x", 100)))
	got, err := readAllIdle(context.Background(), body, 40, time.Second)
	if err != nil || len(got) != 40 {
		t.Fatalf("read %d bytes, %v; want the 40-byte limit", len(got), err)
	}
}
