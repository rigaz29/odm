package engine

import (
	"encoding/json"
	"testing"
)

// Queue names compare without regard to case. A download carries its
// queue's listed spelling, so the queue's schedule, running state and
// positions are found whichever spelling named it.
func TestQueueNamesIgnoreCase(t *testing.T) {
	m := NewManager(Settings{
		Dir:       t.TempDir(),
		Queues:    []string{"Main", "Night"},
		Schedules: map[string]QueueSchedule{"Night": {Retries: 7, RetriesEnabled: true}},
	}, "")

	d := m.Create(AddOptions{URL: "http://127.0.0.1:1/a.bin", Queue: "night"})
	if d.Queue != "Night" {
		t.Fatalf("created in %q, want the listed spelling Night", d.Queue)
	}
	other := m.Create(AddOptions{URL: "http://127.0.0.1:1/b.bin", Queue: "MAIN"})
	if other.Queue != "Main" {
		t.Fatalf("created in %q, want Main", other.Queue)
	}
	if !m.MoveToQueue(other.ID, "NIGHT") {
		t.Fatal("move refused")
	}
	if got, _ := m.Get(other.ID); got.Queue != "Night" {
		t.Fatalf("moved to %q, want Night", got.Queue)
	}
	// Positions are counted per queue: the moved download keeps its own,
	// and a third one in "night" comes after the first.
	third := m.Create(AddOptions{URL: "http://127.0.0.1:1/c.bin", Queue: "nIgHt"})
	if third.Order <= d.Order {
		t.Fatalf("order %d after %d: the spellings were counted as two queues", third.Order, d.Order)
	}

	m.StartQueue("night")
	if !m.QueueRunning("Night") || !m.QueueRunning("NIGHT") {
		t.Fatal("the queue started as night is not running as Night")
	}
	m.StopQueue("NiGhT")
	if m.QueueRunning("Night") {
		t.Fatal("stopping NiGhT left Night running")
	}

	// A list exported elsewhere with another spelling joins the listed queue.
	data, _ := json.Marshal([]State{{ID: "imp1", URL: "http://h/d.bin", Queue: "night"}})
	if n, err := m.Import(data); err != nil || n != 1 {
		t.Fatalf("import: %d, %v", n, err)
	}
	if got, _ := m.Get("imp1"); got.Queue != "Night" {
		t.Fatalf("imported into %q, want Night", got.Queue)
	}

	// Removing "night" removes Night and sends its downloads to Main.
	if !m.RemoveQueue("night") {
		t.Fatal("RemoveQueue(night) did not find Night")
	}
	for _, q := range m.Queues() {
		if q == "Night" {
			t.Fatal("Night is still listed")
		}
	}
	for _, id := range []string{d.ID, other.ID, third.ID, "imp1"} {
		if got, _ := m.Get(id); got.Queue != DefaultQueue {
			t.Fatalf("%s left in %q", id, got.Queue)
		}
	}
	if m.RemoveQueue("MAIN") {
		t.Fatal("the main queue was removed under another spelling")
	}
}

func TestRestoredQueueSpelling(t *testing.T) {
	m := NewManager(Settings{Dir: t.TempDir(), Queues: []string{"Main", "Night"}}, "")
	m.Restore([]State{{ID: "r1", URL: "http://h/a.bin", Queue: "NIGHT", Status: StatusPaused}})
	if got, _ := m.Get("r1"); got.Queue != "Night" {
		t.Fatalf("restored into %q, want Night", got.Queue)
	}
}

func TestSettingsQueueSpellings(t *testing.T) {
	s := Settings{
		Queues: []string{"main", "Night", "night", " NIGHT "},
		Schedules: map[string]QueueSchedule{
			"night": {Retries: 7},
			"MAIN":  {Parallel: 2},
		},
	}
	s.normalize("")
	if len(s.Queues) != 2 || s.Queues[0] != DefaultQueue || s.Queues[1] != "Night" {
		t.Fatalf("queues %q: want one per name, the first spelling kept and Main as Main", s.Queues)
	}
	if s.Schedules["Night"].Retries != 7 || s.Schedules[DefaultQueue].Parallel != 2 {
		t.Fatalf("schedules %+v: want them under the listed spellings", s.Schedules)
	}
	if _, ok := s.Schedules["night"]; ok {
		t.Fatal("the other spelling's schedule was kept as well")
	}
}
