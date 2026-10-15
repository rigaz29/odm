package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestMain lets the test binary stand in for yt-dlp when FAKE_YTDLP is set
// (see fakeYtDlp): it reports progress for each format of -f and writes
// the joined file to -o, or, for -J, describes a two-quality video.
func TestMain(m *testing.M) {
	if os.Getenv("FAKE_YTDLP") == "" {
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
