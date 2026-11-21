package ipc

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

// lockFile locks f's first byte for f's handle alone, without waiting.
// Another handle on the file, in this process or another, cannot lock it
// until this one is unlocked or closed.
func lockFile(f *os.File) error {
	err := windows.LockFileEx(windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, new(windows.Overlapped))
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		return ErrRunning
	}
	return err
}

// unlockFile unlocks what lockFile locked. Closing the handle does it too,
// but Windows may take a moment then.
func unlockFile(f *os.File) error {
	return windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, new(windows.Overlapped))
}
