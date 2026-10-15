package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"odm/internal/media"
)

// fastLive shortens the reload loop's waits for tests.
func fastLive(t *testing.T) {
	t.Helper()
	minW, maxW, stall, targets := liveMinWait, liveMaxWait, liveMinStall, liveStallTargets
	liveMinWait, liveMaxWait, liveMinStall, liveStallTargets = time.Millisecond, 20*time.Millisecond, 400*time.Millisecond, 0
	t.Cleanup(func() { liveMinWait, liveMaxWait, liveMinStall, liveStallTargets = minW, maxW, stall, targets })
}

// liveServer serves a sliding three-segment live playlist that gains one
// segment per reload, starting at sequence first. With total > 0 it ends
// (EXT-X-ENDLIST) once that many segments are out. Segment s<n>.ts serves
// "[s<n>]" unless fail says otherwise.
type liveServer struct {
	mu      sync.Mutex
	first   int
	next    int // segments published so far, counted from first
	total   int
	fail    map[string]int // path -> status
	reloads int
}

func (l *liveServer) start(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		l.mu.Lock()
		defer l.mu.Unlock()
		if code, ok := l.fail[r.URL.Path]; ok {
			http.Error(w, "no", code)
			return
		}
		if r.URL.Path != "/live.m3u8" {
			fmt.Fprintf(w, "[%s]", strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/"), ".ts"))
			return
		}
		l.reloads++
		if l.total == 0 || l.next < l.total {
			l.next++
		}
		lo := max(0, l.next-3)
		fmt.Fprintf(w, "#EXTM3U\n#EXT-X-TARGETDURATION:1\n#EXT-X-MEDIA-SEQUENCE:%d\n", l.first+lo)
		for i := lo; i < l.next; i++ {
			fmt.Fprintf(w, "#EXTINF:1,\ns%d.ts\n", l.first+i)
		}
		if l.total > 0 && l.next >= l.total {
			fmt.Fprint(w, "#EXT-X-ENDLIST\n")
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestLiveRecordsUntilEnd(t *testing.T) {
	fastLive(t)
	srv := (&liveServer{total: 6}).start(t)
	d := newStreamDownload(t, srv.Client(), 2, StreamJob{Kind: media.KindHLS, Manifest: srv.URL + "/live.m3u8"})
	d.Start(context.Background())
	if got := string(readResult(t, d)); got != "[s0][s1][s2][s3][s4][s5]" {
		t.Fatalf("recording %q", got)
	}
	if info := d.Info(); !info.Live || info.FileName != "clip.ts" {
		t.Fatalf("info %+v", info)
	}
}

func TestLiveStopFinishes(t *testing.T) {
	fastLive(t)
	srv := (&liveServer{}).start(t) // never ends
	d := newStreamDownload(t, srv.Client(), 1, StreamJob{Kind: media.KindHLS, Manifest: srv.URL + "/live.m3u8"})
	done := make(chan struct{})
	go func() { d.Start(context.Background()); close(done) }()
	deadline := time.Now().Add(5 * time.Second)
	for d.Info().SegmentsDone < 4 {
		if time.Now().After(deadline) {
			t.Fatal("recording did not progress")
		}
		time.Sleep(5 * time.Millisecond)
	}
	d.Pause()
	<-done
	got := string(readResult(t, d))
	if !strings.HasPrefix(got, "[s0][s1][s2][s3]") {
		t.Fatalf("recording %q", got)
	}
}

func TestLiveSkipsLostSegment(t *testing.T) {
	fastLive(t)
	srv := (&liveServer{total: 5, fail: map[string]int{"/s2.ts": http.StatusNotFound}}).start(t)
	d := newStreamDownload(t, srv.Client(), 1, StreamJob{Kind: media.KindHLS, Manifest: srv.URL + "/live.m3u8"})
	d.Start(context.Background())
	if got := string(readResult(t, d)); got != "[s0][s1][s3][s4]" {
		t.Fatalf("recording %q", got)
	}
}

func TestLiveStallEnds(t *testing.T) {
	fastLive(t)
	// Without EXT-X-ENDLIST the playlist just stops growing.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/live.m3u8" {
			fmt.Fprint(w, "#EXTM3U\n#EXT-X-TARGETDURATION:1\n#EXTINF:1,\ns0.ts\n#EXTINF:1,\ns1.ts\n")
			return
		}
		fmt.Fprintf(w, "[%s]", strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/"), ".ts"))
	}))
	defer srv.Close()
	d := newStreamDownload(t, srv.Client(), 1, StreamJob{Kind: media.KindHLS, Manifest: srv.URL + "/live.m3u8"})
	start := time.Now()
	d.Start(context.Background())
	if got := string(readResult(t, d)); got != "[s0][s1]" {
		t.Fatalf("recording %q", got)
	}
	if time.Since(start) > 3*time.Second {
		t.Fatal("stall limit not applied")
	}
}

// A recording cut short (ODM closed before it was finished) continues on
// the next run, after a gap, keeping what it had.
func TestLiveContinues(t *testing.T) {
	fastLive(t)
	srv := (&liveServer{first: 50, total: 2}).start(t)
	d := newStreamDownload(t, srv.Client(), 1, StreamJob{Kind: media.KindHLS, Manifest: srv.URL + "/live.m3u8", Live: true, Recording: true})
	d.FileName = "clip.ts"
	work := filepath.Join(d.BaseDir, "clip.ts.odmparts")
	if err := os.MkdirAll(filepath.Join(work, "t0"), 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(work, "t0", "0.seg"), []byte("[old0]"), 0o644)
	os.WriteFile(filepath.Join(work, "t0", "1.seg"), []byte("[old1]"), 0o644)
	st, _ := json.Marshal(liveState{Tracks: []*liveTrack{{Ext: "ts", Video: true, SegInit: []int{-1, -1}, LastSeq: 11, Started: true}}})
	os.WriteFile(filepath.Join(work, liveStateFile), st, 0o644)
	os.WriteFile(filepath.Join(d.BaseDir, "clip.ts.odmpart"), nil, 0o644)
	d.Dir = d.BaseDir

	d.Start(context.Background())
	if got := string(readResult(t, d)); got != "[old0][old1][s50][s51]" {
		t.Fatalf("recording %q", got)
	}
}

// A public live playlist cannot reach into the local network either.
func TestLiveLocalNetworkRefused(t *testing.T) {
	fastLive(t)
	fakeDNS(t, map[string][]string{"cdn.example": {"93.184.216.34"}})
	d := newStreamDownload(t, &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return (&fakeNet{}).RoundTrip(r)
	})}, 1, StreamJob{Kind: media.KindHLS, Manifest: "https://cdn.example/live.m3u8", Live: true})
	d.Stream.Video = "http://127.0.0.1:1/live.m3u8"
	d.Start(context.Background())
	if info := d.Info(); info.Status != StatusError || !strings.Contains(info.Error, "local network") {
		t.Fatalf("%s %q", info.Status, info.Error)
	}
}

// A new recording must not take over another download's live.json that
// happens to sit under the name it starts from.
func TestLiveFreshIgnoresForeignState(t *testing.T) {
	fastLive(t)
	srv := (&liveServer{total: 2}).start(t)
	d := newStreamDownload(t, srv.Client(), 1, StreamJob{Kind: media.KindHLS, Manifest: srv.URL + "/live.m3u8"})
	other := filepath.Join(d.BaseDir, "clip.odmparts")
	os.MkdirAll(filepath.Join(other, "t0"), 0o755)
	os.WriteFile(filepath.Join(other, "t0", "0.seg"), []byte("[foreign]"), 0o644)
	st, _ := json.Marshal(liveState{Tracks: []*liveTrack{{Ext: "ts", Video: true, SegInit: []int{-1}, LastSeq: 99, Started: true}}})
	os.WriteFile(filepath.Join(other, liveStateFile), st, 0o644)
	d.Start(context.Background())
	if got := string(readResult(t, d)); got != "[s0][s1]" {
		t.Fatalf("recording %q", got)
	}
	if _, err := os.Stat(filepath.Join(other, liveStateFile)); err != nil {
		t.Fatalf("the other download's state was touched: %v", err)
	}
}

// Separate audio: a round where one playlist fails takes nothing, so no
// track advances past segments it never fetched.
func TestLiveTracksAdvanceTogether(t *testing.T) {
	fastLive(t)
	var mu sync.Mutex
	reloads := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch r.URL.Path {
		case "/v.m3u8", "/a.m3u8":
			kind := r.URL.Path[1:2]
			if kind == "v" {
				reloads++
			}
			n := min(reloads, 4)
			if kind == "a" && reloads == 2 {
				http.Error(w, "busy", http.StatusServiceUnavailable)
				return
			}
			fmt.Fprint(w, "#EXTM3U\n#EXT-X-TARGETDURATION:1\n")
			for i := 0; i < n; i++ {
				fmt.Fprintf(w, "#EXTINF:1,\n%s%d.ts\n", kind, i)
			}
			if n == 4 {
				fmt.Fprint(w, "#EXT-X-ENDLIST\n")
			}
		default:
			fmt.Fprintf(w, "[%s]", strings.TrimSuffix(r.URL.Path[1:], ".ts"))
		}
	}))
	defer srv.Close()
	d := newStreamDownload(t, srv.Client(), 1, StreamJob{Kind: media.KindHLS, Manifest: srv.URL + "/v.m3u8",
		Video: srv.URL + "/v.m3u8", Audio: srv.URL + "/a.m3u8"})
	setFFmpegPath(filepath.Join(t.TempDir(), "no-ffmpeg-here"))
	defer setFFmpegPath("")
	d.Start(context.Background())
	// Joining needs ffmpeg, absent here: the failure must be reported, not
	// shown as a pause, and every segment of both tracks must be on disk.
	info := d.Info()
	if info.Status != StatusError || !strings.Contains(info.Error, "ffmpeg") {
		t.Fatalf("%s %q", info.Status, info.Error)
	}
	work := filepath.Join(d.Dir, d.FileName+".odmparts")
	for ti, kind := range []string{"v", "a"} {
		for i := 0; i < 4; i++ {
			b, err := os.ReadFile(filepath.Join(work, fmt.Sprintf("t%d", ti), fmt.Sprintf("%d.seg", i)))
			if err != nil || string(b) != fmt.Sprintf("[%s%d]", kind, i) {
				t.Errorf("track %s piece %d: %q %v", kind, i, b, err)
			}
		}
	}
}

// A lagging CDN edge may return an older copy: nothing is taken twice.
func TestLiveStalePlaylistIgnored(t *testing.T) {
	fastLive(t)
	var mu sync.Mutex
	call := 0
	// Sequence windows served per reload; the third is a stale copy.
	windows := [][2]int{{0, 3}, {1, 4}, {0, 3}, {2, 5}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/live.m3u8" {
			fmt.Fprintf(w, "[%s]", strings.TrimSuffix(r.URL.Path[1:], ".ts"))
			return
		}
		mu.Lock()
		win := windows[min(call, len(windows)-1)]
		last := call >= len(windows)-1
		call++
		mu.Unlock()
		fmt.Fprintf(w, "#EXTM3U\n#EXT-X-TARGETDURATION:1\n#EXT-X-MEDIA-SEQUENCE:%d\n", win[0])
		for i := win[0]; i < win[1]; i++ {
			fmt.Fprintf(w, "#EXTINF:1,\ns%d.ts\n", i)
		}
		if last {
			fmt.Fprint(w, "#EXT-X-ENDLIST\n")
		}
	}))
	defer srv.Close()
	d := newStreamDownload(t, srv.Client(), 1, StreamJob{Kind: media.KindHLS, Manifest: srv.URL + "/live.m3u8"})
	d.Start(context.Background())
	if got := string(readResult(t, d)); got != "[s0][s1][s2][s3][s4]" {
		t.Fatalf("recording %q", got)
	}
}

// An init section that fails is fetched again on the next round instead
// of blocking assembly; signed init URLs that change per reload count as
// one init.
func TestLiveInitRetriedAndSigned(t *testing.T) {
	fastLive(t)
	var mu sync.Mutex
	reloads, initHits := 0, 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch r.URL.Path {
		case "/live.m3u8":
			reloads++
			n := min(reloads, 3)
			fmt.Fprintf(w, "#EXTM3U\n#EXT-X-TARGETDURATION:1\n#EXT-X-MAP:URI=\"init.mp4?sig=%d\"\n", reloads)
			for i := 0; i < n; i++ {
				fmt.Fprintf(w, "#EXTINF:1,\ns%d.m4s\n", i)
			}
			if n == 3 {
				fmt.Fprint(w, "#EXT-X-ENDLIST\n")
			}
		case "/init.mp4":
			initHits++
			if initHits == 1 {
				http.Error(w, "busy", http.StatusServiceUnavailable)
				return
			}
			fmt.Fprint(w, "[init]")
		default:
			fmt.Fprintf(w, "[%s]", strings.TrimSuffix(r.URL.Path[1:], ".m4s"))
		}
	}))
	defer srv.Close()
	d := newStreamDownload(t, srv.Client(), 1, StreamJob{Kind: media.KindHLS, Manifest: srv.URL + "/live.m3u8"})
	d.Start(context.Background())
	if got := string(readResult(t, d)); got != "[init][s0][s1][s2]" {
		t.Fatalf("recording %q", got)
	}
}

// Removing a recording abandons it: no file is assembled.
func TestLiveRemoveAbandons(t *testing.T) {
	fastLive(t)
	srv := (&liveServer{}).start(t)
	m := NewManager(DefaultSettings(t.TempDir()), "")
	info := m.Create(AddOptions{URL: srv.URL + "/live.m3u8", FileName: "clip", Stream: &StreamJob{Kind: media.KindHLS, Manifest: srv.URL + "/live.m3u8"}})
	d := m.get(info.ID)
	d.client = srv.Client()
	done := make(chan struct{})
	go func() { d.Start(context.Background()); close(done) }()
	deadline := time.Now().Add(5 * time.Second)
	for d.Info().SegmentsDone < 2 {
		if time.Now().After(deadline) {
			t.Fatal("recording did not progress")
		}
		time.Sleep(5 * time.Millisecond)
	}
	m.RemoveWithFile(info.ID, true)
	<-done
	entries, _ := os.ReadDir(d.Dir)
	if len(entries) != 0 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("left behind: %v", names)
	}
}

// twoTrackServer serves a growing video and audio playlist pair that ends
// after n segments.
func twoTrackServer(t *testing.T, n int) *httptest.Server {
	var mu sync.Mutex
	reloads := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch r.URL.Path {
		case "/v.m3u8", "/a.m3u8":
			kind := r.URL.Path[1:2]
			if kind == "v" {
				reloads++
			}
			k := min(reloads, n)
			fmt.Fprint(w, "#EXTM3U\n#EXT-X-TARGETDURATION:1\n")
			for i := 0; i < k; i++ {
				fmt.Fprintf(w, "#EXTINF:1,\n%s%d.ts\n", kind, i)
			}
			if k == n {
				fmt.Fprint(w, "#EXT-X-ENDLIST\n")
			}
		default:
			fmt.Fprintf(w, "[%s]", strings.TrimSuffix(r.URL.Path[1:], ".ts"))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// Stop, then Remove while the stopped recording is still being joined:
// the join is killed and nothing comes back afterwards.
func TestLiveRemoveDuringAssembly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a shell script as ffmpeg")
	}
	fastLive(t)
	bin := filepath.Join(t.TempDir(), "ffmpeg")
	os.WriteFile(bin, []byte("#!/bin/sh\nsleep 30\n"), 0o755)
	setFFmpegPath(bin)
	defer setFFmpegPath("")

	srv := twoTrackServer(t, 1000) // does not end on its own
	m := NewManager(DefaultSettings(t.TempDir()), "")
	info := m.Create(AddOptions{URL: srv.URL + "/v.m3u8", FileName: "clip", Stream: &StreamJob{Kind: media.KindHLS,
		Manifest: srv.URL + "/v.m3u8", Video: srv.URL + "/v.m3u8", Audio: srv.URL + "/a.m3u8"}})
	d := m.get(info.ID)
	d.client = srv.Client()
	done := make(chan struct{})
	go func() { d.Start(context.Background()); close(done) }()
	deadline := time.Now().Add(5 * time.Second)
	for d.Info().SegmentsDone < 4 {
		if time.Now().After(deadline) {
			t.Fatal("recording did not progress")
		}
		time.Sleep(5 * time.Millisecond)
	}
	d.Pause() // Stop: starts joining with the sleeping "ffmpeg"
	time.Sleep(200 * time.Millisecond)
	start := time.Now()
	m.RemoveWithFile(info.ID, true)
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("assembly was not cancelled by Remove")
	}
	if time.Since(start) > 5*time.Second {
		t.Fatalf("Remove took %v", time.Since(start))
	}
	time.Sleep(100 * time.Millisecond)
	if entries, _ := os.ReadDir(d.Dir); len(entries) != 0 {
		t.Fatalf("left behind: %d entries", len(entries))
	}
}

// An init that keeps failing does not hold off the stall limit, and its
// segments are dropped from the file.
func TestLiveDeadInitStalls(t *testing.T) {
	fastLive(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/live.m3u8":
			// Stops growing, never ends; its init is always refused.
			fmt.Fprint(w, "#EXTM3U\n#EXT-X-TARGETDURATION:1\n#EXT-X-MAP:URI=\"init.mp4\"\n#EXTINF:1,\ns0.m4s\n")
		case "/init.mp4":
			http.Error(w, "busy", http.StatusServiceUnavailable)
		default:
			fmt.Fprint(w, "[seg]")
		}
	}))
	defer srv.Close()
	d := newStreamDownload(t, srv.Client(), 1, StreamJob{Kind: media.KindHLS, Manifest: srv.URL + "/live.m3u8"})
	finished := make(chan struct{})
	go func() { d.Start(context.Background()); close(finished) }()
	select {
	case <-finished:
	case <-time.After(15 * time.Second):
		d.Pause()
		<-finished
		t.Fatal("the stall limit never fired")
	}
	if got := string(readResult(t, d)); got != "" {
		t.Fatalf("recording %q, want the init-less segment dropped", got)
	}
}

// Recording set but its list gone: the folder starts over instead of
// mixing old pieces in.
func TestLiveRecordingWithoutState(t *testing.T) {
	fastLive(t)
	srv := (&liveServer{first: 7, total: 2}).start(t)
	d := newStreamDownload(t, srv.Client(), 1, StreamJob{Kind: media.KindHLS, Manifest: srv.URL + "/live.m3u8", Live: true, Recording: true})
	d.FileName, d.Dir = "clip.ts", d.BaseDir
	work := filepath.Join(d.BaseDir, "clip.ts.odmparts", "t0")
	os.MkdirAll(work, 0o755)
	os.WriteFile(filepath.Join(work, "1.seg"), []byte("[stale]"), 0o644)
	os.WriteFile(filepath.Join(d.BaseDir, "clip.ts.odmpart"), nil, 0o644)
	d.Start(context.Background())
	if got := string(readResult(t, d)); got != "[s7][s8]" {
		t.Fatalf("recording %q", got)
	}
}

// A damaged live.json starts the folder over; a list that cannot be read
// for another reason keeps the pieces and fails, so Resume can retry.
func TestLiveStateReadErrors(t *testing.T) {
	fastLive(t)
	srv := (&liveServer{first: 3, total: 1}).start(t)
	d := newStreamDownload(t, srv.Client(), 1, StreamJob{Kind: media.KindHLS, Manifest: srv.URL + "/live.m3u8", Live: true, Recording: true})
	d.FileName, d.Dir = "clip.ts", d.BaseDir
	work := filepath.Join(d.BaseDir, "clip.ts.odmparts")
	os.MkdirAll(filepath.Join(work, "t0"), 0o755)
	os.WriteFile(filepath.Join(work, "t0", "0.seg"), []byte("[kept]"), 0o644)
	os.WriteFile(filepath.Join(d.BaseDir, "clip.ts.odmpart"), nil, 0o644)

	// Unreadable for another reason: a directory where the file should be.
	os.MkdirAll(filepath.Join(work, liveStateFile), 0o755)
	d.Start(context.Background())
	if info := d.Info(); info.Status != StatusError {
		t.Fatalf("unreadable list: %s", info.Status)
	}
	if _, err := os.Stat(filepath.Join(work, "t0", "0.seg")); err != nil {
		t.Fatalf("pieces lost on a read error: %v", err)
	}

	// Damaged: starts over.
	os.RemoveAll(filepath.Join(work, liveStateFile))
	os.WriteFile(filepath.Join(work, liveStateFile), []byte("{not json"), 0o644)
	d.Start(context.Background())
	if got := string(readResult(t, d)); got != "[s3]" {
		t.Fatalf("recording %q", got)
	}
}
