package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestMain lets the test binary stand in for yt-dlp when FAKE_YTDLP is set
// (see fakeYtDlp): it reports progress for each format of -f and writes
// the joined file to -o, or, for -J, describes a two-quality video; with
// FAKE_YTDLP=live, a broadcast whose master playlist is
// FAKE_LIVE_MANIFEST (its DASH manifest with FAKE_LIVE_KIND=dash, its
// endless FLV with FAKE_LIVE_KIND=flv). With FAKE_FFMPEG set it stands in
// for ffmpeg: "copy" copies the input to the output (the parts one after
// the other, for the concat demuxer; "merged:<video>|<audio>" for two
// inputs), "fail" fails. With both set, a run
// with ffmpeg's -hide_banner is ffmpeg.
func TestMain(m *testing.M) {
	switch {
	case os.Getenv("FAKE_FFMPEG") != "" && (os.Getenv("FAKE_YTDLP") == "" || slices.Contains(os.Args[1:], "-hide_banner")):
		fakeFFmpegMain()
	case os.Getenv("FAKE_YTDLP") == "":
		os.Exit(m.Run())
	}
	args := os.Args[1:]
	if os.Getenv("FAKE_YTDLP") == "live" {
		for _, a := range args {
			switch a {
			case "--version":
				fmt.Println("2026.09.27")
				os.Exit(0)
			case "-J":
				format := `{"format_id": "95", "ext": "mp4", "protocol": "m3u8_native", "height": 720, "url": "https://example.invalid/720.m3u8",
					 "manifest_url": %q, "http_headers": {"X-Live": "1"}}`
				switch os.Getenv("FAKE_LIVE_KIND") {
				case "dash":
					format = `{"format_id": "v1", "ext": "mp4", "protocol": "http_dash_segments", "height": 720,
					 "manifest_url": %q, "http_headers": {"X-Live": "1"}}`
				case "flv":
					format = `{"format_id": "flv-ld", "ext": "flv", "protocol": "https", "url": "https://example.invalid/ld.flv"},
					 {"format_id": "flv-hd", "ext": "flv", "protocol": "https", "url": %q, "http_headers": {"X-Live": "1"}}`
				}
				fmt.Printf(`{"title": "Launch", "is_live": true, "live_status": "is_live", "formats": [`+format+`]}`, os.Getenv("FAKE_LIVE_MANIFEST"))
				os.Exit(0)
			}
		}
		os.Exit(2)
	}
	value := func(flag string) string {
		for i, a := range args {
			if a == flag && i+1 < len(args) {
				return args[i+1]
			}
		}
		return ""
	}
	for _, a := range args {
		switch a {
		case "--version":
			fmt.Println("2026.09.27")
			os.Exit(0)
		case "-J":
			fmt.Print(`{"title": "A talk", "duration": 10, "formats": [
				{"format_id": "137", "ext": "mp4", "vcodec": "avc1.640028", "acodec": "none", "height": 1080, "tbr": 800, "protocol": "https"},
				{"format_id": "140", "ext": "m4a", "vcodec": "none", "acodec": "mp4a.40.2", "abr": 128, "tbr": 128, "protocol": "https"}]}`)
			os.Exit(0)
		}
	}
	out := strings.ReplaceAll(strings.TrimSuffix(value("-o"), ".%(ext)s"), "%%", "%")
	os.WriteFile(filepath.Join(filepath.Dir(out), "args.txt"), []byte(strings.Join(args, "\n")), 0o644)
	for _, f := range strings.Split(value("-f"), "+") {
		for _, n := range []int{0, 600, 1200} {
			fmt.Printf("odm downloading %d 1200 NA %s\n", n, f)
		}
		if os.Getenv("FAKE_YTDLP") == "slow" {
			time.Sleep(time.Minute)
		}
	}
	os.WriteFile(out+".mkv", []byte("joined video"), 0o644)
	os.Exit(0)
}

// fakeFFmpegMain is ffmpeg for FAKE_FFMPEG: the output is the last
// argument, the input the one after -i.
func fakeFFmpegMain() {
	args := os.Args[1:]
	if os.Getenv("FAKE_FFMPEG") == "fail" {
		fmt.Fprintln(os.Stderr, "Invalid data found when processing input")
		os.Exit(1)
	}
	in, concat := "", false
	var inputs []string
	for i, a := range args {
		if a == "-i" && i+1 < len(args) {
			in = args[i+1]
			inputs = append(inputs, in)
		}
		if a == "-f" && i+1 < len(args) && args[i+1] == "concat" {
			concat = true
		}
	}
	if len(inputs) == 2 {
		v, verr := os.ReadFile(inputs[0])
		a, aerr := os.ReadFile(inputs[1])
		if verr != nil || aerr != nil {
			os.Exit(1)
		}
		os.WriteFile(args[len(args)-1], []byte("merged:"+string(v)+"|"+string(a)), 0o644)
		os.Exit(0)
	}
	data, err := os.ReadFile(in)
	if err != nil {
		os.Exit(1)
	}
	if concat {
		// The list: file '<path>' per line, a ' written as '\''.
		var joined []byte
		for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
			path := strings.ReplaceAll(strings.TrimSuffix(strings.TrimPrefix(line, "file '"), "'"), `'\''`, "'")
			part, err := os.ReadFile(path)
			if err != nil {
				os.Exit(1)
			}
			joined = append(joined, part...)
		}
		os.WriteFile(args[len(args)-1], append([]byte("joined:"), joined...), 0o644)
		os.Exit(0)
	}
	os.WriteFile(args[len(args)-1], append([]byte("remuxed:"), data...), 0o644)
	os.Exit(0)
}

// fakeFFmpeg makes downloads run this test binary as ffmpeg.
func fakeFFmpeg(t *testing.T, mode string) {
	t.Helper()
	t.Setenv("FAKE_FFMPEG", mode)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	setFFmpegPath(exe)
	t.Cleanup(func() { setFFmpegPath("") })
}

// fakeYtDlp makes the manager run this test binary as yt-dlp.
func fakeYtDlp(t *testing.T, mode string) *Manager {
	t.Helper()
	t.Setenv("FAKE_YTDLP", mode)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	m := NewManager(Settings{Dir: t.TempDir(), YtDlpPath: exe}, "")
	t.Cleanup(func() { setYtDlpPath("") })
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	m.Begin(ctx)
	return m
}

func TestProbeSite(t *testing.T) {
	m := fakeYtDlp(t, "ok")
	c, err := m.ProbeSite(context.Background(), "https://www.youtube.com/watch?v=abc")
	if err != nil {
		t.Fatal(err)
	}
	if c.Title != "A talk" || len(c.Qualities) != 2 || c.Qualities[0].Format != "137+140" || c.Qualities[0].Ext != "mp4" {
		t.Fatalf("%+v", c)
	}
	if c.Quality(c.Qualities[1].Label) != 1 || c.Quality("nope") != -1 {
		t.Fatal("Quality")
	}

	setYtDlpPath(filepath.Join(t.TempDir(), "missing"))
	if _, err := m.ProbeSite(context.Background(), "https://youtu.be/abc"); !errors.Is(err, ErrNoYtDlp) {
		t.Fatalf("without yt-dlp: %v", err)
	}
}

func TestSiteDownload(t *testing.T) {
	m := fakeYtDlp(t, "ok")
	dir := t.TempDir()
	info := m.Create(AddOptions{
		URL: "https://www.youtube.com/watch?v=abc", Dir: dir, FileName: "A talk.mp4", Start: true,
		Site:        &SiteJob{Format: "137+140", Estimate: 5000},
		Headers:     map[string]string{"Cookie": "SID=secret"},
		Connections: 4, SpeedLimit: 9000,
	})
	waitFor(t, "the download to finish", func() bool {
		i, _ := m.Get(info.ID)
		return i.Status == StatusComplete || i.Status == StatusError
	})
	got, _ := m.Get(info.ID)
	if got.Status != StatusComplete || !got.Site {
		t.Fatalf("status %s, error %q", got.Status, got.Error)
	}
	// yt-dlp chose Matroska, so the name follows.
	if got.FileName != "A talk.mkv" || got.Total != int64(len("joined video")) {
		t.Fatalf("file %q, total %d", got.FileName, got.Total)
	}
	data, err := os.ReadFile(filepath.Join(dir, "A talk.mkv"))
	if err != nil || string(data) != "joined video" {
		t.Fatalf("file: %q %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "A talk.mp4.odmparts")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("the work folder was left behind")
	}
	states, _ := m.Export()
	if strings.Contains(string(states), "secret") {
		t.Fatal("the browser's cookie was kept for a video page")
	}
}

func TestSiteDownloadArgs(t *testing.T) {
	m := fakeYtDlp(t, "slow")
	dir := t.TempDir()
	info := m.Create(AddOptions{
		URL: "https://www.youtube.com/watch?v=abc", Dir: dir, FileName: "A talk.mp4", Start: true,
		Site: &SiteJob{Format: "137+140", Estimate: 5000}, Connections: 4, SpeedLimit: 9000,
	})
	work := filepath.Join(dir, "A talk.mp4.odmparts")
	waitFor(t, "progress", func() bool {
		i, _ := m.Get(info.ID)
		return i.Done == 1200
	})
	i, _ := m.Get(info.ID)
	if i.Total != 5000 { // one format's size known: the estimate stands
		t.Fatalf("total %d", i.Total)
	}
	data, _ := os.ReadFile(filepath.Join(work, "args.txt"))
	args := string(data)
	for _, want := range []string{"\n-f\n137+140\n", "\n--limit-rate\n9000\n", "\n-N\n4\n", "\n--\nhttps://www.youtube.com/watch?v=abc"} {
		if !strings.Contains(args, want) {
			t.Errorf("args lack %q:\n%s", want, args)
		}
	}
	if strings.Contains(args, "--cookies-from-browser") {
		t.Errorf("yt-dlp was given a browser's cookies unasked:\n%s", args)
	}

	// Pause stops yt-dlp; the bytes count stays for the next run.
	m.Pause(info.ID)
	waitFor(t, "the pause", func() bool {
		i, _ := m.Get(info.ID)
		return i.Status == StatusPaused
	})
	states, _ := m.Export()
	if !strings.Contains(string(states), `"done": 1200`) {
		t.Fatalf("the progress was not kept:\n%s", states)
	}
	if err := m.Readdress(context.Background(), info.ID, "https://www.youtube.com/watch?v=other"); err == nil {
		t.Fatal("a video page changed its address")
	}
}

// A site download named after a page that had not named the video (TikTok
// keeps "TikTok - Make Your Day"), or not named at all, takes yt-dlp's
// title of the video; one with a name of its own keeps it.
func TestSiteDownloadNamedAfterTheVideo(t *testing.T) {
	m := fakeYtDlp(t, "ok")
	dir := t.TempDir()
	for _, c := range []struct{ given, want string }{
		{"TikTok_-_Make_Your_Day.mp4", "A_talk.mkv"},
		{"", "A_talk.mkv"},
		{"Cats.mp4", "Cats.mkv"},
	} {
		info := m.Create(AddOptions{
			URL: "https://www.tiktok.com/@someone/video/123", Dir: dir, FileName: c.given, Start: true,
			Site: &SiteJob{Format: "137+140"},
		})
		waitFor(t, "the download to finish", func() bool {
			i, _ := m.Get(info.ID)
			return i.Status == StatusComplete || i.Status == StatusError
		})
		got, _ := m.Get(info.ID)
		if got.Status != StatusComplete || got.FileName != c.want {
			t.Errorf("given %q: status %s, name %q, want %q (%s)", c.given, got.Status, got.FileName, c.want, got.Error)
		}
		m.RemoveWithFile(info.ID, true)
	}
}

func TestSiteTitleNames(t *testing.T) {
	for name, want := range map[string]bool{
		"": true, "video": true, "TikTok_-_Make_Your_Day.mp4": true, "TikTok - Make Your Day.mp4": true,
		"YouTube.webm": true, "Cats.mp4": false, "TikTok_dance.mp4": false,
	} {
		if NeedsSiteTitle(name) != want {
			t.Errorf("NeedsSiteTitle(%q) = %v", name, !want)
		}
	}
	for title, want := range map[string]string{
		"Balas @yolaaftwsr hayu yu ? #SquadRandom_ 🔥":                  "Balas_@yolaaftwsr_hayu_yu_#SquadRandom_🔥",
		"Tag 1 Friend reverse this Video and look what happens 🤩😱 ...": "Tag_1_Friend_reverse_this_Video_and_look_what_happens_🤩😱",
		"a/b: c":                 "a_b_c",
		strings.Repeat("x", 200): strings.Repeat("x", maxSiteTitle),
		"   ":                    "",
	} {
		if got := siteTitleName(title); got != want {
			t.Errorf("siteTitleName(%q) = %q, want %q", title, got, want)
		}
	}
}

// The browser chosen in the Options goes to yt-dlp, which signs in with
// its cookies; an unknown one reads as none.
func TestSiteDownloadSignsInWhenAsked(t *testing.T) {
	m := fakeYtDlp(t, "slow")
	t.Cleanup(func() { setYtDlpCookies("") })
	st := m.Settings()
	st.YtDlpCookies = " Firefox "
	m.UpdateSettings(st)
	if got := m.Settings().YtDlpCookies; got != "firefox" {
		t.Fatalf("browser %q", got)
	}
	dir := t.TempDir()
	info := m.Create(AddOptions{
		URL: "https://www.youtube.com/watch?v=abc", Dir: dir, FileName: "A talk.mp4", Start: true,
		Site: &SiteJob{Format: "137+140"},
	})
	waitFor(t, "progress", func() bool {
		i, _ := m.Get(info.ID)
		return i.Done == 1200
	})
	data, _ := os.ReadFile(filepath.Join(dir, "A talk.mp4.odmparts", "args.txt"))
	if !strings.Contains(string(data), "\n--cookies-from-browser\nfirefox\n") {
		t.Fatalf("args lack the browser:\n%s", data)
	}
	m.Pause(info.ID)
	waitFor(t, "the pause", func() bool {
		i, _ := m.Get(info.ID)
		return i.Status == StatusPaused
	})

	st.YtDlpCookies = "netscape"
	m.UpdateSettings(st)
	if got := m.Settings().YtDlpCookies; got != "" {
		t.Fatalf("an unknown browser was kept: %q", got)
	}
}

// With the extension's cookies chosen, yt-dlp gets the ones it sent for
// the page's site, in a cookie file.
func TestSiteDownloadUsesTheExtensionsCookies(t *testing.T) {
	m := fakeYtDlp(t, "slow")
	t.Cleanup(func() {
		setYtDlpCookies("")
		forgetSiteCookies()
	})
	st := m.Settings()
	st.YtDlpCookies = CookiesFromExtension
	m.UpdateSettings(st)
	m.SetSiteCookies("https://www.youtube.com/watch?v=other", []BrowserCookie{{Name: "SID", Value: "s1", Domain: ".youtube.com", Path: "/", Secure: true}})
	dir := t.TempDir()
	info := m.Create(AddOptions{
		URL: "https://www.youtube.com/watch?v=abc", Dir: dir, FileName: "A talk.mp4", Start: true,
		Site: &SiteJob{Format: "137+140"},
	})
	waitFor(t, "progress", func() bool {
		i, _ := m.Get(info.ID)
		return i.Done == 1200
	})
	data, _ := os.ReadFile(filepath.Join(dir, "A talk.mp4.odmparts", "args.txt"))
	args := strings.Split(string(data), "\n")
	file := ""
	for i, a := range args {
		if a == "--cookies" && i+1 < len(args) {
			file = args[i+1]
		}
	}
	if cookies, err := os.ReadFile(file); err != nil || !strings.Contains(string(cookies), "\tSID\ts1\n") {
		t.Fatalf("cookie file %q: %q %v\nargs:\n%s", file, cookies, err, data)
	}
	m.Pause(info.ID)
	waitFor(t, "the pause", func() bool {
		i, _ := m.Get(info.ID)
		return i.Status == StatusPaused
	})
	if _, err := os.Stat(file); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("the cookie file outlived yt-dlp")
	}
}
