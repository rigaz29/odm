package gui

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"odm/internal/engine"
)

// stubTeller records the notifications and sounds asked for.
type stubTeller struct {
	mu  sync.Mutex
	log []string
}

func (s *stubTeller) notify(title, body string) {
	s.mu.Lock()
	s.log = append(s.log, title+": "+body)
	s.mu.Unlock()
}

func (s *stubTeller) beep(failed bool) {
	s.mu.Lock()
	s.log = append(s.log, fmt.Sprintf("beep %v", failed))
	s.mu.Unlock()
}

func (s *stubTeller) got() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := strings.Join(s.log, " | ")
	s.log = nil
	return out
}

func TestFinishedAndFailedAreAnnounced(t *testing.T) {
	svc, mgr := newTestService(t)
	tell := &stubTeller{}
	svc.announce = tell
	at := func(a, b engine.Status) []engine.Info {
		return []engine.Info{
			{ID: "a", FileName: "a.iso", Status: a},
			{ID: "b", URL: "http://h/b", Status: b, Error: "404 Not Found"},
		}
	}
	svc.noticeFinished(at(engine.StatusDownloading, engine.StatusDownloading))
	svc.noticeFinished(at(engine.StatusComplete, engine.StatusError))
	if got := tell.got(); got != "Download complete: a.iso | Download failed: http://h/b: 404 Not Found" {
		t.Fatalf("announced %q", got)
	}

	// With sounds on, and the complete window shown: a sound, no notification.
	svc.windows = &stubWindows{}
	st := mgr.Settings()
	st.Sounds = true
	mgr.UpdateSettings(st)
	svc.noticeFinished(at(engine.StatusDownloading, engine.StatusDownloading))
	svc.noticeFinished(at(engine.StatusComplete, engine.StatusDownloading))
	if got := tell.got(); got != "beep false" {
		t.Fatalf("with the complete window: %q", got)
	}

	// A download that failed before it was first seen is announced when
	// it was just added, not when it came from an imported list.
	st.Sounds = false
	mgr.UpdateSettings(st)
	now := time.Now()
	svc.now = func() time.Time { return now }
	svc.noticeFinished(append(at(engine.StatusComplete, engine.StatusDownloading),
		engine.Info{ID: "c", FileName: "c.zip", Status: engine.StatusError, Error: "refused", CreatedAt: now.Add(-2 * time.Second)},
		engine.Info{ID: "d", FileName: "d.zip", Status: engine.StatusComplete, CreatedAt: now.Add(-48 * time.Hour)}))
	if got := tell.got(); got != "Download failed: c.zip: refused" {
		t.Fatalf("new downloads: %q", got)
	}
	st.Sounds = true
	mgr.UpdateSettings(st)
	svc.noticeFinished(at(engine.StatusDownloading, engine.StatusDownloading))
	tell.got()

	// Notifications off: only the sound.
	st.NoNotifications = true
	mgr.UpdateSettings(st)
	svc.noticeFinished(at(engine.StatusDownloading, engine.StatusError))
	if got := tell.got(); got != "beep true" {
		t.Fatalf("with notifications off: %q", got)
	}
}
