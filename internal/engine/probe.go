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
	"time"
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
	return inspectWith(ctx, client, rawURL, headers, nil)
}

// inspectWith is Inspect answering an authentication challenge with a site
// login through a (nil sends the request as it is).
func inspectWith(ctx context.Context, client *http.Client, rawURL string, headers map[string]string, a *authorizer) (*Probe, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	// One byte. A range that covers the whole file may be answered with 200,
	// which would hide that the server supports ranges.
	req.Header.Set("Range", "bytes=0-0")
	req.Header.Set("Accept-Encoding", "identity")
	applyHeaders(req, headers)

	resp, err := a.do(req, client.Do)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := busyResponse(resp, time.Now()); err != nil {
		return nil, err
	}

	p := &Probe{
		FinalURL:     resp.Request.URL.String(),
		ContentType:  resp.Header.Get("Content-Type"),
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
		Cookies:      resp.Cookies(),
	}

	switch resp.StatusCode {
	case http.StatusPartialContent:
		// "bytes 0-0/*" names no total: ranges cannot be planned without it.
		if _, _, total, ok := ParseContentRange(resp.Header.Get("Content-Range")); ok && total > 0 {
			p.RangeOK = true
			p.Total = total
		}
	case http.StatusOK:
		if resp.ContentLength > 0 {
			p.Total = resp.ContentLength
		}
	default:
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}
	// Total 0: the server did not say how big the file is (a generated
	// archive, a chunked response). It is fetched on one connection, and its
	// size is known when the response ends.
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
// on a character boundary, keeping its extension. Characters that change
// the direction text is shown in are dropped. "" means nothing usable was
// left.
func SanitizeName(name string) string {
	name = strings.Map(func(r rune) rune {
		if bidiControl(r) {
			return -1
		}
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
	if windowsReserved(name) {
		name = "_" + name
	}
	return name
}

// bidiControl reports whether r changes the direction of the text around
// it. In a file name it disguises the real extension: "photo‮gpj.exe"
// is shown as "photoexe.jpg", an executable that looks like a picture.
func bidiControl(r rune) bool {
	switch {
	case r == '؜', r == '‎', r == '‏': // ALM, LRM, RLM
		return true
	case r >= '‪' && r <= '‮': // LRE, RLE, PDF, LRO, RLO
		return true
	case r >= '⁦' && r <= '⁩': // LRI, RLI, FSI, PDI
		return true
	}
	return false
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

// windowsReserved reports whether Windows treats name as a device (CON,
// NUL, COM1, …): there opening "nul.mp4" writes to the null device and
// "con.mp4" fails. The device name counts with any extension and with
// spaces before it, so only the part before the first dot is compared.
// Checked on every OS, as the file may be copied to Windows later.
func windowsReserved(name string) bool {
	stem, _, _ := strings.Cut(name, ".")
	stem = strings.ToUpper(strings.TrimRight(stem, " "))
	switch stem {
	case "CON", "PRN", "AUX", "NUL", "CONIN$", "CONOUT$":
		return true
	}
	if len(stem) >= 4 && (strings.HasPrefix(stem, "COM") || strings.HasPrefix(stem, "LPT")) {
		switch stem[3:] {
		case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "¹", "²", "³":
			return true
		}
	}
	return false
}
