package fluent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Appearance is how the page should look.
type Appearance struct {
	// Language is "en" or "id".
	Language string `json:"language"`
	// Theme is "system", "light" or "dark".
	Theme string `json:"theme"`
	// Accent is Windows' accent colour as "#rrggbb", "" when unknown.
	Accent string `json:"accent"`
}

// currentAppearance reads the choices the Fyne UI keeps (Options → General:
// language and theme, in Fyne's preferences) so both UIs look alike, and
// the accent colour from Windows. Stage 1 moves these choices to ODM's own
// settings.
func currentAppearance() Appearance {
	prefs := fynePreferences()
	return Appearance{
		Language: resolveLanguage(prefs["appearance.language"], systemLanguage()),
		Theme:    resolveTheme(prefs["appearance.theme"]),
		Accent:   windowsAccent(),
	}
}

// fynePreferences reads the Fyne UI's preferences file, nil when there is
// none. Fyne keeps it under the user profile, not %APPDATA%.
func fynePreferences() map[string]string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(home, "AppData", "Roaming", "fyne", "com.ochi.odm", "preferences.json"))
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
