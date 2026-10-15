package engine

import (
	"sync"
	"testing"
)

// The run goroutine reports a download once more after Start returns,
// which can come after RemoveWithFile. That late update must not write the
// state back, or the removed download returns at the next start.
func TestRemovedDownloadIsNotSavedAgain(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m := NewManager(Settings{Dir: t.TempDir()}, "")
	m.SetStore(store)
	info := m.Create(AddOptions{URL: "http://h/file.zip"})
	d := m.get(info.ID)
	m.changed(d)
	if states, _ := store.Load(); len(states) != 1 {
		t.Fatalf("%d states saved, want 1", len(states))
	}

	if !m.Remove(info.ID) {
		t.Fatal("remove refused")
	}
	m.changed(d) // the late update
	if states, _ := store.Load(); len(states) != 0 {
		t.Fatalf("removed download saved again: %d states", len(states))
	}
}

func TestStoreConcurrentSavesStayReadable(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	chunks := NewPlan(64<<20, 32)
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 25; i++ {
				_ = store.Save(State{ID: "same", URL: "http://h/f", Order: g*100 + i, Chunks: chunks})
			}
		}(g)
	}
	wg.Wait()
	states, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(states) != 1 || states[0].ID != "same" || len(states[0].Chunks) != len(chunks) {
		t.Fatalf("state after concurrent saves: %d states", len(states))
	}
}

func TestResumeAllSkipsRemovedDownloads(t *testing.T) {
	m := NewManager(Settings{Dir: t.TempDir()}, "")
	info := m.Create(AddOptions{URL: "http://h/a.zip"})
	m.mu.Lock()
	m.order = append(m.order, "gone") // listed, but no longer in the map
	m.mu.Unlock()
	m.ResumeAll() // used to dereference a nil download
	if s := m.get(info.ID).status(); s != StatusQueued {
		t.Fatalf("status %s, want queued", s)
	}
}
