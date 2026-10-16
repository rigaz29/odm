package engine

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"strings"
	"sync"
	"testing"

	"odm/internal/media"
)

// dashLiveServer serves a live MPD as Facebook's look: a SegmentTimeline
// of 1 s segments named by $Time$, a window of the latest four, and
// signatures that change on every load. Each load publishes one more
// segment; at total the manifest turns static. Segment v/<t>.m4s serves
// "[v<n>]" (n = t/1000), the init "[vi]"; audio likewise under a/. The
// manifest at expire's path is refused after that many loads.
type dashLiveServer struct {
	mu     sync.Mutex
	next   int
	total  int
	audio  bool
	loads  map[string]int
	expire map[string]int
	need   string // a header the manifest at /new/ needs
}

func (s *dashLiveServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	file := path.Base(r.URL.Path)
	if file != "live.mpd" {
		track := strings.Trim(path.Dir(r.URL.Path), "/")
		track = track[strings.LastIndex(track, "/")+1:]
		if file == "init.mp4" {
			fmt.Fprintf(w, "[%si]", track)
			return
		}
		var t int
		fmt.Sscanf(file, "%d.m4s", &t)
		fmt.Fprintf(w, "[%s%d]", track, t/1000)
		return
	}
	if s.loads == nil {
		s.loads = map[string]int{}
	}
	s.loads[r.URL.Path]++
	if n, ok := s.expire[r.URL.Path]; ok && s.loads[r.URL.Path] > n {
		http.Error(w, "expired", http.StatusForbidden)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/new/") && s.need != "" && r.Header.Get(s.need) == "" {
		http.Error(w, "header missing", http.StatusBadRequest)
		return
	}
	if s.next < s.total {
		s.next++
	}
	typ := "dynamic"
	if s.next >= s.total {
		typ = "static"
	}
	lo := max(0, s.next-4)
	sig := s.loads[r.URL.Path]
	set := func(track, mime string) string {
		return fmt.Sprintf(`<AdaptationSet mimeType="%s"><SegmentTemplate timescale="1000" initialization="%s/init.mp4?sig=%d" media="%s/$Time$.m4s?sig=%d">
			<SegmentTimeline><S t="%d" d="1000" r="%d"/></SegmentTimeline></SegmentTemplate>
			<Representation id="%s1" bandwidth="1000"/></AdaptationSet>`, mime, track, sig, track, sig, lo*1000, s.next-lo-1, track)
	}
	sets := set("v", "video/mp4")
	if s.audio {
		sets += set("a", "audio/mp4")
	}
	fmt.Fprintf(w, `<MPD type="%s" minimumUpdatePeriod="PT1S" availabilityStartTime="2026-10-16T00:00:00Z"><Period id="p0" start="PT0S">%s</Period></MPD>`, typ, sets)
}

// A live manifest is recorded from the live edge (the last three segments
// listed) to its end (static), each segment once although its URL changes
// on every load. (Reading the manifest as a stream, then as a live one,
// loads it twice first, which publishes two segments.)
func TestDASHLiveRecordsToTheEnd(t *testing.T) {
	fastLive(t)
	srv := httptest.NewServer(&dashLiveServer{next: 4, total: 8})
	t.Cleanup(srv.Close)
	d := newStreamDownload(t, srv.Client(), 2, StreamJob{Kind: media.KindDASH, Manifest: srv.URL + "/live.mpd"})
	d.Start(context.Background())
	if got := string(readResult(t, d)); got != "[vi][v4][v5][v6][v7]" {
		t.Fatalf("recording %q", got)
	}
	info := d.Info()
	if !info.Live || info.FileName != "clip.mp4" {
		t.Fatalf("%+v", info)
	}
}

// Separate video and audio are recorded side by side and joined by ffmpeg.
func TestDASHLiveJoinsVideoAndAudio(t *testing.T) {
	fastLive(t)
	fakeFFmpeg(t, "copy")
	srv := httptest.NewServer(&dashLiveServer{next: 4, total: 7, audio: true})
	t.Cleanup(srv.Close)
	d := newStreamDownload(t, srv.Client(), 2, StreamJob{Kind: media.KindDASH, Manifest: srv.URL + "/live.mpd"})
	d.Start(context.Background())
	if got := string(readResult(t, d)); got != "merged:[vi][v4][v5][v6]|[ai][a4][a5][a6]" {
		t.Fatalf("recording %q", got)
	}
	if st := d.State(); st.Stream.VideoRep != "v1" || st.Stream.AudioRep != "a1" {
		t.Fatalf("tracks not pinned: %+v", st.Stream)
	}
}

// A manifest found by yt-dlp that stops working (its signature expired)
// is looked up again, and the recording goes on from the new one.
func TestDASHLiveAsksYtDlpForAFreshManifest(t *testing.T) {
	fastLive(t)
	ls := &dashLiveServer{next: 4, total: 9, expire: map[string]int{"/old/live.mpd": 3}, need: "X-Live"}
	srv := httptest.NewServer(ls)
	t.Cleanup(srv.Close)
	t.Setenv("FAKE_YTDLP", "live")
	t.Setenv("FAKE_LIVE_KIND", "dash")
	t.Setenv("FAKE_LIVE_MANIFEST", srv.URL+"/new/live.mpd")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	setYtDlpPath(exe)
	t.Cleanup(func() { setYtDlpPath("") })

	d := newStreamDownload(t, srv.Client(), 1, StreamJob{Kind: media.KindDASH, Manifest: srv.URL + "/old/live.mpd",
		Page: "https://www.facebook.com/someone/videos/1"})
	d.Start(context.Background())
	if got := string(readResult(t, d)); got != "[vi][v4][v5][v6][v7][v8]" {
		t.Fatalf("recording %q", got)
	}
	if st := d.State(); st.Stream.Manifest != srv.URL+"/new/live.mpd" || st.Headers["X-Live"] != "1" {
		t.Fatalf("after the new manifest: %+v, headers %v", st.Stream, st.Headers)
	}
}

// The window offers a live manifest's qualities, marked live.
func TestProbeDASHLive(t *testing.T) {
	srv := httptest.NewServer(&dashLiveServer{next: 4, total: 100, audio: true})
	t.Cleanup(srv.Close)
	m := NewManager(Settings{Dir: t.TempDir()}, "")
	c, err := m.ProbeStream(context.Background(), srv.URL+"/live.mpd", nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.Kind != media.KindDASH || !c.Live || len(c.Qualities) != 1 || len(c.Qualities[0].Audios) != 1 || !strings.HasPrefix(c.Note, liveNote) {
		t.Fatalf("%+v", c)
	}
}
