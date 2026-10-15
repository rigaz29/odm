package engine

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
)

func TestSettingsCopySharesNothing(t *testing.T) {
	m := NewManager(Settings{
		Dir:             t.TempDir(),
		HostConnections: map[string]int{"a.example": 2},
		CategoryDirs:    map[string]string{"Video": "v"},
		Logins:          []Login{{Host: "a.example", User: "u", Pass: "p"}},
	}, "")
	got := m.Settings()
	got.HostConnections["b.example"] = 3
	got.CategoryDirs["Audio"] = "a"
	got.Logins[0].User = "changed"
	got.Queues[0] = "Renamed"
	sc := got.Schedules[DefaultQueue]
	sc.Days[0] = !sc.Days[0]

	again := m.Settings()
	if len(again.HostConnections) != 1 || len(again.CategoryDirs) != 1 {
		t.Fatalf("maps changed through a copy: %v %v", again.HostConnections, again.CategoryDirs)
	}
	if again.Logins[0].User != "u" || again.Queues[0] != DefaultQueue {
		t.Fatalf("slices changed through a copy: %v %v", again.Logins, again.Queues)
	}
	if again.Schedules[DefaultQueue].Days[0] == sc.Days[0] {
		t.Fatal("schedule days changed through a copy")
	}
}

func TestUpdateSettingsLeavesCallerMapsAlone(t *testing.T) {
	m := NewManager(Settings{Dir: t.TempDir()}, "")
	next := m.Settings()
	next.HostConnections = map[string]int{" Mixed.Example ": 4}
	applied := m.UpdateSettings(next)
	if _, ok := next.HostConnections[" Mixed.Example "]; !ok || len(next.HostConnections) != 1 {
		t.Fatalf("UpdateSettings rewrote the caller's map: %v", next.HostConnections)
	}
	if applied.HostConnections["mixed.example"] != 4 {
		t.Fatalf("normalised map: %v", applied.HostConnections)
	}
}

// Reading a Settings copy while settings are updated used to touch the
// same maps from two goroutines, which the runtime can abort on.
func TestSettingsReadWhileUpdating(t *testing.T) {
	m := NewManager(Settings{Dir: t.TempDir()}, "")
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			_, _ = json.Marshal(m.Settings())
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			next := m.Settings()
			next.Connections = 1 + i%8
			m.UpdateSettings(next)
		}
	}()
	wg.Wait()
}

func TestConcurrentQueueAddsAreAllSaved(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	m := NewManager(Settings{Dir: t.TempDir()}, path)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			m.AddQueue(fmt.Sprintf("q%d", i))
		}(i)
	}
	wg.Wait()
	saved := LoadSettings(path, t.TempDir())
	if len(saved.Queues) != 11 {
		t.Fatalf("settings.json has %d queues, want 11: %v", len(saved.Queues), saved.Queues)
	}
}
