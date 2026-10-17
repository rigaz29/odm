// Package browsers finds the web browsers installed for this user, for the
// Setup window's extension step: which ones, the family of each (how the
// extension is installed in it) and the program that opens it.
package browsers

import (
	"os/exec"
	"sort"
	"strings"
)

// Families: how ODM's extension is installed in a browser.
const (
	Chromium = "chromium" // "Load unpacked" on its extensions page
	Firefox  = "firefox"  // Mozilla's signed add-on (.xpi)
)

// Browser is one installed browser.
type Browser struct {
	// Key names it for ODM ("chrome", "edge", "brave", "opera", "vivaldi",
	// "chromium", "firefox", "zen", ...); the extension reports the same
	// names (background.js, browserName).
	Key    string `json:"key"`
	Name   string `json:"name"` // as the system lists it: "Google Chrome"
	Exe    string `json:"exe"`
	Family string `json:"family"`
}

type kind struct{ key, family, name string }

// known are the browsers ODM tells apart, by program file.
var known = map[string]kind{
	"chrome.exe":    {"chrome", Chromium, "Google Chrome"},
	"msedge.exe":    {"edge", Chromium, "Microsoft Edge"},
	"brave.exe":     {"brave", Chromium, "Brave"},
	"opera.exe":     {"opera", Chromium, "Opera"},
	"vivaldi.exe":   {"vivaldi", Chromium, "Vivaldi"},
	"firefox.exe":   {"firefox", Firefox, "Mozilla Firefox"},
	"zen.exe":       {"zen", Firefox, "Zen Browser"},
	"librewolf.exe": {"librewolf", Firefox, "LibreWolf"},
	"waterfox.exe":  {"waterfox", Firefox, "Waterfox"},
	"floorp.exe":    {"floorp", Firefox, "Floorp"},
}

// identify makes a Browser of a system's entry: its display name and the
// command that opens it ("\"C:\\...\\chrome.exe\""). ok is false for a
// browser the extension does not run in (Internet Explorer).
func identify(name, command string) (Browser, bool) {
	exe := program(command)
	base := strings.ToLower(exe[strings.LastIndexAny(exe, `\/`)+1:])
	k, ok := known[base]
	switch {
	case base == "chrome.exe" && strings.Contains(strings.ToLower(name), "chromium"):
		k, ok = kind{"chromium", Chromium, "Chromium"}, true
	case base == "launcher.exe" && strings.Contains(strings.ToLower(name), "opera"):
		k, ok = known["opera.exe"], true // Opera starts through its launcher
	}
	if !ok {
		return Browser{}, false
	}
	if name = strings.TrimSpace(name); name == "" {
		name = k.name
	}
	return Browser{Key: k.key, Name: name, Exe: exe, Family: k.family}, true
}

// program is the program a command line starts: the quoted path, or the
// text up to ".exe" (paths have spaces), or the first word.
func program(command string) string {
	c := strings.TrimSpace(command)
	if rest, ok := strings.CutPrefix(c, `"`); ok {
		p, _, _ := strings.Cut(rest, `"`)
		return p
	}
	if i := strings.Index(strings.ToLower(c), ".exe"); i >= 0 {
		return c[:i+len(".exe")]
	}
	p, _, _ := strings.Cut(c, " ")
	return p
}

// unique keeps the first browser of each key, in name order.
func unique(list []Browser) []Browser {
	seen := map[string]bool{}
	var out []Browser
	for _, b := range list {
		if !seen[b.Key] {
			seen[b.Key] = true
			out = append(out, b)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out
}

// ExtensionsPage is the address of a Chromium browser's extensions page,
// where "Load unpacked" is; "" for Firefox. Browsers do not open their own
// pages from a command line, so it is for the person to paste.
func (b Browser) ExtensionsPage() string {
	if b.Family != Chromium {
		return ""
	}
	switch b.Key {
	case "edge", "brave", "opera", "vivaldi":
		return b.Key + "://extensions"
	}
	return "chrome://extensions"
}

// Open starts the browser with args (an address or a file to open) and
// does not wait for it.
func (b Browser) Open(args ...string) error {
	cmd := exec.Command(b.Exe, args...)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
