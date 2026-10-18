package torrent

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"odm/internal/engine"
	"odm/internal/version"
)

// Session is the engine's torrents on odm-libtorrent (engine.Torrents). It
// starts the program when the first torrent needs it, adds the engine's
// torrents to it under their downloads' IDs, and hands each torrent's
// events to the download that watches it. When the program stops, every
// torrent in it gets an error, and the next torrent to start starts the
// program again.
type Session struct {
	// Program returns odm-libtorrent's path, installing it first when the
	// person allowed that (tools.Keeper.Ensure).
	Program func(ctx context.Context) (string, error)
	// State keeps the session's DHT nodes between runs; "" for none.
	State string
	// Settings are the program's settings (PROTOCOL.md, "settings"), sent
	// when it starts; nil for DefaultSettings.
	Settings func() map[string]any
	Log      func(format string, args ...any)

	mu       sync.Mutex
	c        *Client
	starting chan struct{} // closed when a start under way ends
	added    map[string]bool
	watchers map[string][]chan engine.TorrentEvent
	waiters  map[*waiter]bool
	port     int
	down, up int64
	closed   bool
}

// DefaultSettings: listen on every interface at a port the system picks,
// with DHT and local peer discovery for magnet links.
func DefaultSettings() map[string]any {
	return map[string]any{
		"listen": "0.0.0.0:0,[::]:0", "incoming": true, "port_forwarding": false,
		"dht": true, "lsd": true, "user_agent": "ODM/" + version.Version,
	}
}

// errClosed is the error of a session ODM is closing.
var errClosed = errors.New("ODM is closing")

// quitWait bounds how long the program may take to save its torrents when
// ODM closes.
var quitWait = 15 * time.Second

func (s *Session) logf(format string, args ...any) {
	if s.Log != nil {
		s.Log(format, args...)
	}
}

// client returns the running program, starting it first.
func (s *Session) client(ctx context.Context) (*Client, error) {
	for {
		s.mu.Lock()
		switch {
		case s.closed:
			s.mu.Unlock()
			return nil, errClosed
		case s.c != nil:
			c := s.c
			s.mu.Unlock()
			return c, nil
		case s.starting != nil:
			ch := s.starting
			s.mu.Unlock()
			select {
			case <-ch:
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		ch := make(chan struct{})
		s.starting = ch
		s.mu.Unlock()
		c, err := s.start(ctx)
		s.mu.Lock()
		s.starting = nil
		if err == nil {
			if s.closed {
				err = errClosed
			} else {
				s.c, s.added = c, map[string]bool{}
			}
		}
		s.mu.Unlock()
		close(ch)
		if err != nil {
			if c != nil {
				c.Close(quitWait)
			}
			return nil, err
		}
		go s.watchExit(c)
		return c, nil
	}
}

func (s *Session) start(ctx context.Context) (*Client, error) {
	if s.Program == nil {
		return nil, errors.New("this ODM cannot run odm-libtorrent")
	}
	prog, err := s.Program(ctx)
	if err != nil {
		return nil, fmt.Errorf("odm-libtorrent: %w", err)
	}
	var args []string
	if s.State != "" {
		args = []string{"--state", s.State}
	}
	c, err := Start(ctx, prog, Options{Args: args, OnEvent: s.dispatch})
	if err != nil {
		return nil, err
	}
	settings := DefaultSettings()
	if s.Settings != nil {
		settings = s.Settings()
	}
	s.mu.Lock()
	settings["download_limit"], settings["upload_limit"] = s.down, s.up
	s.mu.Unlock()
	if err := c.Call(ctx, "settings", settings, nil); err != nil {
		c.Close(quitWait)
		return nil, fmt.Errorf("odm-libtorrent settings: %w", err)
	}
	h := c.Hello()
	s.logf("torrent: odm-libtorrent %s (libtorrent %s) started", h.Version, h.Libtorrent)
	return c, nil
}

// watchExit tells the torrents of a program that stopped on its own.
func (s *Session) watchExit(c *Client) {
	<-c.Done()
	s.mu.Lock()
	if s.c != c {
		s.mu.Unlock()
		return
	}
	s.c, s.port = nil, 0
	keys := s.added
	s.added = nil
	closing := s.closed
	for w := range s.waiters {
		close(w.gone)
		delete(s.waiters, w)
	}
	s.mu.Unlock()
	if closing {
		return
	}
	err := c.exitError()
	s.logf("torrent: %v", err)
	for key := range keys {
		s.deliver(key, engine.TorrentEvent{Kind: engine.TorrentEventError, Err: err.Error()})
	}
}

// dispatch takes the program's events.
func (s *Session) dispatch(ev Event) {
	s.mu.Lock()
	for w := range s.waiters {
		if w.key == ev.Key && w.match(ev) {
			select {
			case w.ch <- ev:
			default:
			}
		}
	}
	s.mu.Unlock()
	switch ev.Event {
	case "status":
		for _, st := range ev.Torrents {
			s.deliver(st.Key, engine.TorrentEvent{Kind: engine.TorrentEventStatus, Status: engineStatus(st)})
		}
	case "metadata":
		m := engineMeta(ev.Metadata)
		s.deliver(ev.Key, engine.TorrentEvent{Kind: engine.TorrentEventMetadata, Meta: &m})
	case "finished":
		s.deliver(ev.Key, engine.TorrentEvent{Kind: engine.TorrentEventFinished})
	case "error":
		msg := ev.Error
		if ev.File != "" {
			msg += ": " + ev.File
		}
		if ev.Key == "" {
			s.logf("torrent: %s", msg)
			return
		}
		s.deliver(ev.Key, engine.TorrentEvent{Kind: engine.TorrentEventError, Err: msg})
	case "listening":
		if ev.Type == "tcp" {
			s.mu.Lock()
			s.port = ev.Port
			s.mu.Unlock()
		}
		s.logf("torrent: listening on %s port %d (%s)", ev.Address, ev.Port, ev.Type)
	case "listen_failed":
		s.logf("torrent: cannot listen on %s port %d (%s): %s", ev.Address, ev.Port, ev.Type, ev.Error)
	case "portmap":
		s.logf("torrent: port %d opened through %s", ev.Port, ev.Via)
	case "portmap_failed":
		s.logf("torrent: no port through %s: %s", ev.Via, ev.Error)
	}
}

// deliver hands ev to the downloads watching key. A status is dropped for
// a reader that fell behind; the next one follows within a second.
func (s *Session) deliver(key string, ev engine.TorrentEvent) {
	s.mu.Lock()
	list := append([]chan engine.TorrentEvent(nil), s.watchers[key]...)
	s.mu.Unlock()
	for _, ch := range list {
		select {
		case ch <- ev:
		default:
			if ev.Kind != engine.TorrentEventStatus {
				s.logf("torrent: %s: %s event dropped", key, ev.Kind)
			}
		}
	}
}

// Port is the TCP port the program listens on, 0 while none.
func (s *Session) Port() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.port
}

// waiter collects the events of one torrent that match, for a command whose
// result comes as events (rename, move, remove).
type waiter struct {
	key   string
	match func(Event) bool
	ch    chan Event
	gone  chan struct{} // closed when the program stopped
}

func (s *Session) expect(key string, match func(Event) bool) *waiter {
	w := &waiter{key: key, match: match, ch: make(chan Event, 256), gone: make(chan struct{})}
	s.mu.Lock()
	if s.waiters == nil {
		s.waiters = map[*waiter]bool{}
	}
	s.waiters[w] = true
	s.mu.Unlock()
	return w
}

func (s *Session) forget(w *waiter) {
	s.mu.Lock()
	delete(s.waiters, w)
	s.mu.Unlock()
}

func (w *waiter) next(ctx context.Context) (Event, error) {
	select {
	case ev := <-w.ch:
		return ev, nil
	case <-w.gone:
		return Event{}, errors.New("odm-libtorrent stopped")
	case <-ctx.Done():
		return Event{}, ctx.Err()
	}
}

// in returns the running program when it has the torrent id.
func (s *Session) in(id string) *Client {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.c == nil || !s.added[id] {
		return nil
	}
	return s.c
}

// Inspect reads a .torrent file or a magnet link.
func (s *Session) Inspect(ctx context.Context, file, magnet string) (engine.TorrentMeta, error) {
	c, err := s.client(ctx)
	if err != nil {
		return engine.TorrentMeta{}, err
	}
	fields := map[string]any{"magnet": magnet}
	if file != "" {
		fields = map[string]any{"torrent": file}
	}
	var m Metadata
	if err := c.Call(ctx, "inspect", fields, &m); err != nil {
		return engine.TorrentMeta{}, err
	}
	return engineMeta(m), nil
}

// Add adds a torrent, or resumes it when the program has it already.
func (s *Session) Add(ctx context.Context, a engine.TorrentAdd) error {
	c, err := s.client(ctx)
	if err != nil {
		return err
	}
	if s.in(a.ID) == c {
		if err := c.Call(ctx, "resume", map[string]any{"key": a.ID}, nil); err != nil {
			return err
		}
		return c.Call(ctx, "limits", map[string]any{"key": a.ID, "download_limit": a.DownloadLimit}, nil)
	}
	fields := map[string]any{"key": a.ID, "save_path": a.SavePath}
	if a.Resume != "" {
		fields["resume"] = a.Resume
	}
	if a.File != "" {
		fields["torrent"] = a.File
	}
	if a.Magnet != "" {
		fields["magnet"] = a.Magnet
	}
	if len(a.Names) > 0 {
		fields["names"] = names(a.Names)
	}
	if a.Priorities != nil {
		fields["priorities"] = a.Priorities
	}
	if a.Hold {
		fields["hold"] = true
	}
	if a.DownloadLimit > 0 {
		fields["download_limit"] = a.DownloadLimit
	}
	if err := c.Call(ctx, "add", fields, nil); err != nil {
		return err
	}
	s.mu.Lock()
	if s.c == c {
		s.added[a.ID] = true
	}
	s.mu.Unlock()
	return nil
}

func names(m map[int]string) map[string]string {
	out := make(map[string]string, len(m))
	for i, n := range m {
		out[strconv.Itoa(i)] = n
	}
	return out
}

// Watch delivers the events of the torrent id until stop.
func (s *Session) Watch(id string) (<-chan engine.TorrentEvent, func()) {
	ch := make(chan engine.TorrentEvent, 64)
	s.mu.Lock()
	if s.watchers == nil {
		s.watchers = map[string][]chan engine.TorrentEvent{}
	}
	s.watchers[id] = append(s.watchers[id], ch)
	s.mu.Unlock()
	return ch, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		list := s.watchers[id]
		for i, c := range list {
			if c == ch {
				s.watchers[id] = append(list[:i], list[i+1:]...)
				break
			}
		}
		if len(s.watchers[id]) == 0 {
			delete(s.watchers, id)
		}
	}
}

func (s *Session) Pause(ctx context.Context, id string) error {
	if c := s.in(id); c != nil {
		return c.Call(ctx, "pause", map[string]any{"key": id}, nil)
	}
	return nil
}

func (s *Session) SetFiles(ctx context.Context, id string, names map[int]string, priorities []int) error {
	if len(names) > 0 {
		if err := s.Rename(ctx, id, names); err != nil {
			return err
		}
	}
	c := s.in(id)
	if c == nil {
		return errors.New("odm-libtorrent stopped")
	}
	return c.Call(ctx, "priorities", map[string]any{"key": id, "priorities": priorities}, nil)
}

func (s *Session) Rename(ctx context.Context, id string, m map[int]string) error {
	c := s.in(id)
	if c == nil {
		return errors.New("odm-libtorrent stopped")
	}
	w := s.expect(id, func(ev Event) bool { return ev.Event == "renamed" || ev.Event == "rename_failed" })
	defer s.forget(w)
	if err := c.Call(ctx, "rename", map[string]any{"key": id, "names": names(m)}, nil); err != nil {
		return err
	}
	left := map[int]bool{}
	for i := range m {
		left[i] = true
	}
	for len(left) > 0 {
		ev, err := w.next(ctx)
		if err != nil {
			return err
		}
		if !left[ev.Index] {
			continue
		}
		if ev.Event == "rename_failed" {
			return fmt.Errorf("renaming %s: %s", m[ev.Index], ev.Error)
		}
		delete(left, ev.Index)
	}
	return nil
}

func (s *Session) Move(ctx context.Context, id, savePath string) error {
	c := s.in(id)
	if c == nil {
		return errors.New("odm-libtorrent stopped")
	}
	w := s.expect(id, func(ev Event) bool { return ev.Event == "moved" || ev.Event == "move_failed" })
	defer s.forget(w)
	if err := c.Call(ctx, "move", map[string]any{"key": id, "save_path": savePath}, nil); err != nil {
		return err
	}
	ev, err := w.next(ctx)
	if err != nil {
		return err
	}
	if ev.Event == "move_failed" {
		if ev.File != "" {
			return fmt.Errorf("moving %s: %s", ev.File, ev.Error)
		}
		return fmt.Errorf("moving the files: %s", ev.Error)
	}
	return nil
}

func (s *Session) SetLimits(ctx context.Context, id string, download, upload int64) error {
	if c := s.in(id); c != nil {
		return c.Call(ctx, "limits", map[string]any{"key": id, "download_limit": download, "upload_limit": upload}, nil)
	}
	return nil
}

// SetGlobalLimits sets the limits for every torrent, now and when the
// program starts.
func (s *Session) SetGlobalLimits(download, upload int64) {
	s.mu.Lock()
	s.down, s.up = download, upload
	c := s.c
	s.mu.Unlock()
	if c == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := c.Call(ctx, "settings", map[string]any{"download_limit": download, "upload_limit": upload}, nil); err != nil {
			s.logf("torrent: limits: %v", err)
		}
	}()
}

func (s *Session) Remove(ctx context.Context, id string, deleteFiles bool) error {
	c := s.in(id)
	if c == nil {
		return nil
	}
	w := s.expect(id, func(ev Event) bool { return ev.Event == "removed" })
	defer s.forget(w)
	if err := c.Call(ctx, "remove", map[string]any{"key": id, "delete_files": deleteFiles}, nil); err != nil {
		return err
	}
	s.mu.Lock()
	if s.c == c {
		delete(s.added, id)
	}
	s.mu.Unlock()
	ev, err := w.next(ctx)
	if err != nil {
		return err
	}
	if ev.Error != "" {
		return fmt.Errorf("deleting the files: %s", ev.Error)
	}
	return nil
}

// Close stops the program, which first saves its torrents' resume data.
func (s *Session) Close() {
	s.mu.Lock()
	s.closed = true
	c := s.c
	s.mu.Unlock()
	if c != nil {
		c.Close(quitWait)
	}
}

func engineStatus(st Status) engine.TorrentStatus {
	return engine.TorrentStatus{
		State: st.State, Paused: st.Paused, Size: st.Size, Done: st.Done,
		DownloadRate: st.DownloadRate, UploadRate: st.UploadRate, Uploaded: st.Uploaded,
		Peers: st.Peers, Seeds: st.Seeds, Error: st.Error,
	}
}

func engineMeta(m Metadata) engine.TorrentMeta {
	out := engine.TorrentMeta{Name: m.Name, Size: m.Size, InfoHash: m.InfoHash, InfoHashV2: m.InfoHashV2,
		Private: m.Private, Trackers: m.Trackers, Magnet: m.Magnet}
	for _, f := range m.Files {
		out.Files = append(out.Files, engine.TorrentFile{Path: f.Path, Size: f.Size, Pad: f.Pad})
	}
	return out
}
