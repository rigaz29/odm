package engine

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func buildZip(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range files {
		h := &zip.FileHeader{Name: name, Method: zip.Deflate, Modified: time.Date(2026, 10, 1, 9, 30, 0, 0, time.UTC)}
		w, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write(data)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestPreviewZipReadsOnlyTheEnd(t *testing.T) {
	big := randomData(3 << 20) // incompressible: the archive is about 3 MB
	archive := buildZip(t, map[string][]byte{
		"docs/readme.txt": []byte(strings.Repeat("hello ", 200)),
		"bin/tool.exe":    big,
		"docs/":           nil,
	})
	var served int64
	srv := httpServer(t, func(w http.ResponseWriter, r *http.Request) {
		start, end, _ := requestedSpan(r.Header.Get("Range"), len(archive))
		if end >= len(archive) {
			end = len(archive) - 1
		}
		atomic.AddInt64(&served, int64(end-start+1))
		serveRange(w, r, archive)
	})
	m := NewManager(Settings{Dir: t.TempDir()}, "")
	list, err := m.PreviewZip(context.Background(), srv.URL+"/pack.zip", nil)
	if err != nil {
		t.Fatal(err)
	}
	if list.Files != 2 || len(list.Entries) != 3 {
		t.Fatalf("%d files, %d entries", list.Files, len(list.Entries))
	}
	var tool ZipEntry
	for _, e := range list.Entries {
		if e.Name == "bin/tool.exe" {
			tool = e
		}
	}
	if tool.Size != int64(len(big)) || tool.Modified.IsZero() {
		t.Fatalf("tool.exe entry %+v", tool)
	}
	if n := atomic.LoadInt64(&served); n > 256<<10 {
		t.Fatalf("read %d bytes of a %d-byte archive to list it", n, len(archive))
	}
}

func TestPreviewZipManyEntries(t *testing.T) {
	files := map[string][]byte{}
	for i := 0; i < 3000; i++ {
		files[fmt.Sprintf("dir%02d/file%04d.txt", i%20, i)] = []byte(strconv.Itoa(i))
	}
	archive := buildZip(t, files)
	srv := httpServer(t, func(w http.ResponseWriter, r *http.Request) { serveRange(w, r, archive) })
	m := NewManager(Settings{Dir: t.TempDir()}, "")
	list, err := m.PreviewZip(context.Background(), srv.URL+"/many.zip", nil)
	if err != nil {
		t.Fatal(err)
	}
	if list.Files != 3000 {
		t.Fatalf("%d files listed, want 3000", list.Files)
	}
}

func TestPreviewZipRefusals(t *testing.T) {
	m := NewManager(Settings{Dir: t.TempDir()}, "")

	// No ranges: the archive would have to be downloaded.
	archive := buildZip(t, map[string][]byte{"a.txt": []byte("a")})
	noRange := httpServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(archive)))
		_, _ = w.Write(archive)
	})
	if _, err := m.PreviewZip(context.Background(), noRange.URL+"/a.zip", nil); err == nil || !strings.Contains(err.Error(), "parts of the file") {
		t.Fatalf("no ranges: %v", err)
	}

	// Not a ZIP at all.
	notZip := randomData(100 << 10)
	srv := httpServer(t, func(w http.ResponseWriter, r *http.Request) { serveRange(w, r, notZip) })
	if _, err := m.PreviewZip(context.Background(), srv.URL+"/fake.zip", nil); err == nil || !strings.Contains(err.Error(), "not a ZIP") {
		t.Fatalf("not a zip: %v", err)
	}
}

func TestZipModTime(t *testing.T) {
	dos := time.Date(2026, 10, 15, 13, 17, 0, 0, time.UTC) // MS-DOS time: no zone
	got := zipModTime(dos)
	if got.Location() != time.Local || got.Hour() != 13 || got.Minute() != 17 {
		t.Fatalf("MS-DOS time read as %v, want 13:17 local", got)
	}
	ext := time.Date(2026, 10, 15, 6, 17, 0, 0, time.FixedZone("", 0)) // extended timestamp
	if !zipModTime(ext).Equal(ext) {
		t.Fatal("an extended timestamp was changed")
	}
}

func TestIsZipName(t *testing.T) {
	if !IsZipName("a.ZIP", "") || !IsZipName("download", "application/zip; charset=binary") || IsZipName("a.rar", "application/octet-stream") {
		t.Fatal("IsZipName")
	}
}
