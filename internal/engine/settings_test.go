package engine

import (
	"net/http"
	"net/url"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCategoryMapping(t *testing.T) {
	cases := map[string]string{
		"movie.MP4":       "Video",
		"song.flac":       "Audio",
		"report.pdf":      "Documents",
		"archive.tar.gz":  "Compressed",
		"setup.exe":       "Programs",
		"photo.jpeg":      "Images",
		"ubuntu.iso":      "Disk Images",
		"data.unknownext": "Other",
		"noext":           "Other",
	}
	for name, want := range cases {
		if got := Category(name); got != want {
			t.Errorf("Category(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestSettingsNormalize(t *testing.T) {
	s := Settings{}
	s.normalize("/fallback")
	if s.Dir != "/fallback" {
		t.Errorf("dir fallback = %q", s.Dir)
	}
	if s.Connections != DefaultConnections {
		t.Errorf("connections = %d", s.Connections)
	}
	if s.Concurrent != DefaultFileLimit {
		t.Errorf("concurrent = %d", s.Concurrent)
	}

	big := Settings{Dir: "/d", Connections: 99, Concurrent: -1, SpeedLimit: -5}
	big.normalize("/fallback")
	if big.Connections != 32 {
		t.Errorf("connections cap = %d", big.Connections)
	}
	if big.Concurrent != DefaultFileLimit {
		t.Errorf("concurrent = %d", big.Concurrent)
	}
	if big.SpeedLimit != 0 {
		t.Errorf("speed limit = %d", big.SpeedLimit)
	}
}

func TestLoadSaveSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	def := LoadSettings(path, "/fallback")
	if def.Dir != "/fallback" || def.Connections != DefaultConnections || def.YtDlpCookies != CookiesFromExtension {
		t.Fatalf("defaults wrong: %+v", def)
	}

	want := Settings{Dir: "/downloads", Connections: 12, Concurrent: 5, SpeedLimit: 1048576, Categories: false, Proxy: ProxyConfig{Mode: "none"}, Queues: []string{DefaultQueue}, FileTypes: DefaultFileTypes(), CategoryDirs: map[string]string{},
		HostConnections: map[string]int{}, DuplicateAuto: DuplicateSkip,
		Schedules: map[string]QueueSchedule{DefaultQueue: {Days: []bool{true, true, true, true, true, true, true}, Parallel: 1, Retries: 10}},
		Limits:    DownloadLimits{MB: 200, Hours: 5}, YtDlpCookies: CookiesFromExtension}
	if err := SaveSettings(path, want); err != nil {
		t.Fatal(err)
	}
	got := LoadSettings(path, "/fallback")
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip = %+v, want %+v", got, want)
	}
}

func TestProxyModes(t *testing.T) {
	target, _ := http.NewRequest(http.MethodGet, "https://files.example/a.zip", nil)
	SystemProxy = func(*http.Request) (*url.URL, error) { return url.Parse("http://sys:3128") }
	defer func() { SystemProxy = http.ProxyFromEnvironment }()

	proxyOf := func(p ProxyConfig) string {
		s := DefaultSettings(t.TempDir())
		s.Proxy = p
		s.normalize("")
		tr := buildClient(s).Transport.(*http.Transport)
		if tr.Proxy == nil {
			return "direct"
		}
		u, _ := tr.Proxy(target)
		if u == nil {
			return "direct"
		}
		return u.String()
	}
	cases := []struct {
		p    ProxyConfig
		want string
	}{
		{ProxyConfig{}, "http://sys:3128"}, // fresh install follows the OS
		{ProxyConfig{Mode: ProxySystem}, "http://sys:3128"},
		{ProxyConfig{Mode: ProxyNone}, "direct"},
		{ProxyConfig{Mode: ProxyHTTP, Host: "p.corp", Port: 8080}, "http://p.corp:8080"},
		{ProxyConfig{Mode: ProxySOCKS5, Host: "s.corp", User: "u", Pass: "pw"}, "socks5://u:pw@s.corp:1080"},
	}
	for _, c := range cases {
		if got := proxyOf(c.p); got != c.want {
			t.Errorf("%+v: got %s, want %s", c.p, got, c.want)
		}
	}
}

func TestConnectionsForHost(t *testing.T) {
	s := DefaultSettings(t.TempDir())
	s.HostConnections = map[string]int{" CDN.Example ": 2, "zero.example": 0}
	s.normalize("")
	if got := connectionsFor(s, "https://cdn.example/x.iso"); got != 2 {
		t.Fatalf("exception host: %d, want 2", got)
	}
	if got := connectionsFor(s, "https://other.example/x.iso"); got != s.Connections {
		t.Fatalf("other host: %d, want default %d", got, s.Connections)
	}
	if _, ok := s.HostConnections["zero.example"]; ok {
		t.Fatal("a 0 exception should be dropped")
	}
}

func TestKnownFileTypeList(t *testing.T) {
	if !KnownFileType("a.zip") || KnownFileType("a.torrent") {
		t.Fatal("built-in list wrong")
	}
	// An explicit list replaces the built-ins, as in the File types tab.
	if !KnownFileType("a.torrent", "torrent", "iso") || KnownFileType("a.zip", "torrent", "iso") {
		t.Fatal("explicit list should be exclusive")
	}
	if KnownFileType("noext", "torrent") {
		t.Fatal("no extension matched")
	}
	def := DefaultFileTypes()
	if len(def) == 0 || def[0] > def[len(def)-1] {
		t.Fatal("defaults should be a sorted, non-empty list")
	}
}

func TestUserAgentSetting(t *testing.T) {
	defer setUserAgent("")
	setUserAgent("Custom/1.0")
	req, _ := http.NewRequest(http.MethodGet, "http://x/", nil)
	applyHeaders(req, nil)
	if got := req.Header.Get("User-Agent"); got != "Custom/1.0" {
		t.Fatalf("UA %q", got)
	}
	setUserAgent("")
	req, _ = http.NewRequest(http.MethodGet, "http://x/", nil)
	applyHeaders(req, nil)
	if got := req.Header.Get("User-Agent"); got != DefaultUserAgent {
		t.Fatalf("empty setting should restore the default, got %q", got)
	}
}
