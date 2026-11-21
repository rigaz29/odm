package engine

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"unicode/utf8"

	"odm/internal/ffmpeg"
	"odm/internal/media"
	"odm/internal/ytdlp"
)

// SiteJob is a download made from a video site's page (YouTube and the
// other sites yt-dlp reads): ODM runs yt-dlp for it, shows its progress and
// finishes the file like any other. The download's URL is the page.
//
// yt-dlp reads the page again on every run, so a link that expired while
// the download was paused does not matter; its partial files in the work
// folder are continued.
type SiteJob struct {
	// Format is yt-dlp's format selector ("137+140": video and audio to
	// join); "" lets yt-dlp pick its best.
	Format string `json:"format,omitempty"`
	// Estimate is the expected size in bytes, 0 when unknown, shown until
	// yt-dlp reports the sizes itself.
	Estimate int64 `json:"estimate,omitempty"`
	// Done is the bytes fetched when the state was saved.
	Done int64 `json:"done,omitempty"`
	// Claimed is set once the download holds its name (claimName) and so
	// the work folder named after it: two downloads of one title do not
	// share yt-dlp's files.
	Claimed bool `json:"claimed,omitempty"`
}

// ErrNoYtDlp wraps ytdlp.ErrNotFound for video pages, so callers can tell
// a missing program from a page yt-dlp could not read.
var ErrNoYtDlp = ytdlp.ErrNotFound

// EnsureTool, when set, provides a companion program that was not found
// ("yt-dlp", "ffmpeg"): ODM's own copy, downloaded when the person allowed
// it (see tools.Keeper). It returns the program's path.
var EnsureTool func(ctx context.Context, name string) (string, error)

// locateYtDlp finds yt-dlp, or has it installed.
func locateYtDlp(ctx context.Context) (string, error) {
	bin, err := ytdlp.Locate(ytdlpConfigured())
	if err != nil && EnsureTool != nil && ytdlpConfigured() == "" {
		p, eerr := EnsureTool(ctx, "yt-dlp")
		if eerr != nil {
			return "", fmt.Errorf("%w; %v", err, eerr)
		}
		return p, nil
	}
	return bin, err
}

// locateFFmpeg finds ffmpeg, or has it installed.
func locateFFmpeg(ctx context.Context) (string, error) {
	bin, err := ffmpeg.Locate(ffmpegConfigured())
	if err != nil && EnsureTool != nil && ffmpegConfigured() == "" {
		p, eerr := EnsureTool(ctx, "ffmpeg")
		if eerr != nil {
			return "", fmt.Errorf("%w; %v", err, eerr)
		}
		return p, nil
	}
	return bin, err
}

// ytdlpSetting is the configured yt-dlp path (Settings.YtDlpPath), read
// from download goroutines.
var ytdlpSetting atomic.Pointer[string]

func setYtDlpPath(p string) { ytdlpSetting.Store(&p) }

// ytdlpCookies is the browser yt-dlp signs in with (Settings.YtDlpCookies),
// read from download goroutines.
var ytdlpCookies atomic.Pointer[string]

func setYtDlpCookies(b string) { ytdlpCookies.Store(&b) }

func cookiesFrom() string {
	if b := ytdlpCookies.Load(); b != nil {
		return *b
	}
	return ""
}

func ytdlpConfigured() string {
	if p := ytdlpSetting.Load(); p != nil {
		return *p
	}
	return ""
}

// SiteChoices is what yt-dlp found at a page, for a window or the browser
// extension to offer before the download is created.
type SiteChoices struct {
	Title     string
	Qualities []ytdlp.Quality
	// Live is a broadcast on now. ODM records it itself from Manifest,
	// fetched with Headers: an HLS playlist or a DASH manifest (Kind; the
	// playlist lists the qualities, see ProbeStream), else the address of
	// an endless HTTP response (KindHTTPLive, one quality). Qualities is
	// then empty.
	Live     bool
	Kind     media.Kind
	Manifest string
	Headers  map[string]string
}

// siteLive picks what ODM records of a live video: its HLS playlist, else
// its DASH manifest, else an endless HTTP response.
func siteLive(v *ytdlp.Video) (media.Kind, string, map[string]string, bool) {
	if m, h, ok := v.LiveManifest(); ok {
		return media.KindHLS, m, h, true
	}
	if m, h, ok := v.LiveDASH(); ok {
		return media.KindDASH, m, h, true
	}
	if m, h, ok := v.LiveHTTP(); ok {
		return media.KindHTTPLive, m, h, true
	}
	return media.KindNone, "", nil, false
}

// Quality returns the index of the quality labelled label, -1 for none.
func (c *SiteChoices) Quality(label string) int {
	for i, q := range c.Qualities {
		if q.Label == label {
			return i
		}
	}
	return -1
}

// ProbeSite asks yt-dlp what the page offers. Without yt-dlp the error
// matches ErrNoYtDlp; a live stream or a page with nothing to download is
// refused (ErrStreamRefused).
func (m *Manager) ProbeSite(ctx context.Context, page string) (*SiteChoices, error) {
	bin, err := locateYtDlp(ctx)
	if err != nil {
		return nil, err
	}
	page = strings.TrimSpace(page)
	v, err := probeSite(ctx, bin, page)
	if err != nil {
		return nil, err
	}
	switch {
	case v.IsLive():
		kind, m, h, ok := siteLive(v)
		if !ok {
			return nil, refusal("this live stream comes in a form ODM cannot record")
		}
		return &SiteChoices{Title: v.Title, Live: true, Kind: kind, Manifest: m, Headers: h}, nil
	case v.LiveStatus == "is_upcoming":
		return nil, refusal("this live stream has not started yet")
	}
	c := &SiteChoices{Title: v.Title, Qualities: v.Qualities()}
	if len(c.Qualities) == 0 {
		return nil, refusal("yt-dlp found nothing ODM can download on this page")
	}
	return c, nil
}

// probeSite has yt-dlp read page, signed in to its site as the Options
// say, and once more without the cookies when that fails or finds nothing
// to download: YouTube can refuse yt-dlp a signed-in session (cookies the
// browser has rotated, a session it limits) and show the same video to
// anyone. The site is then asked without cookies for a while
// (cookiesRefused), so that its download works as well. Failures are
// logged: the extension's menu shows only their gist.
func probeSite(ctx context.Context, bin, page string) (*ytdlp.Video, error) {
	cookies, done := siteCookieArgs(page)
	v, err := ytdlp.Probe(ctx, bin, page, cookies)
	done()
	if ctx.Err() != nil || err == nil && siteUsable(v) {
		return v, err
	}
	if cookies == (ytdlp.Cookies{}) {
		log.Printf("yt-dlp: %s: %s", page, siteFailure(v, err))
		return v, err
	}
	bare, berr := ytdlp.Probe(ctx, bin, page, ytdlp.Cookies{})
	if ctx.Err() != nil {
		return v, err
	}
	if berr == nil && siteUsable(bare) {
		log.Printf("yt-dlp: %s: with the browser's cookies: %s; read without them, as the site is asked for %v", page, siteFailure(v, err), cookieRest)
		cookiesRefused(page)
		return bare, nil
	}
	log.Printf("yt-dlp: %s: with the browser's cookies: %s; without them: %s", page, siteFailure(v, err), siteFailure(bare, berr))
	return v, err
}

// siteUsable reports whether yt-dlp's reading of a page says what to do
// with it: a broadcast, live or to come, or qualities to download.
func siteUsable(v *ytdlp.Video) bool {
	return v != nil && (v.IsLive() || v.LiveStatus == "is_upcoming" || len(v.Qualities()) > 0)
}

func siteFailure(v *ytdlp.Video, err error) string {
	if err != nil {
		return err.Error()
	}
	return "nothing to download"
}

// resolveSiteLive asks yt-dlp again for the address of the broadcast on
// page, of the kind the recording reads: it is signed and expires,
// YouTube's after some hours, TikTok's sooner.
func resolveSiteLive(ctx context.Context, page string, kind media.Kind) (string, map[string]string, error) {
	bin, err := locateYtDlp(ctx)
	if err != nil {
		return "", nil, err
	}
	v, err := probeSite(ctx, bin, page)
	if err != nil {
		return "", nil, err
	}
	if !v.IsLive() {
		return "", nil, refusal("the broadcast is over")
	}
	var m string
	var h map[string]string
	var ok bool
	switch kind {
	case media.KindDASH:
		m, h, ok = v.LiveDASH()
	case media.KindHTTPLive:
		m, h, ok = v.LiveHTTP()
	default:
		m, h, ok = v.LiveManifest()
	}
	if !ok {
		return "", nil, refusal("the broadcast is no longer offered as it was")
	}
	return m, h, nil
}

// useSiteManifest switches a recording to a fresh playlist address from
// yt-dlp and the headers it gave with it.
func (d *Download) useSiteManifest(manifest string, headers map[string]string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.Stream != nil {
		d.Stream.Manifest, d.Stream.Entry = manifest, ""
	}
	d.URL, d.FinalURL = manifest, ""
	next := make(map[string]string, len(d.Headers)+len(headers))
	for k, v := range d.Headers {
		next[k] = v
	}
	for k, v := range headers {
		next[k] = v
	}
	d.Headers = next
}

// runSite is Start for a video page.
func (d *Download) runSite(ctx context.Context) {
	if err := d.claimSite(ctx); err != nil {
		if ctx.Err() != nil {
			d.mu.Lock()
			d.Status = StatusPaused
			d.mu.Unlock()
			return
		}
		d.fail(err)
		return
	}
	// finish may give the file another name; the work folder keeps the
	// one it was created under.
	work := d.workDir()
	file, err := d.fetchSite(ctx, work)
	if d.isRemoved() {
		return
	}
	if err == nil {
		err = d.placeSite(file)
	}
	if err == nil {
		err = d.finish()
	}
	switch {
	case err == nil:
		_ = os.RemoveAll(work)
		d.mu.Lock()
		d.Status = StatusComplete
		if st, serr := os.Stat(filepath.Join(d.Dir, d.FileName)); serr == nil {
			d.Total, d.streamDone = st.Size(), st.Size()
		}
		d.mu.Unlock()
	case ctx.Err() != nil:
		d.mu.Lock()
		d.Status = StatusPaused
		d.mu.Unlock()
	default:
		d.fail(err)
	}
}

// genericSiteTitles are the titles a video site's page has before it names
// the video, as ODM's names write them ("_" for spaces, compared without
// case). The browser extension falls back to the page title when it has
// no other, and TikTok keeps "TikTok - Make Your Day" while its player
// moves from one video to the next; "video" is ODM's own last resort.
var genericSiteTitles = map[string]bool{
	"tiktok": true, "tiktok_-_make_your_day": true, "youtube": true, "youtube_music": true, "video": true,
}

// NeedsSiteTitle reports whether a site download's name says nothing of
// the video: it has none, or one of genericSiteTitles. ODM then names it
// after yt-dlp's title of the video.
func NeedsSiteTitle(name string) bool {
	stem := strings.TrimSuffix(name, filepath.Ext(name))
	stem = strings.ToLower(strings.Join(strings.Fields(stem), "_"))
	return stem == "" || genericSiteTitles[stem]
}

// maxSiteTitle keeps names made from a video's title well inside path
// limits, as the extension's names from page titles are kept.
const maxSiteTitle = 120

// siteTitleName turns yt-dlp's title of a video into a file name written
// as the extension writes names from titles: "_" for spaces (and for what
// Windows forbids), no "_" or "." at the ends, at most maxSiteTitle
// characters. TikTok's titles are the video's caption.
func siteTitleName(title string) string {
	name := SanitizeName(strings.Join(strings.Fields(title), "_"))
	for strings.Contains(name, "__") {
		name = strings.ReplaceAll(name, "__", "_")
	}
	name = strings.Trim(name, "_.")
	for utf8.RuneCountInString(name) > maxSiteTitle {
		_, size := utf8.DecodeLastRuneInString(name)
		name = name[:len(name)-size]
	}
	return strings.TrimRight(name, "_.")
}

// claimSite gives a site download, on its first run, a name of its own:
// the video's title when the name it came with says nothing (nameFromSite),
// claimed as a file's is (claimName), so the work folder named after it
// and its .odmpart are its alone. Unclaimed, two downloads of one title
// (two qualities of a video, two videos with one caption) shared yt-dlp's
// files, and the first to finish deleted them under the other.
//
// A download that started before names were claimed keeps its name and
// work folder when it fetched something there and the name is still free.
func (d *Download) claimSite(ctx context.Context) error {
	d.mu.Lock()
	claimed, started := d.Site.Claimed, d.streamDone > 0
	d.mu.Unlock()
	if claimed {
		return nil
	}
	keep := false
	if started {
		_, err := os.Stat(d.workDir())
		keep = err == nil
	}
	if !keep {
		d.nameFromSite(ctx)
		if ctx.Err() != nil {
			return ctx.Err() // stopped while yt-dlp read the title: name it next time
		}
	}
	d.mu.Lock()
	dir, name := d.Dir, d.FileName
	if name == "" {
		name = "video.mp4"
	}
	d.mu.Unlock()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	got, f, err := claimName(dir, name)
	if err != nil {
		return err
	}
	_ = f.Close() // yt-dlp's file takes its place (placeSite)
	d.mu.Lock()
	d.FileName = got
	d.Site.Claimed = true
	d.mu.Unlock()
	if !keep || got != name {
		// The name was just claimed: a work folder under it is a leftover
		// of an unrelated download.
		if err := os.RemoveAll(d.workDir()); err != nil {
			return err
		}
	}
	d.notify(true)
	return nil
}

// nameFromSite names a site download after its video, as yt-dlp reads it,
// when the name it came with says nothing of it (NeedsSiteTitle): the
// extension sends yt-dlp's title, but an older one, or one that could not
// ask ODM for the page's qualities, sends the page's.
func (d *Download) nameFromSite(ctx context.Context) {
	d.mu.Lock()
	name, page := d.FileName, d.URL
	d.mu.Unlock()
	if !NeedsSiteTitle(name) {
		return
	}
	bin, err := locateYtDlp(ctx)
	if err != nil {
		return
	}
	v, err := probeSite(ctx, bin, page)
	if err != nil {
		return
	}
	title := siteTitleName(v.Title)
	if title == "" {
		return
	}
	// placeSite puts the container yt-dlp chose in place of this one.
	ext := filepath.Ext(name)
	if ext == "" {
		ext = ".mp4"
	}
	d.mu.Lock()
	d.FileName = title + ext
	d.mu.Unlock()
	d.notify(true)
}

// fetchSite runs yt-dlp until the video is in the work folder and returns
// the file it made.
func (d *Download) fetchSite(ctx context.Context, work string) (string, error) {
	bin, err := locateYtDlp(ctx)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(work, 0o755); err != nil {
		return "", err
	}
	// Video and audio apart ("137+140", or yt-dlp's own best) are joined
	// by ffmpeg; "" lets yt-dlp look for it itself.
	ff, _ := ffmpeg.Locate(ffmpegConfigured())
	d.mu.Lock()
	joins := d.Site.Format == "" || strings.Contains(d.Site.Format, "+")
	d.mu.Unlock()
	if ff == "" && joins {
		ff, _ = locateFFmpeg(ctx)
	}
	d.mu.Lock()
	page := d.URL
	d.mu.Unlock()
	cookies, forget := siteCookieArgs(page)
	defer forget()
	d.mu.Lock()
	o := ytdlp.Options{
		Page: page, Format: d.Site.Format, Out: filepath.Join(work, "media"),
		FFmpeg: ff, RateLimit: d.SpeedLimit, Fragments: d.Connections, Cookies: cookies,
	}
	if o.RateLimit <= 0 && d.shared != nil {
		o.RateLimit = d.shared.Rate() // the limit for all downloads, as yt-dlp's own
	}
	formats := strings.Count(d.Site.Format, "+") + 1
	d.mu.Unlock()

	// yt-dlp fetches the formats of a selector one after the other, each
	// with its own count.
	done, totals := map[string]int64{}, map[string]int64{}
	progress := func(p ytdlp.Progress) {
		if p.Format == "" {
			return
		}
		d.mu.Lock()
		done[p.Format] = p.Done
		if p.Total > 0 {
			totals[p.Format] = p.Total
		}
		var sum, total int64
		for _, n := range done {
			sum += n
		}
		for _, n := range totals {
			total += n
		}
		d.streamDone = sum
		switch {
		case len(totals) >= formats:
			d.Total = total
		case d.Site.Estimate > 0:
			d.Total = d.Site.Estimate
		}
		if d.Total < sum {
			d.Total = sum
		}
		d.mu.Unlock()
		d.notify(false)
	}
	file, err := ytdlp.Download(ctx, bin, o, progress)
	if err == nil || ctx.Err() != nil {
		return file, err
	}
	d.mu.Lock()
	started := len(done) > 0
	d.mu.Unlock()
	if o.Cookies != (ytdlp.Cookies{}) && !started {
		// Refused with the person's cookies before a byte came (see
		// probeSite): once more without them. The error that counts is
		// the first, when that fails too.
		o.Cookies = ytdlp.Cookies{}
		bare, berr := ytdlp.Download(ctx, bin, o, progress)
		if berr == nil {
			log.Printf("yt-dlp: %s: download with the browser's cookies: %v; done without them, as the site is asked for %v", page, err, cookieRest)
			cookiesRefused(page)
			return bare, nil
		}
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		log.Printf("yt-dlp: %s: download with the browser's cookies: %v; without them: %v", page, err, berr)
		return "", err
	}
	log.Printf("yt-dlp: %s: download: %v", page, err)
	return file, err
}

// placeSite moves yt-dlp's file to the download's .odmpart, giving the
// name the container yt-dlp chose, so finish can name it. A name that
// changes with the container is claimed as the first one was (claimSite),
// and the first claim given back: the .odmpart of that name may be
// another download's, which the move would replace.
func (d *Download) placeSite(file string) error {
	d.mu.Lock()
	dir, name := d.Dir, d.FileName
	d.mu.Unlock()
	base := name
	if base == "" {
		base = "video"
	}
	want := strings.TrimSuffix(base, filepath.Ext(base)) + filepath.Ext(file)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if want != name {
		claimed, f, err := claimName(dir, want)
		if err != nil {
			return err
		}
		_ = f.Close()
		if name != "" {
			_ = os.Remove(filepath.Join(dir, name) + ".odmpart") // the first claim, still empty
		}
		want = claimed
	}
	d.mu.Lock()
	d.FileName = want
	d.mu.Unlock()
	if err := renameRetry(file, filepath.Join(dir, want)+".odmpart"); err != nil {
		return fmt.Errorf("moving the video into place: %w", err)
	}
	return nil
}
