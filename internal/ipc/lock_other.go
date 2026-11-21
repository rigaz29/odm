//go:build !windows && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris

package ipc

import (
	"errors"
	"os"
)

// lockFile has no file lock to take on this system: TakeLock fails, and
// ODM tells a running copy apart by its socket alone (Listen).
func lockFile(*os.File) error { return errors.ErrUnsupported }

func unlockFile(*os.File) error { return nil }
