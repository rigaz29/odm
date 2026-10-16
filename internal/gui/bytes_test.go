package gui

import "testing"

// The same steps as the page's humanBytes (frontend/src/format.test.ts).
func TestFormatBytes(t *testing.T) {
	for _, c := range []struct {
		n    int64
		lang string
		want string
	}{
		{0, "en", ""},
		{999, "en", "999 B"},
		{1000, "en", "0.98 KB"},
		{6_300_000, "en", "6.01 MB"},
		{1000 * 1024 * 1024, "en", "0.98 GB"},
		{58 * 1024 * 1024 * 1024 / 10, "id", "5,80 GB"},
		{412 * 1024 * 1024, "id", "412,0 MB"},
	} {
		if got := formatBytes(c.n, c.lang); got != c.want {
			t.Errorf("formatBytes(%d, %s) = %q, want %q", c.n, c.lang, got, c.want)
		}
	}
}
