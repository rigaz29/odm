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
	"os"
	"os/exec"
	"path/filepath"
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
	m := engine.NewManager(engine.Settings{Dir: dir, Concurrent: 3}, "")
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
