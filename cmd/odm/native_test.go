package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"odm/internal/native"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// relayed runs relayDownload against a fake ODM and returns what it posted.
func relayed(t *testing.T, m native.Message) (map[string]any, native.Reply) {
	t.Helper()
	var got map[string]any
	client := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		return &http.Response{StatusCode: http.StatusCreated, Body: io.NopCloser(strings.NewReader(`{"id":"x"}`))}, nil
	})}
	reply := relayDownload(client, m)
	return got, reply
}

func TestRelayStream(t *testing.T) {
	got, reply := relayed(t, native.Message{
		Type: "download", Kind: "stream", URL: "https://cdn.example/v/master.m3u8?sig=1",
		Mime: "application/vnd.apple.mpegurl", Title: "Episode 3 | Site", Cookies: "a=1",
	})
	if !reply.OK || got["stream"] != "hls" || got["file_name"] != "Episode_3" {
		t.Fatalf("reply %+v, posted %+v", reply, got)
	}
	if h, _ := got["headers"].(map[string]any); h["Cookie"] != "a=1" {
		t.Fatalf("headers %+v", got["headers"])
	}

	// The extension calls a .mpd with an unhelpful type a stream too.
	got, _ = relayed(t, native.Message{Kind: "stream", URL: "https://cdn.example/m.mpd", Mime: "text/plain"})
	if got["stream"] != "dash" {
		t.Fatalf("dash: %+v", got)
	}
	got, _ = relayed(t, native.Message{Kind: "stream", URL: "https://cdn.example/p.m3u8", Mime: "video/mp2t"})
	if got["stream"] != "hls" {
		t.Fatalf("mislabelled hls: %+v", got)
	}
}

func TestRelayStreamUnsupported(t *testing.T) {
	_, reply := relayed(t, native.Message{Kind: "stream", URL: "https://cdn.example/x.ism/Manifest"})
	if reply.OK || reply.Error == "" {
		t.Fatalf("smooth stream accepted: %+v", reply)
	}
}

func TestRelayFileHasNoStream(t *testing.T) {
	got, _ := relayed(t, native.Message{URL: "https://cdn.example/a.zip"})
	if _, ok := got["stream"]; ok {
		t.Fatalf("file relayed as stream: %+v", got)
	}
}

func TestRelayStartsODM(t *testing.T) {
	running := false
	relay := func() native.Reply {
		if !running {
			return native.Reply{Error: errNotRunning}
		}
		return native.Reply{OK: true, Message: "queued"}
	}
	starts := 0
	start := func() error { starts++; running = true; return nil }
	if r := relayStarting(relay, start); !r.OK || starts != 1 {
		t.Fatalf("ODM closed: reply %+v after %d starts", r, starts)
	}
	if r := relayStarting(relay, start); !r.OK || starts != 1 {
		t.Fatalf("ODM running: reply %+v after %d starts", r, starts)
	}

	running = false
	if r := relayStarting(relay, nil); r.OK || r.Error != errNotRunning {
		t.Fatalf("no way to start: %+v", r)
	}
	failed := func() error { return io.ErrUnexpectedEOF }
	if r := relayStarting(relay, failed); r.OK || !strings.Contains(r.Error, "could not be started") {
		t.Fatalf("start failed: %+v", r)
	}
	// Other errors are ODM's answer, not a reason to start another one.
	busy := func() native.Reply { return native.Reply{Error: "ODM returned 500"} }
	if r := relayStarting(busy, start); r.Error != "ODM returned 500" || starts != 1 {
		t.Fatalf("an error from ODM started it: %+v, %d starts", r, starts)
	}
}

func TestRelayPingAsksODM(t *testing.T) {
	down := &http.Client{Transport: roundTrip(func(*http.Request) (*http.Response, error) {
		return nil, io.ErrClosedPipe
	})}
	if r := relayPing(down); r.OK || r.Error != errNotRunning {
		t.Fatalf("ping with ODM closed: %+v", r)
	}
	up := &http.Client{Transport: roundTrip(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
	})}
	if r := relayPing(up); !r.OK {
		t.Fatalf("ping with ODM running: %+v", r)
	}
}

func TestRelayStreams(t *testing.T) {
	var sent map[string]any
	reply := func(code int, body string) *http.Client {
		return &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path != "/api/probe/stream" {
				t.Fatalf("asked %s", r.URL.Path)
			}
			sent = nil
			_ = json.NewDecoder(r.Body).Decode(&sent)
			return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(body))}, nil
		})}
	}
	m := native.Message{Type: "streams", URL: "https://cdn.example/master.m3u8", Cookies: "s=1", Referrer: "https://site.example/watch"}

	r := relayStreams(reply(http.StatusOK, `{"kind":"hls","qualities":[{"label":"720p","estimate":5}]}`), m)
	if !r.OK || r.Stream == nil || r.Stream.Kind != "hls" || len(r.Stream.Qualities) != 1 || r.Stream.Qualities[0].Estimate != 5 {
		t.Fatalf("reply %+v", r)
	}
	if h, _ := sent["headers"].(map[string]any); h["Cookie"] != "s=1" || h["Referer"] != "https://site.example/watch" {
		t.Fatalf("headers sent %v", sent["headers"])
	}

	r = relayStreams(reply(http.StatusUnprocessableEntity, `{"error":"this stream is DRM-protected","refused":true}`), m)
	if r.OK || !r.Refused || r.Error != "this stream is DRM-protected" {
		t.Fatalf("refused: %+v", r)
	}
	r = relayStreams(reply(http.StatusBadGateway, `{"error":"timeout"}`), m)
	if r.OK || r.Refused || r.Error != "timeout" {
		t.Fatalf("failed: %+v", r)
	}
}

func TestRelayStreamQuality(t *testing.T) {
	got, _ := relayed(t, native.Message{Kind: "stream", URL: "https://cdn.example/p.m3u8", Quality: "720p · 2.0 Mbps"})
	if got["quality"] != "720p · 2.0 Mbps" {
		t.Fatalf("posted %v", got)
	}
	got, _ = relayed(t, native.Message{URL: "https://cdn.example/a.zip", Quality: "720p"})
	if _, ok := got["quality"]; ok {
		t.Fatalf("a file carried a quality: %v", got)
	}
}

func TestRelaySite(t *testing.T) {
	answer := func(code int, body string) *http.Client {
		return &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path != "/api/probe/site" {
				t.Fatalf("asked %s", r.URL.Path)
			}
			return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(body))}, nil
		})}
	}
	m := native.Message{Type: "site-formats", URL: "https://www.youtube.com/watch?v=abc"}
	r := relaySite(answer(http.StatusOK, `{"title":"A talk","qualities":[{"label":"720p","format":"136+140","ext":"mp4","estimate":9}]}`), m)
	if !r.OK || r.Site == nil || r.Site.Title != "A talk" || r.Site.Qualities[0].Format != "136+140" {
		t.Fatalf("reply %+v", r)
	}
	r = relaySite(answer(http.StatusServiceUnavailable, `{"error":"yt-dlp not found","missing":true}`), m)
	if r.OK || !r.Missing || r.Error != "yt-dlp not found" {
		t.Fatalf("missing: %+v", r)
	}
}

// A site's page goes to ODM as one, without the browser's cookies.
func TestRelaySiteDownload(t *testing.T) {
	got, reply := relayed(t, native.Message{
		Kind: "site", URL: "https://www.youtube.com/watch?v=abc", Title: "A talk | Channel",
		Format: "136+140", Ext: "mp4", Estimate: 1234, Cookies: "SID=secret", UserAgent: "Mozilla",
	})
	if !reply.OK || got["site"] != true || got["format"] != "136+140" || got["estimate"] != float64(1234) {
		t.Fatalf("posted %v", got)
	}
	if got["file_name"] != "A_talk.mp4" {
		t.Fatalf("file name %v", got["file_name"])
	}
	if _, ok := got["headers"]; ok {
		t.Fatalf("headers went with a site page: %v", got["headers"])
	}
}
