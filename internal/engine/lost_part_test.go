package engine

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

// A finished file that the antivirus scan removed (finishing then failed)
// is downloaded again on Resume. Its .odmpart went with it; opened anew it
// was all zeros, every chunk counted as done, and the zeros were renamed
// into place as a complete download.
func TestResumeAfterTheFileWasRemovedFetchesAgain(t *testing.T) {
	data := randomData(256 << 10)
	srv := httpServer(t, func(w http.ResponseWriter, r *http.Request) { serveRange(w, r, data) })
	t.Cleanup(func() { MarkFile = nil })
	var removed atomic.Bool
	MarkFile = func(path, _, _ string) error {
		if removed.CompareAndSwap(false, true) {
			_ = os.Remove(path)
			return errors.New("removed")
		}
		return nil
	}
	dir := t.TempDir()
	d := New("av", srv.URL+"/tool.zip", dir, 4, nil, 0, srv.Client())
	t.Cleanup(func() { closePart(d) })
	d.Start(context.Background())
	if info := d.Info(); info.Status != StatusError {
		t.Fatalf("first run: status=%s err=%q, want the antivirus error", info.Status, info.Error)
	}
	d.Start(context.Background())
	info := d.Info()
	got, _ := os.ReadFile(filepath.Join(dir, info.FileName))
	if info.Status != StatusComplete || !bytes.Equal(got, data) {
		t.Fatalf("resume: status=%s err=%q, %s is %d bytes, the file: %v",
			info.Status, info.Error, info.FileName, len(got), bytes.Equal(got, data))
	}
}

// A partial file cut shorter than the bytes counted in it is not trusted:
// the download starts again from the first byte.
func TestResumeOfAShortenedPartFetchesAgain(t *testing.T) {
	data := randomData(256 << 10)
	var hits int64
	srv := rangeServer(t, data, &hits)
	defer srv.Close()
	dir := t.TempDir()
	const half = 128 << 10
	if err := os.WriteFile(filepath.Join(dir, "a.bin.odmpart"), data[:half/2], 0o644); err != nil {
		t.Fatal(err)
	}
	d := Restore(State{
		ID: "short", URL: srv.URL + "/a.bin", FinalURL: srv.URL + "/a.bin",
		FileName: "a.bin", Dir: dir, Connections: 1, Total: int64(len(data)), RangeOK: true,
		Status: StatusPaused, Chunks: []*Chunk{{ID: 0, Start: 0, End: int64(len(data)) - 1, Done: half}},
	}, srv.Client())
	t.Cleanup(func() { closePart(d) })
	d.Start(context.Background())
	info := d.Info()
	got, _ := os.ReadFile(filepath.Join(dir, info.FileName))
	if info.Status != StatusComplete || !bytes.Equal(got, data) {
		t.Fatalf("status=%s err=%q, the file: %v", info.Status, info.Error, bytes.Equal(got, data))
	}
}

// A download keeps the folder it started in when its category's folder
// changes before it resumes: its stored bytes are used, not left behind
// for an empty file in the new folder.
func TestResumeKeepsTheFolderItStartedIn(t *testing.T) {
	setCategories(nil) // the built-in table: .mp4 is Video
	data := randomData(256 << 10)
	var hits int64
	srv := rangeServer(t, data, &hits)
	defer srv.Close()

	base := t.TempDir()
	oldDir := filepath.Join(base, "Video")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatal(err)
	}
	const half = 128 << 10
	part := make([]byte, len(data))
	copy(part, data[:half])
	if err := os.WriteFile(filepath.Join(oldDir, "clip.mp4.odmpart"), part, 0o644); err != nil {
		t.Fatal(err)
	}
	d := Restore(State{
		ID: "cat", URL: srv.URL + "/clip.mp4", FinalURL: srv.URL + "/clip.mp4",
		FileName: "clip.mp4", Dir: oldDir, BaseDir: base, Connections: 2,
		Total: int64(len(data)), RangeOK: true, Status: StatusPaused,
		Chunks: []*Chunk{
			{ID: 0, Start: 0, End: half - 1, Done: half},
			{ID: 1, Start: half, End: int64(len(data)) - 1},
		},
	}, srv.Client())
	d.Categories = true
	d.SetCategoryDirs(map[string]string{"Video": filepath.Join(base, "Movies")})
	t.Cleanup(func() { closePart(d) })
	d.Start(context.Background())

	info := d.Info()
	got, _ := os.ReadFile(filepath.Join(oldDir, "clip.mp4"))
	if info.Status != StatusComplete || info.Dir != oldDir || !bytes.Equal(got, data) || d.restarted {
		t.Fatalf("status=%s err=%q dir=%s restarted=%v, the file: %v",
			info.Status, info.Error, info.Dir, d.restarted, bytes.Equal(got, data))
	}
}
