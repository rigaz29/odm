//go:build windows

package main

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// startDetached starts exe on its own. A browser may run its native host in
// a job object that ends every process in it when the host exits (Firefox
// does), so the program breaks away from the job when the job allows it.
func startDetached(exe string, args ...string) error {
	const flags = windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS
	cmd := exec.Command(exe, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: flags | windows.CREATE_BREAKAWAY_FROM_JOB}
	if err := cmd.Start(); err != nil {
		// The job does not allow it: start inside it instead.
		cmd = exec.Command(exe, args...)
		cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: flags}
		if err := cmd.Start(); err != nil {
			return err
		}
	}
	return cmd.Process.Release()
}
