package gui

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"odm/internal/engine"
)

// The first start of the new UI takes the look the Fyne UI kept, from
// where Fyne keeps it on each system.
func TestAppearanceFromFynePreferences(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	dir := filepath.Join(home, ".config", "fyne", "com.ochi.odm")
	switch runtime.GOOS {
	case "windows":
		dir = filepath.Join(home, "AppData", "Roaming", "fyne", "com.ochi.odm")
	case "darwin":
		dir = filepath.Join(home, "Library", "Preferences", "fyne", "com.ochi.odm")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	prefs := `{"appearance.language":"id","appearance.theme":"dark"}`
	if err := os.WriteFile(filepath.Join(dir, "preferences.json"), []byte(prefs), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := appearanceFor(engine.Settings{}); got != (Appearance{Language: "id", Theme: "dark"}) {
		t.Fatalf("appearance = %+v, want the Fyne UI's (id, dark)", got)
	}
	// The new UI's own choice wins.
	if got := appearanceFor(engine.Settings{Language: "en", Theme: "light"}); got != (Appearance{Language: "en", Theme: "light"}) {
		t.Fatalf("appearance = %+v, want the settings' (en, light)", got)
	}
}
