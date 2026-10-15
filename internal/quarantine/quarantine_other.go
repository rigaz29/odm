//go:build !windows

package quarantine

// Mark does nothing on this system.
func Mark(path, source, referrer string) error { return nil }
