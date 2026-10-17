package engine

import (
	"context"
	"encoding/binary"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"odm/internal/media"
)

// flvServer streams "[c<n>]" chunks: each connection sends perConn
// chunks, then closes (0: until the client goes); after conns connections
// it answers gone. Chunks are numbered across connections. A path in
// expire is refused (403) after that many connections; one in need is
// only served with that header.
type flvServer struct {
	mu      sync.Mutex
	perConn int
	conns   int
	served  int
	chunk   int
	expire  map[string]int
	need    map[string]string
	byPath  map[string]int
}

func (s *flvServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	if s.byPath == nil {
		s.byPath = map[string]int{}
	}
	if n, ok := s.expire[r.URL.Path]; ok && s.byPath[r.URL.Path] >= n {
		s.mu.Unlock()
		http.Error(w, "expired", http.StatusForbidden)
		return
	}
	if h := s.need[r.URL.Path]; h != "" && r.Header.Get(h) == "" {
		s.mu.Unlock()
		http.Error(w, "header missing", http.StatusBadRequest)
		return
	}
	s.byPath[r.URL.Path]++
	if s.conns > 0 && s.served >= s.conns {
		s.mu.Unlock()
		http.Error(w, "over", http.StatusNotFound)
		return
	}
	s.served++
	s.mu.Unlock()
	w.Header().Set("Content-Type", "video/x-flv")
	flusher := w.(http.Flusher)
	for i := 0; s.perConn == 0 || i < s.perConn; i++ {
		s.mu.Lock()
		n := s.chunk
		s.chunk++
		s.mu.Unlock()
		if _, err := fmt.Fprintf(w, "[c%d]", n); err != nil {
			return
		}
		flusher.Flush()
		select {
		case <-r.Context().Done():
			return
		case <-time.After(5 * time.Millisecond):
		}
	}
}

func newHTTPLiveDownload(t *testing.T, client *http.Client, address, page string) *Download {
	t.Helper()
	d := newStreamDownload(t, client, 1, StreamJob{Kind: media.KindHTTPLive, Manifest: address, Live: true, Page: page})
	d.FileName = "show"
	return d
}

// Pause ends the recording: what came is joined into the MP4.
func TestHTTPLiveRecordsUntilStopped(t *testing.T) {
	fastLive(t)
	fakeFFmpeg(t, "copy")
	srv := httptest.NewServer(&flvServer{})
	t.Cleanup(srv.Close)
	d := newHTTPLiveDownload(t, srv.Client(), srv.URL+"/stage/live.flv?sign=1", "")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		d.Start(ctx)
		close(done)
	}()
	deadline := time.Now().Add(5 * time.Second)
	for d.Info().Done < 40 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if d.Info().Status != StatusDownloading || !d.Info().Live {
		t.Fatalf("while recording: %+v", d.Info())
	}
	cancel()
	<-done
	got := string(readResult(t, d))
	if d.Info().FileName != "show.mp4" || !strings.HasPrefix(got, "remuxed:[c0][c1]") || !strings.HasSuffix(got, "]") {
		t.Fatalf("%s: %q", d.Info().FileName, got)
	}
}

// A dropped connection is opened again into a new part; the parts are
// joined in order once the broadcast is over (the address answers gone
// for longer than the stall limit).
func TestHTTPLiveReconnectsAndJoins(t *testing.T) {
	fastLive(t)
	fakeFFmpeg(t, "copy")
	srv := httptest.NewServer(&flvServer{perConn: 3, conns: 2})
	t.Cleanup(srv.Close)
	d := newHTTPLiveDownload(t, srv.Client(), srv.URL+"/live.flv", "")
	d.Start(context.Background())
	if got := string(readResult(t, d)); got != "joined:remuxed:[c0][c1][c2]remuxed:[c3][c4][c5]" {
		t.Fatalf("recording %q", got)
	}
}

// Without ffmpeg (or when it cannot join them), the first part becomes the
// file as it came and the others are kept next to it.
func TestHTTPLiveKeepsThePartsWithoutFFmpeg(t *testing.T) {
	fastLive(t)
	fakeFFmpeg(t, "fail")
	srv := httptest.NewServer(&flvServer{perConn: 2, conns: 2})
	t.Cleanup(srv.Close)
	d := newHTTPLiveDownload(t, srv.Client(), srv.URL+"/live.flv", "")
	d.Start(context.Background())
	info := d.Info()
	if info.Status != StatusComplete || info.FileName != "show.flv" {
		t.Fatalf("%s %q: %s", info.Status, info.FileName, info.Error)
	}
	first, _ := os.ReadFile(filepath.Join(info.Dir, "show.flv"))
	second, _ := os.ReadFile(filepath.Join(info.Dir, "show (part 2).flv"))
	if string(first) != "[c0][c1]" || string(second) != "[c2][c3]" {
		left, _ := filepath.Glob(filepath.Join(info.Dir, "*"))
		t.Fatalf("parts %q %q, files %v", first, second, left)
	}
	left, _ := filepath.Glob(filepath.Join(info.Dir, "*"))
	if len(left) != 2 {
		t.Fatalf("files %v", left)
	}
}

// A refused (expired) address of a stream found by yt-dlp: yt-dlp is
// asked for the current one, with its headers, and the recording goes on.
func TestHTTPLiveAsksYtDlpForAFreshAddress(t *testing.T) {
	fastLive(t)
	fakeFFmpeg(t, "copy")
	// The old address works once, then its signature has expired.
	fs := &flvServer{perConn: 2, conns: 3, expire: map[string]int{"/old.flv": 1}, need: map[string]string{"/new.flv": "X-Live"}}
	srv := httptest.NewServer(fs)
	t.Cleanup(srv.Close)
	t.Setenv("FAKE_YTDLP", "live")
	t.Setenv("FAKE_LIVE_KIND", "flv")
	t.Setenv("FAKE_LIVE_MANIFEST", srv.URL+"/new.flv")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	setYtDlpPath(exe)
	t.Cleanup(func() { setYtDlpPath("") })

	d := newHTTPLiveDownload(t, srv.Client(), srv.URL+"/old.flv", "https://www.tiktok.com/@someone/live")
	d.Start(context.Background())
	if got := string(readResult(t, d)); got != "joined:remuxed:[c0][c1]remuxed:[c2][c3]remuxed:[c4][c5]" {
		t.Fatalf("recording %q", got)
	}
	st := d.State()
	if st.Stream.Manifest != srv.URL+"/new.flv" || st.Headers["X-Live"] != "1" {
		t.Fatalf("after the new address: %+v, headers %v", st.Stream, st.Headers)
	}
}

// A response with a length that came whole is a file: recorded once, not
// again and again.
func TestHTTPLiveStopsAfterAWholeFile(t *testing.T) {
	fastLive(t)
	fakeFFmpeg(t, "copy")
	var hits int
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits++
		mu.Unlock()
		w.Header().Set("Content-Length", "6")
		w.Write([]byte("[file]"))
	}))
	t.Cleanup(srv.Close)
	d := newHTTPLiveDownload(t, srv.Client(), srv.URL+"/clip.flv", "")
	d.Start(context.Background())
	if got := string(readResult(t, d)); got != "remuxed:[file]" {
		t.Fatalf("recording %q", got)
	}
	if hits != 1 {
		t.Fatalf("fetched %d times", hits)
	}
}

func TestProbeSiteLiveKinds(t *testing.T) {
	t.Setenv("FAKE_LIVE_MANIFEST", "https://pull.example/stage/s.flv")
	t.Setenv("FAKE_LIVE_KIND", "flv")
	m := fakeYtDlp(t, "live")
	c, err := m.ProbeSite(context.Background(), "https://www.tiktok.com/@someone/live")
	if err != nil {
		t.Fatal(err)
	}
	if !c.Live || c.Kind != media.KindHTTPLive || c.Manifest != "https://pull.example/stage/s.flv" || c.Headers["X-Live"] != "1" {
		t.Fatalf("flv: %+v", c)
	}
	t.Setenv("FAKE_LIVE_KIND", "dash")
	t.Setenv("FAKE_LIVE_MANIFEST", "https://fb.example/live.mpd")
	if c, err = m.ProbeSite(context.Background(), "https://www.facebook.com/x/videos/1"); err != nil || c.Kind != media.KindDASH || c.Manifest != "https://fb.example/live.mpd" {
		t.Fatalf("dash: %+v %v", c, err)
	}
}

type flvTag struct {
	kind byte
	ts   int64
	body []byte
}

// flvBytes is an FLV file of tags.
func flvBytes(tags ...flvTag) []byte {
	b := []byte{'F', 'L', 'V', 1, 5, 0, 0, 0, 9}
	prev := uint32(0)
	for _, t := range tags {
		b = binary.BigEndian.AppendUint32(b, prev)
		n := len(t.body)
		b = append(b, t.kind, byte(n>>16), byte(n>>8), byte(n), byte(t.ts>>16), byte(t.ts>>8), byte(t.ts), byte(t.ts>>24), 0, 0, 0)
		b = append(b, t.body...)
		prev = uint32(11 + n)
	}
	return binary.BigEndian.AppendUint32(b, prev)
}

// flvFrames is an FLV as TikTok's server sends one: its metadata and the
// codecs' configuration (HEVC, AAC) at 0, then a video and an audio frame
// every 40 ms from from to to, in the broadcast's milliseconds.
func flvFrames(from, to int64) []byte {
	tags := []flvTag{{18, 0, []byte("onMetaData")}, {9, 0, []byte{0x1c, 0, 0}}, {8, 0, []byte{0xaf, 0, 0x12}}}
	for ts := from; ts <= to; ts += 40 {
		tags = append(tags, flvTag{9, ts, []byte{0x2c, 1, 0, 0, 0}}, flvTag{8, ts, []byte{0xaf, 1, 0x21}})
	}
	return flvBytes(tags...)
}

func TestFLVSpan(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, data []byte) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	data := flvFrames(1500000, 1500400)
	// A cut-off tag at the end, as a part being written has.
	if first, last, ok := flvSpan(write("p.flv", data[:len(data)-6])); !ok || first != 1500000 || last != 1500400 {
		t.Fatalf("span %d..%d %v", first, last, ok)
	}
	// Timestamps past 2^24 ms (4.6 hours) use the extended byte.
	if first, last, ok := flvSpan(write("long.flv", flvFrames(20000000, 20000040))); !ok || first != 20000000 || last != 20000040 {
		t.Fatalf("long span %d..%d %v", first, last, ok)
	}
	// An enhanced FLV's sequence start is configuration too.
	if first, _, ok := flvSpan(write("enh.flv", flvBytes(flvTag{9, 0, []byte{0x90, 'h', 'v', 'c', '1'}}, flvTag{9, 700, []byte{0x91, 'h', 'v', 'c', '1'}}))); !ok || first != 700 {
		t.Fatalf("enhanced first %d %v", first, ok)
	}
	if _, _, ok := flvSpan(write("config.flv", flvBytes(flvTag{9, 0, []byte{0x1c, 0}}))); ok {
		t.Fatal("configuration alone counted as frames")
	}
	if _, _, ok := flvSpan(write("c.flv", []byte("[c0][c1]"))); ok {
		t.Fatal("not an FLV, but a span")
	}
}

func TestIsReplay(t *testing.T) {
	cases := []struct {
		first, last, recorded int64
		want                  bool
	}{
		{1000, 2000, -1, false},         // the first part
		{1000, 2000, 2000, true},        // the same seconds again
		{1500, 1990, 2000, true},        // some of them
		{1960, 3000, 2000, false},       // overlaps, then goes on
		{0, 1000, 500000, false},        // the broadcast's clock started over
		{380000, 400000, 400000, true},  // the last 20 s again
		{200000, 400000, 400000, false}, // starts long before: not a repeat
	}
	for _, c := range cases {
		if got := isReplay(c.first, c.last, c.recorded); got != c.want {
			t.Errorf("isReplay(%d, %d, %d) = %v", c.first, c.last, c.recorded, got)
		}
	}
}

func TestFreshParts(t *testing.T) {
	dir := t.TempDir()
	var parts []string
	for i, data := range [][]byte{flvFrames(1000, 2000), flvFrames(1000, 2000), flvFrames(1960, 3000), []byte("[c0]")} {
		p := filepath.Join(dir, fmt.Sprintf("p%d.flv", i+1))
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
		parts = append(parts, p)
	}
	got := freshParts(parts)
	if len(got) != 3 || got[0] != parts[0] || got[1] != parts[2] || got[2] != parts[3] {
		t.Fatalf("fresh parts %v", got)
	}
}

// replayServer answers each connection with the next of its bodies, then
// with the last one again and again, as TikTok's server sends a
// broadcast's last seconds once it is over. Responses have no length.
type replayServer struct {
	mu     sync.Mutex
	bodies [][]byte
	served int
}

func (s *replayServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	body := s.bodies[min(s.served, len(s.bodies)-1)]
	s.served++
	s.mu.Unlock()
	w.Header().Set("Content-Type", "video/x-flv")
	w.Write(body)
	w.(http.Flusher).Flush()
}

// Parts that only repeat the seconds already recorded are dropped and do
// not count as data, so the recording ends once the broadcast is over;
// a part that goes on, or a broadcast whose clock started over, is kept.
func TestHTTPLiveDropsReplayedSeconds(t *testing.T) {
	cases := []struct {
		name   string
		bodies [][]byte
		want   string
	}{
		{"over", [][]byte{flvFrames(1000, 2000)}, "remuxed:" + string(flvFrames(1000, 2000))},
		{"goes on", [][]byte{flvFrames(1000, 2000), flvFrames(1960, 3000)},
			"joined:remuxed:" + string(flvFrames(1000, 2000)) + "remuxed:" + string(flvFrames(1960, 3000))},
		{"started over", [][]byte{flvFrames(500000, 501000), flvFrames(0, 1000)},
			"joined:remuxed:" + string(flvFrames(500000, 501000)) + "remuxed:" + string(flvFrames(0, 1000))},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fastLive(t)
			fakeFFmpeg(t, "copy")
			rs := &replayServer{bodies: c.bodies}
			srv := httptest.NewServer(rs)
			t.Cleanup(srv.Close)
			d := newHTTPLiveDownload(t, srv.Client(), srv.URL+"/live.flv", "")
			done := make(chan struct{})
			go func() {
				d.Start(context.Background())
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(10 * time.Second):
				t.Fatal("the recording did not end on repeated seconds")
			}
			if got := string(readResult(t, d)); got != c.want {
				t.Fatalf("recording %d bytes, want %d", len(got), len(c.want))
			}
			rs.mu.Lock()
			served := rs.served
			rs.mu.Unlock()
			if served <= len(c.bodies) {
				t.Fatalf("served %d connections: the repeats were not asked for", served)
			}
		})
	}
}
