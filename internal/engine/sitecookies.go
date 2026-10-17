package engine

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"odm/internal/ytdlp"
)

// CookiesFromExtension, as Settings.YtDlpCookies, has yt-dlp sign in to a
// video site with the cookies ODM's browser extension sends along with
// the site's page. yt-dlp cannot take them from Chrome itself: Chrome
// locks its cookie database while it runs, and since version 127 keeps
// the cookies under an encryption yt-dlp cannot undo.
const CookiesFromExtension = "extension"

// BrowserCookie is one cookie of a video site's page, as the extension's
// browser holds it.
type BrowserCookie struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Domain   string `json:"domain"`
	Path     string `json:"path"`
	Secure   bool   `json:"secure"`
	HTTPOnly bool   `json:"httpOnly"`
	HostOnly bool   `json:"hostOnly"`
	Expires  int64  `json:"expires"` // Unix seconds; 0 for a session cookie
}

// siteCookies are the cookies the extension sent last for each video site
// (siteKey), kept in memory only, while the Options say to use them.
var siteCookies = struct {
	sync.Mutex
	bySite map[string][]BrowserCookie
}{bySite: map[string][]BrowserCookie{}}

// SetSiteCookies keeps the cookies the extension sent with a video site's
// page for yt-dlp's next runs on that site, when Settings.YtDlpCookies is
// CookiesFromExtension; otherwise they are dropped. They replace the ones
// kept before for the site.
func (m *Manager) SetSiteCookies(page string, cookies []BrowserCookie) {
	key := siteKey(page)
	if key == "" || len(cookies) == 0 || m.Settings().YtDlpCookies != CookiesFromExtension {
		return
	}
	siteCookies.Lock()
	siteCookies.bySite[key] = append([]BrowserCookie(nil), cookies...)
	siteCookies.Unlock()
}

// forgetSiteCookies drops every site's cookies, when the Options stop
// using them.
func forgetSiteCookies() {
	siteCookies.Lock()
	clear(siteCookies.bySite)
	siteCookies.Unlock()
}

// siteKey names a page's site for its cookies: the host without "www.",
// "m." or "music.", with YouTube's other hosts as youtube.com. "" for an
// address that is not one.
func siteKey(page string) string {
	u, err := url.Parse(strings.TrimSpace(page))
	if err != nil || u.Hostname() == "" {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	for _, p := range []string{"www.", "m.", "music."} {
		host = strings.TrimPrefix(host, p)
	}
	switch host {
	case "youtu.be", "youtube-nocookie.com":
		host = "youtube.com"
	}
	return host
}

// siteCookieArgs is how yt-dlp signs in to page's site, as the Options
// say: none, the cookies of a browser it reads itself, or the cookies the
// extension sent for the site, in a cookie file that done removes.
func siteCookieArgs(page string) (c ytdlp.Cookies, done func()) {
	done = func() {}
	switch b := cookiesFrom(); b {
	case "":
		return c, done
	case CookiesFromExtension:
		siteCookies.Lock()
		list := siteCookies.bySite[siteKey(page)]
		siteCookies.Unlock()
		if len(list) == 0 {
			return c, done
		}
		file, err := writeCookieFile(list, time.Now())
		if err != nil {
			return c, done
		}
		return ytdlp.Cookies{File: file}, func() { _ = os.RemoveAll(filepath.Dir(file)) }
	default:
		return ytdlp.Cookies{Browser: b}, done
	}
}

// writeCookieFile writes cookies in the Netscape format yt-dlp reads
// (--cookies), in a folder of its own only this user can read. A session
// cookie is given a day: the format has no way to mark one that every
// reader takes alike.
func writeCookieFile(cookies []BrowserCookie, now time.Time) (string, error) {
	dir, err := os.MkdirTemp("", "odm-cookies-")
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("# Netscape HTTP Cookie File\n")
	for _, c := range cookies {
		if c.Name == "" || strings.ContainsAny(c.Name+c.Value+c.Domain+c.Path, "\t\r\n") {
			continue
		}
		domain, sub := c.Domain, "TRUE"
		if c.HostOnly {
			domain, sub = strings.TrimPrefix(domain, "."), "FALSE"
		} else if !strings.HasPrefix(domain, ".") {
			domain = "." + domain
		}
		if c.HTTPOnly {
			domain = "#HttpOnly_" + domain
		}
		path := c.Path
		if path == "" {
			path = "/"
		}
		secure := "FALSE"
		if c.Secure {
			secure = "TRUE"
		}
		expires := c.Expires
		if expires <= 0 {
			expires = now.Add(24 * time.Hour).Unix()
		}
		fmt.Fprintf(&b, "%s\t%s\t%s\t%s\t%d\t%s\t%s\n", domain, sub, path, secure, expires, c.Name, c.Value)
	}
	file := filepath.Join(dir, "cookies.txt")
	if err := os.WriteFile(file, []byte(b.String()), 0o600); err != nil {
		_ = os.RemoveAll(dir)
		return "", err
	}
	return file, nil
}
