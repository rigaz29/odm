//go:build fyne

package ui

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"odm/internal/engine"
)

func TestSchedulerFinishAndRepeatRoundTrip(t *testing.T) {
	a := newTestApp(t, engine.Settings{})
	a.openScheduler()
	v := a.scheduler

	v.finish.SetSelected("Hibernate")
	if !v.apply() {
		t.Fatal("apply refused a valid schedule")
	}
	if got := a.mgr.Settings().Schedules[engine.DefaultQueue].FinishAction(); got != engine.ActionHibernate {
		t.Fatalf("finish action saved as %q", got)
	}

	// Repeating: the finish choices no longer apply and are disabled.
	v.repeatOn.SetChecked(true)
	v.repeatMins.SetText("15")
	if !v.finish.Disabled() || !v.openOn.Disabled() {
		t.Fatal("finish choices stay enabled for a repeating queue")
	}
	if !v.apply() {
		t.Fatal("apply refused 15 minutes")
	}
	if got := a.mgr.Settings().Schedules[engine.DefaultQueue].RepeatMinutes; got != 15 {
		t.Fatalf("repeat saved as %d minutes", got)
	}

	// Reopening the queue shows what was saved.
	v.select_(engine.DefaultQueue)
	if !v.repeatOn.Checked || v.repeatMins.Text != "15" || v.finish.Selected != "Hibernate" {
		t.Fatalf("reloaded: repeat %v %q, finish %q", v.repeatOn.Checked, v.repeatMins.Text, v.finish.Selected)
	}

	// A synchronization queue is saved as one.
	v.syncOn.SetChecked(true)
	if !v.apply() || !a.mgr.Settings().Schedules[engine.DefaultQueue].Sync {
		t.Fatal("the synchronization option was not saved")
	}

	// Zero minutes is refused rather than silently turning repeating off.
	v.repeatMins.SetText("0")
	if v.apply() {
		t.Fatal("apply accepted 0 minutes")
	}
}

// Start now starts the queue on screen. An edit that Apply refuses, in that
// queue or another, starts nothing: the window shows that queue instead. It
// used to start the queue Apply switched to.
func TestStartNowStartsTheQueueOnScreen(t *testing.T) {
	a := newTestApp(t, engine.Settings{})
	a.mgr.AddQueue("Night")
	mainDl := a.mgr.Create(engine.AddOptions{URL: "http://127.0.0.1:1/main.bin", FileName: "main.bin"})
	nightDl := a.mgr.Create(engine.AddOptions{URL: "http://127.0.0.1:1/night.bin", FileName: "night.bin", Queue: "Night"})
	a.refresh()
	a.openScheduler()
	v := a.scheduler
	status := func(id string) engine.Status {
		d, _ := a.mgr.Get(id)
		return d.Status
	}

	// Night has an edit not applied yet, with a start time that is not one;
	// Start now is pressed on Main.
	v.select_("Night")
	v.startOn.SetChecked(true)
	v.startAt.SetText("25:99")
	v.select_(engine.DefaultQueue)
	test.Tap(v.startBtn)
	if m, n := status(mainDl.ID), status(nightDl.ID); m != engine.StatusPaused || n != engine.StatusPaused {
		t.Fatalf("Start now with a refused edit started something: main.bin %s, night.bin %s", m, n)
	}
	if v.current != "Night" {
		t.Fatalf("the Scheduler shows %q, not the queue with the bad time", v.current)
	}

	// With the time put right, Start now on Main starts Main alone.
	v.startAt.SetText("23:00")
	v.select_(engine.DefaultQueue)
	test.Tap(v.startBtn)
	if m := status(mainDl.ID); m == engine.StatusPaused {
		t.Fatal("Start now on Main left main.bin paused")
	}
	if n := status(nightDl.ID); n != engine.StatusPaused {
		t.Fatalf("Start now on Main started night.bin (%s)", n)
	}
}

// tapDialogButton taps the button labelled text in the dialog on top of the
// main window.
func tapDialogButton(t *testing.T, a *application, text string) {
	t.Helper()
	if top := a.win.Canvas().Overlays().Top(); top != nil {
		for _, o := range test.LaidOutObjects(top) {
			if b, ok := o.(*widget.Button); ok && b.Text == text {
				test.Tap(b)
				return
			}
		}
	}
	t.Fatalf("no %q button in a dialog", text)
}

// The countdown before a finish action is on the main window, which is
// hidden while ODM sits in the tray: it shows the window and sends a
// notification, so the action can still be cancelled. (ActionExit only:
// the countdown runs the action when it is not cancelled.)
func TestFinishCountdownWhileInTheTray(t *testing.T) {
	a := newTestApp(t, engine.Settings{})
	var notes []*fyne.Notification
	a.notify = func(n *fyne.Notification) { notes = append(notes, n) }
	a.win.Hide() // closed to the tray, as setupTray's close intercept does
	a.foreground.Store(false)

	a.handleAction(engine.Action{Kind: engine.ActionExit})

	if !a.foreground.Load() {
		t.Error("the countdown did not show the main window")
	}
	if len(notes) != 1 || notes[0].Title != "Queues finished" || notes[0].Content != "All queues have finished. ODM will exit in 30 s." {
		t.Errorf("notifications: %+v", notes)
	}
	if txt := overlayText(a); !strings.Contains(txt, "ODM will exit in 30 s.") {
		t.Fatalf("countdown dialog: %q", txt)
	}
	tapDialogButton(t, a, "Cancel")
	if a.win.Canvas().Overlays().Top() != nil {
		t.Fatal("the countdown stayed after Cancel")
	}
}

// The notice that downloads stopped at the download limit shows the main
// window too.
func TestLimitReachedWhileInTheTray(t *testing.T) {
	a := newTestApp(t, engine.Settings{Limits: engine.DownloadLimits{Enabled: true, MB: 100, Hours: 2}})
	a.win.Hide()
	a.foreground.Store(false)

	a.handleAction(engine.Action{Kind: engine.ActionLimit})

	if !a.foreground.Load() {
		t.Error("the notice did not show the main window")
	}
	if txt := overlayText(a); !strings.Contains(txt, "More than 100 MB were downloaded in the last 2 hours") {
		t.Fatalf("notice: %q", txt)
	}
}
