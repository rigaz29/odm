package gui

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"odm/internal/engine"
	"odm/internal/tools"
)

// Torrents in the New Download window: what a .torrent file or a magnet
// link holds, read before it is added. A .torrent is read at once (fetched
// first when it is an address); a magnet link's metadata comes from its
// peers, so the window shows it waiting, with how many peers answered,
// while a torrent held in odm-libtorrent fetches it. What was read goes to
// the download (engine.AddOptions.TorrentFile, TorrentResume), which then
// need not read it again.

// TorrentFileRow is a file of a torrent, as the window lists it.
type TorrentFileRow struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
	Pad  bool   `json:"pad"`
}

// TorrentProbe is what the window knows of a torrent.
type TorrentProbe struct {
	Name    string           `json:"name"`
	Size    int64            `json:"size"`
	Files   []TorrentFileRow `json:"files"`
	Private bool             `json:"private"`
	// Waiting: a magnet link whose metadata has not come yet; Peers is how
	// many peers are connected meanwhile.
	Waiting bool `json:"waiting"`
	Peers   int  `json:"peers"`
	// NeedsProgram: odm-libtorrent is not installed yet; the window offers
	// to install it.
	NeedsProgram bool `json:"needsProgram"`
	// FirstTorrent: no torrent ran yet, so the system's firewall may ask
	// about odm-libtorrent (on Windows).
	FirstTorrent bool   `json:"firstTorrent"`
	Error        string `json:"error"`
}

// torrentProbe is one address's look at a torrent.
type torrentProbe struct {
	file   string // the .torrent read: a copy of ODM's for an address
	temp   bool   // file is ODM's copy, deleted with the probe
	key    string // a magnet link's held torrent in odm-libtorrent
	resume string // its resume data, which holds the metadata once in
	save   string // its folder, which stays empty
	cancel context.CancelFunc
	done   chan struct{} // closed once the held torrent is gone

	mu    sync.Mutex
	state TorrentProbe
}

func (p *torrentProbe) snapshot() TorrentProbe {
	p.mu.Lock()
	defer p.mu.Unlock()
	st := p.state
	st.Files = append([]TorrentFileRow(nil), p.state.Files...)
	return st
}

// probeLife bounds how long a magnet link's look waits for its metadata.
var probeLife = 15 * time.Minute

// resumeWaitingTorrents starts again the torrents that stopped because
// odm-libtorrent could not run, now that it is installed.
func (s *Service) resumeWaitingTorrents() {
	for _, d := range s.mgr.List() {
		if waitsForProgram(d) {
			s.mgr.Resume(d.ID)
		}
	}
}

// waitsForProgram reports whether d is a torrent that stopped because
// odm-libtorrent was missing or would not run (internal/torrent's errors
// name it).
func waitsForProgram(d engine.Info) bool {
	return d.Torrent && d.Status == engine.StatusError && strings.Contains(d.Error, "odm-libtorrent")
}

// libtorrentInstalled reports whether odm-libtorrent is there to run.
func libtorrentInstalled() bool {
	return os.Getenv("ODM_LIBTORRENT") != "" || tools.Find(tools.Libtorrent().Files[0]) != ""
}

// isTorrentAddress reports whether addr, as the window has it, is a torrent:
// a magnet link, a .torrent file on this computer, or the address of one
// (unless the Options say to download those as files).
func (s *Service) isTorrentAddress(addr string) bool {
	switch {
	case engine.IsMagnet(addr):
		return true
	case !engine.IsTorrentFile(addr):
		return false
	case strings.HasPrefix(addr, "http"):
		return !s.mgr.Settings().Torrent.NoTorrentLinks
	}
	st, err := os.Stat(addr)
	return err == nil && st.Mode().IsRegular()
}

// probeTorrent answers for a torrent.
func (s *Service) probeTorrent(addr string, headers map[string]string) ProbeResult {
	res := ProbeResult{URL: addr, Kind: "torrent"}
	if engine.Torrents == nil {
		res.Card = Card{State: "problem", Title: "This ODM cannot download torrents"}
		return res
	}
	st := TorrentProbe{FirstTorrent: runtime.GOOS == "windows" && s.mgr.Settings().Torrent.Port == 0}
	// A torrent in the list that may still run: odm-libtorrent would not
	// take it again, not even to read a magnet link's files.
	if twin, ok := s.twinOf(engine.AddOptions{URL: addr, Torrent: &engine.TorrentJob{}}); ok {
		res.Card = twin
		return res
	}
	if !libtorrentInstalled() {
		st.NeedsProgram = true
		res.Torrent = &st
		res.Card = Card{State: "warn", Title: "Torrents need odm-libtorrent", Detail: "ODM downloads it once from its releases."}
		return res
	}
	p, err := s.torrentProbe(addr, headers)
	if err != nil {
		res.Card = Card{State: "problem", Title: "The torrent could not be read", Error: err.Error()}
		return res
	}
	if p.file != "" {
		// A .torrent at an address: its hashes are known once it is read.
		if twin, ok := s.twinOf(engine.AddOptions{URL: addr, Torrent: &engine.TorrentJob{}, TorrentFile: p.file}); ok {
			res.Card = twin
			return res
		}
	}
	st = p.snapshot()
	res.Torrent = &st
	res.FileName = st.Name
	res.Category = torrentCategory(st.Files)
	res.Card = Card{State: "torrent", Name: st.Name, Size: st.Size}
	return res
}

// twinOf is the card for a download of the same torrent as o that may
// still run; false when there is none.
func (s *Service) twinOf(o engine.AddOptions) (Card, bool) {
	existing, free, ok := s.mgr.FindTorrent(o)
	if !ok || free {
		return Card{}, false
	}
	name := existing.FileName
	if name == "" {
		name = existing.URL
	}
	return Card{State: "twin", Name: name}, true
}

// probeFile is the .torrent the window read for addr; "" for none.
func (s *Service) probeFile(addr string) string {
	s.tmu.Lock()
	defer s.tmu.Unlock()
	if p := s.torrents[addr]; p != nil {
		return p.file
	}
	return ""
}

// torrentCategory is the category of a torrent's largest file.
func torrentCategory(files []TorrentFileRow) string {
	best, size := "", int64(-1)
	for _, f := range files {
		if !f.Pad && f.Size > size {
			best, size = f.Path, f.Size
		}
	}
	if best == "" {
		return ""
	}
	return engine.Category(best)
}

// torrentProbe returns addr's look at its torrent, starting it the first
// time.
func (s *Service) torrentProbe(addr string, headers map[string]string) (*torrentProbe, error) {
	s.tmu.Lock()
	if p := s.torrents[addr]; p != nil {
		s.tmu.Unlock()
		return p, nil
	}
	s.tmu.Unlock()
	b := engine.Torrents
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	first := runtime.GOOS == "windows" && s.mgr.Settings().Torrent.Port == 0
	p := &torrentProbe{}
	if engine.IsMagnet(addr) {
		meta, err := b.Inspect(ctx, "", addr)
		if err != nil {
			return nil, err
		}
		p.state = TorrentProbe{Name: meta.Name, Waiting: true, FirstTorrent: first}
		if err := s.fetchMetadata(p, addr); err != nil {
			return nil, err
		}
	} else {
		file, temp, err := s.torrentFileOf(ctx, addr, headers)
		if err != nil {
			return nil, err
		}
		p.file, p.temp = file, temp
		meta, err := b.Inspect(ctx, file, "")
		if err != nil {
			if temp {
				os.Remove(file)
			}
			return nil, err
		}
		p.state = probeState(meta)
		p.state.FirstTorrent = first
	}
	s.tmu.Lock()
	defer s.tmu.Unlock()
	if s.torrents == nil {
		s.torrents = map[string]*torrentProbe{}
	}
	if old := s.torrents[addr]; old != nil {
		s.tmu.Unlock()
		s.dropProbe(p) // another look got there first
		s.tmu.Lock()
		return old, nil
	}
	s.torrents[addr] = p
	return p, nil
}

func probeState(m engine.TorrentMeta) TorrentProbe {
	st := TorrentProbe{Name: m.Name, Size: m.Size, Private: m.Private}
	for _, f := range m.Files {
		st.Files = append(st.Files, TorrentFileRow{Path: f.Path, Size: f.Size, Pad: f.Pad})
		if !f.Pad {
			continue
		}
		st.Size -= f.Size
	}
	return st
}

// torrentFileOf is the .torrent at addr: the file itself, or ODM's copy of
// the one at an address (temp).
func (s *Service) torrentFileOf(ctx context.Context, addr string, headers map[string]string) (string, bool, error) {
	if !strings.HasPrefix(addr, "http") {
		return addr, false, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, addr, nil)
	if err != nil {
		return "", false, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := s.mgr.HTTPClient().Do(req)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("%s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20+1))
	if err != nil {
		return "", false, err
	}
	if len(data) > 16<<20 {
		return "", false, errors.New("the .torrent file is too large")
	}
	dir := filepath.Join(engine.TorrentDir, "probes")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", false, err
	}
	f, err := os.CreateTemp(dir, "*.torrent")
	if err != nil {
		return "", false, err
	}
	_, err = f.Write(data)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(f.Name())
		return "", false, err
	}
	return f.Name(), true, nil
}

// fetchMetadata holds magnet in odm-libtorrent, downloading nothing, until
// its metadata comes, which then fills p.
func (s *Service) fetchMetadata(p *torrentProbe, magnet string) error {
	b := engine.Torrents
	var id [6]byte
	rand.Read(id[:])
	p.key = "probe-" + hex.EncodeToString(id[:])
	p.resume = filepath.Join(engine.TorrentDir, p.key+".resume")
	p.save = filepath.Join(engine.TorrentDir, "probes", p.key)
	ctx, cancel := context.WithTimeout(context.Background(), probeLife)
	p.cancel, p.done = cancel, make(chan struct{})
	events, stop := b.Watch(p.key)
	if err := b.Add(ctx, engine.TorrentAdd{ID: p.key, Magnet: magnet, Hold: true, SavePath: p.save, Resume: p.resume}); err != nil {
		stop()
		cancel()
		close(p.done)
		return err
	}
	go func() {
		defer close(p.done)
		defer stop()
		for {
			select {
			case <-ctx.Done():
				rctx, rcancel := context.WithTimeout(context.Background(), 15*time.Second)
				_ = b.Remove(rctx, p.key, true)
				rcancel()
				os.RemoveAll(p.save)
				os.Remove(p.resume)
				return
			case ev := <-events:
				p.mu.Lock()
				switch ev.Kind {
				case engine.TorrentEventStatus:
					p.state.Peers = ev.Status.Peers
				case engine.TorrentEventMetadata:
					if ev.Meta != nil && p.state.Waiting {
						first := p.state.FirstTorrent
						p.state = probeState(*ev.Meta)
						p.state.FirstTorrent = first
					}
				case engine.TorrentEventError:
					p.state.Error = ev.Err
				}
				p.mu.Unlock()
			}
		}
	}()
	return nil
}

// dropProbe ends a look: its held torrent leaves odm-libtorrent and ODM's
// copy of the .torrent goes.
func (s *Service) dropProbe(p *torrentProbe) {
	if p.cancel != nil {
		p.cancel()
		<-p.done
	}
	if p.temp {
		os.Remove(p.file)
	}
}

// TorrentProbe returns what the window knows of addr's torrent now, for a
// magnet link waiting for its metadata.
func (s *Service) TorrentProbe(address string) (TorrentProbe, error) {
	s.tmu.Lock()
	p := s.torrents[strings.TrimSpace(address)]
	s.tmu.Unlock()
	if p == nil {
		return TorrentProbe{}, errors.New("not a torrent being read")
	}
	return p.snapshot(), nil
}

// DropTorrentProbe ends the look at addr's torrent: the window closed or
// the address changed.
func (s *Service) DropTorrentProbe(address string) {
	s.tmu.Lock()
	p := s.torrents[strings.TrimSpace(address)]
	delete(s.torrents, strings.TrimSpace(address))
	s.tmu.Unlock()
	if p != nil {
		go s.dropProbe(p)
	}
}

// takeProbe hands what addr's look read to a new download (o), and ends the
// look: a magnet link's held torrent must leave odm-libtorrent before the
// download adds the same torrent. done deletes what o was given, once the
// download has its copies.
func (s *Service) takeProbe(addr string, o *engine.AddOptions, skip []int, prio map[int]int) (done func()) {
	s.tmu.Lock()
	p := s.torrents[addr]
	delete(s.torrents, addr)
	s.tmu.Unlock()
	if p == nil {
		return func() {}
	}
	st := p.snapshot()
	if len(st.Files) > 0 {
		files := make([]engine.TorrentFile, len(st.Files))
		for i, f := range st.Files {
			files[i] = engine.TorrentFile{Path: f.Path, Size: f.Size, Pad: f.Pad}
		}
		for _, i := range skip {
			if i >= 0 && i < len(files) {
				files[i].Skip = true
			}
		}
		for i, p := range prio {
			if i >= 0 && i < len(files) && (p == engine.PriorityLow || p == engine.PriorityHigh) {
				files[i].Priority = p
			}
		}
		o.Torrent.Files = files
	}
	var temps []string
	if p.file != "" {
		o.TorrentFile = p.file
		if p.temp {
			temps = append(temps, p.file)
			p.temp = false // kept until the download has its copy
		}
	}
	if p.key != "" && !st.Waiting {
		// The held torrent wrote its metadata here; a copy outlives it.
		if data, err := os.ReadFile(p.resume); err == nil {
			if f, err := os.CreateTemp(filepath.Join(engine.TorrentDir, "probes"), "*.resume"); err == nil {
				f.Write(data)
				f.Close()
				o.TorrentResume = f.Name()
				temps = append(temps, f.Name())
			}
		}
	}
	s.dropProbe(p)
	return func() {
		for _, t := range temps {
			os.Remove(t)
		}
	}
}

// StopSeeding ends the seeding of the finished torrent id.
func (s *Service) StopSeeding(id string) {
	s.mgr.StopSeeding(id)
	s.poke()
}

// SetTorrentFiles changes which files of the torrent id download and how
// soon, by index: 0 leaves a file out, else engine.PriorityLow,
// PriorityNormal or PriorityHigh.
func (s *Service) SetTorrentFiles(id string, priorities map[int]int) error {
	if err := s.mgr.SetTorrentFiles(id, priorities); err != nil {
		return err
	}
	s.poke()
	return nil
}

// TorrentMagnet is a magnet link to the torrent id, for the clipboard; ""
// when it has none yet.
func (s *Service) TorrentMagnet(id string) string {
	tj, ok := s.mgr.Torrent(id)
	if !ok {
		return ""
	}
	d, _ := s.mgr.Get(id)
	return tj.MagnetLink(d.FileName)
}

// OpenTorrentFile asks for a .torrent file and opens the New Download
// window for it.
func (s *Service) OpenTorrentFile() {
	w := s.opener()
	if w == nil {
		return
	}
	if p := w.pickOpen("Torrent files", "*.torrent"); p != "" {
		s.openTorrentPaths([]string{p})
	}
}

// openTorrentPaths opens a New Download window for each .torrent file
// among paths (dropped on the main window, or picked).
func (s *Service) openTorrentPaths(paths []string) {
	w := s.opener()
	if w == nil {
		return
	}
	for _, p := range paths {
		if engine.IsTorrentFile(p) {
			w.openAdd(s.keepPrefill(engine.AddOptions{URL: p}))
		}
	}
}

// TorrentDetail is a torrent as its progress window shows it.
type TorrentDetail struct {
	Seeding     bool    `json:"seeding"`
	Peers       int     `json:"peers"`
	Seeds       int     `json:"seeds"`
	UploadSpeed int64   `json:"uploadSpeed"`
	Uploaded    int64   `json:"uploaded"`
	Ratio       float64 `json:"ratio"`
	// Pieces is base64 of a bitfield, a bit a piece the torrent has, the
	// first in the high bit; PieceCount is how many there are. 0 when the
	// torrent does not run.
	Pieces     string              `json:"pieces"`
	PieceCount int                 `json:"pieceCount"`
	Files      []TorrentFileDetail `json:"files"`
	Magnet     string              `json:"magnet"`
}

// TorrentFileDetail is a torrent's file with how much of it is there, and
// how soon it downloads: Priority is 0 when it is left out (Skip), else
// engine.PriorityLow, PriorityNormal or PriorityHigh. Index is its place in
// the torrent, which SetTorrentFiles names it by.
type TorrentFileDetail struct {
	Index    int    `json:"index"`
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	Done     int64  `json:"done"`
	Skip     bool   `json:"skip"`
	Priority int    `json:"priority"`
}

func (s *Service) torrentDetail(d engine.Info) *TorrentDetail {
	tj, ok := s.mgr.Torrent(d.ID)
	if !ok {
		return nil
	}
	out := &TorrentDetail{Seeding: d.Seeding, Peers: d.Peers, Seeds: d.Seeds, UploadSpeed: d.UploadSpeed,
		Uploaded: d.Uploaded, Magnet: tj.MagnetLink(d.FileName)}
	if d.Total > 0 {
		out.Ratio = float64(d.Uploaded) / float64(d.Total)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	det, live := s.mgr.TorrentDetail(ctx, d.ID)
	if live {
		out.Pieces, out.PieceCount = base64.StdEncoding.EncodeToString(det.Pieces), det.Count
	}
	for i, f := range tj.Files {
		if f.Pad {
			continue
		}
		name := f.Name
		if name == "" {
			name = f.Path
		}
		fd := TorrentFileDetail{Index: i, Name: name, Size: f.Size, Skip: f.Skip, Priority: f.Prio()}
		switch {
		case live && i < len(det.FileDone):
			fd.Done = det.FileDone[i]
		case d.Status == engine.StatusComplete && !f.Skip:
			fd.Done = f.Size
		}
		out.Files = append(out.Files, fd)
	}
	return out
}

// torrentType says whether a server's type for a file is a .torrent's
// that is to download as a torrent.
func (s *Service) torrentType(contentType string) bool {
	return !s.mgr.Settings().Torrent.NoTorrentLinks &&
		strings.HasPrefix(strings.ToLower(strings.TrimSpace(contentType)), "application/x-bittorrent")
}

// hasTorrentProbe reports whether the window looked at addr as a torrent.
func (s *Service) hasTorrentProbe(addr string) bool {
	s.tmu.Lock()
	defer s.tmu.Unlock()
	return s.torrents[addr] != nil
}

// createTorrent adds the torrent the form describes.
func (s *Service) createTorrent(r CreateRequest, addr string) (CreateResult, error) {
	// The same torrent first, by its info hash: from another address too,
	// and it says whether it can be downloaded again.
	same := engine.AddOptions{URL: addr, Torrent: &engine.TorrentJob{}, TorrentFile: s.probeFile(addr)}
	if existing, free, dup := s.mgr.FindTorrent(same); dup && (!r.Again || !free) {
		it := itemOf(existing)
		return CreateResult{Duplicate: &it, SameTorrent: true, NoAgain: !free}, nil
	}
	if existing, dup := s.mgr.FindByURL(addr); dup && !r.Again {
		it := itemOf(existing)
		return CreateResult{Duplicate: &it}, nil
	}
	folder := strings.TrimSpace(r.Folder)
	o := engine.AddOptions{URL: addr, Dir: folder, Description: strings.TrimSpace(r.Description),
		Category: r.Category, Queue: r.Queue, Start: r.Start, Headers: s.prefill(r.Token).Headers,
		Torrent: &engine.TorrentJob{Name: engine.SanitizeName(r.FileName)}}
	done := s.takeProbe(addr, &o, r.Skip, r.Priorities)
	info := s.mgr.Create(o)
	done()
	s.mgr.NoteSaveFolder(folder)
	if r.Token != "" {
		s.DropPrefill(r.Token)
	}
	s.poke()
	if w := s.opener(); r.Start && !s.mgr.Settings().NoProgressDialog && w != nil {
		w.openProgress(info.ID, true)
	}
	return CreateResult{ID: info.ID}, nil
}
