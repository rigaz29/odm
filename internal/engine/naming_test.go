package engine

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestNumberedName(t *testing.T) {
	cases := map[string]string{
		"file.zip":      "file (2).zip",
		"backup.tar.gz": "backup (2).tar.gz",
		"Data.TAR.XZ":   "Data (2).TAR.XZ",
		"README":        "README (2)",
		"v1.2.3.exe":    "v1.2.3 (2).exe",
		".hidden":       " (2).hidden",
	}
	for in, want := range cases {
		if got := numberedName(in, 2); got != want {
			t.Errorf("numberedName(%q, 2) = %q, want %q", in, got, want)
		}
	}
	if numberedName("a.zip", 1) != "a.zip" {
		t.Fatal("n=1 must keep the name")
	}
}

func payload(n int) []byte { return randomData(n) }

// An existing file with the same name must survive; finishing used to
// delete it.
func TestNewDownloadDoesNotReplaceExistingFile(t *testing.T) {
	data := payload(200 * 1024)
	var hits int64
	srv := rangeServer(t, data, &hits)
	defer srv.Close()
	dir := t.TempDir()
	old := []byte("user's own file")
	if err := os.WriteFile(filepath.Join(dir, "file.bin"), old, 0o644); err != nil {
		t.Fatal(err)
	}

	d := New("n1", srv.URL+"/file.bin", dir, 4, nil, 0, srv.Client())
	d.Start(context.Background())
	info := d.Info()
	if info.Status != StatusComplete {
		t.Fatalf("status %s: %s", info.Status, info.Error)
	}
	if info.FileName != "file (2).bin" {
		t.Fatalf("file name %q, want file (2).bin", info.FileName)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "file.bin")); !bytes.Equal(got, old) {
		t.Fatal("the existing file was overwritten")
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "file (2).bin")); !bytes.Equal(got, data) {
		t.Fatal("downloaded bytes do not match")
	}
}

// Two downloads of the same name started together must not share an
// .odmpart; each gets its own file with intact content.
func TestConcurrentSameNameGetSeparateFiles(t *testing.T) {
	data := payload(300 * 1024)
	var hits int64
	srv := rangeServer(t, data, &hits)
	defer srv.Close()
	dir := t.TempDir()

	var wg sync.WaitGroup
	ds := []*Download{
		New("c1", srv.URL+"/same.bin", dir, 4, nil, 0, srv.Client()),
		New("c2", srv.URL+"/same.bin", dir, 4, nil, 0, srv.Client()),
	}
	for _, d := range ds {
		wg.Add(1)
		go func(d *Download) { defer wg.Done(); d.Start(context.Background()) }(d)
	}
	wg.Wait()

	names := map[string]bool{}
	for _, d := range ds {
		info := d.Info()
		if info.Status != StatusComplete {
			t.Fatalf("%s: %s %s", info.ID, info.Status, info.Error)
		}
		names[info.FileName] = true
		if got, _ := os.ReadFile(filepath.Join(dir, info.FileName)); !bytes.Equal(got, data) {
			t.Fatalf("%s: content corrupted", info.FileName)
		}
	}
	if len(names) != 2 {
		t.Fatalf("both downloads used %v", names)
	}
}

// A resumed download keeps its name even though its own .odmpart exists.
func TestResumeKeepsClaimedName(t *testing.T) {
	dir := t.TempDir()
	name, f, err := claimName(dir, "keep.bin")
	if err != nil || name != "keep.bin" {
		t.Fatalf("claim: %q %v", name, err)
	}
	f.Close()
	if name2, f2, _ := claimName(dir, "keep.bin"); name2 != "keep (2).bin" {
		t.Fatalf("second claim got %q", name2)
	} else {
		f2.Close()
	}
}
