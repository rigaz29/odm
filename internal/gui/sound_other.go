//go:build !windows

package gui

// playSound makes no sound outside Windows; the Options leave the choice
// out there.
func playSound(bool) {}
