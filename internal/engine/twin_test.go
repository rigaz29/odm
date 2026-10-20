package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const twinHash = "322cba6e0dacc7a6c25d2d1f0e773fa2ba18017b"

// The same torrent is found by its info hash, from another address too,
// and is added again only once the first is complete and done seeding.
func TestFindTorrentByInfoHash(t *testing.T) {
	newFakeTorrents(t, TorrentMeta{Name: "Sample", InfoHash: twinHash})
	m := torrentManager(t, Settings{})
	first := m.Create(AddOptions{URL: "magnet:?xt=urn:btih:" + twinHash + "&dn=Sample"})

	// The .torrent of that torrent, and its magnet link in base32.
	data, err := os.ReadFile("testdata/hybrid.torrent")
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "Sample.torrent")
	os.WriteFile(file, data, 0o644)
	for _, o := range []AddOptions{
		{URL: file, Torrent: &TorrentJob{}},
		{URL: "https://example.com/s.torrent", Torrent: &TorrentJob{}, TorrentFile: file},
		{URL: "magnet:?xt=urn:btih:GIWLU3QNVTD2NQS5FUPQ45Z7UK5BQAL3"},
	} {
		got, free, ok := m.FindTorrent(o)
		if !ok || free || got.ID != first.ID {
			t.Fatalf("%s: %+v free %v ok %v", o.URL, got.ID, free, ok)
		}
	}
	if _, _, ok := m.FindTorrent(AddOptions{URL: "https://example.com/s.torrent", Torrent: &TorrentJob{}}); ok {
		t.Fatal("found by a .torrent not read yet")
	}
	if _, _, ok := m.FindTorrent(AddOptions{URL: "magnet:?xt=urn:btih:" + strings.Repeat("0", 40)}); ok {
		t.Fatal("found another torrent")
	}

	// Automatic adds keep the one that may run, whatever the policy.
	st := m.Settings()
	st.DuplicateAuto = DuplicateAgain
	m.UpdateSettings(st)
	if got, added := m.AddAuto(AddOptions{URL: file, Torrent: &TorrentJob{}}); added || got.ID != first.ID {
		t.Fatalf("added a twin: %v %s", added, got.ID)
	}

	// Complete and no longer seeding: out of the session, so it may be
	// downloaded again, which the policy decides.
	d := m.get(first.ID)
	d.mu.Lock()
	d.Status = StatusComplete
	d.mu.Unlock()
	if _, free, ok := m.FindTorrent(AddOptions{URL: file, Torrent: &TorrentJob{}}); !ok || !free {
		t.Fatalf("complete: free %v ok %v", free, ok)
	}
	if _, added := m.AddAuto(AddOptions{URL: file, Torrent: &TorrentJob{}}); !added {
		t.Fatal("the again policy did not add it again")
	}
	st.DuplicateAuto = DuplicateSkip
	m.UpdateSettings(st)
	if _, added := m.AddAuto(AddOptions{URL: "magnet:?xt=urn:btih:" + twinHash}); added {
		t.Fatal("the skip policy added it again")
	}
}

// A twin whose hash was not known when it was added (a .torrent at an
// address) does not start while odm-libtorrent holds the first, and does
// once the first is gone.
func TestTorrentTwinWaitsForTheFirst(t *testing.T) {
	f := newFakeTorrents(t, TorrentMeta{Name: "Sample", InfoHash: twinHash, Files: []TorrentFile{{Path: "Sample", Size: 10}}})
	m := torrentManager(t, Settings{})
	first := m.Create(AddOptions{URL: dotTorrent(t), Torrent: &TorrentJob{}, Start: true})
	f.waitAdd(t)
	waitStatus(t, m.get(first.ID), StatusDownloading)

	// The address is not fetched: the twin's .torrent was read already.
	second := m.Create(AddOptions{URL: "https://example.com/Sample.torrent", Torrent: &TorrentJob{}, TorrentFile: dotTorrent(t), Start: true})
	d := m.get(second.ID)
	waitStatus(t, d, StatusError)
	if e := d.Info().Error; !strings.Contains(e, "already in the list") || !strings.Contains(e, "Sample") {
		t.Fatalf("error %q", e)
	}
	// Paused, the first stays in odm-libtorrent.
	m.Pause(first.ID)
	waitStatus(t, m.get(first.ID), StatusPaused)
	m.Resume(second.ID)
	waitStatus(t, d, StatusError)

	m.RemoveWithFile(first.ID, false)
	m.Resume(second.ID)
	if a := f.waitAdd(t); a.ID != second.ID {
		t.Fatalf("added %s", a.ID)
	}
}
