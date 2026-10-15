package engine

import (
	"bytes"
	"context"
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
