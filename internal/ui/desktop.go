//go:build fyne

// Package ui implements the ODM desktop interface with Fyne: a menu bar, an
// icon toolbar, a category/queue sidebar, a sortable report table and dialogs,
// driving the engine in-process (no web server, no embedded browser).
package ui

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"path"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
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
	"odm/internal/tools"
)

// Available reports whether this build includes the desktop UI.
const Available = true

// Torrents reports whether this UI has the windows for torrents; the Fyne
// UI has none.
const Torrents = false

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
	// Tools keeps yt-dlp and ffmpeg; the window asks the person whether
	// it may and tells them when they are ready.
	Tools *tools.Keeper
	// FirstRun is ODM's first start; the new UI's Setup window opens then.
	// This UI has none.
	FirstRun bool
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
	a.applyLanguage() // before any text is made
	a.applyTheme()
	a.win = a.fyneApp.NewWindow("Ochi Download Manager")
	a.win.Resize(fyne.NewSize(1140, 660)) // the toolbar and the search box fit
	// In the accent colour, not the text colour: a dark icon for the light
	// theme vanished on a dark title bar or taskbar, and a light one on a
	// light bar.
	icon := theme.NewColoredResource(theme.DownloadIcon(), theme.ColorNamePrimary)
	a.fyneApp.SetIcon(icon)
	a.win.SetIcon(icon)
	a.notify = a.fyneApp.SendNotification
	a.beep = playSound
	a.hookTools(opts.Tools)
	// Not in front until a window gets the focus: Windows may start ODM
	// behind the window that launched it.
	a.foreground.Store(false)
	a.fyneApp.Lifecycle().SetOnEnteredForeground(func() { a.foreground.Store(true) })
	a.fyneApp.Lifecycle().SetOnExitedForeground(func() { a.foreground.Store(false) })

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
	a.fyneApp.Lifecycle().SetOnStarted(func() { fyne.Do(a.fitMain) })
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
	a.trayMenu = fyne.NewMenu("ODM",
		fyne.NewMenuItem(T("Show ODM"), a.showWindow),
		fyne.NewMenuItem(T("Add URL…"), func() { a.showWindow(); a.openAddDialog() }),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem(T("Pause all"), func() { a.mgr.PauseAll(); a.refresh() }),
		fyne.NewMenuItem(T("Resume all"), func() { a.mgr.ResumeAll(); a.refresh() }),
		a.newLimiterItem(),
		fyne.NewMenuItemSeparator(),
		&fyne.MenuItem{Label: T("Exit"), IsQuit: true, Action: a.fyneApp.Quit},
	)
	desk.SetSystemTrayMenu(a.trayMenu)
	a.limiterShown = "" // the new item has not been synced yet
	a.syncLimiter(a.mgr.Settings().EffectiveSpeedLimit())
	desk.SetSystemTrayIcon(theme.NewColoredResource(theme.DownloadIcon(), theme.ColorNamePrimary))
	desk.SetSystemTrayWindow(a.win) // left-click shows the window
	a.win.SetCloseIntercept(func() {
		if a.mgr.Settings().NoCloseToTray {
			a.fyneApp.Quit()
			return
		}
		a.win.Hide()
		a.foreground.Store(false)
		if !a.trayHinted {
			a.trayHinted = true
			a.fyneApp.SendNotification(fyne.NewNotification(T("ODM is still running"),
				T("Downloads continue in the background. Use the tray icon to open ODM or exit.")))
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
	a.fitMain()
	a.win.RequestFocus()
	a.foreground.Store(true)
}

// fitMain shrinks the main window to the screen the first time it shows:
// its default size is larger than a small laptop's screen.
func (a *application) fitMain() {
	if a.mainFitted {
		return
	}
	a.mainFitted = true
	fitShown(a.win, a.win.Canvas().Size())
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
	keeper  *tools.Keeper
	// mainFitted is set once the window was fitted to the screen.
	mainFitted bool

	table *downloadTable
	cats  *widget.List
	split *container.Split
	// status names the downloads in the list (left of the status bar);
	// activity what runs and the speed limit (right).
	status   *widget.Label
	activity *widget.Label

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
	// The shown columns in order, those hidden, the widths the user
	// dragged, and the list width they were last laid out for.
	visibleCols []int
	hiddenCols  map[int]bool
	userWidths  map[int]float32
	listWidth   float32
	columnsMenu *fyne.Menu
	// headers are the header cells the table created; hoverGrip is the
	// column whose border the pointer is on, or -1.
	headers    []*headerLabel
	hoverGrip  int
	counts     map[string]int
	total      int
	unfinished int
	finished   int
	queues     []string
	colWidths  []float32
	filter     struct {
		kind  string
		value string
	}
	// search is the filter box; query holds its lower-case words.
	search *searchEntry
	query  []string
	// The menus are kept to refresh the speed limiter items, which show
	// whether the global limit applies; limiterShown is what they show now.
	mainMenu     *fyne.MainMenu
	trayMenu     *fyne.Menu
	limiterItems []*fyne.MenuItem
	limiterShown string
	// foreground is whether an ODM window has the focus. notify and beep
	// announce a finished or failed download; tests replace them.
	foreground atomic.Bool
	notify     func(*fyne.Notification)
	beep       func(failed bool)
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
		return len(a.items), len(a.visibleCols)
	}
	t.CreateCell = func() fyne.CanvasObject {
		c := newTableCell()
		c.onSecondary = func(row int, pos fyne.Position) {
			a.secondaryClickRow(row)
			a.showRowMenu(pos)
		}
		c.onPress = func(row int, mods fyne.KeyModifier) {
			a.clickRow(row, mods)
			// Keys (Delete, Space, arrows, Ctrl+A) go to the list next.
			if cv := a.win.Canvas(); cv != nil {
				cv.Focus(t)
			}
		}
		c.onDoubleTap = func(row int) {
			if d, ok := a.itemAt(row); ok {
				a.openProgressWindow(d.ID)
			}
		}
		return c
	}
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
	case fyne.KeySpace:
		a.toggleSelected()
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

// TypedShortcut handles Ctrl+A, and Shift+Del, which Fyne delivers as the
// alternative Cut shortcut. Shortcuts with a menu item (Ctrl+N, Ctrl+F) are
// run by the menu before they reach a widget.
func (t *downloadTable) TypedShortcut(s fyne.Shortcut) {
	switch s := s.(type) {
	case *fyne.ShortcutSelectAll:
		t.app.selectAll()
	case *fyne.ShortcutCut:
		if s.Secondary {
			t.app.removeSelected(true)
		}
	}
}

func (t *downloadTable) Resize(size fyne.Size) {
	t.Table.Resize(size)
	t.app.layoutColumns(size.Width)
}

// MouseDown keeps widget.Table from arming its own column resize, which
// only answers in the narrow gap between headers and keeps the widths where
// the list cannot read them back. headerGrips resizes columns instead.
func (t *downloadTable) MouseDown(*desktop.MouseEvent) {}

// The pointer reaches the table itself in the gaps between the headers,
// which are on a border too.
func (t *downloadTable) MouseIn(e *desktop.MouseEvent) {
	t.Table.MouseIn(e)
	t.hoverHeaderGap(e)
}

func (t *downloadTable) MouseMoved(e *desktop.MouseEvent) {
	t.Table.MouseMoved(e)
	t.hoverHeaderGap(e)
}

func (t *downloadTable) MouseOut() {
	t.Table.MouseOut()
	t.app.hoverGrip = -1
}

func (t *downloadTable) hoverHeaderGap(e *desktop.MouseEvent) {
	t.app.hoverGrip = -1
	if e.Position.Y < rowHeight() {
		t.app.hoverGrip = t.app.gripAt(e.AbsolutePosition.X)
	}
}

func (t *downloadTable) DoubleTapped(*fyne.PointEvent) {
	if d, ok := t.app.current(); ok {
		t.app.openProgressWindow(d.ID)
	}
}

func (t *downloadTable) TappedSecondary(e *fyne.PointEvent) {
	t.app.showRowMenu(e.AbsolutePosition)
}

func (a *application) build() {
	a.loadColumns()
	a.hoverGrip = -1
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
		tool("add", theme.ContentAddIcon(), colorToolAdd, T("Add URL"), a.openAddDialog),
		tool("resume", theme.DownloadIcon(), colorToolRun, T("Resume"), a.resumeSelected),
		tool("stop", theme.MediaStopIcon(), colorToolStop, T("Stop"), a.stopSelected),
		tool("stopall", theme.MediaPauseIcon(), colorToolStop, T("Stop All"), func() { a.mgr.StopAll(); a.refresh() }),
		tool("delete", theme.DeleteIcon(), colorToolDelete, T("Delete"), a.deleteSelected),
		tool("cleardone", theme.ContentClearIcon(), colorToolDelete, T("Clear Done"), func() { a.mgr.ClearCompleted(); a.refresh() }),
		sep(),
		tool("options", theme.SettingsIcon(), colorToolConfig, T("Options"), func() { a.openOptionsTab(0) }),
		tool("scheduler", theme.HistoryIcon(), colorToolSched, T("Scheduler"), a.openScheduler),
		tool("startq", theme.MediaPlayIcon(), colorToolQueue, T("Start Queue"), a.startQueue),
		tool("stopq", theme.MediaStopIcon(), colorToolQueue, T("Stop Queue"), a.stopQueue),
		sep(),
		tool("progress", theme.InfoIcon(), colorToolRun, T("Progress"), func() {
			if d, ok := a.current(); ok {
				a.openProgressWindow(d.ID)
			}
		}),
		tool("folder", theme.FolderOpenIcon(), colorToolSched, T("Open folder"), func() {
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
	searchBox := container.NewCenter(container.NewGridWrap(fyne.NewSize(200, a.search.MinSize().Height), a.search))
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
	base.CreateHeader = func() fyne.CanvasObject {
		h := newHeaderLabel(a)
		a.headers = append(a.headers, h)
		return h
	}
	base.UpdateHeader = func(id widget.TableCellID, o fyne.CanvasObject) {
		h := o.(*headerLabel)
		h.col = a.visibleColumn(id.Col)
		h.SetText(a.headerText(h.col))
		if hh := h.Size().Height; hh > 0 {
			a.mu.Lock()
			a.headH = hh
			a.mu.Unlock()
		}
	}
	a.table = base
	a.layoutColumns(760)
	a.grid = newGridBackdrop(a)
	list := framed(container.NewStack(a.grid, a.table, container.NewVBox(newHeaderGrips(a))))
	// The category tree uses tighter padding; the table's rows are already
	// compact because its cells draw their own text at a fixed height.
	compact := compactTheme{}

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
	sideTitle := widget.NewLabel(T("Categories"))
	sideHeader := container.NewBorder(nil, widget.NewSeparator(), sideTitle, hideBtn)
	sidebar := framed(container.NewBorder(sideHeader, nil, nil, nil,
		container.NewThemeOverride(a.cats, compact)))

	a.split = container.NewHSplit(sidebar, list)
	a.split.Offset = 0.2

	a.status = widget.NewLabel("")
	a.activity = widget.NewLabel("")
	statusBar := container.New(layout.NewCustomPaddedLayout(0, 0, 4, 4),
		container.NewBorder(nil, nil, nil, a.activity, a.status))
	body := container.New(layout.NewCustomPaddedLayout(4, 0, 4, 4), a.split)
	a.win.SetContent(container.NewBorder(top, statusBar, nil, nil, body))
	a.mainMenu = a.buildMenu()
	a.win.SetMainMenu(a.mainMenu)
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

/* ---------- menu bar ---------- */

// keyHint is a shortcut shown next to a menu item. With a modifier the main
// menu also runs it, before the focused widget sees the keys; plain keys
// (Del, Space) are only shown and handled by the list.
func keyHint(key fyne.KeyName, mod fyne.KeyModifier) fyne.Shortcut {
	return &desktop.CustomShortcut{KeyName: key, Modifier: mod}
}

// withKey returns item with a shortcut shown next to it.
func withKey(item *fyne.MenuItem, key fyne.KeyName, mod fyne.KeyModifier) *fyne.MenuItem {
	item.Shortcut = keyHint(key, mod)
	return item
}

func (a *application) buildMenu() *fyne.MainMenu {
	ctrl := fyne.KeyModifierShortcutDefault
	tasks := fyne.NewMenu(T("Tasks"),
		withKey(fyne.NewMenuItem(T("Add new download…"), a.openAddDialog), fyne.KeyN, ctrl),
		fyne.NewMenuItem(T("Grab site…"), a.openGrabberDialog),
		&fyne.MenuItem{IsSeparator: true},
		fyne.NewMenuItem(T("Export download list…"), a.exportList),
		fyne.NewMenuItem(T("Import download list…"), a.importList),
		&fyne.MenuItem{IsSeparator: true},
		// IsQuit, or Fyne appends a Quit item of its own below Exit.
		&fyne.MenuItem{Label: T("Exit"), IsQuit: true, Action: func() { a.fyneApp.Quit() }},
	)
	downloads := fyne.NewMenu(T("Downloads"),
		fyne.NewMenuItem(T("Pause all"), func() { a.mgr.PauseAll(); a.refresh() }),
		fyne.NewMenuItem(T("Stop all (and stop queues)"), func() { a.mgr.StopAll(); a.refresh() }),
		fyne.NewMenuItem(T("Clear completed"), func() { a.mgr.ClearCompleted(); a.refresh() }),
		&fyne.MenuItem{IsSeparator: true},
		a.newLimiterItem(),
		fyne.NewMenuItem(T("Speed limiter settings…"), func() { a.openOptionsTab(optionsTabConnection) }),
		&fyne.MenuItem{IsSeparator: true},
		fyne.NewMenuItem(T("Start queue"), a.startQueue),
		fyne.NewMenuItem(T("Stop queue"), a.stopQueue),
		fyne.NewMenuItem(T("New queue…"), a.openNewQueueDialog),
		fyne.NewMenuItem(T("Scheduler…"), a.openScheduler),
		&fyne.MenuItem{IsSeparator: true},
		fyne.NewMenuItem(T("Options…"), func() { a.openOptionsTab(0) }),
	)
	a.columnsMenu = fyne.NewMenu("", a.columnMenuItems()...)
	columnsItem := fyne.NewMenuItem(T("Columns"), nil)
	columnsItem.ChildMenu = a.columnsMenu
	view := fyne.NewMenu(T("View"),
		withKey(fyne.NewMenuItem(T("Search downloads"), a.focusSearch), fyne.KeyF, ctrl),
		&fyne.MenuItem{IsSeparator: true},
		fyne.NewMenuItem(T("Hide categories"), a.toggleSidebar),
		columnsItem,
		&fyne.MenuItem{IsSeparator: true},
		fyne.NewMenuItem(T("Sort by name"), func() { a.sortBy(colName) }),
		fyne.NewMenuItem(T("Sort by size"), func() { a.sortBy(colSize) }),
		fyne.NewMenuItem(T("Sort by status"), func() { a.sortBy(colStatus) }),
		fyne.NewMenuItem(T("Sort by time left"), func() { a.sortBy(colETA) }),
		fyne.NewMenuItem(T("Sort by transfer rate"), func() { a.sortBy(colRate) }),
		fyne.NewMenuItem(T("Sort by description"), func() { a.sortBy(colDescription) }),
	)
	help := fyne.NewMenu(T("Help"),
		fyne.NewMenuItem(T("Keyboard shortcuts"), a.showShortcuts),
		fyne.NewMenuItem(T("Check for updates…"), func() { a.checkForUpdates(true) }),
		fyne.NewMenuItem(T("About ODM"), a.showAbout),
	)
	return fyne.NewMainMenu(tasks, downloads, view, help)
}

func (a *application) showAbout() {
	dialog.ShowInformation(T("About"),
		T("Ochi Download Manager (ODM)\n\nA segmented, resumable download manager.\nBuilt with Go and Fyne."),
		a.win)
}

// shortcuts lists the keys of the main window, for Help → Keyboard shortcuts.
var shortcuts = [][2]string{
	{"Ctrl+N", markT("Add a new download")},
	{"Ctrl+F", markT("Search the list (Esc clears the search)")},
	{"Ctrl+A", markT("Select every download in the list")},
	{"Enter", markT("Show the progress of the selected download")},
	{"Space", markT("Pause or resume the selected downloads")},
	{"Del", markT("Remove the selected downloads from the list")},
	{"Shift+Del", markT("Remove them and delete their files")},
	{"Up / Down", markT("Move the selection (with Shift: extend it)")},
	{"Ctrl+click", markT("Add a download to the selection, or take it out")},
	{"Shift+click", markT("Select every download from the last one clicked")},
}

func (a *application) showShortcuts() {
	grid := container.New(layout.NewFormLayout())
	for _, s := range shortcuts {
		grid.Add(widget.NewLabelWithStyle(s[0], fyne.TextAlignTrailing, fyne.TextStyle{Bold: true}))
		grid.Add(widget.NewLabel(T(s[1])))
	}
	dialog.ShowCustom(T("Keyboard shortcuts"), T("Close"), grid, a.win)
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
		dialog.ShowInformation(T("Import"), Tf("Imported %d download(s).", n), a.win)
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
		{text: label(T("All Downloads"), len(all)), kind: "all", icon: theme.FolderOpenIcon(), node: "all", expanded: allOpen},
	}
	if allOpen {
		for _, c := range engine.Categories() {
			out = append(out, sidebarEntry{
				text: label(c, counts[c]), kind: "category", value: c, indent: 1, icon: iconForCategory(c),
			})
		}
	}
	out = append(out,
		sidebarEntry{text: label(T("Unfinished"), unfinished), kind: "unfinished", icon: theme.DownloadIcon()},
		sidebarEntry{text: label(T("Finished"), finished), kind: "finished", icon: theme.ConfirmIcon()},
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
	out = append(out, sidebarEntry{text: T("Queues"), icon: theme.ListIcon(), header: true, node: "queues", expanded: queuesOpen})
	if queuesOpen {
		scheds := a.mgr.Settings().Schedules
		for _, q := range queues {
			icon := folder
			if scheds[q].Sync {
				icon = queueIcon(scheds[q])
			}
			out = append(out, sidebarEntry{
				text: label(q, perQueue[strings.ToLower(q)]), kind: "queue", value: q, indent: 1, icon: icon,
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
	entry.SetPlaceHolder(T("Queue name"))
	d := dialog.NewCustomConfirm(T("New queue"), T("Create"), T("Cancel"),
		container.NewVBox(widget.NewLabel(T("Enter a name for the new queue:")), entry),
		func(ok bool) {
			if !ok || strings.TrimSpace(entry.Text) == "" {
				return
			}
			if !a.mgr.AddQueue(entry.Text) {
				dialog.ShowInformation(T("Queue"), T("That queue already exists."), a.win)
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

	c.row = id.Row
	a.mu.Lock()
	if id.Row < 0 || id.Row >= len(a.items) {
		a.mu.Unlock()
		c.row = -1
		c.setText("")
		c.showIcon(false)
		c.setSelected(false)
		return
	}
	d := a.items[id.Row]
	col := -1
	if id.Col >= 0 && id.Col < len(a.visibleCols) {
		col = a.visibleCols[id.Col]
	}
	a.mu.Unlock()

	if col == colName {
		c.setIcon(iconForCategory(d.Category))
	}
	c.showIcon(col == colName)
	a.mu.Lock()
	selected := a.selected[d.ID]
	a.rowH = rowHeight()
	a.mu.Unlock()
	c.setSelected(selected)

	switch col {
	case colName:
		c.setText(displayName(d))
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
		c.setText(d.UpdatedAt.Format(T("Jan 2 15:04")))
	case colDescription:
		c.setText(d.Description)
	}
}

// displayName is the name shown for a download: its file name, or until the
// first probe names it (and when that probe failed), the last part of its
// address.
func displayName(d engine.Info) string {
	if d.FileName != "" {
		return d.FileName
	}
	if u, err := url.Parse(d.URL); err == nil {
		if base := path.Base(u.Path); base != "" && base != "/" && base != "." {
			return base
		}
		if u.Host != "" {
			return u.Host
		}
	}
	return d.URL
}

// statusText is the Status column: the percentage while transferring, and the
// state plus how far it got otherwise, so a stopped download still shows its
// progress now that the list has no progress bar.
func statusText(d engine.Info) string {
	if d.Verifying {
		return T("Verifying checksum")
	}
	pct := ""
	if d.Total > 0 && d.Progress > 0 && !d.Live {
		pct = fmt.Sprintf(" · %.0f%%", d.Progress)
	}
	switch d.Status {
	case engine.StatusDownloading:
		if d.Live {
			// Total follows Done while recording: a percentage would
			// read 100% throughout.
			return T("Recording")
		}
		if d.Total > 0 {
			return fmt.Sprintf("%.2f%%", d.Progress)
		}
		return T("Downloading")
	case engine.StatusQueued:
		return T("Queued") + pct
	case engine.StatusPaused:
		return T("Paused") + pct
	case engine.StatusComplete:
		return T("Complete")
	case engine.StatusError:
		if d.Error != "" {
			return T("Error: ") + d.Error
		}
		return T("Error") + pct
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
	if col < 0 || col >= columnCount {
		return ""
	}
	a.mu.Lock()
	sc, asc := a.sortCol, a.sortAsc
	a.mu.Unlock()
	if sc == col {
		if asc {
			return T(columns[col].title) + " ▲"
		}
		return T(columns[col].title) + " ▼"
	}
	return T(columns[col].title)
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
	a.saveColumns()
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
	limit := a.mgr.Settings().EffectiveSpeedLimit()
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

	active, queued, speed := 0, 0, int64(0)
	for _, d := range all {
		switch d.Status {
		case engine.StatusDownloading:
			active++
			speed += d.Speed
		case engine.StatusQueued:
			queued++
		}
	}
	if a.activity != nil {
		a.activity.SetText(activityStatus(active, queued, speed, limit))
	}
	a.updateStatus()
	a.syncLimiter(limit)

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

// updateStatus fills the left of the status bar. Selecting rows calls it
// too, so the count does not wait for the next refresh.
func (a *application) updateStatus() {
	if a.status == nil {
		return
	}
	a.mu.Lock()
	text := listStatus(a.total, len(a.items), len(a.query) > 0, len(a.selected))
	a.mu.Unlock()
	a.status.SetText(text)
}

// listStatus says how many downloads there are, how many of them match the
// search, and how many are selected when that is more than one.
func listStatus(total, shown int, searching bool, selected int) string {
	text := Tf("%d items", total)
	if total == 1 {
		text = T("1 item")
	}
	if searching {
		text = Tf("%d of %s match the search", shown, text)
	}
	if selected > 1 {
		text += Tf(" · %d selected", selected)
	}
	return text
}

// activityStatus is the total speed of the running downloads, how many run
// and wait in a queue, and the global speed limit while it applies.
func activityStatus(active, queued int, speed, limit int64) string {
	var parts []string
	switch {
	case active > 0 && speed > 0:
		parts = append(parts, Tf("%d active at %s/s", active, humanBytes(speed)))
	case active > 0:
		parts = append(parts, Tf("%d active", active))
	}
	if queued > 0 {
		parts = append(parts, Tf("%d queued", queued))
	}
	if limit > 0 {
		parts = append(parts, Tf("speed limit %s/s", humanBytes(limit)))
	}
	return strings.Join(parts, " · ")
}

// noticeCompletions closes the progress window of each download that just
// finished and shows the completion dialog in its place, and announces each
// download that just finished or failed.
func (a *application) noticeCompletions(all []engine.Info) {
	first := a.lastStatus == nil
	prev := a.lastStatus
	a.lastStatus = make(map[string]engine.Status, len(all))
	show := !a.mgr.Settings().NoCompleteDialog
	for _, d := range all {
		a.lastStatus[d.ID] = d.Status
		if first {
			continue
		}
		was, seen := prev[d.ID]
		if !seen || was == d.Status {
			continue
		}
		switch d.Status {
		case engine.StatusComplete:
			if v, ok := a.progressWindows[d.ID]; ok {
				v.win.Close()
			}
			if show {
				a.showComplete(d)
			}
			a.announce(d, false, show)
		case engine.StatusError:
			a.announce(d, true, false)
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

// itemAt returns the download shown on row.
func (a *application) itemAt(row int) (*engine.Info, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if row < 0 || row >= len(a.items) {
		return nil, false
	}
	cp := a.items[row]
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
	items := []*fyne.MenuItem{withKey(fyne.NewMenuItem(T("Show progress"), func() { a.openProgressWindow(d.ID) }), fyne.KeyEnter, 0)}
	if d.Status == engine.StatusComplete {
		items = append(items, fyne.NewMenuItem(T("Open file"), func() { openPath(d.FilePath) }))
	}
	refresh := fyne.NewMenuItem(T("Refresh download address…"), func() { a.refreshAddress(*d) })
	// Only for unfinished, stopped downloads: the engine refuses others.
	refresh.Disabled = d.Status == engine.StatusComplete || d.Status == engine.StatusDownloading || d.Status == engine.StatusQueued
	items = append(items,
		fyne.NewMenuItem(T("Open folder"), func() { revealPath(d.FilePath) }),
		refresh,
		withKey(fyne.NewMenuItem(T("Pause / Resume"), a.toggleSelected), fyne.KeySpace, 0),
	)
	move := fyne.NewMenuItem(T("Move to queue"), nil)
	move.ChildMenu = a.queueSubmenu(sel)
	items = append(items, move,
		fyne.NewMenuItem(T("Move up in queue"), func() { a.mgr.MoveInQueue(d.ID, -1); a.refresh() }),
		fyne.NewMenuItem(T("Move down in queue"), func() { a.mgr.MoveInQueue(d.ID, +1); a.refresh() }),
	)
	items = append(items,
		withKey(fyne.NewMenuItem(T("Remove from list"), func() { a.removeSelected(false) }), fyne.KeyDelete, 0),
		withKey(fyne.NewMenuItem(T("Remove + delete file"), func() { a.removeSelected(true) }), fyne.KeyDelete, fyne.KeyModifierShift),
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
		out = append(out, &fyne.MenuItem{Label: T("(no queues)"), Disabled: true})
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
// toggles it, Shift selects the range from the focus row (Ctrl+Shift adds
// that range to the selection).
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
		if mods&fyne.KeyModifierShortcutDefault == 0 {
			a.selected = map[string]bool{}
		}
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
	a.updateStatus()
}

// secondaryClickRow readies a right-click on row, as Explorer does: a row
// outside the selection becomes the selection; within it, the selection
// stays (the menu acts on all of it) and the row becomes the current one,
// which the menu's single-file items use. The list takes the keyboard
// focus, as a left click gives it.
func (a *application) secondaryClickRow(row int) {
	a.mu.Lock()
	if row < 0 || row >= len(a.items) {
		a.mu.Unlock()
		return
	}
	id := a.items[row].ID
	if !a.selected[id] {
		a.selected = map[string]bool{id: true}
	}
	a.focusID, a.selectedRow = id, row
	a.mu.Unlock()
	a.table.Refresh()
	a.updateToolbar()
	a.updateStatus()
	if c := a.win.Canvas(); c != nil {
		c.Focus(a.table)
	}
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
	a.updateStatus()
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

// toggleSelected pauses the selection when the focused download is running
// or waiting in a queue, and resumes it otherwise: the focused row decides
// the direction for all of them. A finished focused download does nothing.
func (a *application) toggleSelected() {
	d, ok := a.current()
	if !ok {
		return
	}
	switch d.Status {
	case engine.StatusDownloading, engine.StatusQueued:
		a.stopSelected()
	case engine.StatusComplete:
	default:
		a.resumeSelected()
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
		what = Tf("%d downloads", len(sel))
	}
	msg := Tf("Remove %s from the list? (files are kept)", what)
	if withFile {
		msg = Tf("Remove %s from the list and delete the files from disk?", what)
	}
	dialogConfirm(a.win, T("Delete"), msg, func(yes bool) {
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
		return Tf("%dm %ds", m, s)
	}
	return Tf("%ds", s)
}
