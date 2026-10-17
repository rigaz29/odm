package engine

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"odm/internal/media"
)

// ErrStreamRefused marks a playlist that can never be downloaded as it is
// (live, DRM-protected, unsupported format, not a playlist), as opposed to
// a probe that failed for now (timeout, server error) and may work when the
// download itself runs.
var ErrStreamRefused = errors.New("stream refused")

type refusal string

func (r refusal) Error() string        { return string(r) }
func (r refusal) Is(target error) bool { return target == ErrStreamRefused }

// StreamChoices is what a playlist offers, for the New Download window to
// let the person pick a quality before the download is created.
type StreamChoices struct {
	Kind media.Kind
	// Qualities is ordered by resolution, then bitrate, highest first.
	// ODM's own pick when nobody chooses (bestHLS, BestDASH) is the highest
	// bitrate, which is usually but not always the first.
	Qualities []StreamQuality
	// Note explains a limitation of this playlist, "" when there is none.
	Note string
	// Single is set for an address that is one quality of a stream (a
	// media playlist) rather than the playlist that lists them.
	Single bool
	// Live is a broadcast on now: downloading it records it until stopped.
	Live bool
	// length is the stream's duration, 0 when unknown.
	length time.Duration
}

// StreamQuality is one video quality and the audio that can go with it.
// Labels are unique within a playlist, so a list of labels can be mapped
// back to the quality.
type StreamQuality struct {
	Label string
	// Estimate is the expected size in bytes with the first audio, 0 when
	// the playlist does not say how long the stream is.
	Estimate int64
	// Audios are the audio tracks to choose from, default first; empty
	// when the audio is inside the video (or there is none). An entry may
	// be the audio inside the video, offered next to separate languages.
	Audios []StreamAudio
	key    string // HLS variant key or DASH Representation ID
	bps    int64  // HLS: the whole variant; DASH: the video alone
}

// StreamAudio is one audio track.
type StreamAudio struct {
	Label string
	key   string // HLS rendition key or DASH Representation ID; "" = in the video
	bps   int64  // DASH only: added to the video's bitrate
}

// Quality returns the index of the quality labelled label, or -1 when
// there is none (the playlist changed since the label was read).
func (c *StreamChoices) Quality(label string) int {
	for i, q := range c.Qualities {
		if q.Label == label {
			return i
		}
	}
	return -1
}

// Job returns the StreamJob for quality q and audio a (an index into
// q.Audios, ignored when it has none) of a playlist found at manifest.
// The choice is pinned, so a resume or an expired link cannot switch to
// another encoding.
func (c *StreamChoices) Job(manifest string, q, a int) (StreamJob, error) {
	if q < 0 || q >= len(c.Qualities) {
		return StreamJob{}, errors.New("no such quality")
	}
	sq := c.Qualities[q]
	var au StreamAudio
	if len(sq.Audios) > 0 {
		if a < 0 || a >= len(sq.Audios) {
			a = 0
		}
		au = sq.Audios[a]
	}
	job := StreamJob{Kind: c.Kind, Manifest: manifest}
	switch c.Kind {
	case media.KindHLS:
		job.Variant, job.AudioKey = sq.key, au.key
		job.Estimate = estimate(sq.bps, c.length)
	case media.KindDASH:
		job.VideoRep, job.AudioRep = sq.key, au.key
		job.Estimate = estimate(sq.bps+au.bps, c.length)
	case media.KindHTTPLive:
		job.Live = true
	}
	return job, nil
}

// HTTPLiveChoices is what a live stream sent as one endless response
// offers: one quality, as the server sends it. Its address is not read
// first, as a playlist is: the response never ends.
func HTTPLiveChoices() *StreamChoices {
	return &StreamChoices{Kind: media.KindHTTPLive, Live: true, Note: liveNote,
		Qualities: []StreamQuality{{Label: "As sent (one quality)"}}}
}

// ProbeStream reads the playlist at rawURL and lists its qualities. The
// format is taken from the playlist itself, so it also works for an
// address typed or pasted by hand. Playlists that cannot be downloaded
// (live, DRM) are reported as errors matching ErrStreamRefused.
func (m *Manager) ProbeStream(ctx context.Context, rawURL string, headers map[string]string) (*StreamChoices, error) {
	rawURL = strings.TrimSpace(rawURL)
	// A throwaway download gives the probe the same headers, cookie
	// scoping and credentials the real one will use.
	d := New("", rawURL, "", 1, headers, 0, m.HTTPClient())
	d.SetAuthLookup(m.credentials)
	d.Stream = &StreamJob{Manifest: rawURL}
	// The probe's guarded client has a connection pool of its own.
	defer d.guardedClient().CloseIdleConnections()
	data, base, err := d.fetchText(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	var c *StreamChoices
	switch media.Detect("", base.String(), data) {
	case media.KindHLS:
		c, err = d.probeHLS(ctx, base, data)
	case media.KindDASH:
		c, err = probeDASH(base, data)
	case media.KindSmooth, media.KindF4M:
		return nil, refusal("this stream format is not supported")
	default:
		return nil, refusal("not an HLS or DASH playlist")
	}
	if err != nil {
		return nil, err
	}
	uniqueLabels(c)
	return c, nil
}

func (d *Download) probeHLS(ctx context.Context, base *url.URL, data []byte) (*StreamChoices, error) {
	p, err := media.ParseHLS(base, data)
	if err != nil {
		return nil, refusal(err.Error())
	}
	c := &StreamChoices{Kind: media.KindHLS}
	if p.Master == nil {
		if err := checkHLSMedia(p.Media); err != nil {
			return nil, err
		}
		c.Qualities = []StreamQuality{{Label: "As found (one quality)"}}
		c.Single = true
		c.Note = "This address is one quality of a stream, and its audio may be separate. " +
			"Download from the page's main playlist to choose the quality."
		if p.Media.Live() {
			c.Live = true
			c.Note = liveNote + " " + c.Note
		} else {
			c.length = hlsLength(p.Media)
		}
		return c, nil
	}
	if len(p.Master.Variants) == 0 {
		return nil, refusal("the playlist lists no video")
	}

	// The length is in the media playlists only. One is read (the best
	// quality's) for the estimate and to refuse live or DRM streams now;
	// the variants of one stream have the same length. A failed read is
	// returned as it is: the download itself may still get through.
	best, _, _ := bestHLS(p.Master)
	body, mbase, err := d.fetchText(ctx, best.URL)
	if err != nil {
		return nil, err
	}
	mp, err := media.ParseHLS(mbase, body)
	if err != nil {
		return nil, refusal(err.Error())
	}
	if mp.Media == nil {
		return nil, refusal("expected a media playlist, got a master playlist")
	}
	if err := checkHLSMedia(mp.Media); err != nil {
		return nil, err
	}
	if mp.Media.Live() {
		// No length, so no size estimate.
		c.Live = true
		c.Note = liveNote
	} else {
		c.length = hlsLength(mp.Media)
	}

	vs := make([]*media.HLSVariant, len(p.Master.Variants))
	for i := range p.Master.Variants {
		vs[i] = &p.Master.Variants[i]
	}
	sort.SliceStable(vs, func(i, j int) bool {
		if vs[i].Height != vs[j].Height {
			return vs[i].Height > vs[j].Height
		}
		return vs[i].Bandwidth > vs[j].Bandwidth
	})
	for _, v := range vs {
		// AVERAGE-BANDWIDTH is the average; BANDWIDTH is the peak segment
		// rate with the heaviest audio (RFC 8216 §4.3.4.2), an upper bound
		// that overstates variable-bitrate video.
		bps := v.AverageBandwidth
		if bps <= 0 {
			bps = v.Bandwidth
		}
		q := StreamQuality{Label: hlsLabel(v), key: hlsVariantKey(*v), bps: bps, Audios: hlsAudios(p.Master, v.Audio)}
		q.Estimate = estimate(bps, c.length)
		c.Qualities = append(c.Qualities, q)
	}
	return c, nil
}

// hlsAudios lists the audio of a variant's group, default first. A
// default rendition without a URI is the audio muxed into the variant; it
// is offered as "In the video" so it stays the default.
func hlsAudios(m *media.HLSMaster, group string) []StreamAudio {
	if group == "" {
		return nil
	}
	var out []StreamAudio
	muxedDefault := false
	for i := range m.Renditions {
		r := &m.Renditions[i]
		if r.Type != "AUDIO" || r.GroupID != group {
			continue
		}
		if r.URL == "" {
			muxedDefault = muxedDefault || r.Default
			continue
		}
		a := StreamAudio{Label: renditionLabel(r), key: hlsAudioKey(*r)}
		if r.Default {
			out = append([]StreamAudio{a}, out...)
		} else {
			out = append(out, a)
		}
	}
	if muxedDefault && len(out) > 0 {
		out = append([]StreamAudio{{Label: "In the video"}}, out...)
	}
	return out
}

func hlsLength(m *media.HLSMedia) time.Duration {
	var n time.Duration
	for _, s := range m.Segments {
		n += s.Duration
	}
	return n
}

// checkHLSMedia refuses what cannot be downloaded. A live playlist is
// recorded (live.go); it may not list a segment yet.
func checkHLSMedia(m *media.HLSMedia) error {
	switch {
	case m.Protected:
		return refusal("this stream is DRM-protected and cannot be downloaded")
	case !m.Live() && len(m.Segments) == 0:
		return refusal("the playlist has no segments")
	}
	return nil
}

// liveNote is shown in the window for a live stream.
const liveNote = "Live stream: ODM records it from now until you stop the download."

func probeDASH(base *url.URL, data []byte) (*StreamChoices, error) {
	m, err := media.ParseDASH(base, data)
	if err != nil {
		return nil, refusal(err.Error())
	}
	if len(m.Periods) == 0 {
		return nil, refusal("the manifest lists no audio or video")
	}
	// The first period lists the tracks; later ones (ads, chapters) are
	// matched by ID or kind when downloading, as in loadDASH. A live
	// manifest is recorded from the period on now, its last.
	period := m.Periods[0]
	if m.Live {
		period = m.Periods[len(m.Periods)-1]
	}
	var videos, audios []*media.DASHRepresentation
	lockedAudio, lockedVideo := false, false
	for i := range period.Representations {
		r := &period.Representations[i]
		switch {
		case r.IsVideo() && r.Protected:
			lockedVideo = true
		case r.IsVideo():
			videos = append(videos, r)
		case r.IsAudio() && r.Protected:
			lockedAudio = true
		case r.IsAudio():
			audios = append(audios, r)
		}
	}
	if len(videos) == 0 && len(audios) == 0 {
		return nil, refusal("this stream is DRM-protected and cannot be downloaded")
	}
	sort.SliceStable(videos, func(i, j int) bool {
		if videos[i].Height != videos[j].Height {
			return videos[i].Height > videos[j].Height
		}
		return videos[i].Bandwidth > videos[j].Bandwidth
	})
	sort.SliceStable(audios, func(i, j int) bool { return audios[i].Bandwidth > audios[j].Bandwidth })
	var tracks []StreamAudio
	for _, a := range audios {
		tracks = append(tracks, StreamAudio{Label: dashAudioLabel(a), key: a.ID, bps: a.Bandwidth})
	}
	c := &StreamChoices{Kind: media.KindDASH, length: m.Duration}
	for _, v := range videos {
		q := StreamQuality{Label: dashVideoLabel(v), key: v.ID, bps: v.Bandwidth, Audios: tracks}
		q.Estimate = estimate(q.bps+firstBps(tracks), c.length)
		c.Qualities = append(c.Qualities, q)
	}
	if len(videos) == 0 {
		// Audio only: one "quality" whose choice is the audio track.
		c.Qualities = []StreamQuality{{Label: "Audio only", Audios: tracks, Estimate: estimate(firstBps(tracks), c.length)}}
		if lockedVideo {
			c.Note = "The video is DRM-protected: only the audio can be downloaded."
		}
	}
	if lockedAudio && len(audios) == 0 && len(videos) > 0 {
		c.Note = "The audio is DRM-protected: only the video can be downloaded."
	}
	if m.Live {
		c.Live = true
		c.Note = strings.TrimSpace(liveNote + " " + c.Note)
	}
	return c, nil
}

func firstBps(a []StreamAudio) int64 {
	if len(a) == 0 {
		return 0
	}
	return a[0].bps
}

// uniqueLabels numbers labels that read the same ("720p · 2.0 Mbps (2)"):
// the window shows labels, and two equal ones could not be told apart.
func uniqueLabels(c *StreamChoices) {
	seen := map[string]int{}
	for i := range c.Qualities {
		q := &c.Qualities[i]
		if n := seen[q.Label]; n > 0 {
			seen[q.Label]++
			q.Label = fmt.Sprintf("%s (%d)", q.Label, n+1)
		} else {
			seen[q.Label] = 1
		}
		// Audio lists are shared between DASH qualities: copy before
		// renaming so each quality's list is numbered once.
		audios := make([]StreamAudio, len(q.Audios))
		copy(audios, q.Audios)
		seenA := map[string]int{}
		for j := range audios {
			l := audios[j].Label
			if n := seenA[l]; n > 0 {
				seenA[l]++
				audios[j].Label = fmt.Sprintf("%s (%d)", l, n+1)
			} else {
				seenA[l] = 1
			}
		}
		q.Audios = audios
	}
}

// estimate is bits per second over a duration, in bytes.
func estimate(bps int64, d time.Duration) int64 {
	if bps <= 0 || d <= 0 {
		return 0
	}
	return int64(float64(bps) / 8 * d.Seconds())
}

func hlsLabel(v *media.HLSVariant) string {
	return strings.Join(nonEmpty([]string{resolution(v.Height, v.FrameRate), bitrate(v.Bandwidth), codecNames(v.Codecs)}), " · ")
}

func dashVideoLabel(r *media.DASHRepresentation) string {
	return strings.Join(nonEmpty([]string{resolution(r.Height, frameRate(r.FrameRate)), bitrate(r.Bandwidth), codecNames(r.Codecs)}), " · ")
}

func dashAudioLabel(r *media.DASHRepresentation) string {
	return strings.Join(nonEmpty([]string{r.Lang, codecNames(r.Codecs), bitrate(r.Bandwidth)}), " · ")
}

// resolution is "1080p", or "1080p60" above 30 fps (24-30 is the norm).
func resolution(height int, fps float64) string {
	if height <= 0 {
		return ""
	}
	s := fmt.Sprintf("%dp", height)
	if fps > 30.5 {
		s += fmt.Sprintf("%.0f", fps)
	}
	return s
}

// frameRate reads a DASH frame rate: "50" or "60000/1001".
func frameRate(s string) float64 {
	num, den, ok := strings.Cut(s, "/")
	n, err := strconv.ParseFloat(num, 64)
	if err != nil {
		return 0
	}
	if ok {
		d, err := strconv.ParseFloat(den, 64)
		if err != nil || d == 0 {
			return 0
		}
		n /= d
	}
	return n
}

func renditionLabel(r *media.HLSRendition) string {
	name := r.Name
	if r.Language != "" && !strings.EqualFold(name, r.Language) {
		name = strings.TrimSpace(name + " (" + r.Language + ")")
	}
	if name == "" {
		name = "Audio"
	}
	return name
}

func bitrate(bps int64) string {
	switch {
	case bps <= 0:
		return ""
	case bps >= 1_000_000:
		return fmt.Sprintf("%.1f Mbps", float64(bps)/1e6)
	}
	return fmt.Sprintf("%d kbps", bps/1000)
}

// codecNames turns RFC 6381 codec strings ("avc1.640028,mp4a.40.2") into
// names people know ("H.264, AAC"); unknown ones are left out.
func codecNames(codecs string) string {
	var out []string
	seen := map[string]bool{}
	for _, c := range strings.Split(codecs, ",") {
		c = strings.ToLower(strings.TrimSpace(c))
		name := ""
		switch {
		case strings.HasPrefix(c, "avc1"), strings.HasPrefix(c, "avc3"):
			name = "H.264"
		case strings.HasPrefix(c, "hvc1"), strings.HasPrefix(c, "hev1"):
			name = "H.265"
		case strings.HasPrefix(c, "av01"):
			name = "AV1"
		case strings.HasPrefix(c, "vp09"), c == "vp9":
			name = "VP9"
		case strings.HasPrefix(c, "mp4a"):
			name = "AAC"
		case c == "opus":
			name = "Opus"
		case c == "ec-3":
			name = "E-AC-3"
		case c == "ac-3":
			name = "AC-3"
		}
		if name != "" && !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	return strings.Join(out, ", ")
}

func nonEmpty(s []string) []string {
	out := s[:0]
	for _, x := range s {
		if x != "" {
			out = append(out, x)
		}
	}
	return out
}
