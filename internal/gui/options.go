package gui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"odm/internal/autostart"
	"odm/internal/browserreg"
	"odm/internal/engine"
	"odm/internal/ffmpeg"
	"odm/internal/i18n"
	"odm/internal/tools"
	"odm/internal/ytdlp"
)

// The Options window, as the Fyne UI's (internal/ui/dialogs_desktop.go,
// openOptionsTab): every setting a person changes, sent to the page as
// OptionsForm and saved back from it with SaveOptions, starting from the
// current settings so what the window does not edit (queues, schedules)
// is kept. Fields are named for what they turn on: the settings' negative
// ones (NoStartDialog…) are turned around here.

// CategoryOption is one category: its name, its extensions (space
// separated; none for Other) and its own folder ("" for the default).
type CategoryOption struct {
	Name   string `json:"name"`
	Exts   string `json:"exts"`
	Folder string `json:"folder"`
}

// HostConnections caps the connections to one server.
type HostConnections struct {
	Host        string `json:"host"`
	Connections int    `json:"connections"`
}

// SiteLogin is a user name and password for a site.
type SiteLogin struct {
	Host string `json:"host"`
	User string `json:"user"`
	Pass string `json:"pass"`
}

// OptionsForm is the Options window's form.
type OptionsForm struct {
	// General
	Startup      bool   `json:"startup"` // launch ODM when Windows starts
	CloseToTray  bool   `json:"closeToTray"`
	Clipboard    bool   `json:"clipboard"`
	UpdateCheck  bool   `json:"updateCheck"`
	ExcludeSites string `json:"excludeSites"` // space separated
	Theme        string `json:"theme"`        // "system", "light", "dark"
	Language     string `json:"language"`     // "system", "en", "id"
	// File types the clipboard monitor and the grabber take, space
	// separated, upper case.
	FileTypes string `json:"fileTypes"`
	// Save to
	Dir             string           `json:"dir"`
	CategoryFolders bool             `json:"categoryFolders"`
	RememberDir     bool             `json:"rememberDir"`
	Categories      []CategoryOption `json:"categories"`
	// Downloads
	StartDialog    bool   `json:"startDialog"`
	ProgressDialog bool   `json:"progressDialog"`
	CompleteDialog bool   `json:"completeDialog"`
	Notifications  bool   `json:"notifications"`
	Sounds         bool   `json:"sounds"`
	DuplicateAgain bool   `json:"duplicateAgain"`
	UserAgent      string `json:"userAgent"`
	// Programs
	FFmpegPath string `json:"ffmpegPath"`
	YtDlpPath  string `json:"ytdlpPath"`
	// YtDlpCookies is the browser yt-dlp signs in to video sites with
	// (ytdlp.CookieBrowsers), "" for none.
	YtDlpCookies string `json:"ytdlpCookies"`
	ToolUpdates  bool   `json:"toolUpdates"` // download and update them automatically
	// Connection
	Connections int               `json:"connections"`
	Concurrent  int               `json:"concurrent"`
	LimitOn     bool              `json:"limitOn"`
	LimitKB     int               `json:"limitKB"`
	Hosts       []HostConnections `json:"hosts"`
	CapOn       bool              `json:"capOn"` // download no more than CapMB every CapHours
	CapMB       int               `json:"capMB"`
	CapHours    int               `json:"capHours"`
	CapWarning  bool              `json:"capWarning"`
	// Proxy: "none", "system", "http" or "socks5"
	ProxyMode string `json:"proxyMode"`
	ProxyHost string `json:"proxyHost"`
	ProxyPort int    `json:"proxyPort"`
	ProxyUser string `json:"proxyUser"`
	ProxyPass string `json:"proxyPass"`
	// Site logins
	Logins []SiteLogin `json:"logins"`

	// What this system offers, not saved: a sound to play, and ODM's own
	// copies of yt-dlp and ffmpeg.
	CanSound bool `json:"canSound"`
	CanTools bool `json:"canTools"`
}

// The default User-Agent, for the field's placeholder.
func (s *Service) DefaultUserAgent() string { return engine.DefaultUserAgent }

// Options returns the settings for the Options window.
func (s *Service) Options() OptionsForm {
	st := s.mgr.Settings()
	o := OptionsForm{
		Startup: autostart.Enabled(), CloseToTray: !st.NoCloseToTray, Clipboard: st.Clipboard,
		UpdateCheck: !st.NoUpdateCheck, ExcludeSites: strings.Join(st.ExcludeSites, " "),
		Theme: st.Theme, Language: st.Language,
		FileTypes: strings.ToUpper(strings.Join(st.FileTypes, " ")),
		Dir:       st.Dir, CategoryFolders: st.Categories, RememberDir: !st.NoRememberDir,
		StartDialog: !st.NoStartDialog, ProgressDialog: !st.NoProgressDialog, CompleteDialog: !st.NoCompleteDialog,
		Notifications: !st.NoNotifications, Sounds: st.Sounds, DuplicateAgain: st.DuplicateAuto == engine.DuplicateAgain,
		UserAgent:  st.UserAgent,
		FFmpegPath: st.FFmpegPath, YtDlpPath: st.YtDlpPath, YtDlpCookies: st.YtDlpCookies, ToolUpdates: st.ToolUpdates == tools.ModeAuto,
		Connections: st.Connections, Concurrent: st.Concurrent,
		LimitOn: st.EffectiveSpeedLimit() > 0, LimitKB: int(st.SpeedLimit / 1024),
		CapOn: st.Limits.Enabled, CapMB: st.Limits.MB, CapHours: st.Limits.Hours, CapWarning: !st.Limits.NoWarning,
		ProxyMode: st.Proxy.Mode, ProxyHost: st.Proxy.Host, ProxyPort: st.Proxy.Port,
		ProxyUser: st.Proxy.User, ProxyPass: st.Proxy.Pass,
		CanSound: runtime.GOOS == "windows", CanTools: s.tools() != nil,
	}
	if o.Theme == "" || o.Language == "" {
		look := appearanceFor(st)
		if o.Theme == "" {
			o.Theme = look.Theme
		}
		if o.Language == "" {
			o.Language = "system"
		}
	}
	if o.ProxyMode == "" {
		o.ProxyMode = engine.ProxySystem
	}
	for _, c := range engine.CategoriesInUse() {
		o.Categories = append(o.Categories, CategoryOption{Name: c.Name, Exts: strings.Join(c.Exts, " "), Folder: st.CategoryDirs[c.Name]})
	}
	for h, n := range st.HostConnections {
		o.Hosts = append(o.Hosts, HostConnections{Host: h, Connections: n})
	}
	slices.SortFunc(o.Hosts, func(a, b HostConnections) int { return strings.Compare(a.Host, b.Host) })
	for _, l := range st.Logins {
		o.Logins = append(o.Logins, SiteLogin{Host: l.Host, User: l.User, Pass: l.Pass})
	}
	return o
}

// DefaultFileTypes and DefaultCategories are what the Default buttons put
// back.
func (s *Service) DefaultFileTypes() string {
	return strings.ToUpper(strings.Join(engine.DefaultFileTypes(), " "))
}

func (s *Service) DefaultCategories() []CategoryOption {
	var out []CategoryOption
	for _, c := range engine.DefaultCategories() {
		out = append(out, CategoryOption{Name: c.Name, Exts: strings.Join(c.Exts, " ")})
	}
	return out
}

var (
	errCategoryName = errors.New("category: bad name")
	errNoTools      = errors.New("ODM cannot download its companion programs in this build")
)

// OptionsError names the field SaveOptions refused, for the page to show.
type OptionsError struct {
	Tab   string `json:"tab"`
	Field string `json:"field"`
}

// SaveOptions applies the Options window's form. It returns what the page
// should tell: the field it refused (nothing is saved then), the new look,
// or a problem launching at start-up (the rest is saved).
func (s *Service) SaveOptions(o OptionsForm) SaveResult {
	if o.ProxyMode == engine.ProxyHTTP || o.ProxyMode == engine.ProxySOCKS5 {
		if strings.TrimSpace(o.ProxyHost) == "" {
			return SaveResult{Refused: &OptionsError{Tab: "proxy", Field: "proxyHost"}}
		}
	}
	defs, dirs, err := categoryDefs(o.Categories)
	if err != nil {
		return SaveResult{Refused: &OptionsError{Tab: "save", Field: "categories"}}
	}

	was := s.mgr.Settings()
	next := was // the fields the window does not edit stay
	next.NoCloseToTray = !o.CloseToTray
	next.Clipboard = o.Clipboard
	next.NoUpdateCheck = !o.UpdateCheck
	next.ExcludeSites = strings.Fields(strings.ReplaceAll(o.ExcludeSites, ",", " "))
	next.Theme, next.Language = o.Theme, o.Language
	next.FileTypes = parseFileTypes(o.FileTypes)
	next.Dir = strings.TrimSpace(o.Dir)
	next.Categories = o.CategoryFolders
	next.NoRememberDir = !o.RememberDir
	// A new default folder, or remembering turned off, offers the default
	// folder again rather than the one chosen before.
	if next.Dir != was.Dir || next.NoRememberDir {
		next.LastDir = ""
	}
	next.CategoryDefs, next.CategoryDirs = defs, dirs
	next.NoStartDialog = !o.StartDialog
	next.NoProgressDialog = !o.ProgressDialog
	next.NoCompleteDialog = !o.CompleteDialog
	next.NoNotifications = !o.Notifications
	next.Sounds = o.Sounds
	next.DuplicateAuto = engine.DuplicateSkip
	if o.DuplicateAgain {
		next.DuplicateAuto = engine.DuplicateAgain
	}
	next.UserAgent = strings.TrimSpace(o.UserAgent)
	next.FFmpegPath = strings.TrimSpace(o.FFmpegPath)
	next.YtDlpPath = strings.TrimSpace(o.YtDlpPath)
	next.YtDlpCookies = o.YtDlpCookies // an unknown browser reads as none
	next.ToolUpdates = toolMode(was.ToolUpdates, o.ToolUpdates)
	next.Connections = o.Connections
	next.Concurrent = o.Concurrent
	next.SpeedLimit = int64(max(0, o.LimitKB)) * 1024
	next.LimiterOff = !o.LimitOn
	next.HostConnections = map[string]int{}
	for _, h := range o.Hosts {
		if host := strings.ToLower(strings.TrimSpace(h.Host)); host != "" && h.Connections > 0 {
			next.HostConnections[host] = h.Connections
		}
	}
	next.Limits = engine.DownloadLimits{Enabled: o.CapOn, MB: o.CapMB, Hours: o.CapHours, NoWarning: !o.CapWarning}
	next.Proxy = engine.ProxyConfig{Mode: o.ProxyMode}
	if o.ProxyMode == engine.ProxyHTTP || o.ProxyMode == engine.ProxySOCKS5 {
		next.Proxy = engine.ProxyConfig{Mode: o.ProxyMode, Host: strings.TrimSpace(o.ProxyHost), Port: o.ProxyPort,
			User: strings.TrimSpace(o.ProxyUser), Pass: o.ProxyPass}
	}
	next.Logins = nil
	for _, l := range o.Logins {
		if host := strings.TrimSpace(l.Host); host != "" {
			next.Logins = append(next.Logins, engine.Login{Host: host, User: l.User, Pass: l.Pass})
		}
	}
	s.mgr.UpdateSettings(next)
	if next.ToolUpdates == tools.ModeAuto && was.ToolUpdates != tools.ModeAuto {
		if k := s.tools(); k != nil {
			k.Kick()
		}
	}

	var res SaveResult
	if look := appearanceFor(next); look != appearanceFor(was) {
		res.Appearance = &look
		s.emitLook(look)
	}
	if o.Startup != autostart.Enabled() {
		exe, err := selfExe()
		if err == nil {
			err = autostart.Set(o.Startup, exe)
		}
		if err != nil {
			res.StartupError = err.Error()
		}
	}
	s.poke()
	return res
}

// SaveResult is how saving the options went.
type SaveResult struct {
	// Refused names a field the form must fix first; nothing was saved.
	Refused *OptionsError `json:"refused"`
	// Appearance is the new look when the theme or language changed:
	// every window takes it at once.
	Appearance *Appearance `json:"appearance"`
	// StartupError says why launching at start-up could not change.
	StartupError string `json:"startupError"`
}

// AppearanceEvent carries a new look to every window.
const AppearanceEvent = "odm:appearance"

func (s *Service) emitLook(look Appearance) {
	s.mu.Lock()
	emit, onLook := s.emit, s.onLook
	s.mu.Unlock()
	if emit != nil {
		emit(AppearanceEvent, look)
	}
	if onLook != nil {
		onLook()
	}
}

// SetToolUpdates answers the question asked the first time yt-dlp or
// ffmpeg is needed: on lets ODM download them and keep them updated.
func (s *Service) SetToolUpdates(on bool) {
	mode := tools.ModeOff
	if on {
		mode = tools.ModeAuto
	}
	next := s.mgr.Settings()
	if next.ToolUpdates != mode {
		next.ToolUpdates = mode
		s.mgr.UpdateSettings(next)
	}
	if k := s.tools(); on && k != nil {
		k.Kick()
	}
}

// categoryDefs turns the edited categories into settings: their names,
// order and extensions (nil when they are the built-in set) and their
// folders. Other goes last, without extensions.
func categoryDefs(cats []CategoryOption) ([]engine.CategoryDef, map[string]string, error) {
	var defs []engine.CategoryDef
	dirs := map[string]string{}
	seen := map[string]bool{}
	other := ""
	for _, c := range cats {
		name := strings.TrimSpace(c.Name)
		if name == "" || seen[strings.ToLower(name)] || strings.ContainsAny(name, `\/:*?"<>|`) {
			return nil, nil, errCategoryName
		}
		seen[strings.ToLower(name)] = true
		if folder := strings.TrimSpace(c.Folder); folder != "" {
			dirs[name] = folder
		}
		if name == engine.OtherCategory {
			other = name
			continue
		}
		defs = append(defs, engine.CategoryDef{Name: name, Exts: parseFileTypes(c.Exts)})
	}
	if other == "" {
		other = engine.OtherCategory
	}
	defs = append(defs, engine.CategoryDef{Name: other})
	if sameCategories(defs, engine.DefaultCategories()) {
		defs = nil // the built-in set
	}
	return defs, dirs, nil
}

func sameCategories(a, b []engine.CategoryDef) bool {
	return slices.EqualFunc(a, b, func(x, y engine.CategoryDef) bool {
		return x.Name == y.Name && slices.Equal(x.Exts, y.Exts)
	})
}

// parseFileTypes turns "torrent, .SRT nupkg" into ["torrent" "srt" "nupkg"].
func parseFileTypes(s string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, f := range strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == ',' || r == ';' || r == '\n' || r == '\t' || r == '\r'
	}) {
		f = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(f), "."))
		if f != "" && !seen[f] {
			seen[f] = true
			out = append(out, f)
		}
	}
	return out
}

// toolMode is the ToolUpdates setting for the box: on is automatic; off
// stays "not asked yet" if it was, so ODM still asks the first time a
// tool is needed.
func toolMode(was string, on bool) string {
	switch {
	case on:
		return tools.ModeAuto
	case was == tools.ModeAsk:
		return tools.ModeAsk
	}
	return tools.ModeOff
}

// BrowserStatus is the browser integration as the Options window shows it.
type BrowserStatus struct {
	// State is "ok" (every browser uses this ODM), "partial" or "none".
	State      string `json:"state"`
	Summary    string `json:"summary"`    // in the page's language
	Registered bool   `json:"registered"` // any browser registered, here or elsewhere
}

// BrowserStatus reports which browsers hand downloads to this ODM.
func (s *Service) BrowserStatus() BrowserStatus {
	exe, _ := selfExe()
	sts := browserreg.Check()
	here, other, missing := 0, 0, 0
	for _, st := range sts {
		switch {
		case st.Points(exe):
			here++
		case st.Registered:
			other++
		case st.Installed:
			missing++
		}
	}
	out := BrowserStatus{Summary: browserreg.SummaryWith(sts, exe, s.translator()), Registered: here+other > 0}
	switch {
	case here > 0 && other == 0 && missing == 0:
		out.State = "ok"
	case here == 0 && other == 0:
		out.State = "none"
	default:
		out.State = "partial"
	}
	return out
}

// RegisterBrowsers points every browser at this ODM; UnregisterBrowsers
// takes ODM out of them, and ODM stops registering itself at start until
// registered again. A browser that could not be changed is in the error.
func (s *Service) RegisterBrowsers() (BrowserStatus, error) {
	exe, err := selfExe()
	if err != nil {
		return s.BrowserStatus(), err
	}
	failed := failures(browserreg.Register(exe, s.mgr.ConfigDir()))
	s.setBrowserRegister(true)
	return s.BrowserStatus(), failed
}

func (s *Service) UnregisterBrowsers() (BrowserStatus, error) {
	failed := failures(browserreg.Unregister())
	s.setBrowserRegister(false)
	return s.BrowserStatus(), failed
}

func failures(rs []browserreg.Result) error {
	var out []string
	for _, r := range rs {
		if r.Err != nil {
			out = append(out, r.String())
		}
	}
	if len(out) == 0 {
		return nil
	}
	return errors.New(strings.Join(out, "\n"))
}

func (s *Service) setBrowserRegister(on bool) {
	next := s.mgr.Settings()
	if next.NoBrowserRegister != !on {
		next.NoBrowserRegister = !on
		s.mgr.UpdateSettings(next)
	}
}

// translator translates into the page's language, for texts made here.
func (s *Service) translator() func(string) string {
	if s.Appearance().Language != "id" {
		return func(t string) string { return t }
	}
	id, _ := i18n.Indonesian()
	return func(t string) string {
		if v := id[t]; v != "" {
			return v
		}
		return t
	}
}

// ToolStatus is what a typed path to ffmpeg or yt-dlp leads to.
type ToolStatus struct {
	// State is "found", "missing" (none found anywhere), "nothere" (none
	// at the typed path) or "broken" (found but it did not run).
	State   string `json:"state"`
	Path    string `json:"path"`
	Version string `json:"version"`
	Error   string `json:"error"`
}

// ToolStatus checks the program tool ("ffmpeg" or "yt-dlp") at typed, or
// where ODM looks for it when typed is empty, and runs it for its version.
func (s *Service) ToolStatus(tool, typed string) ToolStatus {
	locate, version := ffmpeg.Locate, ffmpeg.Version
	if tool == "yt-dlp" {
		locate, version = ytdlp.Locate, ytdlp.Version
	}
	bin, err := locate(typed)
	if err != nil {
		if strings.TrimSpace(typed) == "" {
			return ToolStatus{State: "missing"}
		}
		return ToolStatus{State: "nothere"}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	v, err := version(ctx, bin)
	if err != nil {
		return ToolStatus{State: "broken", Path: bin, Error: err.Error()}
	}
	// "ffmpeg version 7.1 Copyright (c) 2000-2024 the FFmpeg developers"
	v, _, _ = strings.Cut(v, " Copyright")
	return ToolStatus{State: "found", Path: bin, Version: v}
}

// UpdateTools downloads the newest yt-dlp and ffmpeg now.
func (s *Service) UpdateTools() error {
	k := s.tools()
	if k == nil {
		return errNoTools
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	return k.UpdateAll(ctx)
}

func (s *Service) tools() *tools.Keeper {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.keeper
}

// OpenOptions opens the Options window on tab ("" for the first).
func (s *Service) OpenOptions(tab string) {
	if w := s.opener(); w != nil {
		w.openOptions(tab)
	}
}

// selfExe is this ODM's executable as browser registrations record it.
func selfExe() (string, error) {
	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe)
	}
	if err != nil {
		return "", fmt.Errorf("cannot locate the ODM executable: %w", err)
	}
	return exe, nil
}
