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
	// SetFiles names the files and gives their priorities, once the
	// metadata of a torrent added with Hold is in; it returns when they are
	// renamed.
	SetFiles(ctx context.Context, id string, names map[int]string, priorities []int) error
	// Rename and Move return once the files are renamed or moved.
	Rename(ctx context.Context, id string, names map[int]string) error
	Move(ctx context.Context, id, savePath string) error
	SetLimits(ctx context.Context, id string, download, upload int64) error
	SetGlobalLimits(download, upload int64)
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
	// Priorities gives each file's priority: 0 skips it, 4 is normal.
	Priorities []int
	// Hold makes a magnet link download nothing until SetFiles.
	Hold          bool
	DownloadLimit int64
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
	Error                    string
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
	Magnet     string `json:"magnet,omitempty"`
	InfoHash   string `json:"info_hash,omitempty"`
	InfoHashV2 string `json:"info_hash_v2,omitempty"`
	// Folder: the torrent has several files, and FileName is a folder.
	Folder bool `json:"folder,omitempty"`
	// Files are the torrent's files once its metadata is known.
	Files []TorrentFile `json:"files,omitempty"`
	// Done is the bytes downloaded when the state was saved.
	Done int64 `json:"done,omitempty"`
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
}

// errNoTorrents is a torrent download's error where ODM cannot run torrents.
var errNoTorrents = errors.New("this ODM cannot download torrents")

// maxTorrentFile bounds a .torrent file.
const maxTorrentFile = 16 << 20

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
		d.fail(err)
	}
}

func (d *Download) transferTorrent(ctx context.Context, b TorrentBackend) error {
	d.mu.Lock()
	id, magnet := d.ID, d.Torrent.Magnet
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
		if !known {
			meta, err := b.Inspect(ctx, file, "")
			if err != nil {
				return fmt.Errorf("reading the torrent: %w", err)
			}
			if err := d.adoptTorrent(meta); err != nil {
				return err
			}
			known = true
		}
	}

	events, stop := b.Watch(id)
	defer stop()
	resume := torrentPath(id, ".resume")
	add := TorrentAdd{ID: id, File: file, Magnet: magnet, Resume: resume, DownloadLimit: limit}
	if known {
		add.SavePath, add.Names, add.Priorities = d.torrentLayout()
	} else {
		// A magnet link before its metadata: nothing is written until the
		// files have their names.
		add.SavePath, add.Hold = d.torrentDir(), true
	}
	err := b.Add(ctx, add)
	if err != nil && known && file == "" {
		// A magnet link whose resume data lost the metadata: fetch it
		// again, then name the files as before.
		add.Names, add.Priorities, add.SavePath, add.Hold = nil, nil, d.torrentDir(), true
		err = b.Add(ctx, add)
	}
	if err != nil {
		return err
	}
	d.notify(true)

	// downloading: a status showed bytes still missing. A torrent that
	// was complete when added (ODM stopped while finishing it) gets no
	// finished event; its first status tells.
	downloading := false
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ev, ok := <-events:
			if !ok {
				return errors.New("odm-libtorrent stopped")
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
				save, names, prio := d.torrentLayout()
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
				} else if !downloading && (st.State == "finished" || st.State == "seeding") {
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
		data, err = io.ReadAll(io.LimitReader(resp.Body, maxTorrentFile+1))
		if err != nil {
			return "", fmt.Errorf("fetching the .torrent file: %w", err)
		}
	} else {
		f, err := os.Open(src)
		if err != nil {
			return "", err
		}
		data, err = io.ReadAll(io.LimitReader(f, maxTorrentFile+1))
		f.Close()
		if err != nil {
			return "", err
		}
	}
	if len(data) > maxTorrentFile {
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
	name := SanitizeName(m.Name)
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

// torrentLayout is where the backend puts the files while they download.
func (d *Download) torrentLayout() (save string, names map[int]string, prio []int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	tj := d.Torrent
	names = map[int]string{}
	prio = make([]int, len(tj.Files))
	for i, f := range tj.Files {
		if f.Pad {
			continue
		}
		if !f.Skip {
			prio[i] = 4
		}
		if tj.Folder {
			names[i] = f.Name
		} else {
			names[i] = d.FileName + ".odmpart"
		}
	}
	save = d.Dir
	if tj.Folder {
		save = filepath.Join(d.Dir, d.FileName+".odmpart")
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
	d.mu.Unlock()
	d.notify(false)
}

// renameTries bounds the attempts at giving the finished files their names:
// an antivirus program scanning a new file can hold it for a moment.
var renameTries = 4

// finishTorrent gives the finished files their names, takes the torrent out
// of the session and marks the files as downloaded from the internet.
func (d *Download) finishTorrent(ctx context.Context, b TorrentBackend) error {
	d.mu.Lock()
	dir, name, folder := d.Dir, d.FileName, d.Torrent.Folder
	files := append([]TorrentFile(nil), d.Torrent.Files...)
	hashes := []string{d.Torrent.InfoHash, d.Torrent.InfoHashV2}
	d.mu.Unlock()

	final := name
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

	// No seeding yet: the torrent leaves the session once it is done.
	rctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	_ = b.Remove(rctx, d.ID, false)
	cancel()
	if folder {
		removePartFiles(filepath.Join(dir, final), hashes)
	}
	if MarkFile != nil {
		source, referrer := d.origin()
		for _, p := range torrentPaths(dir, final, folder, files) {
			_ = MarkFile(p, source, referrer)
		}
	}
	d.mu.Lock()
	d.Status = StatusComplete
	d.streamDone = d.Total
	d.mu.Unlock()
	return nil
}

func firstFile(files []TorrentFile) int {
	for i, f := range files {
		if !f.Pad {
			return i
		}
	}
	return 0
}

// torrentPaths lists the downloaded files of a torrent named name in dir.
func torrentPaths(dir, name string, folder bool, files []TorrentFile) []string {
	if !folder {
		return []string{filepath.Join(dir, name)}
	}
	var out []string
	for _, f := range files {
		if !f.Pad && !f.Skip && f.Name != "" {
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
	complete := d.Status == StatusComplete
	known := d.Torrent.adopted()
	folder := d.Torrent.Folder
	files := append([]TorrentFile(nil), d.Torrent.Files...)
	d.mu.Unlock()
	if b := Torrents; b != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		_ = b.Remove(ctx, id, !complete || deleteFiles)
		cancel()
	}
	switch {
	case !known || name == "":
	case !complete:
		// What is left of the download: its .odmpart, file or folder.
		_ = os.RemoveAll(filepath.Join(dir, name+".odmpart"))
	case deleteFiles:
		for _, p := range torrentPaths(dir, name, folder, files) {
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
