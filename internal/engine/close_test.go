package engine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

// TestPauseClosesPartialFile pauses a transfer, checks that the run left
// no handle on the .odmpart, and resumes it on the same download. On
// Windows a handle left open makes the final rename fail.
func TestPauseClosesPartialFile(t *testing.T) {
	data := randomData(1 << 20)
	var hold atomic.Bool
	hold.Store(true)
	served := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")
		rng := r.Header.Get("Range")
		start, end := 0, len(data)-1
		if spec := strings.TrimPrefix(rng, "bytes="); spec != rng {
			parts := strings.SplitN(spec, "-", 2)
			start, _ = strconv.Atoi(parts[0])
			if v, err := strconv.Atoi(parts[1]); err == nil && v < end {
				end = v
			}
		}
		body := data[start : end+1]
		if rng != "" {
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(data)))
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		if rng != "" {
			w.WriteHeader(http.StatusPartialContent)
		}
		// A transfer request (bounded range, not the one-byte probe) sends
		// a little and then waits, so the test can pause mid-file.
		transfer := rng != "" && !strings.HasSuffix(rng, "-") && rng != "bytes=0-0"
		if transfer && hold.Load() {
			_, _ = w.Write(body[:4096])
			w.(http.Flusher).Flush()
			select {
			case served <- struct{}{}:
			default:
			}
			<-r.Context().Done()
			return
		}
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	dir := t.TempDir()
	d := New("c1", srv.URL+"/part.bin", dir, 1, nil, 0, srv.Client())
	done := make(chan struct{})
	go func() { d.Start(context.Background()); close(done) }()
	<-served
	d.Pause()
	<-done
	if s := d.status(); s != StatusPaused {
		t.Fatalf("after pause: status %s (%s)", s, d.Err)
	}
	d.mu.Lock()
	open := d.file != nil
	d.mu.Unlock()
	if open {
		t.Fatal("the paused run left the .odmpart open")
	}

	hold.Store(false)
	d.Start(context.Background())
	info := d.Info()
	if info.Status != StatusComplete {
		t.Fatalf("resume: status %s (%s)", info.Status, info.Error)
	}
	got, err := os.ReadFile(filepath.Join(dir, info.FileName))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Fatal("resumed file does not match the source")
	}
	if _, err := os.Stat(filepath.Join(dir, info.FileName+".odmpart")); !os.IsNotExist(err) {
		t.Fatalf(".odmpart still present: %v", err)
	}
}

// A finished file is handed to MarkFile with where it came from; a file the
// mark's scan removed fails the download.
func TestFinishMarksTheFile(t *testing.T) {
	data := randomData(64 << 10)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { serveRange(w, r, data) }))
	defer srv.Close()
	defer func() { MarkFile = nil }()

	var gotPath, gotSource, gotRef string
	MarkFile = func(path, source, referrer string) error {
		gotPath, gotSource, gotRef = path, source, referrer
		return nil
	}
	dir := t.TempDir()
	d := New("m1", srv.URL+"/a.bin", dir, 1, map[string]string{"referer": "https://site.example/page"}, 0, srv.Client())
	d.Start(context.Background())
	if info := d.Info(); info.Status != StatusComplete {
		t.Fatalf("status %s: %s", info.Status, info.Error)
	}
	if gotPath != filepath.Join(dir, "a.bin") || gotSource != srv.URL+"/a.bin" || gotRef != "https://site.example/page" {
		t.Fatalf("marked %q from %q via %q", gotPath, gotSource, gotRef)
	}

	MarkFile = func(path, _, _ string) error {
		_ = os.Remove(path) // what an antivirus program does with malware
		return errors.New("removed")
	}
	d2 := New("m2", srv.URL+"/b.bin", dir, 1, nil, 0, srv.Client())
	d2.Start(context.Background())
	if info := d2.Info(); info.Status != StatusError || !strings.Contains(info.Error, "antivirus") {
		t.Fatalf("a removed file ended as %s (%q)", info.Status, info.Error)
	}

	MarkFile = func(string, string, string) error { return errors.New("no streams on FAT") }
	d3 := New("m3", srv.URL+"/c.bin", dir, 1, nil, 0, srv.Client())
	d3.Start(context.Background())
	if info := d3.Info(); info.Status != StatusComplete {
		t.Fatalf("a file that could not be marked failed: %s (%q)", info.Status, info.Error)
	}
}
