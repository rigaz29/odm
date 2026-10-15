//go:build fyne

package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"odm/internal/engine"
)

// limitsEntry is the pseudo-queue row that shows the download-limits page.
const limitsEntry = "\x00limits"

// schedulerView is the Scheduler window: queues on the left; per queue a
// Schedule tab and a Files tab; plus a Download limits page. Edits stay in
// sched/limits until Apply, as in a classic settings dialog.
type schedulerView struct {
	a       *application
	win     fyne.Window
	names   []string // queue names, then limitsEntry
	current string
	sched   map[string]engine.QueueSchedule
	limits  engine.DownloadLimits
	dirty   bool
	loading bool // set while widgets are filled, so their callbacks don't mark dirty

	list     *widget.List
	title    *widget.Label
	body     *fyne.Container
	queueBox fyne.CanvasObject
	limitBox fyne.CanvasObject
	startBtn *widget.Button
	stopBtn  *widget.Button
	applyBtn *widget.Button
	delBtn   *widget.Button

	// Schedule tab
	onStartup, startOn, stopOn, retryOn, openOn, repeatOn *widget.Check
	startAt, stopAt, retries, openPath, repeatMins        *widget.Entry
	mode, finish                                          *widget.RadioGroup
	onceDate                                              *widget.DateEntry
	days                                                  []*widget.Check

	// Files tab
	parallel *widget.Entry
	files    *widget.Table
	rows     []engine.Info
	selRow   int

	// Download limits page
	limOn, limWarn  *widget.Check
	limMB, limHours *widget.Entry
}

const (
	modeOnce  = "Once at"
	modeDaily = "Daily"
)

// The "when done" choices, in the order shown, with the action each runs.
var finishChoices = []struct{ label, action string }{
	{"Do nothing", ""},
	{"Exit ODM", engine.ActionExit},
	{"Sleep", engine.ActionSleep},
	{"Hibernate", engine.ActionHibernate},
	{"Turn off computer", engine.ActionShutdown},
}

func finishLabel(action string) string {
	for _, c := range finishChoices {
		if c.action == action {
			return c.label
		}
	}
	return finishChoices[0].label
}

func finishAction(label string) string {
	for _, c := range finishChoices {
		if c.label == label {
			return c.action
		}
	}
	return ""
}

func (a *application) openScheduler() {
	if a.scheduler != nil {
		a.scheduler.win.Show()
		a.scheduler.win.RequestFocus()
		return
	}
	st := a.mgr.Settings()
	v := &schedulerView{a: a, sched: map[string]engine.QueueSchedule{}, limits: st.Limits, selRow: -1}
	for q, sc := range st.Schedules {
		v.sched[q] = sc
	}
	v.win = a.fyneApp.NewWindow("Scheduler")
	v.build()
	v.reloadQueues()
	v.select_(engine.DefaultQueue)
	v.win.SetCloseIntercept(v.close)
	v.win.Resize(fyne.NewSize(720, 540))
	a.scheduler = v
	v.win.Show()
}

func (v *schedulerView) build() {
	// Left: queues
	v.list = widget.NewList(
		func() int { return len(v.names) },
		func() fyne.CanvasObject {
			return container.NewHBox(widget.NewIcon(theme.ListIcon()), widget.NewLabel(""))
		},
		func(id widget.ListItemID, o fyne.CanvasObject) {
			row := o.(*fyne.Container)
			icon, label := row.Objects[0].(*widget.Icon), row.Objects[1].(*widget.Label)
			if v.names[id] == limitsEntry {
				icon.SetResource(theme.WarningIcon())
				label.SetText("Download limits")
				return
			}
			icon.SetResource(theme.ListIcon())
			label.SetText(v.names[id])
		},
	)
	v.list.OnSelected = func(id widget.ListItemID) {
		if id >= 0 && id < len(v.names) {
			v.select_(v.names[id])
		}
	}
	newBtn := widget.NewButton("New queue", v.newQueue)
	v.delBtn = widget.NewButton("Delete", v.deleteQueue)
	left := container.NewBorder(widget.NewLabelWithStyle("Queues", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		container.NewGridWithColumns(2, newBtn, v.delBtn), nil, nil, framed(v.list))

	v.queueBox = container.NewAppTabs(
		container.NewTabItem("Schedule", v.buildSchedule()),
		container.NewTabItem("Files in the queue", v.buildFiles()),
	)
	v.limitBox = v.buildLimits()
	v.title = widget.NewLabelWithStyle("", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	v.body = container.NewStack(v.queueBox, v.limitBox)
	right := container.NewBorder(v.title, nil, nil, nil, v.body)

	v.startBtn = widget.NewButtonWithIcon("Start now", theme.MediaPlayIcon(), func() {
		v.apply() // start with what is on screen
		v.a.mgr.StartQueue(v.current)
		v.a.refresh()
		v.updateButtons()
	})
	v.stopBtn = widget.NewButtonWithIcon("Stop", theme.MediaStopIcon(), func() {
		v.a.mgr.StopQueue(v.current)
		v.a.refresh()
		v.updateButtons()
	})
	v.applyBtn = widget.NewButton("Apply", func() { v.apply() })
	v.applyBtn.Importance = widget.HighImportance
	closeBtn := widget.NewButton("Close", v.close)
	buttons := container.NewHBox(v.startBtn, v.stopBtn, layout.NewSpacer(), v.applyBtn, closeBtn)

	split := container.NewHSplit(left, container.NewBorder(nil, buttons, nil, nil, right))
	split.Offset = 0.27
	v.win.SetContent(container.New(layout.NewCustomPaddedLayout(8, 10, 10, 10), split))
}

func (v *schedulerView) buildSchedule() fyne.CanvasObject {
	mark := func() {
		if !v.loading {
			v.dirty = true
			v.store()
			v.updateButtons()
		}
	}
	check := func(text string) *widget.Check {
		c := widget.NewCheck(text, nil)
		c.OnChanged = func(bool) { mark(); v.syncEnabled() }
		return c
	}
	entry := func(ph string) *widget.Entry {
		e := widget.NewEntry()
		e.SetPlaceHolder(ph)
		e.OnChanged = func(string) { mark() }
		return e
	}
	v.onStartup = check("Start download on ODM startup")
	v.startOn = check("Start download at")
	v.startAt = entry("HH:MM")
	v.mode = widget.NewRadioGroup([]string{modeOnce, modeDaily}, func(string) { mark(); v.syncEnabled() })
	v.mode.Horizontal = true
	v.onceDate = widget.NewDateEntry()
	v.onceDate.OnChanged = func(*time.Time) { mark() }
	dayGrid := container.NewGridWithColumns(4)
	for _, d := range []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"} {
		c := check(d)
		v.days = append(v.days, c)
		dayGrid.Add(c)
	}
	v.stopOn = check("Stop download at")
	v.stopAt = entry("HH:MM")
	v.retryOn = check("Number of retries for each file if downloading failed:")
	v.retries = entry("10")
	v.openOn = check("Open the following file when done:")
	v.openPath = entry("file to open")
	browse := widget.NewButton("…", func() {
		dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
			if err == nil && r != nil {
				v.openPath.SetText(r.URI().Path())
				_ = r.Close()
			}
		}, v.win).Show()
	})
	var labels []string
	for _, c := range finishChoices {
		labels = append(labels, c.label)
	}
	v.finish = widget.NewRadioGroup(labels, func(string) { mark() })
	v.finish.Horizontal = true
	v.repeatOn = check("Start the queue again every")
	v.repeatMins = entry("60")

	fixed := func(o fyne.CanvasObject, w float32) fyne.CanvasObject {
		return container.NewGridWrap(fyne.NewSize(w, o.MinSize().Height), o)
	}
	return container.NewVScroll(container.NewVBox(
		v.onStartup,
		widget.NewSeparator(),
		container.NewHBox(v.startOn, fixed(v.startAt, 90)),
		container.NewHBox(v.mode, fixed(v.onceDate, 170)),
		container.New(layout.NewCustomPaddedLayout(0, 0, 24, 0), dayGrid),
		container.NewHBox(v.repeatOn, fixed(v.repeatMins, 70), widget.NewLabel("minutes, until it is stopped")),
		widget.NewSeparator(),
		container.NewHBox(v.stopOn, fixed(v.stopAt, 90)),
		container.NewHBox(v.retryOn, fixed(v.retries, 70)),
		widget.NewSeparator(),
		v.openOn,
		container.NewBorder(nil, nil, nil, browse, v.openPath),
		widget.NewLabel("When the queue has finished:"),
		v.finish,
		schedHint("A queue that starts again is never finished, so these do not run for it."),
	))
}

func (v *schedulerView) buildFiles() fyne.CanvasObject {
	v.parallel = widget.NewEntry()
	v.parallel.OnChanged = func(string) {
		if !v.loading {
			v.dirty = true
			v.store()
			v.updateButtons()
		}
	}
	v.files = widget.NewTableWithHeaders(
		func() (int, int) { return len(v.rows), 4 },
		func() fyne.CanvasObject { return newTableCell() },
		func(id widget.TableCellID, o fyne.CanvasObject) {
			c := o.(*tableCell)
			if id.Row < 0 || id.Row >= len(v.rows) {
				c.setText("")
				return
			}
			d := v.rows[id.Row]
			c.setSelected(id.Row == v.selRow)
			switch id.Col {
			case 0:
				c.setText(d.FileName)
			case 1:
				c.setText(humanBytes(d.Total))
			case 2:
				c.setText(statusText(d))
			case 3:
				c.setText(etaText(d))
			}
		},
	)
	v.files.ShowHeaderColumn = false
	v.files.CreateHeader = func() fyne.CanvasObject { return newHeaderLabel(nil) }
	v.files.UpdateHeader = func(id widget.TableCellID, o fyne.CanvasObject) {
		o.(*headerLabel).SetText([]string{"File Name", "Size", "Status", "Time left"}[id.Col])
	}
	v.files.OnSelected = func(id widget.TableCellID) {
		v.selRow = id.Row
		v.files.UnselectAll()
		v.files.Refresh()
	}
	for i, w := range []float32{230, 80, 110, 80} {
		v.files.SetColumnWidth(i, w)
	}
	move := func(delta int) {
		if v.selRow < 0 || v.selRow >= len(v.rows) {
			return
		}
		id := v.rows[v.selRow].ID
		if v.a.mgr.MoveInQueue(id, delta) {
			v.selRow += delta
		}
		v.refreshFiles()
		v.a.refresh()
	}
	remove := func() {
		if v.selRow < 0 || v.selRow >= len(v.rows) || v.current == engine.DefaultQueue {
			return
		}
		// Every download belongs to a queue; leaving one returns it to Main.
		v.a.mgr.MoveToQueue(v.rows[v.selRow].ID, engine.DefaultQueue)
		v.selRow = -1
		v.refreshFiles()
		v.a.refresh()
	}
	tools := container.NewHBox(
		widget.NewButtonWithIcon("", theme.MoveDownIcon(), func() { move(+1) }),
		widget.NewButtonWithIcon("", theme.MoveUpIcon(), func() { move(-1) }),
		widget.NewButtonWithIcon("", theme.DeleteIcon(), remove),
	)
	top := container.NewHBox(widget.NewLabel("Download"),
		container.NewGridWrap(fyne.NewSize(56, v.parallel.MinSize().Height), v.parallel),
		widget.NewLabel("files at the same time"))
	return container.NewBorder(top, tools, nil, nil, framed(v.files))
}

func (v *schedulerView) buildLimits() fyne.CanvasObject {
	mark := func() {
		if !v.loading {
			v.dirty = true
			v.storeLimits()
			v.updateButtons()
		}
	}
	v.limOn = widget.NewCheck("Download limits", func(bool) { mark(); v.syncEnabled() })
	v.limMB = widget.NewEntry()
	v.limMB.OnChanged = func(string) { mark() }
	v.limHours = widget.NewEntry()
	v.limHours.OnChanged = func(string) { mark() }
	v.limWarn = widget.NewCheck("Show a warning when downloads are stopped", func(bool) { mark() })
	box := func(e *widget.Entry) fyne.CanvasObject {
		return container.NewGridWrap(fyne.NewSize(70, e.MinSize().Height), e)
	}
	return container.NewVBox(
		v.limOn,
		container.NewHBox(widget.NewLabel("Download no more than"), box(v.limMB), widget.NewLabel("MBytes")),
		container.NewHBox(widget.NewLabel("every"), box(v.limHours), widget.NewLabel("hours")),
		v.limWarn,
	)
}

// reloadQueues lists the queues known to settings, then Download limits.
func (v *schedulerView) reloadQueues() {
	v.names = append(v.a.mgr.Queues(), limitsEntry)
	for _, q := range v.names[:len(v.names)-1] {
		if _, ok := v.sched[q]; !ok {
			v.sched[q] = engine.NewQueueSchedule()
		}
	}
	v.list.Refresh()
}

func (v *schedulerView) select_(name string) {
	v.current = name
	for i, n := range v.names {
		if n == name {
			v.list.Select(i)
		}
	}
	v.loading = true
	defer func() { v.loading = false; v.syncEnabled(); v.updateButtons() }()
	if name == limitsEntry {
		v.title.SetText("Download limits")
		v.queueBox.Hide()
		v.limitBox.Show()
		v.limOn.SetChecked(v.limits.Enabled)
		v.limMB.SetText(fmt.Sprint(v.limits.MB))
		v.limHours.SetText(fmt.Sprint(v.limits.Hours))
		v.limWarn.SetChecked(!v.limits.NoWarning)
		return
	}
	v.title.SetText(name)
	v.limitBox.Hide()
	v.queueBox.Show()
	sc := v.sched[name]
	v.onStartup.SetChecked(sc.OnStartup)
	v.startOn.SetChecked(sc.StartEnabled)
	v.startAt.SetText(orDefault(sc.StartAt, "23:00"))
	if sc.OnceDate != "" {
		v.mode.SetSelected(modeOnce)
		if d, err := time.ParseInLocation("2006-01-02", sc.OnceDate, time.Local); err == nil {
			v.onceDate.SetDate(&d)
		}
	} else {
		v.mode.SetSelected(modeDaily)
		now := time.Now()
		v.onceDate.SetDate(&now)
	}
	for i, c := range v.days {
		c.SetChecked(i < len(sc.Days) && sc.Days[i])
	}
	v.stopOn.SetChecked(sc.StopEnabled)
	v.stopAt.SetText(orDefault(sc.StopAt, "07:30"))
	v.retryOn.SetChecked(sc.RetriesEnabled)
	v.retries.SetText(fmt.Sprint(sc.Retries))
	v.openOn.SetChecked(sc.OpenWhenDone != "")
	v.openPath.SetText(sc.OpenWhenDone)
	v.finish.SetSelected(finishLabel(sc.FinishAction()))
	v.repeatOn.SetChecked(sc.RepeatMinutes > 0)
	mins := "60"
	if sc.RepeatMinutes > 0 {
		mins = fmt.Sprint(sc.RepeatMinutes)
	}
	v.repeatMins.SetText(mins)
	v.parallel.SetText(fmt.Sprint(sc.Parallel))
	v.selRow = -1
	v.refreshFiles()
}

// schedHint is a small explanatory line under scheduler options.
func schedHint(s string) *widget.Label {
	l := widget.NewLabel(s)
	l.Wrapping = fyne.TextWrapWord
	l.Importance = widget.LowImportance
	return l
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// store copies the Schedule and Files widgets into the edit buffer. Values
// are kept as typed; Apply validates them.
func (v *schedulerView) store() {
	if v.current == "" || v.current == limitsEntry {
		return
	}
	sc := v.sched[v.current]
	sc.OnStartup = v.onStartup.Checked
	sc.StartEnabled = v.startOn.Checked
	sc.StartAt = strings.TrimSpace(v.startAt.Text)
	sc.OnceDate = ""
	if v.mode.Selected == modeOnce && v.onceDate.Date != nil {
		sc.OnceDate = v.onceDate.Date.Format("2006-01-02")
	}
	sc.Days = make([]bool, 7)
	for i, c := range v.days {
		sc.Days[i] = c.Checked
	}
	sc.StopEnabled = v.stopOn.Checked
	sc.StopAt = strings.TrimSpace(v.stopAt.Text)
	sc.RetriesEnabled = v.retryOn.Checked
	fmt.Sscanf(v.retries.Text, "%d", &sc.Retries)
	sc.OpenWhenDone = ""
	if v.openOn.Checked {
		sc.OpenWhenDone = strings.TrimSpace(v.openPath.Text)
	}
	sc.SetFinishAction(finishAction(v.finish.Selected))
	sc.RepeatMinutes = 0
	if v.repeatOn.Checked {
		sc.RepeatMinutes = -1 // apply refuses it unless a number is typed
		fmt.Sscanf(strings.TrimSpace(v.repeatMins.Text), "%d", &sc.RepeatMinutes)
		if sc.RepeatMinutes < 1 {
			sc.RepeatMinutes = -1
		}
	}
	fmt.Sscanf(v.parallel.Text, "%d", &sc.Parallel)
	v.sched[v.current] = sc
}

func (v *schedulerView) storeLimits() {
	v.limits.Enabled = v.limOn.Checked
	fmt.Sscanf(v.limMB.Text, "%d", &v.limits.MB)
	fmt.Sscanf(v.limHours.Text, "%d", &v.limits.Hours)
	v.limits.NoWarning = !v.limWarn.Checked
}

// syncEnabled greys out controls whose checkbox is off, as the reference
// dialog does, so it is clear which values are in effect.
func (v *schedulerView) syncEnabled() {
	set := func(on bool, ws ...fyne.Disableable) {
		for _, w := range ws {
			if on {
				w.Enable()
			} else {
				w.Disable()
			}
		}
	}
	set(v.startOn.Checked, v.startAt, v.mode)
	set(v.startOn.Checked && v.mode.Selected == modeOnce, v.onceDate)
	for _, c := range v.days {
		set(v.startOn.Checked && v.mode.Selected == modeDaily, c)
	}
	set(v.stopOn.Checked, v.stopAt)
	set(v.retryOn.Checked, v.retries)
	set(v.openOn.Checked, v.openPath)
	set(v.repeatOn.Checked, v.repeatMins)
	set(!v.repeatOn.Checked, v.openOn, v.finish)
	set(!v.repeatOn.Checked && v.openOn.Checked, v.openPath)
	set(v.limOn.Checked, v.limMB, v.limHours, v.limWarn)
}

func (v *schedulerView) updateButtons() {
	isQueue := v.current != limitsEntry
	running := isQueue && v.a.mgr.QueueRunning(v.current)
	set := func(b *widget.Button, on bool) {
		if on {
			b.Enable()
		} else {
			b.Disable()
		}
	}
	// A repeating queue waiting for its next start is not running, but Stop
	// is how its repeating ends.
	repeating := isQueue && v.a.mgr.QueueRepeating(v.current)
	set(v.startBtn, isQueue && !running)
	set(v.stopBtn, running || repeating)
	set(v.applyBtn, v.dirty)
	set(v.delBtn, isQueue && v.current != engine.DefaultQueue)
}

// apply validates every queue's schedule and saves; on a bad value it
// switches to the offending queue and explains instead of saving.
func (v *schedulerView) apply() bool {
	v.store()
	if v.current == limitsEntry {
		v.storeLimits()
	}
	for q, sc := range v.sched {
		for _, t := range []struct {
			on   bool
			val  string
			what string
		}{{sc.StartEnabled, sc.StartAt, "Start"}, {sc.StopEnabled, sc.StopAt, "Stop"}} {
			if !t.on {
				continue
			}
			if _, err := time.Parse("15:04", t.val); err != nil {
				v.select_(q)
				dialog.ShowInformation("Scheduler",
					fmt.Sprintf("%s time %q in %q is not a time; use HH:MM, e.g. 23:00.", t.what, t.val, q), v.win)
				return false
			}
		}
		if sc.RepeatMinutes < 0 {
			v.select_(q)
			dialog.ShowInformation("Scheduler",
				fmt.Sprintf("Enter how many minutes %q waits before it starts again (1 or more).", q), v.win)
			return false
		}
	}
	next := v.a.mgr.Settings()
	next.Schedules = map[string]engine.QueueSchedule{}
	for q, sc := range v.sched {
		next.Schedules[q] = sc
	}
	next.Limits = v.limits
	saved := v.a.mgr.UpdateSettings(next)
	for q, sc := range saved.Schedules { // keep the normalised values on screen
		v.sched[q] = sc
	}
	v.limits = saved.Limits
	v.dirty = false
	v.updateButtons()
	return true
}

func (v *schedulerView) close() {
	if !v.dirty {
		v.win.Close()
		v.a.scheduler = nil
		return
	}
	dialog.ShowConfirm("Scheduler", "Apply the changes before closing?", func(yes bool) {
		if yes && !v.apply() {
			return
		}
		v.win.Close()
		v.a.scheduler = nil
	}, v.win)
}

func (v *schedulerView) newQueue() {
	entry := widget.NewEntry()
	entry.SetText(fmt.Sprintf("Queue # %d", len(v.names)))
	dialog.ShowCustomConfirm("Enter queue name", "OK", "Cancel",
		container.NewVBox(widget.NewLabel("Enter a name for the new queue that will be displayed in the list of queues"), entry),
		func(ok bool) {
			name := strings.TrimSpace(entry.Text)
			if !ok || name == "" {
				return
			}
			if !v.a.mgr.AddQueue(name) {
				dialog.ShowInformation("Scheduler", "That queue already exists.", v.win)
				return
			}
			v.reloadQueues()
			v.select_(name)
			v.a.refresh()
		}, v.win)
}

func (v *schedulerView) deleteQueue() {
	name := v.current
	if name == limitsEntry || name == engine.DefaultQueue {
		return
	}
	dialog.ShowConfirm("Delete queue",
		fmt.Sprintf("Delete %q? Its downloads move to %s.", name, engine.DefaultQueue), func(yes bool) {
			if !yes || !v.a.mgr.RemoveQueue(name) {
				return
			}
			delete(v.sched, name)
			v.reloadQueues()
			v.select_(engine.DefaultQueue)
			v.a.refresh()
		}, v.win)
}

// refreshFiles lists the current queue's downloads in run order.
func (v *schedulerView) refreshFiles() {
	if v.current == limitsEntry {
		return
	}
	var rows []engine.Info
	for _, d := range v.a.mgr.List() {
		q := d.Queue
		if q == "" {
			q = engine.DefaultQueue
		}
		if q == v.current {
			rows = append(rows, d)
		}
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Order < rows[j].Order })
	v.rows = rows
	if v.selRow >= len(rows) {
		v.selRow = len(rows) - 1
	}
	v.files.Refresh()
}
