//go:build fyne && windows

package ui

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var procSystemParametersInfo = windows.NewLazySystemDLL("user32.dll").NewProc("SystemParametersInfoW")

// spiGetWorkArea asks SystemParametersInfo for the primary monitor's work
// area: the screen without the taskbar, in pixels (ODM is DPI aware).
const spiGetWorkArea = 0x0030

// workArea is the usable size of the primary screen in pixels.
func workArea() (width, height int, ok bool) {
	var r struct{ Left, Top, Right, Bottom int32 }
	ret, _, _ := procSystemParametersInfo.Call(spiGetWorkArea, 0, uintptr(unsafe.Pointer(&r)), 0)
	if ret == 0 || r.Right <= r.Left || r.Bottom <= r.Top {
		return 0, 0, false
	}
	return int(r.Right - r.Left), int(r.Bottom - r.Top), true
}
