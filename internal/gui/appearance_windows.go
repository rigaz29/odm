package gui

import (
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

// systemDark reports whether Windows draws apps dark (Settings →
// Personalisation → Colours).
func systemDark() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue("AppsUseLightTheme")
	return err == nil && v == 0
}
