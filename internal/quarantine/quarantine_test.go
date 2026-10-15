package quarantine

import "testing"

func TestSourceURL(t *testing.T) {
	cases := map[string]string{
		"https://user:secret@example.com/a.zip?x=1": "https://example.com/a.zip?x=1",
		"http://example.com/":                       "http://example.com/",
		"":                                          "about:internet",
		"not a url":                                 "about:internet",
	}
	for in, want := range cases {
		if got := sourceURL(in); got != want {
			t.Errorf("sourceURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLocal(t *testing.T) {
	for _, u := range []string{"http://127.0.0.1:8899/a", "http://localhost/a", "http://192.168.1.4/a", "http://[::1]/a", "http://app.localhost/a"} {
		if !local(u) {
			t.Errorf("%s should be local", u)
		}
	}
	for _, u := range []string{"https://example.com/a", "http://8.8.8.8/a", "about:internet"} {
		if local(u) {
			t.Errorf("%s should not be local", u)
		}
	}
}
