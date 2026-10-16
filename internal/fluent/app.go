//go:build fluent

package fluent

import (
	"context"
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"odm/internal/engine"
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
	// Prompt receives downloads handed over by the browser. Without the
	// New Download window (stage 3) they are added at once.
	Prompt <-chan engine.AddOptions
	// Hidden would start in the tray, which stage 4 brings: the window
	// shows anyway.
	Hidden bool
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

// Run opens the main window and blocks until it is closed or ctx ends.
func Run(ctx context.Context, mgr *engine.Manager, opts Options) error {
	assets, err := fs.Sub(dist, "frontend/dist")
	if err != nil {
		return err
	}
	svc := NewService(mgr)
	app := application.New(application.Options{
		Name:        "Ochi Download Manager",
		Description: "Ochi Download Manager",
		Services:    []application.Service{application.NewService(svc)},
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

	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:      "main",
		Title:     "Ochi Download Manager",
		Width:     1180,
		Height:    700,
		MinWidth:  760,
		MinHeight: 440,
		URL:       "/",
		// Mica shows through the page's transparent background on Windows
		// 11; Windows 10 draws the window colour instead.
		BackgroundType: application.BackgroundTypeTranslucent,
		Windows: application.WindowsWindow{
			BackdropType: application.Mica,
			// Mica's tint follows the window's theme: the same choice as
			// the page's.
			Theme: windowTheme(currentAppearance().Theme),
		},
	})

	go svc.watch(ctx, 500*time.Millisecond)
	if opts.Prompt != nil {
		go func() {
			for o := range opts.Prompt {
				mgr.AddAuto(o)
				svc.poke()
			}
		}()
	}
	if opts.Show != nil {
		go func() {
			for range opts.Show {
				win.Show()
				win.Restore()
				win.Focus()
			}
		}()
	}
	go func() {
		<-ctx.Done()
		app.Quit()
	}()
	return app.Run()
}
