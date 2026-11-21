package ipc

import (
	"os"
	"path/filepath"
)

// LockName is the file in the config directory whose lock marks the ODM
// that runs.
const LockName = "odm.lock"

// Lock is the single-instance lock: an exclusive lock the system keeps on
// LockName (LockFileEx on Windows, flock elsewhere) while the file stays
// open. The system drops it when the process ends, a crash included, so it
// never outlives the ODM that took it, and a second ODM cannot take it
// while the first runs, however close together they start.
type Lock struct {
	f *os.File
}

// TakeLock takes the single-instance lock in configDir, creating the
// folder (for the user only) when it is missing. ErrRunning means another
// ODM holds it; any other error means the lock cannot be used there.
func TakeLock(configDir string) (*Lock, error) {
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(configDir, LockName), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	if err := lockFile(f); err != nil {
		f.Close()
		return nil, err
	}
	return &Lock{f: f}, nil
}

// Release drops the lock. It may be called more than once, and on a nil
// Lock (none was taken).
func (l *Lock) Release() error {
	if l == nil || l.f == nil {
		return nil
	}
	_ = unlockFile(l.f)
	err := l.f.Close()
	l.f = nil
	return err
}
