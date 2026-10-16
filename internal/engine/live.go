package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"odm/internal/media"
)

// Live HLS recording.
//
// A live playlist (no EXT-X-ENDLIST and not VOD) lists only the latest
// segments and grows while the broadcast runs. ODM records it: it reloads
// the playlist about once per target duration, fetches the segments it has
// not taken yet, and stops when the download is stopped (Pause), when the
// playlist ends (EXT-X-ENDLIST), or when nothing new arrives for a while.
// The pieces are then joined like those of a finished stream.
//
// Gotchas:
//   - Recording starts with the oldest segment the playlist still lists:
//     usually seconds to a minute before "now", the whole event so far on
//     an EVENT playlist.
//   - A segment is listed only for a while. One that cannot be fetched in
//     time is skipped, which leaves a short jump in the file, instead of
//     failing the recording. An init section that cannot be fetched drops
//     the segments that need it.
//   - Stopping is how a recording ends, so Pause finishes the file rather
//     than pausing it. Removing the download abandons it instead. Until
//     then the pieces and the list of them (live.json) stay in the work
//     folder; when ODM is closed mid-way, Resume continues the recording
//     after a gap.
//   - With separate audio, both playlists must load in a round before
//     either one's new segments are taken, or one track would advance
//     past segments that are then never fetched.
//   - A reload can return an older copy of the playlist (a lagging CDN
//     edge). Numbers going back count as an encoder restart only when
//     they drop below the whole window; a slightly older copy is ignored.
//   - Init sections are told apart by URL path and byte range, not the
//     full URL: many CDNs re-sign every URI on each reload.
//   - Separate audio is joined with ffmpeg, which lines the two tracks up
//     by their timestamps.
//   - Live DASH manifests are not recorded.

// errLiveStream tells runStream that the playlist is live and has to be
// recorded rather than downloaded.
var errLiveStream = errors.New("live stream")

// errRemoved cancels a recording that is being removed: unlike a Stop it
// must not assemble a file.
var errRemoved = errors.New("download removed")

const liveStateFile = "live.json"

// liveTrack is the recording of one playlist, saved in live.json. Pieces
// are numbered as for a finished stream ("init<i>", "<n>"), so assembly
// works the same; a piece that was skipped has no file.
type liveTrack struct {
	Ext     string   `json:"ext"`
	Video   bool     `json:"video"`
	Inits   []string `json:"inits"`    // init section identities, by piece number
	SegInit []int    `json:"seg_init"` // init piece per segment, -1 for none
	LastSeq int64    `json:"last_seq"` // media sequence of the newest segment taken
	Started bool     `json:"started"`  // LastSeq is set
}

type liveState struct {
	Tracks []*liveTrack `json:"tracks"`
}

// Timing of the reload loop. A recording ends when nothing new arrived
// for liveStallTargets target durations, and at least liveMinStall.
var (
	liveMinWait      = time.Second
	liveMaxWait      = 10 * time.Second
	liveMinStall     = 30 * time.Second
	liveStallTargets = time.Duration(6)
)

// liveResolveAfter is the number of failed reloads in a row after which
// the master playlist is read again: media playlist URLs often carry a
// token that expires.
const liveResolveAfter = 3

func (d *Download) liveStatePath() string {
	return filepath.Join(d.workDir(), liveStateFile)
}

func (d *Download) loadLiveState() (*liveState, error) {
	data, err := os.ReadFile(d.liveStatePath())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var st liveState
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("%w: %v", errLiveStateCorrupt, err)
	}
	return &st, nil
}

// errLiveStateCorrupt marks a live.json that exists but cannot be parsed.
var errLiveStateCorrupt = errors.New("live.json is damaged")

// saveLiveState writes live.json through a synced temporary file, so a
// crash or power loss leaves the previous list rather than half a new one.
func (d *Download) saveLiveState(st *liveState) error {
	data, err := json.Marshal(st)
	if err != nil {
		return err
	}
	p := d.liveStatePath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	f, err := os.Create(p + ".tmp")
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	return os.Rename(p+".tmp", p)
}

// livePlaylists returns the media playlist(s) to record: the pinned (or
// best) variant and its separate audio for a master playlist, else the
// manifest itself.
func (d *Download) livePlaylists(ctx context.Context, job StreamJob) ([]string, error) {
	if job.Video != "" {
		urls := []string{job.Video}
		if job.Audio != "" {
			urls = append(urls, job.Audio)
		}
		return urls, nil
	}
	data, base, err := d.fetchText(ctx, job.Manifest)
	if err != nil {
		return nil, err
	}
	p, err := media.ParseHLS(base, data)
	if err != nil {
		return nil, err
	}
	if p.Master == nil {
		return []string{base.String()}, nil
	}
	v, a, err := pickHLS(p.Master, job.Variant, job.AudioKey)
	if err != nil {
		return nil, err
	}
	d.mu.Lock()
	if d.Stream.Variant == "" {
		d.Stream.Variant = hlsVariantKey(*v)
		if a != nil {
			d.Stream.AudioKey = hlsAudioKey(*a)
		}
	}
	d.mu.Unlock()
	urls := []string{v.URL}
	if a != nil {
		urls = append(urls, a.URL)
	}
	return urls, nil
}

// recordLive records until stopped, then assembles what it has into the
// .odmpart. It returns nil once the file is assembled, also when ctx was
// cancelled: for a recording that is the normal end. A removed download
// returns errRemoved without assembling.
func (d *Download) recordLive(ctx context.Context) error {
	d.mu.Lock()
	d.Stream.Live = true
	job := *d.Stream
	d.mu.Unlock()
	urls, err := d.livePlaylists(ctx, job)
	if err != nil && job.Page != "" && ctx.Err() == nil {
		// A playlist found by yt-dlp may have expired since (a Resume
		// hours later): ask it for the broadcast's current one.
		if m, h, rerr := resolveSiteLive(ctx, job.Page); rerr == nil {
			d.useSiteManifest(m, h)
			job.Manifest = m
			urls, err = d.livePlaylists(ctx, job)
		}
	}
	if err != nil {
		return err
	}

	// Whether this download already claimed its file is its own state
	// (Recording), not whether a live.json exists under its name: another
	// download may have claimed that name and written one.
	var st *liveState
	if job.Recording {
		st, err = d.loadLiveState()
		if err != nil && !errors.Is(err, errLiveStateCorrupt) {
			// Maybe only for now (a file lock, an I/O error): keep the
			// pieces, so Resume can try again.
			return err
		}
		if st == nil {
			// The list is missing or damaged: pieces left in the folder
			// cannot be matched to it, and new ones would reuse their
			// numbers. Start the folder over under the claimed name.
			if err := os.RemoveAll(d.workDir()); err != nil {
				return err
			}
		}
	}
	if st == nil {
		st = &liveState{}
		for i := range urls {
			st.Tracks = append(st.Tracks, &liveTrack{Video: i == 0})
		}
	} else if len(st.Tracks) != len(urls) {
		return errors.New("the stream changed since this recording started; remove it and add it again")
	}

	r := &liveRecorder{d: d, st: st, urls: urls, job: job, run: &streamRun{d: d, keys: map[string][]byte{}},
		initTries: map[string]int{}, regressed: make([]int, len(urls))}
	// A live segment is gone soon: an expired link is not worth a reload
	// of the whole playlist, as it is for a finished stream.
	r.run.refreshed = true
	prepared := job.Recording
	if prepared {
		if err := d.openTarget(false); err != nil {
			return err
		}
		r.countExisting()
	}

	var stopErr error // a reason to stop that is reported after assembly
	lastNew := time.Now()
	failed := 0
	for {
		started := time.Now()
		round, ended, target, err := r.reload(ctx)
		switch {
		case err == nil:
			failed = 0
		case ctx.Err() != nil:
		case errors.Is(err, errLocalNetwork) || (!prepared && isRefusal(err)):
			stopErr = err
		default:
			// A failed reload (network, an error page instead of the
			// playlist) is retried until the stall limit.
			round = nil
			failed++
			if failed%liveResolveAfter == 0 {
				r.resolveAgain(ctx)
			}
		}
		if stopErr != nil {
			break
		}
		if len(round) > 0 && !prepared {
			if err := r.prepare(); err != nil {
				return err
			}
			prepared = true
		}
		// Init sections lost earlier (a failed fetch, a crash mid-way)
		// are fetched again; they cost one request each.
		round = append(r.missingInits(round), round...)
		if len(round) > 0 {
			if err := d.saveLiveState(st); err != nil {
				return err
			}
			if err := r.fetch(ctx, round); err != nil {
				stopErr = err
				break
			}
		}
		// Only new segments count as progress: a lost init fetched again
		// must not hold off the stall limit.
		if len(r.lastTaken) > 0 {
			lastNew = time.Now()
		}
		if ctx.Err() != nil || ended {
			break
		}
		if time.Since(lastNew) > max(liveStallTargets*target, liveMinStall) {
			break
		}
		// RFC 8216 6.3.4: reload one target duration after the previous
		// load began, half of one when the playlist had nothing new.
		wait := target
		if len(r.lastTaken) == 0 {
			wait /= 2
		}
		wait = min(max(wait, liveMinWait), liveMaxWait) - time.Since(started)
		if wait > 0 {
			select {
			case <-ctx.Done():
			case <-time.After(wait):
			}
		}
		if ctx.Err() != nil {
			break
		}
	}

	if d.isRemoved() {
		return errRemoved
	}
	if !prepared || r.recorded() == 0 {
		if stopErr != nil {
			return stopErr
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("the live stream sent nothing to record")
	}
	// Stopping ends the recording: assemble even though ctx is done. Only
	// removing the download cancels this (assemblyContext).
	if err := d.assembleStream(d.assemblyContext(), r.tracks()); err != nil {
		if d.isRemoved() {
			return errRemoved
		}
		return err
	}
	if stopErr != nil {
		// What was recorded is kept as the finished file; the reason the
		// recording ended early stays visible with it.
		d.mu.Lock()
		d.Err = "recording ended early: " + stopErr.Error()
		d.mu.Unlock()
	}
	return nil
}

func isRefusal(err error) bool {
	var r refusal
	return errors.As(err, &r)
}

// liveRecorder holds one run of recordLive.
type liveRecorder struct {
	d         *Download
	st        *liveState
	urls      []string
	job       StreamJob
	run       *streamRun
	lastTaken []pieceJob                    // the latest round's new pieces
	inits     map[int]map[int]*media.HLSMap // see initURLs
	initTries map[string]int                // fetch rounds per lost init
	regressed []int                         // reloads in a row whose numbers went back, per track
}

// resolveAgain re-reads the master playlist for fresh media playlist URLs
// (expired tokens). It keeps the old ones when that fails or the track
// count differs.
func (r *liveRecorder) resolveAgain(ctx context.Context) {
	if r.job.Video != "" {
		return // given directly, nothing to re-read
	}
	urls, err := r.d.livePlaylists(ctx, r.job)
	if err != nil && r.job.Page != "" {
		// The master playlist itself expired: yt-dlp finds the
		// broadcast's current one on its page.
		if m, h, rerr := resolveSiteLive(ctx, r.job.Page); rerr == nil {
			r.d.useSiteManifest(m, h)
			r.job.Manifest = m
			urls, err = r.d.livePlaylists(ctx, r.job)
		}
	}
	if err == nil && len(urls) == len(r.urls) {
		r.urls = urls
	}
}

// prepare names the file and claims it once the first segments show what
// container the recording becomes.
func (r *liveRecorder) prepare() error {
	d := r.d
	d.mu.Lock()
	d.FileName = streamName(d.FileName, outputExt(r.tracks()))
	d.mu.Unlock()
	if err := d.openTarget(true); err != nil {
		return err
	}
	// The name was just claimed: a work folder under it is a leftover of
	// an unrelated download.
	if err := os.RemoveAll(d.workDir()); err != nil {
		return err
	}
	d.mu.Lock()
	d.Stream.Recording = true
	d.RangeOK = true // resumable: Resume continues the recording
	d.mu.Unlock()
	return nil
}

// reload reads every playlist and, only when all of them loaded, takes
// their new segments. It returns those pieces, whether all playlists have
// ended, and the longest target duration.
func (r *liveRecorder) reload(ctx context.Context) ([]pieceJob, bool, time.Duration, error) {
	r.lastTaken = nil
	lists := make([]*media.HLSMedia, len(r.urls))
	ended := true
	var target time.Duration
	for ti, u := range r.urls {
		data, base, err := r.d.fetchText(ctx, u)
		if err != nil {
			return nil, false, target, err
		}
		p, err := media.ParseHLS(base, data)
		if err != nil {
			return nil, false, target, refusal(err.Error())
		}
		if p.Media == nil {
			return nil, false, target, refusal("expected a media playlist, got a master playlist")
		}
		m := p.Media
		if m.Protected {
			return nil, false, target, refusal("this stream is DRM-protected and cannot be downloaded")
		}
		target = max(target, m.TargetDuration)
		ended = ended && !m.Live()
		lists[ti] = m
	}
	// Only inits the current playlists still use are worth fetching again.
	r.inits = nil
	var jobs []pieceJob
	for ti, m := range lists {
		r.noteInits(ti, m)
		jobs = append(jobs, r.take(ti, m)...)
	}
	r.lastTaken = jobs
	return jobs, ended, target, nil
}

// initID identifies an init section across reloads: by URL path and byte
// range, as the query (a signature) may change every time.
func initID(m *media.HLSMap) string {
	id := urlPath(m.URL)
	if m.Range != nil {
		id += "|" + m.Range.Header()
	}
	return id
}

// take appends the playlist's new segments (and any init section they
// need) to track ti and returns them as jobs.
func (r *liveRecorder) take(ti int, m *media.HLSMedia) []pieceJob {
	t := r.st.Tracks[ti]
	n := len(m.Segments)
	if n == 0 {
		return nil
	}
	if newest := m.Segments[n-1].Sequence; t.Started && newest <= t.LastSeq {
		if newest < t.LastSeq {
			r.regressed[ti]++
		} else {
			r.regressed[ti] = 0 // up to date, nothing new
		}
		// An older copy from a lagging edge is ignored. Numbers that drop
		// below the whole window, or stay behind for three reloads, mean
		// the encoder restarted: take the new numbers from the start.
		if newest >= t.LastSeq-int64(n) && r.regressed[ti] < 3 {
			return nil
		}
		t.LastSeq = m.Segments[0].Sequence - 1
	}
	r.regressed[ti] = 0
	var jobs []pieceJob
	for _, s := range m.Segments {
		if t.Started && s.Sequence <= t.LastSeq {
			continue
		}
		if t.Ext == "" {
			switch {
			case s.Map != nil && t.Video:
				t.Ext = "mp4"
			case s.Map != nil:
				t.Ext = "m4a"
			case !t.Video && strings.EqualFold(path.Ext(urlPath(s.URL)), ".aac"):
				t.Ext = "aac"
			default:
				t.Ext = "ts"
			}
		}
		idx := -1
		if s.Map != nil {
			id := initID(s.Map)
			for i, have := range t.Inits {
				if have == id {
					idx = i
					break
				}
			}
			if idx < 0 {
				idx = len(t.Inits)
				t.Inits = append(t.Inits, id)
				jobs = append(jobs, initJob(ti, idx, s.Map))
			}
		}
		n := len(t.SegInit)
		t.SegInit = append(t.SegInit, idx)
		jobs = append(jobs, pieceJob{track: ti, name: strconv.Itoa(n),
			p: &piece{url: s.URL, rng: s.Range, key: s.Key, seq: s.Sequence}})
		t.LastSeq, t.Started = s.Sequence, true
	}
	return jobs
}

func initJob(ti, idx int, m *media.HLSMap) pieceJob {
	return pieceJob{track: ti, name: "init" + strconv.Itoa(idx), init: true,
		p: &piece{url: m.URL, rng: m.Range, key: m.Key}}
}

// noteInits records, for missingInits, the current playlist entry of each
// init piece the playlist still uses (with its latest, freshly signed URL).
func (r *liveRecorder) noteInits(ti int, m *media.HLSMedia) {
	t := r.st.Tracks[ti]
	for _, s := range m.Segments {
		if s.Map == nil {
			continue
		}
		id := initID(s.Map)
		for i, have := range t.Inits {
			if have == id {
				r.initURLs(ti)[i] = s.Map
				break
			}
		}
	}
}

// initURLs maps a track's init pieces to their entry in the current
// playlist. Rebuilt on every reload, so only inits still in use are
// fetched again.
func (r *liveRecorder) initURLs(ti int) map[int]*media.HLSMap {
	if r.inits == nil {
		r.inits = map[int]map[int]*media.HLSMap{}
	}
	if r.inits[ti] == nil {
		r.inits[ti] = map[int]*media.HLSMap{}
	}
	return r.inits[ti]
}

// missingInits returns jobs for init sections that have no file and are
// not already in round.
func (r *liveRecorder) missingInits(round []pieceJob) []pieceJob {
	queued := map[string]bool{}
	for _, j := range round {
		if j.init {
			queued[strconv.Itoa(j.track)+"/"+j.name] = true
		}
	}
	var jobs []pieceJob
	for ti := range r.st.Tracks {
		for idx, m := range r.initURLs(ti) {
			name := "init" + strconv.Itoa(idx)
			key := strconv.Itoa(ti) + "/" + name
			if queued[key] || fileSize(r.d.piecePath(ti, name)) > 0 || r.initTries[key] >= 2 {
				continue // two more rounds at most: its segments are dropped then
			}
			r.initTries[key]++
			jobs = append(jobs, initJob(ti, idx, m))
		}
	}
	return jobs
}

// fetch downloads a round's pieces with the download's connections. A
// piece that still fails after a few quick tries is skipped: a segment
// leaves a jump, an init section drops the segments that need it (see
// concat). Only a refusal to reach the local network stops the run.
func (r *liveRecorder) fetch(ctx context.Context, jobs []pieceJob) error {
	d := r.d
	for ti := range r.st.Tracks {
		if err := os.MkdirAll(filepath.Dir(d.piecePath(ti, "x")), 0o755); err != nil {
			return err
		}
	}
	d.mu.Lock()
	d.streamSegs = r.recorded()
	workers := max(d.Connections, 1)
	d.mu.Unlock()

	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	var hardErr error
	var once sync.Once
	for _, j := range jobs {
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
		}
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		go func(j pieceJob) {
			defer wg.Done()
			defer func() { <-sem }()
			err := r.fetchOne(ctx, j)
			switch {
			case err == nil:
				if !j.init {
					d.mu.Lock()
					d.streamHave++
					d.mu.Unlock()
				}
				d.updateEstimate()
				d.notify(false)
			case errors.Is(err, errLocalNetwork):
				once.Do(func() { hardErr = err })
			}
		}(j)
	}
	wg.Wait()
	return hardErr
}

func (r *liveRecorder) fetchOne(ctx context.Context, j pieceJob) error {
	var err error
	wait := 500 * time.Millisecond
	for attempt := 0; attempt < 3; attempt++ {
		err = r.run.fetch(ctx, j)
		// A 4xx (permanentError) on a live segment means it is gone.
		var perm permanentError
		if err == nil || ctx.Err() != nil || errors.Is(err, errLocalNetwork) || errors.As(err, &perm) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
		wait *= 2
	}
	return err
}

// recorded is the number of segments taken over all tracks.
func (r *liveRecorder) recorded() int {
	n := 0
	for _, t := range r.st.Tracks {
		n += len(t.SegInit)
	}
	return n
}

// countExisting restores the counters from the pieces on disk when a
// recording continues.
func (r *liveRecorder) countExisting() {
	var done int64
	have := 0
	for ti, t := range r.st.Tracks {
		for i := range t.Inits {
			done += fileSize(r.d.piecePath(ti, "init"+strconv.Itoa(i)))
		}
		for i := range t.SegInit {
			if n := fileSize(r.d.piecePath(ti, strconv.Itoa(i))); n > 0 {
				done += n
				have++
			}
		}
	}
	r.d.mu.Lock()
	r.d.streamDone, r.d.streamHave, r.d.streamSegs = done, have, r.recorded()
	r.d.mu.Unlock()
	r.d.updateEstimate()
}

// tracks turns the recording into tracks for assembleStream; a skipped
// segment has no file and is left out there.
func (r *liveRecorder) tracks() []*streamTrack {
	out := make([]*streamTrack, len(r.st.Tracks))
	for i, t := range r.st.Tracks {
		out[i] = &streamTrack{
			ext: t.Ext, video: t.Video, live: true,
			inits:   make([]piece, len(t.Inits)),
			segs:    make([]piece, len(t.SegInit)),
			segInit: t.SegInit,
		}
	}
	return out
}
