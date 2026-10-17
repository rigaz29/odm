package gui

import "testing"

func TestUsableVersion(t *testing.T) {
	for pv, want := range map[string]bool{"155.0.4283.45": true, "0.0.0.0": false, "": false} {
		if got := usableVersion(pv); got != want {
			t.Errorf("usableVersion(%q) = %v", pv, got)
		}
	}
	if !webView2Installed() {
		t.Log("the WebView2 Runtime is not installed here")
	}
}
