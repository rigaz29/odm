package gui

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32                   = windows.NewLazySystemDLL("user32.dll")
	procSystemParametersInfo = user32.NewProc("SystemParametersInfoW")
	procGetDpiForSystem      = user32.NewProc("GetDpiForSystem")
)

const spiGetWorkArea = 0x0030

// workArea is the primary screen's work area (the screen without the
// taskbar) in the unit window sizes are given in: pixels at 96 DPI. ODM is
// DPI aware (cmd/odm/odm.manifest), so Windows answers in pixels, scaled
// back here by the screen's DPI.
func workArea() (width, height int, ok bool) {
	var r struct{ Left, Top, Right, Bottom int32 }
	ret, _, _ := procSystemParametersInfo.Call(spiGetWorkArea, 0, uintptr(unsafe.Pointer(&r)), 0)
	if ret == 0 || r.Right <= r.Left || r.Bottom <= r.Top {
		return 0, 0, false
	}
	dpi := uintptr(96)
	if procGetDpiForSystem.Find() == nil {
		if d, _, _ := procGetDpiForSystem.Call(); d > 0 {
			dpi = d
		}
	}
	return int(r.Right-r.Left) * 96 / int(dpi), int(r.Bottom-r.Top) * 96 / int(dpi), true
}
