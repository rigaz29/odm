package engine

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"
)

// Duplicate policies for automatic adds.
const (
	DuplicateSkip  = "skip"
	DuplicateAgain = "again"
)

// Proxy modes.
const (
	ProxySystem = "system" // OS / environment setting
	ProxyNone   = "none"   // always connect directly
	ProxyHTTP   = "http"
	ProxySOCKS5 = "socks5"
)

// ProxyConfig describes how downloads reach the network.
type ProxyConfig struct {
	Mode string `json:"mode"` // ProxySystem, ProxyNone, ProxyHTTP or ProxySOCKS5
	Host string `json:"host"`
	Port int    `json:"port"`
	User string `json:"user"`
	Pass string `json:"pass"`
}

// Login is a per-site credential applied when a download targets that host.
type Login struct {
	Host string `json:"host"`
	User string `json:"user"`
	Pass string `json:"pass"`
}

// QueueSchedule is one queue's scheduler settings.
type QueueSchedule struct {
	OnStartup    bool   `json:"on_startup,omitempty"`
	StartEnabled bool   `json:"start_enabled,omitempty"`
	StartAt      string `json:"start_at,omitempty"`  // "HH:MM"
	OnceDate     string `json:"once_date,omitempty"` // "YYYY-MM-DD"; empty = daily on Days
	Days         []bool `json:"days,omitempty"`      // Sunday..Saturday
	StopEnabled  bool   `json:"stop_enabled,omitempty"`
	StopAt       string `json:"stop_at,omitempty"` // "HH:MM"
	// Retries re-queues a file that failed while the queue was running.
	RetriesEnabled bool `json:"retries_enabled,omitempty"`
	Retries        int  `json:"retries,omitempty"`
	// What to do once the queue has run out of work on its own. At most one
	// of exit, sleep, hibernate and shut down is kept (the strongest).
	OpenWhenDone      string `json:"open_when_done,omitempty"` // file to open
	ExitWhenDone      bool   `json:"exit_when_done,omitempty"`
	SleepWhenDone     bool   `json:"sleep_when_done,omitempty"`
	HibernateWhenDone bool   `json:"hibernate_when_done,omitempty"`
	ShutdownWhenDone  bool   `json:"shutdown_when_done,omitempty"`
	// RepeatMinutes starts the queue again that many minutes after it was
	// last started, whenever it is not running, until it is stopped (by
	// hand, at StopAt or by Stop all). 0 is off. A repeating queue never
	// finishes, so the actions above do not run for it.
	RepeatMinutes int `json:"repeat_minutes,omitempty"`
	// Parallel is how many of the queue's files download at once.
	Parallel int `json:"parallel,omitempty"`
	// Sync makes this a synchronization queue: starting it also checks each
	// finished file against the server and downloads it again, replacing
	// the local copy, when it changed there. With RepeatMinutes the check
	// is periodic.
	Sync bool `json:"sync,omitempty"`
}

// DownloadLimits caps how much is downloaded in a rolling window.
type DownloadLimits struct {
	Enabled bool `json:"enabled"`
	MB      int  `json:"mb"`
	Hours   int  `json:"hours"`
	// NoWarning is negative so the warning defaults to on.
	NoWarning bool `json:"no_warning,omitempty"`
}

// Settings are the user-facing options persisted between runs.
type Settings struct {
	Dir         string `json:"dir"`
	Connections int    `json:"connections"`
	Concurrent  int    `json:"concurrent"`
	SpeedLimit  int64  `json:"speed_limit"`
	// LimiterOff keeps SpeedLimit but does not apply it, so the speed
	// limiter can be switched off and on again without retyping the value.
	LimiterOff   bool              `json:"limiter_off,omitempty"`
	Categories   bool              `json:"categories"`
	Clipboard    bool              `json:"clipboard"`
	Proxy        ProxyConfig       `json:"proxy"`
	Logins       []Login           `json:"logins"`
	Queues       []string          `json:"queues"`
	FileTypes    []string          `json:"file_types,omitempty"`
	CategoryDirs map[string]string `json:"category_dirs"`
	// CategoryDefs replaces the built-in categories when set: their names,
	// order and extensions. nil means the built-in set.
	CategoryDefs []CategoryDef            `json:"category_defs,omitempty"`
	Schedules    map[string]QueueSchedule `json:"schedules"`
	Limits       DownloadLimits           `json:"limits"`
	// HostConnections overrides Connections per server (lower-case host).
	HostConnections map[string]int `json:"host_connections,omitempty"`
	// UserAgent replaces the default User-Agent when not empty.
	UserAgent string `json:"user_agent,omitempty"`
	// DuplicateAuto decides what an automatic add (browser, clipboard,
	// grabber, API) does with a URL already in the list: DuplicateSkip or
	// DuplicateAgain. The Add dialog always asks instead.
	DuplicateAuto string `json:"duplicate_auto,omitempty"`
	// NoStartDialog skips the New Download window for downloads handed over
	// by the browser (negative so the window is shown by default).
	NoStartDialog bool `json:"no_start_dialog,omitempty"`
	// ExcludeSites are host patterns ("*.example.com", "example.org") the
	// browser extension leaves alone.
	ExcludeSites []string `json:"exclude_sites,omitempty"`
	// NoUpdateCheck is phrased negatively so settings files written before
	// the option existed keep the default: check for updates at start-up.
	NoUpdateCheck bool `json:"no_update_check,omitempty"`
	// Same negative phrasing: both windows are shown unless turned off.
	NoProgressDialog bool `json:"no_progress_dialog,omitempty"`
	NoCompleteDialog bool `json:"no_complete_dialog,omitempty"`
	// NoNotifications turns off the system notification for a download that
	// finished or failed (shown by default). Sounds plays the system sound
	// for those events (off by default).
	NoNotifications bool `json:"no_notifications,omitempty"`
	Sounds          bool `json:"sounds,omitempty"`
	// NoCloseToTray makes closing the main window quit instead of hiding it
	// to the system tray (where downloads keep running).
	NoCloseToTray bool `json:"no_close_to_tray,omitempty"`
	// NoBrowserRegister keeps ODM from registering itself with the
	// browsers when it starts. Unregister sets it and Register clears it,
	// so an integration someone removed stays removed.
	NoBrowserRegister bool `json:"no_browser_register,omitempty"`
	// LastDir is the save folder last chosen in the New Download window,
	// which offers it next time unless NoRememberDir is set (it is offered
	// by default). Automatic adds (clipboard, grabber, API) use Dir.
	LastDir       string `json:"last_dir,omitempty"`
	NoRememberDir bool   `json:"no_remember_dir,omitempty"`
	// FFmpegPath points at ffmpeg (the program or its folder), used to join
	// streams that come as separate video and audio. Empty: look next to
	// ODM, then on PATH.
	FFmpegPath string `json:"ffmpeg_path,omitempty"`
	// YtDlpPath points at yt-dlp (the program or its folder), which reads
	// video sites' pages such as YouTube's. Empty: look next to ODM, then
	// on PATH.
	YtDlpPath string `json:"ytdlp_path,omitempty"`
	// ToolUpdates says whether ODM may download yt-dlp and ffmpeg itself
	// and keep them updated: "" (not asked yet), "auto" or "off".
	ToolUpdates string `json:"tool_updates,omitempty"`
	// Theme ("light", "dark", "system") and Language ("en", "id",
	// "system") are the new UI's (internal/gui) look; empty until chosen
	// there. The Fyne UI keeps its own in its preferences.
	Theme    string `json:"theme,omitempty"`
	Language string `json:"language,omitempty"`
}

// DefaultQueue is the queue used when none is specified.
const DefaultQueue = "Main"

// canonicalQueue is name as the queue it names is listed. Queue names
// compare without regard to case, and a download carries its queue's
// listed spelling, so everything keyed by queue (running queues,
// schedules, positions) sees one queue. An empty name is the main queue; a
// name no listed queue has is kept as given.
func (s *Settings) canonicalQueue(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return DefaultQueue
	}
	for _, q := range s.Queues {
		if strings.EqualFold(q, name) {
			return q
		}
	}
	return name
}

// DefaultSettings returns settings for a fresh install.
func DefaultSettings(dir string) Settings {
	return Settings{
		Dir:         dir,
		Connections: DefaultConnections,
		Concurrent:  DefaultFileLimit,
		Categories:  true,
		Queues:      []string{DefaultQueue},
	}
}

// EffectiveSpeedLimit is the global limit in bytes per second that applies
// now: 0 when the limiter is switched off or no limit is set.
func (s Settings) EffectiveSpeedLimit() int64 {
	if s.LimiterOff || s.SpeedLimit <= 0 {
		return 0
	}
	return s.SpeedLimit
}

// SaveFolder is the folder the New Download window offers: the one last
// chosen there while that is remembered and the folder still exists (a
// removed drive, a deleted folder), else the default download folder.
func (s Settings) SaveFolder() string {
	if s.NoRememberDir || s.LastDir == "" {
		return s.Dir
	}
	if st, err := os.Stat(s.LastDir); err != nil || !st.IsDir() {
		return s.Dir
	}
	return s.LastDir
}

// clone returns a copy that shares no map or slice with s. The manager
// never changes a published Settings in place, and the copies it hands out
// can be changed or encoded while it keeps working.
func (s Settings) clone() Settings {
	c := s
	c.Logins = slices.Clone(s.Logins)
	c.Queues = slices.Clone(s.Queues)
	c.FileTypes = slices.Clone(s.FileTypes)
	c.ExcludeSites = slices.Clone(s.ExcludeSites)
	c.CategoryDirs = maps.Clone(s.CategoryDirs)
	c.HostConnections = maps.Clone(s.HostConnections)
	if s.CategoryDefs != nil {
		c.CategoryDefs = make([]CategoryDef, len(s.CategoryDefs))
		for i, d := range s.CategoryDefs {
			c.CategoryDefs[i] = CategoryDef{Name: d.Name, Exts: slices.Clone(d.Exts)}
		}
	}
	if s.Schedules != nil {
		c.Schedules = make(map[string]QueueSchedule, len(s.Schedules))
		for q, sc := range s.Schedules {
			sc.Days = slices.Clone(sc.Days)
			c.Schedules[q] = sc
		}
	}
	return c
}

func (s *Settings) normalize(fallbackDir string) {
	if strings.TrimSpace(s.Dir) == "" {
		s.Dir = fallbackDir
	}
	if s.Connections <= 0 {
		s.Connections = DefaultConnections
	}
	if s.Connections > 32 {
		s.Connections = 32
	}
	if s.Concurrent <= 0 {
		s.Concurrent = DefaultFileLimit
	}
	if s.SpeedLimit < 0 {
		s.SpeedLimit = 0
	}
	switch s.Proxy.Mode {
	case ProxyNone, ProxySystem, ProxyHTTP, ProxySOCKS5:
	default:
		// Unset (fresh install) follows the OS, which is what users behind
		// a corporate proxy need without touching Options.
		s.Proxy.Mode = ProxySystem
	}
	if s.Proxy.Port == 0 {
		switch s.Proxy.Mode {
		case ProxyHTTP:
			s.Proxy.Port = 8080
		case ProxySOCKS5:
			s.Proxy.Port = 1080
		}
	}
	if len(s.FileTypes) == 0 {
		s.FileTypes = DefaultFileTypes()
	}
	if s.HostConnections == nil {
		s.HostConnections = map[string]int{}
	}
	for h, n := range s.HostConnections {
		key := strings.ToLower(strings.TrimSpace(h))
		if key == "" || n <= 0 {
			delete(s.HostConnections, h)
			continue
		}
		if n > 32 {
			n = 32
		}
		if key != h {
			delete(s.HostConnections, h)
		}
		s.HostConnections[key] = n
	}
	s.UserAgent = strings.TrimSpace(s.UserAgent)
	var sites []string
	for _, p := range s.ExcludeSites {
		if p = strings.ToLower(strings.TrimSpace(p)); p != "" {
			sites = append(sites, p)
		}
	}
	s.ExcludeSites = sites
	if s.DuplicateAuto != DuplicateAgain {
		s.DuplicateAuto = DuplicateSkip
	}
	// Queue names compare without regard to case: of two spellings of a
	// name the first listed stays, and the main queue is always Main, the
	// name a download without a queue gets.
	seen := map[string]bool{}
	queues := make([]string, 0, len(s.Queues)+1)
	for _, q := range s.Queues {
		q = strings.TrimSpace(q)
		if q == "" || seen[strings.ToLower(q)] {
			continue
		}
		if strings.EqualFold(q, DefaultQueue) {
			q = DefaultQueue
		}
		seen[strings.ToLower(q)] = true
		queues = append(queues, q)
	}
	if !seen[strings.ToLower(DefaultQueue)] {
		queues = append([]string{DefaultQueue}, queues...)
	}
	s.Queues = queues
	if s.Schedules == nil {
		s.Schedules = map[string]QueueSchedule{}
	}
	// A schedule saved under another spelling of a queue's name is that
	// queue's, unless the queue has one under its own.
	for k, sc := range s.Schedules {
		if c := s.canonicalQueue(k); c != k {
			if _, ok := s.Schedules[c]; !ok {
				s.Schedules[c] = sc
			}
			delete(s.Schedules, k)
		}
	}
	keep := map[string]bool{}
	for _, q := range s.Queues {
		keep[q] = true
		sc := s.Schedules[q]
		sc.normalize()
		s.Schedules[q] = sc
	}
	for q := range s.Schedules {
		if !keep[q] { // schedule of a deleted queue
			delete(s.Schedules, q)
		}
	}
	if s.Limits.MB <= 0 {
		s.Limits.MB = 200
	}
	if s.Limits.Hours <= 0 {
		s.Limits.Hours = 5
	}
	if s.CategoryDirs == nil {
		s.CategoryDirs = map[string]string{}
	}
	s.CategoryDefs = normalizeCategories(s.CategoryDefs)
}

// NewQueueSchedule returns the defaults a new queue starts with.
func NewQueueSchedule() QueueSchedule {
	var sc QueueSchedule
	sc.normalize()
	return sc
}

// normalize fills defaults and drops values the scheduler cannot use, so an
// invalid time disables that trigger instead of firing at a wrong moment.
func (sc *QueueSchedule) normalize() {
	if len(sc.Days) != 7 {
		sc.Days = []bool{true, true, true, true, true, true, true}
	}
	if !validClock(sc.StartAt) || sc.StartAt == "" {
		sc.StartAt, sc.StartEnabled = "", false
	}
	if !validClock(sc.StopAt) || sc.StopAt == "" {
		sc.StopAt, sc.StopEnabled = "", false
	}
	if sc.OnceDate != "" {
		if _, err := time.Parse("2006-01-02", sc.OnceDate); err != nil {
			sc.OnceDate = ""
		}
	}
	if sc.Parallel < 1 {
		sc.Parallel = 1
	}
	if sc.RepeatMinutes < 0 {
		sc.RepeatMinutes = 0
	}
	// One finish action: the strongest of those ticked.
	switch {
	case sc.ShutdownWhenDone:
		sc.ExitWhenDone, sc.SleepWhenDone, sc.HibernateWhenDone = false, false, false
	case sc.HibernateWhenDone:
		sc.ExitWhenDone, sc.SleepWhenDone = false, false
	case sc.SleepWhenDone:
		sc.ExitWhenDone = false
	}
	if sc.Retries <= 0 {
		sc.Retries = 10
	}
}

// FinishAction is the action the queue runs when it finishes on its own:
// ActionShutdown, ActionHibernate, ActionSleep, ActionExit or "" for none.
func (sc QueueSchedule) FinishAction() string {
	switch {
	case sc.ShutdownWhenDone:
		return ActionShutdown
	case sc.HibernateWhenDone:
		return ActionHibernate
	case sc.SleepWhenDone:
		return ActionSleep
	case sc.ExitWhenDone:
		return ActionExit
	}
	return ""
}

// SetFinishAction ticks exactly the finish action named (or none for "").
func (sc *QueueSchedule) SetFinishAction(action string) {
	sc.ExitWhenDone = action == ActionExit
	sc.SleepWhenDone = action == ActionSleep
	sc.HibernateWhenDone = action == ActionHibernate
	sc.ShutdownWhenDone = action == ActionShutdown
}

// finishRank orders finish actions: when several queues finish together,
// the strongest one is the one carried out.
func finishRank(action string) int {
	switch action {
	case ActionShutdown:
		return 4
	case ActionHibernate:
		return 3
	case ActionSleep:
		return 2
	case ActionExit:
		return 1
	}
	return 0
}

// validClock reports whether s is an empty string or a HH:MM time.
func validClock(s string) bool {
	if s == "" {
		return true
	}
	_, err := time.Parse("15:04", s)
	return err == nil
}

// LoadSettings reads settings from path, falling back to defaults.
func LoadSettings(path, fallbackDir string) Settings {
	s := DefaultSettings(fallbackDir)
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &s)
	}
	s.normalize(fallbackDir)
	return s
}

// SaveSettings writes settings to path. The file is written next to it and
// renamed into place: a half-written settings.json does not parse, and
// LoadSettings would then start from defaults, losing logins and queues.
func SaveSettings(path string, s Settings) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

var categoryExt = map[string]string{
	".mp4": "Video", ".mkv": "Video", ".avi": "Video", ".mov": "Video", ".webm": "Video",
	".flv": "Video", ".wmv": "Video", ".m4v": "Video", ".ts": "Video", ".mpg": "Video",
	".mpeg": "Video", ".3gp": "Video", ".vob": "Video", ".mts": "Video",

	".mp3": "Audio", ".m4a": "Audio", ".aac": "Audio", ".flac": "Audio", ".wav": "Audio",
	".ogg": "Audio", ".wma": "Audio", ".opus": "Audio", ".aiff": "Audio",

	".pdf": "Documents", ".doc": "Documents", ".docx": "Documents", ".xls": "Documents",
	".xlsx": "Documents", ".ppt": "Documents", ".pptx": "Documents", ".txt": "Documents",
	".epub": "Documents", ".csv": "Documents", ".rtf": "Documents", ".odt": "Documents",

	".zip": "Compressed", ".rar": "Compressed", ".7z": "Compressed", ".tar": "Compressed",
	".gz": "Compressed", ".bz2": "Compressed", ".xz": "Compressed", ".zst": "Compressed",

	".exe": "Programs", ".msi": "Programs", ".dmg": "Programs", ".pkg": "Programs",
	".deb": "Programs", ".rpm": "Programs", ".appx": "Programs", ".apk": "Programs",
	".bat": "Programs", ".sh": "Programs",

	".jpg": "Images", ".jpeg": "Images", ".png": "Images", ".gif": "Images", ".webp": "Images",
	".bmp": "Images", ".svg": "Images", ".tiff": "Images", ".ico": "Images", ".heic": "Images",

	".iso": "Disk Images", ".img": "Disk Images", ".vhd": "Disk Images",
}

// KnownFileType reports whether name has a downloadable extension. With
// types (the Options "File types" list, with or without dots) only those
// count; without, the built-in list does. types is only compared, never
// merged into categoryExt, the built-in table the default categories are
// made from: the takeover list must not change category folders.
func KnownFileType(name string, types ...string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	if ext == "" {
		return false
	}
	if len(types) == 0 {
		_, ok := categoryExt[ext]
		return ok
	}
	for _, t := range types {
		if ext == "."+strings.ToLower(strings.TrimPrefix(strings.TrimSpace(t), ".")) {
			return true
		}
	}
	return false
}

// DefaultFileTypes is the built-in takeover list, extensions without dots,
// sorted, as the File types tab's Default button restores it.
func DefaultFileTypes() []string {
	out := make([]string, 0, len(categoryExt))
	for ext := range categoryExt {
		out = append(out, strings.TrimPrefix(ext, "."))
	}
	sort.Strings(out)
	return out
}
