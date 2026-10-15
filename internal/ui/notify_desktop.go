//go:build fyne

package ui

import (
	"fyne.io/fyne/v2"

	"odm/internal/engine"
)

// announce tells the person that a download finished or failed: the system
// sound when Sounds is on, and a system notification unless turned off. A
// finished download needs no notification when the complete dialog was just
// shown in a window that is in front.
func (a *application) announce(d engine.Info, failed, dialogShown bool) {
	st := a.mgr.Settings()
	if st.Sounds && a.beep != nil {
		a.beep(failed)
	}
	if st.NoNotifications || a.notify == nil {
		return
	}
	if !failed && dialogShown && a.foreground.Load() {
		return
	}
	name := d.FileName
	if name == "" {
		name = d.URL
	}
	title, body := "Download complete", name
	if failed {
		title = "Download failed"
		if d.Error != "" {
			body = name + ": " + d.Error
		}
	}
	a.notify(fyne.NewNotification(title, body))
}
