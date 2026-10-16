package engine

import (
	"testing"
	"time"
)

// After a restart the list keeps the order downloads were added in, each
// keeps its last-attempt time, and a new download joins the end of its
// queue instead of sharing a position with a restored one.
func TestRestoreKeepsOrderTimesAndQueuePositions(t *testing.T) {
	base := time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)
	states := []State{ // as the store lists them: by random ID
		{ID: "f3", URL: "http://h/c", Status: StatusPaused, Order: 3, CreatedAt: base.Add(3 * time.Minute), UpdatedAt: base.Add(30 * time.Minute)},
		{ID: "a1", URL: "http://h/a", Status: StatusPaused, Order: 1, CreatedAt: base.Add(1 * time.Minute), UpdatedAt: base.Add(10 * time.Minute)},
		{ID: "c2", URL: "http://h/b", Status: StatusPaused, Order: 2, CreatedAt: base.Add(2 * time.Minute), UpdatedAt: base.Add(20 * time.Minute)},
	}
	m := NewManager(Settings{Dir: t.TempDir()}, "")
	m.Restore(states)

	list := m.List()
	if len(list) != 3 || list[0].ID != "a1" || list[1].ID != "c2" || list[2].ID != "f3" {
		t.Fatalf("restored order: %v %v %v", list[0].ID, list[1].ID, list[2].ID)
	}
	if !list[0].UpdatedAt.Equal(base.Add(10 * time.Minute)) {
		t.Fatalf("last attempt became %v", list[0].UpdatedAt)
	}

	added := m.Create(AddOptions{URL: "http://h/new"})
	if added.Order <= 3 {
		t.Fatalf("new download got queue position %d, already used by a restored one", added.Order)
	}
}

func TestImportJoinsTheEndOfTheQueue(t *testing.T) {
	m := NewManager(Settings{Dir: t.TempDir()}, "")
	first := m.Create(AddOptions{URL: "http://h/here"})
	n, err := m.Import([]byte(`[{"id":"x","url":"http://h/imported","order":1,"queue":"Main"}]`))
	if err != nil || n != 1 {
		t.Fatalf("import: %d, %v", n, err)
	}
	imported, _ := m.Get("x")
	if imported.Order <= first.Order {
		t.Fatalf("imported position %d does not follow %d", imported.Order, first.Order)
	}
}

// A download added without starting it ("Download Later", or start:false
// from the API or the browser) is saved at once: it used to be saved only
// when its state changed, so it was gone after ODM closed.
func TestCreateSavesAStoppedDownload(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	m := NewManager(Settings{Dir: t.TempDir()}, "")
	m.SetStore(store)
	d := m.Create(AddOptions{URL: "http://h/later.zip", FileName: "later.zip"})

	states, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(states) != 1 || states[0].ID != d.ID || states[0].Status != StatusPaused || states[0].FileName != "later.zip" {
		t.Fatalf("saved %+v", states)
	}
	again := NewManager(Settings{Dir: t.TempDir()}, "")
	again.Restore(states)
	if got, ok := again.Get(d.ID); !ok || got.FileName != "later.zip" {
		t.Fatalf("after a restart: %+v %v", got, ok)
	}
}
