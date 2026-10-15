//go:build fyne

package ui

import (
	"context"
	"errors"
	"fmt"
	"image/color"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"odm/internal/autostart"
	"odm/internal/browserreg"
	"odm/internal/engine"
	"odm/internal/ffmpeg"
	"odm/internal/media"
	"odm/internal/osutil"
	"odm/internal/update"
	"odm/internal/version"
)

func openPath(p string)   { _ = osutil.Open(p) }
func revealPath(p string) { _ = osutil.Reveal(p) }

func dialogConfirm(parent fyne.Window, title, msg string, fn func(bool)) {
	dialog.NewConfirm(title, msg, fn, parent).Show()
}

// checksumText says how a finished download's file was checked.
func checksumText(sum string) string {
	if sum == "" {
		return T("Not checked")
	}
	return Tf("%s matches", engine.ChecksumName(sum))
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

/* ---------- Add: "New Download" ---------- */

func (a *application) openAddDialog() {
	var dlg *dialog.CustomDialog
	content, buttons := a.newAddForm(a.win, engine.AddOptions{}, func() { dlg.Hide() },
		func() { dlg.Resize(fyne.NewSize(620, 1)) })
	dlg = dialog.NewCustomWithoutButtons(T("New Download"), content, a.win)
	dlg.SetButtons(buttons)
	dlg.Resize(fyne.NewSize(620, 1)) // height clamps to content: no empty band above the buttons
	dlg.Show()
}

// openAddWindow shows the New Download form in its own window, prefilled,
// for a download handed over by the browser. A separate window (rather than
// a dialog on the main window) works while ODM sits hidden in the tray.
//
// It stays above other windows: the browser keeps the focus, as Windows
// does not let a background process take it, and the window would open
// behind the browser.
func (a *application) openAddWindow(pre engine.AddOptions) {
	w := a.fyneApp.NewWindow(T("New Download"))
	if dw, ok := w.(desktop.Window); ok {
		dw.RequestAlwaysOnTop() // before Show
	}
	content, buttons := a.newAddForm(w, pre, w.Close, func() {
		width := w.Canvas().Size().Width // keep a width the person chose
		if width < 640 {
			width = 640
		}
		w.Resize(fyne.NewSize(width, 1))
	})
	w.SetContent(container.New(layout.NewCustomPaddedLayout(8, 10, 10, 10),
		container.NewBorder(nil, container.NewHBox(layout.NewSpacer(), container.NewHBox(buttons...)), nil, nil, content)))
	w.Resize(fyne.NewSize(640, 1))
	w.Show()
	w.RequestFocus()
}

// newAddForm builds the New Download form and its buttons. pre prefills it;
// pre.Headers (cookies, referrer, user agent from the browser) are used for
// the size probe and kept for the download, since many servers refuse a
// request without them. done closes whatever hosts the form.
//
// relayout, when set, resizes the host after rows are added: a dialog does
// not grow with its content on its own.
func (a *application) newAddForm(parent fyne.Window, pre engine.AddOptions, done, relayout func()) (fyne.CanvasObject, []fyne.CanvasObject) {
	settings := a.mgr.Settings()

	urlEntry := widget.NewEntry()
	urlEntry.SetPlaceHolder("https://host/file.zip")
	catSelect := widget.NewSelect(engine.Categories(), nil)
	catSelect.SetSelected("Other")
	folderEntry := widget.NewEntry()
	folderEntry.SetText(settings.SaveFolder()) // the folder used last time, when remembered
	nameEntry := widget.NewEntry()
	nameEntry.SetText(pre.FileName)
	if pre.FileName != "" {
		catSelect.SetSelected(engine.Category(pre.FileName))
	}
	if pre.Category != "" {
		catSelect.SetSelected(pre.Category)
	}
	// The hash the publisher gives for the file; checked once it is down.
	sumEntry := widget.NewEntry()
	sumEntry.SetPlaceHolder(T("SHA-256, SHA-1 or MD5 (optional)"))
	descEntry := widget.NewEntry()
	descEntry.SetText(pre.Description)
	connEntry := widget.NewEntry()
	connEntry.SetText(fmt.Sprint(settings.Connections))
	queueSelect := widget.NewSelect(a.mgr.Queues(), nil)
	queueSelect.SetSelected(engine.DefaultQueue)
	// Preview lists a ZIP archive's contents before it is downloaded; shown
	// once the probe finds a ZIP on a server that sends parts of files.
	var previewURL, previewName string
	previewBtn := widget.NewButton(T("Preview…"), func() {
		a.showZipPreview(parent, previewURL, previewName, pre.Headers)
	})
	previewBtn.Hide()
	// The card under the form says what the check of the address found.
	card := newProbeCard(previewBtn)

	// Quality and Audio rows, added to the form the first time the address
	// turns out to be an HLS/DASH playlist. All of the stream state below
	// belongs to the address streamFor and is reset whenever the address
	// changes, so Start never applies one playlist's choice to another.
	//   - stream: what the playlist offers;
	//   - streamKind: the playlist format, kept when reading it failed for
	//     now (timeout, server error) so Start still queues it and ODM
	//     picks the best quality when the download runs;
	//   - streamErr: why it can never be downloaded (live, DRM), so Start
	//     refuses instead of queueing something that will fail.
	var (
		stream     *engine.StreamChoices
		streamKind media.Kind
		streamErr  error
		streamFor  string
		streamRows bool
		streamCat  string // the category before it was set to Video for a stream
	)
	qualitySelect := widget.NewSelect(nil, nil)
	audioSelect := widget.NewSelect(nil, nil)
	fillAudio := func(q int) {
		audioSelect.Options = nil
		if stream != nil && q >= 0 && q < len(stream.Qualities) {
			for _, au := range stream.Qualities[q].Audios {
				audioSelect.Options = append(audioSelect.Options, au.Label)
			}
		}
		if len(audioSelect.Options) == 0 {
			audioSelect.Options = []string{T("In the video")}
		}
		audioSelect.SetSelectedIndex(0)
		if len(audioSelect.Options) > 1 {
			audioSelect.Enable()
		} else {
			audioSelect.Disable()
		}
	}
	qualitySelect.OnChanged = func(string) { fillAudio(qualitySelect.SelectedIndex()) }

	var form *widget.Form
	addStreamRows := func() {
		if streamRows {
			return
		}
		streamRows = true
		form.Append(T("Quality"), qualitySelect)
		form.Append(T("Audio"), audioSelect)
		if relayout != nil {
			relayout()
		}
	}
	// Video replaces only the category ODM guessed from the name, not one
	// the browser request or the person chose.
	setVideoCategory := func() {
		auto := catSelect.Selected == engine.Category(nameEntry.Text) && pre.Category == ""
		if auto && catSelect.Selected != "Video" {
			streamCat = catSelect.Selected
			catSelect.SetSelected("Video")
		}
	}
	showStream := func(addr string, kind media.Kind, c *engine.StreamChoices, err error) {
		addStreamRows()
		streamFor, streamKind, stream, streamErr = addr, kind, nil, nil
		qualitySelect.Options = nil
		qualitySelect.ClearSelected()
		qualitySelect.PlaceHolder = "—"
		setVideoCategory()
		switch {
		case errors.Is(err, engine.ErrStreamRefused):
			streamErr = err
			card.problem(T("This stream cannot be downloaded"), err.Error())
		case err != nil:
			card.warn(T("The playlist could not be read just now"),
				T("Start lets ODM pick the best quality when it runs · ")+err.Error())
		}
		if err != nil {
			qualitySelect.Disable()
			fillAudio(-1)
			return
		}
		stream = c
		for _, q := range c.Qualities {
			label := q.Label
			if q.Estimate > 0 {
				label += " · ~" + humanBytes(q.Estimate)
			}
			qualitySelect.Options = append(qualitySelect.Options, label)
		}
		qualitySelect.Enable()
		sel := 0
		// The quality picked in the browser's menu, for the address it
		// was picked for.
		if pre.StreamQuality != "" && addr == strings.TrimSpace(pre.URL) {
			if q := c.Quality(pre.StreamQuality); q >= 0 {
				sel = q
			}
		}
		qualitySelect.SetSelectedIndex(sel) // OnChanged fills the audio list
		info := Tf("%s stream", strings.ToUpper(string(c.Kind)))
		if c.Note != "" {
			info += " · " + c.Note
		}
		title := nameEntry.Text
		if title == "" {
			title = T("Video stream")
		}
		card.stream(title, info)
	}
	hideStream := func() {
		stream, streamKind, streamErr, streamFor = nil, "", nil, ""
		if streamRows {
			qualitySelect.Options = nil
			qualitySelect.ClearSelected()
			qualitySelect.PlaceHolder = "—"
			qualitySelect.Disable()
			audioSelect.Options = nil
			audioSelect.ClearSelected()
			audioSelect.Disable()
		}
		if streamCat != "" {
			if catSelect.Selected == "Video" { // not changed by hand since
				catSelect.SetSelected(streamCat)
			}
			streamCat = ""
		}
	}

	var probeTimer *time.Timer
	urlEntry.OnChanged = func(s string) {
		if probeTimer != nil {
			probeTimer.Stop()
		}
		hideStream()
		previewBtn.Hide()
		switch {
		case strings.TrimSpace(s) == "":
			card.idle()
			return
		case !strings.HasPrefix(s, "http"):
			card.warn(T("Not checked"), T("Only http and https addresses are checked before they are added."))
			return
		}
		card.check()
		probeTimer = time.AfterFunc(600*time.Millisecond, func() {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			// The browser already knows a playlist when it sends one; a
			// typed address is probed as a file first, and read as a
			// playlist when its type or extension says it is one.
			var kind media.Kind
			if pre.Stream != nil && s == pre.URL {
				kind = pre.Stream.Kind
			}
			var p *engine.Probe
			var err error
			if kind == "" {
				p, err = a.mgr.Probe(ctx, s, pre.Headers)
				if err == nil {
					if k := media.Detect(p.ContentType, s, nil); k == media.KindHLS || k == media.KindDASH {
						kind = k
					}
				}
			}
			var choices *engine.StreamChoices
			if kind != "" {
				choices, err = a.mgr.ProbeStream(ctx, s, pre.Headers)
				if choices != nil {
					kind = choices.Kind // the playlist itself is the better judge
				}
			}
			fyne.Do(func() {
				if urlEntry.Text != s {
					return // the address changed while this probe ran
				}
				fromBrowser := pre.Stream != nil && s == pre.URL
				switch {
				case kind != "" && (fromBrowser || !errors.Is(err, engine.ErrStreamRefused)):
					showStream(strings.TrimSpace(s), kind, choices, err)
					return
				case kind != "":
					// A typed address that only looked like a stream (an
					// .m3u music list, say) is downloaded as the file it is.
					card.warn(T("Not a stream ODM can read"), T("It downloads as a file · ")+err.Error())
					return
				}
				if err != nil {
					card.problem(T("The file could not be checked"), err.Error())
					return
				}
				if nameEntry.Text == "" {
					nameEntry.SetText(p.FileName)
					catSelect.SetSelected(engine.Category(p.FileName))
				}
				card.file(p, nameEntry.Text)
				if p.RangeOK && p.Total > 0 && engine.IsZipName(p.FileName, p.ContentType) {
					previewURL, previewName = strings.TrimSpace(s), p.FileName
					previewBtn.Show()
				}
			})
		})
	}

	browseButton := widget.NewButtonWithIcon(T("Browse…"), theme.FolderOpenIcon(), func() {
		dialog.NewFolderOpen(func(dir fyne.ListableURI, err error) {
			if err != nil || dir == nil {
				return
			}
			folderEntry.SetText(dir.Path())
		}, parent).Show()
	})
	folderRow := container.NewBorder(nil, nil, nil, browseButton, folderEntry)

	form = widget.NewForm(
		widget.NewFormItem(T("URL"), urlEntry),
		widget.NewFormItem(T("Category"), catSelect),
		widget.NewFormItem(T("Save folder"), folderRow),
		widget.NewFormItem(T("File name"), nameEntry),
		widget.NewFormItem(T("Description"), descEntry),
		widget.NewFormItem(T("Connections"), connEntry),
		widget.NewFormItem(T("Queue"), queueSelect),
		widget.NewFormItem(T("Checksum"), sumEntry),
	)
	urlEntry.SetText(pre.URL) // after OnChanged and the form, so a prefilled URL is probed
	if pre.Stream != nil {
		// Known to be a playlist: add the rows before the window is shown,
		// so it is sized for them, and fill them once the playlist is read.
		addStreamRows()
		qualitySelect.PlaceHolder = T("Reading the playlist…")
		qualitySelect.Disable()
		audioSelect.Disable()
		setVideoCategory()
	}

	var create func(start, confirmed bool)
	create = func(start, confirmed bool) {
		conns := settings.Connections
		fmt.Sscanf(connEntry.Text, "%d", &conns)
		url := strings.TrimSpace(urlEntry.Text)
		if url == "" {
			return
		}
		sum, err := engine.ParseChecksum(sumEntry.Text)
		if err != nil {
			dialog.ShowError(err, parent)
			return
		}
		var job *engine.StreamJob
		switch {
		case streamFor == url && streamErr != nil:
			dialog.ShowError(streamErr, parent)
			return
		case streamFor == url && stream != nil:
			j, err := stream.Job(url, qualitySelect.SelectedIndex(), audioSelect.SelectedIndex())
			if err != nil {
				dialog.ShowError(err, parent)
				return
			}
			job = &j
		case streamFor == url && streamKind != "":
			// The playlist could not be read just now: ODM picks the best
			// quality when the download runs.
			job = &engine.StreamJob{Kind: streamKind, Manifest: url}
		case pre.Stream != nil && url == pre.URL:
			// Started before the playlist was read: ODM picks the best.
			j := *pre.Stream
			job = &j
		}
		if existing, dup := a.mgr.FindByURL(url); dup && !confirmed {
			a.askDuplicate(parent, existing, func() { create(start, true) })
			return
		}
		folder := strings.TrimSpace(folderEntry.Text)
		info := a.mgr.Create(engine.AddOptions{
			Stream:      job,
			URL:         url,
			Dir:         folder,
			FileName:    engine.SanitizeName(nameEntry.Text),
			Description: strings.TrimSpace(descEntry.Text),
			Checksum:    sum,
			Connections: conns,
			Category:    catSelect.Selected,
			Queue:       queueSelect.Selected,
			Headers:     pre.Headers,
			Start:       start,
		})
		a.mgr.NoteSaveFolder(folder)
		done()
		a.refresh()
		if start {
			a.autoProgress(info.ID)
		}
	}

	startBtn := widget.NewButtonWithIcon(T("Start Download"), theme.ConfirmIcon(), func() { create(true, false) })
	startBtn.Importance = widget.HighImportance
	laterBtn := widget.NewButtonWithIcon(T("Download Later"), theme.ContentAddIcon(), func() { create(false, false) })
	cancelBtn := widget.NewButton(T("Cancel"), done)
	return container.NewVBox(form, card.root), []fyne.CanvasObject{laterBtn, cancelBtn, startBtn}
}

/* ---------- Options ---------- */

// openOptionsTab shows the Options window (its own window, like a classic
// configuration dialog) with the given tab selected. OK applies and closes;
// Cancel discards.
func (a *application) openOptionsTab(tab int) {
	if a.optionsWin != nil {
		a.optionsWin.Show()
		a.optionsWin.RequestFocus()
		return
	}
	settings := a.mgr.Settings()
	w := a.fyneApp.NewWindow(T("ODM Options"))
	a.optionsWin = w
	browseDir := func(e *widget.Entry) *widget.Button {
		return widget.NewButtonWithIcon(T("Browse"), theme.FolderOpenIcon(), func() {
			dialog.NewFolderOpen(func(u fyne.ListableURI, err error) {
				if err == nil && u != nil {
					e.SetText(u.Path())
				}
			}, w).Show()
		})
	}
	hint := func(s string) *widget.Label {
		l := widget.NewLabel(s)
		l.Wrapping = fyne.TextWrapWord
		l.Importance = widget.LowImportance
		return l
	}
	header := func(s string) fyne.CanvasObject {
		return container.NewVBox(widget.NewLabelWithStyle(s, fyne.TextAlignTrailing, fyne.TextStyle{Bold: true}), widget.NewSeparator())
	}
	numEntry := func(n int) *widget.Entry {
		e := widget.NewEntry()
		e.SetText(fmt.Sprint(n))
		return e
	}
	small := func(o fyne.CanvasObject, wdt float32) fyne.CanvasObject {
		return container.NewGridWrap(fyne.NewSize(wdt, o.MinSize().Height), o)
	}

	// General
	clipCheck := widget.NewCheck(T("Automatically start downloading of URLs placed to the clipboard"), nil)
	clipCheck.SetChecked(settings.Clipboard)
	updCheck := widget.NewCheck(T("Check for updates at start-up"), nil)
	updCheck.SetChecked(!settings.NoUpdateCheck)
	// Browser integration status: an icon and coloured line (all set /
	// partly set / not registered), a hint for the next step, and a brief
	// highlight whenever Register or Unregister changes it, so the click
	// visibly did something. Dialogs appear only for failures, on this
	// Options window.
	var regBtn, unregBtn *widget.Button
	statusIcon := widget.NewIcon(theme.QuestionIcon())
	browserStatus := widget.NewLabel("")
	browserStatus.Wrapping = fyne.TextWrapWord
	browserHint := widget.NewLabel("")
	browserHint.Wrapping = fyne.TextWrapWord
	browserHint.Importance = widget.LowImportance
	statusBg := canvas.NewRectangle(color.Transparent)
	statusBg.CornerRadius = 6
	statusRow := container.NewStack(statusBg, container.NewPadded(
		container.NewBorder(nil, nil, container.NewCenter(statusIcon), nil, browserStatus)))
	flash := func() {
		accent := theme.PrimaryColor()
		r, g, b, _ := accent.RGBA()
		start := color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: 0x70}
		canvas.NewColorRGBAAnimation(start, color.Transparent, 900*time.Millisecond, func(c color.Color) {
			statusBg.FillColor = c
			statusBg.Refresh()
		}).Start()
	}
	refreshBrowser := func(highlight bool) {
		exe, _ := selfExe()
		sts := browserreg.Check()
		here, other, missing := 0, 0, 0
		for _, st := range sts {
			switch {
			case st.Points(exe):
				here++
			case st.Registered:
				other++
			case st.Installed:
				missing++
			}
		}
		browserStatus.SetText(browserreg.SummaryWith(sts, exe, T))
		switch {
		case here > 0 && other == 0 && missing == 0:
			statusIcon.SetResource(theme.NewSuccessThemedResource(theme.ConfirmIcon()))
			browserStatus.Importance = widget.SuccessImportance
			browserHint.SetText(T("Install the ODM extension in each browser (see the README), then restart the browser once."))
			regBtn.SetText(T("Register again"))
		case here == 0 && other == 0:
			statusIcon.SetResource(theme.NewErrorThemedResource(theme.CancelIcon()))
			browserStatus.Importance = widget.DangerImportance
			browserHint.SetText(T("Register so browsers can hand downloads to ODM."))
			regBtn.SetText(T("Register"))
		default:
			statusIcon.SetResource(theme.NewWarningThemedResource(theme.WarningIcon()))
			browserStatus.Importance = widget.WarningImportance
			browserHint.SetText(T("Register to point every browser at this ODM."))
			regBtn.SetText(T("Register"))
		}
		browserStatus.Refresh()
		if here+other > 0 {
			unregBtn.Enable()
		} else {
			unregBtn.Disable()
		}
		if highlight {
			flash()
		}
	}
	regBtn = widget.NewButtonWithIcon(T("Register"), theme.LoginIcon(), func() {
		a.registerBrowser(w)
		refreshBrowser(true)
	})
	regBtn.Importance = widget.HighImportance
	unregBtn = widget.NewButtonWithIcon(T("Unregister"), theme.DeleteIcon(), func() {
		dialog.ShowConfirm(T("Unregister browser integration"),
			T("Browsers will stop handing downloads to ODM until you register again. Continue?"), func(yes bool) {
				if !yes {
					return
				}
				var failed []string
				for _, r := range browserreg.Unregister() {
					if r.Err != nil {
						failed = append(failed, r.String())
					}
				}
				a.setBrowserRegister(false) // or the next start registers again
				refreshBrowser(true)
				if len(failed) > 0 {
					dialog.ShowError(fmt.Errorf(T("could not unregister:\n%s"), strings.Join(failed, "\n")), w)
				}
			}, w)
	})
	refreshBrowser(false)
	startupCheck := widget.NewCheck(T("Launch ODM on startup (in the system tray)"), nil)
	startupWas := autostart.Enabled()
	startupCheck.SetChecked(startupWas)
	trayCheck := widget.NewCheck(T("Keep ODM running in the system tray when the window is closed"), nil)
	trayCheck.SetChecked(!settings.NoCloseToTray)
	prefs := a.fyneApp.Preferences()
	themeWas, accentWas := prefs.StringWithFallback(prefTheme, themeSystem), prefs.String(prefAccent)
	themeSelect := widget.NewSelect(themeChoices(), nil)
	themeSelect.SetSelectedIndex(themeIndex(themeWas))
	langWas := prefs.StringWithFallback(prefLanguage, langSystem)
	langSelect := widget.NewSelect(languageChoices(), nil)
	langSelect.SetSelectedIndex(languageIndex(langWas))
	accentRow, accentPicked := newAccentPicker(accentWas)
	generalTab := container.NewVBox(
		header(T("Browser / system integration")),
		widget.NewLabelWithStyle(T("Browser extension (Chrome, Edge, Chromium, Firefox)"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		statusRow,
		browserHint,
		container.NewHBox(regBtn, unregBtn),
		widget.NewSeparator(),
		startupCheck,
		trayCheck,
		clipCheck,
		updCheck,
		header(T("Appearance")),
		container.New(layout.NewFormLayout(),
			widget.NewLabel(T("Theme")), container.NewHBox(small(themeSelect, 200)),
			widget.NewLabel(T("Accent colour")), accentRow,
			widget.NewLabel(T("Language")), container.NewHBox(small(langSelect, 200))),
		hint(T("A new language shows after ODM is started again.")),
	)

	// File types
	typesEntry := widget.NewMultiLineEntry()
	typesEntry.Wrapping = fyne.TextWrapWord
	typesEntry.SetText(strings.ToUpper(strings.Join(settings.FileTypes, " ")))
	typesEntry.SetMinRowsVisible(6)
	typesDefault := widget.NewButton(T("Default"), func() {
		typesEntry.SetText(strings.ToUpper(strings.Join(engine.DefaultFileTypes(), " ")))
	})
	typesTab := container.NewVBox(
		header(T("Downloaded file types")),
		widget.NewLabel(T("Automatically start downloading the following file types:")),
		typesEntry,
		container.NewHBox(layout.NewSpacer(), typesDefault),
		hint(T("Used by the clipboard monitor and the site grabber. Separate extensions with spaces.")),
	)

	// Save to
	dirEntry := widget.NewEntry()
	dirEntry.SetText(settings.Dir)
	catsCheck := widget.NewCheck(T("Put files in category subfolders"), nil)
	catsCheck.SetChecked(settings.Categories)
	rememberCheck := widget.NewCheck(T("Remember the folder last chosen in the New Download window"), nil)
	rememberCheck.SetChecked(!settings.NoRememberDir)
	catDirs := map[string]string{}
	for k, v := range settings.CategoryDirs {
		catDirs[k] = v
	}
	catFolder := widget.NewEntry()
	// The categories are edited in catDefs and saved with OK: names, order,
	// and the extensions that decide which category (and folder) a file
	// gets. Other is always last and takes everything else.
	cats := newCategoryEditor(w, engine.CategoriesInUse(), catDirs, catFolder, dirEntry)
	saveTab := container.NewVBox(
		header(T("Categories and folders")),
		widget.NewForm(widget.NewFormItem(T("Default download folder"),
			container.NewBorder(nil, nil, nil, browseDir(dirEntry), dirEntry))),
		catsCheck,
		rememberCheck,
		hint(T("Downloads added from the clipboard, the site grabber or the API use the default folder.")),
		widget.NewSeparator(),
		widget.NewForm(widget.NewFormItem(T("Category"), container.NewBorder(nil, nil, nil,
			container.NewHBox(cats.newBtn, cats.delBtn, cats.resetBtn), cats.sel))),
		widget.NewForm(widget.NewFormItem(T("File types"), cats.exts)),
		cats.note,
		widget.NewForm(widget.NewFormItem(T("Folder for this category"),
			container.NewBorder(nil, nil, nil, browseDir(catFolder), catFolder))),
		hint(T("Empty keeps <default folder>/<category>. Category folders apply when subfolders are on.")),
	)

	// Downloads
	startCheck := widget.NewCheck(T("Show the New Download window for downloads from the browser"), nil)
	startCheck.SetChecked(!settings.NoStartDialog)
	excludeEntry := widget.NewMultiLineEntry()
	excludeEntry.SetText(strings.Join(settings.ExcludeSites, " "))
	excludeEntry.SetPlaceHolder("*.update.microsoft.com example.org")
	excludeEntry.SetMinRowsVisible(2)
	progCheck := widget.NewCheck(T("Show the progress window when a download starts"), nil)
	progCheck.SetChecked(!settings.NoProgressDialog)
	doneCheck := widget.NewCheck(T("Show the download complete dialog"), nil)
	doneCheck.SetChecked(!settings.NoCompleteDialog)
	notifyCheck := widget.NewCheck(T("Show a notification when a download finishes or fails"), nil)
	notifyCheck.SetChecked(!settings.NoNotifications)
	soundCheck := widget.NewCheck(T("Play a sound when a download finishes or fails"), nil)
	soundCheck.SetChecked(settings.Sounds)
	announceBox := container.NewVBox(notifyCheck)
	if soundSupported {
		announceBox.Add(soundCheck)
	}
	var (
		dupSkip  = T("Skip it")
		dupAgain = T("Download it again")
	)
	dupSelect := widget.NewSelect([]string{dupSkip, dupAgain}, nil)
	if settings.DuplicateAuto == engine.DuplicateAgain {
		dupSelect.SetSelected(dupAgain)
	} else {
		dupSelect.SetSelected(dupSkip)
	}
	uaEntry := widget.NewEntry()
	uaEntry.SetText(settings.UserAgent)
	uaEntry.SetPlaceHolder(engine.DefaultUserAgent)
	// ffmpeg joins the separate video and audio of some streams. The
	// status line says what ODM would use now: the typed path, else the
	// copy next to odm.exe, else PATH. Checking runs "ffmpeg -version",
	// so it waits for typing to pause, and a stale result (the text
	// changed while it ran) is dropped by the generation counter.
	ffEntry := widget.NewEntry()
	ffEntry.SetText(settings.FFmpegPath)
	ffEntry.SetPlaceHolder(T("Found automatically"))
	ffStatus := hint("")
	var ffGen int
	var ffTimer *time.Timer
	checkFFmpeg := func(delay time.Duration) {
		ffGen++
		gen, typed := ffGen, ffEntry.Text
		if ffTimer != nil {
			ffTimer.Stop()
		}
		ffTimer = time.AfterFunc(delay, func() {
			msg := ffmpegStatus(typed)
			fyne.Do(func() {
				if gen == ffGen {
					ffStatus.SetText(msg)
				}
			})
		})
	}
	ffEntry.OnChanged = func(string) { checkFFmpeg(600 * time.Millisecond) }
	ffBrowse := widget.NewButtonWithIcon(T("Browse"), theme.FolderOpenIcon(), func() {
		dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
			if err == nil && r != nil {
				_ = r.Close()
				ffEntry.SetText(r.URI().Path())
			}
		}, w).Show()
	})
	checkFFmpeg(0)
	downloadsTab := container.NewVBox(
		header(T("Default download settings")),
		startCheck,
		progCheck,
		doneCheck,
		hint(T("These settings do not apply to queue processing.")),
		widget.NewSeparator(),
		announceBox,
		hint(T("A finished download is not notified when its complete dialog is shown while ODM is in front.")),
		widget.NewSeparator(),
		widget.NewLabel(T("If a link that is already in the list is added by the browser, clipboard or grabber:")),
		small(dupSelect, 220),
		hint(T("Adding a duplicate by hand always asks.")),
		widget.NewSeparator(),
		widget.NewLabel(T("User-Agent for downloads:")),
		uaEntry,
		widget.NewSeparator(),
		widget.NewLabel(T("The browser extension leaves downloads from these sites alone:")),
		excludeEntry,
		hint(T("Separate names by spaces. A leading *. matches subdomains.")),
		widget.NewSeparator(),
		widget.NewLabel(T("ffmpeg (joins the video and audio of some streams):")),
		container.NewBorder(nil, nil, nil, ffBrowse, ffEntry),
		ffStatus,
	)

	// Connection
	connOpts := make([]string, 32)
	for i := range connOpts {
		connOpts[i] = fmt.Sprint(i + 1)
	}
	connSelect := widget.NewSelect(connOpts, nil)
	connSelect.SetSelected(fmt.Sprint(settings.Connections))
	concEntry := numEntry(settings.Concurrent)
	limitEntry := numEntry(int(settings.SpeedLimit / 1024))
	// The value stays when the limiter is off, so it can be turned on again
	// from the menu or the tray without retyping it.
	limitCheck := widget.NewCheck(T("On"), nil)
	limitCheck.SetChecked(settings.EffectiveSpeedLimit() > 0)
	type hostRow struct {
		host string
		n    int
	}
	var hosts []hostRow
	for h, n := range settings.HostConnections {
		hosts = append(hosts, hostRow{h, n})
	}
	sort.Slice(hosts, func(i, j int) bool { return hosts[i].host < hosts[j].host })
	hostSel := -1
	hostTable := widget.NewTableWithHeaders(
		func() (int, int) { return len(hosts), 2 },
		func() fyne.CanvasObject { return newTableCell() },
		func(id widget.TableCellID, o fyne.CanvasObject) {
			c := o.(*tableCell)
			if id.Row < 0 || id.Row >= len(hosts) {
				c.setText("")
				return
			}
			c.setSelected(id.Row == hostSel)
			if id.Col == 0 {
				c.setText(hosts[id.Row].host)
			} else {
				c.setText(fmt.Sprint(hosts[id.Row].n))
			}
		},
	)
	hostTable.ShowHeaderColumn = false
	hostTable.CreateHeader = func() fyne.CanvasObject { return newHeaderLabel(nil) }
	hostTable.UpdateHeader = func(id widget.TableCellID, o fyne.CanvasObject) {
		o.(*headerLabel).SetText([]string{T("Server"), T("Number")}[id.Col])
	}
	hostTable.SetColumnWidth(0, 300)
	hostTable.SetColumnWidth(1, 80)
	hostTable.OnSelected = func(id widget.TableCellID) {
		hostSel = id.Row
		hostTable.UnselectAll()
		hostTable.Refresh()
	}
	newHost := widget.NewButton(T("New"), func() {
		host := widget.NewEntry()
		host.SetPlaceHolder("cdn.example.com")
		num := widget.NewSelect(connOpts, nil)
		num.SetSelected("4")
		dialog.ShowForm(T("Connection exception"), T("Add"), T("Cancel"), []*widget.FormItem{
			widget.NewFormItem(T("Server"), host), widget.NewFormItem(T("Connections"), num),
		}, func(ok bool) {
			h := strings.ToLower(strings.TrimSpace(host.Text))
			if !ok || h == "" {
				return
			}
			n := 4
			fmt.Sscanf(num.Selected, "%d", &n)
			hosts = append(hosts, hostRow{h, n})
			hostTable.Refresh()
		}, w)
	})
	delHost := widget.NewButton(T("Delete"), func() {
		if hostSel >= 0 && hostSel < len(hosts) {
			hosts = append(hosts[:hostSel], hosts[hostSel+1:]...)
			hostSel = -1
			hostTable.Refresh()
		}
	})
	lim := settings.Limits
	limOn := widget.NewCheck(T("Download limits"), nil)
	limOn.SetChecked(lim.Enabled)
	limMB := numEntry(lim.MB)
	limHours := numEntry(lim.Hours)
	limWarn := widget.NewCheck(T("Show a warning when downloads are stopped"), nil)
	limWarn.SetChecked(!lim.NoWarning)
	syncLimits := func(on bool) {
		for _, c := range []fyne.Disableable{limMB, limHours, limWarn} {
			if on {
				c.Enable()
			} else {
				c.Disable()
			}
		}
	}
	limOn.OnChanged = syncLimits
	syncLimits(lim.Enabled)
	connTab := container.NewBorder(
		container.NewVBox(
			header(T("Connections and limits")),
			widget.NewForm(
				widget.NewFormItem(T("Default max. connections"), small(connSelect, 90)),
				widget.NewFormItem(T("Simultaneous downloads"), small(concEntry, 90)),
				widget.NewFormItem(T("Global speed limit"), container.NewHBox(limitCheck, small(limitEntry, 90), widget.NewLabel("KB/s"))),
			),
			widget.NewLabel(T("Exceptions:")),
		),
		container.NewVBox(
			widget.NewSeparator(),
			limOn,
			container.NewHBox(widget.NewLabel(T("Download no more than")), small(limMB, 70), widget.NewLabel(T("MBytes every")),
				small(limHours, 60), widget.NewLabel(T("hours"))),
			limWarn,
		),
		nil,
		container.NewVBox(newHost, delHost),
		framed(hostTable),
	)

	// Proxy / Socks
	var (
		pxNone   = T("No proxy")
		pxSystem = T("Use system settings")
		pxManual = T("Manual proxy configuration")
	)
	pxMode := widget.NewRadioGroup([]string{pxNone, pxSystem, pxManual}, nil)
	pxType := widget.NewSelect([]string{"HTTP", "SOCKS5"}, nil)
	pxHost := widget.NewEntry()
	pxHost.SetText(settings.Proxy.Host)
	pxPort := numEntry(settings.Proxy.Port)
	if settings.Proxy.Port == 0 {
		pxPort.SetText("8080")
	}
	// Switching type suggests that protocol's usual port unless one was typed.
	pxType.OnChanged = func(t string) {
		if p := strings.TrimSpace(pxPort.Text); p == "" || p == "8080" || p == "1080" {
			if t == "SOCKS5" {
				pxPort.SetText("1080")
			} else {
				pxPort.SetText("8080")
			}
		}
	}
	pxUser := widget.NewEntry()
	pxUser.SetText(settings.Proxy.User)
	pxPass := widget.NewPasswordEntry()
	pxPass.SetText(settings.Proxy.Pass)
	manual := []fyne.Disableable{pxType, pxHost, pxPort, pxUser, pxPass}
	pxMode.OnChanged = func(m string) {
		for _, c := range manual {
			if m == pxManual {
				c.Enable()
			} else {
				c.Disable()
			}
		}
	}
	switch settings.Proxy.Mode {
	case engine.ProxyNone:
		pxMode.SetSelected(pxNone)
		pxType.SetSelected("HTTP")
	case engine.ProxyHTTP:
		pxMode.SetSelected(pxManual)
		pxType.SetSelected("HTTP")
	case engine.ProxySOCKS5:
		pxMode.SetSelected(pxManual)
		pxType.SetSelected("SOCKS5")
	default:
		pxMode.SetSelected(pxSystem)
		pxType.SetSelected("HTTP")
	}
	proxyTab := container.NewVBox(
		header(T("Proxy / socks configuration")),
		pxMode,
		widget.NewForm(
			widget.NewFormItem(T("Type"), small(pxType, 120)),
			widget.NewFormItem(T("Proxy server"), pxHost),
			widget.NewFormItem(T("Port"), small(pxPort, 90)),
			widget.NewFormItem(T("User name"), pxUser),
			widget.NewFormItem(T("Password"), pxPass),
		),
		hint(T("System settings use HTTP(S)_PROXY, and on Windows the proxy entered in Internet Options. Automatic configuration scripts (PAC) are not evaluated.")),
	)

	// Site logins
	logins := append([]engine.Login(nil), settings.Logins...)
	loginSel := -1
	loginsTable := widget.NewTableWithHeaders(
		func() (int, int) { return len(logins), 3 },
		func() fyne.CanvasObject { return newTableCell() },
		func(id widget.TableCellID, o fyne.CanvasObject) {
			c := o.(*tableCell)
			if id.Row < 0 || id.Row >= len(logins) {
				c.setText("")
				return
			}
			c.setSelected(id.Row == loginSel)
			l := logins[id.Row]
			c.setText([]string{l.Host, l.User, strings.Repeat("•", len(l.Pass))}[id.Col])
		},
	)
	loginsTable.ShowHeaderColumn = false
	loginsTable.CreateHeader = func() fyne.CanvasObject { return newHeaderLabel(nil) }
	loginsTable.UpdateHeader = func(id widget.TableCellID, o fyne.CanvasObject) {
		o.(*headerLabel).SetText([]string{T("Site"), T("User"), T("Password")}[id.Col])
	}
	loginsTable.SetColumnWidth(0, 250)
	loginsTable.SetColumnWidth(1, 130)
	loginsTable.SetColumnWidth(2, 110)
	loginsTable.OnSelected = func(id widget.TableCellID) {
		loginSel = id.Row
		loginsTable.UnselectAll()
		loginsTable.Refresh()
	}
	editLogin := func(idx int) {
		var cur engine.Login
		if idx >= 0 {
			cur = logins[idx]
		}
		site, user, pass := widget.NewEntry(), widget.NewEntry(), widget.NewPasswordEntry()
		site.SetText(cur.Host)
		site.SetPlaceHolder("files.example.com")
		user.SetText(cur.User)
		pass.SetText(cur.Pass)
		dialog.ShowForm(T("Site login"), T("OK"), T("Cancel"), []*widget.FormItem{
			widget.NewFormItem(T("Site"), site), widget.NewFormItem(T("User"), user), widget.NewFormItem(T("Password"), pass),
		}, func(ok bool) {
			if !ok || strings.TrimSpace(site.Text) == "" {
				return
			}
			l := engine.Login{Host: strings.TrimSpace(site.Text), User: user.Text, Pass: pass.Text}
			if idx >= 0 {
				logins[idx] = l
			} else {
				logins = append(logins, l)
			}
			loginsTable.Refresh()
		}, w)
	}
	loginsTab := container.NewBorder(
		header(T("User names and passwords for sites")),
		container.NewHBox(
			widget.NewButton(T("New"), func() { editLogin(-1) }),
			widget.NewButton(T("Edit"), func() {
				if loginSel >= 0 && loginSel < len(logins) {
					editLogin(loginSel)
				}
			}),
			widget.NewButton(T("Remove"), func() {
				if loginSel >= 0 && loginSel < len(logins) {
					logins = append(logins[:loginSel], logins[loginSel+1:]...)
					loginSel = -1
					loginsTable.Refresh()
				}
			}),
		),
		nil, nil, framed(loginsTable),
	)

	tabs := container.NewAppTabs(
		container.NewTabItem(T("General"), generalTab),
		container.NewTabItem(T("File types"), typesTab),
		container.NewTabItem(T("Save to"), saveTab),
		container.NewTabItem(T("Downloads"), downloadsTab),
		container.NewTabItem(T("Connection"), connTab),
		container.NewTabItem(T("Proxy / Socks"), proxyTab),
		container.NewTabItem(T("Site logins"), loginsTab),
	)
	tabs.SelectIndex(tab)

	closeWin := func() {
		w.Close()
	}
	w.SetOnClosed(func() {
		a.optionsWin = nil
		if ffTimer != nil {
			ffTimer.Stop()
		}
	})
	save := func() {
		conns, conc, limit := settings.Connections, settings.Concurrent, settings.SpeedLimit/1024
		fmt.Sscanf(connSelect.Selected, "%d", &conns)
		fmt.Sscanf(concEntry.Text, "%d", &conc)
		fmt.Sscanf(limitEntry.Text, "%d", &limit)
		cats.store()
		cleanDirs := map[string]string{}
		for k, v := range catDirs {
			if v != "" && cats.has(k) { // a deleted category keeps no folder
				cleanDirs[k] = v
			}
		}
		hostMap := map[string]int{}
		for _, h := range hosts {
			hostMap[h.host] = h.n
		}
		proxy := engine.ProxyConfig{Mode: engine.ProxySystem}
		switch pxMode.Selected {
		case pxNone:
			proxy.Mode = engine.ProxyNone
		case pxManual:
			port := 0
			fmt.Sscanf(pxPort.Text, "%d", &port)
			proxy = engine.ProxyConfig{Mode: engine.ProxyHTTP, Host: strings.TrimSpace(pxHost.Text), Port: port,
				User: strings.TrimSpace(pxUser.Text), Pass: pxPass.Text}
			if pxType.Selected == "SOCKS5" {
				proxy.Mode = engine.ProxySOCKS5
			}
			if proxy.Host == "" {
				tabs.SelectIndex(5)
				dialog.ShowInformation(T("Proxy"), T("Enter the proxy server address, or choose another option."), w)
				return
			}
		}
		lim := engine.DownloadLimits{Enabled: limOn.Checked, NoWarning: !limWarn.Checked}
		fmt.Sscanf(limMB.Text, "%d", &lim.MB)
		fmt.Sscanf(limHours.Text, "%d", &lim.Hours)

		// Start from the current settings so fields this window does not
		// edit (queues, schedules) survive OK.
		next := a.mgr.Settings()
		next.Dir = strings.TrimSpace(dirEntry.Text)
		next.NoRememberDir = !rememberCheck.Checked
		// A new default folder, or remembering turned off, offers the
		// default folder again rather than the one chosen before.
		if next.Dir != settings.Dir || next.NoRememberDir {
			next.LastDir = ""
		}
		next.Connections = conns
		next.Concurrent = conc
		next.SpeedLimit = limit * 1024
		next.LimiterOff = !limitCheck.Checked
		next.Categories = catsCheck.Checked
		next.Clipboard = clipCheck.Checked
		next.NoUpdateCheck = !updCheck.Checked
		next.NoCloseToTray = !trayCheck.Checked
		next.NoProgressDialog = !progCheck.Checked
		next.NoStartDialog = !startCheck.Checked
		next.ExcludeSites = strings.Fields(strings.ReplaceAll(excludeEntry.Text, ",", " "))
		next.NoCompleteDialog = !doneCheck.Checked
		next.NoNotifications = !notifyCheck.Checked
		next.Sounds = soundCheck.Checked
		next.FileTypes = parseFileTypes(typesEntry.Text)
		next.CategoryDirs = cleanDirs
		next.CategoryDefs = cats.defs // the built-in set comes back as nil
		next.HostConnections = hostMap
		next.UserAgent = strings.TrimSpace(uaEntry.Text)
		next.FFmpegPath = strings.TrimSpace(ffEntry.Text)
		next.DuplicateAuto = engine.DuplicateSkip
		if dupSelect.Selected == dupAgain {
			next.DuplicateAuto = engine.DuplicateAgain
		}
		next.Proxy = proxy
		next.Logins = logins
		next.Limits = lim
		a.mgr.UpdateSettings(next)
		if mode, accent := themeModes[themeSelect.SelectedIndex()], accentPicked(); mode != themeWas || accent != accentWas {
			prefs.SetString(prefTheme, mode)
			prefs.SetString(prefAccent, accent)
			a.applyTheme()
		}
		if mode := languageModes[langSelect.SelectedIndex()]; mode != langWas {
			prefs.SetString(prefLanguage, mode)
			dialog.ShowInformation(T("Language"), T("The new language shows after ODM is started again."), a.win)
		}
		if startupCheck.Checked != startupWas {
			exe, err := os.Executable()
			if err == nil {
				exe, err = filepath.EvalSymlinks(exe)
			}
			if err == nil {
				err = autostart.Set(startupCheck.Checked, exe)
			}
			if err != nil {
				dialog.ShowError(fmt.Errorf(T("could not change start-up launch: %w"), err), a.win)
			}
		}
		a.refresh()
		closeWin()
	}
	okBtn := widget.NewButton(T("OK"), save)
	okBtn.Importance = widget.HighImportance
	buttons := container.NewHBox(layout.NewSpacer(), okBtn, widget.NewButton(T("Cancel"), closeWin))
	w.SetContent(container.New(layout.NewCustomPaddedLayout(8, 10, 10, 10),
		container.NewBorder(nil, buttons, nil, nil, tabs)))
	w.Resize(fyne.NewSize(640, 560))
	w.Show()
}

// ffmpegStatus describes the ffmpeg that the typed path (empty: the
// automatic search) leads to, with its version line.
func ffmpegStatus(typed string) string {
	bin, err := ffmpeg.Locate(typed)
	if err != nil {
		if strings.TrimSpace(typed) == "" {
			return T("Not found. Streams that keep video and audio apart need it: put ffmpeg next to ODM, add it to PATH, or choose it here.")
		}
		return T("No ffmpeg at this path. Leave it empty to search next to ODM and on PATH.")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	v, err := ffmpeg.Version(ctx, bin)
	if err != nil {
		return Tf("Found %s, but it did not run: %s", bin, err.Error())
	}
	// "ffmpeg version 7.1 Copyright (c) 2000-2024 the FFmpeg developers"
	v, _, _ = strings.Cut(v, " Copyright")
	if v == "" {
		v = T("unknown version")
	}
	return Tf("Using %s (%s)", bin, v)
}

// parseFileTypes turns "torrent, .SRT nupkg" into ["torrent" "srt" "nupkg"].
func parseFileTypes(s string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, f := range strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == ',' || r == ';' || r == '\n' || r == '\t' || r == '\r'
	}) {
		f = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(f), "."))
		if f != "" && !seen[f] {
			seen[f] = true
			out = append(out, f)
		}
	}
	return out
}

// askDuplicate tells the user the address is already in the list and lets
// them show that entry, download it again (it gets a numbered file name),
// or cancel.
func (a *application) askDuplicate(parent fyne.Window, existing engine.Info, again func()) {
	msg := widget.NewLabel(Tf("This address is already in the list as %q (%s).\nDownloading it again saves it under a numbered name.",
		existing.FileName, statusText(existing)))
	msg.Wrapping = fyne.TextWrapWord
	var dlg *dialog.CustomDialog
	show := widget.NewButton(T("Show existing"), func() {
		dlg.Hide()
		a.openProgressWindow(existing.ID)
	})
	dl := widget.NewButton(T("Download again"), func() { dlg.Hide(); again() })
	dl.Importance = widget.HighImportance
	dlg = dialog.NewCustomWithoutButtons(T("Duplicate download"), msg, parent)
	dlg.SetButtons([]fyne.CanvasObject{widget.NewButton(T("Cancel"), func() { dlg.Hide() }), show, dl})
	dlg.Resize(fyne.NewSize(460, 1))
	dlg.Show()
}

// refreshAddress asks for a new URL for an unfinished download whose link
// expired. The engine checks it serves the same file before switching.
func (a *application) refreshAddress(d engine.Info) {
	entry := widget.NewEntry()
	entry.SetText(d.URL)
	entry.MultiLine = false
	note := widget.NewLabel(T("Paste the new address of the same file. Progress is kept if the server supports resuming."))
	note.Wrapping = fyne.TextWrapWord
	note.Importance = widget.LowImportance
	dlg := dialog.NewCustomConfirm(T("Refresh download address"), T("Change"), T("Cancel"),
		container.NewVBox(widget.NewLabel(d.FileName), entry, note), func(ok bool) {
			if !ok || strings.TrimSpace(entry.Text) == "" || strings.TrimSpace(entry.Text) == d.URL {
				return
			}
			newURL := strings.TrimSpace(entry.Text)
			go func() { // probes the network; keep it off the UI thread
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				err := a.mgr.Readdress(ctx, d.ID, newURL)
				fyne.Do(func() {
					if err != nil {
						dialog.ShowError(err, a.win)
						return
					}
					a.refresh()
				})
			}()
		}, a.win)
	dlg.Resize(fyne.NewSize(560, 1)) // wide enough for a typical URL; height fits content
	dlg.Show()
}

/* ---------- browser integration ---------- */

// registerBrowser points Chrome, Edge, Chromium and Firefox at this ODM
// executable as the extension's native host, then reports per browser.
// selfExe is this ODM's executable path as registrations record it.
func selfExe() (string, error) {
	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe)
	}
	return exe, err
}

func (a *application) registerBrowser(parent fyne.Window) {
	exe, err := selfExe()
	if err != nil {
		dialog.ShowError(fmt.Errorf(T("cannot locate the ODM executable: %w"), err), parent)
		return
	}
	var failed []string
	for _, r := range browserreg.Register(exe, a.mgr.ConfigDir()) {
		if r.Err != nil {
			failed = append(failed, r.String())
		}
	}
	a.setBrowserRegister(true)
	// Success shows in the status line on the Options window; a dialog is
	// worth it only when a browser could not be registered.
	if len(failed) > 0 {
		dialog.ShowError(fmt.Errorf(T("could not register:\n%s"), strings.Join(failed, "\n")), parent)
	}
}

// setBrowserRegister records whether ODM registers itself with the
// browsers when it starts.
func (a *application) setBrowserRegister(on bool) {
	next := a.mgr.Settings()
	if next.NoBrowserRegister != !on {
		next.NoBrowserRegister = !on
		a.mgr.UpdateSettings(next)
	}
}

/* ---------- update check ---------- */

// checkForUpdates asks GitHub for the latest release. A start-up check stays
// silent unless there is something new; a manual one always answers.
func (a *application) checkForUpdates(manual bool) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		rel, err := update.Latest(ctx, a.mgr.HTTPClient(), update.DefaultAPI, version.Repo)
		fyne.Do(func() {
			switch {
			case err != nil:
				if manual {
					dialog.ShowError(fmt.Errorf(T("could not check for updates: %w"), err), a.win)
				}
			case update.Newer(rel.Tag, version.Version):
				a.showUpdate(rel)
			case manual:
				dialog.ShowInformation(T("Check for updates"),
					Tf("You are running the latest version (%s).", version.Version), a.win)
			}
		})
	}()
}

func (a *application) showUpdate(rel update.Release) {
	head := widget.NewLabel(Tf("ODM %s is available — you have %s.", rel.Tag, version.Version))
	notes := widget.NewRichTextFromMarkdown(rel.Notes)
	notes.Wrapping = fyne.TextWrapWord
	scroll := container.NewVScroll(notes)
	scroll.SetMinSize(fyne.NewSize(480, 220))
	dlg := dialog.NewCustomWithoutButtons(T("Update available"), container.NewBorder(head, nil, nil, nil, scroll), a.win)
	open := widget.NewButtonWithIcon(T("Open release page"), theme.DownloadIcon(), func() {
		if u, err := url.Parse(rel.URL); err == nil {
			_ = a.fyneApp.OpenURL(u)
		}
		dlg.Hide()
	})
	open.Importance = widget.HighImportance
	dlg.SetButtons([]fyne.CanvasObject{widget.NewButton(T("Later"), dlg.Hide), open})
	dlg.Show()
}

/* ---------- scheduler completion ---------- */

// handleAction carries out a scheduler request in the UI.
func (a *application) handleAction(act engine.Action) {
	switch act.Kind {
	case engine.ActionOpen:
		openPath(act.Path)
	case engine.ActionLimit:
		lim := a.mgr.Settings().Limits
		dialog.ShowInformation(T("Download limit reached"), Tf(
			T("More than %d MB were downloaded in the last %d hours, so downloads were stopped.\nThey can start again once the window has passed."),
			lim.MB, lim.Hours), a.win)
	case engine.ActionExit, engine.ActionSleep, engine.ActionHibernate, engine.ActionShutdown:
		a.confirmWhenDone(act.Kind)
	}
}

// confirmWhenDone counts down before the scheduler's finish action so the
// user can cancel a shutdown they did not expect.
func (a *application) confirmWhenDone(action string) {
	const wait = 30
	verb := map[string]string{
		engine.ActionExit:      T("ODM will exit"),
		engine.ActionSleep:     T("The computer will go to sleep"),
		engine.ActionHibernate: T("The computer will hibernate"),
		engine.ActionShutdown:  T("The computer will shut down"),
	}[action]
	msg := widget.NewLabel("")
	left := wait
	update := func() { msg.SetText(Tf("All queues have finished.\n%s in %d s.", verb, left)) }
	update()

	var dlg *dialog.CustomDialog
	done := make(chan struct{})
	var once sync.Once
	finish := func(run bool) {
		once.Do(func() {
			close(done)
			dlg.Hide()
			if run {
				a.runWhenDone(action)
			}
		})
	}
	dlg = dialog.NewCustomWithoutButtons(T("Queues finished"), msg, a.win)
	nowBtn := widget.NewButton(T("Now"), func() { finish(true) })
	nowBtn.Importance = widget.DangerImportance
	dlg.SetButtons([]fyne.CanvasObject{widget.NewButton(T("Cancel"), func() { finish(false) }), nowBtn})
	dlg.Show()

	go func() {
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				fyne.Do(func() {
					left--
					update()
					if left <= 0 {
						finish(true)
					}
				})
			}
		}
	}()
}

func (a *application) runWhenDone(action string) {
	switch action {
	case engine.ActionSleep, engine.ActionHibernate:
		// ODM keeps running and carries on after the computer wakes.
		if err := osutil.Suspend(action == engine.ActionHibernate); err != nil {
			dialog.ShowError(fmt.Errorf(T("could not %s: %w"), action, err), a.win)
		}
		return
	case engine.ActionShutdown:
		if err := osutil.Shutdown(); err != nil {
			dialog.ShowError(fmt.Errorf(T("could not shut down: %w"), err), a.win)
			return
		}
	}
	a.fyneApp.Quit()
}

/* ---------- Site Grabber ---------- */

func (a *application) openGrabberDialog() {
	settings := a.mgr.Settings()

	urlEntry := widget.NewEntry()
	urlEntry.SetPlaceHolder("https://site.example/downloads/")
	depthEntry := widget.NewEntry()
	depthEntry.SetText("1")
	pagesEntry := widget.NewEntry()
	pagesEntry.SetText("50")
	filesEntry := widget.NewEntry()
	filesEntry.SetText("100")
	sameHost := widget.NewCheck(T("Stay on the same host"), nil)
	sameHost.SetChecked(true)
	startNow := widget.NewCheck(T("Start downloads immediately"), nil)
	startNow.SetChecked(true)
	info := widget.NewLabel("")

	form := widget.NewForm(
		widget.NewFormItem(T("Start page"), urlEntry),
		widget.NewFormItem(T("Depth"), depthEntry),
		widget.NewFormItem(T("Max pages"), pagesEntry),
		widget.NewFormItem(T("Max files"), filesEntry),
	)
	content := container.NewVBox(form, sameHost, startNow, info)

	dlg := dialog.NewCustomWithoutButtons(T("ODM Site Grabber"), content, a.win)
	start := func() {
		depth, pages, files := 1, 50, 100
		fmt.Sscanf(depthEntry.Text, "%d", &depth)
		fmt.Sscanf(pagesEntry.Text, "%d", &pages)
		fmt.Sscanf(filesEntry.Text, "%d", &files)
		opts := engine.GrabberOptions{
			URL:      strings.TrimSpace(urlEntry.Text),
			Depth:    depth,
			MaxPages: pages,
			MaxFiles: files,
			SameHost: sameHost.Checked,
			Dir:      settings.Dir,
			Start:    startNow.Checked,
		}
		if opts.URL == "" {
			info.SetText(T("Enter a start page."))
			return
		}
		info.SetText(T("Exploring…"))
		go func() {
			res, err := a.mgr.Grab(context.Background(), opts, func(p, f int) {
				fyne.Do(func() { info.SetText(Tf("Exploring… %d pages, %d files", p, f)) })
			})
			fyne.Do(func() {
				if err != nil {
					info.SetText(T("Failed: ") + err.Error())
					return
				}
				info.SetText(Tf("Done: %d pages explored, %d files queued.", res.Pages, len(res.Files)))
				a.refresh()
			})
		}()
	}
	startBtn := widget.NewButtonWithIcon(T("Start"), theme.MediaPlayIcon(), start)
	startBtn.Importance = widget.HighImportance
	closeBtn := widget.NewButton(T("Close"), func() { dlg.Hide() })
	dlg.SetButtons([]fyne.CanvasObject{closeBtn, startBtn})
	dlg.Resize(fyne.NewSize(560, 1))
	dlg.Show()
}

/* ---------- Progress window ---------- */

// progressView is the per-download window: tabs on top, then the progress
// bar and Hide details / Pause / Cancel, then the connection map and table,
// which "Hide details" folds away.
type progressView struct {
	win        fyne.Window
	url        *widget.Label
	status     *widget.Label
	size       *widget.Label
	done       *widget.Label
	rate       *widget.Label
	limitRate  *widget.Label
	eta        *widget.Label
	resume     *widget.Label
	saveTo     *widget.Label
	bar        *widget.ProgressBar
	graph      *speedGraph
	strip      *chunkStrip
	mapLabel   *widget.Label
	conns      *widget.Table
	connBox    fyne.CanvasObject
	connRows   []engine.ChunkInfo
	connMu     sync.Mutex
	pauseBtn   *widget.Button
	openCheck  *widget.Check
	details    fyne.CanvasObject
	top        fyne.CanvasObject
	hideBtn    *widget.Button
	detailsOff bool
	live       bool // a live stream being recorded
}

// infoGrid lays out "label  value" rows with muted, left-aligned captions.
func infoGrid(rows ...fyne.CanvasObject) *fyne.Container {
	return container.New(layout.NewFormLayout(), rows...)
}

func caption(s string) *widget.Label {
	l := widget.NewLabel(s)
	l.Importance = widget.LowImportance
	return l
}

func (a *application) openProgressWindow(id string) {
	if v, ok := a.progressWindows[id]; ok {
		v.win.Show()
		v.win.RequestFocus()
		a.updateProgressWindow(id)
		return
	}
	d, ok := a.mgr.Get(id) // not a.byID: the list filter may hide the item
	if !ok {
		return
	}

	v := &progressView{}
	v.win = a.fyneApp.NewWindow(T("Download"))

	newValue := func() *widget.Label {
		l := widget.NewLabel("")
		l.Truncation = fyne.TextTruncateEllipsis
		return l
	}
	v.url, v.status, v.size, v.done = newValue(), newValue(), newValue(), newValue()
	v.rate, v.limitRate, v.eta, v.resume, v.saveTo = newValue(), newValue(), newValue(), newValue(), newValue()
	v.bar = widget.NewProgressBar()
	v.bar.TextFormatter = func() string {
		if v.live {
			return T("Recording")
		}
		return fmt.Sprintf("%.1f%%", v.bar.Value*100)
	}
	v.strip = newChunkStrip()
	v.graph = newSpeedGraph()

	// Download status, with the rate of the last minute under it.
	statusTab := container.NewVBox(
		v.url,
		infoGrid(
			caption(T("Status")), v.status,
			caption(T("File size")), v.size,
			caption(T("Downloaded")), v.done,
			caption(T("Transfer rate")), v.rate,
			caption(T("Time left")), v.eta,
			caption(T("Resumable")), v.resume,
		),
		v.graph,
	)

	// Speed limiter: applied as soon as it changes, like the reference UI.
	limitEntry := widget.NewEntry()
	limitEntry.SetText("500")
	useLimit := widget.NewCheck(T("Use speed limiter"), nil)
	apply := func() {
		kb := 0
		fmt.Sscanf(strings.TrimSpace(limitEntry.Text), "%d", &kb)
		if useLimit.Checked && kb > 0 {
			a.mgr.SetSpeedLimit(id, int64(kb)*1024)
		} else {
			a.mgr.SetSpeedLimit(id, 0)
		}
	}
	if d.SpeedLimit > 0 {
		limitEntry.SetText(fmt.Sprint(d.SpeedLimit / 1024))
		useLimit.SetChecked(true)
	} else {
		limitEntry.Disable()
	}
	useLimit.OnChanged = func(on bool) {
		if on {
			limitEntry.Enable()
		} else {
			limitEntry.Disable()
		}
		apply()
	}
	limitEntry.OnChanged = func(string) { apply() }
	limitTab := container.NewVBox(
		infoGrid(caption(T("Transfer rate")), v.limitRate),
		useLimit,
		infoGrid(caption(T("Maximum speed")), container.NewBorder(nil, nil, nil, widget.NewLabel("KB/s"), limitEntry)),
	)

	// Options on completion
	settings := a.mgr.Settings()
	completeCheck := widget.NewCheck(T("Show download complete dialog"), func(on bool) {
		next := a.mgr.Settings()
		next.NoCompleteDialog = !on
		a.mgr.UpdateSettings(next)
	})
	completeCheck.SetChecked(!settings.NoCompleteDialog)
	v.openCheck = widget.NewCheck(T("Open the file when done"), func(b bool) { a.mgr.SetOpenWhenDone(id, b) })
	compTab := container.NewVBox(
		infoGrid(caption(T("Save to")), v.saveTo),
		completeCheck,
		v.openCheck,
	)

	tabs := container.NewAppTabs(
		container.NewTabItem(T("Download status"), statusTab),
		container.NewTabItem(T("Speed limiter"), limitTab),
		container.NewTabItem(T("Options on completion"), compTab),
	)

	v.pauseBtn = widget.NewButtonWithIcon(T("Pause"), theme.MediaPauseIcon(), func() {
		if d, ok := a.mgr.Get(id); ok {
			if d.Status == engine.StatusDownloading || d.Status == engine.StatusQueued {
				a.mgr.Pause(id)
			} else {
				a.mgr.Resume(id)
			}
			a.updateProgressWindow(id)
		}
	})
	// Cancel stops the transfer and closes the window; closing the window
	// with its title-bar button leaves the download running.
	cancelBtn := widget.NewButton(T("Cancel"), func() {
		a.mgr.Pause(id)
		v.win.Close()
	})
	v.hideBtn = widget.NewButton(T("<< Hide details"), func() { a.toggleProgressDetails(v) })

	// Connection table with a header row (N. / Downloaded / Info).
	v.conns = widget.NewTableWithHeaders(
		func() (int, int) {
			v.connMu.Lock()
			defer v.connMu.Unlock()
			return len(v.connRows), 3
		},
		func() fyne.CanvasObject { return newTableCell() },
		func(cell widget.TableCellID, o fyne.CanvasObject) {
			v.connMu.Lock()
			defer v.connMu.Unlock()
			c := o.(*tableCell)
			if cell.Row < 0 || cell.Row >= len(v.connRows) {
				c.setText("")
				return
			}
			r := v.connRows[cell.Row]
			switch cell.Col {
			case 0:
				c.setText(fmt.Sprint(cell.Row + 1))
			case 1:
				// humanBytes leaves 0 blank (fine for list cells); here a
				// connection that has not started yet should read 0 B.
				if r.Done <= 0 {
					c.setText("0 B")
				} else {
					c.setText(humanBytes(r.Done))
				}
			case 2:
				c.setText(r.State)
			}
		},
	)
	v.conns.ShowHeaderColumn = false
	v.conns.CreateHeader = func() fyne.CanvasObject { return newHeaderLabel(nil) }
	v.conns.UpdateHeader = func(id widget.TableCellID, o fyne.CanvasObject) {
		o.(*headerLabel).SetText([]string{"N.", T("Downloaded"), T("Info")}[id.Col])
	}
	v.conns.SetColumnWidth(0, 44)
	v.conns.SetColumnWidth(1, 130)
	v.conns.SetColumnWidth(2, 330)

	buttons := container.NewHBox(v.hideBtn, layout.NewSpacer(), v.pauseBtn, cancelBtn)
	v.top = container.NewVBox(tabs, v.bar, buttons)
	v.mapLabel = widget.NewLabelWithStyle(T("Connection map"), fyne.TextAlignCenter, fyne.TextStyle{})
	v.connBox = framed(v.conns)
	v.details = container.NewBorder(container.NewVBox(v.mapLabel, v.strip), nil, nil, nil, v.connBox)

	v.win.SetContent(container.New(layout.NewCustomPaddedLayout(6, 10, 10, 10),
		container.NewBorder(v.top, nil, nil, nil, v.details)))
	v.win.SetOnClosed(func() { delete(a.progressWindows, id) })

	a.progressWindows[id] = v
	a.updateProgressWindow(id)
	v.win.Resize(fyne.NewSize(560, detailsHeight(v)))
	v.win.Show()
}

// toggleProgressDetails folds the connection map and table away and shrinks
// the window to the top part, or restores them.
func (a *application) toggleProgressDetails(v *progressView) {
	v.detailsOff = !v.detailsOff
	w := v.win.Canvas().Size().Width
	if v.detailsOff {
		v.details.Hide()
		v.hideBtn.SetText(T("Show details >>"))
		v.win.Resize(fyne.NewSize(w, v.top.MinSize().Height+20))
	} else {
		v.details.Show()
		v.hideBtn.SetText(T("<< Hide details"))
		v.win.Resize(fyne.NewSize(w, detailsHeight(v)))
	}
}

// detailsHeight is the window height with details shown: room for the
// connection table, or just the strip for a stream, which has no table.
func detailsHeight(v *progressView) float32 {
	if v.connBox.Visible() {
		return 560
	}
	return v.top.MinSize().Height + v.details.MinSize().Height + 30
}

// segmentsText is "12 of 340", or "" before a run has read the playlist.
func segmentsText(d engine.Info) string {
	if d.Segments <= 0 {
		return ""
	}
	return Tf("%d of %d", d.SegmentsDone, d.Segments)
}

// autoProgress opens the progress window for a download the user just
// started, unless that is turned off in Options.
func (a *application) autoProgress(id string) {
	if !a.mgr.Settings().NoProgressDialog {
		a.openProgressWindow(id)
	}
}

func (a *application) updateProgressWindow(id string) {
	v, ok := a.progressWindows[id]
	if !ok {
		return
	}
	d, ok := a.mgr.Get(id)
	if !ok {
		return
	}
	// A recording's Total follows Done, so its percentage means nothing.
	v.live = d.Live && d.Status == engine.StatusDownloading
	if v.live {
		v.win.SetTitle(Tf("Recording %s", d.FileName))
	} else {
		v.win.SetTitle(fmt.Sprintf("%.0f%% %s", d.Progress, d.FileName))
	}
	v.url.SetText(d.URL)
	// Same wording as the list's Status column, minus the percentage that
	// the Downloaded row already shows.
	status := map[engine.Status]string{
		engine.StatusQueued:      T("Queued"),
		engine.StatusDownloading: T("Receiving data…"),
		engine.StatusPaused:      T("Paused"),
		engine.StatusComplete:    T("Complete"),
		engine.StatusError:       T("Error"),
	}[d.Status]
	if status == "" {
		status = string(d.Status)
	}
	if v.live {
		status = T("Recording live stream…")
	}
	if d.Verifying {
		status = T("Verifying checksum…")
	}
	if d.Error != "" {
		status += " — " + d.Error
	}
	v.status.SetText(status)
	v.size.SetText(dash(humanBytes(d.Total)))
	if d.Live {
		v.done.SetText(dash(humanBytes(d.Done)))
	} else {
		v.done.SetText(fmt.Sprintf("%s (%.2f %%)", dash(humanBytes(d.Done)), d.Progress))
	}
	v.rate.SetText(dash(rateText(d)))
	v.limitRate.SetText(dash(rateText(d)))
	if d.Status == engine.StatusDownloading {
		v.graph.add(d.Speed, time.Now())
	}
	if d.Live {
		v.eta.SetText(dash("")) // until the broadcast ends or is stopped
	} else {
		v.eta.SetText(dash(etaText(d)))
	}
	if d.RangeOK {
		v.resume.SetText(T("Yes"))
	} else {
		v.resume.SetText(T("No"))
	}
	if d.FileName != "" {
		v.saveTo.SetText(filepath.Join(d.Dir, d.FileName))
	}
	if v.live {
		v.bar.SetValue(0)
	} else if d.Total > 0 {
		v.bar.SetValue(d.Progress / 100)
	}
	if d.Stream != "" {
		// A stream has no byte ranges or connections to map: its pieces
		// are segments, fetched a few at a time in any order. The strip
		// becomes one bar of segments done, and the empty table goes.
		// The counts are not saved, so after a restart a finished stream
		// has none; it is shown whole rather than as "-".
		segs, have := d.Segments, d.SegmentsDone
		text := segmentsText(d)
		if segs == 0 && d.Status == engine.StatusComplete {
			segs, have, text = 1, 1, T("complete")
		}
		if d.Live && d.Status != engine.StatusComplete {
			text = ""
			if d.SegmentsDone > 0 {
				text = Tf("%d recorded", d.SegmentsDone)
			}
			segs, have = 0, 0 // nothing to fill toward
		}
		v.mapLabel.SetText(T("Segments: ") + dash(text))
		var bar []engine.ChunkInfo
		if segs > 0 {
			bar = []engine.ChunkInfo{{End: int64(segs) - 1, Done: int64(have)}}
		}
		v.strip.set(bar, int64(segs))
		v.connBox.Hide()
	} else {
		v.strip.set(d.Chunks, d.Total)
	}
	if v.live {
		// Pausing a recording ends it and saves the file (live.go).
		v.pauseBtn.SetText(T("Stop"))
		v.pauseBtn.SetIcon(theme.MediaStopIcon())
	} else if d.Status == engine.StatusDownloading || d.Status == engine.StatusQueued {
		v.pauseBtn.SetText(T("Pause"))
		v.pauseBtn.SetIcon(theme.MediaPauseIcon())
	} else {
		v.pauseBtn.SetText(T("Start"))
		v.pauseBtn.SetIcon(theme.MediaPlayIcon())
	}
	v.openCheck.SetChecked(d.OpenWhenDone)

	v.connMu.Lock()
	v.connRows = d.Chunks
	v.connMu.Unlock()
	v.conns.Refresh()
}

/* ---------- download complete ---------- */

// showComplete is the window shown when a download finishes: what was saved
// where, with Open / Open folder / Close.
func (a *application) showComplete(d engine.Info) {
	w := a.fyneApp.NewWindow(T("Download complete"))
	path := filepath.Join(d.Dir, d.FileName)
	value := func(s string) *widget.Label {
		l := widget.NewLabel(s)
		l.Truncation = fyne.TextTruncateEllipsis
		return l
	}
	again := widget.NewCheck(T("Don't show this dialog again"), func(on bool) {
		next := a.mgr.Settings()
		next.NoCompleteDialog = on
		a.mgr.UpdateSettings(next)
	})
	openBtn := widget.NewButtonWithIcon(T("Open"), theme.FileIcon(), func() { openPath(path); w.Close() })
	openBtn.Importance = widget.HighImportance
	folderBtn := widget.NewButtonWithIcon(T("Open folder"), theme.FolderOpenIcon(), func() { revealPath(path); w.Close() })
	closeBtn := widget.NewButton(T("Close"), w.Close)
	content := container.New(layout.NewCustomPaddedLayout(8, 10, 12, 12), container.NewVBox(
		widget.NewLabelWithStyle(T("Download complete"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		infoGrid(
			caption(T("Address")), value(d.URL),
			caption(T("File size")), value(dash(humanBytes(d.Total))),
			caption(T("Saved to")), value(path),
			caption(T("Checksum")), value(checksumText(d.Checksum)),
		),
		again,
		container.NewHBox(layout.NewSpacer(), openBtn, folderBtn, closeBtn),
	))
	w.SetContent(content)
	w.Resize(fyne.NewSize(520, content.MinSize().Height))
	w.Show()
}
