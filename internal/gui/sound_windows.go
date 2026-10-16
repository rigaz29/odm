package gui

import "golang.org/x/sys/windows"

var procMessageBeep = windows.NewLazySystemDLL("user32.dll").NewProc("MessageBeep")

const (
	mbIconAsterisk = 0x40 // the "Asterisk" system sound
	mbIconHand     = 0x10 // the "Critical Stop" system sound
)

// playSound plays the system sound for a finished download, or for a
// failed one, as chosen in the Windows sound settings (as the Fyne UI's,
// internal/ui/sound_windows.go).
func playSound(failed bool) {
	kind := uintptr(mbIconAsterisk)
	if failed {
		kind = mbIconHand
	}
	_, _, _ = procMessageBeep.Call(kind)
}
