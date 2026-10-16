package fluent

import (
	"fmt"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// systemLanguage is the first of the user's preferred Windows display
// languages, such as "id-ID".
func systemLanguage() string {
	langs, err := windows.GetUserPreferredUILanguages(windows.MUI_LANGUAGE_NAME)
	if err != nil || len(langs) == 0 {
		return ""
	}
	return langs[0]
}

// windowsAccent is the accent colour Windows uses for title bars and the
// Start menu, from DWM's AccentColor (0xAABBGGRR).
func windowsAccent() string {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\DWM`, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue("AccentColor")
	if err != nil {
		return ""
	}
	return fmt.Sprintf("#%02x%02x%02x", v&0xff, (v>>8)&0xff, (v>>16)&0xff)
}
