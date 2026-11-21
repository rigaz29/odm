//go:build windows

package osutil

import (
	"path/filepath"
	"strings"
	"testing"
)

// Explorer gets /select, and the path in quotes as one piece, also for a
// path with a space or a comma, which Go's own quoting broke.
func TestRevealCommandQuotesThePath(t *testing.T) {
	for _, p := range []string{
		`C:\Users\a b\Downloads\my file.txt`,
		`C:\Downloads\part1,part2.txt`,
		`C:\Downloads\plain.txt`,
	} {
		cmd := revealCommand(p)
		if !strings.EqualFold(filepath.Base(cmd.Path), "explorer.exe") {
			t.Fatalf("%s: runs %s", p, cmd.Path)
		}
		want := `explorer /select,"` + p + `"`
		if cmd.SysProcAttr == nil || cmd.SysProcAttr.CmdLine != want {
			t.Errorf("%s: command line %+v, want %s", p, cmd.SysProcAttr, want)
		}
	}
}
