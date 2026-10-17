package native

import "testing"

func TestLaunchedByBrowser(t *testing.T) {
	cases := []struct {
		args []string
		want bool
	}{
		{[]string{"chrome-extension://fnondfibgeiffjplpokfgiacjelmbcbk/", "--parent-window=0"}, true},
		{[]string{"chrome-extension://abc/"}, true},
		{[]string{`C:\Users\u\AppData\Roaming\odm\native\com.ochi.odm.firefox.json`, "odm@rigaz29.github.io"}, true},
		{nil, false},
		{[]string{"-no-ui"}, false},
		{[]string{"-add", "https://host/file.zip"}, false},
		{[]string{"settings.json"}, false}, // a lone .json is not Firefox's pair
	}
	for _, c := range cases {
		if got := LaunchedByBrowser(c.args); got != c.want {
			t.Errorf("LaunchedByBrowser(%q) = %v, want %v", c.args, got, c.want)
		}
	}
}
