package gui

import "testing"

func TestFitSize(t *testing.T) {
	for _, c := range []struct {
		w, h, areaW, areaH, wantW, wantH int
	}{
		{1280, 780, 1920, 1032, 1280, 780}, // 100%: as asked
		{1280, 780, 1536, 825, 1280, 780},  // 125%
		{1280, 780, 1280, 688, 1248, 656},  // 150%: inside the work area
		{1280, 780, 960, 516, 928, 484},    // 200%
		{640, 600, 0, 0, 640, 600},         // the area unknown
		{640, 600, 40, 30, 20, 15},         // a tiny area: at least half of it
	} {
		w, h := fitSize(c.w, c.h, c.areaW, c.areaH, 32)
		if w != c.wantW || h != c.wantH {
			t.Errorf("fitSize(%d×%d in %d×%d) = %d×%d, want %d×%d", c.w, c.h, c.areaW, c.areaH, w, h, c.wantW, c.wantH)
		}
	}
}
