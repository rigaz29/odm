//go:build fyne

package ui

import (
	"testing"

	"fyne.io/fyne/v2"

	"odm/internal/engine"
)

type announced struct {
	notes []*fyne.Notification
	beeps []bool
}

func captureAnnouncements(a *application) *announced {
	got := &announced{}
	a.notify = func(n *fyne.Notification) { got.notes = append(got.notes, n) }
	a.beep = func(failed bool) { got.beeps = append(got.beeps, failed) }
	return got
}

func TestAnnounceFinishedAndFailed(t *testing.T) {
	a := newTestApp(t, engine.Settings{Sounds: true})
	got := captureAnnouncements(a)
	run := engine.Info{ID: "a", FileName: "movie.mkv", Status: engine.StatusDownloading}
	bad := engine.Info{ID: "b", FileName: "setup.exe", Status: engine.StatusDownloading}
	a.lastStatus = nil
	a.noticeCompletions([]engine.Info{run, bad}) // first pass only records

	// In front with the complete dialog on: the dialog says it, so only
	// the sound plays.
	a.foreground.Store(true)
	run.Status = engine.StatusComplete
	a.noticeCompletions([]engine.Info{run, bad})
	if len(got.notes) != 0 || len(got.beeps) != 1 || got.beeps[0] {
		t.Fatalf("in front: %d notes, beeps %v", len(got.notes), got.beeps)
	}

	// A failure is always notified.
	bad.Status, bad.Error = engine.StatusError, "404 Not Found"
	a.noticeCompletions([]engine.Info{run, bad})
	if len(got.notes) != 1 || got.notes[0].Title != "Download failed" || got.notes[0].Content != "setup.exe: 404 Not Found" {
		t.Fatalf("failure: %+v", got.notes)
	}
	if len(got.beeps) != 2 || !got.beeps[1] {
		t.Fatalf("failure sound: %v", got.beeps)
	}

	// Hidden in the tray: a finished download is notified too.
	a.foreground.Store(false)
	again := engine.Info{ID: "c", FileName: "song.mp3", Status: engine.StatusDownloading}
	a.noticeCompletions([]engine.Info{run, bad, again})
	again.Status = engine.StatusComplete
	a.noticeCompletions([]engine.Info{run, bad, again})
	if len(got.notes) != 2 || got.notes[1].Title != "Download complete" || got.notes[1].Content != "song.mp3" {
		t.Fatalf("hidden: %+v", got.notes)
	}
}

func TestAnnounceCanBeTurnedOff(t *testing.T) {
	a := newTestApp(t, engine.Settings{NoNotifications: true})
	got := captureAnnouncements(a)
	a.foreground.Store(false)
	d := engine.Info{ID: "a", FileName: "x.zip", Status: engine.StatusDownloading}
	a.lastStatus = nil
	a.noticeCompletions([]engine.Info{d})
	d.Status = engine.StatusError
	a.noticeCompletions([]engine.Info{d})
	if len(got.notes) != 0 || len(got.beeps) != 0 {
		t.Fatalf("turned off, still announced: %d notes, %d beeps", len(got.notes), len(got.beeps))
	}
}
