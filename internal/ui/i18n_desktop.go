//go:build fyne

package ui

import (
	"embed"
	"encoding/json"
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/lang"
)

// The desktop UI is written in English; translations/id.json holds its text
// in Indonesian, keyed by the English. fyne.id.json translates the text of
// Fyne's own widgets (dialog buttons, the file dialog, the entry menu).
//
//go:embed translations/*.json
var translationFS embed.FS

// Languages, as stored in the preferences.
const (
	langSystem     = "system"
	langEnglish    = "en"
	langIndonesian = "id"
)

const prefLanguage = "appearance.language"

// languageModes are the choices of Options → General → Language, in order.
var languageModes = []string{langSystem, langEnglish, langIndonesian}

func languageChoices() []string {
	return []string{T("Same as the system"), "English", "Bahasa Indonesia"}
}

func languageIndex(mode string) int {
	for i, m := range languageModes {
		if m == mode {
			return i
		}
	}
	return 0
}

// translations is the text of the language in use, nil for English.
var translations map[string]string

// T is s in the language in use.
func T(s string) string {
	if t, ok := translations[s]; ok && t != "" {
		return t
	}
	return s
}

// markT marks s for translation where T runs later, on a value (a table
// filled before the language is chosen): the translation test finds the
// text through it.
func markT(s string) string { return s }

// Tf formats the translation of format, whose verbs the translation keeps.
func Tf(format string, a ...any) string { return fmt.Sprintf(T(format), a...) }

// resolveLanguage is the language for a saved choice: the system's own
// when that is Indonesian and the choice is to follow it, else English.
func resolveLanguage(choice string, system fyne.Locale) string {
	switch choice {
	case langEnglish, langIndonesian:
		return choice
	}
	// "id-ID", "id_ID" or "id"; "in" is the old code for Indonesian.
	base, _, _ := strings.Cut(strings.ReplaceAll(string(system), "_", "-"), "-")
	if base = strings.ToLower(base); base == langIndonesian || base == "in" {
		return langIndonesian
	}
	return langEnglish
}

// setLanguage switches the UI text to lang. It runs before any window is
// built: text already on screen keeps its language.
func setLanguage(language string) error {
	translations = nil
	if language != langIndonesian {
		return nil
	}
	m, err := readTranslations("translations/id.json")
	if err != nil {
		return err
	}
	translations = m
	// Fyne's widgets ask its own localizer, which follows the system
	// locale and cannot be told otherwise. Giving the Indonesian text to
	// the languages it may pick (English, which it falls back to,
	// Indonesian and the system's) makes them speak Indonesian too.
	data, err := translationFS.ReadFile("translations/fyne.id.json")
	if err != nil {
		return err
	}
	for _, l := range []fyne.Locale{"en", "id", lang.SystemLocale()} {
		if err := lang.AddTranslationsForLocale(data, l); err != nil {
			return err
		}
	}
	return nil
}

func readTranslations(name string) (map[string]string, error) {
	data, err := translationFS.ReadFile(name)
	if err != nil {
		return nil, err
	}
	m := map[string]string{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return m, nil
}

// applyLanguage sets the language saved in the preferences.
func (a *application) applyLanguage() {
	choice := a.fyneApp.Preferences().StringWithFallback(prefLanguage, langSystem)
	if err := setLanguage(resolveLanguage(choice, lang.SystemLocale())); err != nil {
		fyne.LogError("loading the translations", err)
	}
}
