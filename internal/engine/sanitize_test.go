package engine

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSanitizeName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"report.pdf", "report.pdf"},
		{`a/b\c:d*e?f"g<h>i|j.zip`, "a_b_c_d_e_f_g_h_i_j.zip"},
		{"line\nbreak\t.txt", "line_break_.txt"},
		{"  .hidden. ", "hidden"},
		{"..", ""},
		{"   ", ""},
		{"CON", "_CON"},
		{"con.txt", "_con.txt"},
		{"Nul.tar.gz", "_Nul.tar.gz"},
		{"COM1.mp4", "_COM1.mp4"},
		{"lpt9", "_lpt9"},
		{"COM0.mp4", "_COM0.mp4"},
		{"CONSOLE.txt", "CONSOLE.txt"},
		{"concert.mp3", "concert.mp3"},
	}
	for _, c := range cases {
		if got := SanitizeName(c.in); got != c.want {
			t.Errorf("SanitizeName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSanitizeNameShortensOnACharacterBoundary(t *testing.T) {
	long := strings.Repeat("é", 150) + ".mp4" // 304 bytes
	got := SanitizeName(long)
	if len(got) > maxNameBytes {
		t.Fatalf("%d bytes, want at most %d", len(got), maxNameBytes)
	}
	if !utf8.ValidString(got) {
		t.Fatalf("cut inside a character: %q", got)
	}
	if !strings.HasSuffix(got, ".mp4") {
		t.Fatalf("extension lost: %q", got)
	}

	dots := strings.Repeat("a", 190) + ". . . . . . . .bin"
	if got := SanitizeName(dots); strings.HasSuffix(strings.TrimSuffix(got, ".bin"), ".") ||
		strings.HasSuffix(strings.TrimSuffix(got, ".bin"), " ") {
		t.Fatalf("trailing dot or space before the extension: %q", got)
	}

	noExt := strings.Repeat("x", 300)
	if got := SanitizeName(noExt); len(got) != maxNameBytes {
		t.Fatalf("%d bytes, want %d", len(got), maxNameBytes)
	}
}
