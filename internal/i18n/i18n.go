// Package i18n holds ODM's translations, keyed by the English text: id.json
// is Indonesian. Both desktop UIs read it: the Fyne UI (internal/ui) when
// it starts, the new UI's page (internal/gui/frontend) when it is built.
// The tests here check that every text either UI shows has a translation
// and that the catalog holds nothing neither uses.
package i18n

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"
)

//go:embed id.json
var idJSON []byte

var (
	once       sync.Once
	indonesian map[string]string
	loadErr    error
)

// Indonesian returns the Indonesian text of every English one. The map is
// shared: callers only read it.
func Indonesian() (map[string]string, error) {
	once.Do(func() {
		m := map[string]string{}
		if err := json.Unmarshal(idJSON, &m); err != nil {
			loadErr = fmt.Errorf("i18n: id.json: %w", err)
			return
		}
		indonesian = m
	})
	return indonesian, loadErr
}
