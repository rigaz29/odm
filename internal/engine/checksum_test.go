package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestParseChecksum(t *testing.T) {
	sha := strings.Repeat("ab", 32)
	cases := []struct {
		in, want string
		bad      bool
	}{
		{"", "", false},
		{sha, "sha256:" + sha, false},
		{"SHA-256:" + strings.ToUpper(sha), "sha256:" + sha, false},
		{"sha256=" + sha, "sha256:" + sha, false},
		{"  md5: " + strings.Repeat("0f", 16) + " ", "md5:" + strings.Repeat("0f", 16), false},
		{strings.Repeat("1", 40), "sha1:" + strings.Repeat("1", 40), false},
		{strings.Repeat("2", 128), "sha512:" + strings.Repeat("2", 128), false},
		{"sha256:" + strings.Repeat("ab", 16), "", true}, // an MD5's length
		{"crc32:deadbeef", "", true},
		{"xyz", "", true},
		{strings.Repeat("a", 50), "", true},
	}
	for _, c := range cases {
		got, err := ParseChecksum(c.in)
		if (err != nil) != c.bad || got != c.want {
			t.Errorf("ParseChecksum(%q) = %q, %v; want %q (error %v)", c.in, got, err, c.want, c.bad)
		}
	}
	if ChecksumName("sha256:x") != "SHA-256" || ChecksumName("md5:x") != "MD5" {
		t.Fatal("ChecksumName")
	}
}

func TestChecksumVerifiesAndRefetches(t *testing.T) {
	data := randomData(300 << 10)
	sum := sha256.Sum256(data)
	want := "sha256:" + hex.EncodeToString(sum[:])

	var corrupt atomic.Bool
	corrupt.Store(true)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := data
		if corrupt.Load() && strings.HasPrefix(r.Header.Get("Range"), "bytes=0-") && r.Header.Get("Range") != "bytes=0-0" {
			body = append([]byte(nil), data...)
			body[1000] ^= 0xFF // one bad byte, as a broken proxy or disk might give
		}
		serveRange(w, r, body)
	}))
	defer srv.Close()

	dir := t.TempDir()
	d := New("sum", srv.URL+"/file.bin", dir, 2, nil, 0, srv.Client())
	d.Checksum = want
	d.Start(context.Background())
	info := d.Info()
	if info.Status != StatusError || !strings.Contains(info.Error, "SHA-256") || !strings.Contains(info.Error, "Resume") {
		t.Fatalf("a corrupt file ended as %s (%q)", info.Status, info.Error)
	}
	if _, err := os.Stat(filepath.Join(dir, "file.bin")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("the corrupt file took its final name")
	}
	for _, c := range d.State().Chunks {
		if c.Done != 0 {
			t.Fatalf("chunk %d keeps %d bytes; every byte should be fetched again", c.ID, c.Done)
		}
	}

	// The server is fixed; Resume fetches the whole file again.
	corrupt.Store(false)
	d.Start(context.Background())
	info = d.Info()
	if info.Status != StatusComplete {
		t.Fatalf("after Resume: %s (%q)", info.Status, info.Error)
	}
	got, err := os.ReadFile(filepath.Join(dir, info.FileName))
	if err != nil || string(got) != string(data) {
		t.Fatalf("file after Resume does not match (%v)", err)
	}
	if info.Checksum != want || info.Verifying {
		t.Fatalf("info checksum %q, verifying %v", info.Checksum, info.Verifying)
	}
}

func TestChecksumFromAddOptions(t *testing.T) {
	m := NewManager(Settings{Dir: t.TempDir()}, "")
	sha := strings.Repeat("cd", 32)
	if got := m.Create(AddOptions{URL: "http://h/a.bin", Checksum: "SHA256:" + sha}).Checksum; got != "sha256:"+sha {
		t.Fatalf("checksum %q", got)
	}
	if got := m.Create(AddOptions{URL: "http://h/b.bin", Checksum: "nonsense"}).Checksum; got != "" {
		t.Fatalf("an unreadable checksum was kept: %q", got)
	}
}
