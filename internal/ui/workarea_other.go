//go:build fyne && !windows

package ui

// workArea is not known here; windows keep the size asked for, and pages
// that do not fit scroll.
func workArea() (width, height int, ok bool) { return 0, 0, false }
