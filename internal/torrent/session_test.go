package torrent

// These tests download torrents through the engine and a Session on the
// real odm-libtorrent, named by ODM_LIBTORRENT, from a second odm-libtorrent
// that seeds on 127.0.0.1; they skip without it. CI takes the program from
// the newest odm-libtorrent release.

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"odm/internal/engine"
)

func program(t *testing.T) string {
	t.Helper()
	p := os.Getenv("ODM_LIBTORRENT")
	if p == "" {
		t.Skip("ODM_LIBTORRENT is not set")
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

// local keeps odm-libtorrent on 127.0.0.1, with no DHT or discovery.
func local() map[string]any {
	return map[string]any{"listen": "127.0.0.1:0", "incoming": true, "port_forwarding": false,
		"dht": false, "lsd": false, "status_interval": 200}
}

func makeFile(t *testing.T, path string, size int) []byte {
	t.Helper()
	b := make([]byte, size)
	rand.Read(b)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return b
}

func sameFile(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s: its %d bytes are not the %d seeded", path, len(got), len(want))
	}
}

// createTorrent makes a .torrent of src and returns its v1 info hash.
func createTorrent(t *testing.T, src, out string) string {
	t.Helper()
	cmd := exec.Command(program(t), "create", src, out, "--piece-size", "32768")
	hide(cmd)
	b, err := cmd.Output()
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	var m struct {
		InfoHash string `json:"info_hash"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(b), &m); err != nil || m.InfoHash == "" {
		t.Fatalf("create printed %q", b)
	}
	return m.InfoHash
}

// seeder is a second odm-libtorrent seeding one torrent.
type seeder struct {
	c *Client
}

func seed(t *testing.T, torrent, dir string, uploadLimit int64) *seeder {
	t.Helper()
	c, err := Start(context.Background(), program(t), Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close(10 * time.Second) })
	ctx := context.Background()
	if err := c.Call(ctx, "settings", local(), nil); err != nil {
		t.Fatal(err)
	}
	if err := c.Call(ctx, "add", map[string]any{"key": "seed", "torrent": torrent, "save_path": dir,
		"upload_limit": uploadLimit}, nil); err != nil {
		t.Fatal(err)
	}
	for i := 0; ; i++ {
		var r struct{ Torrents []Status }
		if err := c.Call(ctx, "status", map[string]any{"key": "seed"}, &r); err != nil {
			t.Fatal(err)
		}
		if len(r.Torrents) == 1 && r.Torrents[0].Seeding {
			return &seeder{c: c}
		}
		if i > 300 {
			t.Fatalf("the seeder is not seeding: %+v", r.Torrents)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// connect has the seeder call the session, which listens on 127.0.0.1.
func (sd *seeder) connect(t *testing.T, s *Session) {
	t.Helper()
	if p := s.Port(); p > 0 {
		_ = sd.c.Call(context.Background(), "connect", map[string]any{"key": "seed", "peer": "127.0.0.1:" + strconv.Itoa(p)}, nil)
	}
}

// harness is a Session as the engine's torrents, and a manager.
type harness struct {
	t   *testing.T
	s   *Session
	m   *engine.Manager
	dir string
}

// newHarness sets engine.Torrents and engine.TorrentDir, which the tests
// share: none of them runs in parallel.
func newHarness(t *testing.T, store *engine.Store, torrentDir, dir string) *harness {
	t.Helper()
	prog := program(t)
	s := &Session{Program: func(context.Context) (string, error) { return prog, nil }, Settings: local, Log: t.Logf}
	prevT, prevD := engine.Torrents, engine.TorrentDir
	engine.Torrents, engine.TorrentDir = s, torrentDir
	// No seeding unless a test asks for it: a finished torrent leaves the
	// session.
	m := engine.NewManager(engine.Settings{Dir: dir, Concurrent: 3, Torrent: engine.TorrentSettings{Seed: engine.SeedNever}}, "")
	if store != nil {
		m.SetStore(store)
		if states, err := store.Load(); err == nil {
			m.Restore(states)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.Begin(ctx)
	h := &harness{t: t, s: s, m: m, dir: dir}
	t.Cleanup(func() {
		cancel()
		s.Close()
		engine.Torrents, engine.TorrentDir = prevT, prevD
	})
	return h
}

// stop closes the session as ODM does when it exits, and lets the next
// harness take over.
func (h *harness) stop() {
	h.m.StopAll()
	h.s.Close()
}

// wait waits for the download id to reach want, having the seeder call the
// session while it runs.
func (h *harness) wait(id string, sd *seeder, timeout time.Duration, what string, ok func(engine.Info) bool) engine.Info {
	h.t.Helper()
	until := time.Now().Add(timeout)
	next := time.Now()
	for {
		info, found := h.m.Get(id)
		if !found {
			h.t.Fatalf("%s is gone", id)
		}
		if ok(info) {
			return info
		}
		if info.Status == engine.StatusError && what != "an error" {
			h.t.Fatalf("waiting for %s: %s", what, info.Error)
		}
		if time.Now().After(until) {
			h.t.Fatalf("no %s in %v: %+v", what, timeout, info)
		}
		if sd != nil && time.Now().After(next) && info.Status == engine.StatusDownloading {
			sd.connect(h.t, h.s)
			next = time.Now().Add(time.Second)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func complete(i engine.Info) bool { return i.Status == engine.StatusComplete }

func TestSessionDownloadsAFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "seed", "data.bin")
	want := makeFile(t, src, 2<<20+5)
	tf := filepath.Join(dir, "data.torrent")
	createTorrent(t, src, tf)
	sd := seed(t, tf, filepath.Dir(src), 0)

	h := newHarness(t, nil, filepath.Join(dir, "torrents"), filepath.Join(dir, "got"))
	info := h.m.Create(engine.AddOptions{URL: tf, Torrent: &engine.TorrentJob{}, Start: true})
	got := h.wait(info.ID, sd, 60*time.Second, "the download", complete)
	sameFile(t, filepath.Join(h.dir, "data.bin"), want)
	if _, live := h.m.TorrentDetail(context.Background(), info.ID); live {
		t.Fatal("detail of a torrent that left the session")
	}
	if got.FileName != "data.bin" || got.Done != int64(len(want)) || !got.Torrent {
		t.Fatalf("info: %+v", got)
	}
	if h.s.in(info.ID) != nil {
		t.Fatal("the finished torrent is still in the session")
	}
	if _, err := os.Stat(filepath.Join(h.dir, "data.bin.odmpart")); !os.IsNotExist(err) {
		t.Fatalf("data.bin.odmpart: %v", err)
	}
}

func TestSessionDownloadsAFolderWithoutAFileLeftOut(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "seed", "Album")
	a := makeFile(t, filepath.Join(root, "a.bin"), 200000)
	makeFile(t, filepath.Join(root, "sub", "b.bin"), 300000)
	c := makeFile(t, filepath.Join(root, "c.bin"), 100001)
	tf := filepath.Join(dir, "album.torrent")
	createTorrent(t, root, tf)
	sd := seed(t, tf, filepath.Join(dir, "seed"), 0)

	h := newHarness(t, nil, filepath.Join(dir, "torrents"), filepath.Join(dir, "got"))
	meta, err := h.s.Inspect(context.Background(), tf, "")
	if err != nil {
		t.Fatal(err)
	}
	job := &engine.TorrentJob{Files: meta.Files}
	for i := range job.Files {
		job.Files[i].Skip = strings.HasSuffix(job.Files[i].Path, "/b.bin")
	}
	info := h.m.Create(engine.AddOptions{URL: tf, Torrent: job, Start: true})
	got := h.wait(info.ID, sd, 60*time.Second, "the download", complete)
	if got.FileName != "Album" || got.Total != int64(len(a)+len(c)) {
		t.Fatalf("info: %+v", got)
	}
	sameFile(t, filepath.Join(h.dir, "Album", "a.bin"), a)
	sameFile(t, filepath.Join(h.dir, "Album", "c.bin"), c)
	if _, err := os.Stat(filepath.Join(h.dir, "Album", "sub", "b.bin")); !os.IsNotExist(err) {
		t.Fatalf("b.bin was downloaded: %v", err)
	}
	if left, _ := filepath.Glob(filepath.Join(h.dir, "Album", ".*.parts")); len(left) > 0 {
		t.Fatalf("part file left: %v", left)
	}
}

// A file raised while the torrent runs is taken at once, and one left out
// while it is paused in the session stays out when it resumes.
func TestSessionTakesFilesChosenAfterTheAdd(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "seed", "Set")
	a := makeFile(t, filepath.Join(root, "a.bin"), 1<<20)
	makeFile(t, filepath.Join(root, "b.bin"), 2<<20)
	c := makeFile(t, filepath.Join(root, "c.bin"), 1<<20)
	tf := filepath.Join(dir, "set.torrent")
	createTorrent(t, root, tf)
	// 1 MiB/s: the download takes about four seconds.
	sd := seed(t, tf, filepath.Join(dir, "seed"), 1<<20)

	h := newHarness(t, nil, filepath.Join(dir, "torrents"), filepath.Join(dir, "got"))
	info := h.m.Create(engine.AddOptions{URL: tf, Torrent: &engine.TorrentJob{}, Start: true})
	h.wait(info.ID, sd, 30*time.Second, "256 KiB", func(i engine.Info) bool { return i.Done >= 256<<10 })
	tj, _ := h.m.Torrent(info.ID)
	index := map[string]int{}
	for i, f := range tj.Files {
		index[path.Base(f.Path)] = i
	}
	if err := h.m.SetTorrentFiles(info.ID, map[int]int{index["c.bin"]: engine.PriorityHigh}); err != nil {
		t.Fatal(err)
	}
	h.m.Pause(info.ID)
	h.wait(info.ID, nil, 10*time.Second, "a pause", func(i engine.Info) bool { return i.Status == engine.StatusPaused })
	if h.s.in(info.ID) == nil {
		t.Fatal("the paused torrent left the session")
	}
	if err := h.m.SetTorrentFiles(info.ID, map[int]int{index["b.bin"]: 0}); err != nil {
		t.Fatal(err)
	}
	h.m.Resume(info.ID)
	got := h.wait(info.ID, sd, 60*time.Second, "the download", complete)
	if got.Total != int64(len(a)+len(c)) {
		t.Fatalf("total %d, want %d", got.Total, len(a)+len(c))
	}
	sameFile(t, filepath.Join(h.dir, "Set", "a.bin"), a)
	sameFile(t, filepath.Join(h.dir, "Set", "c.bin"), c)
	if st, err := os.Stat(filepath.Join(h.dir, "Set", "b.bin")); err == nil && st.Size() >= 2<<20 {
		if b, _ := os.ReadFile(filepath.Join(h.dir, "Set", "b.bin")); bytes.Count(b, []byte{0}) < len(b)/4 {
			t.Fatal("b.bin, left out, was downloaded whole")
		}
	}
}

// A file left out of a finished torrent that seeds, in the session, is
// added: the torrent downloads it in its folder and is complete again.
// One that left the session takes another file the same way, its files
// checked again.
func TestSessionAddsFilesAfterItFinished(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "seed", "Set")
	a := makeFile(t, filepath.Join(root, "a.bin"), 300000)
	b := makeFile(t, filepath.Join(root, "b.bin"), 200001)
	c := makeFile(t, filepath.Join(root, "c.bin"), 100003)
	tf := filepath.Join(dir, "set.torrent")
	createTorrent(t, root, tf)
	sd := seed(t, tf, filepath.Join(dir, "seed"), 0)

	h := newHarness(t, nil, filepath.Join(dir, "torrents"), filepath.Join(dir, "got"))
	meta, err := h.s.Inspect(context.Background(), tf, "")
	if err != nil {
		t.Fatal(err)
	}
	job := &engine.TorrentJob{Files: meta.Files, Seed: &engine.TorrentSeed{Mode: engine.SeedForever}}
	index := map[string]int{}
	for i := range job.Files {
		index[path.Base(job.Files[i].Path)] = i
		job.Files[i].Skip = strings.HasSuffix(job.Files[i].Path, "/b.bin") || strings.HasSuffix(job.Files[i].Path, "/c.bin")
	}
	info := h.m.Create(engine.AddOptions{URL: tf, Torrent: job, Start: true})
	h.wait(info.ID, sd, 60*time.Second, "the download", func(i engine.Info) bool { return i.Status == engine.StatusComplete && i.Seeding })
	folder := filepath.Join(h.dir, "Set")
	sameFile(t, filepath.Join(folder, "a.bin"), a)

	if err := h.m.SetTorrentFiles(info.ID, map[int]int{index["b.bin"]: engine.PriorityNormal}); err != nil {
		t.Fatal(err)
	}
	got := h.wait(info.ID, sd, 60*time.Second, "b.bin", func(i engine.Info) bool { return i.Status == engine.StatusComplete && i.Total == int64(len(a)+len(b)) })
	if got.FilePath != folder {
		t.Fatalf("complete again at %s", got.FilePath)
	}
	sameFile(t, filepath.Join(folder, "b.bin"), b)
	h.wait(info.ID, nil, 10*time.Second, "seeding again", func(i engine.Info) bool { return i.Seeding })

	// Out of the session: it is added again, and checks what it has.
	h.m.StopSeeding(info.ID)
	h.wait(info.ID, nil, 10*time.Second, "the end of seeding", func(i engine.Info) bool { return !i.Seeding })
	if h.s.in(info.ID) != nil {
		t.Fatal("still in the session")
	}
	if err := h.m.SetTorrentFiles(info.ID, map[int]int{index["c.bin"]: engine.PriorityNormal}); err != nil {
		t.Fatal(err)
	}
	h.wait(info.ID, sd, 60*time.Second, "c.bin", func(i engine.Info) bool {
		return i.Status == engine.StatusComplete && i.Total == int64(len(a)+len(b)+len(c))
	})
	sameFile(t, filepath.Join(folder, "a.bin"), a)
	sameFile(t, filepath.Join(folder, "b.bin"), b)
	sameFile(t, filepath.Join(folder, "c.bin"), c)
}

func TestSessionDownloadsAMagnetLink(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "seed", "data.bin")
	want := makeFile(t, src, 1<<20+3)
	tf := filepath.Join(dir, "data.torrent")
	ih := createTorrent(t, src, tf)
	sd := seed(t, tf, filepath.Dir(src), 0)

	h := newHarness(t, nil, filepath.Join(dir, "torrents"), filepath.Join(dir, "got"))
	info := h.m.Create(engine.AddOptions{URL: "magnet:?xt=urn:btih:" + ih + "&dn=Data", Start: true})
	if info.FileName != "Data" {
		t.Fatalf("before the metadata: %+v", info)
	}
	h.wait(info.ID, sd, 60*time.Second, "the download", complete)
	sameFile(t, filepath.Join(h.dir, "data.bin"), want)
}

func TestSessionResumesAfterODMRestarts(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "seed", "big.bin")
	want := makeFile(t, src, 6<<20)
	tf := filepath.Join(dir, "big.torrent")
	createTorrent(t, src, tf)
	// 1 MiB/s: the download takes about six seconds.
	sd := seed(t, tf, filepath.Dir(src), 1<<20)
	store, err := engine.NewStore(filepath.Join(dir, "state"))
	if err != nil {
		t.Fatal(err)
	}
	torrents, got := filepath.Join(dir, "torrents"), filepath.Join(dir, "got")

	h := newHarness(t, store, torrents, got)
	info := h.m.Create(engine.AddOptions{URL: tf, Torrent: &engine.TorrentJob{}, Start: true})
	before := h.wait(info.ID, sd, 30*time.Second, "1 MiB", func(i engine.Info) bool { return i.Done >= 1<<20 })
	if before.Done == before.Total {
		t.Fatalf("the seeder's limit did not hold: %+v", before)
	}
	h.m.Pause(info.ID)
	h.wait(info.ID, nil, 10*time.Second, "a pause", func(i engine.Info) bool { return i.Status == engine.StatusPaused })
	h.stop()
	// The next ODM takes over.
	h2 := newHarness(t, store, torrents, got)
	restored, ok := h2.m.Get(info.ID)
	if !ok || restored.Done < before.Done/2 {
		t.Fatalf("restored %+v (had %d bytes)", restored, before.Done)
	}
	h2.m.Resume(info.ID)
	h2.wait(info.ID, sd, 60*time.Second, "the download", complete)
	sameFile(t, filepath.Join(got, "big.bin"), want)
}

func TestSessionStartsAgainAfterTheProgramStops(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "seed", "big.bin")
	want := makeFile(t, src, 4<<20)
	tf := filepath.Join(dir, "big.torrent")
	createTorrent(t, src, tf)
	sd := seed(t, tf, filepath.Dir(src), 1<<20)

	h := newHarness(t, nil, filepath.Join(dir, "torrents"), filepath.Join(dir, "got"))
	info := h.m.Create(engine.AddOptions{URL: tf, Torrent: &engine.TorrentJob{}, Start: true})
	h.wait(info.ID, sd, 30*time.Second, "some bytes", func(i engine.Info) bool { return i.Done > 0 })
	h.s.mu.Lock()
	c := h.s.c
	h.s.mu.Unlock()
	c.Kill()
	failed := h.wait(info.ID, nil, 15*time.Second, "an error", func(i engine.Info) bool { return i.Status == engine.StatusError })
	if !strings.Contains(failed.Error, "odm-libtorrent stopped") {
		t.Fatalf("error: %q", failed.Error)
	}
	h.m.Resume(info.ID)
	h.wait(info.ID, sd, 60*time.Second, "the download", complete)
	sameFile(t, filepath.Join(h.dir, "big.bin"), want)
	if fmt.Sprint(h.s.Port()) == "0" {
		t.Fatal("the program did not start again")
	}
}

// leech is a second odm-libtorrent that downloads the torrent into dir.
func leech(t *testing.T, torrent, dir string) *Client {
	t.Helper()
	c, err := Start(context.Background(), program(t), Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close(10 * time.Second) })
	if err := c.Call(context.Background(), "settings", local(), nil); err != nil {
		t.Fatal(err)
	}
	if err := c.Call(context.Background(), "add", map[string]any{"key": "peer", "torrent": torrent, "save_path": dir}, nil); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestSettingsFrom(t *testing.T) {
	st := engine.Settings{SpeedLimit: 5000, Torrent: engine.TorrentSettings{Port: 41000, NoPortForwarding: true, UploadLimit: 300},
		Proxy: engine.ProxyConfig{Mode: engine.ProxySOCKS5, Host: "p", Port: 1080, User: "u", Pass: "s"}}
	got := SettingsFrom(st, nil)
	want := map[string]any{"listen": "0.0.0.0:41000,[::]:41000", "incoming": true, "port_forwarding": false,
		"dht": true, "lsd": true, "download_limit": int64(5000), "upload_limit": int64(300)}
	for k, v := range want {
		if !reflect.DeepEqual(got[k], v) {
			t.Errorf("%s: %#v, want %#v", k, got[k], v)
		}
	}
	if p := got["proxy"].(map[string]any); p["type"] != "socks5" || p["peers"] != true || p["user"] != "u" {
		t.Errorf("SOCKS5 proxy: %v", p)
	}
	st.Proxy = engine.ProxyConfig{Mode: engine.ProxyHTTP, Host: "p", Port: 8080}
	if p := SettingsFrom(st, nil)["proxy"].(map[string]any); p["type"] != "http" || p["peers"] != false {
		t.Errorf("HTTP proxy: %v", p)
	}
	st.Proxy = engine.ProxyConfig{Mode: engine.ProxySystem}
	system := func(*http.Request) (*url.URL, error) { return url.Parse("http://corp:3128") }
	if p := SettingsFrom(st, system)["proxy"].(map[string]any); p["type"] != "http" || p["host"] != "corp" || p["port"] != 3128 {
		t.Errorf("system proxy: %v", p)
	}
	if p := SettingsFrom(st, nil)["proxy"].(map[string]any); p["type"] != "none" {
		t.Errorf("no system proxy: %v", p)
	}
	st.LimiterOff = true
	if got := SettingsFrom(st, nil)["download_limit"]; got != int64(0) {
		t.Errorf("limiter off: %v", got)
	}
}

func TestSessionStopsTheProgramWhenIdle(t *testing.T) {
	prev := idleStop
	idleStop = 300 * time.Millisecond
	t.Cleanup(func() { idleStop = prev })
	dir := t.TempDir()
	src := filepath.Join(dir, "seed", "big.bin")
	want := makeFile(t, src, 4<<20)
	tf := filepath.Join(dir, "big.torrent")
	createTorrent(t, src, tf)
	sd := seed(t, tf, filepath.Dir(src), 1<<20)

	h := newHarness(t, nil, filepath.Join(dir, "torrents"), filepath.Join(dir, "got"))
	info := h.m.Create(engine.AddOptions{URL: tf, Torrent: &engine.TorrentJob{}, Start: true})
	h.wait(info.ID, sd, 30*time.Second, "1 MiB", func(i engine.Info) bool { return i.Done >= 1<<20 })
	h.m.Pause(info.ID)
	stopped := time.Now().Add(10 * time.Second)
	for {
		h.s.mu.Lock()
		running := h.s.c != nil
		h.s.mu.Unlock()
		if !running {
			break
		}
		if time.Now().After(stopped) {
			t.Fatal("odm-libtorrent still runs with nothing to do")
		}
		time.Sleep(50 * time.Millisecond)
	}
	h.m.Resume(info.ID)
	h.wait(info.ID, sd, 60*time.Second, "the download", complete)
	sameFile(t, filepath.Join(h.dir, "big.bin"), want)
}

func TestSessionSeedsUntilTheRatio(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "seed", "data.bin")
	makeFile(t, src, 1<<20)
	tf := filepath.Join(dir, "data.torrent")
	createTorrent(t, src, tf)
	sd := seed(t, tf, filepath.Dir(src), 0)

	h := newHarness(t, nil, filepath.Join(dir, "torrents"), filepath.Join(dir, "got"))
	st := h.m.Settings()
	st.Torrent.Seed = engine.SeedTarget // 100 %, or 30 minutes
	h.m.UpdateSettings(st)
	info := h.m.Create(engine.AddOptions{URL: tf, Torrent: &engine.TorrentJob{}, Start: true})
	got := h.wait(info.ID, sd, 60*time.Second, "the download", complete)
	if !got.Seeding || h.s.in(info.ID) == nil {
		t.Fatalf("not seeding: %+v", got)
	}
	det, live := h.m.TorrentDetail(context.Background(), info.ID)
	if !live || det.Count != 32 || len(det.Pieces) != 4 || det.Pieces[0] != 0xff || len(det.FileDone) != 1 || det.FileDone[0] != 1<<20 {
		t.Fatalf("detail while seeding: %v %+v", live, det)
	}
	// The first seeder goes: a second peer can only get the file from ODM.
	sd.c.Close(10 * time.Second)
	b := leech(t, tf, filepath.Join(dir, "b"))
	for i := 0; ; i++ {
		info, _ := h.m.Get(info.ID)
		if !info.Seeding {
			if info.Uploaded < 1<<20 {
				t.Fatalf("stopped seeding at %d bytes uploaded", info.Uploaded)
			}
			break
		}
		if i%20 == 0 {
			_ = b.Call(context.Background(), "connect", map[string]any{"key": "peer", "peer": "127.0.0.1:" + strconv.Itoa(h.s.Port())}, nil)
		}
		if i > 1200 {
			t.Fatalf("still seeding after a peer took the whole file: %+v", info)
		}
		time.Sleep(50 * time.Millisecond)
	}
	if h.s.in(info.ID) != nil {
		t.Fatal("the torrent is still in the session after seeding")
	}
}
