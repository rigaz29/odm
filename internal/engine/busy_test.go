package engine

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"odm/internal/media"
)

func TestRetryAfter(t *testing.T) {
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		v    string
		want time.Duration
	}{
		{"", 0},
		{"5", 5 * time.Second},
		{" 120 ", 2 * time.Minute},
		{"0", 0},
		{"-3", 0},
		{"Thu, 15 Oct 2026 12:00:30 GMT", 30 * time.Second},
		{"Thu, 15 Oct 2026 11:59:00 GMT", 0}, // in the past
		{"soon", 0},
	}
	for _, c := range cases {
		if got := retryAfter(c.v, now); got != c.want {
			t.Errorf("retryAfter(%q) = %v, want %v", c.v, got, c.want)
		}
	}
	if got := retryWait(&busyError{wait: time.Hour}, time.Second); got != maxRetryAfter {
		t.Errorf("an hour is capped at %v, got %v", maxRetryAfter, got)
	}
	if got := retryWait(&busyError{}, 300*time.Millisecond); got != 300*time.Millisecond {
		t.Errorf("busy without Retry-After waits the backoff, got %v", got)
	}
}

func httpServer(t *testing.T, h http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

func TestBusyTransferHonoursRetryAfter(t *testing.T) {
	data := randomData(256 << 10)
	var refused int64
	var refusedAt, retriedAt time.Time
	srv := httpServer(t, func(w http.ResponseWriter, r *http.Request) {
		rng := r.Header.Get("Range")
		if rng != "" && rng != "bytes=0-0" && atomic.AddInt64(&refused, 1) == 1 {
			refusedAt = time.Now()
			w.Header().Set("Retry-After", "1")
			http.Error(w, "slow down", http.StatusTooManyRequests)
			return
		}
		if rng != "" && rng != "bytes=0-0" && retriedAt.IsZero() {
			retriedAt = time.Now()
		}
		serveRange(w, r, data)
	})
	dir := t.TempDir()
	d := New("busy1", srv.URL+"/f.bin", dir, 1, nil, 0, srv.Client())
	d.retryBase = time.Millisecond // the backoff alone would retry at once
	d.Start(context.Background())
	if info := d.Info(); info.Status != StatusComplete {
		t.Fatalf("status %s (%s)", info.Status, info.Error)
	}
	if gap := retriedAt.Sub(refusedAt); gap < 900*time.Millisecond {
		t.Fatalf("retried %v after the 429, want about the 1s Retry-After", gap)
	}
	got, _ := os.ReadFile(filepath.Join(dir, d.Info().FileName))
	if !bytes.Equal(got, data) {
		t.Fatal("file does not match")
	}
}

func TestBusyProbeIsRetried(t *testing.T) {
	data := randomData(128 << 10)
	var probes int64
	srv := httpServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") == "bytes=0-0" && atomic.AddInt64(&probes, 1) == 1 {
			w.Header().Set("Retry-After", "1")
			http.Error(w, "maintenance", http.StatusServiceUnavailable)
			return
		}
		serveRange(w, r, data)
	})
	d := New("busy2", srv.URL+"/f.bin", t.TempDir(), 2, nil, 0, srv.Client())
	d.retryBase = time.Millisecond
	d.Start(context.Background())
	if info := d.Info(); info.Status != StatusComplete {
		t.Fatalf("a 503 on the first probe failed the download: %s (%s)", info.Status, info.Error)
	}
	if n := atomic.LoadInt64(&probes); n < 2 {
		t.Fatalf("probe sent %d time(s), want a retry", n)
	}
}

func TestBusyStreamSegmentIsRetried(t *testing.T) {
	var calls int64
	srv := segServer(t, threeSegs, map[string]http.HandlerFunc{
		"/seg1.ts": func(w http.ResponseWriter, r *http.Request) {
			if atomic.AddInt64(&calls, 1) == 1 {
				w.Header().Set("Retry-After", "1")
				http.Error(w, "slow down", http.StatusTooManyRequests)
				return
			}
			_, _ = w.Write([]byte("[/seg1.ts]"))
		},
	})
	d := newStreamDownload(t, srv.Client(), 2, StreamJob{Kind: media.KindHLS, Manifest: srv.URL + "/p.m3u8"})
	start := time.Now()
	d.Start(context.Background())
	if got := string(readResult(t, d)); got != "[/seg0.ts][/seg1.ts][/seg2.ts]" {
		t.Fatalf("output %q", got)
	}
	if time.Since(start) < 900*time.Millisecond {
		t.Fatal("the segment was retried before its Retry-After")
	}
}

// serveRange answers r from data, honouring a single Range.
func serveRange(w http.ResponseWriter, r *http.Request, data []byte) {
	w.Header().Set("Accept-Ranges", "bytes")
	start, end, whole := requestedSpan(r.Header.Get("Range"), len(data))
	if whole && r.Header.Get("Range") == "" {
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		_, _ = w.Write(data)
		return
	}
	if end >= len(data) {
		end = len(data) - 1
	}
	body := data[start : end+1]
	w.Header().Set("Content-Range", "bytes "+strconv.Itoa(start)+"-"+strconv.Itoa(end)+"/"+strconv.Itoa(len(data)))
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(http.StatusPartialContent)
	_, _ = w.Write(body)
}
