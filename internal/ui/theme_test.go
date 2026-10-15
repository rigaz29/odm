//go:build fyne

package ui

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"

	"odm/internal/engine"
)

func TestThemeModes(t *testing.T) {
	bg := theme.ColorNameBackground
	system := themeFor(themeSystem, "")
	if system.Color(bg, theme.VariantLight) != lightPalette.background ||
		system.Color(bg, theme.VariantDark) != darkPalette.background {
		t.Fatal("the system mode does not follow the system's variant")
	}
	for _, v := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		if got := themeFor(themeLight, "").Color(bg, v); got != lightPalette.background {
			t.Fatalf("light mode, system variant %d: background %v", v, got)
		}
		if got := themeFor(themeDark, "").Color(bg, v); got != darkPalette.background {
			t.Fatalf("dark mode, system variant %d: background %v", v, got)
		}
	}
	// The toolbar tints come from the palette in use.
	if themeFor(themeLight, "").Color(colorToolAdd, theme.VariantDark) != lightPalette.tools[colorToolAdd] {
		t.Fatal("light mode kept the dark toolbar tints")
	}
}

func TestThemeAccent(t *testing.T) {
	green := accentColor("green")
	th := themeFor(themeDark, "green")
	if th.Color(theme.ColorNamePrimary, theme.VariantDark) != green {
		t.Fatal("the accent is not the primary colour")
	}
	// The selection is the accent toned down towards the background.
	sel := th.Color(theme.ColorNameSelection, theme.VariantDark).(color.NRGBA)
	if sel == green || sel == darkPalette.background || sel.G <= darkPalette.background.G {
		t.Fatalf("selection %v", sel)
	}
	if accentColor("no-such") != accents[0].color {
		t.Fatal("an unknown accent is not the default")
	}
}

func TestApplyThemeFromPreferences(t *testing.T) {
	a := newTestApp(t, engine.Settings{})
	p := a.fyneApp.Preferences()
	p.SetString(prefTheme, themeLight)
	p.SetString(prefAccent, "orange")
	a.applyTheme()
	th := a.fyneApp.Settings().Theme()
	if th.Color(theme.ColorNameBackground, theme.VariantDark) != lightPalette.background {
		t.Fatal("the saved light theme was not applied")
	}
	if th.Color(theme.ColorNamePrimary, theme.VariantDark) != accentColor("orange") {
		t.Fatal("the saved accent was not applied")
	}
	// The category tree's compact theme follows the app's theme.
	if (compactTheme{}).Color(theme.ColorNameBackground, theme.VariantDark) != lightPalette.background {
		t.Fatal("the category tree kept the old colours")
	}
	if themeModes[themeIndex(themeDark)] != themeDark || themeIndex("bogus") != 0 {
		t.Fatal("themeIndex")
	}
}
