//go:build gui

package gui

import (
	"reflect"
	"runtime"
	"testing"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"golang.org/x/sys/windows"
)

var (
	procGetMenuStringW = windows.NewLazySystemDLL("user32.dll").NewProc("GetMenuStringW")
	procGetMenuState   = windows.NewLazySystemDLL("user32.dll").NewProc("GetMenuState")
)

// trayRow is row pos of the menu the tray shows on a right click, as
// Windows has it: its label and whether it is ticked. Wails keeps that menu
// unexported (SystemTray.impl.menu.menu, an HMENU).
func trayRow(tray *application.SystemTray, pos int) (label string, ticked, ok bool) {
	impl := reflect.ValueOf(tray).Elem().FieldByName("impl")
	if !impl.IsValid() || impl.IsNil() {
		return "", false, false
	}
	popup := impl.Elem().Elem().FieldByName("menu")
	if !popup.IsValid() || popup.IsNil() {
		return "", false, false
	}
	h := uintptr(popup.Elem().FieldByName("menu").Uint())
	const byPosition, checked = 0x400, 0x8
	buf := make([]uint16, 256)
	if n, _, _ := procGetMenuStringW.Call(h, uintptr(pos), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), byPosition); n == 0 {
		return "", false, false
	}
	state, _, _ := procGetMenuState.Call(h, uintptr(pos), byPosition)
	return windows.UTF16ToString(buf), state != 0xFFFFFFFF && state&checked != 0, true
}

// The tray's menu follows the speed limit and the language each time, not
// only the first: the limiter item is ticked and named while a limit
// applies, and a click that ticks it with no limit saved is undone.
func TestTrayMenuFollowsTheLimiterAndTheLanguage(t *testing.T) {
	runtime.LockOSThread() // Wails' message loop runs on this goroutine
	defer runtime.UnlockOSThread()
	svc, mgr := newTestService(t)
	app := application.New(application.Options{Name: "ODM tray test"})

	const show, limiter = 0, 5 // the rows: show, add, -, pause, resume, limiter, -, exit
	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		go func() {
			defer app.Quit()
			// Made once the app runs, so that the tray is up when newTray
			// returns.
			d := newTray(app, svc)
			want := func(step string, row int, label string, ticked bool) {
				got, tick, ok := trayRow(d.tray, row)
				switch {
				case !ok:
					t.Errorf("%s: the tray shows no row %d", step, row)
				case got != label || tick != ticked:
					t.Errorf("%s: the tray shows %q (ticked %v), want %q (ticked %v)", step, got, tick, label, ticked)
				}
			}
			setLanguage := func(lang string) {
				st := mgr.Settings()
				st.Language = lang
				mgr.UpdateSettings(st)
				d.relabel() // the Options' new look
			}

			want("at start", limiter, "Speed limiter", false)
			svc.SetSpeedLimiter(true, 1<<20)
			d.syncLimiter() // desk.watch's next look
			want("limit on", limiter, "Speed limiter (1.00 MB/s)", true)
			svc.SetSpeedLimiter(false, 0)
			d.syncLimiter()
			want("limit off", limiter, "Speed limiter", false)
			svc.SetSpeedLimiter(true, 0)
			d.syncLimiter()
			want("limit on again", limiter, "Speed limiter (1.00 MB/s)", true)

			setLanguage("id")
			want("in Indonesian", show, "Tampilkan ODM", false)
			want("in Indonesian", limiter, "Pembatas kecepatan (1,00 MB/s)", true)
			setLanguage("en")
			want("in English again", show, "Show ODM", false)
			want("in English again", limiter, "Speed limiter (1.00 MB/s)", true)

			st := mgr.Settings()
			st.SpeedLimit, st.LimiterOff = 0, true
			mgr.UpdateSettings(st)
			d.syncLimiter()
			want("no limit saved", limiter, "Speed limiter", false)
			d.items.limiter.SetChecked(true) // what a click does before toggleLimiter
			d.syncLimiter()                  // which ends with this
			want("clicked with no limit saved", limiter, "Speed limiter", false)
		}()
	})
	if err := app.Run(); err != nil {
		t.Fatal(err)
	}
}
