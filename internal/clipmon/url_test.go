package clipmon

import "testing"

func TestClipboardURL(t *testing.T) {
	cases := []struct {
		text string
		want string
		ok   bool
	}{
		{"https://example.com/movie.mp4", "https://example.com/movie.mp4", true},
		{"get it at https://cdn.host/path/archive.tar.gz now", "https://cdn.host/path/archive.tar.gz", true},
		{"see https://example.com/page", "", false},
		{"no links here", "", false},
		{"https://host/file.zip.", "https://host/file.zip", true},
		{"ftp://host/file.zip", "", false},
		{"grab magnet:?xt=urn:btih:ab12&dn=Ubuntu+24.04 please", "magnet:?xt=urn:btih:ab12&dn=Ubuntu+24.04", true},
		{"magnet:?dn=no-hash", "", false},
	}
	for _, c := range cases {
		got, ok := ClipboardURL(c.text)
		if ok != c.ok || got != c.want {
			t.Errorf("ClipboardURL(%q) = (%q,%v), want (%q,%v)", c.text, got, ok, c.want, c.ok)
		}
	}
}

func TestClipboardURLExtraTypes(t *testing.T) {
	const text = "magnet-ish https://host/linux.torrent"
	if _, ok := ClipboardURL(text); ok {
		t.Fatal(".torrent matched without being configured")
	}
	for _, extra := range []string{"torrent", ".TORRENT", " torrent "} {
		if got, ok := ClipboardURL(text, extra); !ok || got != "https://host/linux.torrent" {
			t.Errorf("extra %q: got (%q,%v)", extra, got, ok)
		}
	}
	// Extras must not leak into later calls (the table is shared state).
	if _, ok := ClipboardURL(text); ok {
		t.Fatal("extra type persisted after the call")
	}
}
