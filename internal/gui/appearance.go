package gui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"odm/internal/engine"
)

// Appearance is how the page should look.
type Appearance struct {
	// Language is "en" or "id".
	Language string `json:"language"`
	// Theme is "system", "light" or "dark".
	Theme string `json:"theme"`
}

// appearanceFor is the look the settings choose (the new UI's Options),
// else the one the Fyne UI keeps in its preferences, so a first start of
// the new UI looks as the Fyne UI did.
func appearanceFor(st engine.Settings) Appearance {
	lang, theme := st.Language, st.Theme
	if lang == "" || theme == "" {
		prefs := fynePreferences()
		if lang == "" {
			lang = prefs["appearance.language"]
		}
		if theme == "" {
			theme = prefs["appearance.theme"]
		}
	}
	return Appearance{Language: resolveLanguage(lang, systemLanguage()), Theme: resolveTheme(theme)}
}

// fynePreferences reads the Fyne UI's preferences file, nil when there is
// none.
func fynePreferences() map[string]string {
	dir := fyneConfigDir()
	if dir == "" {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(dir, "com.ochi.odm", "preferences.json"))
	if err != nil {
		return nil
	}
	var raw map[string]any
	if json.Unmarshal(data, &raw) != nil {
		return nil
	}
	out := map[string]string{}
	for k, v := range raw {
		if s, ok := v.(string); ok {
			out[k] = s
		}
	}
	return out
}

// fyneConfigDir is where Fyne keeps its apps' preferences, as Fyne's own
// rootConfigDir: under the user profile on Windows (not %APPDATA%),
// ~/Library/Preferences on macOS, and the XDG config folder on Linux.
func fyneConfigDir() string {
	switch runtime.GOOS {
	case "windows", "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		if runtime.GOOS == "darwin" {
			return filepath.Join(home, "Library", "Preferences", "fyne")
		}
		return filepath.Join(home, "AppData", "Roaming", "fyne")
	}
	cfg, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(cfg, "fyne")
}

// resolveLanguage picks "id" or "en" from the Options choice ("en", "id",
// or "system"/"" to follow system, a locale such as "id-ID").
func resolveLanguage(choice, system string) string {
	switch choice {
	case "en", "id":
		return choice
	}
	base, _, _ := strings.Cut(strings.ReplaceAll(system, "_", "-"), "-")
	if base = strings.ToLower(base); base == "id" || base == "in" {
		return "id"
	}
	return "en"
}

func resolveTheme(choice string) string {
	switch choice {
	case "light", "dark":
		return choice
	}
	return "system"
}

// darkWindow reports whether the page draws dark for theme: the window is
// painted that colour before the page loads, so it does not flash.
func darkWindow(theme string) bool {
	switch theme {
	case "dark":
		return true
	case "light":
		return false
	}
	return systemDark()
}
