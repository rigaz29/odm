package fluent

import (
	"context"
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

// The page hears of a change once: watch sends a snapshot when it differs
// from the last one, and at once when an action pokes it.
func TestWatchSendsChanges(t *testing.T) {
	svc, mgr := newTestService(t)
	var mu sync.Mutex
	var sent []Snapshot
	svc.emit = func(name string, data any) {
		if name != SnapshotEvent {
			t.Errorf("event %q", name)
		}
		mu.Lock()
		sent = append(sent, data.(Snapshot))
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
	svc.Remove([]string{d.ID}) // pokes; the list is back as it was
	svc.poke()
	time.Sleep(50 * time.Millisecond)
	mgr.Create(engine.AddOptions{URL: "http://127.0.0.1:9/c.zip", FileName: "c.zip"})
	svc.poke()
	waitFor(2)
	mu.Lock()
	last := sent[len(sent)-1]
	mu.Unlock()
	if len(last.Items) != 1 || last.Items[0].FileName != "c.zip" {
		t.Fatalf("last snapshot %+v", last.Items)
	}
}

func TestAddURL(t *testing.T) {
	svc, mgr := newTestService(t)
	if _, err := svc.AddURL("ftp://x/y"); err == nil {
		t.Fatal("a non-http address was added")
	}
	it, err := svc.AddURL("  http://127.0.0.1:9/z.iso ")
	if err != nil || it.ID == "" {
		t.Fatalf("%+v %v", it, err)
	}
	if d, ok := mgr.Get(it.ID); !ok || d.URL != "http://127.0.0.1:9/z.iso" {
		t.Fatalf("download %+v %v", d, ok)
	}
	if err := svc.OpenFile("nope"); err == nil {
		t.Fatal("an unknown download was opened")
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
