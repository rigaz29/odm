//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package ipc

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

// lockFile takes flock's exclusive lock on f without waiting. The lock
// belongs to this open file: another open of the file, in this process or
// another, cannot take it until this one is unlocked or closed. Files Go
// opens are not passed to the programs ODM starts (close-on-exec), so
// yt-dlp or odm-libtorrent never keep it after ODM ends.
func lockFile(f *os.File) error {
	err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if errors.Is(err, unix.EWOULDBLOCK) {
		return ErrRunning
	}
	return err
}

// unlockFile drops what lockFile took; closing the file does it too.
func unlockFile(f *os.File) error {
	return unix.Flock(int(f.Fd()), unix.LOCK_UN)
}
