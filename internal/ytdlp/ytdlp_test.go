package ytdlp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestMain makes the test binary stand in for yt-dlp when FAKE_YTDLP is
// set: "--version" prints a version, -J prints the fixture, and a download
// reports progress for each format of -f and writes the result to -o.
// FAKE_YTDLP=slow keeps it running until it is killed.
func TestMain(m *testing.M) {
	mode := os.Getenv("FAKE_YTDLP")
	if mode == "" {
		os.Exit(m.Run())
	}
	args := os.Args[1:]
	value := func(flag string) string {
		for i, a := range args {
			if a == flag && i+1 < len(args) {
				return args[i+1]
			}
		}
		return ""
	}
	switch {
	case len(args) == 1 && args[0] == "--version":
		fmt.Println("2026.09.27")
	case value("-J") != "" || contains(args, "-J"):
		data, _ := os.ReadFile(filepath.Join("testdata", "youtube.json"))
		os.Stdout.Write(data)
	case mode == "fail":
		fmt.Fprintln(os.Stderr, "WARNING: something minor")
		fmt.Fprintln(os.Stderr, "ERROR: [youtube] abc: Video unavailable")
		os.Exit(1)
	default:
		out := strings.ReplaceAll(strings.TrimSuffix(value("-o"), ".%(ext)s"), "%%", "%")
		os.WriteFile(filepath.Join(filepath.Dir(out), "args.txt"), []byte(strings.Join(args, "\n")), 0o644)
		for _, f := range strings.Split(value("-f"), "+") {
			for _, n := range []int{0, 500, 1000} {
				fmt.Printf("odm downloading %d 1000 NA %s NA\n", n, f)
			}
			fmt.Printf("odm finished 1000 1000 NA %s NA\n", f)
			if mode == "slow" {
				time.Sleep(time.Minute)
			}
		}
		fmt.Println("[Merger] Merging formats into \"x\"")
		os.WriteFile(out+".mp4", []byte("video"), 0o644)
	}
	os.Exit(0)
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func fixture(t *testing.T) *Video {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "youtube.json"))
	if err != nil {
		t.Fatal(err)
	}
	var v Video
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatal(err)
	}
	return &v
}

func TestQualities(t *testing.T) {
	qs := fixture(t).Qualities()
	got := map[string]Quality{}
	var order []string
	for _, q := range qs {
		got[q.Label] = q
		order = append(order, q.Label)
	}
	want := []string{
		"240p · 313 kbps · H.264, AAC · MP4",
		"144p · 212 kbps · H.264, AAC · MP4",
		"Audio only · 130 kbps · AAC · M4A",
		"Audio only · 106 kbps · Opus · WEBM",
	}
	if strings.Join(order, "\n") != strings.Join(want, "\n") {
		t.Fatalf("qualities:\n%s", strings.Join(order, "\n"))
	}
	// H.264 over AV1 and VP9, a single file over the HLS copy, and the
	// audio that fits MP4, without the DRC copies.
	if q := got[want[0]]; q.Format != "133+140" || q.Ext != "mp4" || q.Estimate != 433081+309288 {
		t.Fatalf("240p: %+v", q)
	}
	if q := got[want[3]]; q.Format != "251" || q.Ext != "webm" {
		t.Fatalf("opus: %+v", q)
	}
	for _, q := range qs {
		if strings.Contains(q.Format, "drc") {
			t.Fatalf("DRC audio offered: %+v", q)
		}
	}
}

func TestQualitiesOfOtherSites(t *testing.T) {
	v := Video{Duration: 100, Formats: []Format{
		// A site with muxed HLS only, two qualities and a DRM one.
		{ID: "hls-1080", Ext: "mp4", VCodec: "avc1.640028", ACodec: "mp4a.40.2", Height: 1080, TBR: 4000, Protocol: "m3u8_native"},
		{ID: "hls-480", Ext: "mp4", VCodec: "avc1.4d401e", ACodec: "mp4a.40.2", Height: 480, TBR: 900, Protocol: "m3u8_native"},
		{ID: "drm", Ext: "mp4", VCodec: "avc1", ACodec: "mp4a", Height: 2160, DRM: true},
		// 60 fps HDR VP9 next to 30 fps H.264 at the same height.
		{ID: "vp9hdr", Ext: "webm", VCodec: "vp09.02", Height: 1080, FPS: 60, DynamicRange: "HDR10", TBR: 6000, Protocol: "https"},
		{ID: "opus", Ext: "webm", VCodec: "none", ACodec: "opus", ABR: 160, Protocol: "https"},
	}}
	var labels []string
	for _, q := range v.Qualities() {
		labels = append(labels, q.Format+" "+q.Ext+" | "+q.Label)
	}
	want := []string{
		"vp9hdr+opus webm | 1080p60 HDR · 6.2 Mbps · VP9, Opus · WEBM",
		"hls-1080 mp4 | 1080p · 4.0 Mbps · H.264, AAC · MP4",
		"hls-480 mp4 | 480p · 900 kbps · H.264, AAC · MP4",
		"opus webm | Audio only · 160 kbps · Opus · WEBM",
	}
	if strings.Join(labels, "\n") != strings.Join(want, "\n") {
		t.Fatalf("qualities:\n%s", strings.Join(labels, "\n"))
	}
}

func TestSite(t *testing.T) {
	for raw, want := range map[string]string{
		"https://www.youtube.com/watch?v=abc":   "YouTube",
		"https://youtu.be/abc":                  "YouTube",
		"https://music.youtube.com/watch?v=abc": "YouTube",
		"https://example.com/watch?v=abc":       "",
		"ftp://youtube.com/x":                   "",
		"not a url":                             "",
	} {
		if got := Site(raw); got != want {
			t.Errorf("Site(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestParseProgress(t *testing.T) {
	for line, want := range map[string]Progress{
		"odm downloading 1024 195278 NA 160 NA":        {Format: "160", Done: 1024, Total: 195278},
		"odm downloading 2048 NA 50000.5 hls-720 1e6":  {Format: "hls-720", Done: 2048, Total: 50000},
		"odm finished 117526 117526 NA 139 1676762.84": {Format: "139", Done: 117526, Total: 117526},
		`[Merger] Merging formats into "media.mp4"`:    {Merging: true},
	} {
		got, ok := parseProgress(line)
		if !ok || got != want {
			t.Errorf("%q: %+v %v, want %+v", line, got, ok, want)
		}
	}
	for _, line := range []string{"[download] Destination: x", "odm too few", ""} {
		if _, ok := parseProgress(line); ok {
			t.Errorf("%q read as progress", line)
		}
	}
}

func fakeBin(t *testing.T, mode string) string {
	t.Helper()
	t.Setenv("FAKE_YTDLP", mode)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return exe
}

func TestDownload(t *testing.T) {
	bin := fakeBin(t, "ok")
	dir := filepath.Join(t.TempDir(), "100% work") // a % must not read as a template
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Leftovers of an earlier run are not the result.
	os.WriteFile(filepath.Join(dir, "media.f137.mp4.part"), nil, 0o644)
	var reports []Progress
	file, err := Download(context.Background(), bin, Options{
		Page: "https://www.youtube.com/watch?v=abc", Format: "137+140",
		Out: filepath.Join(dir, "media"), FFmpeg: "C:/ff/ffmpeg.exe", RateLimit: 5000, Fragments: 4,
	}, func(p Progress) { reports = append(reports, p) })
	if err != nil {
		t.Fatal(err)
	}
	if file != filepath.Join(dir, "media.mp4") {
		t.Fatalf("result %q", file)
	}
	if n := len(reports); n != 9 || reports[3] != (Progress{Format: "137", Done: 1000, Total: 1000}) || !reports[8].Merging {
		t.Fatalf("reports %+v", reports)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "args.txt"))
	args := strings.Split(string(data), "\n")
	for _, want := range []string{"--ignore-config", "--no-playlist", "--limit-rate", "5000", "-N", "4", "--ffmpeg-location", "C:/ff/ffmpeg.exe"} {
		if !contains(args, want) {
			t.Errorf("args lack %q: %q", want, args)
		}
	}
	if args[len(args)-2] != "--" || args[len(args)-1] != "https://www.youtube.com/watch?v=abc" {
		t.Fatalf("the page does not come last, after --: %q", args)
	}
}

func TestDownloadFailureSaysWhy(t *testing.T) {
	bin := fakeBin(t, "fail")
	_, err := Download(context.Background(), bin, Options{Page: "https://youtu.be/abc", Out: filepath.Join(t.TempDir(), "media")}, nil)
	if err == nil || err.Error() != "yt-dlp: [youtube] abc: Video unavailable" {
		t.Fatalf("error %v", err)
	}
}

// Pausing stops yt-dlp at once, the process it started included.
func TestDownloadStops(t *testing.T) {
	bin := fakeBin(t, "slow")
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	var once bool
	go func() {
		<-started
		cancel()
	}()
	begin := time.Now()
	_, err := Download(ctx, bin, Options{Page: "https://youtu.be/abc", Format: "1", Out: filepath.Join(t.TempDir(), "media")},
		func(Progress) {
			if !once {
				once = true
				close(started)
			}
		})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error %v", err)
	}
	if d := time.Since(begin); d > 15*time.Second {
		t.Fatalf("stopping took %v", d)
	}
}

func TestProbe(t *testing.T) {
	bin := fakeBin(t, "ok")
	v, err := Probe(context.Background(), bin, "https://www.youtube.com/watch?v=jNQXAC9IVRw")
	if err != nil {
		t.Fatal(err)
	}
	if v.Title != "Me at the zoo" || v.Duration != 19 || len(v.Formats) != 24 {
		t.Fatalf("video %q %v %d formats", v.Title, v.Duration, len(v.Formats))
	}
}

func TestLocate(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, exeName())
	if err := os.WriteFile(bin, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got, err := Locate(dir); err != nil || got != bin {
		t.Fatalf("folder: %q %v", got, err)
	}
	if _, err := Locate(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("a missing path was accepted")
	}
}

func TestLiveManifest(t *testing.T) {
	v := Video{LiveStatus: "is_live", Formats: []Format{
		{ID: "sb0", Protocol: "mhtml", URL: "https://i/sb"},
		{ID: "94", Protocol: "m3u8_native", Height: 480, URL: "https://m/480.m3u8"},
		{ID: "95", Protocol: "m3u8_native", Height: 720, URL: "https://m/720.m3u8", Headers: map[string]string{"User-Agent": "yt"}},
	}}
	// Without a master playlist, the best quality's own playlist.
	if m, h, ok := v.LiveManifest(); !v.IsLive() || !ok || m != "https://m/720.m3u8" || h["User-Agent"] != "yt" {
		t.Fatalf("media playlist: %q %v %v", m, h, ok)
	}
	v.Formats[1].ManifestURL = "https://m/master.m3u8"
	if m, _, ok := v.LiveManifest(); !ok || m != "https://m/master.m3u8" {
		t.Fatalf("master playlist: %q", m)
	}
	if _, _, ok := (&Video{Formats: []Format{{Protocol: "https", URL: "https://f/v.flv"}}}).LiveManifest(); ok {
		t.Fatal("a live stream without HLS gave a playlist")
	}
}
