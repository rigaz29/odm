//go:build gui

package gui

import (
	"context"
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"

	"odm/internal/engine"
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
}

// The window's colours before the page has painted, the page's own
// backgrounds (frontend/src/styles.css).
var (
	darkBackground  = application.NewRGB(9, 9, 11)
	lightBackground = application.NewRGB(255, 255, 255)
)

// Run opens the main window and blocks until it is closed or ctx ends.
//
// The window has no frame: the page draws the title bar, and marks its
// parts with --wails-non-client-region (caption, minimize, maximize,
// close). Windows itself then moves the window, maximizes it on a double
// click, offers the snap layouts over the maximize button and keeps the
// resize border and the shadow. Wails reads those regions only when
// WebView2 is hosted through composition (WebView2CompositionHosting):
// with a child window for the page, the frame never sees the pointer.
func Run(ctx context.Context, mgr *engine.Manager, opts Options) error {
	assets, err := fs.Sub(dist, "frontend/dist")
	if err != nil {
		return err
	}
	svc := NewService(mgr)
	notes := notifications.New()
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
	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "main",
		Title:            "Ochi Download Manager",
		Width:            1280,
		Height:           780,
		MinWidth:         860,
		MinHeight:        520,
		URL:              "/",
		Frameless:        true,
		Hidden:           opts.Hidden,
		BackgroundColour: background,
		Windows: application.WindowsWindow{
			NonClientRegionSupport:     true,
			WebView2CompositionHosting: true,
			Theme:                      windowTheme(look.Theme),
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

	go svc.watch(ctx, 500*time.Millisecond)
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
