package gui

import (
	"context"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"odm/internal/engine"
)

// The progress window (one per download) and the download complete window,
// as the Fyne UI's (internal/ui/dialogs_desktop.go: openProgressWindow,
// showComplete).

// windowOpener opens ODM's other windows; the app supplies it (windows.go)
// and tests leave it nil.
type windowOpener interface {
	openAdd(token string)
	openProgress(id string)
	closeProgress(id string)
	openComplete(id string)
	openOptions(tab string)
	openScheduler(newQueue bool)
	// openNotice shows the scheduler's notices: the download limit
	// reached, or the countdown before a finish action.
	openNotice(q url.Values)
	pickFolder(current string) string
	pickFile(current string) string
	pickOpen(filterName, pattern string) string
	pickSave(name string) string
}

// opener is the app's windowOpener, nil before the window exists and in
// tests.
func (s *Service) opener() windowOpener {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.windows
}

// Chunk is one connection's part of a file.
type Chunk struct {
	Start int64  `json:"start"`
	End   int64  `json:"end"`
	Done  int64  `json:"done"`
	State string `json:"state"`
}

// Detail is one download as its progress window shows it.
type Detail struct {
	Item         Item    `json:"item"`
	Path         string  `json:"path"` // where the file is saved, "" before it has a name
	Checksum     string  `json:"checksum"`
	ChecksumName string  `json:"checksumName"` // "SHA-256", for a checksum to check
	Resumable    bool    `json:"resumable"`
	SpeedLimit   int64   `json:"speedLimit"` // this download's own, 0 for none
	OpenWhenDone bool    `json:"openWhenDone"`
	Segments     int     `json:"segments"`
	SegmentsDone int     `json:"segmentsDone"`
	Chunks       []Chunk `json:"chunks"`
	// CompleteDialog is the setting "Show the download complete dialog".
	CompleteDialog bool `json:"completeDialog"`
}

// Detail returns the download id for its progress window.
func (s *Service) Detail(id string) (Detail, error) {
	d, ok := s.mgr.Get(id)
	if !ok {
		return Detail{}, errNoDownload
	}
	s.hmu.Lock()
	it := s.itemLocked(d)
	s.hmu.Unlock()
	out := Detail{
		Item: it, Checksum: d.Checksum, Resumable: d.RangeOK, SpeedLimit: d.SpeedLimit,
		ChecksumName: engine.ChecksumName(d.Checksum),
		OpenWhenDone: d.OpenWhenDone, Segments: d.Segments, SegmentsDone: d.SegmentsDone,
		CompleteDialog: !s.mgr.Settings().NoCompleteDialog,
	}
	if d.FileName != "" {
		out.Path = filepath.Join(d.Dir, d.FileName)
	}
	for _, c := range d.Chunks {
		out.Chunks = append(out.Chunks, Chunk{Start: c.Start, End: c.End, Done: c.Done, State: c.State})
	}
	return out, nil
}

// SetDownloadLimit caps the download id's own speed; 0 lifts the cap.
func (s *Service) SetDownloadLimit(id string, bytesPerSecond int64) {
	s.mgr.SetSpeedLimit(id, max(0, bytesPerSecond))
	s.poke()
}

// SetOpenWhenDone has the download id's file opened once it finishes.
func (s *Service) SetOpenWhenDone(id string, on bool) {
	s.mgr.SetOpenWhenDone(id, on)
}

// SetCompleteDialog turns the download complete window on or off.
func (s *Service) SetCompleteDialog(on bool) {
	next := s.mgr.Settings()
	if next.NoCompleteDialog == on {
		next.NoCompleteDialog = !on
		s.mgr.UpdateSettings(next)
	}
}

// OpenProgress opens (or brings forward) the download id's progress window.
func (s *Service) OpenProgress(id string) {
	if w := s.opener(); w != nil {
		w.openProgress(id)
	}
}

// PickFolder asks for a folder, starting at current; "" when cancelled.
func (s *Service) PickFolder(current string) string {
	w := s.opener()
	if w == nil {
		return ""
	}
	return w.pickFolder(current)
}

// PickFile asks for a program file (ffmpeg, yt-dlp), starting where
// current is; "" when cancelled.
func (s *Service) PickFile(current string) string {
	w := s.opener()
	if w == nil {
		return ""
	}
	return w.pickFile(current)
}

// Readdress points the unfinished download id at a new address of the same
// file, for links that expire.
func (s *Service) Readdress(id, address string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err := s.mgr.Readdress(ctx, id, strings.TrimSpace(address))
	s.poke()
	return err
}

// noticeFinished closes the progress window of each download that just
// finished and opens the download complete window in its place, unless
// that is turned off. The first look only learns the statuses.
func (s *Service) noticeFinished(list []engine.Info) {
	s.amu.Lock()
	first := s.statuses == nil
	prev := s.statuses
	s.statuses = make(map[string]engine.Status, len(list))
	var done []string
	for _, d := range list {
		s.statuses[d.ID] = d.Status
		if was, seen := prev[d.ID]; !first && seen && was != d.Status && d.Status == engine.StatusComplete {
			done = append(done, d.ID)
		}
	}
	s.amu.Unlock()
	w := s.opener()
	if len(done) == 0 || w == nil {
		return
	}
	show := !s.mgr.Settings().NoCompleteDialog
	for _, id := range done {
		w.closeProgress(id)
		if show {
			w.openComplete(id)
		}
	}
}
