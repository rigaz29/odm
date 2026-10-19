//go:build gui

package gui

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"

	"odm/internal/engine"
	"odm/internal/tools"
)

// desk is ODM on the desktop, as the Fyne UI's (internal/ui/
// desktop.go, setupTray; notify_desktop.go): the tray icon and its menu,
// closing the main window into the tray, and the system notifications and
// sounds for finished and failed downloads.
type desk struct {
	app   *application.App
	win   *application.WebviewWindow
	svc   *Service
	tray  *application.SystemTray
	menu  *application.Menu
	notes *notifier

	items struct{ show, add, pause, resume, limiter, exit *application.MenuItem }

	mu     sync.Mutex
	hinted bool   // told once that ODM keeps running in the tray
	shown  string // the limiter label shown, to change it only when it changes
	seq    int
}

func newDesk(app *application.App, win *application.WebviewWindow, svc *Service, notes *notifier) *desk {
	d := &desk{app: app, win: win, svc: svc, notes: notes}
	d.menu = application.NewMenu()
	d.items.show = d.menu.Add("").OnClick(func(*application.Context) { d.showMain() })
	d.items.add = d.menu.Add("").OnClick(func(*application.Context) {
		if w := svc.opener(); w != nil {
			w.openAdd(svc.keepPrefill(engine.AddOptions{}))
		}
	})
	d.menu.AddSeparator()
	d.items.pause = d.menu.Add("").OnClick(func(*application.Context) { svc.PauseAll() })
	d.items.resume = d.menu.Add("").OnClick(func(*application.Context) { svc.ResumeAll() })
	d.items.limiter = d.menu.AddCheckbox("", false).OnClick(func(*application.Context) { d.toggleLimiter() })
	d.menu.AddSeparator()
	d.items.exit = d.menu.Add("").OnClick(func(*application.Context) { app.Quit() })
	d.relabel()

	d.tray = app.SystemTray.New()
	d.tray.SetIcon(appIcon(64))
	d.tray.SetTooltip("Ochi Download Manager")
	if runtime.GOOS == "linux" {
		// Wails has no tray tooltips on Linux: the tray shows the label as
		// the icon's name and tooltip, "Wails" without one. (On macOS a
		// label is text beside the icon.)
		d.tray.SetLabel("Ochi Download Manager")
	}
	d.tray.SetMenu(d.menu)
	d.tray.OnClick(d.showMain) // a left click shows the window, the right the menu

	// Closing the main window hides it in the tray, where downloads carry
	// on, unless the Options say to quit.
	win.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		if svc.mgr.Settings().NoCloseToTray {
			app.Quit()
			return
		}
		e.Cancel()
		win.Hide()
		d.mu.Lock()
		first := !d.hinted
		d.hinted = true
		d.mu.Unlock()
		if first {
			tr := svc.translator()
			d.notify(tr("ODM is still running"), tr("Downloads continue in the background. Use the tray icon to open ODM or exit."))
		}
	})
	notes.svc.OnNotificationResponse(func(notifications.NotificationResult) { d.showMain() })
	return d
}

// relabel puts the menu in the page's language and names the speed limit.
func (d *desk) relabel() {
	tr := d.svc.translator()
	d.items.show.SetLabel(tr("Show ODM"))
	d.items.add.SetLabel(tr("Add URL…"))
	d.items.pause.SetLabel(tr("Pause all"))
	d.items.resume.SetLabel(tr("Resume all"))
	d.items.exit.SetLabel(tr("Exit"))
	d.mu.Lock()
	d.shown = "" // the limiter item too
	d.mu.Unlock()
	d.syncLimiter()
	d.menu.Update()
}

// syncLimiter ticks the limiter item while the limit applies and names it.
func (d *desk) syncLimiter() {
	rate := d.svc.mgr.Settings().EffectiveSpeedLimit()
	tr := d.svc.translator()
	label := tr("Speed limiter")
	if rate > 0 {
		label = fmt.Sprintf(tr("Speed limiter (%s/s)"), formatBytes(rate, d.svc.Appearance().Language))
	}
	d.mu.Lock()
	same := label == d.shown
	d.shown = label
	d.mu.Unlock()
	if same {
		return
	}
	d.items.limiter.SetLabel(label)
	d.items.limiter.SetChecked(rate > 0)
	d.menu.Update()
}

// toggleLimiter switches the limit off, or on again with the one saved;
// with none saved the main window opens, where the status bar sets one.
func (d *desk) toggleLimiter() {
	st := d.svc.mgr.Settings()
	switch {
	case st.EffectiveSpeedLimit() > 0:
		d.svc.SetSpeedLimiter(false, 0)
	case st.SpeedLimit > 0:
		d.svc.SetSpeedLimiter(true, 0)
	default:
		d.showMain()
	}
	d.syncLimiter()
}

func (d *desk) showMain() {
	d.win.Show()
	d.win.Restore()
	d.win.Focus()
	// After a start in the tray, the Setup window waits for this.
	go d.svc.offerSetup()
}

// watch keeps the tray up to date: the limiter item, and a tooltip with
// what is running.
func (d *desk) watch(ctx context.Context) {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	tip := ""
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		d.syncLimiter()
		running, speed := 0, int64(0)
		for _, it := range d.svc.mgr.List() {
			if it.Status == engine.StatusDownloading {
				running++
				speed += it.Speed
			}
		}
		next := "Ochi Download Manager"
		if running > 0 {
			tr := d.svc.translator()
			next += fmt.Sprintf("\n%s · %s/s", fmt.Sprintf(tr("%d active"), running), formatBytes(speed, d.svc.Appearance().Language))
		}
		if next != tip {
			tip = next
			d.tray.SetTooltip(tip)
		}
	}
}

// notify shows a system notification, if there are any (notifier); a
// click on it shows the window.
func (d *desk) notify(title, body string) {
	if !d.notes.ok.Load() {
		return
	}
	d.mu.Lock()
	d.seq++
	id := "odm-" + strconv.FormatInt(time.Now().UnixNano(), 36) + "-" + strconv.Itoa(d.seq)
	d.mu.Unlock()
	_ = d.notes.svc.SendNotification(notifications.NotificationOptions{ID: id, Title: title, Body: body})
}

// notifier is Wails' notification service, which ODM starts without: on
// macOS it needs ODM to be an .app (a bundle identifier), on Linux a
// session bus. A service that fails to start stops the whole app, so this
// one logs why instead, and ODM goes on without notifications. It has no
// methods for the page.
type notifier struct {
	svc *notifications.NotificationService
	ok  atomic.Bool // started
}

func (n *notifier) ServiceName() string { return n.svc.ServiceName() }

func (n *notifier) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	if err := n.svc.ServiceStartup(ctx, options); err != nil {
		log.Println("notifications:", err)
		return nil
	}
	n.ok.Store(true)
	return nil
}

func (n *notifier) ServiceShutdown() error {
	if !n.ok.Load() {
		return nil
	}
	return n.svc.ServiceShutdown()
}

func (d *desk) beep(failed bool) { playSound(failed) }

// hookTools asks once whether ODM may download yt-dlp and ffmpeg itself,
// the first time one is needed, and whether it may download odm-libtorrent
// whenever a torrent needs it; and tells when one is ready. Torrents that
// stopped for want of odm-libtorrent start again once it is in.
func (d *desk) hookTools(k *tools.Keeper) {
	if k == nil {
		return
	}
	k.Ask = func(tool string) {
		w := d.svc.opener()
		switch {
		case w == nil:
		case tool == "":
			w.openNotice(url.Values{"kind": {"tools"}})
		default:
			w.openNotice(url.Values{"kind": {"program"}, "name": {tool}})
		}
	}
	k.OnInstalled = func(t tools.Tool, version string, first bool) {
		if first {
			tr := d.svc.translator()
			d.notify("ODM", fmt.Sprintf(tr("%s %s is ready"), t.Name, version))
		}
		if t.Name == tools.Libtorrent().Name {
			d.svc.resumeWaitingTorrents()
		}
	}
}
