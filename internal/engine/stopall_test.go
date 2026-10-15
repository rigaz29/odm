package engine

import "testing"

// Pause all leaves a started queue started, so resuming one of its files
// carries on with the queue. Stop all also stops the queue, and drops an
// exit or shut-down that was waiting for it.
func TestStopAllStopsStartedQueues(t *testing.T) {
	m, url := heldManager(t, 3)
	var ids []string
	for i := 0; i < 3; i++ {
		ids = append(ids, addHeld(m, url, false).ID)
	}
	m.StartQueue(DefaultQueue)
	if !m.QueueRunning(DefaultQueue) {
		t.Fatal("queue not started")
	}

	m.PauseAll()
	for _, id := range ids {
		waitStatus(t, m.get(id), StatusPaused)
	}
	if !m.QueueRunning(DefaultQueue) {
		t.Fatal("Pause all stopped the queue; it should stay started")
	}

	m.mu.Lock()
	m.pendingEnd = ActionShutdown // another queue finished and asked for it
	m.mu.Unlock()
	m.StopAll()
	if m.QueueRunning(DefaultQueue) {
		t.Fatal("Stop all left the queue started")
	}
	m.mu.Lock()
	pending := m.pendingEnd
	m.mu.Unlock()
	if pending != "" {
		t.Fatalf("Stop all kept the pending %q", pending)
	}
	for _, id := range ids {
		if s := m.get(id).status(); s != StatusPaused {
			t.Fatalf("download %s is %s after Stop all", id, s)
		}
	}
}

func TestStopAllPausesRunningDownloads(t *testing.T) {
	m, url := heldManager(t, 2)
	a := addHeld(m, url, true)
	b := addHeld(m, url, true)
	waitStatus(t, m.get(a.ID), StatusDownloading)
	m.StopAll()
	waitStatus(t, m.get(a.ID), StatusPaused)
	waitStatus(t, m.get(b.ID), StatusPaused)
}
