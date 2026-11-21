package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// An imported download is fetched anew. Its progress counted bytes in an
// .odmpart that is not on this computer (or no longer is), and resuming
// it "finished" at once as a file of zeros.
func TestImportFetchesAnew(t *testing.T) {
	for _, keepFile := range []bool{false, true} {
		name := "another computer"
		if keepFile {
			name = "the same computer after Clear completed"
		}
		t.Run(name, func(t *testing.T) {
			data := randomData(256 << 10)
			srv := httpServer(t, func(w http.ResponseWriter, r *http.Request) { serveRange(w, r, data) })
			dir := t.TempDir()
			m := NewManager(Settings{Dir: dir}, "")
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			m.Begin(ctx)

			info := m.Create(AddOptions{URL: srv.URL + "/report.pdf", Start: true})
			d := m.get(info.ID)
			waitFor(t, "the first download", func() bool { return d.status() == StatusComplete })
			list, err := m.Export()
			if err != nil {
				t.Fatal(err)
			}
			if keepFile {
				m.ClearCompleted() // the entry goes, the file stays
			} else {
				m.RemoveWithFile(info.ID, true) // no file, as on another computer
			}
			if n, err := m.Import(list); err != nil || n != 1 {
				t.Fatalf("import: %d %v", n, err)
			}
			m.ResumeAll()
			d = m.get(info.ID)
			waitFor(t, "the imported download", func() bool {
				s := d.status()
				return s == StatusComplete || s == StatusError
			})
			got := d.Info()
			content, err := os.ReadFile(got.FilePath)
			if err != nil {
				t.Fatal(err)
			}
			if got.Status != StatusComplete || !bytes.Equal(content, data) {
				t.Fatalf("status %s: %d bytes that are not the file", got.Status, len(content))
			}
		})
	}
}

// A list is a file anyone can write: an imported download's ID, name and
// folders become paths on this computer only when they are safe ones.
func TestImportDoesNotTrustNamesOrFolders(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(Settings{Dir: dir}, "")
	startup := filepath.Join(t.TempDir(), "Startup")
	list, _ := json.Marshal([]State{{
		ID: `..\..\settings`, URL: "http://example.com/tool.exe", FileName: `..\..\run.exe`,
		Dir: startup, BaseDir: startup, Status: StatusComplete,
		Total: 100, RangeOK: true, Chunks: []*Chunk{{ID: 0, Start: 0, End: 99, Done: 100}},
	}})
	if n, err := m.Import(list); err != nil || n != 1 {
		t.Fatalf("import: %d %v", n, err)
	}
	infos := m.List()
	if len(infos) != 1 {
		t.Fatalf("%d downloads", len(infos))
	}
	d := m.get(infos[0].ID)
	st := d.State()
	switch {
	case !validID(st.ID):
		t.Errorf("ID %q", st.ID)
	case strings.ContainsAny(st.FileName, `\/:`) || st.FileName == "" || st.FileName == "..":
		t.Errorf("file name %q", st.FileName)
	case st.Dir != dir || (st.BaseDir != "" && st.BaseDir != dir):
		t.Errorf("folders %q %q, want the download folder %q", st.Dir, st.BaseDir, dir)
	case st.Status != StatusPaused || len(st.Chunks) != 0 || st.Total != 0:
		t.Errorf("progress kept: status %s, %d chunks, total %d", st.Status, len(st.Chunks), st.Total)
	}

	// A folder inside the download folder is kept.
	inside := filepath.Join(dir, "Mine")
	list, _ = json.Marshal([]State{{ID: "abc123", URL: "http://example.com/b.bin", FileName: "b.bin", Dir: inside, BaseDir: inside}})
	if _, err := m.Import(list); err != nil {
		t.Fatal(err)
	}
	if st := m.get("abc123").State(); st.Dir != inside || st.BaseDir != inside {
		t.Errorf("folders %q %q, want %q kept", st.Dir, st.BaseDir, inside)
	}
}

// An imported torrent whose files are already known places them by their
// paths in the torrent, made safe again, not by the names the list gives.
func TestImportedTorrentNamesAreMadeSafe(t *testing.T) {
	f := newFakeTorrents(t, albumMeta())
	m := torrentManager(t, Settings{Torrent: TorrentSettings{Seed: SeedNever}})
	dir := m.Settings().Dir
	st := []State{{ID: "imp1", URL: "magnet:?xt=urn:btih:ab", Dir: dir, FileName: "Album", Status: StatusPaused,
		Torrent: &TorrentJob{Magnet: "magnet:?xt=urn:btih:ab", InfoHash: "ab", Folder: true, Files: []TorrentFile{
			{Path: "Album/01.flac", Name: "../../escaped.flac", Size: 300}, {Path: ".pad/1", Size: 10, Pad: true},
			{Path: "Album/02.flac", Name: "02.flac", Size: 200}, {Path: "Album/cover.jpg", Name: "cover.jpg", Size: 50}}}}}
	data, _ := json.Marshal(st)
	if _, err := m.Import(data); err != nil {
		t.Fatal(err)
	}
	m.Resume("imp1")
	a := f.waitAdd(t)
	for _, n := range a.Names {
		if strings.Contains(n, "..") {
			t.Errorf("odm-libtorrent is told to write %s", filepath.Join(a.SavePath, n))
		}
	}
}
