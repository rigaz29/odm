package fluent

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"odm/internal/engine"
	"odm/internal/osutil"
)

// Item is one download as the page shows it: engine.Info without the
// per-connection detail the list has no use for.
type Item struct {
	ID          string    `json:"id"`
	URL         string    `json:"url"`
	FileName    string    `json:"fileName"`
	FilePath    string    `json:"filePath"`
	Status      string    `json:"status"`
	Error       string    `json:"error"`
	Total       int64     `json:"total"`
	Done        int64     `json:"done"`
	Progress    float64   `json:"progress"`
	Speed       int64     `json:"speed"`
	Category    string    `json:"category"`
	Queue       string    `json:"queue"`
	Description string    `json:"description"`
	Live        bool      `json:"live"`
	Site        bool      `json:"site"`
	Stream      string    `json:"stream"`
	Verifying   bool      `json:"verifying"`
	CreatedAt   time.Time `json:"createdAt"`
	// UpdatedAt is the last attempt: when the download last started,
	// stopped or failed.
	UpdatedAt time.Time `json:"updatedAt"`
}

// Snapshot is everything the main window shows.
type Snapshot struct {
	Items      []Item   `json:"items"`
	Categories []string `json:"categories"`
	Queues     []string `json:"queues"`
	// Running lists the queues that are started.
	Running []string `json:"running"`
	// Limit is the speed limit in force, in bytes per second; 0 for none.
	Limit int64 `json:"limit"`
}

// SnapshotEvent is the event that carries a new Snapshot to the page.
const SnapshotEvent = "odm:snapshot"

// Service is what the page calls. Its exported methods become the
// frontend's bindings (frontend/src/bindings, made by wails3 generate
// bindings).
type Service struct {
	mgr *engine.Manager
	// emit sends an event to the page; nil until the window exists.
	emit func(name string, data any)

	mu   sync.Mutex
	last []byte        // the snapshot sent last, to send only changes
	wake chan struct{} // asks watch to send now
}

// NewService makes the page's Service for mgr.
func NewService(mgr *engine.Manager) *Service {
	return &Service{mgr: mgr, wake: make(chan struct{}, 1)}
}

// Snapshot returns what the main window shows now.
func (s *Service) Snapshot() Snapshot {
	list := s.mgr.List()
	items := make([]Item, len(list))
	for i, d := range list {
		items[i] = Item{
			ID: d.ID, URL: d.URL, FileName: d.FileName, FilePath: d.FilePath,
			Status: string(d.Status), Error: d.Error,
			Total: d.Total, Done: d.Done, Progress: d.Progress, Speed: d.Speed,
			Category: d.Category, Queue: d.Queue, Description: d.Description,
			Live: d.Live, Site: d.Site, Stream: d.Stream, Verifying: d.Verifying,
			CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
		}
	}
	queues := s.mgr.Queues()
	var running []string
	for _, q := range queues {
		if s.mgr.QueueRunning(q) {
			running = append(running, q)
		}
	}
	return Snapshot{
		Items: items, Categories: engine.Categories(), Queues: queues, Running: running,
		Limit: s.mgr.Settings().EffectiveSpeedLimit(),
	}
}

// watch sends the snapshot to the page whenever it changed, checking
// every interval and at once when an action asks (poke), until ctx ends.
func (s *Service) watch(ctx context.Context, interval time.Duration) {
	tick := time.NewTicker(interval)
	defer tick.Stop()
	for {
		s.send()
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		case <-s.wake:
		}
	}
}

// send emits the snapshot if it differs from the last one sent.
func (s *Service) send() {
	snap := s.Snapshot()
	data, err := json.Marshal(snap)
	if err != nil {
		return
	}
	s.mu.Lock()
	same := bytes.Equal(data, s.last)
	if !same {
		s.last = data
	}
	emit := s.emit
	s.mu.Unlock()
	if !same && emit != nil {
		emit(SnapshotEvent, snap)
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

// PauseAll stops every running download.
func (s *Service) PauseAll() {
	s.mgr.PauseAll()
	s.poke()
}

// Remove takes the downloads ids off the list, their files kept.
func (s *Service) Remove(ids []string) {
	for _, id := range ids {
		s.mgr.Remove(id)
	}
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

// AddURL adds a download of address and starts it. Stage 3 brings the New
// Download window with its probe, folder and queue choices.
func (s *Service) AddURL(address string) (Item, error) {
	address = strings.TrimSpace(address)
	if !strings.HasPrefix(address, "http://") && !strings.HasPrefix(address, "https://") {
		return Item{}, errNotHTTP
	}
	d := s.mgr.Create(engine.AddOptions{URL: address, Start: true})
	s.poke()
	return Item{ID: d.ID, FileName: d.FileName, Status: string(d.Status)}, nil
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

// Appearance returns how the page should look: the language, and the
// Windows accent colour (empty when Windows does not say).
func (s *Service) Appearance() Appearance {
	return currentAppearance()
}

type serviceError string

func (e serviceError) Error() string { return string(e) }

const (
	errNotHTTP    serviceError = "an http or https address is needed"
	errNoDownload serviceError = "no such download"
)
