//go:build fyne && !windows

package ui

// soundSupported reports whether playSound makes a sound on this system.
// Fyne has no audio API, so only Windows plays one for now.
const soundSupported = false

func playSound(bool) {}
