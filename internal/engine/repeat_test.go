package engine

import (
	"testing"
	"time"
)

func TestFinishActionKeepsTheStrongest(t *testing.T) {
	sc := QueueSchedule{ExitWhenDone: true, SleepWhenDone: true, HibernateWhenDone: true}
	sc.normalize()
	if a := sc.FinishAction(); a != ActionHibernate || sc.ExitWhenDone || sc.SleepWhenDone {
		t.Fatalf("normalised to %q (exit %v, sleep %v)", a, sc.ExitWhenDone, sc.SleepWhenDone)
	}
	sc.SetFinishAction(ActionSleep)
	if sc.FinishAction() != ActionSleep || sc.HibernateWhenDone {
		t.Fatalf("SetFinishAction(sleep) left %+v", sc)
	}
	sc.SetFinishAction("")
	if sc.FinishAction() != "" {
		t.Fatalf("none left %q", sc.FinishAction())
	}
}

// When two queues finish together, the strongest action is carried out.
func TestWhenDonePicksTheStrongestAction(t *testing.T) {
	m := NewManager(Settings{Dir: t.TempDir(), Queues: []string{DefaultQueue, "Night"}}, "")
	m.settings.Schedules[DefaultQueue] = QueueSchedule{SleepWhenDone: true, Days: allDays()}
	m.settings.Schedules["Night"] = QueueSchedule{ExitWhenDone: true, Days: allDays()}
	var got []string
	m.SetActionHandler(func(a Action) { got = append(got, a.Kind) })
	m.mu.Lock()
	m.drained = []string{"Night", DefaultQueue}
	m.mu.Unlock()
	m.whenDone()
	if len(got) != 1 || got[0] != ActionSleep {
		t.Fatalf("actions %v, want only sleep", got)
	}
}

// A repeating queue starts again N minutes after it was last started,
// whenever it is not running, until it is stopped; it never runs its
// finish action.
func TestRepeatingQueue(t *testing.T) {
	m := NewManager(Settings{Dir: t.TempDir()}, "")
	m.settings.Schedules[DefaultQueue] = QueueSchedule{RepeatMinutes: 30, ShutdownWhenDone: true, Days: allDays()}
	var actions []string
	m.SetActionHandler(func(a Action) { actions = append(actions, a.Kind) })
	t0 := time.Date(2026, 10, 15, 8, 0, 0, 0, time.Local)

	m.startQueueAt(DefaultQueue, t0) // nothing to do yet: the cycle still begins
	if !m.QueueRepeating(DefaultQueue) {
		t.Fatal("not repeating after start")
	}
	info := m.Create(AddOptions{URL: "http://h/new.zip"}) // added later, paused
	m.checkSchedule(t0.Add(29 * time.Minute))
	if s := m.get(info.ID).status(); s != StatusPaused {
		t.Fatalf("started early: %s", s)
	}
	m.checkSchedule(t0.Add(30 * time.Minute))
	if s := m.get(info.ID).status(); s != StatusQueued {
		t.Fatalf("not started again after 30 minutes: %s", s)
	}

	m.mu.Lock()
	m.drained = []string{DefaultQueue}
	m.mu.Unlock()
	m.whenDone()
	if len(actions) != 0 {
		t.Fatalf("a repeating queue ran %v", actions)
	}

	m.StopQueue(DefaultQueue)
	if m.QueueRepeating(DefaultQueue) {
		t.Fatal("still repeating after Stop")
	}
	m.Pause(info.ID)
	m.checkSchedule(t0.Add(2 * time.Hour))
	if s := m.get(info.ID).status(); s != StatusPaused {
		t.Fatalf("a stopped queue started again: %s", s)
	}
}
