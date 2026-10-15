package engine

import (
	"path/filepath"
	"testing"
)

func limiterRate(m *Manager) float64 {
	m.limiter.mu.Lock()
	defer m.limiter.mu.Unlock()
	return m.limiter.rate
}

// Switching the limiter off keeps the limit, so switching it on again
// brings back the same value without retyping it.
func TestSpeedLimiterToggleKeepsTheLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	m := NewManager(Settings{Dir: t.TempDir(), SpeedLimit: 300 << 10, LimiterOff: true}, path)
	if r := limiterRate(m); r != 0 {
		t.Fatalf("limiter off at start, rate %v", r)
	}

	st := m.SetSpeedLimiter(true, 0)
	if r := limiterRate(m); r != 300<<10 || st.EffectiveSpeedLimit() != 300<<10 {
		t.Fatalf("on: rate %v, effective %d", r, st.EffectiveSpeedLimit())
	}

	st = m.SetSpeedLimiter(false, 0)
	if r := limiterRate(m); r != 0 || st.SpeedLimit != 300<<10 {
		t.Fatalf("off: rate %v, saved limit %d", r, st.SpeedLimit)
	}
	if saved := LoadSettings(path, t.TempDir()); !saved.LimiterOff || saved.SpeedLimit != 300<<10 {
		t.Fatalf("settings.json: off=%v limit=%d", saved.LimiterOff, saved.SpeedLimit)
	}

	st = m.SetSpeedLimiter(true, 50<<10) // on with a new value
	if r := limiterRate(m); r != 50<<10 || st.SpeedLimit != 50<<10 {
		t.Fatalf("new value: rate %v, limit %d", r, st.SpeedLimit)
	}
}

func TestEffectiveSpeedLimit(t *testing.T) {
	cases := []struct {
		s    Settings
		want int64
	}{
		{Settings{SpeedLimit: 0}, 0},
		{Settings{SpeedLimit: 1000}, 1000},
		{Settings{SpeedLimit: 1000, LimiterOff: true}, 0},
		{Settings{SpeedLimit: -5}, 0},
	}
	for _, c := range cases {
		if got := c.s.EffectiveSpeedLimit(); got != c.want {
			t.Errorf("%+v: %d, want %d", c.s, got, c.want)
		}
	}
}
