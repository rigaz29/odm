//go:build fyne

package ui

import (
	"image/color"
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// palette is one set of the ODM colours, for a dark or a light window.
type palette struct {
	background  color.NRGBA
	surface     color.NRGBA
	surfaceAlt  color.NRGBA
	border      color.NRGBA
	foreground  color.NRGBA
	muted       color.NRGBA
	hover       color.NRGBA
	pressed     color.NRGBA
	success     color.NRGBA
	warning     color.NRGBA
	danger      color.NRGBA
	scrollTrack color.NRGBA
	scrollThumb color.NRGBA
	// selectMix is how much of the accent colours the selection.
	selectMix float64
	// tools tints the toolbar icons: soft, desaturated hues on the dark
	// background, deeper ones on the light.
	tools map[fyne.ThemeColorName]color.NRGBA
}

var darkPalette = palette{
	background:  color.NRGBA{R: 0x0E, G: 0x10, B: 0x14, A: 0xFF},
	surface:     color.NRGBA{R: 0x15, G: 0x18, B: 0x1F, A: 0xFF},
	surfaceAlt:  color.NRGBA{R: 0x1B, G: 0x1F, B: 0x27, A: 0xFF},
	border:      color.NRGBA{R: 0x26, G: 0x2B, B: 0x36, A: 0xFF},
	foreground:  color.NRGBA{R: 0xE7, G: 0xEA, B: 0xF1, A: 0xFF},
	muted:       color.NRGBA{R: 0x8B, G: 0x93, B: 0xA5, A: 0xFF},
	hover:       color.NRGBA{R: 0x22, G: 0x27, B: 0x31, A: 0xFF},
	pressed:     color.NRGBA{R: 0x2A, G: 0x30, B: 0x3D, A: 0xFF},
	success:     color.NRGBA{R: 0x3F, G: 0xBF, B: 0x8F, A: 0xFF},
	warning:     color.NRGBA{R: 0xE0, G: 0xA9, B: 0x3B, A: 0xFF},
	danger:      color.NRGBA{R: 0xF0, G: 0x55, B: 0x5C, A: 0xFF},
	scrollTrack: color.NRGBA{R: 0x1B, G: 0x1F, B: 0x27, A: 0xFF},
	scrollThumb: color.NRGBA{R: 0x39, G: 0x41, B: 0x52, A: 0xFF},
	selectMix:   0.38,
	tools: map[fyne.ThemeColorName]color.NRGBA{
		colorToolAdd:    {R: 0x5C, G: 0xC8, B: 0xA8, A: 0xFF},
		colorToolRun:    {R: 0x7F, G: 0xB2, B: 0xFF, A: 0xFF},
		colorToolStop:   {R: 0xB4, G: 0xBC, B: 0xCC, A: 0xFF},
		colorToolDelete: {R: 0xF0, G: 0x7A, B: 0x9A, A: 0xFF},
		colorToolConfig: {R: 0x9D, G: 0x8C, B: 0xF5, A: 0xFF},
		colorToolSched:  {R: 0xE6, G: 0xA8, B: 0x8C, A: 0xFF},
		colorToolQueue:  {R: 0x7C, G: 0xC4, B: 0xE8, A: 0xFF},
	},
}

var lightPalette = palette{
	background:  color.NRGBA{R: 0xF4, G: 0xF5, B: 0xF8, A: 0xFF},
	surface:     color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF},
	surfaceAlt:  color.NRGBA{R: 0xE9, G: 0xEC, B: 0xF1, A: 0xFF},
	border:      color.NRGBA{R: 0xD5, G: 0xDA, B: 0xE3, A: 0xFF},
	foreground:  color.NRGBA{R: 0x1A, G: 0x1E, B: 0x26, A: 0xFF},
	muted:       color.NRGBA{R: 0x68, G: 0x70, B: 0x80, A: 0xFF},
	hover:       color.NRGBA{R: 0xE3, G: 0xE7, B: 0xEE, A: 0xFF},
	pressed:     color.NRGBA{R: 0xD6, G: 0xDC, B: 0xE6, A: 0xFF},
	success:     color.NRGBA{R: 0x1E, G: 0x9A, B: 0x6A, A: 0xFF},
	warning:     color.NRGBA{R: 0xB7, G: 0x79, B: 0x1F, A: 0xFF},
	danger:      color.NRGBA{R: 0xD6, G: 0x3E, B: 0x45, A: 0xFF},
	scrollTrack: color.NRGBA{R: 0xE9, G: 0xEC, B: 0xF1, A: 0xFF},
	scrollThumb: color.NRGBA{R: 0xBF, G: 0xC6, B: 0xD2, A: 0xFF},
	selectMix:   0.2,
	tools: map[fyne.ThemeColorName]color.NRGBA{
		colorToolAdd:    {R: 0x1E, G: 0x9E, B: 0x78, A: 0xFF},
		colorToolRun:    {R: 0x3B, G: 0x7B, B: 0xEA, A: 0xFF},
		colorToolStop:   {R: 0x6B, G: 0x73, B: 0x85, A: 0xFF},
		colorToolDelete: {R: 0xD9, G: 0x46, B: 0x6F, A: 0xFF},
		colorToolConfig: {R: 0x6E, G: 0x5B, B: 0xD8, A: 0xFF},
		colorToolSched:  {R: 0xD0, G: 0x7A, B: 0x4F, A: 0xFF},
		colorToolQueue:  {R: 0x2E, G: 0x8F, B: 0xC4, A: 0xFF},
	},
}

// accents are the accent colours on offer; the first is the default. Each
// reads on both backgrounds and carries white text.
var accents = []struct {
	key, name string
	color     color.NRGBA
}{
	{"blue", markT("Blue"), color.NRGBA{R: 0x4F, G: 0x7C, B: 0xFF, A: 0xFF}},
	{"indigo", markT("Indigo"), color.NRGBA{R: 0x63, G: 0x66, B: 0xF1, A: 0xFF}},
	{"purple", markT("Purple"), color.NRGBA{R: 0x9B, G: 0x5D, B: 0xE5, A: 0xFF}},
	{"pink", markT("Pink"), color.NRGBA{R: 0xDE, G: 0x4F, B: 0x98, A: 0xFF}},
	{"red", markT("Red"), color.NRGBA{R: 0xE0, G: 0x48, B: 0x4E, A: 0xFF}},
	{"orange", markT("Orange"), color.NRGBA{R: 0xE2, G: 0x74, B: 0x2A, A: 0xFF}},
	{"green", markT("Green"), color.NRGBA{R: 0x2C, G: 0xA0, B: 0x6A, A: 0xFF}},
	{"teal", markT("Teal"), color.NRGBA{R: 0x1C, G: 0x9C, B: 0xAB, A: 0xFF}},
}

func accentColor(key string) color.NRGBA {
	for _, a := range accents {
		if a.key == key {
			return a.color
		}
	}
	return accents[0].color
}

// Theme modes: follow the system's light or dark setting, or keep one.
const (
	themeSystem = "system"
	themeLight  = "light"
	themeDark   = "dark"
)

const (
	prefTheme  = "appearance.theme"
	prefAccent = "appearance.accent"
)

// themeModes are the choices of Options → General → Theme, in order.
var themeModes = []string{themeSystem, themeLight, themeDark}

func themeChoices() []string {
	system := T("Same as the system")
	if runtime.GOOS == "windows" {
		system = T("Same as Windows")
	}
	return []string{system, T("Light"), T("Dark")}
}

func themeIndex(mode string) int {
	for i, m := range themeModes {
		if m == mode {
			return i
		}
	}
	return 0
}

// odmTheme is a flat, rounded look over Fyne's default theme, in the light
// or dark palette and an accent colour.
type odmTheme struct {
	mode   string // themeSystem, themeLight or themeDark
	accent color.NRGBA
}

// newTheme is the default look: the system's light or dark, blue accent.
func newTheme() fyne.Theme { return themeFor(themeSystem, "") }

func themeFor(mode, accent string) fyne.Theme {
	return odmTheme{mode: mode, accent: accentColor(accent)}
}

// applyTheme sets the look saved in the preferences.
func (a *application) applyTheme() {
	p := a.fyneApp.Preferences()
	a.fyneApp.Settings().SetTheme(themeFor(p.StringWithFallback(prefTheme, themeSystem), p.String(prefAccent)))
}

// variant is the palette this theme draws with for the system's variant v.
func (t odmTheme) variant(v fyne.ThemeVariant) fyne.ThemeVariant {
	switch t.mode {
	case themeLight:
		return theme.VariantLight
	case themeDark:
		return theme.VariantDark
	}
	return v
}

func (t odmTheme) palette(v fyne.ThemeVariant) *palette {
	if t.variant(v) == theme.VariantLight {
		return &lightPalette
	}
	return &darkPalette
}

// mix blends a over b, frac parts of a.
func mix(a, b color.NRGBA, frac float64) color.NRGBA {
	m := func(x, y uint8) uint8 { return uint8(float64(x)*frac + float64(y)*(1-frac) + 0.5) }
	return color.NRGBA{R: m(a.R, b.R), G: m(a.G, b.G), B: m(a.B, b.B), A: 0xFF}
}

func (t odmTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	p := t.palette(variant)
	if c, ok := p.tools[name]; ok {
		return c
	}
	switch name {
	case theme.ColorNameBackground:
		return p.background
	case theme.ColorNameOverlayBackground, theme.ColorNameHeaderBackground, theme.ColorNameMenuBackground:
		return p.surface
	case theme.ColorNameInputBackground, theme.ColorNameButton:
		return p.surfaceAlt
	case theme.ColorNamePrimary, theme.ColorNameHyperlink:
		return t.accent
	case theme.ColorNameSelection:
		return mix(t.accent, p.background, p.selectMix)
	case theme.ColorNameHover:
		return p.hover
	case theme.ColorNamePressed:
		return p.pressed
	case theme.ColorNameFocus:
		return color.NRGBA{R: t.accent.R, G: t.accent.G, B: t.accent.B, A: 0x88}
	case theme.ColorNameSeparator, theme.ColorNameInputBorder:
		return p.border
	case theme.ColorNameScrollBar:
		return p.scrollThumb
	case theme.ColorNameScrollBarBackground:
		return p.scrollTrack
	case theme.ColorNameForeground:
		return p.foreground
	case theme.ColorNamePlaceHolder, theme.ColorNameDisabled:
		return p.muted
	case theme.ColorNameSuccess:
		return p.success
	case theme.ColorNameWarning:
		return p.warning
	case theme.ColorNameError:
		return p.danger
	case theme.ColorNameForegroundOnPrimary:
		return color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	}
	return theme.DefaultTheme().Color(name, t.variant(variant))
}

func (t odmTheme) Font(s fyne.TextStyle) fyne.Resource { return theme.DefaultTheme().Font(s) }

func (t odmTheme) Icon(n fyne.ThemeIconName) fyne.Resource { return theme.DefaultTheme().Icon(n) }

func (t odmTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 3
	case theme.SizeNameInnerPadding:
		return 4
	case theme.SizeNameInputRadius, theme.SizeNameButtonRadius:
		return 5
	case theme.SizeNameSelectionRadius:
		return 4
	case theme.SizeNameCardRadius:
		return 8
	case theme.SizeNameDialogRadius, theme.SizeNamePopupRadius, theme.SizeNameMenuRadius:
		return 10
	case theme.SizeNameInputBorder:
		return 1
	case theme.SizeNameSeparatorThickness:
		return 1
	case theme.SizeNameScrollBar:
		return 8
	case theme.SizeNameScrollBarSmall:
		return 4
	case theme.SizeNameScrollBarRadius:
		return 4
	case theme.SizeNameText:
		return 12
	case theme.SizeNameSubHeadingText:
		return 13
	case theme.SizeNameHeadingText:
		return 16
	case theme.SizeNameCaptionText:
		return 10
	case theme.SizeNameInlineIcon:
		return 16
	case theme.SizeNameLineSpacing:
		return 2
	}
	return theme.DefaultTheme().Size(name)
}

// Toolbar icon tints, resolved through odmTheme.Color so they follow the
// palette.
const (
	colorToolAdd    fyne.ThemeColorName = "odm-tool-add"
	colorToolRun    fyne.ThemeColorName = "odm-tool-run"
	colorToolStop   fyne.ThemeColorName = "odm-tool-stop"
	colorToolDelete fyne.ThemeColorName = "odm-tool-delete"
	colorToolConfig fyne.ThemeColorName = "odm-tool-config"
	colorToolSched  fyne.ThemeColorName = "odm-tool-sched"
	colorToolQueue  fyne.ThemeColorName = "odm-tool-queue"
)

// compactTheme tightens padding for the category tree so its rows sit at
// about 20px, like a classic tree view, without shrinking dialogs and forms
// that use the base theme. Everything else comes from the app's theme of
// the moment, so a change of look reaches the tree too.
type compactTheme struct{}

func (compactTheme) base() fyne.Theme { return fyne.CurrentApp().Settings().Theme() }

func (t compactTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	return t.base().Color(n, v)
}

func (t compactTheme) Font(s fyne.TextStyle) fyne.Resource { return t.base().Font(s) }

func (t compactTheme) Icon(n fyne.ThemeIconName) fyne.Resource { return t.base().Icon(n) }

func (t compactTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 1
	case theme.SizeNameInnerPadding:
		return 2
	case theme.SizeNameLineSpacing:
		return 1
	case theme.SizeNameInlineIcon:
		return 15
	}
	return t.base().Size(name)
}
