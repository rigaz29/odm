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
