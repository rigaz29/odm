package gui

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	"odm/internal/engine"
	"odm/internal/media"
	"odm/internal/ytdlp"
)

// The New Download window, as the Fyne UI's (internal/ui/dialogs_desktop.go,
// newAddForm): the page shows the form and the card that says what the
// address is; checking the address and building the download happen here.
// What a check found stays here, keyed by the address, so the page sends
// only the indices of the quality and audio it picked.

// AddDefaults are what an empty New Download form starts with.
type AddDefaults struct {
	Folder      string   `json:"folder"`
	Connections int      `json:"connections"`
	Categories  []string `json:"categories"`
	Queues      []string `json:"queues"`
	Queue       string   `json:"queue"`
}

// AddPrefill is a download the browser handed over, for the window it
// opens: Site and Stream say the address is a video page or a playlist,
// whose qualities the check lists.
type AddPrefill struct {
	URL         string `json:"url"`
	FileName    string `json:"fileName"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Site        bool   `json:"site"`
	Stream      bool   `json:"stream"`
}

// Card is what checking an address found, in the card under the form.
// Title and Detail are English texts the page translates; Name is a file
// name or a video's title, shown as it is; Error is the engine's reason.
type Card struct {
	// State is "idle", "file", "stream", "site", "warn" or "problem".
	State  string `json:"state"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Name   string `json:"name"`
	Error  string `json:"error"`
	// A file: its size (0 unknown), type, server and whether it resumes.
	Size        int64  `json:"size"`
	ContentType string `json:"contentType"`
	Host        string `json:"host"`
	Resumable   bool   `json:"resumable"`
	// A stream: "HLS", "DASH" or "HTTP", and the playlist's note. A
	// video site: its name ("" for one yt-dlp reads but ODM does not know).
	Format string `json:"format"`
	Note   string `json:"note"`
	Site   string `json:"site"`
}

// Quality is one quality the address offers, with its audio tracks (none:
// the audio is in the video) and, for a video page, its container.
type Quality struct {
	Label    string   `json:"label"`
	Estimate int64    `json:"estimate"`
	Audios   []string `json:"audios"`
	Ext      string   `json:"ext"`
}

// ProbeResult is the answer for one address.
type ProbeResult struct {
	URL  string `json:"url"`
	Card Card   `json:"card"`
	// Kind is "file", "stream" or "site"; "" when the address could not
	// be checked.
	Kind string `json:"kind"`
	// FileName and Category are suggestions for empty fields; Video asks
	// for the Video category (a stream or a video page).
	FileName  string    `json:"fileName"`
	Category  string    `json:"category"`
	Video     bool      `json:"video"`
	Qualities []Quality `json:"qualities"`
	Quality   int       `json:"quality"` // the one to select
	// Zip offers a preview of the archive's contents.
	Zip bool `json:"zip"`
	// Torrent is what a torrent holds (Kind "torrent").
	Torrent *TorrentProbe `json:"torrent"`
}

// CreateRequest is the form, sent by Start Download and Download Later.
type CreateRequest struct {
	Token       string `json:"token"` // the browser's prefill, "" for none
	URL         string `json:"url"`
	Folder      string `json:"folder"`
	FileName    string `json:"fileName"`
	Description string `json:"description"`
	Checksum    string `json:"checksum"`
	Category    string `json:"category"`
	Queue       string `json:"queue"`
	Connections int    `json:"connections"`
	Quality     int    `json:"quality"`
	Audio       int    `json:"audio"`
	Start       bool   `json:"start"`
	// Again adds an address already in the list once more.
	Again bool `json:"again"`
	// Skip lists the files of a torrent left out, by index.
	Skip []int `json:"skip"`
}

// CreateResult is the new download, or the one already in the list with
// the same address (Duplicate; ID is then "").
type CreateResult struct {
	ID        string `json:"id"`
	Duplicate *Item  `json:"duplicate"`
}

// probed is what checking one address found, for Create.
type probed struct {
	at time.Time
	// A playlist: what it offers, or why it can never be downloaded
	// (streamErr with ErrStreamRefused); kind stays when reading it
	// failed for now, so ODM picks the best quality when it runs.
	kind      media.Kind
	stream    *engine.StreamChoices
	streamErr error
	// A video page yt-dlp reads, or why it cannot (no yt-dlp, refused).
	site    bool
	choices *engine.SiteChoices
	siteErr error
	// A broadcast found on a page, recorded from its playlist.
	live *engine.SiteChoices
}

const maxProbed = 32 // addresses remembered for Create

// AddDefaults returns what a new form starts with.
func (s *Service) AddDefaults() AddDefaults {
	st := s.mgr.Settings()
	return AddDefaults{
		Folder: st.SaveFolder(), Connections: st.Connections,
		Categories: engine.Categories(), Queues: s.mgr.Queues(), Queue: engine.DefaultQueue,
	}
}

// keepPrefill stores a browser hand-off for its window and returns the
// token the window asks with.
func (s *Service) keepPrefill(o engine.AddOptions) string {
	s.amu.Lock()
	defer s.amu.Unlock()
	s.seq++
	token := "p" + strconv.Itoa(s.seq)
	s.prefills[token] = o
	return token
}

func (s *Service) prefill(token string) engine.AddOptions {
	s.amu.Lock()
	defer s.amu.Unlock()
	return s.prefills[token]
}

// Prefill returns the browser's hand-off for the window token opened.
func (s *Service) Prefill(token string) AddPrefill {
	o := s.prefill(token)
	name := o.FileName
	if o.Site != nil && engine.NeedsSiteTitle(name) {
		// The page's title was only the site's ("TikTok - Make Your Day"):
		// the form takes yt-dlp's title of the video instead.
		name = ""
	}
	return AddPrefill{URL: o.URL, FileName: name, Category: o.Category, Description: o.Description,
		Site: o.Site != nil, Stream: o.Stream != nil}
}

// DropPrefill forgets a hand-off once its window closed.
func (s *Service) DropPrefill(token string) {
	s.amu.Lock()
	delete(s.prefills, token)
	s.amu.Unlock()
}

func (s *Service) remember(addr string, p *probed) {
	s.amu.Lock()
	defer s.amu.Unlock()
	p.at = s.now()
	s.probes[addr] = p
	for len(s.probes) > maxProbed {
		oldest := ""
		for k, v := range s.probes {
			if oldest == "" || v.at.Before(s.probes[oldest].at) {
				oldest = k
			}
		}
		delete(s.probes, oldest)
	}
}

func (s *Service) probedFor(addr string) *probed {
	s.amu.Lock()
	defer s.amu.Unlock()
	return s.probes[addr]
}

// Probe checks address: a file's size and name, a playlist's qualities, a
// video page's qualities through yt-dlp. token is the browser's prefill,
// whose headers the check sends and whose quality it selects.
func (s *Service) Probe(address, token string) ProbeResult {
	addr := strings.TrimSpace(address)
	pre := s.prefill(token)
	fromBrowser := pre.URL != "" && addr == strings.TrimSpace(pre.URL)
	res := ProbeResult{URL: addr, Quality: 0}
	switch {
	case addr == "":
		res.Card = Card{State: "idle", Title: "No address yet", Detail: "Paste or type one: ODM checks the file's size and type before adding it."}
		return res
	case s.isTorrentAddress(addr):
		return s.probeTorrent(addr, pre.Headers)
	case !strings.HasPrefix(addr, "http"):
		res.Card = Card{State: "warn", Title: "Not checked", Detail: "Only http and https addresses are checked before they are added."}
		return res
	}
	if (fromBrowser && pre.Site != nil) || ytdlp.Site(addr) != "" {
		return s.probeSite(addr, pre, fromBrowser)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	// The browser already knows a playlist when it sends one; a typed
	// address is checked as a file first, and read as a playlist when its
	// type or extension says it is one.
	var kind media.Kind
	if fromBrowser && pre.Stream != nil {
		kind = pre.Stream.Kind
	}
	var p *engine.Probe
	var err error
	if kind == "" {
		p, err = s.mgr.Probe(ctx, addr, pre.Headers)
		if err == nil {
			if k := media.Detect(p.ContentType, addr, nil); k == media.KindHLS || k == media.KindDASH {
				kind = k
			}
		}
	}
	if kind != "" {
		var choices *engine.StreamChoices
		if kind == media.KindHTTPLive {
			choices = engine.HTTPLiveChoices() // an endless response: nothing to read before
		} else {
			choices, err = s.mgr.ProbeStream(ctx, addr, pre.Headers)
			if choices != nil {
				kind = choices.Kind // the playlist itself is the better judge
			}
		}
		if fromBrowser || !errors.Is(err, engine.ErrStreamRefused) {
			pick := ""
			if fromBrowser && pre.StreamQuality != "" {
				pick = pre.StreamQuality
			}
			return s.streamResult(addr, kind, choices, err, pick, nil)
		}
		// A typed address that only looked like a stream (an .m3u music
		// list, say) downloads as the file it is.
		res.Kind = "file"
		res.Card = Card{State: "warn", Title: "Not a stream ODM can read", Detail: "It downloads as a file", Error: err.Error()}
		return res
	}
	if err != nil {
		res.Card = Card{State: "problem", Title: "The file could not be checked", Error: err.Error()}
		return res
	}
	if s.torrentType(p.ContentType) {
		return s.probeTorrent(addr, pre.Headers)
	}
	s.remember(addr, &probed{})
	res.Kind = "file"
	res.FileName = p.FileName
	res.Category = engine.Category(p.FileName)
	res.Card = Card{State: "file", Name: p.FileName, Size: p.Total, Resumable: p.RangeOK,
		ContentType: strings.TrimSpace(strings.SplitN(p.ContentType, ";", 2)[0])}
	if u, err := url.Parse(p.FinalURL); err == nil {
		res.Card.Host = u.Hostname()
	}
	res.Zip = p.RangeOK && p.Total > 0 && engine.IsZipName(p.FileName, p.ContentType)
	return res
}

// streamResult answers for a playlist: its qualities, or why it cannot be
// read. pick is a quality label to select (the browser's menu); live is the
// page a broadcast was found on.
func (s *Service) streamResult(addr string, kind media.Kind, c *engine.StreamChoices, err error, pick string, live *engine.SiteChoices) ProbeResult {
	res := ProbeResult{URL: addr, Kind: "stream", Video: true}
	pr := &probed{kind: kind, live: live}
	defer func() { s.remember(addr, pr) }()
	switch {
	case errors.Is(err, engine.ErrStreamRefused):
		pr.streamErr = err
		res.Card = Card{State: "problem", Title: "This stream cannot be downloaded", Error: err.Error()}
		return res
	case err != nil:
		res.Card = Card{State: "warn", Title: "The playlist could not be read just now", Detail: "Start lets ODM pick the best quality when it runs", Error: err.Error()}
		return res
	}
	pr.stream = c
	for _, q := range c.Qualities {
		var audios []string
		for _, a := range q.Audios {
			audios = append(audios, a.Label)
		}
		res.Qualities = append(res.Qualities, Quality{Label: q.Label, Estimate: q.Estimate, Audios: audios})
	}
	if pick != "" {
		if q := c.Quality(pick); q >= 0 {
			res.Quality = q
		}
	}
	format := strings.ToUpper(string(c.Kind))
	if c.Kind == media.KindHTTPLive {
		format = "HTTP" // one endless response
	}
	res.Card = Card{State: "stream", Format: format, Note: c.Note}
	if live != nil {
		res.Card.Name = live.Title
		res.FileName = engine.SanitizeName(live.Title)
	}
	return res
}

// probeSite asks yt-dlp about a video page; a broadcast on it is recorded
// as a stream from the playlist yt-dlp finds.
func (s *Service) probeSite(addr string, pre engine.AddOptions, fromBrowser bool) ProbeResult {
	// yt-dlp reads the page and the player: it takes a while.
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	c, err := s.mgr.ProbeSite(ctx, addr)
	if err == nil && c.Live {
		var sc *engine.StreamChoices
		var serr error
		if c.Kind == media.KindHTTPLive {
			sc = engine.HTTPLiveChoices()
		} else {
			sc, serr = s.mgr.ProbeStream(ctx, c.Manifest, c.Headers)
		}
		return s.streamResult(addr, c.Kind, sc, serr, "", c)
	}

	res := ProbeResult{URL: addr, Kind: "site", Video: true}
	pr := &probed{site: true}
	defer func() { s.remember(addr, pr) }()
	site := ytdlp.Site(addr)
	switch {
	case errors.Is(err, engine.ErrNoYtDlp):
		pr.siteErr = err
		res.Card = Card{State: "problem", Title: "Videos from this site need yt-dlp",
			Detail: "Install yt-dlp (put yt-dlp.exe next to ODM or on PATH), or choose it in Options → Downloads."}
		return res
	case errors.Is(err, engine.ErrStreamRefused):
		pr.siteErr = err
		res.Card = Card{State: "problem", Title: "This video cannot be downloaded", Error: err.Error()}
		return res
	case err != nil:
		res.Card = Card{State: "warn", Title: "yt-dlp could not read the page just now",
			Detail: "Start lets yt-dlp pick the best quality when it runs", Error: err.Error()}
		return res
	}
	pr.choices = c
	for i, q := range c.Qualities {
		res.Qualities = append(res.Qualities, Quality{Label: q.Label, Estimate: q.Estimate, Ext: q.Ext})
		if fromBrowser && pre.Site != nil && q.Format == pre.Site.Format {
			res.Quality = i // picked in the browser's menu
		}
	}
	res.FileName = engine.SanitizeName(c.Title)
	res.Card = Card{State: "site", Name: c.Title, Site: site}
	return res
}

// Create adds the download the form describes, and starts it with Start.
// An address already in the list comes back as Duplicate unless Again.
func (s *Service) Create(r CreateRequest) (CreateResult, error) {
	addr := strings.TrimSpace(r.URL)
	if addr == "" {
		return CreateResult{}, errNoAddress
	}
	sum, err := engine.ParseChecksum(r.Checksum)
	if err != nil {
		return CreateResult{}, err
	}
	if s.hasTorrentProbe(addr) || s.isTorrentAddress(addr) {
		return s.createTorrent(r, addr)
	}
	pre := s.prefill(r.Token)
	fromBrowser := pre.URL != "" && addr == strings.TrimSpace(pre.URL)
	pr := s.probedFor(addr)
	if pr == nil {
		pr = &probed{}
	}

	var siteJob *engine.SiteJob
	switch {
	case pr.site && pr.siteErr != nil:
		return CreateResult{}, pr.siteErr
	case pr.site:
		siteJob = &engine.SiteJob{} // yt-dlp's best when the page was not read
		if c := pr.choices; c != nil && r.Quality >= 0 && r.Quality < len(c.Qualities) {
			siteJob.Format, siteJob.Estimate = c.Qualities[r.Quality].Format, c.Qualities[r.Quality].Estimate
		}
	case fromBrowser && pre.Site != nil:
		j := *pre.Site // started before yt-dlp answered: the browser's pick
		siteJob = &j
	}

	// A broadcast found on a page is recorded from its playlist.
	manifest, headers := addr, pre.Headers
	if pr.live != nil {
		manifest = pr.live.Manifest
		headers = map[string]string{}
		for k, v := range pre.Headers {
			headers[k] = v
		}
		for k, v := range pr.live.Headers {
			headers[k] = v
		}
	}
	var job *engine.StreamJob
	switch {
	case siteJob != nil:
	case pr.streamErr != nil:
		return CreateResult{}, pr.streamErr
	case pr.stream != nil:
		j, err := pr.stream.Job(manifest, r.Quality, r.Audio)
		if err != nil {
			return CreateResult{}, err
		}
		job = &j
	case pr.kind != "":
		// The playlist could not be read just now: ODM picks the best
		// quality when the download runs.
		job = &engine.StreamJob{Kind: pr.kind, Manifest: manifest}
	case fromBrowser && pre.Stream != nil:
		j := *pre.Stream // started before the playlist was read
		job = &j
	}
	switch {
	case job == nil:
	case pr.live != nil:
		job.Page = addr
	case fromBrowser && pre.Stream != nil:
		job.Page = pre.Stream.Page // a broadcast the browser found on a page
	}

	if existing, dup := s.mgr.FindByURL(manifest); dup && !r.Again {
		it := itemOf(existing)
		return CreateResult{Duplicate: &it}, nil
	}
	folder := strings.TrimSpace(r.Folder)
	conns := r.Connections
	if conns <= 0 {
		conns = s.mgr.Settings().Connections
	}
	info := s.mgr.Create(engine.AddOptions{
		Stream: job, Site: siteJob, URL: manifest, Dir: folder,
		FileName:    engine.SanitizeName(r.FileName),
		Description: strings.TrimSpace(r.Description),
		Checksum:    sum, Connections: conns,
		Category: r.Category, Queue: r.Queue, Headers: headers, Start: r.Start,
	})
	s.mgr.NoteSaveFolder(folder)
	if r.Token != "" {
		s.DropPrefill(r.Token)
	}
	s.poke()
	if w := s.opener(); r.Start && !s.mgr.Settings().NoProgressDialog && w != nil {
		w.openProgress(info.ID)
	}
	return CreateResult{ID: info.ID}, nil
}

// ZipEntry is one file or folder in a ZIP archive.
type ZipEntry struct {
	Name      string    `json:"name"`
	Size      int64     `json:"size"`
	Modified  time.Time `json:"modified"`
	Dir       bool      `json:"dir"`
	Encrypted bool      `json:"encrypted"`
}

// ZipListing is what an archive holds and how much was read to list it.
type ZipListing struct {
	Entries []ZipEntry `json:"entries"`
	Files   int        `json:"files"`
	Size    int64      `json:"size"`
	Fetched int64      `json:"fetched"`
}

// ZipPreview lists a ZIP archive's contents, reading only its end.
func (s *Service) ZipPreview(address, token string) (ZipListing, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	l, err := s.mgr.PreviewZip(ctx, strings.TrimSpace(address), s.prefill(token).Headers)
	if err != nil {
		return ZipListing{}, err
	}
	out := ZipListing{Files: l.Files, Size: l.Size, Fetched: l.Fetched}
	for _, e := range l.Entries {
		out.Entries = append(out.Entries, ZipEntry{Name: e.Name, Size: e.Size, Modified: e.Modified, Dir: e.Dir, Encrypted: e.Encrypted})
	}
	return out, nil
}
