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
)

func TestTakeOverRules(t *testing.T) {
	const mb = 1 << 20
	slow := func() *Chunk { return &Chunk{ID: 0, Start: 0, End: mb - 1, Done: 100 << 10, pace: 32 << 10} }
	fast := func() *Chunk { return &Chunk{ID: 1, Start: mb, End: 3*mb - 1, Done: 512 << 10, pace: 2 * mb} }

	// A freed fast connection takes all the slow chunk has left.
	s, f := slow(), fast()
	nc, ok := splitFor([]*Chunk{s, f}, MinChunkSize, 2*mb)
	if !ok || nc.Start != 100<<10 || nc.End != mb-1 || s.End != 100<<10-1 || !s.Complete() {
		t.Fatalf("takeover: new %+v, slow %+v", nc, s)
	}
	if nc.ID != 2 {
		t.Fatalf("new chunk id %d, want 2", nc.ID)
	}

	// Not when the freed connection is only a little faster: halve instead.
	s, f = slow(), fast()
	s.pace = 1 * mb
	if nc, ok := splitFor([]*Chunk{s, f}, MinChunkSize, 2*mb); !ok || nc.Start == s.Next() && s.Complete() {
		t.Fatalf("a connection half as fast was taken over: %+v", nc)
	}

	// Not for a rest the slow connection finishes within takeoverAfter.
	s, f = slow(), fast()
	s.Done = s.Size() - 20<<10 // 20 KB left at 32 KB/s
	if nc := takeOver([]*Chunk{s, f}, 2*mb); nc != nil {
		t.Fatalf("a nearly done chunk was taken over: %+v", nc)
	}

	// A freed slow connection does not take half of a fast one's work.
	s, f = slow(), fast()
	s.Done = s.Size() // it finished its own chunk
	if nc, ok := splitFor([]*Chunk{s, f}, MinChunkSize, 32<<10); ok {
		t.Fatalf("a slow connection took %+v from a fast one", nc)
	}
	// With its pace unknown it still helps, as before.
	if _, ok := splitFor([]*Chunk{s, fast()}, MinChunkSize, 0); !ok {
		t.Fatal("a connection of unknown pace got no work")
	}
}

// throttledRanges serves data with each response paced to rate bytes per
// second, except the first response for the start of the file, paced to
// slowRate.
func throttledRanges(t *testing.T, data []byte, rate, slowRate int64) *httptest.Server {
	var slowTaken atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start, end, _ := requestedSpan(r.Header.Get("Range"), len(data))
		if end >= len(data) {
			end = len(data) - 1
		}
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Range", "bytes "+strconv.Itoa(start)+"-"+strconv.Itoa(end)+"/"+strconv.Itoa(len(data)))
		w.Header().Set("Content-Length", strconv.Itoa(end-start+1))
		w.WriteHeader(http.StatusPartialContent)
		pace := rate
		if start == 0 && end > 0 && slowTaken.CompareAndSwap(false, true) {
			pace = slowRate
		}
		body := data[start : end+1]
		began := time.Now()
		for sent := 0; sent < len(body); {
			k := min(8<<10, len(body)-sent)
			if _, err := w.Write(body[sent : sent+k]); err != nil {
				return
			}
			w.(http.Flusher).Flush()
			sent += k
			if d := time.Until(began.Add(time.Duration(float64(sent) / float64(pace) * float64(time.Second)))); d > 0 {
				time.Sleep(d)
			}
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// One connection crawls. The others used to finish and wait while its last
// piece, which halving could not shrink below 2*MinChunkSize, came in at
// its pace: 128 KB at 16 KB/s is eight seconds of nothing else happening.
func TestSlowConnectionIsTakenOver(t *testing.T) {
	data := randomData(6 << 20)
	srv := throttledRanges(t, data, 4<<20, 16<<10)
	dir := t.TempDir()
	d := New("slow", srv.URL+"/file.bin", dir, 4, nil, 0, srv.Client())
	start := time.Now()
	d.Start(context.Background())
	took := time.Since(start)
	info := d.Info()
	if info.Status != StatusComplete {
		t.Fatalf("status=%s err=%q", info.Status, info.Error)
	}
	got, err := os.ReadFile(filepath.Join(dir, info.FileName))
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("downloaded bytes do not match (%v)", err)
	}
	if took > 5*time.Second {
		t.Fatalf("took %v: the crawling connection kept its last piece", took)
	}
	t.Logf("6 MB with one connection at 16 KB/s: %v", took)
}
