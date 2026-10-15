package engine

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"strings"
)

// checksumAlgos are the hashes a download can be checked against, by name
// and by the length of their hex digest.
var checksumAlgos = []struct {
	name   string
	hexLen int
	new    func() hash.Hash
}{
	{"md5", 32, md5.New},
	{"sha1", 40, sha1.New},
	{"sha256", 64, sha256.New},
	{"sha512", 128, sha512.New},
}

// ParseChecksum reads the hash a file must have: "sha256:<hex>" (also sha1,
// sha512 and md5, with ":" or "=", any case, "SHA-256" too) or the hex
// alone, whose length names the algorithm. It returns "algo:hex" in lower
// case, or "" for an empty string.
func ParseChecksum(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	algo, digest := "", s
	if i := strings.IndexAny(s, ":="); i >= 0 {
		algo = strings.ReplaceAll(strings.ToLower(strings.TrimSpace(s[:i])), "-", "")
		digest = strings.TrimSpace(s[i+1:])
	}
	digest = strings.ToLower(digest)
	if _, err := hex.DecodeString(digest); err != nil || digest == "" {
		return "", fmt.Errorf("checksum %q is not hexadecimal", digest)
	}
	for _, a := range checksumAlgos {
		if (algo == "" && len(digest) == a.hexLen) || algo == a.name {
			if len(digest) != a.hexLen {
				return "", fmt.Errorf("a %s checksum has %d hex digits, not %d", strings.ToUpper(a.name), a.hexLen, len(digest))
			}
			return a.name + ":" + digest, nil
		}
	}
	if algo != "" {
		return "", fmt.Errorf("unknown checksum type %q (use SHA-256, SHA-1, SHA-512 or MD5)", algo)
	}
	return "", fmt.Errorf("a checksum of %d hex digits is not SHA-256, SHA-1, SHA-512 or MD5", len(digest))
}

// ChecksumName is how a checksum's algorithm is written for people.
func ChecksumName(sum string) string {
	algo, _, _ := strings.Cut(sum, ":")
	switch algo {
	case "sha1":
		return "SHA-1"
	case "sha256":
		return "SHA-256"
	case "sha512":
		return "SHA-512"
	}
	return strings.ToUpper(algo)
}

// ErrChecksum is wrapped by the error of a download whose file does not
// have the hash it was given.
var ErrChecksum = errors.New("checksum does not match")

// fileChecksum hashes the file at path with the algorithm of want
// ("algo:hex") and returns the result in the same form.
func fileChecksum(path, want string) (string, error) {
	algo, _, _ := strings.Cut(want, ":")
	for _, a := range checksumAlgos {
		if a.name != algo {
			continue
		}
		f, err := os.Open(path)
		if err != nil {
			return "", err
		}
		defer f.Close()
		h := a.new()
		if _, err := io.CopyBuffer(h, f, make([]byte, 1<<20)); err != nil {
			return "", err
		}
		return algo + ":" + hex.EncodeToString(h.Sum(nil)), nil
	}
	return "", fmt.Errorf("unknown checksum type %q", algo)
}

// verifyChecksum checks the finished file at path against the download's
// checksum. On a mismatch the next run fetches every byte again: the bytes
// on disk are not trusted, whichever of them is wrong.
func (d *Download) verifyChecksum(path string) error {
	d.mu.Lock()
	want := d.Checksum
	d.mu.Unlock()
	if want == "" {
		return nil
	}
	d.mu.Lock()
	d.verifying = true
	d.mu.Unlock()
	d.notify(true)
	got, err := fileChecksum(path, want)
	d.mu.Lock()
	d.verifying = false
	d.mu.Unlock()
	if err != nil {
		return fmt.Errorf("checking the %s checksum: %w", ChecksumName(want), err)
	}
	if got == want {
		return nil
	}
	d.mu.Lock()
	d.discardPart = true
	switch {
	case d.RangeOK && d.Total > 0:
		d.Chunks = NewPlan(d.Total, d.Connections)
	case d.Total > 0:
		d.Chunks = []*Chunk{{ID: 0, Start: 0, End: d.Total - 1}}
	default:
		d.Chunks = nil
	}
	d.mu.Unlock()
	short := func(s string) string {
		_, h, _ := strings.Cut(s, ":")
		if len(h) > 16 {
			h = h[:16] + "…"
		}
		return h
	}
	return fmt.Errorf("%w: %s should be %s but is %s; Resume downloads the file again",
		ErrChecksum, ChecksumName(want), short(want), short(got))
}
