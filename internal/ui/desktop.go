//go:build fyne

// Package ui implements the ODM desktop interface with Fyne: a menu bar, an
// icon toolbar, a category/queue sidebar, a sortable report table and dialogs,
// driving the engine in-process (no web server, no embedded browser).
package ui

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"odm/internal/clipmon"
	"odm/internal/engine"
)

// Available reports whether this build includes the desktop UI.
const Available = true

// Options tunes how the desktop UI starts.
type Options struct {
	// Show receives a value whenever another ODM launch asks this one to
	// bring its window forward.
	Show <-chan struct{}
	// Prompt receives downloads handed over by the browser, each opened in
	// a New Download window for the person to confirm.
	Prompt <-chan engine.AddOptions
	// Hidden starts in the system tray (login autostart) when a tray exists.
	Hidden bool
}

// Run starts the desktop UI and blocks until ODM quits or ctx ends.
func Run(ctx context.Context, mgr *engine.Manager, opts Options) error {
	a := &application{
		mgr:             mgr,
		progressWindows: map[string]*progressView{},
		selectedRow:     -1,
		sortCol:         -1,
	}
	a.fyneApp = app.NewWithID("com.ochi.odm")
	a.fyneApp.Settings().SetTheme(newTheme())
	a.win = a.fyneApp.NewWindow("Ochi Download Manager")
	a.win.Resize(fyne.NewSize(1080, 660))
	a.win.SetIcon(theme.DownloadIcon())

	a.build()
	a.refresh()
	go a.loop()
	// The UI owns the scheduler's completion action while it is open so a
	// shutdown can be cancelled from a countdown instead of happening blind.
	mgr.SetActionHandler(func(act engine.Action) { fyne.Do(func() { a.handleAction(act) }) })
	go func() {
		<-ctx.Done()
		fyne.Do(a.fyneApp.Quit)
	}()
	if !mgr.Settings().NoUpdateCheck {
		// A short delay keeps the request off the start-up path.
		time.AfterFunc(3*time.Second, func() { a.checkForUpdates(false) })
	}
	if !clipmon.Native {
		go clipmon.Poll(ctx, time.Second, a.readClipboard, func() clipmon.Config {
			st := mgr.Settings()
			return clipmon.Config{Enabled: st.Clipboard, FileTypes: st.FileTypes}
		}, func(u string) {
			mgr.AddAuto(engine.AddOptions{URL: u, Start: true})
		})
	}
	if opts.Prompt != nil {
		go func() {
			for o := range opts.Prompt {
				fyne.Do(func() { a.openAddWindow(o) })
			}
		}()
	}
	if opts.Show != nil {
		go func() {
			for range opts.Show {
				fyne.Do(a.showWindow)
			}
		}()
	}
	a.setupTray()
	if opts.Hidden && a.trayOK {
		a.fyneApp.Run() // window stays hidden until the tray or a relaunch shows it
		return nil
	}
	a.win.ShowAndRun()
	return nil
}

// setupTray adds the system-tray icon and menu and makes closing the window
// hide it there, so downloads keep running. The tray menu ends with Exit;
// marked IsQuit so Fyne does not append its own Quit item.
func (a *application) setupTray() {
	desk, ok := a.fyneApp.(desktop.App)
	if !ok {
		return
	}
	a.trayOK = true
	desk.SetSystemTrayMenu(fyne.NewMenu("ODM",
		fyne.NewMenuItem("Show ODM", a.showWindow),
		fyne.NewMenuItem("Add URL…", func() { a.showWindow(); a.openAddDialog() }),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Pause all", func() { a.mgr.PauseAll(); a.refresh() }),
		fyne.NewMenuItem("Resume all", func() { a.mgr.ResumeAll(); a.refresh() }),
		fyne.NewMenuItemSeparator(),
		&fyne.MenuItem{Label: "Exit", IsQuit: true, Action: a.fyneApp.Quit},
	))
	desk.SetSystemTrayIcon(theme.DownloadIcon())
	desk.SetSystemTrayWindow(a.win) // left-click shows the window
	a.win.SetCloseIntercept(func() {
		if a.mgr.Settings().NoCloseToTray {
			a.fyneApp.Quit()
			return
		}
		a.win.Hide()
		if !a.trayHinted {
			a.trayHinted = true
			a.fyneApp.SendNotification(fyne.NewNotification("ODM is still running",
				"Downloads continue in the background. Use the tray icon to open ODM or exit."))
		}
	})
}

// readClipboard returns the clipboard text. Fyne's clipboard must be used on
// the UI thread, hence DoAndWait from the polling goroutine.
func (a *application) readClipboard() string {
	var text string
	fyne.DoAndWait(func() { text = a.fyneApp.Clipboard().Content() })
	return text
}

func (a *application) showWindow() {
	a.win.Show()
	a.win.RequestFocus()
}

// sidebarEntry is one row of the category/queue sidebar.
type sidebarEntry struct {
	text   string
	kind   string
	value  string
	indent int
	icon   fyne.Resource
	header bool
	// node names a collapsible branch ("all", "queues"); empty for leaves.
	node     string
	expanded bool
}

type application struct {
	mgr     *engine.Manager
	fyneApp fyne.App
	win     fyne.Window

	table  *downloadTable
	cats   *widget.List
	split  *container.Split
	status *widget.Label

	mu      sync.Mutex
	items   []engine.Info
	entries []sidebarEntry
	// selected holds the IDs of the selected downloads; focusID is the row
	// the keyboard and single-item actions use, and the anchor for Shift.
	// IDs, not row numbers: the list re-sorts every second, and an index
	// would silently point at a different download.
	selected    map[string]bool
	focusID     string
	selectedRow int // index of focusID in items, kept in step by refresh
	sortCol     int
	sortAsc     bool
	counts      map[string]int
	total       int
	unfinished  int
	finished    int
	queues      []string
	colWidths   []float32
	filter      struct {
		kind  string
		value string
	}
	// search is the filter box; query holds its lower-case words.
	search *searchEntry
	query  []string
	// rowH/headH are the table's real row and header heights, captured from
	// laid-out cells so gridBackdrop can continue the grid below the rows.
	rowH, headH float32
	collapsed   map[string]bool
	// lastStatus remembers each download's previous status so a transition
	// to complete is noticed once; nil until the first refresh, which only
	// records, so restored finished downloads do not all pop a dialog.
	lastStatus map[string]engine.Status
	grid       *gridBackdrop
	scheduler  *schedulerView
	optionsWin fyne.Window
	trayOK     bool // a system tray was set up, so hiding the window is safe
	trayHinted bool
	tools      map[string]*toolButton

	progressWindows map[string]*progressView
}

// downloadTable is the download list. It extends widget.Table (rather than
// wrapping a *Table) so that focus lands on it when a row is clicked, which
// is what delivers Delete / Enter / arrow keys and Ctrl+A here.
type downloadTable struct {
	widget.Table
	app *application
}

func newDownloadTable(a *application) *downloadTable {
	t := &downloadTable{app: a}
	t.Length = func() (int, int) {
		a.mu.Lock()
		defer a.mu.Unlock()
		return len(a.items), columnCount
	}
	t.CreateCell = func() fyne.CanvasObject { return newTableCell() }
	t.UpdateCell = a.updateCell
	t.ShowHeaderRow = true
	t.ExtendBaseWidget(t)
	return t
}

func (t *downloadTable) TypedKey(ev *fyne.KeyEvent) {
	a := t.app
	switch ev.Name {
	case fyne.KeyDelete:
		a.deleteSelected()
	case fyne.KeyReturn, fyne.KeyEnter:
		if d, ok := a.current(); ok {
			a.openProgressWindow(d.ID)
		}
	case fyne.KeyUp, fyne.KeyDown:
		a.mu.Lock()
		row, n := a.selectedRow, len(a.items)
		a.mu.Unlock()
		if ev.Name == fyne.KeyUp {
			row--
		} else {
			row++
		}
		if row >= 0 && row < n {
			a.clickRow(row, currentModifiers()&fyne.KeyModifierShift)
			t.ScrollTo(widget.TableCellID{Row: row, Col: 0})
		}
	default:
		t.Table.TypedKey(ev)
	}
}

// TypedShortcut handles Ctrl+A here and passes the rest to the window, since
// a focused widget receives shortcuts before the canvas does.
func (t *downloadTable) TypedShortcut(s fyne.Shortcut) {
	if _, ok := s.(*fyne.ShortcutSelectAll); ok {
		t.app.selectAll()
		return
	}
	t.app.handleShortcut(s)
}

func (t *downloadTable) Resize(size fyne.Size) {
	t.Table.Resize(size)
	t.app.layoutColumns(size.Width)
}

func (t *downloadTable) DoubleTapped(*fyne.PointEvent) {
	if d, ok := t.app.current(); ok {
		t.app.openProgressWindow(d.ID)
	}
}

func (t *downloadTable) TappedSecondary(e *fyne.PointEvent) {
	t.app.showRowMenu(e.AbsolutePosition)
}

var columnTitles = []string{"File Name", "Q", "Size", "Status", "Time left", "Transfer rate", "Last attempt", "Description"}

const (
	colName = iota
	colQueue
	colSize
	colStatus
	colETA
	colRate
	colLastTry
	colDescription
	columnCount
)

// fixed column widths; the name and description columns share the slack.
var columnFixed = map[int]float32{
	colQueue:   44,
	colSize:    80,
	colStatus:  112,
	colETA:     78,
	colRate:    100,
	colLastTry: 92,
}

func (a *application) build() {
	tint := func(icon fyne.Resource, c fyne.ThemeColorName) fyne.Resource {
		return theme.NewColoredResource(icon, c)
	}
	a.tools = map[string]*toolButton{}
	tool := func(key string, icon fyne.Resource, c fyne.ThemeColorName, text string, fn func()) *toolButton {
		b := newToolButton(tint(icon, c), text, fn)
		a.tools[key] = b
		return b
	}
	sep := func() fyne.CanvasObject {
		return container.New(layout.NewCustomPaddedLayout(8, 8, 4, 4), widget.NewSeparator())
	}
	toolbar := container.NewHBox(
		tool("add", theme.ContentAddIcon(), colorToolAdd, "Add URL", a.openAddDialog),
		tool("resume", theme.DownloadIcon(), colorToolRun, "Resume", a.resumeSelected),
		tool("stop", theme.MediaStopIcon(), colorToolStop, "Stop", a.stopSelected),
		tool("stopall", theme.MediaPauseIcon(), colorToolStop, "Stop All", func() { a.mgr.StopAll(); a.refresh() }),
		tool("delete", theme.DeleteIcon(), colorToolDelete, "Delete", a.deleteSelected),
		tool("cleardone", theme.ContentClearIcon(), colorToolDelete, "Clear Done", func() { a.mgr.ClearCompleted(); a.refresh() }),
		sep(),
		tool("options", theme.SettingsIcon(), colorToolConfig, "Options", func() { a.openOptionsTab(0) }),
		tool("scheduler", theme.HistoryIcon(), colorToolSched, "Scheduler", a.openScheduler),
		tool("startq", theme.MediaPlayIcon(), colorToolQueue, "Start Queue", a.startQueue),
		tool("stopq", theme.MediaStopIcon(), colorToolQueue, "Stop Queue", a.stopQueue),
		sep(),
		tool("progress", theme.InfoIcon(), colorToolRun, "Progress", func() {
			if d, ok := a.current(); ok {
				a.openProgressWindow(d.ID)
			}
		}),
		tool("folder", theme.FolderOpenIcon(), colorToolSched, "Open folder", func() {
			if d, ok := a.current(); ok {
				revealPath(d.FilePath)
			}
		}),
	)
	// A horizontal scroller lets the window shrink below the toolbar's full
	// width (the HBox MinSize would otherwise set the window minimum).
	toolScroll := container.NewHScroll(container.New(layout.NewCustomPaddedLayout(2, 2, 4, 4), toolbar))
	toolScroll.SetMinSize(fyne.NewSize(200, toolbar.MinSize().Height+4))
	a.search = newSearchEntry(a.setSearch, func() { a.win.Canvas().Focus(a.table) })
	searchBox := container.NewCenter(container.NewGridWrap(fyne.NewSize(240, a.search.MinSize().Height), a.search))
	top := container.NewVBox(
		container.NewBorder(nil, nil, nil, container.New(layout.NewCustomPaddedLayout(0, 0, 4, 8), searchBox), toolScroll),
		widget.NewSeparator(),
	)

	base := newDownloadTable(a)
	base.OnSelected = func(id widget.TableCellID) {
		// Drop Fyne's single-cell highlight; updateCell paints whole rows.
		base.UnselectAll()
		if id.Row >= 0 {
			a.clickRow(id.Row, currentModifiers())
		}
	}
	base.CreateHeader = func() fyne.CanvasObject { return newHeaderLabel(a) }
	base.UpdateHeader = func(id widget.TableCellID, o fyne.CanvasObject) {
		h := o.(*headerLabel)
		h.col = id.Col
		h.SetText(a.headerText(id.Col))
		if hh := h.Size().Height; hh > 0 {
			a.mu.Lock()
			a.headH = hh
			a.mu.Unlock()
		}
	}
	a.table = base
	a.layoutColumns(760)
	a.grid = newGridBackdrop(a)
	list := framed(container.NewStack(a.grid, a.table))
	// The category tree uses tighter padding; the table's rows are already
	// compact because its cells draw their own text at a fixed height.
	compact := compactTheme{Theme: newTheme()}

	a.collapsed = map[string]bool{}
	a.cats = widget.NewList(
		func() int {
			a.mu.Lock()
			defer a.mu.Unlock()
			return len(a.entries)
		},
		func() fyne.CanvasObject {
			arrow := newTapIcon()
			icon := widget.NewIcon(theme.FolderIcon())
			label := widget.NewLabel("")
			label.Truncation = fyne.TextTruncateEllipsis
			left := container.NewHBox(newIndentSpacer(), arrow, icon)
			return container.NewBorder(nil, nil, left, nil, label)
		},
		a.updateSidebarRow,
	)
	a.cats.HideSeparators = true
	a.cats.OnSelected = func(id widget.ListItemID) {
		a.mu.Lock()
		if id < 0 || id >= len(a.entries) {
			a.mu.Unlock()
			return
		}
		e := a.entries[id]
		a.mu.Unlock()
		if e.header {
			return
		}
		a.applyFilter(e.kind, e.value)
	}

	hideBtn := widget.NewButtonWithIcon("", theme.CancelIcon(), a.toggleSidebar)
	hideBtn.Importance = widget.LowImportance
	sideTitle := widget.NewLabel("Categories")
	sideHeader := container.NewBorder(nil, widget.NewSeparator(), sideTitle, hideBtn)
	sidebar := framed(container.NewBorder(sideHeader, nil, nil, nil,
		container.NewThemeOverride(a.cats, compact)))

	a.split = container.NewHSplit(sidebar, list)
	a.split.Offset = 0.2

	a.status = widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{})
	body := container.New(layout.NewCustomPaddedLayout(4, 0, 4, 4), a.split)
	a.win.SetContent(container.NewBorder(top, a.status, nil, nil, body))
	a.win.SetMainMenu(a.buildMenu())
	a.win.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyN, Modifier: fyne.KeyModifierShortcutDefault}, a.handleShortcut)
	a.win.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyF, Modifier: fyne.KeyModifierShortcutDefault}, a.handleShortcut)
	// Keys typed with nothing focused (e.g. right after start) still reach
	// the list's handler.
	a.win.Canvas().SetOnTypedKey(a.table.TypedKey)
}

// updateSidebarRow fills one row of the category tree.
func (a *application) updateSidebarRow(id widget.ListItemID, o fyne.CanvasObject) {
	a.mu.Lock()
	if id < 0 || id >= len(a.entries) {
		a.mu.Unlock()
		return
	}
	e := a.entries[id]
	a.mu.Unlock()
	row := o.(*fyne.Container)
	label := row.Objects[0].(*widget.Label)
	left := row.Objects[1].(*fyne.Container)
	spacer := left.Objects[0].(*indentSpacer)
	arrow := left.Objects[1].(*tapIcon)
	icon := left.Objects[2].(*widget.Icon)

	spacer.set(float32(e.indent * 16))
	// Leaves keep an empty arrow slot so names stay aligned with branches.
	arrow.onTap = nil
	arrow.SetResource(nil)
	if e.node != "" {
		node := e.node
		arrow.onTap = func() { a.toggleNode(node) }
		if e.expanded {
			arrow.SetResource(theme.MenuDropDownIcon())
		} else {
			arrow.SetResource(theme.MenuExpandIcon())
		}
	}
	// The HBox only re-reads the spacer's MinSize when laid out again;
	// without this, recycled rows kept whatever indent they had before.
	left.Refresh()
	row.Refresh()
	icon.SetResource(e.icon)
	label.SetText(e.text)
}

func (a *application) toggleNode(node string) {
	a.mu.Lock()
	a.collapsed[node] = !a.collapsed[node]
	a.mu.Unlock()
	a.refresh()
}

// updateToolbar greys out actions that do not apply to the selection.
func (a *application) updateToolbar() {
	if a.tools == nil {
		return
	}
	_, focused := a.current()
	sel := a.selectedItems()
	a.mu.Lock()
	all := a.items
	a.mu.Unlock()
	anyActive, anyDone := false, false
	for _, it := range all {
		switch it.Status {
		case engine.StatusDownloading, engine.StatusQueued:
			anyActive = true
		case engine.StatusComplete:
			anyDone = true
		}
	}
	canResume, canStop := false, false
	for _, d := range sel {
		switch d.Status {
		case engine.StatusDownloading, engine.StatusQueued:
			canStop = true
		case engine.StatusComplete:
		default:
			canResume = true
		}
	}
	a.tools["resume"].setEnabled(canResume)
	a.tools["stop"].setEnabled(canStop)
	a.tools["stopall"].setEnabled(anyActive)
	a.tools["delete"].setEnabled(len(sel) > 0)
	a.tools["cleardone"].setEnabled(anyDone)
	a.tools["progress"].setEnabled(focused)
	a.tools["folder"].setEnabled(focused)
}

// layoutColumns sizes the table columns so they fill the available width.
func (a *application) layoutColumns(total float32) {
	if total <= 0 {
		return
	}
	var fixedSum float32
	for _, w := range columnFixed {
		fixedSum += w
	}
	// widget.Table spaces columns by its theme's Padding (not the separator
	// thickness); leaving that gap out of the sum made the row wider than the
	// view, which showed a horizontal scrollbar and clipped Description. The
	// last 2px absorb float rounding.
	gap := theme.Padding()
	if a.table != nil {
		gap = a.table.Theme().Size(theme.SizeNamePadding)
	}
	total -= float32(columnCount)*gap + 2
	slack := total - fixedSum
	// Below these minimums the table scrolls horizontally rather than
	// squeezing names into unreadable slivers.
	const minName, minDesc = 180, 110
	nameW, descW := float32(minName), float32(minDesc)
	if slack > minName+minDesc {
		nameW = slack * 0.6
		descW = slack - nameW
	}
	widths := make([]float32, columnCount)
	for col, w := range columnFixed {
		widths[col] = w
	}
	widths[colName] = nameW
	widths[colDescription] = descW

	a.mu.Lock()
	if len(a.colWidths) == len(widths) {
		same := true
		for i := range widths {
			if a.colWidths[i] != widths[i] {
				same = false
				break
			}
		}
		if same {
			a.mu.Unlock()
			return
		}
	}
	a.colWidths = widths
	a.mu.Unlock()

	for i, w := range widths {
		a.table.SetColumnWidth(i, w)
	}
}

/* ---------- menu bar ---------- */

func (a *application) buildMenu() *fyne.MainMenu {
	tasks := fyne.NewMenu("Tasks",
		fyne.NewMenuItem("Add new download…", a.openAddDialog),
		fyne.NewMenuItem("Grab site…", a.openGrabberDialog),
		&fyne.MenuItem{IsSeparator: true},
		fyne.NewMenuItem("Export download list…", a.exportList),
		fyne.NewMenuItem("Import download list…", a.importList),
		&fyne.MenuItem{IsSeparator: true},
		fyne.NewMenuItem("Exit", func() { a.fyneApp.Quit() }),
	)
	downloads := fyne.NewMenu("Downloads",
		fyne.NewMenuItem("Pause all", func() { a.mgr.PauseAll(); a.refresh() }),
		fyne.NewMenuItem("Stop all (and stop queues)", func() { a.mgr.StopAll(); a.refresh() }),
		fyne.NewMenuItem("Clear completed", func() { a.mgr.ClearCompleted(); a.refresh() }),
		&fyne.MenuItem{IsSeparator: true},
		fyne.NewMenuItem("Start queue", a.startQueue),
		fyne.NewMenuItem("Stop queue", a.stopQueue),
		fyne.NewMenuItem("New queue…", a.openNewQueueDialog),
		fyne.NewMenuItem("Scheduler…", a.openScheduler),
		&fyne.MenuItem{IsSeparator: true},
		fyne.NewMenuItem("Options…", func() { a.openOptionsTab(0) }),
	)
	view := fyne.NewMenu("View",
		fyne.NewMenuItem("Hide categories", a.toggleSidebar),
		fyne.NewMenuItem("Sort by name", func() { a.sortBy(colName) }),
		fyne.NewMenuItem("Sort by size", func() { a.sortBy(colSize) }),
		fyne.NewMenuItem("Sort by status", func() { a.sortBy(colStatus) }),
		fyne.NewMenuItem("Sort by time left", func() { a.sortBy(colETA) }),
		fyne.NewMenuItem("Sort by transfer rate", func() { a.sortBy(colRate) }),
		fyne.NewMenuItem("Sort by description", func() { a.sortBy(colDescription) }),
	)
	help := fyne.NewMenu("Help",
		fyne.NewMenuItem("Check for updates…", func() { a.checkForUpdates(true) }),
		fyne.NewMenuItem("About ODM", a.showAbout),
	)
	return fyne.NewMainMenu(tasks, downloads, view, help)
}

func (a *application) showAbout() {
	dialog.ShowInformation("About",
		"Ochi Download Manager (ODM)\n\nA segmented, resumable download manager.\nBuilt with Go and Fyne.",
		a.win)
}

func (a *application) exportList() {
	data, err := a.mgr.Export()
	if err != nil {
		dialog.ShowError(err, a.win)
		return
	}
	d := dialog.NewFileSave(func(w fyne.URIWriteCloser, err error) {
		if err != nil || w == nil {
			return
		}
		defer w.Close()
		if _, err := w.Write(data); err != nil {
			dialog.ShowError(err, a.win)
		}
	}, a.win)
	d.SetFileName("odm-downloads.json")
	d.Show()
}

func (a *application) importList() {
	d := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
		if err != nil || r == nil {
			return
		}
		defer r.Close()
		data, err := io.ReadAll(r)
		if err != nil {
			dialog.ShowError(err, a.win)
			return
		}
		n, err := a.mgr.Import(data)
		if err != nil {
			dialog.ShowError(err, a.win)
			return
		}
		a.refresh()
		dialog.ShowInformation("Import", fmt.Sprintf("Imported %d download(s).", n), a.win)
	}, a.win)
	d.Show()
}

/* ---------- sidebar ---------- */

func (a *application) toggleSidebar() {
	if a.split.Offset > 0.001 {
		a.split.Offset = 0
	} else {
		a.split.Offset = 0.21
	}
	a.split.Refresh()
}

func (a *application) buildEntries(all []engine.Info) []sidebarEntry {
	counts := map[string]int{}
	unfinished, finished := 0, 0
	for _, d := range all {
		counts[d.Category]++
		if d.Status == engine.StatusComplete {
			finished++
		} else {
			unfinished++
		}
	}
	// Counts only where there is something to count, so the tree reads as
	// a list of places rather than a column of zeros.
	label := func(name string, n int) string {
		if n > 0 {
			return fmt.Sprintf("%s (%d)", name, n)
		}
		return name
	}
	a.mu.Lock()
	queues := append([]string(nil), a.queues...)
	allOpen, queuesOpen := !a.collapsed["all"], !a.collapsed["queues"]
	a.mu.Unlock()

	folder := theme.FolderIcon()
	out := []sidebarEntry{
		{text: label("All Downloads", len(all)), kind: "all", icon: theme.FolderOpenIcon(), node: "all", expanded: allOpen},
	}
	if allOpen {
		for _, c := range engine.Categories() {
			out = append(out, sidebarEntry{
				text: label(c, counts[c]), kind: "category", value: c, indent: 1, icon: iconForCategory(c),
			})
		}
	}
	out = append(out,
		sidebarEntry{text: label("Unfinished", unfinished), kind: "unfinished", icon: theme.DownloadIcon()},
		sidebarEntry{text: label("Finished", finished), kind: "finished", icon: theme.ConfirmIcon()},
	)

	// Downloads can carry a queue that is not in settings (imported lists,
	// older state); list those too or their items are unreachable here.
	perQueue := map[string]int{}
	known := map[string]bool{}
	for _, q := range queues {
		known[strings.ToLower(q)] = true
	}
	for _, d := range all {
		q := d.Queue
		if q == "" {
			q = engine.DefaultQueue
		}
		perQueue[strings.ToLower(q)]++
		if !known[strings.ToLower(q)] {
			known[strings.ToLower(q)] = true
			queues = append(queues, q)
		}
	}
	out = append(out, sidebarEntry{text: "Queues", icon: theme.ListIcon(), header: true, node: "queues", expanded: queuesOpen})
	if queuesOpen {
		for _, q := range queues {
			out = append(out, sidebarEntry{
				text: label(q, perQueue[strings.ToLower(q)]), kind: "queue", value: q, indent: 1, icon: folder,
			})
		}
	}
	return out
}

func (a *application) applyFilter(kind, value string) {
	a.mu.Lock()
	a.filter.kind, a.filter.value = kind, value
	a.mu.Unlock()
	a.refresh()
}

func (a *application) currentQueueFilter() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.filter.kind == "queue" {
		return a.filter.value
	}
	return "*"
}

/* ---------- queues ---------- */

func (a *application) startQueue() {
	a.mgr.StartQueue(a.currentQueueFilter())
	a.refresh()
}

func (a *application) stopQueue() {
	a.mgr.StopQueue(a.currentQueueFilter())
	a.refresh()
}

func (a *application) openNewQueueDialog() {
	entry := widget.NewEntry()
	entry.SetPlaceHolder("Queue name")
	d := dialog.NewCustomConfirm("New queue", "Create", "Cancel",
		container.NewVBox(widget.NewLabel("Enter a name for the new queue:"), entry),
		func(ok bool) {
			if !ok || strings.TrimSpace(entry.Text) == "" {
				return
			}
			if !a.mgr.AddQueue(entry.Text) {
				dialog.ShowInformation("Queue", "That queue already exists.", a.win)
			}
			a.refresh()
		}, a.win)
	d.Show()
}

/* ---------- table ---------- */

func (a *application) updateCell(id widget.TableCellID, o fyne.CanvasObject) {
	c, ok := o.(*tableCell)
	if !ok {
		return
	}

	a.mu.Lock()
	if id.Row < 0 || id.Row >= len(a.items) {
		a.mu.Unlock()
		c.setText("")
		c.showIcon(false)
		c.setSelected(false)
		return
	}
	d := a.items[id.Row]
	a.mu.Unlock()

	if id.Col == colName {
		c.setIcon(iconForCategory(d.Category))
	}
	c.showIcon(id.Col == colName)
	a.mu.Lock()
	selected := a.selected[d.ID]
	a.rowH = rowHeight()
	a.mu.Unlock()
	c.setSelected(selected)

	switch id.Col {
	case colName:
		c.setText(d.FileName)
	case colQueue:
		c.setText(d.Queue)
	case colSize:
		c.setText(humanBytes(d.Total))
	case colStatus:
		c.setText(statusText(d))
	case colETA:
		if d.Live {
			c.setText("") // a recording has no end to count down to
		} else {
			c.setText(etaText(d))
		}
	case colRate:
		c.setText(rateText(d))
	case colLastTry:
		c.setText(d.UpdatedAt.Format("Jan 2 15:04"))
	case colDescription:
		c.setText(d.Description)
	}
}

// statusText is the Status column: the percentage while transferring, and the
// state plus how far it got otherwise, so a stopped download still shows its
// progress now that the list has no progress bar.
func statusText(d engine.Info) string {
	pct := ""
	if d.Total > 0 && d.Progress > 0 && !d.Live {
		pct = fmt.Sprintf(" · %.0f%%", d.Progress)
	}
	switch d.Status {
	case engine.StatusDownloading:
		if d.Live {
			// Total follows Done while recording: a percentage would
			// read 100% throughout.
			return "Recording"
		}
		if d.Total > 0 {
			return fmt.Sprintf("%.2f%%", d.Progress)
		}
		return "Downloading"
	case engine.StatusQueued:
		return "Queued" + pct
	case engine.StatusPaused:
		return "Paused" + pct
	case engine.StatusComplete:
		return "Complete"
	case engine.StatusError:
		if d.Error != "" {
			return "Error: " + d.Error
		}
		return "Error" + pct
	}
	return string(d.Status)
}

// iconForCategory picks a small file-type icon for a category folder.
func iconForCategory(cat string) fyne.Resource {
	switch cat {
	case "Video":
		return theme.MediaVideoIcon()
	case "Audio":
		return theme.MediaMusicIcon()
	case "Documents":
		return theme.DocumentIcon()
	case "Programs":
		return theme.ComputerIcon()
	case "Compressed":
		// Fyne has no archive glyph; stacked pages reads as "bundle of files".
		return theme.ContentCopyIcon()
	case "Disk Images":
		return theme.StorageIcon()
	case "Images":
		return theme.MediaPhotoIcon()
	default:
		return theme.FileIcon()
	}
}

func (a *application) headerText(col int) string {
	if col < 0 || col >= len(columnTitles) {
		return ""
	}
	a.mu.Lock()
	sc, asc := a.sortCol, a.sortAsc
	a.mu.Unlock()
	if sc == col {
		if asc {
			return columnTitles[col] + " ▲"
		}
		return columnTitles[col] + " ▼"
	}
	return columnTitles[col]
}

func (a *application) sortBy(col int) {
	a.mu.Lock()
	if a.sortCol == col {
		a.sortAsc = !a.sortAsc
	} else {
		a.sortCol = col
		a.sortAsc = true
	}
	a.mu.Unlock()
	a.refresh()
}

/* ---------- refresh ---------- */

func (a *application) loop() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for range ticker.C {
		fyne.Do(a.refresh)
	}
}

func (a *application) refresh() {
	a.mu.Lock()
	col, asc := a.sortCol, a.sortAsc
	kind, value := a.filter.kind, a.filter.value
	terms := a.query
	a.mu.Unlock()

	all := a.mgr.List()
	queues := a.mgr.Queues()
	filtered := make([]engine.Info, 0, len(all))
	counts := map[string]int{}
	unfinished, finished := 0, 0
	for _, d := range all {
		counts[d.Category]++
		if d.Status == engine.StatusComplete {
			finished++
		} else {
			unfinished++
		}
		if matchesFilter(d, kind, value) && matchesSearch(d, terms) {
			filtered = append(filtered, d)
		}
	}
	if col >= 0 {
		sortSlice(filtered, col, asc)
	} else if kind == "queue" {
		// A queue view shows the run order, so Move up/down is visible.
		sort.SliceStable(filtered, func(i, j int) bool { return filtered[i].Order < filtered[j].Order })
	}

	a.mu.Lock()
	a.items = filtered
	present := make(map[string]bool, len(filtered))
	a.selectedRow = -1
	for i, d := range filtered {
		present[d.ID] = true
		if d.ID == a.focusID {
			a.selectedRow = i
		}
	}
	for id := range a.selected {
		if !present[id] { // removed, or hidden by the filter
			delete(a.selected, id)
		}
	}
	if a.selectedRow < 0 {
		a.focusID = ""
	}
	a.counts = counts
	a.queues = queues
	a.total = len(all)
	a.unfinished = unfinished
	a.finished = finished
	a.mu.Unlock()

	entries := a.buildEntries(all)

	a.mu.Lock()
	a.entries = entries
	a.mu.Unlock()

	if a.table != nil {
		a.table.Refresh()
	}
	if a.cats != nil {
		a.cats.Refresh()
	}
	if a.grid != nil {
		a.grid.Refresh()
	}
	a.updateToolbar()

	active, speed := 0, int64(0)
	for _, d := range all {
		if d.Status == engine.StatusDownloading {
			active++
			speed += d.Speed
		}
	}
	if a.status != nil {
		items := fmt.Sprintf("%d items", len(all))
		if len(all) == 1 {
			items = "1 item"
		}
		if len(terms) > 0 {
			items = fmt.Sprintf("%d of %s match the search", len(filtered), items)
		}
		if active > 0 {
			a.status.SetText(fmt.Sprintf("  %d active · %s/s · %s", active, humanBytes(speed), items))
		} else {
			a.status.SetText("  " + items)
		}
	}

	a.noticeCompletions(all)

	for id, v := range a.progressWindows {
		if v.win == nil {
			continue
		}
		a.updateProgressWindow(id)
	}
	if a.scheduler != nil {
		a.scheduler.refreshFiles()
		a.scheduler.updateButtons()
	}
}

// noticeCompletions closes the progress window of each download that just
// finished and shows the completion dialog in its place.
func (a *application) noticeCompletions(all []engine.Info) {
	first := a.lastStatus == nil
	prev := a.lastStatus
	a.lastStatus = make(map[string]engine.Status, len(all))
	show := !a.mgr.Settings().NoCompleteDialog
	for _, d := range all {
		a.lastStatus[d.ID] = d.Status
		if first || d.Status != engine.StatusComplete {
			continue
		}
		if was, seen := prev[d.ID]; seen && was != engine.StatusComplete {
			if v, ok := a.progressWindows[d.ID]; ok {
				v.win.Close()
			}
			if show {
				a.showComplete(d)
			}
		}
	}
}

func sortSlice(items []engine.Info, col int, asc bool) {
	less := func(i, j int) bool { return compareInfo(items[i], items[j], col) }
	sort.SliceStable(items, func(i, j int) bool {
		if asc {
			return less(i, j)
		}
		return less(j, i)
	})
}

func compareInfo(x, y engine.Info, col int) bool {
	switch col {
	case colQueue:
		return strings.ToLower(x.Queue) < strings.ToLower(y.Queue)
	case colName:
		return strings.ToLower(x.FileName) < strings.ToLower(y.FileName)
	case colSize:
		return x.Total < y.Total
	case colStatus:
		return x.Status < y.Status
	case colETA:
		return etaSeconds(x) < etaSeconds(y)
	case colRate:
		return x.Speed < y.Speed
	case colLastTry:
		return x.UpdatedAt.Before(y.UpdatedAt)
	case colDescription:
		return strings.ToLower(x.Description) < strings.ToLower(y.Description)
	}
	return false
}

func matchesFilter(d engine.Info, kind, value string) bool {
	switch kind {
	case "unfinished":
		return d.Status != engine.StatusComplete
	case "finished":
		return d.Status == engine.StatusComplete
	case "category":
		return d.Category == value
	case "queue":
		q := d.Queue
		if q == "" {
			q = engine.DefaultQueue
		}
		return strings.EqualFold(q, value)
	default:
		return true
	}
}

/* ---------- selection and actions ---------- */

func (a *application) current() (*engine.Info, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	idx := a.selectedRow
	if idx < 0 || idx >= len(a.items) {
		return nil, false
	}
	cp := a.items[idx]
	return &cp, true
}

func (a *application) byID(id string) (*engine.Info, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for i := range a.items {
		if a.items[i].ID == id {
			cp := a.items[i]
			return &cp, true
		}
	}
	return nil, false
}

func (a *application) showRowMenu(pos fyne.Position) {
	d, ok := a.current()
	if !ok {
		return
	}
	sel := a.selectedItems()
	items := []*fyne.MenuItem{fyne.NewMenuItem("Show progress", func() { a.openProgressWindow(d.ID) })}
	if d.Status == engine.StatusComplete {
		items = append(items, fyne.NewMenuItem("Open file", func() { openPath(d.FilePath) }))
	}
	refresh := fyne.NewMenuItem("Refresh download address…", func() { a.refreshAddress(*d) })
	// Only for unfinished, stopped downloads: the engine refuses others.
	refresh.Disabled = d.Status == engine.StatusComplete || d.Status == engine.StatusDownloading || d.Status == engine.StatusQueued
	items = append(items,
		fyne.NewMenuItem("Open folder", func() { revealPath(d.FilePath) }),
		refresh,
		fyne.NewMenuItem("Pause / Resume", func() {
			// The clicked row decides the direction for the whole selection.
			if d.Status == engine.StatusDownloading || d.Status == engine.StatusQueued {
				a.stopSelected()
				return
			}
			a.resumeSelected()
		}),
	)
	move := fyne.NewMenuItem("Move to queue", nil)
	move.ChildMenu = a.queueSubmenu(sel)
	items = append(items, move,
		fyne.NewMenuItem("Move up in queue", func() { a.mgr.MoveInQueue(d.ID, -1); a.refresh() }),
		fyne.NewMenuItem("Move down in queue", func() { a.mgr.MoveInQueue(d.ID, +1); a.refresh() }),
	)
	items = append(items,
		fyne.NewMenuItem("Remove from list", func() { a.removeSelected(false) }),
		fyne.NewMenuItem("Remove + delete file", func() { a.removeSelected(true) }),
	)
	pop := widget.NewPopUpMenu(fyne.NewMenu("", items...), a.win.Canvas())
	pop.ShowAtPosition(pos)
}

func (a *application) queueSubmenu(sel []engine.Info) *fyne.Menu {
	out := []*fyne.MenuItem{}
	for _, q := range a.mgr.Queues() {
		q := q
		out = append(out, fyne.NewMenuItem(q, func() {
			for _, d := range sel {
				a.mgr.MoveToQueue(d.ID, q)
			}
			a.refresh()
		}))
	}
	if len(out) == 0 {
		out = append(out, &fyne.MenuItem{Label: "(no queues)", Disabled: true})
	}
	return fyne.NewMenu("", out...)
}

/* ---------- selection ---------- */

// currentModifiers reports the keyboard modifiers held right now; the table's
// selection callback carries no event, so Ctrl/Shift-click is read here.
func currentModifiers() fyne.KeyModifier {
	if drv, ok := fyne.CurrentApp().Driver().(desktop.Driver); ok {
		return drv.CurrentKeyModifiers()
	}
	return 0
}

// clickRow applies a click on row: plain selects only it, Ctrl (Cmd on macOS)
// toggles it, Shift selects the range from the focus row.
func (a *application) clickRow(row int, mods fyne.KeyModifier) {
	a.mu.Lock()
	if row < 0 || row >= len(a.items) {
		a.mu.Unlock()
		return
	}
	if a.selected == nil {
		a.selected = map[string]bool{}
	}
	id := a.items[row].ID
	switch {
	case mods&fyne.KeyModifierShift != 0 && a.selectedRow >= 0:
		lo, hi := a.selectedRow, row
		if lo > hi {
			lo, hi = hi, lo
		}
		a.selected = map[string]bool{}
		for i := lo; i <= hi; i++ {
			a.selected[a.items[i].ID] = true
		}
		// The anchor stays put so further Shift-clicks extend from it.
	case mods&fyne.KeyModifierShortcutDefault != 0:
		if a.selected[id] {
			delete(a.selected, id)
		} else {
			a.selected[id] = true
		}
		a.focusID, a.selectedRow = id, row
	default:
		a.selected = map[string]bool{id: true}
		a.focusID, a.selectedRow = id, row
	}
	a.mu.Unlock()
	a.table.Refresh()
	a.updateToolbar()
}

func (a *application) selectAll() {
	a.mu.Lock()
	a.selected = map[string]bool{}
	for _, d := range a.items {
		a.selected[d.ID] = true
	}
	if a.selectedRow < 0 && len(a.items) > 0 {
		a.focusID, a.selectedRow = a.items[0].ID, 0
	}
	a.mu.Unlock()
	a.table.Refresh()
	a.updateToolbar()
}

// selectedItems returns the selected downloads in list order.
func (a *application) selectedItems() []engine.Info {
	a.mu.Lock()
	defer a.mu.Unlock()
	var out []engine.Info
	for _, d := range a.items {
		if a.selected[d.ID] {
			out = append(out, d)
		}
	}
	return out
}

// handleShortcut is the window-wide keyboard shortcut handler.
func (a *application) handleShortcut(s fyne.Shortcut) {
	cs, ok := s.(*desktop.CustomShortcut)
	if !ok || cs.Modifier != fyne.KeyModifierShortcutDefault {
		return
	}
	switch cs.KeyName {
	case fyne.KeyN:
		a.openAddDialog()
	case fyne.KeyF:
		a.focusSearch()
	}
}

/* ---------- actions on the selection ---------- */

func (a *application) resumeSelected() {
	sel := a.selectedItems()
	for _, d := range sel {
		a.mgr.Resume(d.ID)
	}
	a.refresh()
	if len(sel) == 1 { // one window, not one per file
		a.autoProgress(sel[0].ID)
	}
}

func (a *application) stopSelected() {
	for _, d := range a.selectedItems() {
		a.mgr.Pause(d.ID)
	}
	a.refresh()
}

func (a *application) deleteSelected() { a.removeSelected(false) }

func (a *application) removeSelected(withFile bool) {
	sel := a.selectedItems()
	if len(sel) == 0 {
		return
	}
	what := fmt.Sprintf("%q", sel[0].FileName)
	if len(sel) > 1 {
		what = fmt.Sprintf("%d downloads", len(sel))
	}
	msg := fmt.Sprintf("Remove %s from the list? (files are kept)", what)
	if withFile {
		msg = fmt.Sprintf("Remove %s from the list and delete the files from disk?", what)
	}
	dialogConfirm(a.win, "Delete", msg, func(yes bool) {
		if !yes {
			return
		}
		for _, d := range sel {
			a.mgr.RemoveWithFile(d.ID, withFile)
		}
		a.refresh()
	})
}

/* ---------- formatting ---------- */

func humanBytes(n int64) string {
	if n <= 0 {
		return ""
	}
	units := []string{"B", "KB", "MB", "GB", "TB"}
	v := float64(n)
	i := 0
	// Step up at 1000 rather than 1024 so a 1000 MiB file reads 0.98 GB
	// instead of a four-digit "1000.0 MB"; the divisor stays binary.
	for v >= 1000 && i < len(units)-1 {
		v /= 1024
		i++
	}
	switch {
	case i == 0:
		return fmt.Sprintf("%.0f %s", v, units[i])
	case v < 10:
		return fmt.Sprintf("%.2f %s", v, units[i])
	default:
		return fmt.Sprintf("%.1f %s", v, units[i])
	}
}

func rateText(d engine.Info) string {
	if d.Status != engine.StatusDownloading || d.Speed <= 0 {
		return ""
	}
	return humanBytes(d.Speed) + "/s"
}

func etaSeconds(d engine.Info) float64 {
	if d.Status != engine.StatusDownloading || d.Speed <= 0 || d.Total <= 0 {
		return -1
	}
	return float64(d.Total-d.Done) / float64(d.Speed)
}

func etaText(d engine.Info) string {
	secs := etaSeconds(d)
	if secs < 0 {
		return ""
	}
	m, s := int(secs)/60, int(secs)%60
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}
