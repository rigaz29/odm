package ytdlp

import (
	"testing"
	"unicode/utf8"
)

// yt-dlp.exe's error output comes in the ANSI code page; it reads as text.
func TestLocalText(t *testing.T) {
	if got := localText([]byte("you’re")); got != "you’re" {
		t.Errorf("UTF-8 changed: %q", got)
	}
	got := localText([]byte("Sign in to confirm you\x92re not a bot"))
	if !utf8.ValidString(got) || got == "" {
		t.Errorf("ANSI text: %q", got)
	}
}
