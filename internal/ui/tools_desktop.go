//go:build fyne

package ui

import (
	"context"
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"odm/internal/engine"
	"odm/internal/tools"
)

// hookTools lets the tool keeper ask the person and tell them, through
// this window.
func (a *application) hookTools(k *tools.Keeper) {
	if k == nil {
		return
	}
	a.keeper = k
	k.Ask = func() { fyne.Do(a.askTools) }
	k.OnInstalled = func(t tools.Tool, version string, first bool) {
		if first && a.notify != nil {
			a.notify(fyne.NewNotification("ODM", Tf("%s %s is ready", t.Name, version)))
		}
	}
}

// setToolUpdates records whether ODM may download yt-dlp and ffmpeg, and
// starts the downloads when it may.
func (a *application) setToolUpdates(mode string) {
	next := a.mgr.Settings()
	if next.ToolUpdates != mode {
		next.ToolUpdates = mode
		a.mgr.UpdateSettings(next)
	}
	if mode == tools.ModeAuto && a.keeper != nil {
		a.keeper.Kick()
	}
}

// askTools asks, once, whether ODM may download yt-dlp and ffmpeg itself.
// Its own window, above the others, like the New Download window: the
// question comes while ODM may sit in the tray behind the browser.
func (a *application) askTools() {
	if a.mgr.Settings().ToolUpdates != tools.ModeAsk {
		return
	}
	w := a.fyneApp.NewWindow(T("Companion programs"))
	if dw, ok := w.(desktop.Window); ok {
		dw.RequestAlwaysOnTop()
	}
	text := widget.NewLabel(T("YouTube videos need yt-dlp, and joining a video with its audio needs ffmpeg. ODM can download both from their official releases (about 20 MB), check their checksums, and keep them up to date by itself. Download them now?"))
	text.Wrapping = fyne.TextWrapWord
	yes := widget.NewButtonWithIcon(T("Download now"), theme.DownloadIcon(), func() {
		a.setToolUpdates(tools.ModeAuto)
		w.Close()
	})
	yes.Importance = widget.HighImportance
	no := widget.NewButton(T("No"), func() {
		a.setToolUpdates(tools.ModeOff)
		w.Close()
	})
	w.SetContent(container.New(layout.NewCustomPaddedLayout(10, 10, 12, 12), container.NewBorder(nil,
		container.NewHBox(layout.NewSpacer(), no, yes), nil, nil, text)))
	w.Resize(fyne.NewSize(460, 1))
	w.Show()
	w.RequestFocus()
}

// toolsOptions is the Options part for the downloaded programs: whether
// ODM keeps them itself, and a button to update them now. refresh redraws
// the status lines of yt-dlp and ffmpeg after an update.
func (a *application) toolsOptions(settings engine.Settings, refresh func()) (*widget.Check, fyne.CanvasObject) {
	auto := widget.NewCheck(T("Download and update yt-dlp and ffmpeg automatically"), nil)
	auto.SetChecked(settings.ToolUpdates == tools.ModeAuto)
	status := widget.NewLabel("")
	status.Wrapping = fyne.TextWrapWord
	status.Importance = widget.LowImportance
	var update *widget.Button
	update = widget.NewButtonWithIcon(T("Update now"), theme.ViewRefreshIcon(), func() {
		if a.keeper == nil {
			return
		}
		update.Disable()
		status.SetText(T("Downloading…"))
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()
			err := a.keeper.UpdateAll(ctx)
			fyne.Do(func() {
				update.Enable()
				if err != nil {
					status.SetText(fmt.Sprint(err))
				} else {
					status.SetText(T("Up to date."))
				}
				refresh()
			})
		}()
	})
	if a.keeper == nil {
		update.Disable()
	}
	box := container.NewVBox(auto,
		container.NewBorder(nil, nil, nil, update, status),
	)
	return auto, box
}

// toolMode is the setting the Options checkbox stands for: a person who
// never answered and leaves it unticked is still asked when it matters.
func toolMode(was string, checked bool) string {
	switch {
	case checked:
		return tools.ModeAuto
	case was == tools.ModeAsk:
		return tools.ModeAsk
	}
	return tools.ModeOff
}
