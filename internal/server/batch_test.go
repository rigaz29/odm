package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"odm/internal/engine"
)

func TestStopAllActionStopsQueues(t *testing.T) {
	mgr := engine.NewManager(engine.Settings{Dir: t.TempDir()}, "")
	mgr.Create(engine.AddOptions{URL: "http://h/a.zip"})
	mgr.StartQueue(engine.DefaultQueue)
	if !mgr.QueueRunning(engine.DefaultQueue) {
		t.Fatal("queue not started")
	}
	w := httptest.NewRecorder()
	New(mgr).Handler().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/actions/stop-all", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("stop-all: %d %s", w.Code, w.Body)
	}
	if mgr.QueueRunning(engine.DefaultQueue) {
		t.Fatal("stop-all left the queue started")
	}
}

func TestQueueStartStop(t *testing.T) {
	mgr := engine.NewManager(engine.Settings{Dir: t.TempDir()}, "")
	mgr.Create(engine.AddOptions{URL: "http://127.0.0.1:1/a.zip"})
	h := New(mgr).Handler()
	post := func(path string) int {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, path, nil))
		return w.Code
	}
	if c := post("/api/queues/Main/start"); c != http.StatusOK || !mgr.QueueRunning(engine.DefaultQueue) {
		t.Fatalf("start: %d, running %v", c, mgr.QueueRunning(engine.DefaultQueue))
	}
	if c := post("/api/queues/*/stop"); c != http.StatusOK || mgr.QueueRunning(engine.DefaultQueue) {
		t.Fatalf("stop all: %d, running %v", c, mgr.QueueRunning(engine.DefaultQueue))
	}
	if c := post("/api/queues/Main/jump"); c != http.StatusBadRequest {
		t.Fatalf("unknown action: %d", c)
	}
}
