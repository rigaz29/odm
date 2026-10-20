package engine

// Torrent downloads: a .torrent file or a magnet link. ODM's companion
// program odm-libtorrent does the transfer (package torrent,
// helper/libtorrent); the engine reaches it only through TorrentBackend,
// which the program sets, and so stays on the standard library.
//
// The engine decides where every file goes and the backend applies it:
//   - a torrent of one file downloads to <dir>/<name>.odmpart, which is
//     renamed to <name> once complete;
//   - a torrent of several files downloads into the folder
//     <dir>/<name>.odmpart, without the torrent's own top folder, and the
//     folder is moved to <dir>/<name> once complete.
//
// <name> is the torrent's name, sanitized and made unique in <dir>; every
// path inside the folder is sanitized part by part, so a torrent cannot
// write outside it. <dir> is the download's folder, below its category's
// when categories are on; the category is the largest file's.
//
// A finished torrent is complete at once and then seeds, outside the
// queue's slots, until Settings.Torrent's target: the Seeding mark. It
// leaves the session once that is reached or StopSeeding is asked, and
// seeds again after ODM starts again if it did not get there.
//
// Trackers and web seeds on this computer or its network are left out of a
// torrent that did not come from there, as the playlists' guard does: a
// torrent from the internet cannot make ODM reach the routers and services
// next to it. Peers on the network stay: finding them is BitTorrent's.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"time"
)

// TorrentBackend runs the torrents of torrent downloads, each named by its
// download's ID. Package torrent implements it on odm-libtorrent.
type TorrentBackend interface {
	// Inspect reads a .torrent file, or a magnet link (which names no
	// files), without adding it.
	Inspect(ctx context.Context, file, magnet string) (TorrentMeta, error)
	// Add adds a torrent, or resumes it when it is in the session already.
	Add(ctx context.Context, a TorrentAdd) error
	// Watch delivers the torrent's metadata, status, finished and error
	// events until stop is called. Status events may be dropped when the
	// reader falls behind.
	Watch(id string) (events <-chan TorrentEvent, stop func())
	Pause(ctx context.Context, id string) error
	// SetFiles names the files and gives their priorities: once the
	// metadata of a torrent added with Hold is in (it returns when they are
	// renamed), or with no names when the files to download change.
	SetFiles(ctx context.Context, id string, names map[int]string, priorities []int) error
	// Rename and Move return once the files are renamed or moved.
	Rename(ctx context.Context, id string, names map[int]string) error
	Move(ctx context.Context, id, savePath string) error
	SetLimits(ctx context.Context, id string, download, upload int64) error
	// Reconfigure applies ODM's settings to the session again: they
	// changed.
	Reconfigure()
	// Detail is what a running torrent's progress window shows.
	Detail(ctx context.Context, id string) (TorrentDetail, error)
	// Remove takes the torrent out of the session, deleting what it
	// downloaded when deleteFiles is set, and returns once it is gone. A
	// torrent that is not in the session is not an error.
	Remove(ctx context.Context, id string, deleteFiles bool) error
}

// Torrents runs torrents; nil where ODM cannot (tests that need none). The
// program sets it.
var Torrents TorrentBackend

// TorrentDir keeps each torrent download's .torrent file and resume data,
// as <id>.torrent and <id>.resume. The program sets it.
var TorrentDir string

// TorrentAdd is a torrent to add.
type TorrentAdd struct {
	ID string
	// File is a .torrent file; Magnet a magnet link.
	File, Magnet string
	// Resume is the file that keeps the torrent's resume data.
	Resume   string
	SavePath string
	// Names gives files other paths, relative to SavePath, by index. It
	// needs the metadata: a File, or resume data that has it.
	Names map[int]string
	// Priorities gives each file's priority: 0 skips it, then PriorityLow,
	// PriorityNormal or PriorityHigh. A torrent in the session already takes
	// them too.
	Priorities []int
	// Hold makes a magnet link download nothing until SetFiles.
	Hold          bool
	DownloadLimit int64
	// DropTrackers and DropWebSeeds are left out of the torrent's own.
	DropTrackers, DropWebSeeds []string
}

// TorrentMeta describes a torrent.
type TorrentMeta struct {
	Name     string
	Size     int64
	InfoHash string // v1, in hex
	// InfoHashV2 is the v2 info hash of a v2 or hybrid torrent.
	InfoHashV2 string
	Private    bool
	Trackers   []string
	WebSeeds   []string
	Magnet     string
	// Files are the torrent's files in its own order, with their paths in
	// the torrent (Path); Name is left to the engine.
	Files []TorrentFile
}

// TorrentStatus is how far a torrent is.
type TorrentStatus struct {
	State  string // "checking", "metadata", "downloading", "finished", "seeding", …
	Paused bool
	// Size and Done are the bytes wanted and those of them downloaded and
	// checked.
	Size, Done               int64
	DownloadRate, UploadRate int64
	Uploaded                 int64
	Peers, Seeds             int
	// SeedingTime is how long it seeded, in seconds.
	SeedingTime int64
	Error       string
}

// TorrentDetail is a torrent's pieces and files, for its progress window.
type TorrentDetail struct {
	// Pieces has a bit for each piece the torrent has, the first piece in
	// the high bit of the first byte; Count is how many pieces it has.
	Pieces []byte
	Count  int
	// FileDone is each file's bytes downloaded.
	FileDone []int64
}

// Kinds of TorrentEvent.
const (
	TorrentEventMetadata = "metadata"
	TorrentEventStatus   = "status"
	// TorrentEventFinished: every wanted file is downloaded, checked and on
	// the disk.
	TorrentEventFinished = "finished"
	TorrentEventError    = "error"
)

// TorrentEvent is something that happened to a torrent.
type TorrentEvent struct {
	Kind   string
	Meta   *TorrentMeta  // metadata
	Status TorrentStatus // status
	Err    string        // error
}

// TorrentJob makes a download a torrent. URL is the magnet link, or where
// the .torrent file came from.
type TorrentJob struct {
	// Magnet is the magnet link it was added from; "" for a .torrent file,
	// kept as <id>.torrent in TorrentDir.
	Magnet string `json:"magnet,omitempty"`
	// Name is the name chosen for it before its metadata came, "" for the
	// torrent's own.
	Name       string `json:"name,omitempty"`
	InfoHash   string `json:"info_hash,omitempty"`
	InfoHashV2 string `json:"info_hash_v2,omitempty"`
	// Folder: the torrent has several files, and FileName is a folder.
	Folder bool `json:"folder,omitempty"`
	// Files are the torrent's files once its metadata is known.
	Files []TorrentFile `json:"files,omitempty"`
	// Done is the bytes downloaded when the state was saved.
	Done int64 `json:"done,omitempty"`
	// Seeding: finished, and uploading until the seeding target.
	Seeding bool `json:"seeding,omitempty"`
	// Uploaded is the bytes uploaded over the torrent's life; SeedTime the
	// seconds it seeded.
	Uploaded int64 `json:"uploaded,omitempty"`
	SeedTime int64 `json:"seed_time,omitempty"`
	// Seed is the torrent's own seeding target, over Settings.Torrent's;
	// nil follows those.
	Seed *TorrentSeed `json:"seed,omitempty"`
	// Final: the files have their own names in their place, as the torrent
	// finished once. Files added afterwards download there too.
	Final bool `json:"final,omitempty"`
}

// TorrentSeed is a torrent's own seeding target: Mode is SeedTarget,
// SeedNever or SeedForever, as TorrentSettings.Seed; Ratio (percent) and
// Minutes are the target's, 0 taking Settings.Torrent's.
type TorrentSeed struct {
	Mode    string `json:"mode,omitempty"`
	Ratio   int    `json:"ratio,omitempty"`
	Minutes int    `json:"minutes,omitempty"`
}

// TorrentFile is a file of a torrent.
type TorrentFile struct {
	// Path is its path in the torrent, with /.
	Path string `json:"path"`
	// Name is where it goes in a folder torrent, relative to the folder,
	// with /.
	Name string `json:"name,omitempty"`
	Size int64  `json:"size"`
	// Skip: not downloaded. Pad: padding between files, never a file.
	Skip bool `json:"skip,omitempty"`
	Pad  bool `json:"pad,omitempty"`
	// Priority is how soon its pieces are fetched, PriorityLow or
	// PriorityHigh; 0 is PriorityNormal.
	Priority int `json:"priority,omitempty"`
}

// A file's priorities, as libtorrent counts them; 0 leaves it out.
const (
	PriorityLow    = 1
	PriorityNormal = 4
	PriorityHigh   = 7
)

// Prio is the file's priority: 0 when it is left out.
func (f TorrentFile) Prio() int {
	switch {
	case f.Skip || f.Pad:
		return 0
	case f.Priority == PriorityLow || f.Priority == PriorityHigh:
		return f.Priority
	}
	return PriorityNormal
}

// torrentPolicy is Settings.Torrent, for the downloads to read.
var torrentPolicy atomic.Pointer[TorrentSettings]

func setTorrentSettings(ts TorrentSettings) { torrentPolicy.Store(&ts) }

func torrentSettings() TorrentSettings {
	if p := torrentPolicy.Load(); p != nil {
		return *p
	}
	return TorrentSettings{SeedRatio: DefaultSeedRatio, SeedMinutes: DefaultSeedMinutes}
}

// seedPolicy is the seeding target the download's torrent goes by: the
// Options', with the torrent's own over them.
func (d *Download) seedPolicy() TorrentSettings {
	ts := torrentSettings()
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.Torrent == nil || d.Torrent.Seed == nil {
		return ts
	}
	s := d.Torrent.Seed
	ts.Seed = s.Mode
	if s.Ratio > 0 {
		ts.SeedRatio = s.Ratio
	}
	if s.Minutes > 0 {
		ts.SeedMinutes = s.Minutes
	}
	return ts
}

// seedMinute is a minute of SeedMinutes, and seedCheck how often a seeding
// torrent with no news is held against its target; tests shorten them.
var (
	seedMinute = time.Minute
	seedCheck  = 30 * time.Second
)

// seeded reports whether a torrent of size bytes that uploaded uploaded
// bytes, and has seeded for seeding, has seeded enough under ts.
func seeded(ts TorrentSettings, size, uploaded int64, seeding time.Duration) bool {
	switch ts.Seed {
	case SeedNever:
		return true
	case SeedForever:
		return false
	}
	if size <= 0 || uploaded*100 >= size*int64(ts.SeedRatio) {
		return true
	}
	return seeding >= time.Duration(ts.SeedMinutes)*seedMinute
}

// localURL reports whether raw names this computer or a private network by
// itself: a local name, or an address literal (in any form the system's
// resolver reads) that is not public. A name is not looked up.
func localURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return false
	}
	host := canonicalHost(u.Hostname())
	if localName(host) {
		return true
	}
	if ip, ok := literalIP(host); ok {
		return localAddr(ip)
	}
	return false
}

// torrentDrops lists the trackers and web seeds of m that a torrent from
// source must not use: those on this computer or its network, unless the
// .torrent came from there too.
func torrentDrops(source string, m TorrentMeta) (trackers, webSeeds []string) {
	if u, err := url.Parse(source); err == nil && (u.Scheme == "http" || u.Scheme == "https") && localURL(source) {
		return nil, nil
	}
	for _, t := range m.Trackers {
		if localURL(t) {
			trackers = append(trackers, t)
		}
	}
	for _, w := range m.WebSeeds {
		if localURL(w) {
			webSeeds = append(webSeeds, w)
		}
	}
	return trackers, webSeeds
}

// errNoTorrents is a torrent download's error where ODM cannot run torrents.
var errNoTorrents = errors.New("this ODM cannot download torrents")

// MaxTorrentFile bounds a .torrent file.
const MaxTorrentFile = 16 << 20

// IsMagnet reports whether s is a magnet link.
func IsMagnet(s string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(s)), "magnet:?")
}

// IsTorrentFile reports whether s, an address or a path, names a .torrent
// file.
func IsTorrentFile(s string) bool {
	s = strings.TrimSpace(s)
	if u, err := url.Parse(s); err == nil && (u.Scheme == "http" || u.Scheme == "https") {
		s = u.Path
	}
	return strings.HasSuffix(strings.ToLower(s), ".torrent")
}

// magnetName is the name a magnet link gives its torrent (dn=), else its
// info hash, for the list until the metadata comes in.
func magnetName(link string) string {
	s := strings.TrimSpace(link)
	if len(s) < len("magnet:?") {
		return "torrent"
	}
	q, err := url.ParseQuery(s[len("magnet:?"):])
	if err != nil {
		return "torrent"
	}
	if n := SanitizeName(q.Get("dn")); n != "" {
		return n
	}
	for _, xt := range q["xt"] {
		if i := strings.LastIndexByte(xt, ':'); i >= 0 && i+1 < len(xt) {
			return SanitizeName(xt[i+1:])
		}
	}
	return "torrent"
}

func torrentPath(id, ext string) string {
	return filepath.Join(TorrentDir, id+ext)
}

// MagnetLink is a magnet link to the torrent: the one it came from, else
// one made of its info hashes and name. "" before its metadata.
func (tj *TorrentJob) MagnetLink(name string) string {
	if tj.Magnet != "" {
		return tj.Magnet
	}
	if !tj.adopted() {
		return ""
	}
	q := url.Values{}
	if tj.InfoHash != "" {
		q.Add("xt", "urn:btih:"+tj.InfoHash)
	}
	if tj.InfoHashV2 != "" {
		q.Add("xt", "urn:btmh:1220"+tj.InfoHashV2) // multihash: SHA-256, 32 bytes
	}
	if name != "" {
		q.Set("dn", name)
	}
	return "magnet:?" + strings.NewReplacer("%3A", ":").Replace(q.Encode())
}

// torrentKey is the info hashes the download's torrent is known by: its
// metadata's, else its magnet link's, else its kept .torrent file's, with
// where it is. ok is false for a download that is no torrent, or whose
// hashes are not known yet.
func (d *Download) torrentKey() (k torrentKey, ok bool) {
	d.mu.Lock()
	if d.Torrent == nil {
		d.mu.Unlock()
		return k, false
	}
	k = torrentKey{v1: d.Torrent.InfoHash, v2: d.Torrent.InfoHashV2, inSession: d.inSession,
		mayRun: d.Status != StatusComplete || d.Torrent.Seeding}
	id, magnet := d.ID, d.Torrent.Magnet
	d.mu.Unlock()
	switch {
	case k.v1 != "" || k.v2 != "":
	case magnet != "":
		k.v1, k.v2 = MagnetHashes(magnet)
	case TorrentDir != "":
		k.v1, k.v2 = torrentFileHashes(torrentPath(id, ".torrent"))
	}
	return k, k.v1 != "" || k.v2 != ""
}

// torrentKey is a torrent download as Download.torrentKey reads it.
// inSession: odm-libtorrent holds it. mayRun: it may run its torrent, now or
// later (not complete, or seeding), so another of the same torrent is not
// added beside it.
type torrentKey struct {
	v1, v2            string
	inSession, mayRun bool
}

// setInSession records whether odm-libtorrent holds the download's torrent.
func (d *Download) setInSession(on bool) {
	d.mu.Lock()
	d.inSession = on
	d.mu.Unlock()
}

// adopted reports whether the torrent's metadata is in (adoptTorrent):
// Files may be there before, chosen with the torrent.
func (tj *TorrentJob) adopted() bool { return tj.InfoHash != "" || tj.InfoHashV2 != "" }

// largest is the largest wanted file's path, which gives a torrent its
// category; "" before the metadata.
func (tj *TorrentJob) largest() string {
	best, size := "", int64(-1)
	for _, f := range tj.Files {
		if !f.Pad && !f.Skip && f.Size > size {
			best, size = f.Path, f.Size
		}
	}
	return best
}

// wanted is the bytes of the files downloaded.
func (tj *TorrentJob) wanted() int64 {
	var n int64
	for _, f := range tj.Files {
		if !f.Pad && !f.Skip {
			n += f.Size
		}
	}
	return n
}

// runTorrent is Start for a torrent.
func (d *Download) runTorrent(ctx context.Context) {
	b := Torrents
	if b == nil {
		d.fail(errNoTorrents)
		return
	}
	err := d.transferTorrent(ctx, b)
	switch {
	case err == nil:
	case d.isRemoved():
		// RemoveWithFile takes it out of the session.
	case ctx.Err() != nil:
		pctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_ = b.Pause(pctx, d.ID)
		cancel()
		d.mu.Lock()
		d.Status = StatusPaused
		d.mu.Unlock()
	default:
		pctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_ = b.Pause(pctx, d.ID)
		cancel()
		if errors.Is(err, errHelperStopped) {
			d.setInSession(false) // a new odm-libtorrent holds nothing
		}
		d.fail(err)
	}
}

func (d *Download) transferTorrent(ctx context.Context, b TorrentBackend) error {
	d.mu.Lock()
	id, magnet, source := d.ID, d.Torrent.Magnet, d.URL
	known := d.Torrent.adopted()
	limit := d.SpeedLimit
	d.mu.Unlock()

	file := ""
	if magnet == "" {
		p, err := d.torrentFile(ctx)
		if err != nil {
			return err
		}
		file = p
	}
	// Read every time: it gives the trackers and web seeds to leave out.
	meta, err := b.Inspect(ctx, file, magnet)
	if err != nil {
		return fmt.Errorf("reading the torrent: %w", err)
	}
	// odm-libtorrent runs a torrent once: a second download of it waits
	// for the first to be complete and done seeding, or removed.
	if d.twin != nil {
		if name := d.twin(id, meta.InfoHash, meta.InfoHashV2); name != "" {
			return fmt.Errorf("%w: %s", errTwinTorrent, name)
		}
	}
	if !known && file != "" {
		if err := d.adoptTorrent(meta); err != nil {
			return err
		}
		known = true
	}

	events, stop := b.Watch(id)
	defer stop()
	resume := torrentPath(id, ".resume")
	add := TorrentAdd{ID: id, File: file, Magnet: magnet, Resume: resume, DownloadLimit: limit}
	add.DropTrackers, add.DropWebSeeds = torrentDrops(source, meta)
	if known {
		add.SavePath, add.Names, add.Priorities = d.torrentLayout(false)
	} else {
		// A magnet link before its metadata: nothing is written until the
		// files have their names.
		add.SavePath, add.Hold = d.torrentDir(), true
	}
	err = b.Add(ctx, add)
	if err != nil && known && file == "" {
		// A magnet link whose resume data lost the metadata: fetch it
		// again, then name the files as before.
		add.Names, add.Priorities, add.SavePath, add.Hold = nil, nil, d.torrentDir(), true
		err = b.Add(ctx, add)
	}
	if err != nil {
		return err
	}
	d.setInSession(true)
	d.notify(true)

	// downloading: a status showed bytes still missing. A torrent that
	// was complete when added (ODM stopped while finishing it) gets no
	// finished event; its first status tells. One given more files after
	// it finished (placed) may still report what it wanted before: its
	// status tells only once it counts them (want).
	downloading := false
	d.mu.Lock()
	placed, want := d.Torrent.Final, d.Torrent.wanted()
	d.mu.Unlock()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ev, ok := <-events:
			if !ok {
				return errHelperStopped
			}
			switch ev.Kind {
			case TorrentEventMetadata:
				if !add.Hold || ev.Meta == nil {
					continue
				}
				if !known {
					if err := d.adoptTorrent(*ev.Meta); err != nil {
						return err
					}
					known = true
				}
				save, names, prio := d.torrentLayout(false)
				if save != add.SavePath {
					if err := b.Move(ctx, id, save); err != nil {
						return err
					}
				}
				if err := b.SetFiles(ctx, id, names, prio); err != nil {
					return err
				}
				add.Hold = false
				d.notify(true)
			case TorrentEventStatus:
				st := ev.Status
				if st.Error != "" {
					return errors.New(st.Error)
				}
				d.torrentProgress(st)
				if add.Hold || st.Size == 0 {
					continue
				}
				if st.Done < st.Size {
					downloading = true
				} else if !downloading && (st.State == "finished" || st.State == "seeding") && (!placed || st.Size >= want) {
					return d.finishTorrent(ctx, b)
				}
			case TorrentEventFinished:
				if !add.Hold {
					return d.finishTorrent(ctx, b)
				}
			case TorrentEventError:
				return errors.New(ev.Err)
			}
		}
	}
}

// torrentFile is the download's .torrent file in TorrentDir, fetched from
// its address or copied from its path the first time.
func (d *Download) torrentFile(ctx context.Context) (string, error) {
	p := torrentPath(d.ID, ".torrent")
	if st, err := os.Stat(p); err == nil && st.Size() > 0 {
		return p, nil
	}
	d.mu.Lock()
	src, headers := d.URL, d.Headers
	d.mu.Unlock()
	var data []byte
	if u, err := url.Parse(src); err == nil && (u.Scheme == "http" || u.Scheme == "https") {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, src, nil)
		if err != nil {
			return "", err
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, err := d.httpClient().Do(req)
		if err != nil {
			return "", fmt.Errorf("fetching the .torrent file: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("fetching the .torrent file: %s", resp.Status)
		}
		data, err = io.ReadAll(io.LimitReader(resp.Body, MaxTorrentFile+1))
		if err != nil {
			return "", fmt.Errorf("fetching the .torrent file: %w", err)
		}
	} else {
		f, err := os.Open(src)
		if err != nil {
			return "", err
		}
		data, err = io.ReadAll(io.LimitReader(f, MaxTorrentFile+1))
		f.Close()
		if err != nil {
			return "", err
		}
	}
	if len(data) > MaxTorrentFile {
		return "", errors.New("the .torrent file is too large")
	}
	if err := os.MkdirAll(TorrentDir, 0o755); err != nil {
		return "", err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return "", err
	}
	return p, os.Rename(tmp, p)
}

// adoptTorrent takes in a torrent's metadata: the files, their names, the
// download's folder (from the category of the largest file) and a name in
// it that nothing else has.
func (d *Download) adoptTorrent(m TorrentMeta) error {
	d.mu.Lock()
	prev := d.Torrent.Files
	files := make([]TorrentFile, len(m.Files))
	folder := false
	for i, f := range m.Files {
		files[i] = TorrentFile{Path: f.Path, Size: f.Size, Pad: f.Pad}
		if len(prev) == len(m.Files) {
			files[i].Skip = prev[i].Skip // chosen before the metadata came
		}
		if !f.Pad && strings.Contains(f.Path, "/") {
			folder = true
		}
	}
	if folder {
		nameFiles(files)
	}
	d.Torrent.Files, d.Torrent.Folder = files, folder
	d.Torrent.InfoHash, d.Torrent.InfoHashV2 = m.InfoHash, m.InfoHashV2
	name := SanitizeName(d.Torrent.Name)
	if name == "" {
		name = SanitizeName(m.Name)
	}
	if name == "" {
		name = SanitizeName(m.InfoHash)
	}
	if name == "" {
		name = "torrent"
	}
	dir := d.targetDirLocked(d.category())
	d.mu.Unlock()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	var claimed string
	if folder {
		c, err := claimFolder(dir, name)
		if err != nil {
			return err
		}
		claimed = c
	} else {
		c, f, err := claimName(dir, name)
		if err != nil {
			return err
		}
		f.Close() // libtorrent writes it
		claimed = c
	}
	d.mu.Lock()
	d.Dir, d.FileName = dir, claimed
	d.Total = d.Torrent.wanted()
	d.mu.Unlock()
	d.notify(true)
	return nil
}

// nameFiles gives the files of a folder torrent their places in the folder:
// the torrent's path without its top folder, each part sanitized, and
// numbered when two come out the same (as Windows compares them).
func nameFiles(files []TorrentFile) {
	taken := map[string]bool{}
	for i := range files {
		f := &files[i]
		if f.Pad {
			continue
		}
		parts := strings.Split(f.Path, "/")
		if len(parts) > 1 {
			parts = parts[1:] // the torrent's own folder
		}
		var clean []string
		for _, p := range parts {
			if s := SanitizeName(p); s != "" {
				clean = append(clean, s)
			}
		}
		if len(clean) == 0 {
			clean = []string{fmt.Sprintf("file %d", i+1)}
		}
		name := strings.Join(clean, "/")
		for n := 2; taken[strings.ToLower(name)] && n < maxNameTries; n++ {
			last := len(clean) - 1
			name = strings.Join(append(append([]string(nil), clean[:last]...), numberedName(clean[last], n)), "/")
		}
		taken[strings.ToLower(name)] = true
		f.Name = name
	}
}

// claimFolder finds the first free folder name starting from name and
// creates its .odmpart folder, so a concurrent download cannot take it.
func claimFolder(dir, name string) (string, error) {
	for i := 1; i <= maxNameTries; i++ {
		cand := folderName(name, i)
		final := filepath.Join(dir, cand)
		if _, err := os.Stat(final); err == nil {
			continue
		}
		err := os.Mkdir(final+".odmpart", 0o755)
		if os.IsExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		return cand, nil
	}
	return "", fmt.Errorf("no free folder name for %q in %s", name, dir)
}

// freeFolder is the first numbered variant of name with neither a folder
// nor an .odmpart in dir.
func freeFolder(dir, name string) (string, error) {
	for i := 2; i <= maxNameTries; i++ {
		cand := folderName(name, i)
		_, e1 := os.Stat(filepath.Join(dir, cand))
		_, e2 := os.Stat(filepath.Join(dir, cand+".odmpart"))
		if os.IsNotExist(e1) && os.IsNotExist(e2) {
			return cand, nil
		}
	}
	return "", fmt.Errorf("no free folder name for %q in %s", name, dir)
}

// folderName is name for n == 1, else "name (n)": a folder has no
// extension to keep ("Ubuntu 24.04 (2)").
func folderName(name string, n int) string {
	if n <= 1 {
		return name
	}
	return fmt.Sprintf("%s (%d)", name, n)
}

// torrentDir is the folder the download goes to, as far as it is known.
func (d *Download) torrentDir() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.targetDirLocked(d.category())
}

// torrentLayout is where the backend puts the files: while they download,
// or once they are finished (final).
func (d *Download) torrentLayout(final bool) (save string, names map[int]string, prio []int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	tj := d.Torrent
	part := ".odmpart"
	if final || tj.Final {
		part = ""
	}
	names = map[int]string{}
	prio = make([]int, len(tj.Files))
	for i, f := range tj.Files {
		if f.Pad {
			continue
		}
		prio[i] = f.Prio()
		if tj.Folder {
			names[i] = f.Name
		} else {
			names[i] = d.FileName + part
		}
	}
	save = d.Dir
	if tj.Folder {
		save = filepath.Join(d.Dir, d.FileName+part)
	}
	return save, names, prio
}

func (d *Download) torrentProgress(st TorrentStatus) {
	d.mu.Lock()
	if st.Size > 0 {
		d.Total = st.Size
	}
	d.streamDone = st.Done
	d.peers, d.seeds = st.Peers, st.Seeds
	d.uploadSpeed = st.UploadRate
	d.Torrent.Uploaded = st.Uploaded
	d.mu.Unlock()
	d.notify(false)
}

// renameTries bounds the attempts at giving the finished files their names:
// an antivirus program scanning a new file can hold it for a moment.
var renameTries = 4

// finishTorrent gives the finished files their names, marks them as
// downloaded from the internet and has the torrent seed, or takes it out of
// the session when it seeded enough already (or is not to seed).
func (d *Download) finishTorrent(ctx context.Context, b TorrentBackend) error {
	d.mu.Lock()
	dir, name, folder, placed := d.Dir, d.FileName, d.Torrent.Folder, d.Torrent.Final
	files := append([]TorrentFile(nil), d.Torrent.Files...)
	d.mu.Unlock()

	final := name
	if placed {
		// Files added after it finished: they downloaded in place.
		return d.finishedTorrent(b, dir, final, folder, files)
	}
	if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
		// Something took the name meanwhile: keep it and finish under the
		// next free one.
		var err error
		if folder {
			final, err = freeFolder(dir, name)
		} else {
			final, err = freeName(dir, name)
		}
		if err != nil {
			return err
		}
	}
	var err error
	for try := 0; try < renameTries; try++ {
		if try > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(renameWait * 2):
			}
		}
		if folder {
			err = b.Move(ctx, d.ID, filepath.Join(dir, final))
		} else {
			err = b.Rename(ctx, d.ID, map[int]string{firstFile(files): final})
		}
		if err == nil {
			break
		}
	}
	if err != nil {
		return fmt.Errorf("naming the finished files: %w", err)
	}
	d.mu.Lock()
	d.FileName = final
	d.mu.Unlock()
	if folder {
		_ = os.Remove(filepath.Join(dir, name+".odmpart")) // empty after the move
	}
	return d.finishedTorrent(b, dir, final, folder, files)
}

// finishedTorrent marks the files of a torrent finished as final in dir and
// has it seed, or takes it out of the session.
func (d *Download) finishedTorrent(b TorrentBackend, dir, final string, folder bool, files []TorrentFile) error {
	d.mu.Lock()
	d.Torrent.Final = true
	size, uploaded, secs := d.Total, d.Torrent.Uploaded, d.Torrent.SeedTime
	d.mu.Unlock()

	seed := !seeded(d.seedPolicy(), size, uploaded, time.Duration(secs)*time.Second)
	if !seed {
		d.leaveSession(b)
	}
	if MarkFile != nil {
		source, referrer := d.origin()
		for _, p := range torrentPaths(dir, final, folder, files, false) {
			_ = MarkFile(p, source, referrer)
		}
	}
	d.mu.Lock()
	d.Status = StatusComplete
	d.streamDone = d.Total
	d.Torrent.Seeding = seed
	d.mu.Unlock()
	return nil
}

// leaveSession takes a finished torrent out of the session, keeping its
// files, and deletes the part file it needed for seeding.
func (d *Download) leaveSession(b TorrentBackend) {
	d.mu.Lock()
	id, dir, name, folder := d.ID, d.Dir, d.FileName, d.Torrent.Folder
	hashes := []string{d.Torrent.InfoHash, d.Torrent.InfoHashV2}
	d.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	_ = b.Remove(ctx, id, false)
	cancel()
	d.setInSession(false)
	if folder {
		removePartFiles(filepath.Join(dir, name), hashes)
	}
}

// How seeding ended.
const (
	seedReached  = iota // the target, or settings that want no more
	seedStopped         // StopSeeding
	seedShutdown        // ODM closing: it seeds again at the next start
	seedRemoved         // the download was removed
	seedFailed          // the torrent stopped on an error
)

// seedTorrent has a finished torrent upload until its target, StopSeeding,
// the end of ctx or an error. readd puts it back into the session first,
// as ODM started again.
func (d *Download) seedTorrent(ctx context.Context, b TorrentBackend, readd bool) {
	events, stop := b.Watch(d.ID)
	defer stop()
	switch d.seedLoop(ctx, b, events, readd) {
	case seedShutdown, seedRemoved:
		return // Seeding stays set for the next start, or nothing is left
	}
	d.leaveSession(b)
	d.mu.Lock()
	d.Torrent.Seeding = false
	d.peers, d.seeds, d.uploadSpeed = 0, 0, 0
	d.mu.Unlock()
	d.notify(true)
}

func (d *Download) seedLoop(ctx context.Context, b TorrentBackend, events <-chan TorrentEvent, readd bool) int {
	d.mu.Lock()
	base := time.Duration(d.Torrent.SeedTime) * time.Second
	d.mu.Unlock()
	start := time.Now()
	// enough also keeps the seconds seeded, for the next start.
	enough := func() bool {
		elapsed := base + time.Since(start)
		d.mu.Lock()
		d.Torrent.SeedTime = int64(elapsed / time.Second)
		size, uploaded := d.Total, d.Torrent.Uploaded
		d.mu.Unlock()
		return seeded(d.seedPolicy(), size, uploaded, elapsed)
	}
	if enough() {
		return seedReached
	}
	if readd {
		if err := d.readdSeed(ctx, b); err != nil {
			if ctx.Err() != nil {
				return d.seedEnd()
			}
			return seedFailed
		}
	}
	tick := time.NewTicker(seedCheck)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return d.seedEnd()
		case <-tick.C:
			if enough() {
				return seedReached
			}
			d.notify(false)
		case ev, ok := <-events:
			if !ok {
				return seedFailed
			}
			switch ev.Kind {
			case TorrentEventStatus:
				if ev.Status.Error != "" {
					return seedFailed
				}
				d.mu.Lock()
				d.peers, d.seeds = ev.Status.Peers, ev.Status.Seeds
				d.uploadSpeed = ev.Status.UploadRate
				d.Torrent.Uploaded = ev.Status.Uploaded
				d.mu.Unlock()
				if enough() {
					return seedReached
				}
				d.notify(false)
			case TorrentEventError:
				return seedFailed
			}
		}
	}
}

// seedEnd is how seeding ended when its context did.
func (d *Download) seedEnd() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	switch {
	case d.removed:
		return seedRemoved
	case d.seedStop:
		return seedStopped
	}
	return seedShutdown
}

// readdSeed puts a finished torrent back into the session, where its files
// are now.
func (d *Download) readdSeed(ctx context.Context, b TorrentBackend) error {
	d.mu.Lock()
	id, magnet, source := d.ID, d.Torrent.Magnet, d.URL
	d.mu.Unlock()
	file := ""
	if magnet == "" {
		file = torrentPath(id, ".torrent")
		if _, err := os.Stat(file); err != nil {
			return err
		}
	}
	add := TorrentAdd{ID: id, File: file, Magnet: magnet, Resume: torrentPath(id, ".resume")}
	add.SavePath, add.Names, add.Priorities = d.torrentLayout(true)
	if meta, err := b.Inspect(ctx, file, magnet); err == nil {
		add.DropTrackers, add.DropWebSeeds = torrentDrops(source, meta)
	}
	if err := b.Add(ctx, add); err != nil {
		return err
	}
	d.setInSession(true)
	return nil
}

// beginSeeding records a seeding run of the download, unless one runs.
func (d *Download) beginSeeding(cancel context.CancelFunc, done chan struct{}) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.seedCancel != nil || d.removed || d.Torrent == nil || !d.Torrent.Seeding {
		return false
	}
	d.seedCancel, d.seedDone, d.seedStop = cancel, done, false
	return true
}

func (d *Download) endSeeding() {
	d.mu.Lock()
	d.seedCancel, d.seedDone = nil, nil
	d.mu.Unlock()
}

// haltSeeding ends a seeding run and waits up to timeout for it.
func (d *Download) haltSeeding(timeout time.Duration) {
	d.mu.Lock()
	cancel, done := d.seedCancel, d.seedDone
	d.mu.Unlock()
	if cancel == nil {
		return
	}
	cancel()
	select {
	case <-done:
	case <-time.After(timeout):
	}
}

func firstFile(files []TorrentFile) int {
	for i, f := range files {
		if !f.Pad {
			return i
		}
	}
	return 0
}

// torrentPaths lists the downloaded files of a torrent named name in dir;
// with skipped, also those left out, which may hold what was downloaded
// before they were.
func torrentPaths(dir, name string, folder bool, files []TorrentFile, skipped bool) []string {
	if !folder {
		return []string{filepath.Join(dir, name)}
	}
	var out []string
	for _, f := range files {
		if !f.Pad && (!f.Skip || skipped) && f.Name != "" {
			out = append(out, filepath.Join(dir, name, filepath.FromSlash(f.Name)))
		}
	}
	return out
}

// removePartFiles deletes the part file libtorrent keeps in a folder
// torrent's folder for pieces shared with files that were not downloaded.
func removePartFiles(folder string, hashes []string) {
	for _, h := range hashes {
		if h != "" {
			_ = os.Remove(filepath.Join(folder, "."+h+".parts"))
		}
	}
}

// removeTorrent is RemoveWithFile's part for a torrent: out of the session
// with what it downloaded unless it is complete and kept, then its files in
// TorrentDir.
func (d *Download) removeTorrent(deleteFiles bool) {
	d.mu.Lock()
	id, dir, name := d.ID, d.Dir, d.FileName
	// A torrent given more files after it finished keeps the ones it had,
	// as a complete one.
	complete := d.Status == StatusComplete || d.Torrent.Final
	known := d.Torrent.adopted()
	folder := d.Torrent.Folder
	files := append([]TorrentFile(nil), d.Torrent.Files...)
	d.mu.Unlock()
	if b := Torrents; b != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		_ = b.Remove(ctx, id, !complete || deleteFiles)
		cancel()
		d.setInSession(false)
	}
	switch {
	case !known || name == "":
	case !complete:
		// What is left of the download: its .odmpart, file or folder.
		_ = os.RemoveAll(filepath.Join(dir, name+".odmpart"))
	case deleteFiles:
		for _, p := range torrentPaths(dir, name, folder, files, true) {
			_ = os.Remove(p)
		}
		if folder {
			removeEmptyDirs(filepath.Join(dir, name), files)
		}
	}
	_ = os.Remove(torrentPath(id, ".torrent"))
	_ = os.Remove(torrentPath(id, ".resume"))
}

// removeEmptyDirs removes the folders of a folder torrent that deleting its
// files left empty, deepest first, and the torrent's folder last.
func removeEmptyDirs(root string, files []TorrentFile) {
	dirs := map[string]bool{}
	for _, f := range files {
		for p := path.Dir(f.Name); f.Name != "" && p != "." && p != "/"; p = path.Dir(p) {
			dirs[p] = true
		}
	}
	list := make([]string, 0, len(dirs))
	for p := range dirs {
		list = append(list, p)
	}
	sort.Slice(list, func(i, j int) bool { return len(list[i]) > len(list[j]) })
	for _, p := range list {
		_ = os.Remove(filepath.Join(root, filepath.FromSlash(p))) // only if empty
	}
	_ = os.Remove(root)
}

// errHelperStopped: odm-libtorrent ended while the torrent ran.
var errHelperStopped = errors.New("odm-libtorrent stopped")

// errTwinTorrent is a torrent download's error while odm-libtorrent holds
// another download of the same torrent.
var errTwinTorrent = errors.New("the same torrent is already in the list")

// Why SetTorrentFiles refuses a change.
var (
	errFilesUnknown  = errors.New("the torrent's files are not known yet")
	errTorrentDone   = errors.New("a complete torrent keeps its files: more can be added")
	errNoFileWanted  = errors.New("choose at least one file to download")
	errBadPriority   = errors.New("a file's priority is 0 (skip), 1 (low), 4 (normal) or 7 (high)")
	errNoSuchFile    = errors.New("the torrent has no such file")
	errNotTorrentJob = errors.New("not a torrent")
)

// setFiles gives the files of the torrent their priorities, by index: 0
// leaves a file out. It reports whether the torrent runs in the session and
// is to be told, and the priorities of every file then. A complete torrent
// only takes files it left out (reopen): it downloads them where its files
// are, and the caller starts it again.
func (d *Download) setFiles(prio map[int]int) (live, reopen bool, all []int, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	switch {
	case d.Torrent == nil:
		return false, false, nil, errNotTorrentJob
	case len(d.Torrent.Files) == 0:
		return false, false, nil, errFilesUnknown
	}
	complete := d.Status == StatusComplete
	files := append([]TorrentFile(nil), d.Torrent.Files...)
	for i, p := range prio {
		if i < 0 || i >= len(files) || files[i].Pad {
			return false, false, nil, errNoSuchFile
		}
		if complete {
			switch {
			case p == 0 && !files[i].Skip:
				return false, false, nil, errTorrentDone
			case p != 0 && files[i].Skip:
				reopen = true
			case p != 0:
				continue // its pieces are all in: no priority to give
			}
		}
		switch p {
		case 0:
			files[i].Skip = true
		case PriorityLow, PriorityHigh:
			files[i].Skip, files[i].Priority = false, p
		case PriorityNormal:
			files[i].Skip, files[i].Priority = false, 0
		default:
			return false, false, nil, errBadPriority
		}
	}
	tj := TorrentJob{Files: files}
	if tj.largest() == "" {
		return false, false, nil, errNoFileWanted
	}
	d.Torrent.Files = files
	all = make([]int, len(files))
	for i, f := range files {
		all[i] = f.Prio()
	}
	if reopen {
		// What it had is in: the bytes of the files it downloaded.
		d.streamDone = d.Total
		d.Torrent.Final = true
	}
	// A run reports the bytes wanted itself; until then they are counted
	// here.
	if d.Status != StatusDownloading {
		d.Total = tj.wanted()
		d.streamDone = min(d.streamDone, d.Total)
	}
	// Before its metadata, a magnet link's files take these priorities
	// when it comes.
	return d.Status == StatusDownloading && d.Torrent.adopted(), reopen, all, nil
}

// reopenTorrent has a complete torrent given more files download them:
// its seeding stops (the torrent stays in the session), and it queues.
func (d *Download) reopenTorrent() {
	d.haltSeeding(15 * time.Second)
	d.mu.Lock()
	d.Torrent.Seeding = false
	d.peers, d.seeds, d.uploadSpeed = 0, 0, 0
	d.Status = StatusQueued
	d.Err = ""
	d.mu.Unlock()
}

// Why SetTorrentSeed refuses a target.
var errBadSeed = errors.New("seeding is \"\" (until a ratio or a time), \"never\" or \"forever\"")

// setSeed gives the torrent its own seeding target; nil follows the
// Options. It reports whether the torrent, complete and no longer seeding,
// is to seed again under it.
func (d *Download) setSeed(s *TorrentSeed) (again bool, err error) {
	if s != nil {
		switch s.Mode {
		case SeedTarget, SeedNever, SeedForever:
		default:
			return false, errBadSeed
		}
		c := *s
		c.Ratio, c.Minutes = max(c.Ratio, 0), max(c.Minutes, 0)
		s = &c
	}
	d.mu.Lock()
	if d.Torrent == nil {
		d.mu.Unlock()
		return false, errNotTorrentJob
	}
	d.Torrent.Seed = s
	stopped := d.Status == StatusComplete && !d.Torrent.Seeding
	size, uploaded, secs := d.Total, d.Torrent.Uploaded, d.Torrent.SeedTime
	d.mu.Unlock()
	if !stopped || seeded(d.seedPolicy(), size, uploaded, time.Duration(secs)*time.Second) {
		return false, nil
	}
	d.mu.Lock()
	d.Torrent.Seeding = true
	d.mu.Unlock()
	return true, nil
}
