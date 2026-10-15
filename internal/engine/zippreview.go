package engine

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"sort"
	"strings"
	"sync"
	"time"
)

// ZipEntry is one file or folder inside a ZIP archive.
type ZipEntry struct {
	Name       string
	Size       int64 // uncompressed
	Compressed int64
	Modified   time.Time
	Dir        bool
	Encrypted  bool
}

// ZipListing is what a ZIP archive holds, read without downloading it.
type ZipListing struct {
	Entries []ZipEntry
	Files   int   // entries that are not folders
	Size    int64 // uncompressed size of all files
	Fetched int64 // bytes read from the server to list it
}

// IsZipName reports whether a file name or content type is a ZIP archive.
func IsZipName(name, contentType string) bool {
	ct := strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))
	return strings.EqualFold(path.Ext(name), ".zip") ||
		ct == "application/zip" || ct == "application/x-zip-compressed"
}

// errNoZipPreview is why an archive cannot be listed without downloading.
var errNoZipPreview = errors.New("the server does not send parts of the file, so the archive cannot be listed before it is downloaded")

// maxZipPreviewBytes caps what a preview may fetch (a central directory of
// many thousand entries is a few megabytes).
const maxZipPreviewBytes = 32 << 20

// PreviewZip lists a ZIP archive at rawURL by reading only its end, where
// ZIP keeps the list of contents, with range requests.
func (m *Manager) PreviewZip(ctx context.Context, rawURL string, headers map[string]string) (*ZipListing, error) {
	client := m.HTTPClient()
	p, err := inspectWith(ctx, client, strings.TrimSpace(rawURL), headers, m.authz)
	if err != nil {
		return nil, err
	}
	if !p.RangeOK || p.Total <= 0 {
		return nil, errNoZipPreview
	}
	r := &rangeReaderAt{ctx: ctx, client: client, url: p.FinalURL, size: p.Total,
		headers: headers, cookies: p.Cookies, authz: m.authz, blocks: map[int64][]byte{}}
	zr, err := zip.NewReader(r, p.Total)
	if err != nil {
		if r.err != nil {
			return nil, r.err
		}
		return nil, fmt.Errorf("not a ZIP archive this preview can read: %w", err)
	}
	out := &ZipListing{}
	for _, f := range zr.File {
		e := ZipEntry{
			Name:       f.Name,
			Size:       int64(f.UncompressedSize64),
			Compressed: int64(f.CompressedSize64),
			Modified:   zipModTime(f.Modified),
			Dir:        strings.HasSuffix(f.Name, "/"),
			Encrypted:  f.Flags&0x1 != 0,
		}
		if !e.Dir {
			out.Files++
			out.Size += e.Size
		}
		out.Entries = append(out.Entries, e)
	}
	sort.SliceStable(out.Entries, func(i, j int) bool {
		return strings.ToLower(out.Entries[i].Name) < strings.ToLower(out.Entries[j].Name)
	})
	out.Fetched = r.fetched
	return out, nil
}

// zipModTime gives an entry's time as the moment it names. Many tools
// (Explorer, Python) store only the MS-DOS time, a wall clock with no zone,
// which archive/zip labels UTC; shown as local time it would be off by the
// zone's offset. Such a time is read as a local wall clock instead. An
// entry with an extended timestamp has its own zone and is kept.
func zipModTime(t time.Time) time.Time {
	if t.IsZero() || t.Location() != time.UTC {
		return t
	}
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.Local)
}

// rangeReaderAt reads a remote file with range requests, in 64 KiB blocks
// that are kept, so archive/zip's many small reads cost few requests.
type rangeReaderAt struct {
	ctx     context.Context
	client  *http.Client
	url     string
	size    int64
	headers map[string]string
	cookies []*http.Cookie
	authz   *authorizer

	mu      sync.Mutex
	blocks  map[int64][]byte
	fetched int64
	err     error // the first failure, reported instead of zip's own error
}

const rangeBlock = 64 << 10

func (r *rangeReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if off >= r.size {
		return 0, io.EOF
	}
	n := 0
	for n < len(p) && off+int64(n) < r.size {
		pos := off + int64(n)
		b, err := r.block(pos / rangeBlock)
		if err != nil {
			return n, err
		}
		n += copy(p[n:], b[pos%rangeBlock:])
	}
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

func (r *rangeReaderAt) block(i int64) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if b, ok := r.blocks[i]; ok {
		return b, nil
	}
	if r.err != nil {
		return nil, r.err
	}
	start := i * rangeBlock
	end := start + rangeBlock - 1
	if end >= r.size {
		end = r.size - 1
	}
	if r.fetched+end-start+1 > maxZipPreviewBytes {
		r.err = errors.New("the archive's list of contents is too large to preview")
		return nil, r.err
	}
	b, err := r.fetch(start, end)
	if err != nil {
		r.err = err
		return nil, err
	}
	r.blocks[i] = b
	r.fetched += int64(len(b))
	return b, nil
}

func (r *rangeReaderAt) fetch(start, end int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(r.ctx, http.MethodGet, r.url, nil)
	if err != nil {
		return nil, err
	}
	applyHeaders(req, r.headers)
	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	if len(r.cookies) > 0 {
		req.Header.Set("Cookie", cookieHeader(r.cookies))
	}
	resp, err := r.authz.do(req, r.client.Do)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := busyResponse(resp, time.Now()); err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusPartialContent {
		return nil, errNoZipPreview
	}
	if cs, _, _, ok := ParseContentRange(resp.Header.Get("Content-Range")); ok && cs != start {
		return nil, fmt.Errorf("range mismatch: asked %d, got %d", start, cs)
	}
	want := end - start + 1
	b, err := io.ReadAll(io.LimitReader(resp.Body, want))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) != want {
		return nil, fmt.Errorf("short answer: %d of %d bytes", len(b), want)
	}
	return b, nil
}
