//go:build !windows

package gui

// workArea is not known here: windows keep the size they ask for.
func workArea() (width, height int, ok bool) { return 0, 0, false }
