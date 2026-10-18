package clipmon

import (
	"net/url"
	"path"
	"regexp"
	"strings"

	"odm/internal/engine"
)

var (
	urlRe    = regexp.MustCompile(`https?://[^\s"'<>()\[\]]+`)
	magnetRe = regexp.MustCompile(`(?i)magnet:\?[^\s"'<>]+`)
)

// ClipboardURL returns the first magnet link in text, else the first
// HTTP(S) URL in it whose path names a file type ODM recognises as
// downloadable, built in or listed in extra.
func ClipboardURL(text string, extra ...string) (string, bool) {
	if m := magnetRe.FindString(text); m != "" && strings.Contains(strings.ToLower(m), "xt=urn:bt") {
		return m, true
	}
	for _, raw := range urlRe.FindAllString(text, -1) {
		raw = strings.TrimRight(raw, ".,;:)")
		u, err := url.Parse(raw)
		if err != nil || u.Host == "" {
			continue
		}
		if !engine.KnownFileType(path.Base(u.Path), extra...) {
			continue
		}
		return raw, true
	}
	return "", false
}
