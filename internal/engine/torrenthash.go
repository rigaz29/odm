package engine

// The info hashes that name a torrent, read without odm-libtorrent: a
// magnet link's, or a .torrent file's (SHA-1 and SHA-256 of its info
// dictionary, as BitTorrent v1 and v2 count them). ODM finds the same
// torrent added twice by them: odm-libtorrent runs a torrent once, and
// refuses a second download of it.

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// errBadTorrent: the data is not a .torrent file.
var errBadTorrent = errors.New("not a .torrent file")

// MagnetHashes returns the v1 and v2 info hashes a magnet link names, in
// lowercase hex; "" for one it does not name.
func MagnetHashes(link string) (v1, v2 string) {
	s := strings.TrimSpace(link)
	if !IsMagnet(s) {
		return "", ""
	}
	q, _ := url.ParseQuery(s[len("magnet:?"):]) // what it read, even past a bad part
	for _, xt := range q["xt"] {
		lower := strings.ToLower(xt)
		switch {
		case strings.HasPrefix(lower, "urn:btih:"):
			h := xt[len("urn:btih:"):]
			if b, err := hex.DecodeString(h); err == nil && len(b) == sha1.Size {
				v1 = hex.EncodeToString(b)
			} else if b, err := base32.StdEncoding.DecodeString(strings.ToUpper(h)); err == nil && len(b) == sha1.Size {
				v1 = hex.EncodeToString(b)
			}
		case strings.HasPrefix(lower, "urn:btmh:1220"): // multihash: SHA-256, 32 bytes
			if b, err := hex.DecodeString(xt[len("urn:btmh:1220"):]); err == nil && len(b) == sha256.Size {
				v2 = hex.EncodeToString(b)
			}
		}
	}
	return v1, v2
}

// TorrentFileHashes returns the v1 and v2 info hashes of a .torrent file's
// contents, in lowercase hex: v1 is "" for a torrent of v2 alone, v2 "" for
// one of v1 alone.
func TorrentFileHashes(data []byte) (v1, v2 string, err error) {
	info, ok, err := bencDictValue(data, "info")
	if err != nil {
		return "", "", err
	}
	if !ok || len(info) == 0 || info[0] != 'd' {
		return "", "", errBadTorrent
	}
	if _, ok, err := bencDictValue(info, "pieces"); err != nil {
		return "", "", err
	} else if ok {
		sum := sha1.Sum(info)
		v1 = hex.EncodeToString(sum[:])
	}
	if mv, ok, err := bencDictValue(info, "meta version"); err != nil {
		return "", "", err
	} else if ok && string(mv) == "i2e" {
		sum := sha256.Sum256(info)
		v2 = hex.EncodeToString(sum[:])
	}
	if v1 == "" && v2 == "" {
		return "", "", errBadTorrent
	}
	return v1, v2, nil
}

// torrentFileHashes is TorrentFileHashes of the .torrent file at path; ""
// when it cannot be read.
func torrentFileHashes(path string) (v1, v2 string) {
	data, err := os.ReadFile(path)
	if err != nil || len(data) > MaxTorrentFile {
		return "", ""
	}
	v1, v2, _ = TorrentFileHashes(data)
	return v1, v2
}

// sameTorrent reports whether two torrents, by their v1 and v2 info
// hashes, are one: a hybrid torrent's magnet link may name only one.
func sameTorrent(a1, a2, b1, b2 string) bool {
	return a1 != "" && strings.EqualFold(a1, b1) || a2 != "" && strings.EqualFold(a2, b2)
}

// bencDepth bounds the nesting bencEnd follows; a torrent's goes a few deep.
const bencDepth = 64

// bencEnd returns where the bencoded value at b[i:] ends.
func bencEnd(b []byte, i, depth int) (int, error) {
	if depth > bencDepth || i >= len(b) {
		return 0, errBadTorrent
	}
	switch c := b[i]; {
	case c == 'i':
		j := bytes.IndexByte(b[i:], 'e')
		if j < 0 {
			return 0, errBadTorrent
		}
		return i + j + 1, nil
	case c == 'l' || c == 'd':
		// A dictionary's keys and values alternate: each is skipped alike.
		i++
		for i < len(b) && b[i] != 'e' {
			var err error
			if i, err = bencEnd(b, i, depth+1); err != nil {
				return 0, err
			}
		}
		if i >= len(b) {
			return 0, errBadTorrent
		}
		return i + 1, nil
	case c >= '0' && c <= '9':
		j := bytes.IndexByte(b[i:], ':')
		if j < 0 {
			return 0, errBadTorrent
		}
		n, err := strconv.Atoi(string(b[i : i+j]))
		if err != nil || n < 0 || n > len(b)-(i+j+1) {
			return 0, errBadTorrent
		}
		return i + j + 1 + n, nil
	}
	return 0, errBadTorrent
}

// bencDictValue returns the bencoded value of key in the dictionary d, as
// it is in d; false when d has no such key.
func bencDictValue(d []byte, key string) ([]byte, bool, error) {
	if len(d) == 0 || d[0] != 'd' {
		return nil, false, errBadTorrent
	}
	for i := 1; i < len(d) && d[i] != 'e'; {
		if d[i] < '0' || d[i] > '9' {
			return nil, false, errBadTorrent // keys are strings
		}
		kend, err := bencEnd(d, i, 1)
		if err != nil {
			return nil, false, err
		}
		vend, err := bencEnd(d, kend, 1)
		if err != nil {
			return nil, false, err
		}
		k := d[i:kend]
		if string(k[bytes.IndexByte(k, ':')+1:]) == key {
			return d[kend:vend], true, nil
		}
		i = vend
	}
	return nil, false, nil
}
