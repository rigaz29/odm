package gui

import (
	"testing"

	"odm/internal/engine"
)

// The New Download window's choices reach the download, and the progress
// window lists the files by index with their priorities and can change
// them.
func TestTorrentFilesFromTheWindows(t *testing.T) {
	svc, mgr := newTestService(t)
	addr := "https://example.com/Season.torrent"
	svc.torrents = map[string]*torrentProbe{addr: {state: TorrentProbe{Name: "Season", Files: []TorrentFileRow{
		{Path: "Season/01.mkv", Size: 100},
		{Path: ".pad/1", Size: 4, Pad: true},
		{Path: "Season/02.mkv", Size: 200},
		{Path: "Season/subs.srt", Size: 1},
	}}}}
	res, err := svc.createTorrent(CreateRequest{URL: addr, Skip: []int{2},
		Priorities: map[int]int{0: engine.PriorityHigh, 3: 5}}, addr)
	if err != nil {
		t.Fatal(err)
	}
	tj, _ := mgr.Torrent(res.ID)
	if tj.Files[0].Prio() != engine.PriorityHigh || !tj.Files[2].Skip || tj.Files[3].Prio() != engine.PriorityNormal {
		t.Fatalf("files %+v", tj.Files)
	}

	if err := svc.SetTorrentFiles(res.ID, map[int]int{2: engine.PriorityLow, 0: 0}); err != nil {
		t.Fatal(err)
	}
	d, err := svc.Detail(res.ID)
	if err != nil || d.Torrent == nil {
		t.Fatalf("detail %+v, %v", d, err)
	}
	type row struct{ index, prio int }
	var got []row
	for _, f := range d.Torrent.Files {
		got = append(got, row{f.Index, f.Priority})
	}
	want := []row{{0, 0}, {2, engine.PriorityLow}, {3, engine.PriorityNormal}}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("listed %v, want %v", got, want)
	}
	if err := svc.SetTorrentFiles(res.ID, map[int]int{2: 0, 3: 0}); err == nil {
		t.Fatal("every file left out")
	}
}

// The New Download window finds the same torrent by its info hash, from
// another form of its magnet link, and does not offer to download it again
// while it may run.
func TestTorrentTwinInTheWindow(t *testing.T) {
	svc, mgr := newTestService(t)
	const hash = "322cba6e0dacc7a6c25d2d1f0e773fa2ba18017b"
	first := mgr.Create(engine.AddOptions{URL: "magnet:?xt=urn:btih:" + hash + "&dn=Sample"})
	addr := "magnet:?xt=urn:btih:GIWLU3QNVTD2NQS5FUPQ45Z7UK5BQAL3"
	if card, ok := svc.twinOf(engine.AddOptions{URL: addr}); !ok || card.State != "twin" || card.Name != "Sample" {
		t.Fatalf("card %+v %v", card, ok)
	}
	for _, again := range []bool{false, true} {
		res, err := svc.createTorrent(CreateRequest{URL: addr, Again: again}, addr)
		if err != nil || res.Duplicate == nil || res.Duplicate.ID != first.ID || !res.SameTorrent || !res.NoAgain || res.ID != "" {
			t.Fatalf("again %v: %+v %v", again, res, err)
		}
	}
	if n := len(mgr.List()); n != 1 {
		t.Fatalf("%d downloads", n)
	}
}
