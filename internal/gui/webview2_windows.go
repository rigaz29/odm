package gui

import (
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// The page needs the Microsoft Edge WebView2 Runtime. It comes with
// Windows 11 and with Windows 10 kept up to date, but an old Windows 10
// may lack it, and Wails then opens a window with nothing in it. ODM asks
// first and says what is missing instead.

// webView2Client is where the Runtime's installer records its version, as
// Microsoft documents for detecting it: per machine (in the 32-bit view of
// the registry) or per user.
const webView2Client = `Software\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}`

// webView2Page is Microsoft's page with the Runtime's installer.
const webView2Page = "https://developer.microsoft.com/microsoft-edge/webview2/"

// webView2Installed reports whether the WebView2 Runtime is installed.
func webView2Installed() bool {
	for _, k := range []struct {
		root   registry.Key
		access uint32
	}{
		{registry.LOCAL_MACHINE, registry.QUERY_VALUE | registry.WOW64_32KEY},
		{registry.LOCAL_MACHINE, registry.QUERY_VALUE},
		{registry.CURRENT_USER, registry.QUERY_VALUE},
	} {
		key, err := registry.OpenKey(k.root, webView2Client, k.access)
		if err != nil {
			continue
		}
		pv, _, err := key.GetStringValue("pv")
		key.Close()
		if err == nil && usableVersion(pv) {
			return true
		}
	}
	return false
}

// usableVersion reports whether pv names an installed version: the
// installer leaves "0.0.0.0", or nothing, after an uninstall.
func usableVersion(pv string) bool { return pv != "" && pv != "0.0.0.0" }

var procMessageBox = windows.NewLazySystemDLL("user32.dll").NewProc("MessageBoxW")

const (
	mbYesNo       = 0x04
	mbIconWarning = 0x30
	idYes         = 6
)

// askForWebView2 says the Runtime is missing and asks whether to open the
// page it is downloaded from.
func askForWebView2(tr func(string) string) bool {
	text, _ := windows.UTF16PtrFromString(tr("ODM's window needs the Microsoft Edge WebView2 Runtime, which this computer does not have. It comes with Windows 11 and updated Windows 10, and Microsoft offers it as a free download.\n\nOpen the download page now?"))
	caption, _ := windows.UTF16PtrFromString("Ochi Download Manager")
	r, _, _ := procMessageBox.Call(0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(caption)), mbYesNo|mbIconWarning)
	return r == idYes
}
