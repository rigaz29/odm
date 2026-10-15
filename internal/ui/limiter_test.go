//go:build fyne

package ui

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"

	"odm/internal/engine"
)

func newTestApp(t *testing.T, st engine.Settings) *application {
	t.Helper()
	if st.Dir == "" {
		st.Dir = t.TempDir()
	}
	mgr := engine.NewManager(st, "")
	a := &application{mgr: mgr, progressWindows: map[string]*progressView{}, selectedRow: -1, sortCol: -1}
	a.fyneApp = test.NewTempApp(t)
	a.fyneApp.Settings().SetTheme(newTheme())
	a.win = a.fyneApp.NewWindow("ODM")
	a.build()
	a.refresh()
	return a
}

func limiterItemsChecked(a *application) (n int, label string) {
	for _, it := range a.limiterItems {
		if it.Checked {
			n++
		}
		label = it.Label
	}
	return n, label
}

func TestLimiterMenuToggle(t *testing.T) {
	a := newTestApp(t, engine.Settings{SpeedLimit: 500 << 10, LimiterOff: true})
	if n, _ := limiterItemsChecked(a); n != 0 {
		t.Fatalf("%d limiter items checked while off", n)
	}
	if strings.Contains(a.activity.Text, "speed limit") {
		t.Fatalf("status names a limit while off: %q", a.activity.Text)
	}

	a.toggleLimiter() // on again with the saved 500 KB/s
	if n, label := limiterItemsChecked(a); n != len(a.limiterItems) || !strings.Contains(label, "500") {
		t.Fatalf("after turning on: %d checked, label %q", n, label)
	}
	if !strings.Contains(a.activity.Text, "speed limit 500") {
		t.Fatalf("status %q", a.activity.Text)
	}

	a.toggleLimiter()
	if n, _ := limiterItemsChecked(a); n != 0 {
		t.Fatal("still checked after turning off")
	}
	if st := a.mgr.Settings(); st.SpeedLimit != 500<<10 || !st.LimiterOff {
		t.Fatalf("turning off lost the value: %+v", st.SpeedLimit)
	}
}

func TestLimiterAsksForAValueWhenNoneIsSaved(t *testing.T) {
	a := newTestApp(t, engine.Settings{})
	a.toggleLimiter()
	if a.mgr.Settings().EffectiveSpeedLimit() != 0 {
		t.Fatal("turned on without a value")
	}
	if a.win.Canvas().Overlays().Top() == nil {
		t.Fatal("no dialog asked for a speed")
	}
}
