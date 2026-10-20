package engine

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestMagnetHashes(t *testing.T) {
	const v1 = "322cba6e0dacc7a6c25d2d1f0e773fa2ba18017b"
	const v2 = "c2691ed0e03c93252787f0e8f21ae25cf07cb76373c432862c21c5dc3d59c164"
	for _, c := range []struct {
		link, v1, v2 string
	}{
		{"magnet:?xt=urn:btih:" + strings.ToUpper(v1) + "&dn=Sample", v1, ""},
		{"magnet:?xt=urn:btih:GIWLU3QNVTD2NQS5FUPQ45Z7UK5BQAL3&tr=udp%3A%2F%2Ft", v1, ""}, // base32
		{"magnet:?xt=urn:btih:" + v1 + "&xt=urn:btmh:1220" + v2, v1, v2},
		{"  MAGNET:?xt=urn:btmh:1220" + v2, "", v2},
		{"magnet:?xt=urn:btih:abc&dn=x", "", ""},
		{"https://example.com/x.torrent", "", ""},
	} {
		if g1, g2 := MagnetHashes(c.link); g1 != c.v1 || g2 != c.v2 {
			t.Errorf("%s: %q %q, want %q %q", c.link, g1, g2, c.v1, c.v2)
		}
	}
}

func TestTorrentFileHashes(t *testing.T) {
	// A hybrid torrent odm-libtorrent made, which printed these hashes.
	data, err := os.ReadFile("testdata/hybrid.torrent")
	if err != nil {
		t.Fatal(err)
	}
	v1, v2, err := TorrentFileHashes(data)
	if err != nil || v1 != "322cba6e0dacc7a6c25d2d1f0e773fa2ba18017b" || v2 != "c2691ed0e03c93252787f0e8f21ae25cf07cb76373c432862c21c5dc3d59c164" {
		t.Fatalf("hybrid: %q %q %v", v1, v2, err)
	}

	// A torrent of v1 alone has no v2 hash.
	info := "d6:lengthi5e4:name5:a.bin12:piece lengthi16384e6:pieces20:" + strings.Repeat("p", 20) + "e"
	sum := sha1.Sum([]byte(info))
	v1, v2, err = TorrentFileHashes([]byte("d8:announce3:url4:info" + info + "e"))
	if err != nil || v1 != hex.EncodeToString(sum[:]) || v2 != "" {
		t.Fatalf("v1: %q %q %v", v1, v2, err)
	}

	for _, bad := range []string{
		"", "<html>", "d4:infoi1ee", "d4:infod4:name1:xee", "d4:info", "d4:infod6:pieces99:xee",
		strings.Repeat("l", 10000) + strings.Repeat("e", 10000),
		"d4:infod" + strings.Repeat("l", 100) + strings.Repeat("e", 100) + "ee",
	} {
		if _, _, err := TorrentFileHashes([]byte(bad)); !errors.Is(err, errBadTorrent) {
			t.Errorf("%.30q: %v", bad, err)
		}
	}
}
