//go:build !windows

package osutil

import (
	"os/exec"
	"runtime"
)

// Suspend puts the computer to sleep, or into hibernation when hibernate is
// set. macOS has no separate hibernate command; it sleeps.
func Suspend(hibernate bool) error {
	if runtime.GOOS == "darwin" {
		return exec.Command("pmset", "sleepnow").Run()
	}
	mode := "suspend"
	if hibernate {
		mode = "hibernate"
	}
	return exec.Command("systemctl", mode).Run()
}
