package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"odm/internal/ffmpeg"
	"odm/internal/media"
)

// Live streams sent as one endless HTTP response.
//
// Some live players read a single response that never ends: TikTok's FLV,
// some TV and camera streams. ODM records it as it comes: it copies the
// response into the work folder until the download is stopped (Pause),
// the broadcast ends (the response closes and opening it again brings
// nothing for a while), or the connection stays silent. A connection that
// drops is opened again into a new part, because an FLV file cannot simply
// go on where another stopped. ffmpeg joins the parts into one MP4 at the
// end, Matroska when the streams do not fit MP4.
//
// Gotchas:
//   - The address is often signed and expires. With a Page (found by
//     yt-dlp), yt-dlp is asked for a fresh one when it is refused.
//   - Parts are numbered p1, p2… in the work folder, so a Resume after
//     ODM stopped mid-way (a crash) goes on with the next part, after a
//     gap.
//   - A response with a length that came whole is a file, not a stream:
//     opening it again would record it again, so the recording ends.
//   - Without ffmpeg the parts cannot be joined: the first becomes the
//     file as it came, the others are kept next to it.

// httpLiveExt is the container a part is saved as: what the response's
// URL ends in, FLV by default (TikTok's).
func httpLiveExt(raw string) string {
	switch ext := strings.ToLower(filepath.Ext(urlPath(raw))); ext {
	case ".flv", ".ts", ".mp4", ".mkv", ".webm":
		return ext
	}
	return ".flv"
}

// httpLiveParts lists the parts recorded so far, in order.
func (d *Download) httpLiveParts() []string {
	matches, _ := filepath.Glob(filepath.Join(d.workDir(), "p*.*"))
	type part struct {
		n    int
		path string
	}
	var parts []part
	for _, m := range matches {
		base := filepath.Base(m)
		n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(base, "p"), filepath.Ext(base)))
		if err == nil && n > 0 {
			parts = append(parts, part{n, m})
		}
	}
	sort.Slice(parts, func(i, j int) bool { return parts[i].n < parts[j].n })
	out := make([]string, len(parts))
	for i, p := range parts {
		out[i] = p.path
	}
	return out
}

// recordHTTPLive is Start for an endless HTTP live stream.
func (d *Download) recordHTTPLive(ctx context.Context) error {
	d.mu.Lock()
	d.Stream.Live = true
	job := *d.Stream
	d.mu.Unlock()
	if !job.Recording {
		d.mu.Lock()
		d.FileName = streamName(d.FileName, "mp4")
		d.mu.Unlock()
		if err := d.openTarget(true); err != nil { // claims the name
			return err
		}
		d.closeFile()
		// The name was just claimed: a work folder under it is a leftover.
		if err := os.RemoveAll(d.workDir()); err != nil {
			return err
		}
		d.mu.Lock()
		d.Stream.Recording = true
		d.RangeOK = true // resumable: Resume goes on recording
		d.mu.Unlock()
	}
	work := d.workDir()
	if err := os.MkdirAll(work, 0o755); err != nil {
		return err
	}
	parts := d.httpLiveParts()
	var done int64
	for _, p := range parts {
		done += fileSize(p)
	}
	d.mu.Lock()
	d.streamDone, d.Total = done, done
	d.mu.Unlock()

	next := len(parts) + 1
	address := job.Manifest
	lastData := time.Now()
	var stopErr error
	for failures := 0; ctx.Err() == nil; {
		part := filepath.Join(work, "p"+strconv.Itoa(next)+httpLiveExt(address))
		n, status, whole, err := d.copyHTTPLive(ctx, address, part)
		if n > 0 {
			next++
			lastData, failures = time.Now(), 0
		} else {
			os.Remove(part)
		}
		if ctx.Err() != nil || whole {
			break
		}
		refused := status == http.StatusForbidden || status == http.StatusNotFound || status == http.StatusGone
		if refused && job.Page != "" {
			// The signed address expired: yt-dlp finds the current one.
			if u, h, rerr := resolveSiteLive(ctx, job.Page, media.KindHTTPLive); rerr == nil && u != address {
				d.useSiteManifest(u, h)
				address = u
				continue
			}
		}
		if errors.Is(err, errLocalNetwork) {
			stopErr = err
			break
		}
		failures++
		if time.Since(lastData) > liveMinStall {
			break // over, or gone for good
		}
		wait := min(liveMinWait*time.Duration(failures), liveMaxWait)
		select {
		case <-ctx.Done():
		case <-time.After(wait):
		}
	}

	if d.isRemoved() {
		return errRemoved
	}
	parts = d.httpLiveParts()
	if len(parts) == 0 {
		if stopErr != nil {
			return stopErr
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("the live stream sent nothing to record")
	}
	// Stopping ends the recording: join even though ctx is done.
	if err := d.assembleHTTPLive(d.assemblyContext(), parts); err != nil {
		if d.isRemoved() {
			return errRemoved
		}
		return err
	}
	if d.workDir() != work {
		// The file took another extension: runStream clears the work
		// folder of the new name, this one is left to clear here.
		_ = os.RemoveAll(work)
	}
	if stopErr != nil {
		d.mu.Lock()
		d.Err = "recording ended early: " + stopErr.Error()
		d.mu.Unlock()
	}
	return nil
}

// closeFile closes the target a claim opened: assembly writes it anew.
func (d *Download) closeFile() {
	d.mu.Lock()
	f := d.file
	d.file = nil
	d.mu.Unlock()
	if f != nil {
		_ = f.Close()
	}
}

// copyHTTPLive copies one connection's response to path until it ends,
// stalls or ctx is done. It returns the bytes copied, the HTTP status (0
// when no response came) and whether the response had a length and came
// whole (a file).
func (d *Download) copyHTTPLive(ctx context.Context, address, path string) (int64, int, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return 0, 0, false, err
	}
	d.decorate(req)
	resp, err := d.authorizer().do(req, d.streamClient().Do)
	if err != nil {
		return 0, 0, false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return 0, resp.StatusCode, false, fmt.Errorf("live stream: %s", resp.Status)
	}
	f, err := os.Create(path)
	if err != nil {
		return 0, resp.StatusCode, false, err
	}
	defer f.Close()
	buf := make([]byte, 64<<10)
	idle := d.readStall()
	var total int64
	for {
		n, rerr := readWithIdle(ctx, resp.Body, buf, idle)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				return total, resp.StatusCode, false, werr
			}
			total += int64(n)
			d.mu.Lock()
			d.streamDone += int64(n)
			d.Total = d.streamDone
			lim := d.limiter
			d.mu.Unlock()
			d.notify(false)
			if werr := lim.Wait(ctx, n); werr != nil {
				return total, resp.StatusCode, false, werr
			}
		}
		if rerr != nil {
			whole := false
			if rerr == io.EOF {
				rerr = nil
				whole = resp.ContentLength > 0 && total == resp.ContentLength
			}
			return total, resp.StatusCode, whole, rerr
		}
	}
}

// assembleHTTPLive joins the parts into the download's .odmpart: one part
// remuxed, several concatenated, into MP4, else Matroska. Without ffmpeg
// (or when both fail) the first part becomes the file as it came and the
// others are kept next to it, numbered.
func (d *Download) assembleHTTPLive(ctx context.Context, parts []string) error {
	d.mu.Lock()
	dir, name := d.Dir, d.FileName
	d.mu.Unlock()
	work := d.workDir()
	base := strings.TrimSuffix(name, filepath.Ext(name))
	claimed := filepath.Join(dir, name) + ".odmpart"
	// place moves the result to the .odmpart of base+ext, the name the
	// file takes; the claimed (empty) .odmpart goes when the name changes.
	place := func(file, ext string) error {
		target := filepath.Join(dir, base+ext) + ".odmpart"
		if target != claimed {
			os.Remove(claimed)
		}
		if err := renameRetry(file, target); err != nil {
			return err
		}
		d.mu.Lock()
		d.FileName = base + ext
		d.mu.Unlock()
		return nil
	}
	if bin, err := locateFFmpeg(ctx); err == nil {
		for _, to := range []struct{ ext, format string }{{".mp4", "mp4"}, {".mkv", "matroska"}} {
			joined := filepath.Join(work, "joined"+to.ext)
			var jerr error
			if len(parts) == 1 {
				jerr = ffmpeg.Remux(ctx, bin, parts[0], joined, to.format)
			} else {
				jerr = ffmpeg.Concat(ctx, bin, parts, joined, to.format)
			}
			if jerr == nil {
				return place(joined, to.ext)
			}
			os.Remove(joined)
			if ctx.Err() != nil {
				return ctx.Err()
			}
		}
	}
	// No way to join: the first part as it came, the rest beside it
	// (first, while the name, and so the work folder, is the same).
	ext := filepath.Ext(parts[0])
	for i, p := range parts[1:] {
		alt := fmt.Sprintf("%s (part %d)%s", base, i+2, ext)
		if _, err := os.Stat(filepath.Join(dir, alt)); !os.IsNotExist(err) {
			if alt, err = freeName(dir, alt); err != nil {
				return err
			}
		}
		if err := renameRetry(p, filepath.Join(dir, alt)); err != nil {
			return err
		}
	}
	return place(parts[0], ext)
}
