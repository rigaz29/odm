package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"odm/internal/ipc"
	"odm/internal/media"
	"odm/internal/native"
)

// errNotRunning is the reply when no ODM answers on the control socket.
const errNotRunning = "ODM is not running"

// startWait bounds how long the host waits for an ODM it started.
const startWait = 20 * time.Second

// runNativeHost relays browser native-messaging requests to a running ODM
// instance over its control socket, starting ODM first when a download
// comes and it is not running. It is launched by the browser, not by hand,
// as the same user, so it resolves the same config directory; ODM_SOCKET
// overrides the socket path.
func runNativeHost(configDir string) {
	path := os.Getenv("ODM_SOCKET")
	var start func() error // nil: ODM_SOCKET names an ODM started some other way
	if path == "" {
		path = ipc.SocketPath(configDir)
		start = func() error { return startODM(path) }
	}
	client := ipc.Client(path, 10*time.Second)

	err := native.Serve(os.Stdin, os.Stdout, func(m native.Message) native.Reply {
		switch m.Type {
		case "ping":
			return relayStarting(func() native.Reply { return relayPing(client) }, start)
		case "config":
			// Asked for every browser download, wanted or not, so it must
			// not start ODM.
			return relayConfig(client)
		case "", "download":
			return relayStarting(func() native.Reply { return relayDownload(client, m) }, start)
		case "streams":
			return relayStarting(func() native.Reply { return relayStreams(client, m) }, start)
		case "site-formats":
			return relayStarting(func() native.Reply { return relaySite(client, m) }, start)
		}
		return native.Reply{Error: "unknown message type: " + m.Type}
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "odm native host:", err)
	}
}

// relayStarting runs relay and, when ODM is not running, starts it with
// start (nil: it cannot) and runs relay again: a download from the browser
// should not depend on ODM having been opened first.
func relayStarting(relay func() native.Reply, start func() error) native.Reply {
	reply := relay()
	if reply.OK || reply.Error != errNotRunning || start == nil {
		return reply
	}
	if err := start(); err != nil {
		return native.Reply{Error: errNotRunning + " and could not be started: " + err.Error()}
	}
	return relay()
}

// startODM starts ODM in the system tray, as a start at login does, and
// waits until it answers on its control socket at sock.
func startODM(sock string) error {
	exe, err := selfExe()
	if err != nil {
		return err
	}
	if err := startDetached(exe, "-hidden"); err != nil {
		return err
	}
	for deadline := time.Now().Add(startWait); time.Now().Before(deadline); time.Sleep(200 * time.Millisecond) {
		if c, err := net.DialTimeout("unix", sock, time.Second); err == nil {
			c.Close()
			return nil
		}
	}
	return fmt.Errorf("no answer within %v", startWait)
}

// relayPing answers the extension's connection test: OK only when ODM
// itself answers, not just this relay.
func relayPing(client *http.Client) native.Reply {
	resp, err := client.Get(ipc.BaseURL + "/api/config")
	if err != nil {
		return native.Reply{Error: errNotRunning}
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return native.Reply{Error: fmt.Sprintf("ODM returned %s", resp.Status)}
	}
	return native.Reply{OK: true, Message: "odm"}
}

// relayDownload hands a browser download to ODM. The extension cancels the
// browser's own download only when this replies OK, so every failure must
// come back as an error rather than a silent success.
func relayDownload(client *http.Client, m native.Message) native.Reply {
	if m.URL == "" {
		return native.Reply{Error: "missing url"}
	}
	headers := browserHeaders(m)
	payload := map[string]any{
		"url":         m.URL,
		"dir":         m.Dir,
		"connections": m.Connections,
		"file_name":   native.SuggestName(m.Filename, m.Title, m.Mime),
		"prompt":      true, // a person clicked this in the browser
	}
	if m.Kind == "site" {
		// yt-dlp reads the page itself: no browser headers, which would
		// hand the site's cookies to it.
		name := native.StreamName(m.Title)
		if name != "" && m.Ext != "" {
			name += "." + m.Ext
		}
		payload["file_name"] = name
		payload["site"] = true
		payload["format"] = m.Format
		payload["estimate"] = m.Estimate
		headers = nil
	}
	if len(headers) > 0 {
		payload["headers"] = headers
	}
	if m.Kind == "stream" {
		// The extension only saw the response headers, not the body, so
		// the format comes from the Content-Type and the URL.
		k := media.Detect(m.Mime, m.URL, nil)
		if k == media.KindFile || k == media.KindNone {
			// The extension trusts a playlist extension over a media type
			// some servers mislabel playlists with (video/mp2t, ...).
			k = media.Detect("", m.URL, nil)
		}
		if k != media.KindHLS && k != media.KindDASH {
			return native.Reply{Error: "this stream format is not supported"}
		}
		payload["stream"] = string(k)
		payload["file_name"] = native.StreamName(m.Title)
		if m.Quality != "" {
			payload["quality"] = m.Quality
		}
	}
	body, _ := json.Marshal(payload)
	resp, err := client.Post(ipc.BaseURL+"/api/downloads", "application/json", bytes.NewReader(body))
	if err != nil {
		return native.Reply{Error: errNotRunning}
	}
	defer resp.Body.Close()
	var info struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&info)
	switch resp.StatusCode {
	case http.StatusAccepted:
		return native.Reply{OK: true, Message: "opened in ODM"}
	case http.StatusCreated:
		return native.Reply{OK: true, ID: info.ID, Message: "queued"}
	case http.StatusOK:
		return native.Reply{OK: true, ID: info.ID, Message: "already in the list"}
	}
	return native.Reply{Error: fmt.Sprintf("ODM returned %s", resp.Status)}
}

// browserHeaders are the request headers the browser would send for m:
// its own, the page as referrer, the site's cookies and its user agent.
func browserHeaders(m native.Message) map[string]string {
	headers := map[string]string{}
	for k, v := range m.Headers {
		headers[k] = v
	}
	if m.Referrer != "" {
		headers["Referer"] = m.Referrer
	}
	if m.Cookies != "" {
		headers["Cookie"] = m.Cookies
	}
	// Some servers only answer the browser that started the session.
	if m.UserAgent != "" {
		headers["User-Agent"] = m.UserAgent
	}
	return headers
}

// relayStreams asks ODM what qualities the playlist at m.URL offers,
// reading it with the browser's headers as the download would.
func relayStreams(client *http.Client, m native.Message) native.Reply {
	if m.URL == "" {
		return native.Reply{Error: "missing url"}
	}
	body, _ := json.Marshal(map[string]any{"url": m.URL, "headers": browserHeaders(m)})
	resp, err := client.Post(ipc.BaseURL+"/api/probe/stream", "application/json", bytes.NewReader(body))
	if err != nil {
		return native.Reply{Error: errNotRunning}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var e struct {
			Error   string `json:"error"`
			Refused bool   `json:"refused"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&e)
		if e.Error == "" {
			e.Error = fmt.Sprintf("ODM returned %s", resp.Status)
		}
		return native.Reply{Error: e.Error, Refused: e.Refused}
	}
	var info native.StreamInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return native.Reply{Error: "unreadable reply: " + err.Error()}
	}
	return native.Reply{OK: true, Stream: &info}
}

// relaySite asks ODM, which asks yt-dlp, what a video site's page offers.
func relaySite(client *http.Client, m native.Message) native.Reply {
	if m.URL == "" {
		return native.Reply{Error: "missing url"}
	}
	body, _ := json.Marshal(map[string]string{"url": m.URL})
	resp, err := client.Post(ipc.BaseURL+"/api/probe/site", "application/json", bytes.NewReader(body))
	if err != nil {
		return native.Reply{Error: errNotRunning}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var e struct {
			Error   string `json:"error"`
			Refused bool   `json:"refused"`
			Missing bool   `json:"missing"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&e)
		if e.Error == "" {
			e.Error = fmt.Sprintf("ODM returned %s", resp.Status)
		}
		return native.Reply{Error: e.Error, Refused: e.Refused, Missing: e.Missing}
	}
	var info native.SiteInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return native.Reply{Error: "unreadable reply: " + err.Error()}
	}
	return native.Reply{OK: true, Site: &info}
}

// relayConfig returns the settings the extension applies itself.
func relayConfig(client *http.Client) native.Reply {
	resp, err := client.Get(ipc.BaseURL + "/api/settings")
	if err != nil {
		return native.Reply{Error: errNotRunning}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return native.Reply{Error: fmt.Sprintf("ODM returned %s", resp.Status)}
	}
	var st struct {
		FileTypes    []string `json:"file_types"`
		ExcludeSites []string `json:"exclude_sites"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&st); err != nil {
		return native.Reply{Error: "unreadable settings: " + err.Error()}
	}
	if st.ExcludeSites == nil {
		st.ExcludeSites = []string{}
	}
	return native.Reply{OK: true, Config: &native.Config{FileTypes: st.FileTypes, ExcludeSites: st.ExcludeSites}}
}
