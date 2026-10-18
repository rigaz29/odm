package gui

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"slices"
	"sync"
	"time"

	"odm/internal/browsers"
	"odm/internal/engine"
	"odm/internal/osutil"
	"odm/internal/tools"
)

// Item is one download as the page shows it: engine.Info without the
// per-connection detail the list has no use for.
type Item struct {
	ID       string  `json:"id"`
	URL      string  `json:"url"`
	FileName string  `json:"fileName"`
	FilePath string  `json:"filePath"`
	Status   string  `json:"status"`
	Error    string  `json:"error"`
	Total    int64   `json:"total"`
	Done     int64   `json:"done"`
	Progress float64 `json:"progress"`
	Speed    int64   `json:"speed"`
	Category string  `json:"category"`
	Queue    string  `json:"queue"`
	// Order is the place in its queue's run order: lower runs first.
	Order       int    `json:"order"`
	Description string `json:"description"`
	Live        bool   `json:"live"`
	Site        bool   `json:"site"`
	Stream      string `json:"stream"`
	Verifying   bool   `json:"verifying"`
	// A torrent: Seeding while it uploads after finishing, at UploadSpeed;
	// Uploaded in all; Peers and Seeds connected.
	Torrent     bool      `json:"torrent"`
	Seeding     bool      `json:"seeding"`
	Uploaded    int64     `json:"uploaded"`
	UploadSpeed int64     `json:"uploadSpeed"`
	Peers       int       `json:"peers"`
	Seeds       int       `json:"seeds"`
	CreatedAt   time.Time `json:"createdAt"`
	// UpdatedAt is the last attempt: when the download last started,
	// stopped or failed.
	UpdatedAt time.Time `json:"updatedAt"`
	// For a running download: each connection's progress through its part
	// (0 to 1; at most maxConns, for a file download), its speed over the
	// last minute (a sample a second, oldest first) and when it started
	// running. Empty and zero otherwise.
	Conns       []float64 `json:"conns"`
	Speeds      []int64   `json:"speeds"`
	ActiveSince time.Time `json:"activeSince"`
}

// Snapshot is everything the main window shows.
type Snapshot struct {
	Items      []Item   `json:"items"`
	Categories []string `json:"categories"`
	Queues     []string `json:"queues"`
	// Running lists the queues that are started.
	Running []string `json:"running"`
	// Limit is the speed limit in force, in bytes per second; 0 for none.
	// SavedLimit is the one the limiter uses when turned on, also while it
	// is off; 0 when none was ever set.
	Limit      int64 `json:"limit"`
	SavedLimit int64 `json:"savedLimit"`
	// Speeds is the total speed over the last minute, a sample a second.
	Speeds []int64 `json:"speeds"`
}

// History lengths and sizes for the page's running-download cards.
const (
	speedSamples = 60 // a minute, a sample a second
	maxConns     = 16 // connection bars on a card
)

// SnapshotEvent is the event that carries a Change to the page.
const SnapshotEvent = "odm:snapshot"

// Change is what changed in the Snapshot since the last one sent: a list
// of a thousand downloads with a few running sends those few twice a
// second, not the thousand. The page applies it to the snapshot it holds
// (frontend/src/snapshot.ts).
type Change struct {
	// Items are the downloads that are new or changed, whole.
	Items []Item `json:"items"`
	// IDs lists every download, in order, when that changed: one was added
	// or removed, or the order changed. Nil (null) when it did not.
	IDs []string `json:"ids"`
	// The rest is the Snapshot's, sent whole: it is small.
	Categories []string `json:"categories"`
	Queues     []string `json:"queues"`
	Running    []string `json:"running"`
	Limit      int64    `json:"limit"`
	SavedLimit int64    `json:"savedLimit"`
	Speeds     []int64  `json:"speeds"`
}

// Service is what the page calls. Its exported methods become the
// frontend's bindings (frontend/src/bindings, made by wails3 generate
// bindings).
type Service struct {
	mgr *engine.Manager
	// emit sends an event to the page; nil until the window exists.
	emit func(name string, data any)

	mu sync.Mutex
	// What the page was sent last, to send only changes (send): each
	// download, the list's order and the rest of the snapshot.
	sent     map[string]Item
	sentIDs  []string
	sentRest []byte
	wake     chan struct{} // asks watch to send now

	hmu    sync.Mutex
	speeds map[string][]int64   // per running download, see sample
	total  []int64              // all downloads together
	since  map[string]time.Time // when each running download started
	now    func() time.Time     // the clock, replaceable in tests

	// The New Download window's browser hand-offs (by token) and what
	// checking addresses found (add.go), and each download's last status
	// (noticeFinished).
	amu      sync.Mutex
	seq      int
	prefills map[string]engine.AddOptions
	probes   map[string]*probed
	statuses map[string]engine.Status
	// The window's looks at torrents (torrent.go), by address.
	tmu      sync.Mutex
	torrents map[string]*torrentProbe

	// windows opens ODM's other windows; nil in tests. keeper keeps
	// yt-dlp and ffmpeg; nil in builds without it. Both under mu.
	windows windowOpener
	keeper  *tools.Keeper
	// quit closes ODM, for a finish action; nil in tests. announce
	// notifies and plays sounds (announce.go); nil in tests.
	quit     func()
	announce announcer
	// onLook is told when the Options change the look (the tray's menu).
	onLook func()
	// installed lists the browsers and launch opens one (setup.go);
	// replaced in tests.
	installed func() []browsers.Browser
	launch    func(b browsers.Browser, args ...string) error
}

// NewService makes the page's Service for mgr.
func NewService(mgr *engine.Manager) *Service {
	return &Service{mgr: mgr, wake: make(chan struct{}, 1), sent: map[string]Item{},
		speeds: map[string][]int64{}, since: map[string]time.Time{}, now: time.Now,
		prefills: map[string]engine.AddOptions{}, probes: map[string]*probed{},
		installed: browsers.Installed, launch: func(b browsers.Browser, args ...string) error { return b.Open(args...) }}
}

// Snapshot returns what the main window shows now.
func (s *Service) Snapshot() Snapshot {
	list := s.mgr.List()
	items := make([]Item, len(list))
	s.hmu.Lock()
	for i, d := range list {
		items[i] = s.itemLocked(d)
	}
	total := slices.Clone(s.total)
	s.hmu.Unlock()
	queues := s.mgr.Queues()
	var running []string
	for _, q := range queues {
		if s.mgr.QueueRunning(q) {
			running = append(running, q)
		}
	}
	st := s.mgr.Settings()
	return Snapshot{
		Items: items, Categories: engine.Categories(), Queues: queues, Running: running,
		Limit: st.EffectiveSpeedLimit(), SavedLimit: st.SpeedLimit, Speeds: total,
	}
}

// connProgress is each connection's progress through its part, at most
// maxConns of them (neighbours averaged when there are more).
func connProgress(chunks []engine.ChunkInfo) []float64 {
	if len(chunks) == 0 {
		return nil
	}
	each := make([]float64, len(chunks))
	for i, c := range chunks {
		switch {
		case c.Complete:
			each[i] = 1
		case c.Done+c.Remaining > 0:
			each[i] = float64(c.Done) / float64(c.Done+c.Remaining)
		}
	}
	if len(each) <= maxConns {
		return each
	}
	out := make([]float64, maxConns)
	for b := range out {
		lo, hi := b*len(each)/maxConns, (b+1)*len(each)/maxConns
		var sum float64
		for _, v := range each[lo:hi] {
			sum += v
		}
		out[b] = sum / float64(hi-lo)
	}
	return out
}

// itemOf is d as the page lists it, without a running download's history.
func itemOf(d engine.Info) Item {
	it := Item{
		ID: d.ID, URL: d.URL, FileName: d.FileName, FilePath: d.FilePath,
		Status: string(d.Status), Error: d.Error,
		Total: d.Total, Done: d.Done, Progress: d.Progress,
		Category: d.Category, Queue: d.Queue, Order: d.Order, Description: d.Description,
		Live: d.Live, Site: d.Site, Stream: d.Stream, Verifying: d.Verifying,
		CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
		Torrent: d.Torrent, Seeding: d.Seeding, Uploaded: d.Uploaded,
	}
	if d.Status == engine.StatusDownloading || d.Seeding {
		it.UploadSpeed, it.Peers, it.Seeds = d.UploadSpeed, d.Peers, d.Seeds
	}
	// The engine keeps a paused download's last speed; only a running one
	// has a speed to show.
	if d.Status == engine.StatusDownloading {
		it.Speed = d.Speed
		it.Conns = connProgress(d.Chunks)
	}
	return it
}

// itemLocked is itemOf with a running download's speed over the last
// minute and when it started. The caller holds s.hmu.
func (s *Service) itemLocked(d engine.Info) Item {
	it := itemOf(d)
	if d.Status == engine.StatusDownloading {
		it.Speeds = slices.Clone(s.speeds[d.ID])
		it.ActiveSince = s.since[d.ID]
	}
	return it
}

// sample records every running download's speed and the total, and when
// each started running, for the cards' charts and the recording time.
func (s *Service) sample() {
	list := s.mgr.List()
	now := s.now()
	s.hmu.Lock()
	defer s.hmu.Unlock()
	running := map[string]bool{}
	var total int64
	for _, d := range list {
		if d.Status != engine.StatusDownloading {
			continue
		}
		running[d.ID] = true
		total += d.Speed
		if _, ok := s.since[d.ID]; !ok {
			s.since[d.ID] = now
		}
		s.speeds[d.ID] = appendSample(s.speeds[d.ID], d.Speed)
	}
	for id := range s.speeds {
		if !running[id] {
			delete(s.speeds, id)
		}
	}
	for id := range s.since {
		if !running[id] {
			delete(s.since, id)
		}
	}
	s.total = appendSample(s.total, total)
}

func appendSample(h []int64, v int64) []int64 {
	h = append(h, v)
	if len(h) > speedSamples {
		h = h[len(h)-speedSamples:]
	}
	return h
}

// watch sends the snapshot to the page whenever it changed, checking
// every interval and at once when an action asks (poke), until ctx ends.
func (s *Service) watch(ctx context.Context, interval time.Duration) {
	tick := time.NewTicker(interval)
	defer tick.Stop()
	var sampled time.Time
	for {
		if now := s.now(); now.Sub(sampled) >= time.Second {
			s.sample()
			s.noticeFinished(s.mgr.List())
			sampled = now
		}
		s.send()
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		case <-s.wake:
		}
	}
}

// send emits what changed in the snapshot since the last one sent, if
// anything did.
func (s *Service) send() {
	snap := s.Snapshot()
	ch := Change{Items: []Item{}, Categories: snap.Categories, Queues: snap.Queues, Running: snap.Running,
		Limit: snap.Limit, SavedLimit: snap.SavedLimit, Speeds: snap.Speeds}
	rest, err := json.Marshal(ch) // before the items go in: the rest
	if err != nil {
		return
	}
	ids := make([]string, len(snap.Items))
	for i, d := range snap.Items {
		ids[i] = d.ID
	}
	s.mu.Lock()
	for _, d := range snap.Items {
		if was, ok := s.sent[d.ID]; !ok || !reflect.DeepEqual(was, d) {
			ch.Items = append(ch.Items, d)
		}
	}
	if !slices.Equal(ids, s.sentIDs) {
		ch.IDs = ids
	}
	changed := len(ch.Items) > 0 || ch.IDs != nil || !bytes.Equal(rest, s.sentRest)
	if changed {
		for _, d := range ch.Items {
			s.sent[d.ID] = d
		}
		if ch.IDs != nil {
			keep := make(map[string]bool, len(ids))
			for _, id := range ids {
				keep[id] = true
			}
			for id := range s.sent {
				if !keep[id] {
					delete(s.sent, id)
				}
			}
		}
		s.sentIDs, s.sentRest = ids, rest
	}
	emit := s.emit
	s.mu.Unlock()
	if changed && emit != nil {
		emit(SnapshotEvent, ch)
	}
}

// poke has watch send the snapshot now, after an action changed it.
func (s *Service) poke() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// Resume starts or resumes the downloads ids.
func (s *Service) Resume(ids []string) {
	for _, id := range ids {
		s.mgr.Resume(id)
	}
	s.poke()
}

// Pause stops the downloads ids, keeping what they have.
func (s *Service) Pause(ids []string) {
	for _, id := range ids {
		s.mgr.Pause(id)
	}
	s.poke()
}

// PauseAll stops every running download; a started queue stays started.
func (s *Service) PauseAll() {
	s.mgr.PauseAll()
	s.poke()
}

// ResumeAll restarts every paused or failed download.
func (s *Service) ResumeAll() {
	s.mgr.ResumeAll()
	s.poke()
}

// StopAll pauses every download and stops every queue, so nothing carries
// on by itself.
func (s *Service) StopAll() {
	s.mgr.StopAll()
	s.poke()
}

// Remove takes the downloads ids off the list, and with withFiles deletes
// their files too.
func (s *Service) Remove(ids []string, withFiles bool) {
	for _, id := range ids {
		s.mgr.RemoveWithFile(id, withFiles)
	}
	s.poke()
}

// MoveToQueue puts the downloads ids in queue.
func (s *Service) MoveToQueue(ids []string, queue string) {
	for _, id := range ids {
		s.mgr.MoveToQueue(id, queue)
	}
	s.poke()
}

// MoveInQueue moves the download id up (delta -1) or down (+1) in its
// queue's order.
func (s *Service) MoveInQueue(id string, delta int) {
	s.mgr.MoveInQueue(id, delta)
	s.poke()
}

// SetSpeedLimiter turns the speed limit for all downloads on or off. A
// bytesPerSecond above 0 also sets the limit; 0 keeps the one saved.
func (s *Service) SetSpeedLimiter(on bool, bytesPerSecond int64) {
	s.mgr.SetSpeedLimiter(on, bytesPerSecond)
	s.poke()
}

// ClearCompleted takes the finished downloads off the list and returns
// how many there were.
func (s *Service) ClearCompleted() int {
	n := s.mgr.ClearCompleted()
	s.poke()
	return n
}

// StartQueue and StopQueue start and stop the queue name.
func (s *Service) StartQueue(name string) {
	s.mgr.StartQueue(name)
	s.poke()
}

func (s *Service) StopQueue(name string) {
	s.mgr.StopQueue(name)
	s.poke()
}

// OpenFile opens a finished download with its program; ShowInFolder
// shows it in Explorer.
func (s *Service) OpenFile(id string) error {
	if d, ok := s.mgr.Get(id); ok {
		return osutil.Open(d.FilePath)
	}
	return errNoDownload
}

func (s *Service) ShowInFolder(id string) error {
	if d, ok := s.mgr.Get(id); ok {
		return osutil.Reveal(d.FilePath)
	}
	return errNoDownload
}

// Appearance returns how the page should look: the language and the theme.
func (s *Service) Appearance() Appearance {
	return appearanceFor(s.mgr.Settings())
}

type serviceError string

func (e serviceError) Error() string { return string(e) }

const (
	errNoAddress  serviceError = "no address"
	errNoDownload serviceError = "no such download"
)
