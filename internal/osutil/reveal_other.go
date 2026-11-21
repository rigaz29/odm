//go:build !windows

package osutil

import (
	"os/exec"
	"path/filepath"
	"runtime"
)

// revealCommand shows the file in Finder, selected, or opens its folder in
// the file manager elsewhere.
func revealCommand(path string) *exec.Cmd {
	if runtime.GOOS == "darwin" {
		return exec.Command("open", "-R", path)
	}
	return exec.Command("xdg-open", filepath.Dir(path))
}
