package engine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// boundaryOverlap is how many bytes a later chunk re-reads from the chunk
// before it. The two copies are compared, so a connection that returns
// different bytes for the same offset is caught. 64 is long enough to be a
// real sample and far smaller than the minimum chunk.
const boundaryOverlap = 64

// errConflict means two reads of the same bytes did not match.
var errConflict = errors.New("connections returned different bytes")

// errConflictGaveUp is errConflict after the partial file was discarded
// and fetched again from the first byte.
var errConflictGaveUp = errors.New("downloaded data did not match; restarted from the first byte and it still did not match")

// conflictError is errConflict plus the file offset of the first byte that
// differed. Bytes before that offset are kept.
type conflictError struct {
	at int64
}

func (e *conflictError) Error() string { return errConflict.Error() }

func (e *conflictError) Unwrap() error { return errConflict }

// witness is a copy of bytes one connection read for a region another
// connection owns. It is compared when those bytes are written.
type witness struct {
	off  int64
	data []byte
}

// Download is a single managed download job.
type Download struct {
	mu sync.Mutex

	ID           string
	URL          string
	FinalURL     string
	FileName     string
	Dir          string
	BaseDir      string
	Categories   bool
	CategoryName string
	Headers      map[string]string
	Connections  int
	SpeedLimit   int64
	OpenWhenDone bool
	Queue        string
	Description  string
	Order        int

	Total   int64
	RangeOK bool
	Status  Status
	Err     string
	Chunks  []*Chunk
	// Stream is set for a segmented (HLS/DASH) download; see stream.go.
	Stream *StreamJob

	CreatedAt time.Time
	UpdatedAt time.Time

	client *http.Client
	// h1 serves several ranges at once over HTTP/1.1. The shared client may
	// use HTTP/2, which would put every range on one socket.
	h1      *http.Client
	limiter *Limiter
	shared  *Limiter
	cancel  context.CancelCauseFunc
	// transferCancel stops the connections of the current pass. It is separate
	// from cancel so one answered range can end the pass without pausing.
	transferCancel context.CancelFunc
	// abortCtx/abort belong to the current run; removed is set for good
	// once the download is being removed (see abandon).
	abortCtx context.Context
	abort    context.CancelFunc
	removed  bool
	// pauseReq records a Pause that arrived while no transfer context exists
	// yet (the manager has dispatched the download but Start has not run), so
	// Start honours it instead of the pause being silently dropped.
	pauseReq bool
	// retryBase is the first pause between attempts. Tests set a short
	// value. Zero keeps the 300ms default.
	retryBase time.Duration
	// beforeWrite runs after a filled buffer is in hand and before it is
	// written. Tests move the chunk end in that window. Nil otherwise.
	beforeWrite func()
	file        *os.File
	cookies     []*http.Cookie
	auth        func(host string) (string, string, bool)
	authz       *authorizer
	catDirs     map[string]string
	onUpdate    func(*Download)

	// connLimit is how many connections this run may open. connOpened is how
	// many workers have been started. connReady is how many of those have
	// stored file bytes. The next worker starts only once every worker
	// already opened has stored bytes.
	connLimit  int
	connOpened int
	connReady  int
	connWg     *sync.WaitGroup

	// alone is set once a ranged request is answered with the whole file.
	// Later passes and a resumed download use one connection.
	alone bool
	// sawIgnored is set for the pass that noticed that answer, so Start
	// fetches the remainder after the other connections have stopped.
	sawIgnored bool
	// pass changes when that answer is noticed. A read still holding the
	// previous plan does not write those bytes into the replacement chunk.
	pass int

	lastDone int64
	lastAt   time.Time
	speed    int64
	notifyAt time.Time

	// stallTimeout overrides DefaultStallTimeout when set. Tests use a
	// short value; a zero value means the default.
	stallTimeout time.Duration

	// ETag and LastModified identify the representation on the server.
	// A resume sends one of them as If-Range and starts over when it no
	// longer matches.
	ETag         string
	LastModified string

	discardPart bool
	changed     bool

	// single is set once two connections disagreed. Later passes, and a
	// resume, stay on one connection.
	single bool
	// restarted is set when that single connection disagreed too and the
	// partial file was discarded. One Start does this only once.
	restarted bool

	conflicted bool
	conflictAt int64
	witnesses  []witness

	// runDone is closed when the current Start returns (nil before any).
	runDone chan struct{}

	// Segmented downloads: bytes on disk, and segments present / total.
	streamDone             int64
	streamHave, streamSegs int
	// guarded is client wrapped by guardLocal for stream requests. It is
	// rebuilt when the client or the manifest (Readdress) changes, since
	// its verdict on the manifest is cached.
	guarded, guardedFrom *http.Client
	guardedFor           string
}

// New creates a queued download. Probing happens on the first Start.
func New(id, rawURL, dir string, conns int, headers map[string]string, speedLimit int64, client *http.Client) *Download {
	if conns <= 0 {
		conns = DefaultConnections
	}
	return &Download{
		ID:          id,
		URL:         rawURL,
		Dir:         dir,
		BaseDir:     dir,
		Connections: conns,
		Headers:     headers,
		SpeedLimit:  speedLimit,
		Status:      StatusQueued,
		client:      client,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

// SetLimiter attaches a shared rate limiter (a nil limiter disables limiting).
func (d *Download) SetLimiter(l *Limiter) {
	d.mu.Lock()
	d.shared = l
	if d.limiter == nil {
		d.limiter = l
	}
	d.mu.Unlock()
}

// SetSpeedLimit sets a per-download cap in bytes/sec (0 restores the shared one).
func (d *Download) SetSpeedLimit(bytesPerSecond int64) {
	d.mu.Lock()
	if bytesPerSecond > 0 {
		d.limiter = NewLimiter(bytesPerSecond)
	} else {
		d.limiter = d.shared
	}
	d.SpeedLimit = bytesPerSecond
	d.mu.Unlock()
}

// SetOpenWhenDone toggles opening the file once the transfer completes.
func (d *Download) SetOpenWhenDone(v bool) {
	d.mu.Lock()
	d.OpenWhenDone = v
	d.mu.Unlock()
}

// SetAuthLookup installs the per-host site login lookup. A login is sent
// only to a host that asks for one (401), with the scheme it names.
func (d *Download) SetAuthLookup(fn func(host string) (string, string, bool)) {
	d.mu.Lock()
	d.auth = fn
	d.authz = newAuthorizer(fn)
	d.mu.Unlock()
}

// authorizer answers this download's authentication challenges; nil (no
// site logins) sends requests as they are.
func (d *Download) authorizer() *authorizer {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.authz
}

// SetCategoryDirs installs per-category destination folders.
func (d *Download) SetCategoryDirs(dirs map[string]string) {
	cp := make(map[string]string, len(dirs))
	for k, v := range dirs {
		cp[k] = v
	}
	d.mu.Lock()
	d.catDirs = cp
	d.mu.Unlock()
}

// SetOnUpdate registers a callback invoked when visible state changes.
func (d *Download) SetOnUpdate(fn func(*Download)) { d.onUpdate = fn }

// httpClient is the client for the next request of this download.
func (d *Download) httpClient() *http.Client {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.client
}

// setClient makes later requests use c, after the proxy settings changed.
// A request already in flight finishes on the client it started with. The
// HTTP/1.1 copy for ranged connections was made from the old client, so it
// is dropped and made again from c on the next request.
func (d *Download) setClient(c *http.Client) {
	d.mu.Lock()
	d.client = c
	d.h1 = nil
	d.mu.Unlock()
}

// Start runs the transfer to completion, pause or error. It blocks.
func (d *Download) Start(parent context.Context) {
	ctx, cancel := context.WithCancelCause(parent)
	// abortCtx outlives a Stop: a recording assembles under it after its
	// own ctx is done, and only removing the download cancels it.
	abortCtx, abort := context.WithCancel(context.Background())
	defer abort()
	done := make(chan struct{})
	d.mu.Lock()
	d.runDone = done
	d.abortCtx, d.abort = abortCtx, abort
	if d.removed {
		abort()
	}
	d.mu.Unlock()
	defer close(done)
	defer func() {
		cancel(nil)
		d.mu.Lock()
		// Drop the spent cancel func: a Pause between this run and the next
		// must set pauseReq, not cancel a context that is already gone.
		d.cancel = nil
		d.pauseReq = false
		// A paused or failed run keeps no handle on the .odmpart: the next
		// run opens it again, and Windows refuses to rename or delete a
		// file that is still open.
		f := d.file
		d.file = nil
		d.mu.Unlock()
		if f != nil {
			_ = f.Close()
		}
	}()
	d.mu.Lock()
	if d.pauseReq {
		d.Status = StatusPaused
		d.mu.Unlock()
		d.notify(true)
		return
	}
	d.cancel = cancel
	d.Status = StatusDownloading
	d.Err = ""
	d.lastDone = 0
	d.lastAt = time.Now()
	// Each Start may discard the partial file once. single stays set so a
	// resume does not go back to parallel connections.
	d.restarted = false
	d.conflicted = false
	d.witnesses = nil
	d.mu.Unlock()
	d.notify(true)

	if d.Stream != nil {
		d.runStream(ctx)
		d.notify(true)
		return
	}

	// A representation change is fetched once more from the first byte.
	// A byte mismatch keeps the matching prefix on one connection, then,
	// if that also mismatches, fetches once more from the first byte.
	changes := 0
	prepareNext := true
	for pass := 0; pass < 5; pass++ {
		if prepareNext {
			if err := d.prepare(ctx); err != nil {
				// A cancelled context means pause (or shutdown) during the probe, not
				// a failure; same rule as the ctx.Err() check after the workers.
				if ctx.Err() != nil {
					d.mu.Lock()
					d.Status = StatusPaused
					d.mu.Unlock()
				} else {
					d.fail(err)
				}
				d.notify(true)
				return
			}
			prepareNext = false
		}

		transferCtx, transferCancel := context.WithCancel(ctx)
		d.mu.Lock()
		d.transferCancel = transferCancel
		d.witnesses = nil
		d.conflicted = false
		d.sawIgnored = false
		limit := d.Connections
		if !d.RangeOK || d.single || d.alone || limit < 1 {
			limit = 1
		}
		var wg sync.WaitGroup
		d.connLimit = limit
		d.connOpened = 1
		d.connReady = 0
		d.connWg = &wg
		d.mu.Unlock()
		wg.Add(1)
		go func() { defer wg.Done(); d.worker(transferCtx) }()
		wg.Wait()
		transferCancel()
		d.mu.Lock()
		d.connWg = nil
		d.transferCancel = nil
		paused := ctx.Err() != nil
		complete := d.allCompleteLocked()
		status := d.Status
		changed := d.changed
		d.changed = false
		conflicted := d.conflicted
		at := d.conflictAt
		single := d.single
		restarted := d.restarted
		again := d.sawIgnored
		d.conflicted = false
		d.mu.Unlock()

		if paused {
			// A disagreement noticed as the user pauses still sticks: the
			// resume keeps the matching prefix and uses one connection.
			if again && !changed {
				d.planAlone()
			} else if conflicted && !single && !changed {
				d.adoptSingle(at)
			}
			d.mu.Lock()
			d.Status = StatusPaused
			d.mu.Unlock()
			d.notify(true)
			return
		}
		if changed {
			if changes >= 1 {
				d.fail(errRepresentationChanged)
				d.notify(true)
				return
			}
			changes++
			prepareNext = true
			d.mu.Lock()
			// The bytes on disk belong to the old file. A later pass may
			// use every connection again.
			d.single = false
			d.restarted = false
			d.alone = false
			d.Status = StatusDownloading
			d.Err = ""
			d.mu.Unlock()
			continue
		}
		if again && status != StatusError {
			d.planAlone()
			d.mu.Lock()
			complete = d.allCompleteLocked()
			d.mu.Unlock()
			if complete {
				if err := d.finish(); err != nil {
					d.fail(err)
				} else {
					d.mu.Lock()
					d.Status = StatusComplete
					d.mu.Unlock()
				}
				d.notify(true)
				return
			}
			continue
		}
		if conflicted {
			if single && restarted {
				d.fail(errConflictGaveUp)
				d.notify(true)
				return
			}
			if single {
				if err := d.restartFromZero(); err != nil {
					d.fail(err)
					d.notify(true)
					return
				}
				continue
			}
			d.adoptSingle(at)
			continue
		}

		switch {
		case complete:
			if err := d.finish(); err != nil {
				d.fail(err)
			} else {
				d.mu.Lock()
				d.Status = StatusComplete
				d.mu.Unlock()
			}
		case status != StatusError:
			d.fail(errors.New("transfer ended with unfinished chunks"))
		}
		d.notify(true)
		return
	}
	d.fail(errConflictGaveUp)
	d.notify(true)
}

func (d *Download) prepare(ctx context.Context) error {
	d.mu.Lock()
	needProbe := len(d.Chunks) == 0
	rawURL, headers := d.URL, d.Headers
	d.mu.Unlock()

	if needProbe {
		p, err := d.inspect(ctx, rawURL, headers)
		if err != nil {
			return err
		}
		d.applyProbe(p)
	} else if err := d.revalidate(ctx); err != nil {
		return err
	}
	return d.openTarget(needProbe)
}

// applyProbe stores a first probe and builds the chunk plan. The caller
// does not hold d.mu.
func (d *Download) applyProbe(p *Probe) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.FinalURL = p.FinalURL
	if d.FileName == "" {
		d.FileName = p.FileName
	}
	d.Total = p.Total
	d.RangeOK = p.RangeOK
	d.ETag = p.ETag
	d.LastModified = p.LastModified
	d.cookies = p.Cookies
	if d.Connections <= 0 {
		d.Connections = DefaultConnections
	}
	if d.SpeedLimit != 0 && d.limiter == nil {
		d.limiter = NewLimiter(d.SpeedLimit)
	}
	d.planLocked(p)
}

// revalidate runs before a resume. When the saved ETag, Last-Modified or
// size no longer matches, the partial file is discarded and planned again.
func (d *Download) revalidate(ctx context.Context) error {
	d.mu.Lock()
	rawURL, headers := d.URL, d.Headers
	etag, modified, total, ranged := d.ETag, d.LastModified, d.Total, d.RangeOK
	done := d.doneLocked()
	d.mu.Unlock()

	p, err := d.inspect(ctx, rawURL, headers)
	if err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.cookies = p.Cookies
	if p.FinalURL != "" {
		d.FinalURL = p.FinalURL
	}
	if !representationDiffers(etag, modified, total, ranged, done, p) {
		if p.ETag != "" {
			d.ETag = p.ETag
		}
		if p.LastModified != "" {
			d.LastModified = p.LastModified
		}
		return nil
	}
	d.ETag = p.ETag
	d.LastModified = p.LastModified
	d.Total = p.Total
	d.RangeOK = p.RangeOK
	d.planLocked(p)
	d.discardPart = true
	return nil
}

// planLocked replaces the chunk list from a probe. The caller holds d.mu.
func (d *Download) planLocked(p *Probe) {
	conns := d.Connections
	if conns < 1 {
		conns = 1
	}
	if p.RangeOK && p.Total > 0 {
		d.Chunks = NewPlan(p.Total, conns)
		return
	}
	// One stream; with the size unknown the chunk is open-ended and gets its
	// end when the response does.
	end := p.Total - 1
	if p.Total <= 0 {
		end = -1
	}
	d.Chunks = []*Chunk{{ID: 0, Start: 0, End: end}}
}

// representationDiffers reports whether p is a different file from the one
// already partly stored. A missing saved validator does not count, so state
// written before validators were kept still resumes. A different size, or a
// ranged download whose server no longer accepts ranges, does.
func representationDiffers(etag, modified string, total int64, ranged bool, done int64, p *Probe) bool {
	if total > 0 && p.Total > 0 && total != p.Total {
		return true
	}
	if etag != "" && p.ETag != "" && etag != p.ETag {
		return true
	}
	if modified != "" && p.LastModified != "" && modified != p.LastModified {
		return true
	}
	if ranged && !p.RangeOK && done > 0 {
		return true
	}
	return false
}

// openTarget opens the .odmpart file. A fresh download (first probe) claims
// a name nobody uses yet, because finishing used to delete whatever file
// already had the name, and two downloads with one name shared an .odmpart.
// A resumed download keeps the name it claimed.
func (d *Download) openTarget(fresh bool) error {
	d.mu.Lock()
	if d.FileName == "" {
		d.FileName = "download.bin"
	}
	discard := d.discardPart
	d.discardPart = false
	existing := d.file
	dir := d.BaseDir
	if dir == "" {
		dir = d.Dir
	}
	if d.Categories {
		cat := d.category()
		if custom := d.catDirs[cat]; strings.TrimSpace(custom) != "" {
			dir = custom
		} else {
			dir = filepath.Join(dir, cat)
		}
	}
	d.Dir = dir
	final := filepath.Join(dir, d.FileName)
	tmp := final + ".odmpart"
	// A stream assembles its file at the end; pre-sizing it would only
	// double the disk space used while the segments are on disk too.
	rangeOK, total := d.RangeOK && d.Stream == nil, d.Total
	d.mu.Unlock()

	if existing != nil && !fresh {
		if discard {
			if err := existing.Truncate(0); err != nil {
				return err
			}
		}
		if rangeOK && total > 0 {
			return existing.Truncate(total)
		}
		return nil
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	var f *os.File
	var err error
	if fresh {
		var name string
		name, f, err = claimName(dir, filepath.Base(final))
		if err != nil {
			return err
		}
		d.mu.Lock()
		d.FileName = name
		d.mu.Unlock()
	} else {
		f, err = os.OpenFile(tmp, os.O_CREATE|os.O_RDWR, 0o644)
		if err != nil {
			return err
		}
	}
	if discard {
		if err := f.Truncate(0); err != nil {
			_ = f.Close()
			return err
		}
	}
	if rangeOK && total > 0 {
		if err := f.Truncate(total); err != nil {
			_ = f.Close()
			return err
		}
	}
	d.mu.Lock()
	d.file = f
	d.mu.Unlock()
	return nil
}

func (d *Download) worker(ctx context.Context) {
	ready := false
	for {
		if ctx.Err() != nil {
			return
		}
		ch := d.takeChunk()
		if ch == nil {
			ch = d.trySplit()
		}
		if ch == nil {
			return
		}
		err := d.fetchWithRetry(ctx, ch, &ready)
		if errors.Is(err, errRepresentationChanged) {
			d.noteChanged()
			d.mu.Lock()
			ch.claimed = false
			d.mu.Unlock()
			return
		}
		if errors.Is(err, errIgnoredRange) {
			d.mu.Lock()
			ch.claimed = false
			d.mu.Unlock()
			d.noteIgnoredRange()
			return
		}
		d.mu.Lock()
		ch.claimed = false
		d.mu.Unlock()
		if err == nil {
			continue
		}
		var conflict *conflictError
		if errors.As(err, &conflict) {
			d.noteConflict(conflict.at)
			return
		}
		// Pause cancels the parent context. A disagreement cancels only this
		// pass. Neither one is a failed download.
		if ctx.Err() != nil || errors.Is(err, context.Canceled) {
			return
		}
		d.mu.Lock()
		d.Err = err.Error()
		d.Status = StatusError
		d.mu.Unlock()
		return
	}
}

func (d *Download) takeChunk() *Chunk {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, c := range d.Chunks {
		if !c.Complete() && !c.claimed {
			c.claimed = true
			return c
		}
	}
	return nil
}

func (d *Download) trySplit() *Chunk {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.alone {
		return nil
	}
	nc, ok := Split(d.Chunks, MinChunkSize)
	if !ok {
		return nil
	}
	nc.claimed = true
	d.Chunks = append(d.Chunks, nc)
	return nc
}

func (d *Download) fetchWithRetry(ctx context.Context, ch *Chunk, ready *bool) error {
	var err error
	backoff := d.retryDelay()
	for attempt := 0; attempt < MaxRetries; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		err = d.fetch(ctx, ch, ready)
		if err == nil || errors.Is(err, errIgnoredRange) || errors.Is(err, errRepresentationChanged) || errors.Is(err, errConflict) {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if serr := sleepCtx(ctx, retryWait(err, backoff)); serr != nil {
			return serr
		}
		if backoff < 8*time.Second {
			backoff *= 2
		}
	}
	return err
}

func (d *Download) fetch(ctx context.Context, ch *Chunk, ready *bool) error {
	d.mu.Lock()
	if ch.Complete() {
		d.mu.Unlock()
		return nil
	}
	if !d.RangeOK && ch.Done > 0 {
		// Without ranges the server sends the file from its first byte, so a
		// retry or a resume cannot carry on where the last attempt stopped.
		// Count from zero again; the same bytes go to the same offsets.
		ch.Done = 0
	}
	start, end := ch.Next(), ch.End
	rangeOK := d.RangeOK
	conns := d.Connections
	finalURL := d.FinalURL
	headers := d.Headers
	cookies := d.cookies
	etag, modified := d.ETag, d.LastModified
	total := d.Total
	reqStart, overlap := d.overlapLocked(ch, start, end, rangeOK)
	alone := d.alone
	pass := d.pass
	d.mu.Unlock()
	client := d.segmentClient(rangeOK, conns)
	validator := etag
	if validator == "" {
		validator = modified
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, finalURL, nil)
	if err != nil {
		return err
	}
	applyHeaders(req, headers)
	if rangeOK {
		// The transport would otherwise ask for gzip. A compressed body is
		// not the file's own bytes and must not be written at this offset.
		// A value the download already set is kept.
		if req.Header.Get("Accept-Encoding") == "" {
			req.Header.Set("Accept-Encoding", "identity")
		}
		if validator != "" {
			req.Header.Set("If-Range", validator)
		}
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", reqStart, end))
	}
	if len(cookies) > 0 {
		req.Header.Set("Cookie", cookieHeader(cookies))
	}

	resp, err := d.authorizer().do(req, func(r *http.Request) (*http.Response, error) {
		return d.exchange(ctx, client, r)
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := busyResponse(resp, time.Now()); err != nil {
		return err
	}

	if rangeOK {
		switch resp.StatusCode {
		case http.StatusPartialContent:
			if cs, _, _, ok := ParseContentRange(resp.Header.Get("Content-Range")); ok && cs != reqStart {
				return fmt.Errorf("range mismatch: asked %d, got %d", reqStart, cs)
			}
		case http.StatusOK:
			// A 200 is the whole file. A shorter length is not that file.
			if resp.ContentLength >= 0 && total > 0 && resp.ContentLength != total {
				return fmt.Errorf("server ignored Range (status %s)", resp.Status)
			}
			// A different validator means the file itself changed. A 200
			// for the whole file is kept only when the validator still matches.
			if validator != "" && wholeFileRange(start, end, total) && !wholeFileOK(start, end, total, etag, modified, resp) {
				return errRepresentationChanged
			}
			if validatorMismatch(etag, modified, resp) {
				return errRepresentationChanged
			}
			if !wholeFileRange(start, end, total) {
				if !alone {
					return errIgnoredRange
				}
				// This connection is already the only one. Skip the prefix
				// that is already stored and write the rest at its offset.
				if err := discardPrefix(ctx, resp.Body, start, d.readStall()); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("server ignored Range (status %s)", resp.Status)
		}
	} else if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("unexpected status: %s", resp.Status)
	}

	if overlap > 0 && resp.StatusCode == http.StatusPartialContent {
		sample := make([]byte, overlap)
		gotN := 0
		for gotN < overlap {
			n, rerr := readWithIdle(ctx, resp.Body, sample[gotN:], d.readStall())
			gotN += n
			if gotN >= overlap {
				break
			}
			if rerr != nil {
				return rerr
			}
		}
		// These bytes belong to the previous chunk. Compare them, or hold
		// them until that chunk writes the same offset. Do not store them
		// again from this connection.
		if err := d.checkOverlap(reqStart, sample); err != nil {
			return err
		}
	}

	buf := make([]byte, 64*1024)
	pos := start
	early := false
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		n, rerr := readWithIdle(ctx, resp.Body, buf, d.readStall())
		if n > 0 {
			if d.beforeWrite != nil {
				d.beforeWrite()
			}
			// The end can move while this buffer is in hand: another
			// connection may have taken the tail. Count and write only the
			// bytes that still belong to this chunk. A copy another
			// connection already read for this offset is compared before
			// and after the write.
			d.mu.Lock()
			if d.pass != pass {
				d.mu.Unlock()
				return errIgnoredRange
			}
			limit := ch.End
			writeN := ownedSpan(pos, n, limit)
			var mismatchAt int64
			bad := false
			if writeN > 0 {
				if at, mis := d.mismatchLocked(pos, buf[:writeN]); mis {
					mismatchAt, bad = at, true
				}
			}
			d.mu.Unlock()
			if bad {
				return &conflictError{at: mismatchAt}
			}
			if writeN > 0 {
				if _, werr := d.file.WriteAt(buf[:writeN], pos); werr != nil {
					return werr
				}
				d.mu.Lock()
				counted := ownedSpan(pos, writeN, ch.End)
				if counted > 0 {
					if at, mis := d.mismatchLocked(pos, buf[:counted]); mis {
						d.mu.Unlock()
						return &conflictError{at: at}
					}
					d.consumeWitnessLocked(pos, buf[:counted])
					ch.Done += int64(counted)
					ch.notePace(int64(counted), time.Now())
				}
				limit = ch.End
				d.mu.Unlock()
				pos += int64(counted)
				d.notify(false)
				if counted > 0 {
					if ready != nil && !*ready {
						*ready = true
						d.noteReady(ctx)
					}
					if werr := d.limiter.Wait(ctx, counted); werr != nil {
						return werr
					}
				}
			}
			if limit >= 0 && pos > limit {
				break
			}
		}
		if rerr != nil {
			if errors.Is(rerr, io.EOF) {
				early = true
				break
			}
			return rerr
		}
	}
	if !early {
		return nil
	}
	d.mu.Lock()
	if ch.End < 0 {
		// Size unknown: the response ending is the end of the file. A body
		// cut off by a dropped connection fails with an error before this.
		ch.End = ch.Start + ch.Done - 1
		d.Total = ch.Start + ch.Done
	}
	complete := ch.Complete()
	d.mu.Unlock()
	if !complete {
		return errShortBody
	}
	return nil
}

// segmentClient returns the HTTP/1.1 client when more than one ranged
// connection will run. One stream keeps the shared client.
func (d *Download) segmentClient(rangeOK bool, conns int) *http.Client {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.client == nil || !rangeOK || conns <= 1 {
		return d.client
	}
	if d.h1 != nil {
		return d.h1
	}
	var base *http.Transport
	if d.client.Transport != nil {
		tr, ok := d.client.Transport.(*http.Transport)
		if !ok {
			return d.client
		}
		base = tr
	}
	d.h1 = &http.Client{
		Transport:     segmentTransport(base),
		CheckRedirect: d.client.CheckRedirect,
		Jar:           d.client.Jar,
		Timeout:       d.client.Timeout,
	}
	return d.h1
}

// errShortBody means the response ended before every byte of the chunk was
// written. The chunk is retried from the last byte written.
var errShortBody = errors.New("connection closed early")

func (d *Download) retryDelay() time.Duration {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.retryBase > 0 {
		return d.retryBase
	}
	return 300 * time.Millisecond
}

// ownedSpan is how many of the n bytes at pos still fall inside a chunk
// whose inclusive end is limit. A negative limit means the end is open.
func ownedSpan(pos int64, n int, limit int64) int {
	if n <= 0 {
		return 0
	}
	if limit < 0 || pos+int64(n) <= limit+1 {
		return n
	}
	remain := limit + 1 - pos
	if remain <= 0 {
		return 0
	}
	return int(remain)
}

// errStalled means a connection produced no bytes within the idle limit,
// while waiting for response headers or while reading the body. The chunk
// is retried from the last byte written. A pause is not a stall: the caller
// returns the context error before counting a retry.
var errStalled = errors.New("connection stalled")

// exchange sends req and returns when the response headers arrive. The stall
// limit bounds that wait, then it is stopped so a long body is not cut short.
// Pausing cancels ctx and is reported as ctx.Err(). client is the connection
// that should send req, so a segmented download keeps its HTTP/1.1 transport.
func (d *Download) exchange(ctx context.Context, client *http.Client, req *http.Request) (*http.Response, error) {
	headerCtx, cancel := context.WithCancel(ctx)
	timer := time.AfterFunc(d.readStall(), cancel)
	resp, err := client.Do(req.WithContext(headerCtx))
	if timer.Stop() {
		if err != nil {
			cancel()
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, err
		}
		resp.Body = &cancelOnClose{ReadCloser: resp.Body, cancel: cancel}
		return resp, nil
	}
	cancel()
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return nil, errStalled
}

// cancelOnClose releases the header context once the body is finished.
type cancelOnClose struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *cancelOnClose) Close() error {
	err := b.ReadCloser.Close()
	b.cancel()
	return err
}

func (d *Download) readStall() time.Duration {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.stallTimeout > 0 {
		return d.stallTimeout
	}
	return DefaultStallTimeout
}

// readWithIdle returns the next read, or errStalled when idle elapses with
// no bytes. Closing the body unblocks a Read left behind in that case.
func readWithIdle(ctx context.Context, body io.ReadCloser, buf []byte, idle time.Duration) (int, error) {
	type result struct {
		n   int
		err error
	}
	done := make(chan result, 1)
	go func() {
		n, err := body.Read(buf)
		done <- result{n, err}
	}()
	timer := time.NewTimer(idle)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		_ = body.Close()
		return 0, ctx.Err()
	case <-timer.C:
		_ = body.Close()
		return 0, errStalled
	case res := <-done:
		return res.n, res.err
	}
}

// noteReady records that one open connection has stored file bytes. When
// every connection already opened has done that, one more connection starts,
// up to the configured limit.
func (d *Download) noteReady(ctx context.Context) {
	d.mu.Lock()
	d.connReady++
	if d.connWg == nil || d.Status == StatusError || d.connOpened >= d.connLimit || d.connReady < d.connOpened {
		d.mu.Unlock()
		return
	}
	d.connOpened++
	wg := d.connWg
	d.mu.Unlock()
	wg.Add(1)
	go func() {
		defer wg.Done()
		d.worker(ctx)
	}()
}

// errRepresentationChanged means the server no longer serves the bytes this
// download has partly stored. The partial file is discarded and fetched again.
var errRepresentationChanged = errors.New("file on the server changed")

// wholeFileOK reports whether a 200 response is the file this request already
// asked for in full. Some servers answer 200, not 206, when the range covers
// every byte and the validator still matches.
func wholeFileOK(start, end, total int64, etag, modified string, resp *http.Response) bool {
	if total <= 0 || start != 0 || end != total-1 {
		return false
	}
	switch {
	case etag != "":
		if resp.Header.Get("ETag") != etag {
			return false
		}
	case modified != "":
		if resp.Header.Get("Last-Modified") != modified {
			return false
		}
	default:
		return false
	}
	if n := resp.ContentLength; n >= 0 && n != total {
		return false
	}
	return true
}

func (d *Download) noteChanged() {
	d.mu.Lock()
	d.changed = true
	cancel := d.transferCancel
	d.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// validatorMismatch reports whether the response names a different
// representation than the one this download stored. A missing response
// validator does not count.
func validatorMismatch(etag, modified string, resp *http.Response) bool {
	if etag != "" {
		got := resp.Header.Get("ETag")
		return got != "" && got != etag
	}
	if modified != "" {
		got := resp.Header.Get("Last-Modified")
		return got != "" && got != modified
	}
	return false
}

// errIgnoredRange means the server answered a smaller range with the whole
// file. The body is not written at the range offset. Finished bytes stay,
// and the download continues on one connection.
var errIgnoredRange = errors.New("server returned the whole file for a range")

// wholeFileRange reports whether [start, end] already covers the whole file.
// end < 0 is an open-ended range.
func wholeFileRange(start, end, total int64) bool {
	if start != 0 {
		return false
	}
	if total <= 0 {
		return end < 0
	}
	return end < 0 || end >= total-1
}

// discardPrefix reads n bytes and drops them. A 200 body starts at byte 0,
// so the single connection skips the prefix already stored. Each read has
// the stall limit, as the prefix can be most of a large file.
func discardPrefix(ctx context.Context, body io.ReadCloser, n int64, idle time.Duration) error {
	buf := make([]byte, 64*1024)
	var dropped int64
	for dropped < n {
		if err := ctx.Err(); err != nil {
			return err
		}
		want := n - dropped
		if want > int64(len(buf)) {
			want = int64(len(buf))
		}
		nr, err := readWithIdle(ctx, body, buf[:want], idle)
		dropped += int64(nr)
		if err != nil {
			if errors.Is(err, io.EOF) && dropped >= n {
				return nil
			}
			if errors.Is(err, io.EOF) {
				return fmt.Errorf("whole-file response ended before byte %d", n)
			}
			return err
		}
	}
	return nil
}

// noteIgnoredRange records that a range was answered with the whole file
// and stops the other connections of this pass. The plan is rebuilt after
// they have stopped.
func (d *Download) noteIgnoredRange() {
	d.mu.Lock()
	if d.alone {
		d.mu.Unlock()
		return
	}
	d.alone = true
	d.sawIgnored = true
	d.pass++
	cancel := d.transferCancel
	d.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// overlapLocked decides whether this request should re-read the bytes just
// before start. The caller holds d.mu. A chunk's first request always does.
// A resumed request does only when those bytes are already stored, which is
// how a fallback re-checks the prefix it kept.
func (d *Download) overlapLocked(ch *Chunk, start, end int64, rangeOK bool) (reqStart int64, overlap int) {
	reqStart = start
	if !rangeOK || start <= 0 || end < start {
		return reqStart, 0
	}
	win := int64(boundaryOverlap)
	if win > start {
		win = start
	}
	if start != ch.Start && d.writtenPrefixLocked(start-win, start) != start {
		return reqStart, 0
	}
	return start - win, int(win)
}

// checkOverlap compares sample with bytes already stored at off. The part
// not stored yet is remembered and compared when the owning chunk writes it.
func (d *Download) checkOverlap(off int64, sample []byte) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	end := off + int64(len(sample))
	if have := d.writtenPrefixLocked(off, end); have > off {
		n := int(have - off)
		got := make([]byte, n)
		rn, err := d.file.ReadAt(got, off)
		if rn != n || (err != nil && !errors.Is(err, io.EOF)) {
			if err != nil {
				return err
			}
			return &conflictError{at: off}
		}
		if !bytes.Equal(got, sample[:n]) {
			return &conflictError{at: differAt(off, got, sample[:n])}
		}
		if have >= end {
			return nil
		}
		sample = sample[n:]
		off = have
	}
	if len(sample) > 0 {
		d.addWitnessLocked(off, sample)
	}
	return nil
}

// noteConflict records the earliest offset that disagreed and stops the
// other connections of this pass. The caller does not hold d.mu.
func (d *Download) noteConflict(at int64) {
	d.mu.Lock()
	if !d.conflicted || at < d.conflictAt {
		d.conflictAt = at
		d.conflicted = true
	}
	cancel := d.transferCancel
	d.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// adoptSingle keeps bytes before at and continues the download on one connection.
func (d *Download) adoptSingle(at int64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.single = true
	d.witnesses = nil
	d.rewindLocked(at)
	d.Status = StatusDownloading
	d.Err = ""
}

// restartFromZero drops the partial file and plans the same chunks again.
// The following pass still uses one connection.
func (d *Download) restartFromZero() error {
	d.mu.Lock()
	total := d.Total
	rangeOK := d.RangeOK
	conns := d.Connections
	f := d.file
	d.mu.Unlock()

	if f != nil {
		if err := f.Truncate(0); err != nil {
			return err
		}
		if rangeOK && total > 0 {
			if err := f.Truncate(total); err != nil {
				return err
			}
		}
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	d.restarted = true
	d.single = true
	d.witnesses = nil
	if rangeOK {
		d.Chunks = NewPlan(total, conns)
	} else if total > 0 {
		d.Chunks = []*Chunk{{ID: 0, Start: 0, End: total - 1}}
	} else {
		d.Chunks = []*Chunk{{ID: 0, Start: 0, End: -1}}
	}
	d.Status = StatusDownloading
	d.Err = ""
	return nil
}

// rewindLocked forgets progress at and after at. The caller holds d.mu.
func (d *Download) rewindLocked(at int64) {
	if at < 0 {
		at = 0
	}
	d.witnesses = nil
	for _, c := range d.Chunks {
		c.claimed = false
		if c.End >= 0 && c.End < at {
			continue
		}
		if c.Start >= at {
			c.Done = 0
			continue
		}
		if c.Start+c.Done > at {
			c.Done = at - c.Start
		}
	}
}

// writtenPrefixLocked returns the end of the contiguous stored run that
// starts at from, capped at to. The caller holds d.mu.
func (d *Download) writtenPrefixLocked(from, to int64) int64 {
	off := from
	for off < to {
		owner := d.chunkAtLocked(off)
		if owner == nil {
			break
		}
		have := owner.Start + owner.Done
		if owner.End >= 0 && have > owner.End+1 {
			have = owner.End + 1
		}
		if off >= have {
			break
		}
		if have >= to {
			return to
		}
		off = have
	}
	return off
}

func (d *Download) chunkAtLocked(off int64) *Chunk {
	for _, c := range d.Chunks {
		if c.End < 0 {
			if off >= c.Start {
				return c
			}
			continue
		}
		if off >= c.Start && off <= c.End {
			return c
		}
	}
	return nil
}

func (d *Download) addWitnessLocked(off int64, sample []byte) {
	end := off + int64(len(sample))
	kept := d.witnesses[:0]
	for _, w := range d.witnesses {
		wEnd := w.off + int64(len(w.data))
		if wEnd <= off || w.off >= end {
			kept = append(kept, w)
		}
	}
	d.witnesses = append(kept, witness{off: off, data: append([]byte(nil), sample...)})
}

// mismatchLocked reports the absolute offset of the first byte of buf that
// differs from a witness. The caller holds d.mu.
func (d *Download) mismatchLocked(pos int64, buf []byte) (int64, bool) {
	end := pos + int64(len(buf))
	for _, w := range d.witnesses {
		wEnd := w.off + int64(len(w.data))
		if wEnd <= pos || w.off >= end {
			continue
		}
		from := pos
		if w.off > from {
			from = w.off
		}
		to := end
		if wEnd < to {
			to = wEnd
		}
		for i := from; i < to; i++ {
			if buf[i-pos] != w.data[i-w.off] {
				return i, true
			}
		}
	}
	return 0, false
}

// consumeWitnessLocked drops the parts of every witness that buf just stored.
// The caller holds d.mu.
func (d *Download) consumeWitnessLocked(pos int64, buf []byte) {
	end := pos + int64(len(buf))
	kept := d.witnesses[:0]
	for _, w := range d.witnesses {
		wEnd := w.off + int64(len(w.data))
		if wEnd <= pos || w.off >= end {
			kept = append(kept, w)
			continue
		}
		if w.off < pos {
			kept = append(kept, witness{off: w.off, data: append([]byte(nil), w.data[:pos-w.off]...)})
		}
		if wEnd > end {
			kept = append(kept, witness{off: end, data: append([]byte(nil), w.data[end-w.off:]...)})
		}
	}
	d.witnesses = kept
}

func differAt(off int64, got, want []byte) int64 {
	n := len(got)
	if len(want) < n {
		n = len(want)
	}
	for i := 0; i < n; i++ {
		if got[i] != want[i] {
			return off + int64(i)
		}
	}
	return off
}

// planAlone keeps the contiguous prefix and leaves one chunk for the rest.
// Every connection has stopped, so chunk progress is stable.
func (d *Download) planAlone() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.Total <= 0 {
		return
	}
	gap := contiguousDone(d.Chunks, d.Total)
	if gap > d.Total {
		gap = d.Total
	}
	d.Chunks = []*Chunk{{ID: 0, Start: 0, End: d.Total - 1, Done: gap}}
}

// contiguousDone is the first offset, starting at 0, that no chunk has stored.
func contiguousDone(chunks []*Chunk, total int64) int64 {
	off := int64(0)
	for {
		if total > 0 && off >= total {
			return total
		}
		next := off
		for _, c := range chunks {
			if c == nil || c.Start > off {
				continue
			}
			have := c.Start + c.Done
			if c.End >= 0 && have > c.End+1 {
				have = c.End + 1
			}
			if have > next {
				next = have
			}
		}
		if next == off {
			return off
		}
		off = next
	}
}

func (d *Download) finish() error {
	d.mu.Lock()
	f := d.file
	dir, name := d.Dir, d.FileName
	d.file = nil
	d.mu.Unlock()

	if f != nil {
		if err := f.Sync(); err != nil {
			_ = f.Close()
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
	}
	final := filepath.Join(dir, name)
	tmp := final + ".odmpart"
	if _, err := os.Stat(final); err == nil {
		// A file took this name while we downloaded. Keep it and finish
		// under the next free name; this used to delete it.
		alt, err := freeName(dir, name)
		if err != nil {
			return err
		}
		d.mu.Lock()
		d.FileName = alt
		d.mu.Unlock()
		final = filepath.Join(dir, alt)
	}
	return os.Rename(tmp, final)
}

// maxNameTries bounds the "name (n).ext" search.
const maxNameTries = 1000

// claimName finds the first free name starting from name and creates its
// .odmpart with O_EXCL, so a concurrent download cannot take the same one.
func claimName(dir, name string) (string, *os.File, error) {
	for i := 1; i <= maxNameTries; i++ {
		cand := numberedName(name, i)
		final := filepath.Join(dir, cand)
		if _, err := os.Stat(final); err == nil {
			continue
		}
		f, err := os.OpenFile(final+".odmpart", os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o644)
		if os.IsExist(err) {
			continue
		}
		if err != nil {
			return "", nil, err
		}
		return cand, f, nil
	}
	return "", nil, fmt.Errorf("no free file name for %q in %s", name, dir)
}

// freeName is the first numbered variant of name with neither a file nor
// an .odmpart in dir.
func freeName(dir, name string) (string, error) {
	for i := 2; i <= maxNameTries; i++ {
		cand := numberedName(name, i)
		final := filepath.Join(dir, cand)
		_, e1 := os.Stat(final)
		_, e2 := os.Stat(final + ".odmpart")
		if os.IsNotExist(e1) && os.IsNotExist(e2) {
			return cand, nil
		}
	}
	return "", fmt.Errorf("no free file name for %q in %s", name, dir)
}

// numberedName returns name for n == 1, else "base (n).ext", keeping
// compound archive extensions whole ("x (2).tar.gz", not "x.tar (2).gz").
func numberedName(name string, n int) string {
	if n <= 1 {
		return name
	}
	ext := filepath.Ext(name)
	lower := strings.ToLower(name)
	for _, c := range []string{".tar.gz", ".tar.bz2", ".tar.xz", ".tar.zst"} {
		if strings.HasSuffix(lower, c) {
			ext = name[len(name)-len(c):]
			break
		}
	}
	return fmt.Sprintf("%s (%d)%s", strings.TrimSuffix(name, ext), n, ext)
}

// Pause stops the running transfer, preserving progress.
func (d *Download) Pause() { d.stop(nil) }

// abandon stops the download for removal. A live recording, which a Pause
// finishes into a file, then assembles nothing (see recordLive).
//
// It also cancels the run's abortCtx, under which a recording that was
// already stopped may still be assembling: a context cause cannot do that,
// as the first cause (the Stop) is the one kept.
func (d *Download) abandon() {
	d.mu.Lock()
	d.removed = true
	abort := d.abort
	d.mu.Unlock()
	if abort != nil {
		abort()
	}
	d.stop(errRemoved)
}

// isRemoved reports whether abandon was called.
func (d *Download) isRemoved() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.removed
}

// assemblyContext is the context a recording assembles under after it was
// stopped: cancelled only by removal.
func (d *Download) assemblyContext() context.Context {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.abortCtx == nil {
		return context.Background()
	}
	return d.abortCtx
}

func (d *Download) stop(cause error) {
	d.mu.Lock()
	cancel := d.cancel
	if cancel == nil {
		d.pauseReq = true
	}
	d.mu.Unlock()
	if cancel != nil {
		cancel(cause)
	}
}

// waitIdle waits up to timeout for a running Start to return, so its
// files are closed before someone deletes them (Windows refuses to delete
// open files).
func (d *Download) waitIdle(timeout time.Duration) {
	d.mu.Lock()
	done := d.runDone
	d.mu.Unlock()
	if done == nil {
		return
	}
	select {
	case <-done:
	case <-time.After(timeout):
	}
}

// Sample recomputes the transfer speed; call roughly once per second.
func (d *Download) Sample() {
	d.mu.Lock()
	defer d.mu.Unlock()
	now := time.Now()
	done := d.doneLocked()
	if !d.lastAt.IsZero() {
		dt := now.Sub(d.lastAt).Seconds()
		if dt > 0 {
			d.speed = int64(float64(done-d.lastDone) / dt)
			if d.speed < 0 { // a stream drops a failed segment's bytes
				d.speed = 0
			}
		}
	}
	d.lastDone, d.lastAt = done, now
}

// State serialises the durable parts of a download.
type State struct {
	ID           string            `json:"id"`
	URL          string            `json:"url"`
	FinalURL     string            `json:"final_url"`
	FileName     string            `json:"file_name"`
	Dir          string            `json:"dir"`
	BaseDir      string            `json:"base_dir,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
	Connections  int               `json:"connections"`
	SpeedLimit   int64             `json:"speed_limit,omitempty"`
	OpenWhenDone bool              `json:"open_when_done,omitempty"`
	Total        int64             `json:"total"`
	RangeOK      bool              `json:"range_ok"`
	ETag         string            `json:"etag,omitempty"`
	LastModified string            `json:"last_modified,omitempty"`
	Single       bool              `json:"single,omitempty"`
	Alone        bool              `json:"alone,omitempty"`
	CategoryName string            `json:"category_name,omitempty"`
	Queue        string            `json:"queue,omitempty"`
	Description  string            `json:"description,omitempty"`
	Order        int               `json:"order,omitempty"`
	Status       Status            `json:"status"`
	Err          string            `json:"error,omitempty"`
	Chunks       []*Chunk          `json:"chunks"`
	Stream       *StreamJob        `json:"stream,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// State snapshots the download for persistence.
func (d *Download) State() State {
	d.mu.Lock()
	defer d.mu.Unlock()
	chunks := make([]*Chunk, len(d.Chunks))
	for i, c := range d.Chunks {
		cc := *c
		cc.claimed = false
		chunks[i] = &cc
	}
	var stream *StreamJob
	if d.Stream != nil {
		sj := *d.Stream
		sj.Counts = append([]int(nil), d.Stream.Counts...)
		sj.Done = d.streamDone
		stream = &sj
	}
	status := d.Status
	if status == StatusDownloading {
		status = StatusPaused
	}
	return State{
		ID: d.ID, URL: d.URL, FinalURL: d.FinalURL, FileName: d.FileName, Dir: d.Dir,
		BaseDir: d.BaseDir, Headers: d.Headers, Connections: d.Connections, SpeedLimit: d.SpeedLimit,
		OpenWhenDone: d.OpenWhenDone, CategoryName: d.CategoryName,
		Queue: d.Queue, Description: d.Description, Order: d.Order,
		Total: d.Total, RangeOK: d.RangeOK, ETag: d.ETag, LastModified: d.LastModified,
		Single: d.single, Alone: d.alone, Status: status, Err: d.Err,
		Chunks: chunks, Stream: stream, CreatedAt: d.CreatedAt, UpdatedAt: time.Now(),
	}
}

// Restore rebuilds a download from persisted state.
func Restore(s State, client *http.Client) *Download {
	if s.Connections <= 0 {
		s.Connections = DefaultConnections
	}
	d := New(s.ID, s.URL, s.Dir, s.Connections, s.Headers, s.SpeedLimit, client)
	d.mu.Lock()
	d.FinalURL = s.FinalURL
	d.FileName = s.FileName
	if s.BaseDir != "" {
		d.BaseDir = s.BaseDir
	}
	d.Dir = s.Dir
	d.Total = s.Total
	d.RangeOK = s.RangeOK
	d.ETag = s.ETag
	d.LastModified = s.LastModified
	d.single = s.Single
	d.alone = s.Alone
	d.OpenWhenDone = s.OpenWhenDone
	d.CategoryName = s.CategoryName
	d.Queue = s.Queue
	d.Description = s.Description
	d.Order = s.Order
	d.Chunks = s.Chunks
	d.Stream = s.Stream
	if s.Stream != nil {
		d.streamDone = s.Stream.Done
		if s.Status == StatusComplete {
			d.streamDone = s.Total
		}
	}
	d.Err = s.Err
	d.CreatedAt = s.CreatedAt
	if !s.UpdatedAt.IsZero() {
		d.UpdatedAt = s.UpdatedAt // shown as "Last attempt"; New set it to now
	}
	switch s.Status {
	case StatusComplete:
		d.Status = StatusComplete
	case StatusError:
		d.Status = StatusError
	default:
		d.Status = StatusPaused
	}
	for _, c := range d.Chunks {
		c.claimed = false
	}
	d.mu.Unlock()
	return d
}

// Info projects the download for the API/UI.
func (d *Download) Info() Info {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.infoLocked()
}

// category is the download's category: the one chosen for it while that
// still exists, else the one its file name falls under. The caller holds
// d.mu.
func (d *Download) category() string {
	if d.CategoryName != "" && knownCategory(d.CategoryName) {
		return d.CategoryName
	}
	return Category(d.FileName)
}

func (d *Download) infoLocked() Info {
	done := d.doneLocked()
	var pct float64
	if d.Total > 0 {
		pct = float64(done) / float64(d.Total) * 100
	}
	chunks := make([]ChunkInfo, 0, len(d.Chunks))
	active := 0
	for _, c := range d.Chunks {
		state := "Waiting"
		switch {
		case c.Complete():
			state = "Complete"
		case c.claimed:
			state = "Receiving…"
			active++
		}
		chunks = append(chunks, ChunkInfo{
			ID: c.ID, Start: c.Start, End: c.End, Done: c.Done,
			Remaining: c.Remaining(), Complete: c.Complete(), State: state,
		})
	}
	path := ""
	if d.FileName != "" {
		path = filepath.Join(d.Dir, d.FileName)
		if d.Status != StatusComplete {
			path += ".odmpart"
		}
	}
	stream := ""
	if d.Stream != nil {
		stream = string(d.Stream.Kind)
	}
	return Info{
		ID: d.ID, URL: d.URL, FinalURL: d.FinalURL, FileName: d.FileName, FilePath: path,
		Dir: d.Dir, Status: d.Status, Error: d.Err, Total: d.Total, Done: done,
		Progress: pct, Speed: d.speed, Connections: d.Connections, Active: active,
		RangeOK: d.RangeOK, Category: d.category(), Queue: d.Queue, Description: d.Description, Order: d.Order, Chunks: chunks,
		SpeedLimit: d.SpeedLimit, OpenWhenDone: d.OpenWhenDone,
		Stream: stream, Segments: d.streamSegs, SegmentsDone: d.streamHave,
		Live:      d.Stream != nil && d.Stream.Live,
		CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}
}

// doneBytes is the number of bytes received so far.
func (d *Download) doneBytes() int64 {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.doneLocked()
}

func (d *Download) doneLocked() int64 {
	done := d.streamDone
	for _, c := range d.Chunks {
		done += c.Done
	}
	return done
}

func (d *Download) allCompleteLocked() bool {
	if len(d.Chunks) == 0 {
		return false
	}
	for _, c := range d.Chunks {
		if !c.Complete() {
			return false
		}
	}
	return true
}

func (d *Download) fail(err error) {
	d.mu.Lock()
	d.Status = StatusError
	d.Err = err.Error()
	d.mu.Unlock()
}

func (d *Download) notify(force bool) {
	d.mu.Lock()
	d.UpdatedAt = time.Now()
	d.mu.Unlock()
	if d.onUpdate == nil {
		return
	}
	d.mu.Lock()
	if !force && time.Since(d.notifyAt) < 250*time.Millisecond {
		d.mu.Unlock()
		return
	}
	d.notifyAt = time.Now()
	d.mu.Unlock()
	d.onUpdate(d)
}

func cookieHeader(cookies []*http.Cookie) string {
	parts := make([]string, 0, len(cookies))
	for _, c := range cookies {
		parts = append(parts, c.Name+"="+c.Value)
	}
	return strings.Join(parts, "; ")
}
