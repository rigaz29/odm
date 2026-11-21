package engine

import (
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
)

// userAgent is the configured User-Agent. Downloads read it from their own
// goroutines while Options may change it, hence the atomic.
var userAgent atomic.Pointer[string]

func setUserAgent(ua string) {
	if ua == "" {
		ua = DefaultUserAgent
	}
	userAgent.Store(&ua)
}

func currentUserAgent() string {
	if p := userAgent.Load(); p != nil {
		return *p
	}
	return DefaultUserAgent
}

// applyHeaders sets the user agent plus any caller-supplied headers.
// Caller headers win over the user agent.
func applyHeaders(req *http.Request, headers map[string]string) {
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", currentUserAgent())
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
}

// headersFor is the headers a download was handed with, for a request to
// target. Any Cookie and Authorization came with the address the download
// was given (from the browser, for that site), so they go only to hosts of
// the same site; anything else would hand the user's session to whichever
// host the address redirects to. Go drops them on a redirect to another
// domain, but each transfer is a fresh request to the final address, so that
// protection does not apply there.
func headersFor(headers map[string]string, origin, target string) map[string]string {
	if u, err := url.Parse(target); err == nil && sameSite(origin, u) {
		return headers
	}
	return withoutCredentials(headers)
}

// withoutCredentials is headers without Cookie and Authorization.
func withoutCredentials(headers map[string]string) map[string]string {
	safe := make(map[string]string, len(headers))
	for k, v := range headers {
		if !strings.EqualFold(k, "Cookie") && !strings.EqualFold(k, "Authorization") {
			safe[k] = v
		}
	}
	return safe
}

// addCookies puts the cookies a server set into the request's Cookie
// header. One with a name the header already has replaces that value, as a
// browser's cookie store would; the others stay, so a login cookie handed
// over with the download is not lost when the server sets one of its own (a
// load balancer's, say).
func addCookies(req *http.Request, cookies []*http.Cookie) {
	if len(cookies) == 0 {
		return
	}
	set := make(map[string]string, len(cookies))
	for _, c := range cookies {
		set[c.Name] = c.Value
	}
	var parts []string
	for _, pair := range strings.Split(req.Header.Get("Cookie"), ";") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		name, _, _ := strings.Cut(pair, "=")
		name = strings.TrimSpace(name)
		if v, ok := set[name]; ok {
			pair = name + "=" + v
			delete(set, name)
		}
		parts = append(parts, pair)
	}
	for _, c := range cookies {
		if v, ok := set[c.Name]; ok {
			parts = append(parts, c.Name+"="+v)
			delete(set, c.Name)
		}
	}
	req.Header.Set("Cookie", strings.Join(parts, "; "))
}
