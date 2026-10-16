package engine

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"odm/internal/media"
)

func TestRemuxTSToMP4(t *testing.T) {
	fakeFFmpeg(t, "copy")
	dir := t.TempDir()
	d := New("r", "http://h/live.m3u8", dir, 1, nil, 0, nil)
	d.FileName = "show.ts"
	if err := os.WriteFile(filepath.Join(dir, "show.ts.odmpart"), []byte("ts data"), 0o644); err != nil {
		t.Fatal(err)
	}
	d.remuxTS(context.Background())
	if d.FileName != "show.mp4" {
		t.Fatalf("name %q", d.FileName)
	}
	data, err := os.ReadFile(filepath.Join(dir, "show.mp4.odmpart"))
	if err != nil || string(data) != "remuxed:ts data" {
		t.Fatalf("MP4: %q %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "show.ts.odmpart")); !os.IsNotExist(err) {
		t.Fatal("the TS file was kept next to the MP4")
	}

	// Anything but TS is left as it is.
	d.FileName = "clip.mp4"
	d.remuxTS(context.Background())
	if d.FileName != "clip.mp4" {
		t.Fatalf("an MP4 was remuxed: %q", d.FileName)
	}
}

// When ffmpeg cannot remux (MP4, then Matroska), the TS recording stays.
func TestRemuxTSKeepsTheRecording(t *testing.T) {
	fakeFFmpeg(t, "fail")
	dir := t.TempDir()
	d := New("r", "http://h/live.m3u8", dir, 1, nil, 0, nil)
	d.FileName = "show.ts"
	os.WriteFile(filepath.Join(dir, "show.ts.odmpart"), []byte("ts data"), 0o644)
	d.remuxTS(context.Background())
	if d.FileName != "show.ts" {
		t.Fatalf("name %q", d.FileName)
	}
	left, _ := filepath.Glob(filepath.Join(dir, "*"))
	if len(left) != 1 || filepath.Base(left[0]) != "show.ts.odmpart" {
		t.Fatalf("files %v", left)
	}
}

func TestProbeSiteLive(t *testing.T) {
	t.Setenv("FAKE_LIVE_MANIFEST", "https://cdn.example/master.m3u8")
	m := fakeYtDlp(t, "live")
	c, err := m.ProbeSite(context.Background(), "https://www.youtube.com/watch?v=live")
	if err != nil {
		t.Fatal(err)
	}
	if !c.Live || c.Manifest != "https://cdn.example/master.m3u8" || c.Headers["X-Live"] != "1" || c.Title != "Launch" || len(c.Qualities) != 0 {
		t.Fatalf("%+v", c)
	}
}

// A recording found by yt-dlp outlives its playlist's signed address: when
// the address stops working, yt-dlp is asked for the broadcast's current
// one and the recording goes on from there.
func TestLiveAsksYtDlpForAFreshPlaylist(t *testing.T) {
	fastLive(t)
	var mu sync.Mutex
	next, oldLoads, total := 0, 0, 8
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		dir, file := filepath.Split(r.URL.Path)
		if file != "live.m3u8" {
			fmt.Fprintf(w, "[%s]", strings.TrimSuffix(file, ".ts"))
			return
		}
		if dir == "/old/" {
			oldLoads++
			if oldLoads > 3 { // the signature expired
				http.Error(w, "expired", http.StatusForbidden)
				return
			}
		}
		if r.Header.Get("X-Live") == "" && dir == "/new/" {
			http.Error(w, "headers from yt-dlp missing", http.StatusBadRequest)
			return
		}
		if next < total {
			next++
		}
		lo := max(0, next-3)
		fmt.Fprintf(w, "#EXTM3U\n#EXT-X-TARGETDURATION:1\n#EXT-X-MEDIA-SEQUENCE:%d\n", lo)
		for i := lo; i < next; i++ {
			fmt.Fprintf(w, "#EXTINF:1,\ns%d.ts\n", i)
		}
		if next >= total {
			fmt.Fprint(w, "#EXT-X-ENDLIST\n")
		}
	}))
	t.Cleanup(srv.Close)

	t.Setenv("FAKE_YTDLP", "live")
	t.Setenv("FAKE_LIVE_MANIFEST", srv.URL+"/new/live.m3u8")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	setYtDlpPath(exe)
	t.Cleanup(func() { setYtDlpPath("") })

	d := newStreamDownload(t, srv.Client(), 1, StreamJob{Kind: media.KindHLS, Manifest: srv.URL + "/old/live.m3u8", Page: "https://www.youtube.com/watch?v=live"})
	d.Start(context.Background())
	got := string(readResult(t, d))
	if !strings.HasPrefix(got, "[s0][s1][s2]") || !strings.HasSuffix(got, "[s7]") {
		t.Fatalf("recording %q", got)
	}
	st := d.State()
	if st.Stream.Manifest != srv.URL+"/new/live.m3u8" || st.Stream.Page == "" || st.Headers["X-Live"] != "1" {
		t.Fatalf("state after the new playlist: %+v, headers %v", st.Stream, st.Headers)
	}
}
