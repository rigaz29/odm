package gui

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"time"

	"odm/internal/engine"
)

func newTestService(t *testing.T) (*Service, *engine.Manager) {
	t.Helper()
	mgr := engine.NewManager(engine.Settings{Dir: t.TempDir()}, "")
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	mgr.Begin(ctx)
	return NewService(mgr), mgr
}

func TestSnapshot(t *testing.T) {
	svc, mgr := newTestService(t)
	mgr.Create(engine.AddOptions{URL: "http://127.0.0.1:9/a/movie.mp4", FileName: "movie.mp4", Description: "note"})
	snap := svc.Snapshot()
	if len(snap.Items) != 1 {
		t.Fatalf("%d items", len(snap.Items))
	}
	d := snap.Items[0]
	if d.FileName != "movie.mp4" || d.Category != "Video" || d.Status != "paused" || d.Description != "note" || d.Queue == "" {
		t.Fatalf("item %+v", d)
	}
	if len(snap.Categories) == 0 || len(snap.Queues) == 0 {
		t.Fatalf("snapshot %+v", snap)
	}
}

// The page hears of a change once: watch sends what changed when the
// snapshot differs from the last one, and at once when an action pokes it.
func TestWatchSendsChanges(t *testing.T) {
	svc, mgr := newTestService(t)
	var mu sync.Mutex
	var sent []Change
	svc.emit = func(name string, data any) {
		if name != SnapshotEvent {
			t.Errorf("event %q", name)
		}
		mu.Lock()
		sent = append(sent, data.(Change))
		mu.Unlock()
	}
	count := func() int {
		mu.Lock()
		defer mu.Unlock()
		return len(sent)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go svc.watch(ctx, time.Hour) // no ticks: only the first send and pokes

	waitFor := func(n int) {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for count() < n {
			if time.Now().After(deadline) {
				t.Fatalf("%d snapshots sent, want %d", count(), n)
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	waitFor(1)
	svc.poke() // nothing changed: nothing sent
	time.Sleep(50 * time.Millisecond)
	if count() != 1 {
		t.Fatalf("an unchanged snapshot was sent again (%d)", count())
	}
	d := mgr.Create(engine.AddOptions{URL: "http://127.0.0.1:9/b.zip", FileName: "b.zip"})
	svc.Remove([]string{d.ID}, false) // pokes; the list is back as it was
	svc.poke()
	time.Sleep(50 * time.Millisecond)
	mgr.Create(engine.AddOptions{URL: "http://127.0.0.1:9/c.zip", FileName: "c.zip"})
	svc.poke()
	waitFor(2)
	mu.Lock()
	last := sent[len(sent)-1]
	mu.Unlock()
	if len(last.Items) != 1 || last.Items[0].FileName != "c.zip" || len(last.IDs) != 1 {
		t.Fatalf("last change %+v", last)
	}
}

// A change carries the downloads that changed, and the list's order only
// when that changed.
func TestChangeCarriesWhatChanged(t *testing.T) {
	svc, mgr := newTestService(t)
	var got []Change
	svc.emit = func(_ string, data any) { got = append(got, data.(Change)) }
	a := mgr.Create(engine.AddOptions{URL: "http://127.0.0.1:9/a.zip", FileName: "a.zip"})
	b := mgr.Create(engine.AddOptions{URL: "http://127.0.0.1:9/b.zip", FileName: "b.zip"})
	names := func(c Change) string {
		var s []string
		for _, d := range c.Items {
			s = append(s, d.FileName)
		}
		return fmt.Sprint(s, c.IDs != nil)
	}

	svc.send()
	if len(got) != 1 || names(got[0]) != "[a.zip b.zip] true" || got[0].IDs[0] != a.ID {
		t.Fatalf("first change %+v", got)
	}
	svc.send()
	if len(got) != 1 {
		t.Fatalf("nothing changed, yet %d changes", len(got))
	}
	mgr.AddQueue("Night")
	svc.MoveToQueue([]string{b.ID}, "Night")
	svc.send()
	if len(got) != 2 || names(got[1]) != "[b.zip] false" || got[1].Items[0].Queue != "Night" || !slices.Contains(got[1].Queues, "Night") {
		t.Fatalf("after moving b: %+v", got[len(got)-1])
	}
	svc.Remove([]string{a.ID}, false)
	svc.send()
	if len(got) != 3 || names(got[2]) != "[] true" || !slices.Equal(got[2].IDs, []string{b.ID}) {
		t.Fatalf("after removing a: %+v", got[len(got)-1])
	}
	svc.Remove([]string{b.ID}, false)
	svc.send()
	if len(got) != 4 || got[3].IDs == nil || len(got[3].IDs) != 0 {
		t.Fatalf("an emptied list must say so: %+v", got[len(got)-1])
	}
}

func TestOpenUnknown(t *testing.T) {
	svc, _ := newTestService(t)
	if err := svc.OpenFile("nope"); err == nil {
		t.Fatal("an unknown download was opened")
	}
	if err := svc.ShowInFolder("nope"); err == nil {
		t.Fatal("an unknown download was shown")
	}
}

func TestQueueAndLimiterActions(t *testing.T) {
	svc, mgr := newTestService(t)
	mgr.AddQueue("Night")
	a := mgr.Create(engine.AddOptions{URL: "http://127.0.0.1:9/a.zip", FileName: "a.zip"})
	b := mgr.Create(engine.AddOptions{URL: "http://127.0.0.1:9/b.zip", FileName: "b.zip"})
	svc.MoveToQueue([]string{a.ID, b.ID}, "Night")
	svc.MoveInQueue(b.ID, -1)
	snap := svc.Snapshot()
	if len(snap.Items) != 2 || snap.Items[0].Queue != "Night" || snap.Items[1].Queue != "Night" {
		t.Fatalf("items %+v", snap.Items)
	}
	if ia, ib := snap.Items[0], snap.Items[1]; ib.Order >= ia.Order {
		t.Fatalf("b was not moved up: orders %d and %d", ia.Order, ib.Order)
	}

	if snap.Limit != 0 || snap.SavedLimit != 0 {
		t.Fatalf("limit %d, saved %d before any was set", snap.Limit, snap.SavedLimit)
	}
	svc.SetSpeedLimiter(true, 300<<10)
	if snap := svc.Snapshot(); snap.Limit != 300<<10 || snap.SavedLimit != 300<<10 {
		t.Fatalf("limit %d, saved %d after turning it on", snap.Limit, snap.SavedLimit)
	}
	svc.SetSpeedLimiter(false, 0)
	if snap := svc.Snapshot(); snap.Limit != 0 || snap.SavedLimit != 300<<10 {
		t.Fatalf("limit %d, saved %d after turning it off", snap.Limit, snap.SavedLimit)
	}

	svc.Remove([]string{a.ID}, false)
	if snap := svc.Snapshot(); len(snap.Items) != 1 || snap.Items[0].ID != b.ID {
		t.Fatalf("items %+v after removing a", snap.Items)
	}
}

func TestResolveAppearance(t *testing.T) {
	for _, c := range []struct{ choice, system, want string }{
		{"id", "en-US", "id"},
		{"en", "id-ID", "en"},
		{"system", "id-ID", "id"},
		{"", "in_ID", "id"},
		{"system", "en-GB", "en"},
		{"", "", "en"},
	} {
		if got := resolveLanguage(c.choice, c.system); got != c.want {
			t.Errorf("resolveLanguage(%q, %q) = %q, want %q", c.choice, c.system, got, c.want)
		}
	}
	if resolveTheme("dark") != "dark" || resolveTheme("light") != "light" || resolveTheme("") != "system" || resolveTheme("odd") != "system" {
		t.Fatal("resolveTheme")
	}
}

func TestConnProgress(t *testing.T) {
	if connProgress(nil) != nil {
		t.Fatal("no chunks gave bars")
	}
	got := connProgress([]engine.ChunkInfo{
		{Done: 50, Remaining: 50},
		{Complete: true},
		{Done: 0, Remaining: 0},
	})
	if len(got) != 3 || got[0] != 0.5 || got[1] != 1 || got[2] != 0 {
		t.Fatalf("%v", got)
	}
	// More connections than bars: neighbours are averaged.
	many := make([]engine.ChunkInfo, 32)
	for i := range many {
		if i%2 == 0 {
			many[i].Complete = true
		}
	}
	bars := connProgress(many)
	if len(bars) != maxConns || bars[0] != 0.5 || bars[maxConns-1] != 0.5 {
		t.Fatalf("%d bars: %v", len(bars), bars)
	}
}

// sample keeps a minute of speeds per running download and in total, and
// when each started; a download that stops loses both.
func TestSampleSpeeds(t *testing.T) {
	svc, mgr := newTestService(t)
	clock := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return clock }
	d := mgr.Create(engine.AddOptions{URL: "http://127.0.0.1:9/x.bin", FileName: "x.bin"})

	svc.sample() // paused: nothing to keep
	if len(svc.speeds) != 0 || len(svc.since) != 0 || len(svc.total) != 1 {
		t.Fatalf("speeds %v since %v total %v", svc.speeds, svc.since, svc.total)
	}
	for i := 0; i < speedSamples+5; i++ {
		svc.total = appendSample(svc.total, int64(i))
	}
	if len(svc.total) != speedSamples || svc.total[len(svc.total)-1] != speedSamples+4 {
		t.Fatalf("total history %d long, last %d", len(svc.total), svc.total[len(svc.total)-1])
	}

	// A running download, as the engine lists it, by hand.
	svc.hmu.Lock()
	svc.speeds[d.ID] = []int64{1, 2}
	svc.since[d.ID] = clock
	svc.hmu.Unlock()
	svc.sample() // still paused in the engine: forgotten
	if _, ok := svc.speeds[d.ID]; ok {
		t.Fatal("a stopped download kept its speeds")
	}
	if _, ok := svc.since[d.ID]; ok {
		t.Fatal("a stopped download kept its start time")
	}
	if snap := svc.Snapshot(); len(snap.Speeds) != speedSamples+1-1 || snap.Items[0].Speeds != nil || !snap.Items[0].ActiveSince.IsZero() {
		t.Fatalf("snapshot %d total samples, item %+v", len(snap.Speeds), snap.Items[0])
	}
}

// A download that really runs gets its connections' bars, its speeds and
// its start time in the snapshot.
func TestSnapshotOfARunningDownload(t *testing.T) {
	body := bytes.Repeat([]byte("x"), 4<<20)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Slow, so the download is still running when it is sampled.
		w = &slowWriter{ResponseWriter: w}
		http.ServeContent(w, r, "big.bin", time.Time{}, bytes.NewReader(body))
	}))
	t.Cleanup(srv.Close)
	svc, mgr := newTestService(t)
	d := mgr.Create(engine.AddOptions{URL: srv.URL + "/big.bin", FileName: "big.bin", Start: true, Connections: 4})
	t.Cleanup(func() { mgr.Pause(d.ID) })
	deadline := time.Now().Add(5 * time.Second)
	for {
		if got, _ := mgr.Get(d.ID); got.Status == engine.StatusDownloading && got.Done > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the download did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	svc.sample()
	svc.sample()
	snap := svc.Snapshot()
	it := snap.Items[0]
	if it.Status != "downloading" || len(it.Speeds) != 2 || it.ActiveSince.IsZero() || len(it.Conns) == 0 || len(snap.Speeds) != 2 {
		t.Fatalf("item %+v, total %v", it, snap.Speeds)
	}
	for _, c := range it.Conns {
		if c < 0 || c > 1 {
			t.Fatalf("connection progress %v", it.Conns)
		}
	}

	mgr.Pause(d.ID)
	for {
		if got, _ := mgr.Get(d.ID); got.Status == engine.StatusPaused {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the download did not pause")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if it := svc.Snapshot().Items[0]; it.Speed != 0 || it.Conns != nil || it.Speeds != nil || !it.ActiveSince.IsZero() {
		t.Fatalf("a paused download shows %+v", it)
	}
}

// slowWriter writes 16 KB at a time with a pause between.
type slowWriter struct{ http.ResponseWriter }

func (w *slowWriter) Write(p []byte) (int, error) {
	n := 0
	for len(p) > 0 {
		k := min(len(p), 16<<10)
		m, err := w.ResponseWriter.Write(p[:k])
		n += m
		if err != nil {
			return n, err
		}
		p = p[k:]
		time.Sleep(20 * time.Millisecond)
	}
	return n, nil
}
