package engine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeTorrents stands in for odm-libtorrent: it records what the engine
// asks, writes the files where it is told, renames and moves them, and
// sends the events a test gives it.
type fakeTorrents struct {
	mu       sync.Mutex
	meta     TorrentMeta
	adds     []TorrentAdd
	pauses   []string
	removes  map[string]bool // id -> deleteFiles
	setFiles []map[int]string
	limits   [][2]int64
	watchers map[string]chan TorrentEvent
	save     map[string]string         // id -> save path
	names    map[string]map[int]string // id -> current names
	added    chan TorrentAdd
}

func newFakeTorrents(t *testing.T, meta TorrentMeta) *fakeTorrents {
	f := &fakeTorrents{meta: meta, removes: map[string]bool{}, watchers: map[string]chan TorrentEvent{},
		save: map[string]string{}, names: map[string]map[int]string{}, added: make(chan TorrentAdd, 16)}
	prev, prevDir := Torrents, TorrentDir
	Torrents, TorrentDir = f, t.TempDir()
	t.Cleanup(func() { Torrents, TorrentDir = prev, prevDir })
	return f
}

func (f *fakeTorrents) Inspect(ctx context.Context, file, magnet string) (TorrentMeta, error) {
	if _, err := os.Stat(file); err != nil {
		return TorrentMeta{}, err
	}
	return f.meta, nil
}

func (f *fakeTorrents) Add(ctx context.Context, a TorrentAdd) error {
	f.mu.Lock()
	f.adds = append(f.adds, a)
	f.save[a.ID] = a.SavePath
	if a.Names != nil {
		f.names[a.ID] = a.Names
		f.writeLocked(a.ID)
	}
	f.mu.Unlock()
	f.added <- a
	return nil
}

// writeLocked creates each named file, as libtorrent would while it
// downloads.
func (f *fakeTorrents) writeLocked(id string) {
	for i, name := range f.names[id] {
		p := filepath.Join(f.save[id], filepath.FromSlash(name))
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(strings.Repeat("x", int(f.meta.Files[i].Size))), 0o644)
	}
}

func (f *fakeTorrents) Watch(id string) (<-chan TorrentEvent, func()) {
	ch := make(chan TorrentEvent, 64)
	f.mu.Lock()
	f.watchers[id] = ch
	f.mu.Unlock()
	return ch, func() {
		f.mu.Lock()
		if f.watchers[id] == ch {
			delete(f.watchers, id)
		}
		f.mu.Unlock()
	}
}

func (f *fakeTorrents) send(t *testing.T, id string, ev TorrentEvent) {
	t.Helper()
	for i := 0; i < 200; i++ {
		f.mu.Lock()
		ch := f.watchers[id]
		f.mu.Unlock()
		if ch != nil {
			ch <- ev
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("nobody watches %s", id)
}

func (f *fakeTorrents) Pause(ctx context.Context, id string) error {
	f.mu.Lock()
	f.pauses = append(f.pauses, id)
	f.mu.Unlock()
	return nil
}

func (f *fakeTorrents) SetFiles(ctx context.Context, id string, names map[int]string, prio []int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.setFiles = append(f.setFiles, names)
	f.names[id] = names
	f.writeLocked(id)
	return nil
}

func (f *fakeTorrents) Rename(ctx context.Context, id string, names map[int]string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, name := range names {
		old := filepath.Join(f.save[id], filepath.FromSlash(f.names[id][i]))
		if err := os.Rename(old, filepath.Join(f.save[id], filepath.FromSlash(name))); err != nil {
			return err
		}
		f.names[id][i] = name
	}
	return nil
}

func (f *fakeTorrents) Move(ctx context.Context, id, savePath string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	// The whole folder moves, part file included; a torrent with no files
	// named yet (a held magnet link) has nothing to move.
	if old := f.save[id]; old != "" && len(f.names[id]) > 0 {
		if err := os.Rename(old, savePath); err != nil {
			return err
		}
	}
	f.save[id] = savePath
	return nil
}

func (f *fakeTorrents) SetLimits(ctx context.Context, id string, down, up int64) error {
	f.mu.Lock()
	f.limits = append(f.limits, [2]int64{down, up})
	f.mu.Unlock()
	return nil
}

func (f *fakeTorrents) SetGlobalLimits(down, up int64) {}

func (f *fakeTorrents) Remove(ctx context.Context, id string, deleteFiles bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.removes[id] = deleteFiles
	if deleteFiles {
		for _, name := range f.names[id] {
			os.Remove(filepath.Join(f.save[id], filepath.FromSlash(name)))
		}
	}
	return nil
}

func (f *fakeTorrents) waitAdd(t *testing.T) TorrentAdd {
	t.Helper()
	select {
	case a := <-f.added:
		return a
	case <-time.After(5 * time.Second):
		t.Fatal("no torrent added")
	}
	return TorrentAdd{}
}

func torrentManager(t *testing.T, st Settings) *Manager {
	t.Helper()
	if st.Dir == "" {
		st.Dir = t.TempDir()
	}
	if st.Concurrent == 0 {
		st.Concurrent = 3
	}
	m := NewManager(st, "")
	ctx, cancel := context.WithCancel(context.Background())
	m.Begin(ctx)
	t.Cleanup(cancel)
	return m
}

func dotTorrent(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "x.torrent")
	if err := os.WriteFile(p, []byte("d4:infod4:name1:xee"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func waitUntil(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("no %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestTorrentOfOneFile(t *testing.T) {
	f := newFakeTorrents(t, TorrentMeta{Name: "ubuntu.iso", Size: 1000, InfoHash: "aa",
		Files: []TorrentFile{{Path: "ubuntu.iso", Size: 1000}}})
	var marked []string
	prevMark := MarkFile
	MarkFile = func(path, source, referrer string) error { marked = append(marked, path); return nil }
	t.Cleanup(func() { MarkFile = prevMark })
	m := torrentManager(t, Settings{})
	src := dotTorrent(t)
	info := m.Create(AddOptions{URL: src, Torrent: &TorrentJob{}, Start: true, SpeedLimit: 5000})
	a := f.waitAdd(t)
	dir := m.Settings().Dir
	if a.ID != info.ID || a.SavePath != dir || !reflect.DeepEqual(a.Names, map[int]string{0: "ubuntu.iso.odmpart"}) ||
		!reflect.DeepEqual(a.Priorities, []int{4}) || a.Hold || a.DownloadLimit != 5000 ||
		a.File != filepath.Join(TorrentDir, info.ID+".torrent") || a.Resume != filepath.Join(TorrentDir, info.ID+".resume") {
		t.Fatalf("add: %+v", a)
	}
	f.send(t, info.ID, TorrentEvent{Kind: TorrentEventStatus, Status: TorrentStatus{State: "downloading", Size: 1000, Done: 400, Peers: 3, Seeds: 2}})
	d := m.get(info.ID)
	waitUntil(t, "progress", func() bool { i := d.Info(); return i.Done == 400 && i.Total == 1000 && i.Peers == 3 })
	if i := d.Info(); !i.Torrent || i.FileName != "ubuntu.iso" || i.FilePath != filepath.Join(dir, "ubuntu.iso.odmpart") {
		t.Fatalf("info: %+v", i)
	}

	f.send(t, info.ID, TorrentEvent{Kind: TorrentEventFinished})
	waitStatus(t, d, StatusComplete)
	if _, err := os.Stat(filepath.Join(dir, "ubuntu.iso")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "ubuntu.iso.odmpart")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("the .odmpart is still there")
	}
	f.mu.Lock()
	del, removed := f.removes[info.ID]
	f.mu.Unlock()
	if !removed || del {
		t.Fatalf("finished torrent not taken out of the session (removed %v, delete %v)", removed, del)
	}
	if len(marked) != 1 || marked[0] != filepath.Join(dir, "ubuntu.iso") {
		t.Fatalf("marked %v", marked)
	}
	if i := d.Info(); i.Done != 1000 || i.Progress != 100 {
		t.Fatalf("done: %+v", i)
	}
}

func TestTorrentFolderIsSanitizedAndMoved(t *testing.T) {
	f := newFakeTorrents(t, TorrentMeta{Name: "Album: Live?", InfoHash: "bb", InfoHashV2: "cc", Files: []TorrentFile{
		{Path: "Album: Live?/01 Song.mp3", Size: 900},
		{Path: ".pad/100", Size: 100, Pad: true},
		{Path: "Album: Live?/../../evil.exe", Size: 10},
		{Path: "Album: Live?/sub/CON.txt", Size: 10},
		{Path: "Album: Live?/01 song.MP3", Size: 20},
		{Path: "Album: Live?/photo\u202egpj.exe", Size: 10},
	}})
	m := torrentManager(t, Settings{Categories: true})
	info := m.Create(AddOptions{URL: dotTorrent(t), Torrent: &TorrentJob{}, Start: true})
	a := f.waitAdd(t)
	base := m.Settings().Dir
	dir := filepath.Join(base, Category("x.mp3"))
	if a.SavePath != filepath.Join(dir, "Album_ Live_.odmpart") {
		t.Fatalf("save path %s", a.SavePath)
	}
	want := map[int]string{0: "01 Song.mp3", 2: "evil.exe", 3: "sub/_CON.txt", 4: "01 song (2).MP3", 5: "photogpj.exe"}
	if !reflect.DeepEqual(a.Names, want) || !reflect.DeepEqual(a.Priorities, []int{4, 0, 4, 4, 4, 4}) {
		t.Fatalf("names %v, priorities %v", a.Names, a.Priorities)
	}
	// Something took the name while it downloaded.
	if err := os.MkdirAll(filepath.Join(dir, "Album_ Live_"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.SavePath, ".bb.parts"), []byte("p"), 0o644); err != nil {
		t.Fatal(err)
	}
	part := filepath.Join(dir, "Album_ Live_ (2)", ".bb.parts")
	f.send(t, info.ID, TorrentEvent{Kind: TorrentEventFinished})
	d := m.get(info.ID)
	waitStatus(t, d, StatusComplete)
	if i := d.Info(); i.FileName != "Album_ Live_ (2)" || i.Total != 950 {
		t.Fatalf("info: %+v", i)
	}
	for _, name := range want {
		if _, err := os.Stat(filepath.Join(dir, "Album_ Live_ (2)", filepath.FromSlash(name))); err != nil {
			t.Error(err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "Album_ Live_.odmpart")); !errors.Is(err, os.ErrNotExist) {
		t.Error("the .odmpart folder is still there")
	}
	if _, err := os.Stat(part); !errors.Is(err, os.ErrNotExist) {
		t.Error("part file left")
	}
}

func TestTorrentMagnetIsHeldUntilItsFilesAreNamed(t *testing.T) {
	f := newFakeTorrents(t, TorrentMeta{Name: "Show", InfoHash: "dd", Files: []TorrentFile{
		{Path: "Show/e1.mkv", Size: 500}, {Path: "Show/e2.mkv", Size: 600},
	}})
	m := torrentManager(t, Settings{})
	magnet := "magnet:?xt=urn:btih:dd&dn=My%20Show"
	info := m.Create(AddOptions{URL: magnet, Start: true})
	if info.FileName != "My Show" || !info.Torrent {
		t.Fatalf("before the metadata: %+v", info)
	}
	a := f.waitAdd(t)
	dir := m.Settings().Dir
	if !a.Hold || a.Magnet != magnet || a.Names != nil || a.File != "" || a.SavePath != dir {
		t.Fatalf("add: %+v", a)
	}
	meta := f.meta
	f.send(t, info.ID, TorrentEvent{Kind: TorrentEventMetadata, Meta: &meta})
	waitUntil(t, "files named", func() bool { f.mu.Lock(); defer f.mu.Unlock(); return len(f.setFiles) == 1 })
	f.mu.Lock()
	got, save := f.setFiles[0], f.save[info.ID]
	f.mu.Unlock()
	if !reflect.DeepEqual(got, map[int]string{0: "e1.mkv", 1: "e2.mkv"}) || save != filepath.Join(dir, "Show.odmpart") {
		t.Fatalf("named %v in %s", got, save)
	}
	d := m.get(info.ID)
	if i := d.Info(); i.FileName != "Show" || i.Total != 1100 {
		t.Fatalf("after the metadata: %+v", i)
	}
	f.send(t, info.ID, TorrentEvent{Kind: TorrentEventFinished})
	waitStatus(t, d, StatusComplete)
	if _, err := os.Stat(filepath.Join(dir, "Show", "e2.mkv")); err != nil {
		t.Fatal(err)
	}
}

func TestTorrentPauseResumeAndRemove(t *testing.T) {
	f := newFakeTorrents(t, TorrentMeta{Name: "big.bin", InfoHash: "ee", Files: []TorrentFile{{Path: "big.bin", Size: 100}}})
	m := torrentManager(t, Settings{})
	info := m.Create(AddOptions{URL: dotTorrent(t), Torrent: &TorrentJob{}, Start: true})
	f.waitAdd(t)
	d := m.get(info.ID)
	waitStatus(t, d, StatusDownloading)
	m.Pause(info.ID)
	waitStatus(t, d, StatusPaused)
	waitUntil(t, "pause", func() bool { f.mu.Lock(); defer f.mu.Unlock(); return len(f.pauses) == 1 })

	m.Resume(info.ID)
	if a := f.waitAdd(t); a.ID != info.ID || a.Names == nil {
		t.Fatalf("resumed: %+v", a)
	}
	m.SetSpeedLimit(info.ID, 7000)
	waitUntil(t, "limit", func() bool { f.mu.Lock(); defer f.mu.Unlock(); return len(f.limits) == 1 && f.limits[0][0] == 7000 })

	dir := m.Settings().Dir
	m.RemoveWithFile(info.ID, false)
	f.mu.Lock()
	del := f.removes[info.ID]
	f.mu.Unlock()
	if !del {
		t.Fatal("an unfinished torrent's files were kept")
	}
	for _, p := range []string{filepath.Join(dir, "big.bin.odmpart"), filepath.Join(TorrentDir, info.ID+".torrent")} {
		if _, err := os.Stat(p); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s left", p)
		}
	}
}

func TestTorrentErrorFailsTheDownload(t *testing.T) {
	f := newFakeTorrents(t, TorrentMeta{Name: "a.bin", InfoHash: "ff", Files: []TorrentFile{{Path: "a.bin", Size: 10}}})
	m := torrentManager(t, Settings{})
	info := m.Create(AddOptions{URL: dotTorrent(t), Torrent: &TorrentJob{}, Start: true})
	f.waitAdd(t)
	f.send(t, info.ID, TorrentEvent{Kind: TorrentEventError, Err: "disk full"})
	d := m.get(info.ID)
	waitStatus(t, d, StatusError)
	if i := d.Info(); i.Error != "disk full" {
		t.Fatalf("error: %q", i.Error)
	}
}

func TestTorrentCompleteWhenAddedFinishes(t *testing.T) {
	f := newFakeTorrents(t, TorrentMeta{Name: "a.bin", InfoHash: "ab", Files: []TorrentFile{{Path: "a.bin", Size: 10}}})
	m := torrentManager(t, Settings{})
	info := m.Create(AddOptions{URL: dotTorrent(t), Torrent: &TorrentJob{}, Start: true})
	f.waitAdd(t)
	// ODM stopped while it finished: no finished event comes again.
	f.send(t, info.ID, TorrentEvent{Kind: TorrentEventStatus, Status: TorrentStatus{State: "seeding", Size: 10, Done: 10}})
	waitStatus(t, m.get(info.ID), StatusComplete)
}

func TestTorrentWithoutBackendFails(t *testing.T) {
	prev := Torrents
	Torrents = nil
	t.Cleanup(func() { Torrents = prev })
	m := torrentManager(t, Settings{})
	info := m.Create(AddOptions{URL: "magnet:?xt=urn:btih:00", Start: true})
	waitStatus(t, m.get(info.ID), StatusError)
}

func TestTorrentStateRoundTrip(t *testing.T) {
	d := New("id1", "magnet:?xt=urn:btih:12", t.TempDir(), 0, nil, 0, nil)
	d.Torrent = &TorrentJob{Magnet: "magnet:?xt=urn:btih:12", InfoHash: "12", Folder: true,
		Files: []TorrentFile{{Path: "F/a", Name: "a", Size: 5, Skip: true}, {Path: "F/b", Name: "b", Size: 7}}}
	d.streamDone, d.Total = 3, 7
	st := d.State()
	r := Restore(st, nil)
	if !reflect.DeepEqual(r.Torrent.Files, d.Torrent.Files) || r.Torrent.Magnet != d.Torrent.Magnet ||
		!r.Torrent.Folder || r.doneBytes() != 3 {
		t.Fatalf("restored %+v", r.Torrent)
	}
}

func TestTorrentAddresses(t *testing.T) {
	for s, want := range map[string]bool{"magnet:?xt=urn:btih:ab": true, " MAGNET:?xt=1": true, "http://h/a.torrent": false, "magnet:": false} {
		if IsMagnet(s) != want {
			t.Errorf("IsMagnet(%q) = %v", s, !want)
		}
	}
	for s, want := range map[string]bool{"http://h/a.torrent?x=1": true, `C:\t\Ubuntu.TORRENT`: true, "http://h/torrent": false} {
		if IsTorrentFile(s) != want {
			t.Errorf("IsTorrentFile(%q) = %v", s, !want)
		}
	}
	for link, want := range map[string]string{
		"magnet:?xt=urn:btih:abcd&dn=Big%2FFile.iso": "Big_File.iso",
		"magnet:?xt=urn:btih:abcd":                   "abcd",
		"magnet:?xt=urn:btmh:1220ef":                 "1220ef",
	} {
		if got := magnetName(link); got != want {
			t.Errorf("magnetName(%q) = %q, want %q", link, got, want)
		}
	}
}
