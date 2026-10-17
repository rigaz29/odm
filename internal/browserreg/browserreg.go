// Package browserreg registers ODM as the native-messaging host for its
// browser extension, the step that otherwise means hand-editing a JSON
// manifest and the registry.
//
// Locations follow the browsers' documentation: on Windows a per-user
// registry key whose default value is the manifest path; on macOS and Linux
// the manifest file itself in each browser's NativeMessagingHosts folder.
package browserreg

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// HostName is the native host name the extension connects to.
const HostName = "com.ochi.odm"

// ChromeExtensionID is fixed by the "key" in extension/manifest.json (see
// TestChromeIDMatchesManifestKey). Chrome's allowed_origins takes no
// wildcards, so the unpacked extension must keep this ID.
const ChromeExtensionID = "fnondfibgeiffjplpokfgiacjelmbcbk"

// ChromiumExtensionIDs are every ID the Chromium-family extension can have:
// the pinned unpacked ID, plus each store's ID once it is published there
// (stores assign their own). The host manifest allows all of them, as a
// browser reads only the first registered host manifest it finds.
var ChromiumExtensionIDs = []string{ChromeExtensionID}

// FirefoxExtensionID is browser_specific_settings.gecko.id in the manifest:
// this fork's add-on, which Mozilla signs on its AMO account.
const FirefoxExtensionID = "odm@rigaz29.github.io"

// FirefoxExtensionIDs are every ID the Firefox add-on can have,
// FirefoxExtensionID first, then odm@troke.id: the add-on of the project
// this fork comes from (signed on that project's AMO account), which may
// be installed already. The host manifest allows them all.
var FirefoxExtensionIDs = []string{FirefoxExtensionID, "odm@troke.id"}

// Result describes one browser registration.
type Result struct {
	Browser  string
	Location string // registry key or manifest file
	Err      error
	Skipped  bool // browser not installed for this user
}

func (r Result) String() string {
	switch {
	case r.Skipped:
		return fmt.Sprintf("%s: not found, skipped", r.Browser)
	case r.Err != nil:
		return fmt.Sprintf("%s: %v", r.Browser, r.Err)
	default:
		return fmt.Sprintf("%s: %s", r.Browser, r.Location)
	}
}

type manifest struct {
	Name              string   `json:"name"`
	Description       string   `json:"description"`
	Path              string   `json:"path"`
	Type              string   `json:"type"`
	AllowedOrigins    []string `json:"allowed_origins,omitempty"`
	AllowedExtensions []string `json:"allowed_extensions,omitempty"`
}

// writeManifests writes the Chromium-family and Firefox host manifests for
// exe into dir and returns their paths. Firefox rejects allowed_origins and
// Chrome rejects allowed_extensions, hence two files.
func writeManifests(dir, exe string) (chromium, firefox string, err error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", err
	}
	base := manifest{Name: HostName, Description: "Ochi Download Manager", Path: exe, Type: "stdio"}
	c := base
	for _, id := range ChromiumExtensionIDs {
		c.AllowedOrigins = append(c.AllowedOrigins, "chrome-extension://"+id+"/")
	}
	f := base
	f.AllowedExtensions = append([]string(nil), FirefoxExtensionIDs...)
	chromium = filepath.Join(dir, HostName+".json")
	firefox = filepath.Join(dir, HostName+".firefox.json")
	for path, m := range map[string]manifest{chromium: c, firefox: f} {
		data, _ := json.MarshalIndent(m, "", "  ")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return "", "", err
		}
	}
	return chromium, firefox, nil
}

// Register writes the manifests under configDir/native and points every
// supported browser found for this user at them. exe must be absolute.
func Register(exe, configDir string) []Result {
	chromium, firefox, err := writeManifests(filepath.Join(configDir, "native"), exe)
	if err != nil {
		return []Result{{Browser: "manifest", Err: err}}
	}
	return register(chromium, firefox)
}

// Status is one browser's current registration.
type Status struct {
	Browser    string
	Installed  bool   // known only on macOS/Linux (profile folder); true on Windows
	Registered bool   // a host manifest for ODM is registered
	Exe        string // the executable that registration launches
}

// Points reports whether the registration launches exe.
func (s Status) Points(exe string) bool {
	return s.Registered && samePath(s.Exe, exe)
}

// NeedsRegister reports whether the ODM at exe should register itself when
// it starts: when no browser launches an ODM yet, when a registration
// launches a program that is gone (ODM was moved or deleted), or when this
// ODM is registered for some browsers but not all. A registration of
// another ODM that still exists is left alone: that copy was registered on
// purpose, and Options offers to switch. exists reports whether a program
// is still there.
func NeedsRegister(statuses []Status, exe string, exists func(string) bool) bool {
	need := false
	for _, s := range statuses {
		switch {
		case s.Points(exe):
		case s.Registered && exists(s.Exe):
			return false
		case s.Registered, s.Installed:
			need = true
		}
	}
	return need
}

// manifestExe reads the executable path from a host manifest file.
func manifestExe(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var m manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return "", err
	}
	return m.Path, nil
}

// Summary describes statuses from the point of view of exe: which browsers
// launch this ODM, which launch another copy (e.g. a release build after a
// test build re-registered), and which have no registration. Browsers that
// are not installed (known on macOS/Linux) are left out.
func Summary(statuses []Status, exe string) string {
	return SummaryWith(statuses, exe, func(s string) string { return s })
}

// SummaryTexts are the sentences of a summary, as format strings, for a
// caller that translates them.
var SummaryTexts = []string{
	"Registered for %s.",
	"%s uses another ODM: %s.",
	"%s use another ODM: %s.",
	"Not registered: %s.",
	"Not registered for: %s.",
	"No supported browser found.",
}

// SummaryWith is Summary with each sentence's format passed through tr.
func SummaryWith(statuses []Status, exe string, tr func(string) string) string {
	var here, missing []string
	elsewhere := map[string][]string{}
	var otherExes []string
	for _, s := range statuses {
		switch {
		case s.Points(exe):
			here = append(here, s.Browser)
		case s.Registered:
			if _, seen := elsewhere[s.Exe]; !seen {
				otherExes = append(otherExes, s.Exe)
			}
			elsewhere[s.Exe] = append(elsewhere[s.Exe], s.Browser)
		case s.Installed:
			missing = append(missing, s.Browser)
		}
	}
	var parts []string
	if len(here) > 0 {
		parts = append(parts, fmt.Sprintf(tr("Registered for %s."), strings.Join(here, ", ")))
	}
	for _, other := range otherExes {
		format := "%s use another ODM: %s."
		if len(elsewhere[other]) == 1 {
			format = "%s uses another ODM: %s."
		}
		parts = append(parts, fmt.Sprintf(tr(format), strings.Join(elsewhere[other], ", "), other))
	}
	if len(missing) > 0 {
		format := "Not registered: %s."
		if len(here) > 0 || len(otherExes) > 0 {
			format = "Not registered for: %s."
		}
		parts = append(parts, fmt.Sprintf(tr(format), strings.Join(missing, ", ")))
	}
	if len(parts) == 0 {
		return tr("No supported browser found.")
	}
	return strings.Join(parts, " ")
}

// Check reports each supported browser's registration.
func Check() []Status { return check() }

// Unregister removes what Register added (the manifests under configDir are
// left; they are inert without the registrations).
func Unregister() []Result { return unregister() }
