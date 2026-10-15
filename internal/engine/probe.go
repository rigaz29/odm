package engine

import (
	"context"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Probe captures what a target advertises before the transfer starts.
type Probe struct {
	FinalURL     string
	Total        int64
	RangeOK      bool
	FileName     string
	ContentType  string
	ETag         string
	LastModified string
	Cookies      []*http.Cookie
}

// Inspect issues a ranged GET for the first byte to learn the size, whether
// the server honours Range requests, the suggested file name and cookies.
func Inspect(ctx context.Context, client *http.Client, rawURL string, headers map[string]string) (*Probe, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Range", "bytes=0-")
	req.Header.Set("Accept-Encoding", "identity")
	applyHeaders(req, headers)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	p := &Probe{
		FinalURL:     resp.Request.URL.String(),
		ContentType:  resp.Header.Get("Content-Type"),
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
		Cookies:      resp.Cookies(),
	}

	switch resp.StatusCode {
	case http.StatusPartialContent:
		if _, _, total, ok := ParseContentRange(resp.Header.Get("Content-Range")); ok {
			p.RangeOK = true
			p.Total = total
		}
	case http.StatusOK:
		p.Total = resp.ContentLength
	default:
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}
	if p.Total <= 0 {
		return nil, fmt.Errorf("server did not report a content length")
	}
	p.FileName = FileName(resp, rawURL)
	return p, nil
}

// ParseContentRange parses "bytes 1-10/100" or "bytes */100".
func ParseContentRange(v string) (start, end, total int64, ok bool) {
	v = strings.TrimSpace(v)
	if !strings.HasPrefix(strings.ToLower(v), "bytes") {
		return 0, 0, 0, false
	}
	v = strings.TrimSpace(v[len("bytes"):])
	slash := strings.IndexByte(v, '/')
	if slash < 0 {
		return 0, 0, 0, false
	}
	rangePart := strings.TrimSpace(v[:slash])
	totalPart := strings.TrimSpace(v[slash+1:])
	if rangePart != "*" {
		dash := strings.IndexByte(rangePart, '-')
		if dash < 0 {
			return 0, 0, 0, false
		}
		s, err1 := strconv.ParseInt(strings.TrimSpace(rangePart[:dash]), 10, 64)
		e, err2 := strconv.ParseInt(strings.TrimSpace(rangePart[dash+1:]), 10, 64)
		if err1 != nil || err2 != nil {
			return 0, 0, 0, false
		}
		start, end = s, e
	}
	if totalPart != "*" {
		t, err := strconv.ParseInt(totalPart, 10, 64)
		if err != nil {
			return 0, 0, 0, false
		}
		total = t
	}
	return start, end, total, true
}

// FileName derives a safe local file name from the response and URL.
func FileName(resp *http.Response, rawURL string) string {
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		if _, params, err := mime.ParseMediaType(cd); err == nil {
			if fn := params["filename"]; fn != "" {
				if s := SanitizeName(fn); s != "" {
					return s
				}
			}
		}
	}
	if u, err := url.Parse(resp.Request.URL.String()); err == nil {
		if base := path.Base(u.Path); base != "" && base != "/" && base != "." {
			if s := SanitizeName(base); s != "" {
				return s
			}
		}
	}
	if u, err := url.Parse(rawURL); err == nil {
		if base := path.Base(u.Path); base != "" && base != "/" && base != "." {
			if s := SanitizeName(base); s != "" {
				return s
			}
		}
	}
	return "download.bin"
}

// maxNameBytes leaves room for " (n)" and ".odmpart" within the 255-unit
// name limit of common file systems.
const maxNameBytes = 200

// maxKeptExt is the longest extension kept whole when a name is shortened.
const maxKeptExt = 16

// SanitizeName turns name into a file name that Windows accepts, and so
// every other system: path separators, reserved and control characters
// become "_", dots and spaces are trimmed from the ends, a device name such
// as "CON" or "com1.txt" gets a leading "_", and a long name is shortened
// on a character boundary, keeping its extension. "" means nothing usable
// was left.
func SanitizeName(name string) string {
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || strings.ContainsRune(`\/:<>"|?*`, r) {
			return '_'
		}
		return r
	}, name)
	name = strings.Trim(strings.TrimSpace(name), ". ")
	if len(name) > maxNameBytes {
		name = shortenName(name, maxNameBytes)
	}
	if name == "" {
		return ""
	}
	if reservedDeviceName(name) {
		name = "_" + name
	}
	return name
}

// shortenName cuts name to at most limit bytes without splitting a UTF-8
// sequence, keeping a short extension so the file still opens with the
// right program. Windows drops trailing dots and spaces, so none are left.
func shortenName(name string, limit int) string {
	ext := path.Ext(name)
	if len(ext) > maxKeptExt {
		ext = ""
	}
	stem := strings.TrimSuffix(name, ext)
	for len(stem)+len(ext) > limit {
		_, size := utf8.DecodeLastRuneInString(stem)
		stem = stem[:len(stem)-size]
	}
	stem = strings.TrimRight(stem, ". ")
	if stem == "" {
		return ""
	}
	return stem + ext
}

// reservedDeviceName reports whether Windows treats name as a device (CON,
// PRN, AUX, NUL, COM1-COM9, LPT1-LPT9), with or without an extension.
// Creating such a file opens the device instead.
func reservedDeviceName(name string) bool {
	stem := name
	if i := strings.IndexByte(stem, '.'); i >= 0 {
		stem = stem[:i]
	}
	stem = strings.ToUpper(strings.TrimRight(stem, " "))
	switch stem {
	case "CON", "PRN", "AUX", "NUL":
		return true
	}
	return len(stem) == 4 && (stem[:3] == "COM" || stem[:3] == "LPT") && stem[3] >= '1' && stem[3] <= '9'
}
