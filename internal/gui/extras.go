package gui

import (
	"context"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"odm/internal/engine"
	"odm/internal/osutil"
	"odm/internal/update"
	"odm/internal/version"
)

// What the Fyne UI's menus and its scheduler do besides windows of their
// own (internal/ui/desktop.go, dialogs_desktop.go): exporting and
// importing the list, the update check, the site grabber, About, and the
// scheduler's requests (open a file, the download limit reached, exit or
// shut down once the queues finished).

// ExportList saves the list to a file the person names; it returns the
// file, "" when cancelled.
func (s *Service) ExportList() (string, error) {
	w := s.opener()
	if w == nil {
		return "", nil
	}
	data, err := s.mgr.Export()
	if err != nil {
		return "", err
	}
	path := w.pickSave("odm-downloads.json")
	if path == "" {
		return "", nil
	}
	if !strings.HasSuffix(strings.ToLower(path), ".json") {
		path += ".json"
	}
	return path, os.WriteFile(path, data, 0o644)
}

// ImportList adds the downloads of a list exported before; it returns how
// many, -1 when cancelled.
func (s *Service) ImportList() (int, error) {
	w := s.opener()
	if w == nil {
		return -1, nil
	}
	path := w.pickOpen("ODM list (*.json)", "*.json")
	if path == "" {
		return -1, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	n, err := s.mgr.Import(data)
	s.poke()
	return n, err
}

// UpdateInfo is the newest release, against this ODM's version.
type UpdateInfo struct {
	Current string `json:"current"`
	Latest  string `json:"latest"`
	Newer   bool   `json:"newer"`
	URL     string `json:"url"`
	Notes   string `json:"notes"`
}

// UpdateEvent carries a newer release found at start-up to the main window.
const UpdateEvent = "odm:update"

// CheckUpdate asks GitHub for the newest release.
func (s *Service) CheckUpdate() (UpdateInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	rel, err := update.Latest(ctx, s.mgr.HTTPClient(), update.DefaultAPI, version.Repo)
	if err != nil {
		return UpdateInfo{Current: version.Version}, err
	}
	return UpdateInfo{Current: version.Version, Latest: rel.Tag, Newer: update.Newer(rel.Tag, version.Version), URL: rel.URL, Notes: rel.Notes}, nil
}

// checkAtStart tells the main window about a newer release, when the
// update check at start-up is on. It waits a little to stay off the start.
func (s *Service) checkAtStart(ctx context.Context) {
	if s.mgr.Settings().NoUpdateCheck {
		return
	}
	select {
	case <-ctx.Done():
		return
	case <-time.After(3 * time.Second):
	}
	if info, err := s.CheckUpdate(); err == nil && info.Newer {
		s.mu.Lock()
		emit := s.emit
		s.mu.Unlock()
		if emit != nil {
			emit(UpdateEvent, info)
		}
	}
}

// About is what the About dialog says.
type About struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Repo    string `json:"repo"`
	Runtime string `json:"runtime"` // Go version, system and architecture
}

func (s *Service) About() About {
	return About{Name: version.Name, Version: version.Version, Repo: version.Repo,
		Runtime: runtime.Version() + " · " + runtime.GOOS + "/" + runtime.GOARCH}
}

// OpenURL opens a web address in the browser.
func (s *Service) OpenURL(address string) error {
	u, err := url.Parse(address)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return errNotWeb
	}
	return osutil.Open(u.String())
}

// GrabRequest is the Site Grabber's form.
type GrabRequest struct {
	URL      string `json:"url"`
	Depth    int    `json:"depth"`
	MaxPages int    `json:"maxPages"`
	MaxFiles int    `json:"maxFiles"`
	SameHost bool   `json:"sameHost"`
	Start    bool   `json:"start"`
}

// GrabProgress is how far a crawl got: sent as GrabEvent while it runs,
// and returned when it is done.
type GrabProgress struct {
	Pages int `json:"pages"`
	Files int `json:"files"`
}

const GrabEvent = "odm:grab"

// grabbing is the crawl running now, to stop it.
var grabbing struct {
	sync.Mutex
	cancel context.CancelFunc
}

// Grab explores a site from a start page and adds the files it links to
// (the types in Options → File types). One crawl runs at a time.
func (s *Service) Grab(r GrabRequest) (GrabProgress, error) {
	addr := strings.TrimSpace(r.URL)
	if addr == "" {
		return GrabProgress{}, errNoAddress
	}
	ctx, cancel := context.WithCancel(context.Background())
	grabbing.Lock()
	if grabbing.cancel != nil {
		grabbing.Unlock()
		cancel()
		return GrabProgress{}, errGrabbing
	}
	grabbing.cancel = cancel
	grabbing.Unlock()
	defer func() {
		grabbing.Lock()
		grabbing.cancel = nil
		grabbing.Unlock()
		cancel()
	}()
	s.mu.Lock()
	emit := s.emit
	s.mu.Unlock()
	res, err := s.mgr.Grab(ctx, engine.GrabberOptions{
		URL: addr, Depth: max(0, r.Depth), MaxPages: max(1, r.MaxPages), MaxFiles: max(1, r.MaxFiles),
		SameHost: r.SameHost, Dir: s.mgr.Settings().Dir, Start: r.Start,
	}, func(pages, files int) {
		if emit != nil {
			emit(GrabEvent, GrabProgress{Pages: pages, Files: files})
		}
	})
	s.poke()
	return GrabProgress{Pages: res.Pages, Files: len(res.Files)}, err
}

// StopGrab ends the crawl running now; what it found so far stays.
func (s *Service) StopGrab() {
	grabbing.Lock()
	if grabbing.cancel != nil {
		grabbing.cancel()
	}
	grabbing.Unlock()
}

// handleAction carries out a scheduler request: open a file, tell about
// the download limit, or count down before exiting or shutting down, so a
// shutdown nobody expected can be cancelled.
func (s *Service) handleAction(a engine.Action) {
	w := s.opener()
	switch a.Kind {
	case engine.ActionOpen:
		_ = osutil.Open(a.Path)
	case engine.ActionLimit:
		if w != nil {
			lim := s.mgr.Settings().Limits
			w.openNotice(url.Values{"kind": {"limit"}, "mb": {strconv.Itoa(lim.MB)}, "hours": {strconv.Itoa(lim.Hours)}})
		}
	case engine.ActionExit, engine.ActionSleep, engine.ActionHibernate, engine.ActionShutdown:
		if w != nil {
			w.openNotice(url.Values{"kind": {"finish"}, "action": {a.Kind}})
		}
	}
}

// RunFinish carries out a finish action the countdown let through: sleep
// and hibernate keep ODM running for after the computer wakes; shut down
// and exit close ODM.
func (s *Service) RunFinish(action string) error {
	switch action {
	case engine.ActionSleep, engine.ActionHibernate:
		return osutil.Suspend(action == engine.ActionHibernate)
	case engine.ActionShutdown:
		if err := osutil.Shutdown(); err != nil {
			return err
		}
	case engine.ActionExit:
	default:
		return errNoAction
	}
	s.mu.Lock()
	quit := s.quit
	s.mu.Unlock()
	if quit != nil {
		quit()
	}
	return nil
}

const (
	errNotWeb   serviceError = "not a web address"
	errGrabbing serviceError = "the site grabber is already running"
	errNoAction serviceError = "no such action"
)
