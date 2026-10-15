//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

// startDetached starts exe in a session of its own, so it outlives the
// browser's native host that started it.
func startDetached(exe string, args ...string) error {
	cmd := exec.Command(exe, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
