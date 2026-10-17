// Package ytdlp runs yt-dlp, the program that reads video sites' pages
// (YouTube and many others), to list a video's formats and to download one.
//
// ODM does not ship yt-dlp; like ffmpeg it is optional and only needed for
// pages of such sites, whose videos are not files or playlists a browser
// can hand over: YouTube's player fetches signed pieces that only its own
// code can name, and yt-dlp is kept up to date with those changes.
//
// Gotchas:
//   - ODM on Windows is a GUI program; a console child flashes a window
//     unless it is started hidden.
//   - yt-dlp from pip is a launcher that starts Python, and the standalone
//     yt-dlp.exe unpacks itself and runs as a child process, so stopping a
//     download ends the whole process tree (killTree).
//   - Python writes in the console code page unless told otherwise;
//     PYTHONIOENCODING keeps titles intact.
//   - YouTube needs a JavaScript runtime for some formats. yt-dlp enables
//     only Deno by default, so Node is enabled when it is on PATH and Deno
//     is not (yt-dlp 2025.11 and later; older ones do not know the option).
//   - Pausing kills yt-dlp. Its .part files stay in the work folder and the
//     next run continues them; a format already complete is skipped.
package ytdlp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"odm/internal/tools"
)

// ErrNotFound means no yt-dlp was configured, bundled or on PATH.
var ErrNotFound = errors.New("yt-dlp not found: install it or set its path in Options")

func exeName() string {
	if runtime.GOOS == "windows" {
		return "yt-dlp.exe"
	}
	return "yt-dlp"
}

// Locate returns the yt-dlp to use, in order: the configured path (a file,
// or a folder containing yt-dlp), the copy ODM installed (tools.Dir), one
// next to the ODM executable, then PATH.
func Locate(configured string) (string, error) {
	if configured = strings.TrimSpace(configured); configured != "" {
		if st, err := os.Stat(configured); err == nil && st.IsDir() {
			configured = filepath.Join(configured, exeName())
		}
		if st, err := os.Stat(configured); err == nil && !st.IsDir() {
			return configured, nil
		}
		return "", fmt.Errorf("%w (looked at %s)", ErrNotFound, configured)
	}
	if p := tools.Find(exeName()); p != "" {
		return p, nil
	}
	if self, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(self), exeName())
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	if p, err := exec.LookPath("yt-dlp"); err == nil {
		return p, nil
	}
	return "", ErrNotFound
}

// Site names the video site of a page yt-dlp is used for, "" for any
// other address. Only these are sent to yt-dlp without being asked: for
// other pages the browser sees the files or playlists themselves.
func Site(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return ""
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	switch host {
	case "youtube.com", "m.youtube.com", "music.youtube.com", "youtu.be", "youtube-nocookie.com":
		return "YouTube"
	case "tiktok.com", "m.tiktok.com", "vm.tiktok.com":
		return "TikTok"
	}
	return ""
}

func command(ctx context.Context, bin string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, bin, args...)
	hide(cmd)
	cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8", "PYTHONUTF8=1")
	cmd.Cancel = func() error { return killTree(cmd.Process) }
	cmd.WaitDelay = 5 * time.Second
	return cmd
}

var versions sync.Map // bin -> version string

// Version is yt-dlp's version ("2026.09.27"), for Options.
func Version(ctx context.Context, bin string) (string, error) {
	out, err := command(ctx, bin, "--version").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// version is Version read once per program, "" when it does not run.
func version(ctx context.Context, bin string) string {
	if v, ok := versions.Load(bin); ok {
		return v.(string)
	}
	v, err := Version(ctx, bin)
	if err != nil {
		return ""
	}
	versions.Store(bin, v)
	return v
}

// baseArgs are the options every run takes.
// CookieBrowsers are the browsers yt-dlp can sign in to video sites with
// (--cookies-from-browser), reading their cookies itself, in the order the
// Options offer them.
var CookieBrowsers = []string{"firefox", "chrome", "edge", "brave", "opera", "vivaldi", "chromium"}

// CookieBrowser reports whether b is one of CookieBrowsers.
func CookieBrowser(b string) bool { return slices.Contains(CookieBrowsers, b) }

func baseArgs(ctx context.Context, bin, cookies string) []string {
	// --ignore-config: a user's yt-dlp config could change the output
	// name or format ODM asks for.
	args := []string{"--ignore-config", "--no-playlist", "--color", "never"}
	if cookies != "" {
		// Chosen in the Options: the site sees the person signed in, as
		// YouTube asks of some ("Sign in to confirm you're not a bot").
		args = append(args, "--cookies-from-browser", cookies)
	}
	if version(ctx, bin) >= "2025.11" {
		if _, err := exec.LookPath("deno"); err != nil {
			if _, err := exec.LookPath("node"); err == nil {
				args = append(args, "--js-runtimes", "node")
			}
		}
	}
	return args
}

// failure turns a failed run into an error, preferring yt-dlp's own
// "ERROR:" line, which says why.
func failure(ctx context.Context, err error, stderr string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	msg, last := "", ""
	for _, line := range strings.Split(stderr, "\n") {
		line = strings.TrimSpace(line)
		if rest, ok := strings.CutPrefix(line, "ERROR: "); ok {
			msg = rest
		}
		if line != "" {
			last = line
		}
	}
	if msg == "" {
		// No ERROR line: Python's own failure (a broken install) ends
		// its traceback with the reason.
		msg = last
	}
	if msg == "" {
		msg = err.Error()
	}
	return errors.New("yt-dlp: " + msg)
}

/* ---------- what a page offers ---------- */

// Video is what yt-dlp found at a page.
type Video struct {
	Title    string  `json:"title"`
	Duration float64 `json:"duration"` // seconds, 0 when unknown
	Live     bool    `json:"is_live"`
	// LiveStatus is "is_live", "is_upcoming", "was_live", "post_live" or
	// "not_live" ("" from extractors that do not say).
	LiveStatus string   `json:"live_status"`
	Formats    []Format `json:"formats"`
}

// IsLive reports a broadcast that is on now.
func (v *Video) IsLive() bool { return v.Live || v.LiveStatus == "is_live" }

// LiveManifest is the HLS playlist of a live video and the headers to
// fetch it with: the master playlist when yt-dlp names one (every
// quality, for ODM to pick from), else the best quality's own playlist.
func (v *Video) LiveManifest() (string, map[string]string, bool) {
	var best *Format
	for i := range v.Formats {
		f := &v.Formats[i]
		if !strings.HasPrefix(f.Protocol, "m3u8") {
			continue
		}
		if f.ManifestURL != "" {
			return f.ManifestURL, f.Headers, true
		}
		if f.URL != "" && (best == nil || f.Height > best.Height || (f.Height == best.Height && f.TBR > best.TBR)) {
			best = f
		}
	}
	if best == nil {
		return "", nil, false
	}
	return best.URL, best.Headers, true
}

// LiveDASH is the DASH manifest of a live video (Facebook's) and the
// headers to fetch it with.
func (v *Video) LiveDASH() (string, map[string]string, bool) {
	for _, f := range v.Formats {
		if f.ManifestURL != "" && (f.Protocol == "http_dash_segments" || strings.Contains(strings.ToLower(f.ManifestURL), ".mpd")) {
			return f.ManifestURL, f.Headers, true
		}
	}
	return "", nil, false
}

// LiveHTTP is the address of a live video sent as one endless HTTP
// response (TikTok's FLV) and the headers to fetch it with: the best
// format with both video and audio. yt-dlp lists formats worst first, so
// of two that look alike the later one wins.
func (v *Video) LiveHTTP() (string, map[string]string, bool) {
	var best *Format
	for i := range v.Formats {
		f := &v.Formats[i]
		if (f.Protocol != "http" && f.Protocol != "https") || f.URL == "" || f.VCodec == "none" || f.ACodec == "none" {
			continue
		}
		if best == nil || f.Height > best.Height || (f.Height == best.Height && f.TBR >= best.TBR) {
			best = f
		}
	}
	if best == nil {
		return "", nil, false
	}
	return best.URL, best.Headers, true
}

// Format is one of the video's encodings as yt-dlp lists it.
type Format struct {
	ID           string  `json:"format_id"`
	Ext          string  `json:"ext"`
	VCodec       string  `json:"vcodec"`
	ACodec       string  `json:"acodec"`
	Height       int     `json:"height"`
	FPS          float64 `json:"fps"`
	TBR          float64 `json:"tbr"` // kbit/s
	ABR          float64 `json:"abr"`
	Size         float64 `json:"filesize"`
	SizeApprox   float64 `json:"filesize_approx"`
	Protocol     string  `json:"protocol"`
	Note         string  `json:"format_note"`
	DynamicRange string  `json:"dynamic_range"`
	LangPref     float64 `json:"language_preference"`
	DRM          any     `json:"has_drm"` // true, false or "maybe"
	// URL is the format's own address and ManifestURL the playlist it
	// was listed in (HLS and DASH); Headers are what to fetch them with.
	URL         string            `json:"url"`
	ManifestURL string            `json:"manifest_url"`
	Headers     map[string]string `json:"http_headers"`
}

// Probe asks yt-dlp what the page offers, signed in with the cookies of
// the browser cookies names ("" for none; see CookieBrowsers).
func Probe(ctx context.Context, bin, page, cookies string) (*Video, error) {
	cmd := command(ctx, bin, append(baseArgs(ctx, bin, cookies), "-J", "--", page)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, failure(ctx, err, stderr.String())
	}
	var v Video
	if err := json.Unmarshal(stdout.Bytes(), &v); err != nil {
		return nil, fmt.Errorf("yt-dlp: unreadable answer: %w", err)
	}
	return &v, nil
}

// Quality is one way to download the video: the format selector for
// yt-dlp, how it reads, the container of the result and its expected size
// in bytes (0 when unknown).
type Quality struct {
	Label    string `json:"label"`
	Format   string `json:"format"`
	Ext      string `json:"ext"`
	Estimate int64  `json:"estimate,omitempty"`
}

func (f *Format) hasVideo() bool { return f.VCodec != "none" && (f.VCodec != "" || f.Height > 0) }
func (f *Format) hasAudio() bool { return f.ACodec != "none" && f.ACodec != "" }

// usable leaves out what cannot or should not be offered: DRM, storyboard
// images, and YouTube's "DRC" audio, a louder copy of a listed track.
func (f *Format) usable() bool {
	if drm, _ := f.DRM.(bool); drm {
		return false
	}
	if f.Protocol == "mhtml" || (!f.hasVideo() && !f.hasAudio()) {
		return false
	}
	return !strings.HasSuffix(f.ID, "-drc")
}

// segmented reports a format fetched as a playlist of pieces; one fetched
// as a single file is preferred when both exist.
func (f *Format) segmented() bool {
	return strings.HasPrefix(f.Protocol, "m3u8") || strings.HasPrefix(f.Protocol, "http_dash")
}

// size is the expected size in bytes for a video of duration seconds.
func (f *Format) size(duration float64) int64 {
	switch {
	case f.Size > 0:
		return int64(f.Size)
	case f.SizeApprox > 0:
		return int64(f.SizeApprox)
	case f.TBR > 0 && duration > 0:
		return int64(f.TBR * 1000 / 8 * duration)
	}
	return 0
}

// videoRank orders codecs by how widely they play: H.264 first.
func videoRank(codec string) int {
	switch c := strings.ToLower(codec); {
	case strings.HasPrefix(c, "avc"), strings.HasPrefix(c, "h264"):
		return 0
	case strings.HasPrefix(c, "vp9"), strings.HasPrefix(c, "vp09"):
		return 1
	case strings.HasPrefix(c, "av01"):
		return 2
	case strings.HasPrefix(c, "hev"), strings.HasPrefix(c, "hvc"), strings.HasPrefix(c, "h265"):
		return 3
	}
	return 4
}

func videoCodec(codec string) string {
	switch c := strings.ToLower(codec); {
	case strings.HasPrefix(c, "avc"), strings.HasPrefix(c, "h264"):
		return "H.264"
	case strings.HasPrefix(c, "vp9"), strings.HasPrefix(c, "vp09"):
		return "VP9"
	case strings.HasPrefix(c, "vp8"):
		return "VP8"
	case strings.HasPrefix(c, "av01"):
		return "AV1"
	case strings.HasPrefix(c, "hev"), strings.HasPrefix(c, "hvc"), strings.HasPrefix(c, "h265"):
		return "H.265"
	}
	return strings.ToUpper(strings.SplitN(codec, ".", 2)[0])
}

func audioCodec(codec string) string {
	switch c := strings.ToLower(codec); {
	case strings.HasPrefix(c, "mp4a"), c == "aac":
		return "AAC"
	case c == "opus":
		return "Opus"
	case c == "vorbis":
		return "Vorbis"
	case c == "mp3":
		return "MP3"
	case c == "flac":
		return "FLAC"
	case strings.HasPrefix(c, "ac-3"), c == "ac3":
		return "AC-3"
	case strings.HasPrefix(c, "ec-3"), c == "eac3":
		return "E-AC-3"
	}
	return strings.ToUpper(strings.SplitN(codec, ".", 2)[0])
}

func bitrate(kbps float64) string {
	switch {
	case kbps <= 0:
		return ""
	case kbps >= 1000:
		return fmt.Sprintf("%.1f Mbps", kbps/1000)
	}
	return fmt.Sprintf("%.0f kbps", kbps)
}

func join(parts ...string) string {
	var out []string
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, " · ")
}

// bestAudio is the audio to pair with a video in container ext ("mp4" or
// "webm"): the original language first, then one that fits the container,
// then a single file over pieces, then the higher bitrate.
func bestAudio(audios []*Format, ext string) *Format {
	var best *Format
	better := func(a, b *Format) bool {
		if a.LangPref != b.LangPref {
			return a.LangPref > b.LangPref
		}
		if fa, fb := fits(a, ext), fits(b, ext); fa != fb {
			return fa
		}
		if a.segmented() != b.segmented() {
			return !a.segmented()
		}
		return audioRate(a) > audioRate(b)
	}
	for _, a := range audios {
		if best == nil || better(a, best) {
			best = a
		}
	}
	return best
}

func fits(a *Format, ext string) bool {
	if ext == "webm" {
		return a.Ext == "webm"
	}
	return a.Ext == "m4a" || a.Ext == "mp4"
}

func audioRate(a *Format) float64 {
	if a.ABR > 0 {
		return a.ABR
	}
	return a.TBR
}

// merged is the container yt-dlp gives video and audio joined: theirs when
// they agree, Matroska when they do not.
func merged(video, audio string) string {
	switch {
	case video == "webm" && audio == "webm":
		return "webm"
	case video == "mp4" && (audio == "m4a" || audio == "mp4"):
		return "mp4"
	}
	return "mkv"
}

// Qualities lists what to offer: per resolution the most widely playable
// video (H.264, then VP9, AV1, H.265) joined with the audio that fits it,
// highest first, then the best audio alone per container. Labels are
// unique, so one can name its quality.
func (v *Video) Qualities() []Quality {
	var videos, audios []*Format
	for i := range v.Formats {
		f := &v.Formats[i]
		switch {
		case !f.usable():
		case f.hasVideo():
			videos = append(videos, f)
		default:
			audios = append(audios, f)
		}
	}

	type key struct {
		height int
		fps    int
		hdr    bool
	}
	// better: the more widely playable codec, then a single file over
	// pieces, then the higher bitrate.
	better := func(a, b *Format) bool {
		if ra, rb := videoRank(a.VCodec), videoRank(b.VCodec); ra != rb {
			return ra < rb
		}
		if a.segmented() != b.segmented() {
			return !a.segmented()
		}
		return a.TBR > b.TBR
	}
	pick := map[key]*Format{}
	for _, f := range videos {
		k := key{f.Height, int(math.Round(f.FPS)), f.DynamicRange != "" && f.DynamicRange != "SDR"}
		if k.fps <= 30 {
			k.fps = 0
		}
		if cur := pick[k]; cur == nil || better(f, cur) {
			pick[k] = f
		}
	}
	keys := make([]key, 0, len(pick))
	for k := range pick {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		if a.height != b.height {
			return a.height > b.height
		}
		if a.fps != b.fps {
			return a.fps > b.fps
		}
		return a.hdr && !b.hdr
	})

	var out []Quality
	for _, k := range keys {
		f := pick[k]
		res := ""
		if f.Height > 0 {
			res = fmt.Sprintf("%dp", f.Height)
			if k.fps > 0 {
				res += strconv.Itoa(k.fps)
			}
			if k.hdr {
				res += " HDR"
			}
		}
		q := Quality{Format: f.ID, Ext: f.Ext, Estimate: f.size(v.Duration)}
		codecs, rate := videoCodec(f.VCodec), f.TBR
		if f.hasAudio() {
			codecs += ", " + audioCodec(f.ACodec)
		} else if a := bestAudio(audios, f.Ext); a != nil {
			q.Format = f.ID + "+" + a.ID
			q.Ext = merged(f.Ext, a.Ext)
			codecs += ", " + audioCodec(a.ACodec)
			rate += audioRate(a)
			if s := a.size(v.Duration); s > 0 && q.Estimate > 0 {
				q.Estimate += s
			}
		}
		q.Label = join(res, bitrate(rate), codecs, strings.ToUpper(q.Ext))
		out = append(out, q)
	}

	// The best audio alone, once per container.
	byExt := map[string]*Format{}
	var exts []string
	for _, a := range audios {
		e := a.Ext
		if byExt[e] == nil {
			exts = append(exts, e)
		}
		if cur := byExt[e]; cur == nil || bestAudio([]*Format{cur, a}, e) == a {
			byExt[e] = a
		}
	}
	sort.SliceStable(exts, func(i, j int) bool { return audioRate(byExt[exts[i]]) > audioRate(byExt[exts[j]]) })
	for _, e := range exts {
		a := byExt[e]
		out = append(out, Quality{
			Label:    join("Audio only", bitrate(audioRate(a)), audioCodec(a.ACodec), strings.ToUpper(a.Ext)),
			Format:   a.ID,
			Ext:      a.Ext,
			Estimate: a.size(v.Duration),
		})
	}

	seen := map[string]int{}
	for i := range out {
		if n := seen[out[i].Label]; n > 0 {
			seen[out[i].Label]++
			out[i].Label = fmt.Sprintf("%s (%d)", out[i].Label, n+1)
		} else {
			seen[out[i].Label] = 1
		}
	}
	return out
}

/* ---------- downloading ---------- */

// Options describe one download.
type Options struct {
	Page   string
	Format string // yt-dlp's format selector; "" for its own best
	// Out is the file to write without its extension; yt-dlp adds the
	// container's. Its folder holds yt-dlp's partial files.
	Out       string
	FFmpeg    string // "" lets yt-dlp look for ffmpeg itself
	RateLimit int64  // bytes per second, 0 for none
	Fragments int    // pieces fetched at once for formats that come in pieces
	Cookies   string // the browser to sign in with (CookieBrowsers), "" for none
}

// Progress is yt-dlp's report on the format it is fetching.
type Progress struct {
	Format  string
	Done    int64
	Total   int64 // 0 when unknown
	Merging bool  // the video and audio are being joined
}

// progressTemplate makes yt-dlp report on lines of its own:
// "odm <status> <downloaded> <total> <estimated total> <format id>".
const progressTemplate = "download:odm %(progress.status)s %(progress.downloaded_bytes)s %(progress.total_bytes)s %(progress.total_bytes_estimate)s %(info.format_id)s"

func args(o Options) []string {
	out := strings.ReplaceAll(o.Out, "%", "%%") + ".%(ext)s"
	a := []string{"--no-mtime", "--newline", "--progress-template", progressTemplate, "-o", out}
	if o.Format != "" {
		a = append(a, "-f", o.Format)
	}
	if o.FFmpeg != "" {
		a = append(a, "--ffmpeg-location", o.FFmpeg)
	}
	if o.RateLimit > 0 {
		a = append(a, "--limit-rate", strconv.FormatInt(o.RateLimit, 10))
	}
	if o.Fragments > 1 {
		a = append(a, "-N", strconv.Itoa(o.Fragments))
	}
	return append(a, "--", o.Page)
}

// parseProgress reads one line of yt-dlp's output.
func parseProgress(line string) (Progress, bool) {
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "[Merger]") {
		return Progress{Merging: true}, true
	}
	f := strings.Fields(line)
	if len(f) < 6 || f[0] != "odm" {
		return Progress{}, false
	}
	num := func(s string) int64 {
		n, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0
		}
		return int64(n)
	}
	p := Progress{Format: f[5], Done: num(f[2]), Total: num(f[3])}
	if p.Total <= 0 {
		p.Total = num(f[4])
	}
	return p, true
}

// Download runs yt-dlp until the video is in Out plus its extension, and
// returns that file. progress is called for each report.
func Download(ctx context.Context, bin string, o Options, progress func(Progress)) (string, error) {
	cmd := command(ctx, bin, append(baseArgs(ctx, bin, o.Cookies), args(o)...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", err
	}
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		if p, ok := parseProgress(sc.Text()); ok && progress != nil {
			progress(p)
		}
	}
	_, _ = io.Copy(io.Discard, stdout)
	if err := cmd.Wait(); err != nil {
		return "", failure(ctx, err, stderr.String())
	}
	return result(o.Out)
}

// result finds the file yt-dlp made: out plus one extension, not a part
// file or the ".f137.mp4" piece of a format joined into it; the largest,
// should anything else share the name.
func result(out string) (string, error) {
	matches, _ := filepath.Glob(strings.NewReplacer("[", "[[]", "*", "[*]", "?", "[?]").Replace(out) + ".*")
	base := filepath.Base(out)
	best, size := "", int64(-1)
	for _, m := range matches {
		rest := strings.TrimPrefix(filepath.Base(m), base+".")
		if rest == "" || strings.Contains(rest, ".") {
			continue
		}
		switch rest {
		case "part", "ytdl", "temp":
			continue
		}
		if st, err := os.Stat(m); err == nil && !st.IsDir() && st.Size() > size {
			best, size = m, st.Size()
		}
	}
	if best == "" {
		return "", errors.New("yt-dlp finished but left no file")
	}
	return best, nil
}
