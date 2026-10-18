//go:build gui

package gui

import (
	"context"
	"embed"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"

	"odm/internal/clipmon"
	"odm/internal/engine"
	"odm/internal/osutil"
	"odm/internal/tools"
)

// dist is the built page (npm run build in frontend/).
//
//go:embed all:frontend/dist
var dist embed.FS

// Options tunes how the UI starts; the same fields as ui.Options.
type Options struct {
	// Show receives a value whenever another ODM launch asks this one to
	// bring its window forward.
	Show <-chan struct{}
	// Prompt receives downloads handed over by the browser, each shown in
	// a New Download window.
	Prompt <-chan engine.AddOptions
	// Hidden starts ODM in the tray, the main window out of sight (at
	// login, or for a download from the browser).
	Hidden bool
	// Tools keeps ODM's copies of yt-dlp and ffmpeg; nil for none.
	Tools *tools.Keeper
	// FirstRun is ODM's first start, when the Setup window opens.
	FirstRun bool
}

// The window's colours before the page has painted, the page's own
// backgrounds (frontend/src/styles.css).
var (
	darkBackground  = application.NewRGB(9, 9, 11)
	lightBackground = application.NewRGB(255, 255, 255)
)

// Run opens the main window and blocks until it is closed or ctx ends.
//
// On Windows the window has no frame: the page draws the title bar, moves
// the window from it (--wails-draggable, so Windows' own move: dragged to
// an edge it snaps) and maximizes it on a double click; Windows keeps the
// resize border and the shadow. WebView2 is hosted in a child window, not
// through composition (WebView2CompositionHosting). With composition
// Windows would take the title bar's parts as its own
// (--wails-non-client-region) and offer the snap layouts over the maximize
// button, but the page's UI Automation, what Narrator and other screen
// readers read, then reaches Windows only through the host window, which
// Wails (v3.0.0-beta.30) does not do: to a screen reader the window was
// empty.
//
// On Linux and macOS the window keeps the system's frame, and the page
// draws no window buttons: ODM runs there as it is, not polished.
func Run(ctx context.Context, mgr *engine.Manager, opts Options) error {
	assets, err := fs.Sub(dist, "frontend/dist")
	if err != nil {
		return err
	}
	svc := NewService(mgr)
	// Without the WebView2 Runtime the window would be empty (webview2_windows.go).
	if !webView2Installed() {
		if askForWebView2(svc.translator()) {
			_ = osutil.Open(webView2Page)
		}
		return errors.New("the Microsoft Edge WebView2 Runtime is not installed: " + webView2Page)
	}
	notes := &notifier{svc: notifications.New()}
	app := application.New(application.Options{
		Name:        "Ochi Download Manager",
		Description: "Ochi Download Manager",
		Icon:        appIcon(256), // the windows' and the taskbar's: the exe has none
		Services:    []application.Service{application.NewService(svc), application.NewService(notes)},
		Assets:      application.AssetOptions{Handler: application.BundledAssetFileServer(assets)},
		Windows: application.WindowsOptions{
			// WebView2's profile next to ODM's state, not in a folder
			// named after the exe.
			WebviewUserDataPath: filepath.Join(mgr.ConfigDir(), "webview"),
			// Wails clears WebView2's own WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS
			// at start, against outside overrides. ODM_WEBVIEW2_ARGS lets
			// tests open the page's DevTools port instead
			// (--remote-debugging-port=…).
			AdditionalBrowserArgs: strings.Fields(os.Getenv("ODM_WEBVIEW2_ARGS")),
		},
	})
	svc.mu.Lock()
	svc.emit = func(name string, data any) { app.Event.Emit(name, data) }
	svc.mu.Unlock()

	look := svc.Appearance()
	background := lightBackground
	if darkWindow(look.Theme) {
		background = darkBackground
	}
	// Smaller on a screen too small for it (workarea.go).
	areaW, areaH, _ := workArea()
	width, height := fitSize(1280, 780, areaW, areaH, 32)
	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "main",
		Title:            "Ochi Download Manager",
		Width:            width,
		Height:           height,
		MinWidth:         min(860, width),
		MinHeight:        min(520, height),
		URL:              "/",
		Frameless:        runtime.GOOS == "windows",
		Hidden:           opts.Hidden,
		BackgroundColour: background,
		Windows: application.WindowsWindow{
			Theme: windowTheme(look.Theme),
		},
	})

	windows := &appWindows{app: app, background: background, theme: windowTheme(look.Theme)}
	svc.mu.Lock()
	svc.windows = windows
	svc.keeper = opts.Tools
	svc.quit = app.Quit
	svc.mu.Unlock()
	d := newDesk(app, win, svc, notes)
	d.hookTools(opts.Tools)
	svc.mu.Lock()
	svc.announce = d
	svc.onLook = d.relabel
	svc.mu.Unlock()
	go d.watch(ctx)
	// The UI carries out the scheduler's requests while it runs, so a
	// shutdown can be cancelled from a countdown instead of happening blind.
	mgr.SetActionHandler(svc.handleAction)
	go svc.checkAtStart(ctx)
	go func() {
		// After the main window, which it opens in front of.
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
		svc.setupAtStart(opts.FirstRun, !opts.Hidden)
	}()

	go svc.watch(ctx, 500*time.Millisecond)
	if !clipmon.Native {
		// Linux and macOS have no clipboard watcher (clipmon.Watch, which
		// cmd/odm runs): the clipboard is read every second through Wails
		// instead, once the app runs, as the Fyne UI reads it through Fyne.
		app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
			go clipmon.Poll(ctx, time.Second, func() string {
				text, _ := app.Clipboard.Text()
				return text
			}, func() clipmon.Config {
				st := mgr.Settings()
				return clipmon.Config{Enabled: st.Clipboard, FileTypes: st.FileTypes}
			}, func(u string) {
				mgr.AddAuto(engine.AddOptions{URL: u, Start: true})
			})
		})
	}
	if opts.Prompt != nil {
		go func() {
			for o := range opts.Prompt {
				windows.openAdd(svc.keepPrefill(o))
			}
		}()
	}
	if opts.Show != nil {
		go func() {
			for range opts.Show {
				d.showMain()
			}
		}()
	}
	go func() {
		<-ctx.Done()
		app.Quit()
	}()
	return app.Run()
}

// windowTheme is the window's theme for the page's choice.
func windowTheme(choice string) application.Theme {
	switch choice {
	case "light":
		return application.Light
	case "dark":
		return application.Dark
	}
	return application.SystemDefault
}
