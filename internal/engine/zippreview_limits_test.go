package engine

import (
	"context"
	"encoding/binary"
	"fmt"
	"net/http"
	"testing"
)

// zip64TailServer serves a file of size bytes, zeros but for a ZIP64 end
// of central directory that claims records entries in an empty list of
// contents. Only the ranges asked for are made.
func zip64TailServer(t *testing.T, size int64, records uint64) string {
	t.Helper()
	le := binary.LittleEndian
	tail := make([]byte, 0, 98)
	// ZIP64 end of central directory record (56 bytes).
	tail = le.AppendUint32(tail, zipEnd64Sig)
	tail = le.AppendUint64(tail, 44)
	tail = le.AppendUint16(tail, 45)
	tail = le.AppendUint16(tail, 45)
	tail = le.AppendUint32(tail, 0)
	tail = le.AppendUint32(tail, 0)
	tail = le.AppendUint64(tail, records)
	tail = le.AppendUint64(tail, records)
	tail = le.AppendUint64(tail, 0) // size of the list of contents
	tail = le.AppendUint64(tail, 0) // its offset
	// ZIP64 locator (20 bytes).
	tail = le.AppendUint32(tail, zipEnd64LocSig)
	tail = le.AppendUint32(tail, 0)
	tail = le.AppendUint64(tail, uint64(size-98))
	tail = le.AppendUint32(tail, 1)
	// End of central directory (22 bytes), pointing at the ZIP64 one.
	tail = le.AppendUint32(tail, zipEndSig)
	tail = le.AppendUint16(tail, 0)
	tail = le.AppendUint16(tail, 0)
	tail = le.AppendUint16(tail, 0xffff)
	tail = le.AppendUint16(tail, 0xffff)
	tail = le.AppendUint32(tail, 0xffffffff)
	tail = le.AppendUint32(tail, 0xffffffff)
	tail = le.AppendUint16(tail, 0)
	tailAt := size - int64(len(tail))
	srv := httpServer(t, func(w http.ResponseWriter, r *http.Request) {
		var a, b int64
		if _, err := fmt.Sscanf(r.Header.Get("Range"), "bytes=%d-%d", &a, &b); err != nil || a < 0 || b < a || b >= size || b-a > 1<<20 {
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return
		}
		body := make([]byte, b-a+1)
		for i := range body {
			if p := a + int64(i); p >= tailAt {
				body[i] = tail[p-tailAt]
			}
		}
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", a, b, size))
		w.Header().Set("Content-Length", fmt.Sprint(len(body)))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(body)
	})
	return srv.URL + "/huge.zip"
}

// An archive whose end record claims more entries than its list of contents
// can hold is refused before archive/zip allocates for them: a server
// claiming 2^57 entries in a 2^62-byte file made ODM panic, and 2^45 in a
// 2^50-byte one made it stop with an out-of-memory error no recover catches.
func TestZipPreviewRefusesAnImpossibleDirectory(t *testing.T) {
	for _, c := range []struct {
		size    int64
		records uint64
	}{{1 << 62, 1 << 57}, {1 << 50, 1 << 45}, {1 << 40, 1 << 20}} {
		m := NewManager(Settings{Dir: t.TempDir()}, "")
		var err error
		func() {
			defer func() {
				if p := recover(); p != nil {
					t.Fatalf("%d entries in %d bytes: PreviewZip panicked: %v", c.records, c.size, p)
				}
			}()
			_, err = m.PreviewZip(context.Background(), zip64TailServer(t, c.size, c.records), nil)
		}()
		if err == nil {
			t.Fatalf("%d entries in %d bytes: listed", c.records, c.size)
		}
	}
}

func TestCheckZipDirectoryTakesRealArchives(t *testing.T) {
	archive := buildZip(t, map[string][]byte{"a.txt": []byte("a"), "dir/b.txt": []byte("bb")})
	if err := checkZipDirectory(byteReader(archive), int64(len(archive))); err != nil {
		t.Fatal(err)
	}
}

type byteReader []byte

func (b byteReader) ReadAt(p []byte, off int64) (int, error) {
	if off >= int64(len(b)) {
		return 0, fmt.Errorf("offset %d past %d", off, len(b))
	}
	n := copy(p, b[off:])
	if n < len(p) {
		return n, fmt.Errorf("short read")
	}
	return n, nil
}
