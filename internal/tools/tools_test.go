package tools

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fakeGitHub serves releases the way api.github.com and its download
// links do. Each release maps asset names to their contents; a checksum
// list is made for the assets unless sums overrides it.
type fakeGitHub struct {
	srv       *httptest.Server
	releases  map[string][]fakeRelease // repo -> newest first
	sums      map[string]string        // asset -> checksum line override
	downloads atomic.Int32
}

type fakeRelease struct {
	tag    string
	draft  bool
	assets map[string][]byte
	sums   string // the checksum list's asset name
}

func newFakeGitHub(t *testing.T) *fakeGitHub {
	g := &fakeGitHub{releases: map[string][]fakeRelease{}, sums: map[string]string{}}
	g.srv = httptest.NewServer(http.HandlerFunc(g.serve))
	t.Cleanup(g.srv.Close)
	return g
}

func (g *fakeGitHub) list(r fakeRelease) map[string]any {
	var assets []map[string]any
	for name, data := range r.assets {
		assets = append(assets, map[string]any{"name": name, "browser_download_url": g.srv.URL + "/dl/" + r.tag + "/" + name, "size": len(data)})
	}
	if r.sums != "" {
		assets = append(assets, map[string]any{"name": r.sums, "browser_download_url": g.srv.URL + "/dl/" + r.tag + "/" + r.sums})
	}
	return map[string]any{"tag_name": r.tag, "draft": r.draft, "assets": assets}
}

func (g *fakeGitHub) serve(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Path
	for repo, rels := range g.releases {
		switch p {
		case "/repos/" + repo + "/releases/latest":
			json.NewEncoder(w).Encode(g.list(rels[0]))
			return
		case "/repos/" + repo + "/releases":
			var out []any
			for _, rel := range rels {
				out = append(out, g.list(rel))
			}
			json.NewEncoder(w).Encode(out)
			return
		}
		for _, rel := range rels {
			prefix := "/dl/" + rel.tag + "/"
			if !strings.HasPrefix(p, prefix) {
				continue
			}
			name := strings.TrimPrefix(p, prefix)
			if name == rel.sums {
				for asset, data := range rel.assets {
					line, ok := g.sums[asset]
					if !ok {
						sum := sha256.Sum256(data)
						line = hex.EncodeToString(sum[:]) + "  " + asset
					}
					fmt.Fprintln(w, line)
				}
				return
			}
			if data, ok := rel.assets[name]; ok {
				g.downloads.Add(1)
				w.Write(data)
				return
			}
		}
	}
	http.NotFound(w, r)
}

func zipOf(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		f, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		f.Write([]byte(body))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

var program = Tool{Name: "prog", Repo: "o/prog", Asset: "prog.exe", Sums: "SUMS", Files: []string{"prog.exe"}}

func installer(t *testing.T, g *fakeGitHub) *Installer {
	return &Installer{Dir: t.TempDir(), Client: g.srv.Client(), API: g.srv.URL}
}

func TestInstallAndUpdate(t *testing.T) {
	g := newFakeGitHub(t)
	g.releases["o/prog"] = []fakeRelease{{tag: "1.0", sums: "SUMS", assets: map[string][]byte{"prog.exe": []byte("version one")}}}
	in := installer(t, g)

	v, err := in.Install(context.Background(), program, nil)
	if err != nil || v != "1.0" || in.Installed(program) != "1.0" {
		t.Fatalf("install: %q %v", v, err)
	}
	path := filepath.Join(in.Dir, "prog.exe")
	if data, _ := os.ReadFile(path); string(data) != "version one" {
		t.Fatalf("installed %q", data)
	}
	// The same release again: nothing is downloaded.
	if _, err := in.Install(context.Background(), program, nil); err != nil || g.downloads.Load() != 1 {
		t.Fatalf("reinstall: %v, %d downloads", err, g.downloads.Load())
	}

	// A newer release replaces it, even while the old one runs: Windows
	// lets a running program be renamed, not overwritten.
	g.releases["o/prog"] = []fakeRelease{{tag: "2.0", sums: "SUMS", assets: map[string][]byte{"prog.exe": []byte("version two")}}}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o755); err != nil {
		t.Fatal(err)
	}
	running := exec.Command(path, "-test.run=TestSleeper")
	running.Env = append(os.Environ(), "TOOLS_SLEEPER=1")
	if err := running.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		running.Process.Kill()
		running.Wait()
	}()
	time.Sleep(300 * time.Millisecond)
	var reports int
	v, err = in.Install(context.Background(), program, func(done, total int64) { reports++ })
	if err != nil || v != "2.0" || reports == 0 {
		t.Fatalf("update: %q %v (%d reports)", v, err, reports)
	}
	if data, _ := os.ReadFile(path); string(data) != "version two" {
		t.Fatalf("updated %q", data)
	}
	if left, _ := filepath.Glob(filepath.Join(in.Dir, ".*")); len(left) != 0 {
		t.Fatalf("temporary files left: %v", left)
	}
}

// TestSleeper stands in for a running program in TestInstallAndUpdate.
func TestSleeper(t *testing.T) {
	if os.Getenv("TOOLS_SLEEPER") == "" {
		t.Skip("only run as a helper")
	}
	time.Sleep(30 * time.Second)
}

func TestInstallRefusesWhatDoesNotMatch(t *testing.T) {
	g := newFakeGitHub(t)
	g.releases["o/prog"] = []fakeRelease{{tag: "1.0", sums: "SUMS", assets: map[string][]byte{"prog.exe": []byte("tampered")}}}
	g.sums["prog.exe"] = strings.Repeat("ab", 32) + "  prog.exe"
	in := installer(t, g)
	if _, err := in.Install(context.Background(), program, nil); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("a mismatching download: %v", err)
	}
	if entries, _ := os.ReadDir(in.Dir); len(entries) != 0 {
		t.Fatalf("something was kept: %v", entries)
	}

	g.sums["prog.exe"] = strings.Repeat("ab", 32) + "  other.exe"
	if _, err := in.Install(context.Background(), program, nil); err == nil || !strings.Contains(err.Error(), "no checksum") {
		t.Fatalf("no checksum for it: %v", err)
	}
	g.releases["o/prog"][0].sums = ""
	if _, err := in.Install(context.Background(), program, nil); err == nil {
		t.Fatal("a release without checksums was installed")
	}
}

func TestInstallFromZipByTagPrefix(t *testing.T) {
	g := newFakeGitHub(t)
	ff := Tool{Name: "ffmpeg", Repo: "me/odm", TagPrefix: "ffmpeg-min-", Asset: "ffmpeg-min-win64.zip",
		Sums: "ffmpeg-min-win64.zip.sha256", Files: []string{"ffmpeg.exe", "ffprobe.exe"}}
	z := zipOf(t, map[string]string{"bin/ffmpeg.exe": "ffmpeg", "bin/ffprobe.exe": "ffprobe", "LICENSE.txt": "LGPL"})
	g.releases["me/odm"] = []fakeRelease{
		{tag: "v9.9.9", assets: map[string][]byte{"odm.zip": []byte("not this")}},
		{tag: "ffmpeg-min-n9", draft: true, sums: ff.Sums, assets: map[string][]byte{ff.Asset: []byte("draft")}},
		{tag: "ffmpeg-min-n8.1-1", sums: ff.Sums, assets: map[string][]byte{ff.Asset: z}},
	}
	in := installer(t, g)
	v, err := in.Install(context.Background(), ff, nil)
	if err != nil || v != "ffmpeg-min-n8.1-1" {
		t.Fatalf("install: %q %v", v, err)
	}
	for name, want := range map[string]string{"ffmpeg.exe": "ffmpeg", "ffprobe.exe": "ffprobe"} {
		if data, _ := os.ReadFile(filepath.Join(in.Dir, name)); string(data) != want {
			t.Fatalf("%s: %q", name, data)
		}
	}
	if _, err := os.Stat(filepath.Join(in.Dir, "LICENSE.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("a file not asked for was taken out")
	}
}

func TestChecksumList(t *testing.T) {
	list := "0000  short\n" + strings.Repeat("c", 64) + " *yt-dlp.exe\n" + strings.Repeat("d", 64) + "  yt-dlp_linux\n"
	if got, ok := checksum(strings.NewReader(list), "yt-dlp.exe"); !ok || got != strings.Repeat("c", 64) {
		t.Fatalf("%q %v", got, ok)
	}
	if _, ok := checksum(strings.NewReader(list), "short"); ok {
		t.Fatal("a short checksum was read")
	}
}

func TestKeeperModes(t *testing.T) {
	g := newFakeGitHub(t)
	yt := YtDlp()
	if !yt.Supported() {
		t.Skip("no yt-dlp build for this system")
	}
	g.releases[yt.Repo] = []fakeRelease{{tag: "2026.08.19", sums: yt.Sums, assets: map[string][]byte{yt.Asset: []byte("yt-dlp")}}}
	in := installer(t, g)
	mode := ModeAsk
	k := NewKeeper(in, func() string { return mode })
	asked := make(chan struct{}, 2)
	k.Ask = func() { asked <- struct{}{} }

	if _, err := k.Ensure(context.Background(), "yt-dlp"); !errors.Is(err, ErrNeedsConsent) {
		t.Fatalf("not asked yet: %v", err)
	}
	<-asked
	k.Ensure(context.Background(), "yt-dlp") // asked once only
	select {
	case <-asked:
		t.Fatal("asked twice")
	default:
	}

	mode = ModeOff
	if _, err := k.Ensure(context.Background(), "yt-dlp"); err == nil || g.downloads.Load() != 0 {
		t.Fatalf("turned off: %v", err)
	}

	mode = ModeAuto
	var installed []string
	k.OnInstalled = func(t Tool, v string, first bool) { installed = append(installed, fmt.Sprint(t.Name, v, first)) }
	p, err := k.Ensure(context.Background(), "yt-dlp")
	if err != nil || p != filepath.Join(in.Dir, yt.Files[0]) {
		t.Fatalf("auto: %q %v", p, err)
	}
	if len(installed) != 1 || installed[0] != "yt-dlp2026.08.19true" {
		t.Fatalf("reported %v", installed)
	}
	if _, err := k.Ensure(context.Background(), "nope"); err == nil {
		t.Fatal("an unknown tool")
	}
}
