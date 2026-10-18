//go:build !windows

package browsers

// Listed reports whether Installed can find the browsers here.
const Listed = false

// Installed is empty here for now: the Setup window points to ODM's
// releases page for the extension instead.
func Installed() []Browser { return nil }
