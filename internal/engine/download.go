package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

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

	CreatedAt time.Time
	UpdatedAt time.Time

	client *http.Client
	// h1 serves several ranges at once over HTTP/1.1. The shared client may
	// use HTTP/2, which would put every range on one socket.
	h1      *http.Client
	limiter *Limiter
	shared  *Limiter
	cancel  context.CancelFunc
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

	lastDone int64
	lastAt   time.Time
	speed    int64
	notifyAt time.Time

	// stallTimeout overrides DefaultStallTimeout when set. Tests use a
	// short value; a zero value means the default.
	stallTimeout time.Duration
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

// SetAuthLookup installs the per-host credential lookup used for Basic auth.
func (d *Download) SetAuthLookup(fn func(host string) (string, string, bool)) {
	d.mu.Lock()
	d.auth = fn
	d.mu.Unlock()
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

// Start runs the transfer to completion, pause or error. It blocks.
func (d *Download) Start(parent context.Context) {
	ctx, cancel := context.WithCancel(parent)
	defer func() {
		cancel()
		d.mu.Lock()
		// Drop the spent cancel func: a Pause between this run and the next
		// must set pauseReq, not cancel a context that is already gone.
		d.cancel = nil
		d.pauseReq = false
		d.mu.Unlock()
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
	d.mu.Unlock()
	d.notify(true)

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

	limit := d.Connections
	if !d.RangeOK || limit < 1 {
		limit = 1
	}
	var wg sync.WaitGroup
	d.mu.Lock()
	d.connLimit = limit
	d.connOpened = 1
	d.connReady = 0
	d.connWg = &wg
	d.mu.Unlock()
	wg.Add(1)
	go func() { defer wg.Done(); d.worker(ctx) }()
	wg.Wait()
	d.mu.Lock()
	d.connWg = nil
	d.mu.Unlock()

	d.mu.Lock()
	paused := ctx.Err() != nil
	complete := d.allCompleteLocked()
	status := d.Status
	d.mu.Unlock()

	switch {
	case complete:
		if err := d.finish(); err != nil {
			d.fail(err)
		} else {
			d.mu.Lock()
			d.Status = StatusComplete
			d.mu.Unlock()
		}
	case paused:
		d.mu.Lock()
		d.Status = StatusPaused
		d.mu.Unlock()
	case status != StatusError:
		d.fail(errors.New("transfer ended with unfinished chunks"))
	}
	d.notify(true)
}

func (d *Download) prepare(ctx context.Context) error {
	d.mu.Lock()
	needProbe := len(d.Chunks) == 0
	rawURL, headers := d.URL, d.Headers
	d.mu.Unlock()

	if needProbe {
		p, err := Inspect(ctx, d.client, rawURL, headers)
		if err != nil {
			return err
		}
		d.mu.Lock()
		d.FinalURL = p.FinalURL
		if d.FileName == "" {
			d.FileName = p.FileName
		}
		d.Total = p.Total
		d.RangeOK = p.RangeOK
		if d.Connections <= 0 {
			d.Connections = DefaultConnections
		}
		if d.SpeedLimit != 0 && d.limiter == nil {
			d.limiter = NewLimiter(d.SpeedLimit)
		}
		d.cookies = p.Cookies
		if p.RangeOK {
			d.Chunks = NewPlan(p.Total, d.Connections)
		} else {
			d.Chunks = []*Chunk{{ID: 0, Start: 0, End: p.Total - 1}}
		}
		d.mu.Unlock()
	}
	return d.openTarget(needProbe)
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
	dir := d.BaseDir
	if dir == "" {
		dir = d.Dir
	}
	if d.Categories {
		cat := d.CategoryName
		if cat == "" {
			cat = Category(d.FileName)
		}
		if custom := d.catDirs[cat]; strings.TrimSpace(custom) != "" {
			dir = custom
		} else {
			dir = filepath.Join(dir, cat)
		}
	}
	d.Dir = dir
	final := filepath.Join(dir, d.FileName)
	tmp := final + ".odmpart"
	rangeOK, total := d.RangeOK, d.Total
	d.mu.Unlock()

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
		d.mu.Lock()
		ch.claimed = false
		if err != nil {
			d.Err = err.Error()
			d.Status = StatusError
		}
		d.mu.Unlock()
		if err != nil {
			return
		}
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
		if err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
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
	start, end := ch.Next(), ch.End
	rangeOK := d.RangeOK
	conns := d.Connections
	finalURL := d.FinalURL
	headers := d.Headers
	cookies := d.cookies
	d.mu.Unlock()
	client := d.segmentClient(rangeOK, conns)

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
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	}
	if d.auth != nil {
		if u, perr := url.Parse(finalURL); perr == nil {
			if user, pass, ok := d.auth(u.Hostname()); ok {
				req.SetBasicAuth(user, pass)
			}
		}
	}
	if len(cookies) > 0 {
		req.Header.Set("Cookie", cookieHeader(cookies))
	}

	resp, err := d.exchange(ctx, client, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if rangeOK {
		if resp.StatusCode != http.StatusPartialContent {
			return fmt.Errorf("server ignored Range (status %s)", resp.Status)
		}
		if cs, _, _, ok := ParseContentRange(resp.Header.Get("Content-Range")); ok && cs != start {
			return fmt.Errorf("range mismatch: asked %d, got %d", start, cs)
		}
	} else if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("unexpected status: %s", resp.Status)
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
			// bytes that still belong to this chunk.
			d.mu.Lock()
			limit := ch.End
			d.mu.Unlock()
			writeN := ownedSpan(pos, n, limit)
			if writeN > 0 {
				if _, werr := d.file.WriteAt(buf[:writeN], pos); werr != nil {
					return werr
				}
				d.mu.Lock()
				counted := ownedSpan(pos, writeN, ch.End)
				if counted > 0 {
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
	if d.client == nil || !rangeOK || conns <= 1 {
		return d.client
	}
	d.mu.Lock()
	defer d.mu.Unlock()
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
func (d *Download) Pause() {
	d.mu.Lock()
	cancel := d.cancel
	if cancel == nil {
		d.pauseReq = true
	}
	d.mu.Unlock()
	if cancel != nil {
		cancel()
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
	CategoryName string            `json:"category_name,omitempty"`
	Queue        string            `json:"queue,omitempty"`
	Description  string            `json:"description,omitempty"`
	Order        int               `json:"order,omitempty"`
	Status       Status            `json:"status"`
	Err          string            `json:"error,omitempty"`
	Chunks       []*Chunk          `json:"chunks"`
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
	status := d.Status
	if status == StatusDownloading {
		status = StatusPaused
	}
	return State{
		ID: d.ID, URL: d.URL, FinalURL: d.FinalURL, FileName: d.FileName, Dir: d.Dir,
		BaseDir: d.BaseDir, Headers: d.Headers, Connections: d.Connections, SpeedLimit: d.SpeedLimit,
		OpenWhenDone: d.OpenWhenDone, CategoryName: d.CategoryName,
		Queue: d.Queue, Description: d.Description, Order: d.Order,
		Total: d.Total, RangeOK: d.RangeOK, Status: status, Err: d.Err,
		Chunks: chunks, CreatedAt: d.CreatedAt, UpdatedAt: time.Now(),
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
	d.OpenWhenDone = s.OpenWhenDone
	d.CategoryName = s.CategoryName
	d.Queue = s.Queue
	d.Description = s.Description
	d.Order = s.Order
	d.Chunks = s.Chunks
	d.Err = s.Err
	d.CreatedAt = s.CreatedAt
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

func (d *Download) category() string {
	if d.CategoryName != "" {
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
	return Info{
		ID: d.ID, URL: d.URL, FinalURL: d.FinalURL, FileName: d.FileName, FilePath: path,
		Dir: d.Dir, Status: d.Status, Error: d.Err, Total: d.Total, Done: done,
		Progress: pct, Speed: d.speed, Connections: d.Connections, Active: active,
		RangeOK: d.RangeOK, Category: d.category(), Queue: d.Queue, Description: d.Description, Order: d.Order, Chunks: chunks,
		SpeedLimit: d.SpeedLimit, OpenWhenDone: d.OpenWhenDone,
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
	var done int64
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
