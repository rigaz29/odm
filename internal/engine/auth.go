package engine

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"net/http"
	"strings"
	"sync"
)

// authorizer answers HTTP authentication challenges with the site login
// stored for the host (Options → Site logins). A request goes out without
// credentials; a 401 naming Digest or Basic is answered once with that
// scheme, and later requests to the host carry it from the start. Nothing
// is sent to a host that never asked, or after a redirect to another host.
type authorizer struct {
	creds func(host string) (user, pass string, ok bool)

	mu    sync.Mutex
	hosts map[string]*hostAuth
}

// hostAuth is what one host asked for.
type hostAuth struct {
	digest bool
	ch     digestChallenge
	nc     int // requests sent with the current nonce
}

// digestChallenge holds the parameters of a WWW-Authenticate: Digest.
type digestChallenge struct {
	realm, nonce, opaque, qop, algorithm string
	stale                                bool
}

func newAuthorizer(creds func(string) (string, string, bool)) *authorizer {
	if creds == nil {
		return nil
	}
	return &authorizer{creds: creds, hosts: map[string]*hostAuth{}}
}

// do sends req through send, answering one authentication challenge with the
// stored login for req's host. send is client.Do or a wrapper around it.
func (a *authorizer) do(req *http.Request, send func(*http.Request) (*http.Response, error)) (*http.Response, error) {
	if a == nil {
		return send(req)
	}
	host := strings.ToLower(req.URL.Hostname())
	user, pass, ok := a.creds(host)
	if !ok {
		return send(req)
	}
	sentWith := a.apply(req, host, user, pass)
	resp, err := send(req)
	if err != nil || resp.StatusCode != http.StatusUnauthorized {
		return resp, err
	}
	// Answer only the host the login is for, not one a redirect led to.
	if resp.Request == nil || !strings.EqualFold(resp.Request.URL.Hostname(), host) {
		return resp, nil
	}
	stale, learned := a.learn(host, resp.Header.Values("WWW-Authenticate"))
	// Credentials sent and refused again, with nothing new to try: the login
	// is wrong, so the 401 stands rather than looping.
	if !learned || (sentWith && !stale) {
		return resp, nil
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	resp.Body.Close()
	retry := req.Clone(req.Context())
	a.apply(retry, host, user, pass)
	return send(retry)
}

// apply sets Authorization for a host that asked for credentials before,
// reporting whether it did.
func (a *authorizer) apply(req *http.Request, host, user, pass string) bool {
	a.mu.Lock()
	h := a.hosts[host]
	if h == nil {
		a.mu.Unlock()
		return false
	}
	if !h.digest {
		a.mu.Unlock()
		req.SetBasicAuth(user, pass)
		return true
	}
	h.nc++
	ch, nc := h.ch, h.nc
	a.mu.Unlock()
	req.Header.Set("Authorization", digestAuthorization(ch, req.Method, req.URL.RequestURI(), user, pass, nc, newCnonce()))
	return true
}

// learn reads the challenges of a 401. Digest is preferred to Basic, and
// SHA-256 to MD5. It reports whether a usable one was found, and whether
// the server only said the nonce was stale.
func (a *authorizer) learn(host string, headers []string) (stale, ok bool) {
	var best *digestChallenge
	basic := false
	for _, v := range headers {
		for _, c := range parseChallenges(v) {
			switch c.scheme {
			case "basic":
				basic = true
			case "digest":
				d := c.digest()
				if !digestSupported(d) {
					continue
				}
				if best == nil || digestRank(d) > digestRank(*best) {
					dd := d
					best = &dd
				}
			}
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	switch {
	case best != nil:
		a.hosts[host] = &hostAuth{digest: true, ch: *best}
		return best.stale, true
	case basic:
		a.hosts[host] = &hostAuth{}
		return false, true
	}
	return false, false
}

func digestSupported(c digestChallenge) bool {
	switch strings.ToUpper(c.algorithm) {
	case "", "MD5", "MD5-SESS", "SHA-256", "SHA-256-SESS":
	default:
		return false
	}
	// qop: absent (RFC 2069) or containing "auth"; "auth-int" alone would
	// need the body hashed, which a GET without one does not.
	if c.qop == "" {
		return true
	}
	for _, q := range strings.Split(c.qop, ",") {
		if strings.TrimSpace(q) == "auth" {
			return true
		}
	}
	return false
}

func digestRank(c digestChallenge) int {
	if strings.HasPrefix(strings.ToUpper(c.algorithm), "SHA-256") {
		return 2
	}
	return 1
}

// digestAuthorization builds the Authorization value for a Digest
// challenge (RFC 7616, and RFC 2617 for MD5).
func digestAuthorization(c digestChallenge, method, uri, user, pass string, nc int, cnonce string) string {
	alg := strings.ToUpper(c.algorithm)
	var newHash func() hash.Hash = md5.New
	if strings.HasPrefix(alg, "SHA-256") {
		newHash = sha256.New
	}
	h := func(s string) string {
		x := newHash()
		_, _ = io.WriteString(x, s)
		return hex.EncodeToString(x.Sum(nil))
	}
	ha1 := h(user + ":" + c.realm + ":" + pass)
	if strings.HasSuffix(alg, "-SESS") {
		ha1 = h(ha1 + ":" + c.nonce + ":" + cnonce)
	}
	ha2 := h(method + ":" + uri)
	ncs := fmt.Sprintf("%08x", nc)
	qop := ""
	if c.qop != "" {
		qop = "auth"
	}
	var response string
	if qop != "" {
		response = h(ha1 + ":" + c.nonce + ":" + ncs + ":" + cnonce + ":" + qop + ":" + ha2)
	} else {
		response = h(ha1 + ":" + c.nonce + ":" + ha2)
	}
	var b strings.Builder
	fmt.Fprintf(&b, `Digest username=%q, realm=%q, nonce=%q, uri=%q, response=%q`,
		user, c.realm, c.nonce, uri, response)
	if c.algorithm != "" {
		fmt.Fprintf(&b, `, algorithm=%s`, c.algorithm)
	}
	if qop != "" {
		fmt.Fprintf(&b, `, qop=%s, nc=%s, cnonce=%q`, qop, ncs, cnonce)
	}
	if c.opaque != "" {
		fmt.Fprintf(&b, `, opaque=%q`, c.opaque)
	}
	return b.String()
}

func newCnonce() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// challenge is one scheme with its parameters from a WWW-Authenticate.
type challenge struct {
	scheme string
	params map[string]string
}

func (c challenge) digest() digestChallenge {
	return digestChallenge{
		realm: c.params["realm"], nonce: c.params["nonce"], opaque: c.params["opaque"],
		qop: c.params["qop"], algorithm: c.params["algorithm"],
		stale: strings.EqualFold(c.params["stale"], "true"),
	}
}

// parseChallenges splits a WWW-Authenticate value into challenges. One
// header may hold several ("Digest realm=..., Basic realm=..."); quoted
// values may hold commas.
func parseChallenges(v string) []challenge {
	var out []challenge
	var cur *challenge
	s := strings.TrimSpace(v)
	for s != "" {
		s = strings.TrimLeft(s, " ,\t")
		if s == "" {
			break
		}
		// A token followed by "=" is a parameter of the current challenge;
		// a token followed by a space or the end starts a new one.
		tok := s
		if i := strings.IndexAny(s, " =,\t"); i >= 0 {
			tok = s[:i]
		}
		rest := s[len(tok):]
		if strings.HasPrefix(strings.TrimLeft(rest, " \t"), "=") && cur != nil {
			rest = strings.TrimLeft(rest, " \t")[1:]
			rest = strings.TrimLeft(rest, " \t")
			var val string
			if strings.HasPrefix(rest, `"`) {
				val, rest = readQuoted(rest[1:])
			} else {
				end := strings.IndexByte(rest, ',')
				if end < 0 {
					end = len(rest)
				}
				val, rest = strings.TrimSpace(rest[:end]), rest[end:]
			}
			cur.params[strings.ToLower(tok)] = val
			s = rest
			continue
		}
		out = append(out, challenge{scheme: strings.ToLower(tok), params: map[string]string{}})
		cur = &out[len(out)-1]
		s = rest
	}
	return out
}

// readQuoted reads a quoted-string body (after the opening quote), undoing
// backslash escapes, and returns it with what follows the closing quote.
func readQuoted(s string) (string, string) {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			if i+1 < len(s) {
				i++
				b.WriteByte(s[i])
			}
		case '"':
			return b.String(), s[i+1:]
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String(), ""
}
