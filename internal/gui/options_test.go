package gui

import (
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"odm/internal/engine"
)

func TestOptionsKeepWhatTheyDoNotChange(t *testing.T) {
	svc, mgr := newTestService(t)
	mgr.AddQueue("Night")
	before := mgr.UpdateSettings(mgr.Settings()) // as the engine keeps them
	if r := svc.SaveOptions(svc.Options()); r.Refused != nil || r.StartupError != "" {
		t.Fatalf("saving the options as they are: %+v", r)
	}
	after := mgr.Settings()
	// Saving fills in the look the window showed; the rest stays.
	after.Theme, after.Language = before.Theme, before.Language
	if !reflect.DeepEqual(normal(before), normal(after)) {
		t.Fatalf("settings changed:\nbefore %+v\nafter  %+v", before, after)
	}
}

// normal treats nil and empty maps and slices alike, and a limiter with
// no limit as off.
func normal(s engine.Settings) engine.Settings {
	if s.SpeedLimit == 0 {
		s.LimiterOff = true
	}
	if len(s.HostConnections) == 0 {
		s.HostConnections = nil
	}
	if len(s.CategoryDirs) == 0 {
		s.CategoryDirs = nil
	}
	if len(s.FileTypes) == 0 {
		s.FileTypes = nil
	}
	if len(s.ExcludeSites) == 0 {
		s.ExcludeSites = nil
	}
	if len(s.Logins) == 0 {
		s.Logins = nil
	}
	return s
}

func TestSaveOptions(t *testing.T) {
	svc, mgr := newTestService(t)
	var mu sync.Mutex
	var looks []Appearance
	svc.emit = func(name string, data any) {
		if name == AppearanceEvent {
			mu.Lock()
			looks = append(looks, data.(Appearance))
			mu.Unlock()
		}
	}
	dir := t.TempDir()
	o := svc.Options()
	o.Dir = dir
	o.StartDialog, o.ProgressDialog, o.CompleteDialog = false, false, true
	o.DuplicateAgain = true
	o.FileTypes = "torrent, .SRT nupkg srt"
	o.ExcludeSites = "*.example.com, example.org"
	o.LimitOn, o.LimitKB = true, 300
	o.Hosts = []HostConnections{{Host: " CDN.Example.com ", Connections: 2}, {Host: "", Connections: 4}}
	o.CapOn, o.CapMB, o.CapHours, o.CapWarning = true, 500, 6, false
	o.Logins = []SiteLogin{{Host: "files.example.com", User: "me", Pass: "pw"}, {Host: " "}}
	o.Theme, o.Language = "dark", "id"
	o.YtDlpCookies = " Firefox "
	o.Categories = append([]CategoryOption{{Name: "Comics", Exts: "CBZ cbr", Folder: filepath.Join(dir, "comics")}}, o.Categories...)
	if r := svc.SaveOptions(o); r.Refused != nil || r.Appearance == nil || r.Appearance.Theme != "dark" || r.Appearance.Language != "id" {
		t.Fatalf("result %+v", r)
	}
	st := mgr.Settings()
	switch {
	case st.Dir != dir || !st.NoStartDialog || !st.NoProgressDialog || st.NoCompleteDialog:
		t.Fatalf("folder and windows: %+v", st)
	case st.DuplicateAuto != engine.DuplicateAgain || !reflect.DeepEqual(st.FileTypes, []string{"torrent", "srt", "nupkg"}):
		t.Fatalf("duplicates and types: %q %q", st.DuplicateAuto, st.FileTypes)
	case !reflect.DeepEqual(st.ExcludeSites, []string{"*.example.com", "example.org"}):
		t.Fatalf("excluded sites %q", st.ExcludeSites)
	case st.EffectiveSpeedLimit() != 300<<10 || !reflect.DeepEqual(st.HostConnections, map[string]int{"cdn.example.com": 2}):
		t.Fatalf("limit %d, hosts %v", st.EffectiveSpeedLimit(), st.HostConnections)
	case !st.Limits.Enabled || st.Limits.MB != 500 || st.Limits.Hours != 6 || !st.Limits.NoWarning:
		t.Fatalf("download limits %+v", st.Limits)
	case len(st.Logins) != 1 || st.Logins[0] != (engine.Login{Host: "files.example.com", User: "me", Pass: "pw"}):
		t.Fatalf("logins %+v", st.Logins)
	case len(st.CategoryDefs) == 0 || st.CategoryDefs[0].Name != "Comics" || !reflect.DeepEqual(st.CategoryDefs[0].Exts, []string{"cbr", "cbz"}):
		t.Fatalf("categories %+v", st.CategoryDefs)
	case st.CategoryDefs[len(st.CategoryDefs)-1].Name != engine.OtherCategory || st.CategoryDirs["Comics"] != filepath.Join(dir, "comics"):
		t.Fatalf("Other is not last, or the folder was lost: %+v %v", st.CategoryDefs, st.CategoryDirs)
	case st.Theme != "dark" || st.Language != "id" || svc.Appearance().Language != "id":
		t.Fatalf("look %q %q", st.Theme, st.Language)
	case st.YtDlpCookies != "firefox" || svc.Options().YtDlpCookies != "firefox":
		t.Fatalf("yt-dlp's browser %q", st.YtDlpCookies)
	}
	o.YtDlpCookies = engine.CookiesFromExtension
	if r := svc.SaveOptions(o); r.Refused != nil || mgr.Settings().YtDlpCookies != engine.CookiesFromExtension {
		t.Fatalf("the extension's cookies: %+v, %q", r, mgr.Settings().YtDlpCookies)
	}
	mu.Lock()
	if len(looks) != 1 || looks[0].Theme != "dark" {
		t.Fatalf("windows were told %+v", looks)
	}
	mu.Unlock()

	// Back to the built-in categories: the settings keep none of their own.
	o = svc.Options()
	o.Categories = svc.DefaultCategories()
	svc.SaveOptions(o)
	if st := mgr.Settings(); st.CategoryDefs != nil {
		t.Fatalf("the built-in set was saved as %+v", st.CategoryDefs)
	}
	engine.Categories() // the defaults are back in use
}

func TestSaveOptionsRefuses(t *testing.T) {
	svc, mgr := newTestService(t)
	o := svc.Options()
	o.ProxyMode, o.ProxyHost = engine.ProxySOCKS5, " "
	if r := svc.SaveOptions(o); r.Refused == nil || r.Refused.Tab != "proxy" {
		t.Fatalf("a proxy without a server: %+v", r)
	}
	o = svc.Options()
	o.Categories = append(o.Categories, CategoryOption{Name: "a/b"})
	if r := svc.SaveOptions(o); r.Refused == nil || r.Refused.Tab != "save" {
		t.Fatalf("a category name with a slash: %+v", r)
	}
	o = svc.Options()
	o.Categories = append(o.Categories, CategoryOption{Name: "video"})
	if r := svc.SaveOptions(o); r.Refused == nil {
		t.Fatal("a second Video was taken")
	}
	if mgr.Settings().Proxy.Mode == engine.ProxySOCKS5 {
		t.Fatal("a refused form was saved")
	}
	o = svc.Options()
	o.ProxyMode, o.ProxyHost, o.ProxyPort = engine.ProxyHTTP, " proxy.lan ", 3128
	if r := svc.SaveOptions(o); r.Refused != nil {
		t.Fatalf("%+v", r)
	}
	if p := mgr.Settings().Proxy; p != (engine.ProxyConfig{Mode: engine.ProxyHTTP, Host: "proxy.lan", Port: 3128}) {
		t.Fatalf("proxy %+v", p)
	}
}

func TestToolStatusAtAPath(t *testing.T) {
	svc, _ := newTestService(t)
	if st := svc.ToolStatus("ffmpeg", filepath.Join(t.TempDir(), "nope.exe")); st.State != "nothere" {
		t.Fatalf("ffmpeg at a missing path: %+v", st)
	}
	if st := svc.ToolStatus("yt-dlp", filepath.Join(t.TempDir(), "nope.exe")); st.State != "nothere" {
		t.Fatalf("yt-dlp at a missing path: %+v", st)
	}
	if err := svc.UpdateTools(); err == nil {
		t.Fatal("updated tools without a keeper")
	}
	if got := toolMode("", false); got != "" {
		t.Fatalf("off before asking: %q", got)
	}
	if got := toolMode("auto", false); got != "off" {
		t.Fatalf("off after auto: %q", got)
	}
}
