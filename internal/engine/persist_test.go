package engine

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func stateDone(st State) int64 {
	var done int64
	for _, c := range st.Chunks {
		done += c.Done
	}
	return done
}

func TestPersistFlushesBeforeSaving(t *testing.T) {
	f, err := os.Create(filepath.Join(t.TempDir(), "f.odmpart"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	t.Cleanup(func() { fsync = (*os.File).Sync })
	d := New("p1", "http://h/f", t.TempDir(), 1, nil, 0, nil)
	d.Chunks = []*Chunk{{ID: 0, Start: 0, End: 999}}
	d.file = f
	write := func(n int64) {
		d.mu.Lock()
		d.Chunks[0].Done += n
		d.unsynced = true
		d.mu.Unlock()
	}
	flushes := 0
	var saved []int64
	save := func(st State) { saved = append(saved, stateDone(st)) }
	check := func(step string, wantFlushes int, want ...int64) {
		t.Helper()
		if flushes != wantFlushes || fmt.Sprint(saved) != fmt.Sprint(want) {
			t.Fatalf("%s: %d flushes, saved %v; want %d, %v", step, flushes, saved, wantFlushes, want)
		}
	}

	// Bytes written while the file is flushed are not in the state saved,
	// and wait for the next flush.
	write(100)
	fsync = func(*os.File) error { flushes++; write(50); return nil }
	d.persist(true, save)
	check("first update", 1, 100)
	fsync = func(*os.File) error { flushes++; return nil }
	d.persist(true, save)
	check("progress before the next flush is due", 1, 100)
	d.persist(false, save)
	check("another update", 2, 100, 150)
	d.persist(true, save)
	check("progress with nothing to flush", 2, 100, 150, 150)

	write(10)
	d.persist(true, save)
	check("progress before the next flush is due", 2, 100, 150, 150)
	d.mu.Lock()
	d.rewindLocked(20)
	d.mu.Unlock()
	d.persist(true, save)
	check("progress gone back", 3, 100, 150, 150, 20)

	write(5)
	fsync = func(*os.File) error { flushes++; return errors.New("disk gone") }
	d.persist(false, save)
	check("a failed flush", 4, 100, 150, 150, 20)
	d.mu.Lock()
	unsynced := d.unsynced
	d.mu.Unlock()
	if !unsynced {
		t.Fatal("bytes a failed flush left are not waiting for the next one")
	}
}

// A running download saves only progress that was flushed to the disk:
// each byte a saved state counts was written before a flush.
func TestSavedProgressIsFlushed(t *testing.T) {
	data := randomData(1 << 20)
	next := make(chan struct{})
	served := make(chan struct{})
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
		// A transfer request (a bounded range, not the probe) sends two
		// pieces, the second once the test saw progress saved, and then
		// nothing until the download is paused. Each piece comes after
		// notify's 250 ms throttle, so its update reaches persist.
		if rng == "" || strings.HasSuffix(rng, "-") || rng == "bytes=0-0" {
			_, _ = w.Write(body)
			return
		}
		time.Sleep(300 * time.Millisecond)
		_, _ = w.Write(body[:4096])
		w.(http.Flusher).Flush()
		select {
		case <-next:
		case <-r.Context().Done():
			return
		}
		time.Sleep(300 * time.Millisecond)
		_, _ = w.Write(body[4096:8192])
		w.(http.Flusher).Flush()
		close(served)
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)

	d := New("p2", srv.URL+"/part.bin", t.TempDir(), 1, nil, 0, srv.Client())
	var mu sync.Mutex
	var flushed int64 // bytes done when the file was last flushed
	var saved []int64
	var bad []string
	t.Cleanup(func() { fsync = (*os.File).Sync })
	fsync = func(f *os.File) error {
		done := d.doneBytes()
		mu.Lock()
		flushed = done
		mu.Unlock()
		return f.Sync()
	}
	d.SetOnUpdate(func(d *Download, progress bool) {
		d.persist(progress, func(st State) {
			done := stateDone(st)
			mu.Lock()
			defer mu.Unlock()
			if done > flushed {
				bad = append(bad, fmt.Sprintf("%d saved, %d flushed", done, flushed))
			}
			saved = append(saved, done)
		})
	})
	savedDone := func() int64 {
		mu.Lock()
		defer mu.Unlock()
		if len(saved) == 0 {
			return 0
		}
		return saved[len(saved)-1]
	}

	done := make(chan struct{})
	go func() { d.Start(context.Background()); close(done) }()
	t.Cleanup(func() { d.Pause(); <-done })
	waitFor(t, "progress saved", func() bool { return savedDone() > 0 })
	first := savedDone()
	close(next)
	<-served
	waitFor(t, "the second piece written", func() bool { return d.doneBytes() == 8192 })
	time.Sleep(50 * time.Millisecond)
	if got := savedDone(); got != first {
		t.Fatalf("progress saved before the next flush was due: %d, then %d", first, got)
	}
	d.Pause()
	<-done
	mu.Lock()
	defer mu.Unlock()
	if len(bad) > 0 {
		t.Fatalf("saved progress the disk may not have: %v", bad)
	}
	if got := saved[len(saved)-1]; got != 8192 {
		t.Fatalf("the pause saved %d bytes done, want 8192", got)
	}
}
