// Package native implements the Chrome/Firefox native-messaging stdio protocol:
// a 4-byte little-endian length followed by a UTF-8 JSON payload.
package native

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// maxMessage caps a single inbound message (Chrome allows up to 64 MiB).
const maxMessage = 64 << 20

// Message is a request from the browser extension.
type Message struct {
	Type        string            `json:"type"`
	URL         string            `json:"url"`
	Filename    string            `json:"filename"`
	Referrer    string            `json:"referrer"`
	Cookies     string            `json:"cookies"`
	Headers     map[string]string `json:"headers"`
	Connections int               `json:"connections"`
	Dir         string            `json:"dir"`
	// Hints the browser has: the page title (a better name for media than
	// "videoplayback"), MIME type, size and the browser's user agent.
	Title     string `json:"title"`
	Mime      string `json:"mime"`
	Size      int64  `json:"size"`
	UserAgent string `json:"userAgent"`
	// Kind is "stream" for an HLS/DASH playlist the extension saw the
	// page's player load, "live" for a live stream it reads as one
	// endless response (FLV) and "site" for a video site's page; anything
	// else is a plain file.
	Kind string `json:"kind"`
	// Quality is the label of the stream quality picked in the
	// extension's menu, as a "streams" reply listed it. Empty: the best.
	Quality string `json:"quality"`
	// For Kind "site", a video site's page that yt-dlp downloads: the
	// format picked in the menu (yt-dlp's selector), the container it
	// gives and the expected size, as a "site-formats" reply listed them.
	Format   string `json:"format"`
	Ext      string `json:"ext"`
	Estimate int64  `json:"estimate"`
	// Page is the video site's page a live stream's playlist (URL) was
	// found on, as a "site-formats" reply gave it.
	Page string `json:"page"`
	// SiteCookies are a video site page's cookies, as the extension's
	// browser holds them, for yt-dlp when ODM's Options say to sign in
	// with them; passed on as they came.
	SiteCookies json.RawMessage `json:"siteCookies,omitempty"`
}

// Reply is the response sent back to the extension.
type Reply struct {
	OK      bool    `json:"ok"`
	Error   string  `json:"error,omitempty"`
	ID      string  `json:"id,omitempty"`
	Message string  `json:"message,omitempty"`
	Config  *Config `json:"config,omitempty"`
	// Refused marks an Error that will not go away: a stream that can
	// never be downloaded (DRM, not a playlist).
	Refused bool        `json:"refused,omitempty"`
	Stream  *StreamInfo `json:"stream,omitempty"`
	// Missing marks an Error that says yt-dlp is not installed.
	Missing bool      `json:"missing,omitempty"`
	Site    *SiteInfo `json:"site,omitempty"`
}

// SiteInfo is what yt-dlp found at a video site's page, answered to a
// "site-formats" message.
type SiteInfo struct {
	Title     string        `json:"title"`
	Qualities []SiteQuality `json:"qualities"`
	// A live broadcast is recorded as a stream from Manifest, fetched with
	// Headers; Stream lists its qualities when ODM could read them. Kind
	// is "hls", "dash" or "http-live" (one endless response, one quality).
	Live     bool              `json:"live,omitempty"`
	Kind     string            `json:"kind,omitempty"`
	Manifest string            `json:"manifest,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`
	Stream   *StreamInfo       `json:"stream,omitempty"`
}

// SiteQuality is one way to download a page's video: its label, yt-dlp's
// format selector, the container and the expected size (0: unknown).
type SiteQuality struct {
	Label    string `json:"label"`
	Format   string `json:"format"`
	Ext      string `json:"ext"`
	Estimate int64  `json:"estimate,omitempty"`
}

// StreamInfo lists a playlist's qualities, answered to a "streams" message
// so the extension's menu can offer them. Single is set for a playlist
// that is one quality of a stream rather than the list of them.
type StreamInfo struct {
	Kind      string          `json:"kind"`
	Single    bool            `json:"single,omitempty"`
	Live      bool            `json:"live,omitempty"`
	Note      string          `json:"note,omitempty"`
	Qualities []StreamQuality `json:"qualities"`
}

// StreamQuality is one quality: its label (resolution, bitrate, codecs),
// the expected size in bytes (0: unknown) and its audio tracks.
type StreamQuality struct {
	Label    string   `json:"label"`
	Estimate int64    `json:"estimate,omitempty"`
	Audios   []string `json:"audios,omitempty"`
}

// Config is what the extension needs from ODM's settings, answered to a
// "config" message so the takeover list is edited in one place.
type Config struct {
	FileTypes    []string `json:"fileTypes"`
	ExcludeSites []string `json:"excludeSites"`
}

// ReadMessage reads one framed message.
func ReadMessage(r io.Reader) (Message, error) {
	var m Message
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return m, err
	}
	n := binary.LittleEndian.Uint32(hdr[:])
	if n == 0 {
		return m, errors.New("native: empty message")
	}
	if n > maxMessage {
		return m, fmt.Errorf("native: message too large (%d bytes)", n)
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return m, err
	}
	if err := json.Unmarshal(buf, &m); err != nil {
		return m, fmt.Errorf("native: invalid json: %w", err)
	}
	return m, nil
}

// WriteMessage frames and writes one value.
func WriteMessage(w io.Writer, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	var hdr [4]byte
	binary.LittleEndian.PutUint32(hdr[:], uint32(len(data)))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

// Serve reads messages until EOF, replying to each one. It returns nil on a
// clean EOF (the browser closed the port).
func Serve(r io.Reader, w io.Writer, handle func(Message) Reply) error {
	for {
		msg, err := ReadMessage(r)
		if err != nil {
			switch {
			case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
				return nil
			default:
				_ = WriteMessage(w, Reply{Error: err.Error()})
				return nil
			}
		}
		if err := WriteMessage(w, handle(msg)); err != nil {
			return err
		}
	}
}
