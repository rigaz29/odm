package engine

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"odm/internal/media"
)

const probeMaster = `#EXTM3U
#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="aud",NAME="Deutsch",LANGUAGE="de",URI="de.m3u8"
#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="aud",NAME="English",LANGUAGE="en",DEFAULT=YES,URI="en.m3u8"
#EXT-X-STREAM-INF:BANDWIDTH=800000,RESOLUTION=1280x720,CODECS="avc1.64001f,mp4a.40.2",AUDIO="aud"
720.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=5000000,RESOLUTION=1920x1080,FRAME-RATE=60,CODECS="avc1.640028,mp4a.40.2",AUDIO="aud"
1080.m3u8
`

func probeServer(t *testing.T, files map[string]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestProbeStreamHLSMaster(t *testing.T) {
	const twelveSeconds = "#EXTM3U\n#EXT-X-TARGETDURATION:4\n#EXTINF:4,\na.ts\n#EXTINF:4,\nb.ts\n#EXTINF:4,\nc.ts\n#EXT-X-ENDLIST\n"
	srv := probeServer(t, map[string]string{"/master.m3u8": probeMaster, "/1080.m3u8": twelveSeconds})
	m := NewManager(DefaultSettings(t.TempDir()), "")

	c, err := m.ProbeStream(context.Background(), srv.URL+"/master.m3u8", nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.Kind != media.KindHLS || len(c.Qualities) != 2 || c.Note != "" {
		t.Fatalf("%+v", c)
	}
	best := c.Qualities[0]
	if best.Label != "1080p60 · 5.0 Mbps · H.264, AAC" || c.Qualities[1].Label != "720p · 800 kbps · H.264, AAC" {
		t.Fatalf("labels %q, %q", best.Label, c.Qualities[1].Label)
	}
	if best.Estimate != 5000000/8*12 {
		t.Fatalf("estimate %d", best.Estimate)
	}
	if len(best.Audios) != 2 || best.Audios[0].Label != "English (en)" || best.Audios[1].Label != "Deutsch (de)" {
		t.Fatalf("audios %+v", best.Audios)
	}

	job, err := c.Job(srv.URL+"/master.m3u8", 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(job.Variant, "720.m3u8") || !strings.Contains(job.AudioKey, "de.m3u8") || job.Kind != media.KindHLS {
		t.Fatalf("job %+v", job)
	}
	if job.Estimate != 800000/8*12 {
		t.Fatalf("job estimate %d", job.Estimate)
	}
	if _, err := c.Job("", 5, 0); err == nil {
		t.Fatal("out-of-range quality accepted")
	}
}

// The quality picked in the window is the one downloaded, not the best.
func TestProbeStreamChoiceIsDownloaded(t *testing.T) {
	master := "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=900000,RESOLUTION=1920x1080\nhi.m3u8\n" +
		"#EXT-X-STREAM-INF:BANDWIDTH=100000,RESOLUTION=640x360\nlo.m3u8\n"
	pl := func(p string) string {
		return "#EXTM3U\n#EXT-X-TARGETDURATION:2\n#EXTINF:2,\n" + p + "1.ts\n#EXTINF:2,\n" + p + "2.ts\n#EXT-X-ENDLIST\n"
	}
	srv := probeServer(t, map[string]string{
		"/m.m3u8": master, "/hi.m3u8": pl("hi"), "/lo.m3u8": pl("lo"),
		"/hi1.ts": "H1", "/hi2.ts": "H2", "/lo1.ts": "L1", "/lo2.ts": "L2",
	})
	m := NewManager(DefaultSettings(t.TempDir()), "")
	c, err := m.ProbeStream(context.Background(), srv.URL+"/m.m3u8", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(c.Qualities[1].Label, "360p") {
		t.Fatalf("order %+v", c.Qualities)
	}
	job, _ := c.Job(srv.URL+"/m.m3u8", 1, 0)
	d := newStreamDownload(t, srv.Client(), 2, job)
	d.Start(context.Background())
	if got := string(readResult(t, d)); got != "L1L2" {
		t.Fatalf("downloaded %q, want the low quality", got)
	}
}

func TestProbeStreamHLSMediaPlaylist(t *testing.T) {
	srv := probeServer(t, map[string]string{"/v.m3u8": threeSegs})
	m := NewManager(DefaultSettings(t.TempDir()), "")
	c, err := m.ProbeStream(context.Background(), srv.URL+"/v.m3u8", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Qualities) != 1 || c.Note == "" {
		t.Fatalf("%+v", c)
	}
	if job, _ := c.Job(srv.URL+"/v.m3u8", 0, 0); job.Variant != "" || job.Manifest != srv.URL+"/v.m3u8" {
		t.Fatalf("job %+v", job)
	}
}

func TestProbeStreamDASH(t *testing.T) {
	srv := probeServer(t, map[string]string{"/m.mpd": twoTrackMPD})
	m := NewManager(DefaultSettings(t.TempDir()), "")
	c, err := m.ProbeStream(context.Background(), srv.URL+"/m.mpd", nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.Kind != media.KindDASH || len(c.Qualities) != 2 || len(c.Qualities[0].Audios) != 1 {
		t.Fatalf("%+v", c)
	}
	if c.Qualities[0].Estimate != (9000+128)/8*4 {
		t.Fatalf("estimate %d", c.Qualities[0].Estimate)
	}
	job, _ := c.Job(srv.URL+"/m.mpd", 1, 0)
	if job.VideoRep != "v1" || job.AudioRep != "a1" {
		t.Fatalf("job %+v", job)
	}
}

func TestProbeStreamRefusals(t *testing.T) {
	live := "#EXTM3U\n#EXT-X-TARGETDURATION:2\n#EXTINF:2,\na.ts\n"
	drm := "#EXTM3U\n#EXT-X-TARGETDURATION:2\n#EXT-X-KEY:METHOD=SAMPLE-AES,URI=\"skd://k\"\n#EXTINF:2,\na.ts\n#EXT-X-ENDLIST\n"
	srv := probeServer(t, map[string]string{
		"/live.m3u8": live, "/drm.m3u8": drm, "/page.html": "<html></html>",
		"/master.m3u8": "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=1\nlive.m3u8\n",
	})
	m := NewManager(DefaultSettings(t.TempDir()), "")
	for path, want := range map[string]string{"/drm.m3u8": "DRM", "/page.html": "not an HLS"} {
		_, err := m.ProbeStream(context.Background(), srv.URL+path, nil)
		if err == nil || !strings.Contains(err.Error(), want) || !errors.Is(err, ErrStreamRefused) {
			t.Errorf("%s: %v, want a refusal %q", path, err, want)
		}
	}
	// Live HLS is offered for recording, without a size estimate.
	for _, path := range []string{"/live.m3u8", "/master.m3u8"} {
		c, err := m.ProbeStream(context.Background(), srv.URL+path, nil)
		if err != nil || !strings.Contains(c.Note, "Live stream") || c.Qualities[0].Estimate != 0 {
			t.Errorf("%s: %+v %v, want a live note", path, c, err)
		}
	}
	// A server error may be temporary: not a refusal.
	_, err := m.ProbeStream(context.Background(), srv.URL+"/missing.m3u8", nil)
	if err == nil || errors.Is(err, ErrStreamRefused) {
		t.Errorf("404: %v, want a plain error", err)
	}
}

func TestCodecNames(t *testing.T) {
	for in, want := range map[string]string{
		"avc1.640028,mp4a.40.2": "H.264, AAC",
		"hvc1.1.6.L93.B0":       "H.265",
		"av01.0.08M.08, opus":   "AV1, Opus",
		"vp09.00.10.08":         "VP9",
		"mp4a.40.2,mp4a.40.5":   "AAC",
		"wvtt":                  "",
	} {
		if got := codecNames(in); got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
}

func TestProbeStreamHLSDetails(t *testing.T) {
	master := `#EXTM3U
#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="a",NAME="Main",DEFAULT=YES
#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="a",NAME="Commentary",LANGUAGE="en",URI="c.m3u8"
#EXT-X-STREAM-INF:BANDWIDTH=3000000,AVERAGE-BANDWIDTH=2000000,RESOLUTION=1280x720,AUDIO="a"
x.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=3010000,RESOLUTION=1280x720,AUDIO="a"
y.m3u8
`
	tenSeconds := "#EXTM3U\n#EXT-X-TARGETDURATION:10\n#EXTINF:10,\na.ts\n#EXT-X-ENDLIST\n"
	srv := probeServer(t, map[string]string{"/m.m3u8": master, "/y.m3u8": tenSeconds})
	m := NewManager(DefaultSettings(t.TempDir()), "")
	c, err := m.ProbeStream(context.Background(), srv.URL+"/m.m3u8", nil)
	if err != nil {
		t.Fatal(err)
	}
	// Equal labels are numbered so the window can tell them apart.
	if c.Qualities[0].Label != "720p · 3.0 Mbps" || c.Qualities[1].Label != "720p · 3.0 Mbps (2)" {
		t.Fatalf("labels %q %q", c.Qualities[0].Label, c.Qualities[1].Label)
	}
	// The average, not the peak, makes the estimate when it is given.
	if c.Qualities[1].Estimate != 2000000/8*10 {
		t.Fatalf("estimate %d", c.Qualities[1].Estimate)
	}
	// The default audio is muxed into the video: it stays the default.
	au := c.Qualities[0].Audios
	if len(au) != 2 || au[0].Label != "In the video" || au[1].Label != "Commentary (en)" {
		t.Fatalf("audios %+v", au)
	}
	if job, _ := c.Job(srv.URL+"/m.m3u8", 0, 0); job.AudioKey != "" || job.Variant == "" {
		t.Fatalf("muxed audio job %+v", job)
	}
}

func TestProbeStreamDASHLockedAudio(t *testing.T) {
	mpd := `<?xml version="1.0"?>
<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" type="static" mediaPresentationDuration="PT4S">
  <Period>
    <AdaptationSet mimeType="video/mp4" frameRate="50">
      <SegmentTemplate media="v-$Number$.m4s" timescale="1" duration="2"/>
      <Representation id="v" bandwidth="1000" height="1080"/>
    </AdaptationSet>
    <AdaptationSet mimeType="audio/mp4">
      <ContentProtection schemeIdUri="urn:uuid:edef8ba9-79d6-4ace-a3c8-27dcd51d21ed"/>
      <SegmentTemplate media="a-$Number$.m4s" timescale="1" duration="2"/>
      <Representation id="a" bandwidth="128"/>
    </AdaptationSet>
  </Period>
</MPD>`
	srv := probeServer(t, map[string]string{"/m.mpd": mpd})
	m := NewManager(DefaultSettings(t.TempDir()), "")
	c, err := m.ProbeStream(context.Background(), srv.URL+"/m.mpd", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Qualities) != 1 || len(c.Qualities[0].Audios) != 0 || c.Note == "" {
		t.Fatalf("%+v", c)
	}
	if !strings.HasPrefix(c.Qualities[0].Label, "1080p50") {
		t.Fatalf("label %q", c.Qualities[0].Label)
	}
}
