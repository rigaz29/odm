//go:build fyne

package ui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// optionsTabConnection is the Options tab with the global speed limit.
const optionsTabConnection = 4

// newLimiterItem returns a "Speed limiter" menu item that is kept checked
// while the global limit applies; the main menu and the tray each get one.
func (a *application) newLimiterItem() *fyne.MenuItem {
	it := fyne.NewMenuItem("Speed limiter", a.toggleLimiter)
	a.limiterItems = append(a.limiterItems, it)
	return it
}

// toggleLimiter switches the global speed limit off, or on again with the
// value saved before. With no value saved yet it asks for one.
func (a *application) toggleLimiter() {
	st := a.mgr.Settings()
	switch {
	case st.EffectiveSpeedLimit() > 0:
		a.mgr.SetSpeedLimiter(false, 0)
	case st.SpeedLimit > 0:
		a.mgr.SetSpeedLimiter(true, 0)
	default:
		a.showWindow() // also reached from the tray, with the window hidden
		a.askSpeedLimit()
		return
	}
	a.refresh()
}

func (a *application) askSpeedLimit() {
	entry := widget.NewEntry()
	entry.SetText("500")
	dialog.NewForm("Speed limiter", "Turn on", "Cancel",
		[]*widget.FormItem{widget.NewFormItem("Maximum speed (KB/s)", entry)},
		func(ok bool) {
			if !ok {
				return
			}
			kb := 0
			fmt.Sscanf(strings.TrimSpace(entry.Text), "%d", &kb)
			if kb <= 0 {
				dialog.ShowInformation("Speed limiter", "Enter a speed above 0 KB/s.", a.win)
				return
			}
			a.mgr.SetSpeedLimiter(true, int64(kb)*1024)
			a.refresh()
		}, a.win).Show()
}

// syncLimiter checks the limiter menu items and names the limit in them.
// Menus are refreshed only when that changes, as refresh runs every second.
func (a *application) syncLimiter(rate int64) {
	state := fmt.Sprint(rate)
	if state == a.limiterShown {
		return
	}
	a.limiterShown = state
	label := "Speed limiter"
	if rate > 0 {
		label = fmt.Sprintf("Speed limiter (%s/s)", humanBytes(rate))
	}
	for _, it := range a.limiterItems {
		it.Checked = rate > 0
		it.Label = label
	}
	if a.mainMenu != nil {
		a.mainMenu.Refresh()
	}
	if a.trayMenu != nil {
		a.trayMenu.Refresh()
	}
}
