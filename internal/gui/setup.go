package gui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"odm/extension"
	"odm/internal/browsers"
	"odm/internal/extpack"
	"odm/internal/osutil"
	"odm/internal/tools"
	"odm/internal/version"
)

// The Setup window (?view=setup): yt-dlp and ffmpeg, and ODM's extension
// in each browser. It opens by itself once (setupAtStart) and from the
// menu (OpenSetup).

// SetupStatus is what the Setup window shows.
type SetupStatus struct {
	Tools []SetupTool `json:"tools"`
	// ToolMode is Settings.ToolUpdates: "" (not asked yet), "auto" or "off".
	ToolMode string         `json:"toolMode"`
	Browsers []SetupBrowser `json:"browsers"`
}

// SetupTool is yt-dlp or ffmpeg as the Setup window lists it: State and
// the rest as ToolStatus gives them.
type SetupTool struct {
	Name    string `json:"name"`
	State   string `json:"state"`
	Path    string `json:"path"`
	Version string `json:"version"`
	// CanInstall: ODM can download it on this system.
	CanInstall bool `json:"canInstall"`
}

// SetupBrowser is an installed browser as the Setup window lists it.
type SetupBrowser struct {
	Key    string `json:"key"`
	Name   string `json:"name"`
	Family string `json:"family"` // "chromium" or "firefox"
	// Connected: its extension has talked to this ODM.
	Connected bool `json:"connected"`
}

// ExtensionStep is what the Setup window needs to finish installing the
// extension in a Chromium browser; empty for Firefox, which asks by itself.
type ExtensionStep struct {
	// Page is the browser's extensions page, for its address bar.
	Page string `json:"page"`
	// Folder holds the extension, for "Load unpacked" there.
	Folder string `json:"folder"`
}

// SetupProgressEvent carries a tool download's progress to the Setup
// window.
const SetupProgressEvent = "odm:setup-progress"

// SetupProgress is how far the download of Tool is: bytes so far and the
// total (0 when unknown).
type SetupProgress struct {
	Tool  string `json:"tool"`
	Done  int64  `json:"done"`
	Total int64  `json:"total"`
}

// releaseBase is where ODM's releases are downloaded from; tests point it
// elsewhere.
var releaseBase = "https://github.com/"

// firefoxAddon is the release asset with the add-on Mozilla signed.
const firefoxAddon = "odm-extension-firefox.xpi"

// SetupStatus reports the tools and the browsers' extensions.
func (s *Service) SetupStatus() SetupStatus {
	st := s.mgr.Settings()
	out := SetupStatus{ToolMode: st.ToolUpdates, Browsers: s.SetupBrowsers()}
	for _, t := range tools.All() {
		out.Tools = append(out.Tools, s.setupTool(t.Name))
	}
	return out
}

func (s *Service) setupTool(name string) SetupTool {
	st := s.mgr.Settings()
	typed := st.YtDlpPath
	if name == "ffmpeg" {
		typed = st.FFmpegPath
	}
	ts := s.ToolStatus(name, typed)
	out := SetupTool{Name: name, State: ts.State, Path: ts.Path, Version: ts.Version}
	for _, t := range tools.All() {
		if t.Name == name {
			out.CanInstall = t.Supported() && s.tools() != nil
		}
	}
	return out
}

// SetupBrowsers lists the installed browsers and whether each one's
// extension has talked to ODM. The Setup window asks again every few
// seconds, so a browser turns connected once the extension is in.
func (s *Service) SetupBrowsers() []SetupBrowser {
	seen := s.mgr.Settings().ExtensionBrowsers
	list := s.installed()
	chromiums := 0
	for _, b := range list {
		if b.Family == browsers.Chromium {
			chromiums++
		}
	}
	out := []SetupBrowser{}
	for _, b := range list {
		// An extension that did not say which browser it runs in counts
		// for the only Chromium browser there is.
		on := slices.Contains(seen, b.Key) ||
			b.Family == browsers.Chromium && chromiums == 1 && slices.Contains(seen, "chromium")
		out = append(out, SetupBrowser{Key: b.Key, Name: b.Name, Family: b.Family, Connected: on})
	}
	return out
}

// InstallTool downloads yt-dlp or ffmpeg into ODM's folder now, as the
// Setup window asks, and returns how it stands after.
func (s *Service) InstallTool(name string) (SetupTool, error) {
	k := s.tools()
	if k == nil {
		return SetupTool{}, errNoTools
	}
	s.mu.Lock()
	emit := s.emit
	s.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	var last time.Time
	_, err := k.Install(ctx, name, func(done, total int64) {
		// A few times a second is enough for a bar.
		if now := time.Now(); emit != nil && (now.Sub(last) >= 150*time.Millisecond || done == total) {
			last = now
			emit(SetupProgressEvent, SetupProgress{Tool: name, Done: done, Total: total})
		}
	})
	if err != nil {
		return SetupTool{}, err
	}
	return s.setupTool(name), nil
}

// InstallExtension starts installing ODM's extension in the browser key
// names. Firefox gets the add-on Mozilla signed, opened in it, and asks
// to add it. A Chromium browser takes an extension from outside its store
// only through "Load unpacked": ODM writes the extension to a folder and
// opens the browser, and the person loads the folder on its extensions
// page, which the browser does not open from outside.
func (s *Service) InstallExtension(key string) (ExtensionStep, error) {
	var b browsers.Browser
	for _, c := range s.installed() {
		if c.Key == key {
			b = c
		}
	}
	if b.Key == "" {
		return ExtensionStep{}, fmt.Errorf("%s is not installed", key)
	}
	if b.Family == browsers.Firefox {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		file, err := s.downloadAddon(ctx)
		if err != nil {
			return ExtensionStep{}, err
		}
		return ExtensionStep{}, s.launch(b, file)
	}
	dir, err := writeExtension()
	if err != nil {
		return ExtensionStep{}, err
	}
	return ExtensionStep{Page: b.ExtensionsPage(), Folder: dir}, s.launch(b)
}

// ShowExtensionFolder opens the folder of the extension ODM wrote, which
// can also be dragged onto a browser's extensions page.
func (s *Service) ShowExtensionFolder() error {
	dir, err := writeExtension()
	if err != nil {
		return err
	}
	return osutil.Open(dir)
}

// OpenSetup opens the Setup window.
func (s *Service) OpenSetup() {
	if w := s.opener(); w != nil {
		w.openSetup()
	}
}

// extensionDir is where ODM writes its Chromium extension for "Load
// unpacked", next to its copies of yt-dlp and ffmpeg.
func extensionDir() string {
	return filepath.Join(filepath.Dir(tools.Dir()), "extension", "chromium")
}

// writeExtension writes the extension ODM carries to extensionDir, with
// this release's version, and returns the folder.
func writeExtension() (string, error) {
	dir := extensionDir()
	v, err := extpack.ManifestVersion(version.Version)
	if err != nil {
		v = "" // not a release: the source's own version
	}
	_, err = extpack.Unpack(extension.Files, extpack.Chromium, dir, v)
	return dir, err
}

// downloadAddon fetches the signed Firefox add-on of this release (the
// newest release's, for a build that is not one) next to the Chromium
// extension and returns its path. Firefox checks Mozilla's signature when
// it installs it.
func (s *Service) downloadAddon(ctx context.Context) (string, error) {
	base := releaseBase + version.Repo + "/releases/"
	var urls []string
	if v, err := extpack.ManifestVersion(version.Version); err == nil && v == strings.TrimPrefix(version.Version, "v") {
		urls = append(urls, base+"download/v"+v+"/"+firefoxAddon)
	}
	urls = append(urls, base+"latest/download/"+firefoxAddon)
	file := filepath.Join(filepath.Dir(extensionDir()), firefoxAddon)
	var errs []error
	for _, u := range urls {
		err := s.fetchTo(ctx, u, file)
		if err == nil {
			return file, nil
		}
		errs = append(errs, err)
	}
	return "", fmt.Errorf("downloading the Firefox add-on: %w", errors.Join(errs...))
}

// fetchTo saves what url answers to file.
func (s *Service) fetchTo(ctx context.Context, url, file string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := s.mgr.HTTPClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", url, resp.Status)
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	tmp := file + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, io.LimitReader(resp.Body, 32<<20))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, file)
}

// setupAtStart offers the Setup window once: at ODM's first start, or to
// someone who used ODM before when yt-dlp or ffmpeg is missing. visible is
// false for a start in the tray, which leaves the offer for a later start.
// It also brings the extension ODM wrote for "Load unpacked" up to this
// release, so the browsers that loaded it get the new one when they
// restart.
func (s *Service) setupAtStart(firstRun, visible bool) {
	if _, err := os.Stat(filepath.Join(extensionDir(), "manifest.json")); err == nil {
		_, _ = writeExtension()
	}
	if !visible || s.mgr.Settings().SetupDone {
		return
	}
	show := firstRun
	if !show {
		for _, t := range tools.All() {
			if t.Supported() && s.setupTool(t.Name).State == "missing" {
				show = true
			}
		}
	}
	next := s.mgr.Settings()
	next.SetupDone = true
	s.mgr.UpdateSettings(next)
	if show {
		s.OpenSetup()
	}
}
