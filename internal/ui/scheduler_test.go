//go:build fyne

package ui

import (
	"testing"

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
