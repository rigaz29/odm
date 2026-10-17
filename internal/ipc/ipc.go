// Package ipc is ODM's local control channel: plain HTTP carried over a Unix
// domain socket in the user's config directory.
//
// Why a socket and not TCP loopback: any web page can send requests to
// 127.0.0.1, so a loopback API is reachable from the browser (CSRF, DNS
// rebinding). A socket file is reachable only by local processes that can
// open it, and the config directory already belongs to the user. Unix sockets
// also exist on Windows 10 1803+, so one transport serves every OS without a
// named-pipe dependency.
package ipc

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// SocketName is the socket file inside the config directory.
const SocketName = "odm.sock"

// BaseURL is the URL prefix for requests over the socket; the host part is
// ignored because Client always dials the socket.
const BaseURL = "http://odm"

// BrowserHeader names the browser whose extension sent a request the
// native host relays ("chrome", "firefox", ...), so ODM knows which
// browsers have its extension.
const BrowserHeader = "X-ODM-Browser"

// ErrRunning reports that another ODM instance already owns the socket.
var ErrRunning = errors.New("ODM is already running")

// SocketPath returns the socket location for a config directory.
func SocketPath(configDir string) string { return filepath.Join(configDir, SocketName) }

// Listen claims the socket, which doubles as the single-instance lock.
//
// A socket file left behind by a crash refuses connections, so a failed dial
// means the file is stale and can be replaced. Two instances starting in the
// same instant could both see it as stale; the window is milliseconds and the
// loser only fails to receive browser hand-offs, so no lock file is added.
func Listen(path string) (net.Listener, error) {
	if c, err := net.DialTimeout("unix", path, 500*time.Millisecond); err == nil {
		_ = c.Close()
		return nil, ErrRunning
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	_ = os.Remove(path)
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if runtime.GOOS != "windows" {
		// Owner-only, in case the config directory is more open than usual.
		// On Windows the file inherits the per-user AppData ACL instead.
		_ = os.Chmod(path, 0o600)
	}
	return ln, nil
}

// Client returns an HTTP client whose every request goes to the socket.
func Client(path string, timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", path)
			},
		},
	}
}
