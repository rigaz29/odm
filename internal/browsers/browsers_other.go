//go:build !windows

package browsers

// Installed is empty here for now: the Setup window's extension step is
// Windows-only until ODM's new UI runs on Linux and macOS.
func Installed() []Browser { return nil }
