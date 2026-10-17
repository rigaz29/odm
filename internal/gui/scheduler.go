package gui

import (
	"strings"
	"time"

	"odm/internal/engine"
)

// The Scheduler window, as the Fyne UI's (internal/ui/scheduler_desktop.go):
// each queue's schedule and files, and the download limits. Apply saves
// every queue's schedule at once, after checking the times.

// Schedule is one queue's schedule as the window edits it.
type Schedule struct {
	// Sync checks finished files and downloads them again when they
	// changed on the server.
	Sync      bool `json:"sync"`
	OnStartup bool `json:"onStartup"`
	// Start at StartAt ("HH:MM"): once on OnceDate ("YYYY-MM-DD") when
	// Once, else on the Days ticked, Sunday first.
	StartEnabled bool   `json:"startEnabled"`
	StartAt      string `json:"startAt"`
	Once         bool   `json:"once"`
	OnceDate     string `json:"onceDate"`
	Days         []bool `json:"days"`
	// RepeatMinutes starts the queue again that long after it last
	// started, until it is stopped.
	RepeatEnabled  bool   `json:"repeatEnabled"`
	RepeatMinutes  int    `json:"repeatMinutes"`
	StopEnabled    bool   `json:"stopEnabled"`
	StopAt         string `json:"stopAt"`
	RetriesEnabled bool   `json:"retriesEnabled"`
	Retries        int    `json:"retries"`
	// When the queue has finished: open OpenPath, and Finish ("", "exit",
	// "sleep", "hibernate" or "shutdown").
	OpenEnabled bool   `json:"openEnabled"`
	OpenPath    string `json:"openPath"`
	Finish      string `json:"finish"`
	// Parallel is how many of its files download at once.
	Parallel int `json:"parallel"`
}

// QueueState is a queue as the Scheduler lists it.
type QueueState struct {
	Name      string `json:"name"`
	Running   bool   `json:"running"`
	Repeating bool   `json:"repeating"` // waiting to start again: Stop ends it
}

// Limits caps how much is downloaded every so many hours.
type Limits struct {
	On      bool `json:"on"`
	MB      int  `json:"mb"`
	Hours   int  `json:"hours"`
	Warning bool `json:"warning"`
}

// SchedulerData is everything the Scheduler window edits.
type SchedulerData struct {
	Queues    []QueueState        `json:"queues"`
	Schedules map[string]Schedule `json:"schedules"`
	Limits    Limits              `json:"limits"`
}

// SchedulerRefusal names the queue and the value Apply could not take:
// Field is "start" or "stop" (Value is not a time) or "repeat" (no
// minutes).
type SchedulerRefusal struct {
	Queue string `json:"queue"`
	Field string `json:"field"`
	Value string `json:"value"`
}

func scheduleOf(sc engine.QueueSchedule, today time.Time) Schedule {
	out := Schedule{
		Sync: sc.Sync, OnStartup: sc.OnStartup,
		StartEnabled: sc.StartEnabled, StartAt: orDefault(sc.StartAt, "23:00"),
		Once: sc.OnceDate != "", OnceDate: orDefault(sc.OnceDate, today.Format("2006-01-02")),
		Days:          append([]bool(nil), sc.Days...),
		RepeatEnabled: sc.RepeatMinutes > 0, RepeatMinutes: 60,
		StopEnabled: sc.StopEnabled, StopAt: orDefault(sc.StopAt, "07:30"),
		RetriesEnabled: sc.RetriesEnabled, Retries: sc.Retries,
		OpenEnabled: sc.OpenWhenDone != "", OpenPath: sc.OpenWhenDone,
		Finish: sc.FinishAction(), Parallel: sc.Parallel,
	}
	if sc.RepeatMinutes > 0 {
		out.RepeatMinutes = sc.RepeatMinutes
	}
	if len(out.Days) != 7 {
		out.Days = []bool{true, true, true, true, true, true, true}
	}
	return out
}

// into is the engine's schedule for s, from the one saved (was).
func (s Schedule) into(was engine.QueueSchedule) engine.QueueSchedule {
	sc := was
	sc.Sync, sc.OnStartup = s.Sync, s.OnStartup
	sc.StartEnabled, sc.StartAt = s.StartEnabled, strings.TrimSpace(s.StartAt)
	sc.OnceDate = ""
	if s.Once {
		sc.OnceDate = strings.TrimSpace(s.OnceDate)
	}
	sc.Days = make([]bool, 7)
	copy(sc.Days, s.Days)
	sc.StopEnabled, sc.StopAt = s.StopEnabled, strings.TrimSpace(s.StopAt)
	sc.RetriesEnabled, sc.Retries = s.RetriesEnabled, s.Retries
	sc.OpenWhenDone = ""
	if s.OpenEnabled {
		sc.OpenWhenDone = strings.TrimSpace(s.OpenPath)
	}
	sc.SetFinishAction(s.Finish)
	sc.RepeatMinutes = 0
	if s.RepeatEnabled {
		sc.RepeatMinutes = s.RepeatMinutes
	}
	sc.Parallel = s.Parallel
	return sc
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// Scheduler returns the queues, their schedules and the download limits.
func (s *Service) Scheduler() SchedulerData {
	st := s.mgr.Settings()
	out := SchedulerData{Schedules: map[string]Schedule{}, Limits: Limits{
		On: st.Limits.Enabled, MB: st.Limits.MB, Hours: st.Limits.Hours, Warning: !st.Limits.NoWarning,
	}}
	today := s.now()
	for _, q := range s.mgr.Queues() {
		out.Queues = append(out.Queues, QueueState{Name: q, Running: s.mgr.QueueRunning(q), Repeating: s.mgr.QueueRepeating(q)})
		sc, ok := st.Schedules[q]
		if !ok {
			sc = engine.NewQueueSchedule()
		}
		out.Schedules[q] = scheduleOf(sc, today)
	}
	return out
}

// SaveScheduler saves every queue's schedule and the download limits. A
// start or stop time that is not HH:MM, or a repeat without minutes, is
// refused with the queue it is in, and nothing is saved.
func (s *Service) SaveScheduler(schedules map[string]Schedule, limits Limits) *SchedulerRefusal {
	for q, sc := range schedules {
		for _, t := range []struct {
			on    bool
			value string
			field string
		}{{sc.StartEnabled, sc.StartAt, "start"}, {sc.StopEnabled, sc.StopAt, "stop"}} {
			if _, err := time.Parse("15:04", strings.TrimSpace(t.value)); t.on && err != nil {
				return &SchedulerRefusal{Queue: q, Field: t.field, Value: t.value}
			}
		}
		if sc.RepeatEnabled && sc.RepeatMinutes < 1 {
			return &SchedulerRefusal{Queue: q, Field: "repeat"}
		}
	}
	next := s.mgr.Settings()
	was := next.Schedules
	next.Schedules = map[string]engine.QueueSchedule{}
	for q, sc := range was {
		next.Schedules[q] = sc
	}
	for q, sc := range schedules {
		base, ok := was[q]
		if !ok {
			base = engine.NewQueueSchedule()
		}
		next.Schedules[q] = sc.into(base)
	}
	next.Limits = engine.DownloadLimits{Enabled: limits.On, MB: limits.MB, Hours: limits.Hours, NoWarning: !limits.Warning}
	s.mgr.UpdateSettings(next)
	s.poke()
	return nil
}

// AddQueue makes a new queue; RemoveQueue deletes one, its downloads
// moving to the main queue, which cannot be deleted.
func (s *Service) AddQueue(name string) error {
	name = strings.TrimSpace(name)
	switch {
	case name == "":
		return errNoName
	case !s.mgr.AddQueue(name):
		return errQueueExists
	}
	s.poke()
	return nil
}

func (s *Service) RemoveQueue(name string) error {
	if strings.EqualFold(name, engine.DefaultQueue) || !s.mgr.RemoveQueue(name) {
		return errNoQueue
	}
	s.poke()
	return nil
}

// OpenScheduler opens the Scheduler window; with newQueue it asks for a
// new queue's name at once.
func (s *Service) OpenScheduler(newQueue bool) {
	if w := s.opener(); w != nil {
		w.openScheduler(newQueue)
	}
}

const (
	errNoName      serviceError = "enter a name"
	errQueueExists serviceError = "that queue already exists"
	errNoQueue     serviceError = "no such queue"
)
