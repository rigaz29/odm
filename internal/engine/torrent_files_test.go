package engine

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// albumMeta is a folder torrent of three files and a pad.
func albumMeta() TorrentMeta {
	return TorrentMeta{Name: "Album", InfoHash: "ab", Files: []TorrentFile{
		{Path: "Album/01.flac", Size: 300},
		{Path: ".pad/1", Size: 10, Pad: true},
		{Path: "Album/02.flac", Size: 200},
		{Path: "Album/cover.jpg", Size: 50},
	}}
}

func TestTorrentFilesChangeWhileItRuns(t *testing.T) {
	f := newFakeTorrents(t, albumMeta())
	m := torrentManager(t, Settings{Torrent: TorrentSettings{Seed: SeedNever}})
	info := m.Create(AddOptions{URL: dotTorrent(t), Torrent: &TorrentJob{}, Start: true})
	if a := f.waitAdd(t); !reflect.DeepEqual(a.Priorities, []int{4, 0, 4, 4}) {
		t.Fatalf("priorities at the add: %v", a.Priorities)
	}
	d := m.get(info.ID)
	waitStatus(t, d, StatusDownloading)

	if err := m.SetTorrentFiles(info.ID, map[int]int{2: 0, 3: PriorityHigh, 0: PriorityLow}); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	prios, names := f.prios, f.setFiles
	f.mu.Unlock()
	if len(prios) != 1 || !reflect.DeepEqual(prios[0], []int{1, 0, 0, 7}) || names[0] != nil {
		t.Fatalf("told the backend %v, names %v", prios, names)
	}
	tj, _ := m.Torrent(info.ID)
	if !tj.Files[2].Skip || tj.Files[3].Priority != PriorityHigh || tj.Files[0].Prio() != PriorityLow {
		t.Fatalf("files %+v", tj.Files)
	}
	// Back to normal is stored as nothing.
	if err := m.SetTorrentFiles(info.ID, map[int]int{0: PriorityNormal}); err != nil {
		t.Fatal(err)
	}
	if tj, _ := m.Torrent(info.ID); tj.Files[0].Priority != 0 || tj.Files[0].Prio() != PriorityNormal {
		t.Fatalf("file 0: %+v", tj.Files[0])
	}

	for _, c := range []struct {
		prio map[int]int
		want error
	}{
		{map[int]int{0: 0, 3: 0}, errNoFileWanted},
		{map[int]int{1: 4}, errNoSuchFile}, // the pad
		{map[int]int{9: 4}, errNoSuchFile},
		{map[int]int{0: 3}, errBadPriority},
	} {
		if err := m.SetTorrentFiles(info.ID, c.prio); !errors.Is(err, c.want) {
			t.Errorf("%v: %v, want %v", c.prio, err, c.want)
		}
	}
	if tj, _ := m.Torrent(info.ID); tj.Files[0].Skip || tj.Files[3].Skip {
		t.Fatal("a refused change was kept")
	}
}

func TestTorrentFilesChangeWhilePaused(t *testing.T) {
	f := newFakeTorrents(t, albumMeta())
	m := torrentManager(t, Settings{})
	info := m.Create(AddOptions{URL: dotTorrent(t), Torrent: &TorrentJob{}, Start: true})
	f.waitAdd(t)
	d := m.get(info.ID)
	waitStatus(t, d, StatusDownloading)
	m.Pause(info.ID)
	waitStatus(t, d, StatusPaused)

	if err := m.SetTorrentFiles(info.ID, map[int]int{0: 0}); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	told := len(f.prios)
	f.mu.Unlock()
	if told != 0 {
		t.Fatal("a paused torrent was told at once")
	}
	if i := d.Info(); i.Total != 250 {
		t.Fatalf("total %d, want the 250 bytes still wanted", i.Total)
	}
	// The next run takes them.
	m.Resume(info.ID)
	if a := f.waitAdd(t); !reflect.DeepEqual(a.Priorities, []int{0, 0, 4, 4}) {
		t.Fatalf("priorities at the resume: %v", a.Priorities)
	}
}

func TestTorrentFilesOfAMagnetBeforeItsMetadata(t *testing.T) {
	newFakeTorrents(t, albumMeta())
	m := torrentManager(t, Settings{})
	info := m.Create(AddOptions{URL: "magnet:?xt=urn:btih:ab", Start: false})
	if err := m.SetTorrentFiles(info.ID, map[int]int{0: 0}); !errors.Is(err, errFilesUnknown) {
		t.Fatalf("%v, want %v", err, errFilesUnknown)
	}
}

func TestTorrentFilesOfACompleteTorrentStay(t *testing.T) {
	f := newFakeTorrents(t, albumMeta())
	m := torrentManager(t, Settings{Torrent: TorrentSettings{Seed: SeedNever}})
	info := m.Create(AddOptions{URL: dotTorrent(t), Torrent: &TorrentJob{}, Start: true})
	f.waitAdd(t)
	d := m.get(info.ID)
	waitStatus(t, d, StatusDownloading)
	// Left out after a part of it came: its file stays in the folder.
	if err := m.SetTorrentFiles(info.ID, map[int]int{3: 0}); err != nil {
		t.Fatal(err)
	}
	f.send(t, info.ID, TorrentEvent{Kind: TorrentEventFinished})
	waitStatus(t, d, StatusComplete)
	if err := m.SetTorrentFiles(info.ID, map[int]int{3: 4}); !errors.Is(err, errTorrentDone) {
		t.Fatalf("%v, want %v", err, errTorrentDone)
	}

	// Deleting it with its files takes that file too, and the folder.
	folder := filepath.Join(m.Settings().Dir, d.Info().FileName)
	if _, err := os.Stat(filepath.Join(folder, "cover.jpg")); err != nil {
		t.Fatal(err)
	}
	m.RemoveWithFile(info.ID, true)
	if _, err := os.Stat(folder); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the folder is left: %v", err)
	}
}
