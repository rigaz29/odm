package gui

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"odm/extension"
	"odm/internal/browsers"
	"odm/internal/engine"
	"odm/internal/tools"
	"odm/internal/version"
)

// setupService is a Service with made-up browsers, whose launches are
// recorded, and ODM's folders in a temporary one.
func setupService(t *testing.T, list ...browsers.Browser) (*Service, *[]string) {
	t.Helper()
	t.Setenv("ODM_TOOLS_DIR", filepath.Join(t.TempDir(), "odm", "tools"))
	svc, _ := newTestService(t)
	var mu sync.Mutex
	var launched []string
	svc.installed = func() []browsers.Browser { return list }
	svc.launch = func(b browsers.Browser, args ...string) error {
		mu.Lock()
		defer mu.Unlock()
		launched = append(launched, strings.TrimSpace(b.Key+" "+strings.Join(args, " ")))
		return nil
	}
	return svc, &launched
}

var (
	chrome  = browsers.Browser{Key: "chrome", Name: "Google Chrome", Family: browsers.Chromium}
	edge    = browsers.Browser{Key: "edge", Name: "Microsoft Edge", Family: browsers.Chromium}
	firefox = browsers.Browser{Key: "firefox", Name: "Mozilla Firefox", Family: browsers.Firefox}
)

func TestSetupBrowsersConnected(t *testing.T) {
	svc, _ := setupService(t, chrome, edge, firefox)
	st := svc.mgr.Settings()
	st.ExtensionBrowsers = []string{"edge", "chromium"}
	svc.mgr.UpdateSettings(st)
	got := map[string]bool{}
	for _, b := range svc.SetupBrowsers() {
		got[b.Key] = b.Connected
	}
	// "chromium" (an extension that did not say which browser) cannot
	// be told apart between Chrome and Edge.
	if !got["edge"] || got["chrome"] || got["firefox"] || len(got) != 3 {
		t.Fatalf("connected: %v", got)
	}

	only, _ := setupService(t, chrome, firefox)
	st = only.mgr.Settings()
	st.ExtensionBrowsers = []string{"chromium"}
	only.mgr.UpdateSettings(st)
	if b := only.SetupBrowsers(); !b[0].Connected || b[1].Connected {
		t.Fatalf("the only Chromium browser: %+v", b)
	}
}

// Where ODM cannot find the browsers (outside Windows), the Setup window
// points to the releases page for the extension.
func TestSetupStatusReleasesPage(t *testing.T) {
	svc, _ := setupService(t)
	got := svc.SetupStatus().ReleasesPage
	want := "https://github.com/" + version.Repo + "/releases/latest"
	if browsers.Listed {
		want = ""
	}
	if got != want {
		t.Fatalf("releases page %q, want %q", got, want)
	}
}

// A Chromium browser gets the extension written to a folder for "Load
// unpacked", its extensions page to paste, and a window.
func TestInstallExtensionChromium(t *testing.T) {
	svc, launched := setupService(t, chrome, edge)
	step, err := svc.InstallExtension("edge")
	if err != nil {
		t.Fatal(err)
	}
	if step.Page != "edge://extensions" || step.Folder != extensionDir() {
		t.Fatalf("step: %+v", step)
	}
	want, _ := extension.Files.ReadFile("background.js")
	if got, err := os.ReadFile(filepath.Join(step.Folder, "background.js")); err != nil || string(got) != string(want) {
		t.Fatalf("background.js not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(step.Folder, "manifest.json")); err != nil {
		t.Fatal(err)
	}
	if strings.Join(*launched, ",") != "edge" {
		t.Fatalf("launched: %v", *launched)
	}
	if _, err := svc.InstallExtension("opera"); err == nil {
		t.Fatal("a browser that is not installed was accepted")
	}
}

// Firefox gets the signed add-on of the release, or of the newest one
// when that release has none, opened in it.
func TestInstallExtensionFirefox(t *testing.T) {
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.Path)
		if r.URL.Path == "/"+version.Repo+"/releases/latest/download/"+firefoxAddon {
			_, _ = w.Write([]byte("signed add-on"))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	old := releaseBase
	releaseBase = srv.URL + "/"
	defer func() { releaseBase = old }()

	svc, launched := setupService(t, firefox)
	step, err := svc.InstallExtension("firefox")
	if err != nil {
		t.Fatal(err)
	}
	if step != (ExtensionStep{}) {
		t.Fatalf("step: %+v", step)
	}
	file := filepath.Join(filepath.Dir(extensionDir()), firefoxAddon)
	if data, err := os.ReadFile(file); err != nil || string(data) != "signed add-on" {
		t.Fatalf("add-on: %q %v", data, err)
	}
	if strings.Join(*launched, ",") != "firefox "+file {
		t.Fatalf("launched: %v", *launched)
	}
	if len(asked) != 2 || !strings.Contains(asked[0], "/download/v"+version.Version+"/") {
		t.Fatalf("asked: %v", asked)
	}
}

// The Setup window opens by itself once, never for a start in the tray,
// and the extension written for "Load unpacked" follows ODM's release.
func TestSetupAtStart(t *testing.T) {
	svc, _ := setupService(t)
	w := &stubWindows{}
	svc.windows = w
	svc.setupAtStart(true, false)
	if w.got() != "" || svc.mgr.Settings().SetupDone {
		t.Fatalf("offered for a start in the tray: %q", w.got())
	}
	svc.setupAtStart(true, true)
	if w.got() != "setup" || !svc.mgr.Settings().SetupDone {
		t.Fatalf("first start: %q", w.got())
	}
	svc.setupAtStart(true, true)
	if w.got() != "setup" {
		t.Fatalf("offered again: %q", w.got())
	}

	dir, err := writeExtension()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "popup.js"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	svc.setupAtStart(false, true)
	want, _ := extension.Files.ReadFile("popup.js")
	if got, _ := os.ReadFile(filepath.Join(dir, "popup.js")); string(got) != string(want) {
		t.Fatal("the extension folder was not brought up to date")
	}
}

func TestToolQuestions(t *testing.T) {
	if !tools.Libtorrent().Supported() || !tools.YtDlp().Supported() {
		t.Skip("no companion programs for this system")
	}
	svc, _ := newTestService(t)
	w := &stubWindows{}
	svc.windows = w
	mode := tools.ModeAsk
	k := tools.NewKeeper(&tools.Installer{Dir: t.TempDir()}, func() string { return mode })
	(&desk{svc: svc}).hookTools(k)
	wait := func(want string) {
		t.Helper()
		for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
			if strings.Contains(w.got(), want) {
				return
			}
		}
		t.Fatalf("asked %q, not %q", w.got(), want)
	}
	// yt-dlp and ffmpeg share a question; a torrent asks about
	// odm-libtorrent, even after yt-dlp and ffmpeg were declined.
	k.Ensure(context.Background(), "yt-dlp")
	wait("notice kind=tools")
	mode = tools.ModeOff
	if _, err := k.Ensure(context.Background(), "odm-libtorrent"); !errors.Is(err, tools.ErrNeedsInstall) {
		t.Fatalf("odm-libtorrent: %v", err)
	}
	wait("notice kind=program&name=odm-libtorrent")
}

func TestWaitsForProgram(t *testing.T) {
	for _, c := range []struct {
		d    engine.Info
		want bool
	}{
		{engine.Info{Torrent: true, Status: engine.StatusError, Error: "odm-libtorrent: " + tools.ErrNeedsInstall.Error()}, true},
		{engine.Info{Torrent: true, Status: engine.StatusError, Error: "odm-libtorrent stopped (exit status 3)"}, true},
		{engine.Info{Torrent: true, Status: engine.StatusError, Error: "reading the torrent: unexpected EOF"}, false},
		{engine.Info{Torrent: true, Status: engine.StatusPaused, Error: ""}, false},
		{engine.Info{Status: engine.StatusError, Error: "odm-libtorrent: x"}, false},
	} {
		if got := waitsForProgram(c.d); got != c.want {
			t.Errorf("%+v: %v", c.d, got)
		}
	}
}
