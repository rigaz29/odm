//go:build fyne

package ui

import (
	"testing"

	"odm/internal/engine"
)

func TestStatusTexts(t *testing.T) {
	list := []struct {
		total, shown int
		searching    bool
		selected     int
		want         string
	}{
		{0, 0, false, 0, "0 items"},
		{1, 1, false, 1, "1 item"},
		{5, 5, false, 3, "5 items · 3 selected"},
		{5, 2, true, 0, "2 of 5 items match the search"},
	}
	for _, c := range list {
		if got := listStatus(c.total, c.shown, c.searching, c.selected); got != c.want {
			t.Errorf("listStatus(%d, %d, %v, %d) = %q, want %q", c.total, c.shown, c.searching, c.selected, got, c.want)
		}
	}
	activity := []struct {
		active, queued int
		speed, limit   int64
		want           string
	}{
		{0, 0, 0, 0, ""},
		{2, 1, 3 << 20, 0, "2 active at 3.00 MB/s · 1 queued"},
		{1, 0, 0, 0, "1 active"}, // connecting: no speed yet
		{0, 0, 0, 500 << 10, "speed limit 500.0 KB/s"},
	}
	for _, c := range activity {
		if got := activityStatus(c.active, c.queued, c.speed, c.limit); got != c.want {
			t.Errorf("activityStatus(%d, %d, %d, %d) = %q, want %q", c.active, c.queued, c.speed, c.limit, got, c.want)
		}
	}
}

func TestStatusCountsTheSelection(t *testing.T) {
	a := newTestApp(t, engine.Settings{})
	for _, n := range []string{"a.bin", "b.bin", "c.bin"} {
		a.mgr.Create(engine.AddOptions{URL: "http://h/" + n, FileName: n})
	}
	a.refresh()
	a.selectAll()
	if got := a.status.Text; got != "3 items · 3 selected" {
		t.Fatalf("after Ctrl+A: %q", got)
	}
	a.clickRow(0, 0)
	if got := a.status.Text; got != "3 items" {
		t.Fatalf("after a click: %q", got)
	}
}
