package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"odm/internal/ffmpeg"
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
	v, err := ytdlp.Probe(ctx, bin, strings.TrimSpace(page))
	if err != nil {
		return nil, err
	}
	if v.Live {
		return nil, refusal("live streams on video sites are not supported")
	}
	c := &SiteChoices{Title: v.Title, Qualities: v.Qualities()}
	if len(c.Qualities) == 0 {
		return nil, refusal("yt-dlp found nothing ODM can download on this page")
	}
	return c, nil
}

// runSite is Start for a video page.
func (d *Download) runSite(ctx context.Context) {
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
	o := ytdlp.Options{
		Page: d.URL, Format: d.Site.Format, Out: filepath.Join(work, "media"),
		FFmpeg: ff, RateLimit: d.SpeedLimit, Fragments: d.Connections,
	}
	if o.RateLimit <= 0 && d.shared != nil {
		o.RateLimit = d.shared.Rate() // the limit for all downloads, as yt-dlp's own
	}
	formats := strings.Count(d.Site.Format, "+") + 1
	d.mu.Unlock()

	// yt-dlp fetches the formats of a selector one after the other, each
	// with its own count.
	done, totals := map[string]int64{}, map[string]int64{}
	return ytdlp.Download(ctx, bin, o, func(p ytdlp.Progress) {
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
	})
}

// placeSite moves yt-dlp's file to the download's .odmpart, giving the
// name the container yt-dlp chose, so finish can name it.
func (d *Download) placeSite(file string) error {
	d.mu.Lock()
	name := d.FileName
	if name == "" {
		name = "video"
	}
	name = strings.TrimSuffix(name, filepath.Ext(name)) + filepath.Ext(file)
	d.FileName = name
	target := filepath.Join(d.Dir, name) + ".odmpart"
	d.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	if err := renameRetry(file, target); err != nil {
		return fmt.Errorf("moving the video into place: %w", err)
	}
	return nil
}
