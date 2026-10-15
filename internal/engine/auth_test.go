package engine

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

// The worked example from RFC 2617 section 3.5.
func TestDigestAuthorizationRFC2617(t *testing.T) {
	ch := digestChallenge{
		realm: "testrealm@host.com", nonce: "dcd98b7102dd2f0e8b11d0f600bfb0c093",
		opaque: "5ccc069c403ebaf9f0171e9517f40e41", qop: "auth,auth-int",
	}
	got := digestAuthorization(ch, "GET", "/dir/index.html", "Mufasa", "Circle Of Life", 1, "0a4f113b")
	if !strings.Contains(got, `response="6629fae49393a05397450978507c4ef1"`) {
		t.Fatalf("digest response wrong: %s", got)
	}
	for _, want := range []string{`username="Mufasa"`, `uri="/dir/index.html"`, `qop=auth`, `nc=00000001`, `cnonce="0a4f113b"`, `opaque="5ccc069c403ebaf9f0171e9517f40e41"`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in %s", want, got)
		}
	}
}

func TestParseChallenges(t *testing.T) {
	cs := parseChallenges(`Digest realm="a, b", qop="auth,auth-int", nonce="n\"1", algorithm=SHA-256, Basic realm="x"`)
	if len(cs) != 2 || cs[0].scheme != "digest" || cs[1].scheme != "basic" {
		t.Fatalf("challenges: %+v", cs)
	}
	d := cs[0].digest()
	if d.realm != "a, b" || d.qop != "auth,auth-int" || d.nonce != `n"1` || d.algorithm != "SHA-256" {
		t.Fatalf("digest params: %+v", d)
	}
	if cs[1].params["realm"] != "x" {
		t.Fatalf("basic realm: %+v", cs[1].params)
	}
}

func TestLoginHost(t *testing.T) {
	for in, want := range map[string]string{
		"example.com":                 "example.com",
		" Example.COM ":               "example.com",
		"https://example.com/files/":  "example.com",
		"example.com:8080":            "example.com",
		"http://user@example.com:81/": "example.com",
		"[::1]:8080":                  "::1",
		"example.com.":                "example.com",
	} {
		if got := loginHost(in); got != want {
			t.Errorf("loginHost(%q) = %q, want %q", in, got, want)
		}
	}
}

// protectedServer serves data with ranges behind Basic or Digest auth for
// user/pass, counting requests and those that came without credentials.
func protectedServer(t *testing.T, scheme string, data []byte) (url string, requests, bare *int64) {
	t.Helper()
	requests, bare = new(int64), new(int64)
	const user, pass, realm, nonce = "ryan", "s3cret", "files", "abc123"
	srv := httpServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(requests, 1)
		authz := r.Header.Get("Authorization")
		if authz == "" {
			atomic.AddInt64(bare, 1)
		}
		ok := false
		switch scheme {
		case "basic":
			u, p, has := r.BasicAuth()
			ok = has && u == user && p == pass
			if !ok {
				w.Header().Set("WWW-Authenticate", `Basic realm="`+realm+`"`)
			}
		case "digest":
			if strings.HasPrefix(authz, "Digest ") {
				cs := parseChallenges(authz)
				if len(cs) == 1 {
					p := cs[0].params
					nc, _ := strconv.ParseInt(p["nc"], 16, 64)
					want := digestAuthorization(digestChallenge{realm: realm, nonce: nonce, qop: "auth", algorithm: p["algorithm"]},
						r.Method, p["uri"], user, pass, int(nc), p["cnonce"])
					ok = p["username"] == user && p["uri"] == r.URL.RequestURI() &&
						strings.Contains(want, `response="`+p["response"]+`"`)
				}
			}
			if !ok {
				w.Header().Add("WWW-Authenticate", `Basic realm="`+realm+`"`)
				w.Header().Add("WWW-Authenticate", `Digest realm="`+realm+`", qop="auth", nonce="`+nonce+`", algorithm=SHA-256`)
			}
		}
		if !ok {
			http.Error(w, "login required", http.StatusUnauthorized)
			return
		}
		serveRange(w, r, data)
	})
	return srv.URL, requests, bare
}

func protectedDownload(t *testing.T, url, pass string, conns int) (*Download, string) {
	t.Helper()
	dir := t.TempDir()
	d := New("auth", url+"/private.bin", dir, conns, nil, 0, http.DefaultClient)
	d.SetAuthLookup(func(host string) (string, string, bool) {
		if host == "127.0.0.1" {
			return "ryan", pass, true
		}
		return "", "", false
	})
	return d, dir
}

func TestSiteLoginBasic(t *testing.T) {
	data := randomData(512 << 10)
	url, _, bare := protectedServer(t, "basic", data)
	d, dir := protectedDownload(t, url, "s3cret", 4)
	d.Start(context.Background())
	info := d.Info()
	if info.Status != StatusComplete {
		t.Fatalf("a Basic-protected file with its login: %s (%s)", info.Status, info.Error)
	}
	got, _ := os.ReadFile(filepath.Join(dir, info.FileName))
	if !bytes.Equal(got, data) {
		t.Fatal("file does not match")
	}
	if n := atomic.LoadInt64(bare); n != 1 {
		t.Fatalf("%d requests went without the login; only the first should, then the host is known", n)
	}
}

func TestSiteLoginDigest(t *testing.T) {
	data := randomData(512 << 10)
	url, _, _ := protectedServer(t, "digest", data)
	d, dir := protectedDownload(t, url, "s3cret", 4)
	d.Start(context.Background())
	info := d.Info()
	if info.Status != StatusComplete {
		t.Fatalf("a Digest-protected file with its login: %s (%s)", info.Status, info.Error)
	}
	got, _ := os.ReadFile(filepath.Join(dir, info.FileName))
	if !bytes.Equal(got, data) {
		t.Fatal("file does not match")
	}
}

func TestSiteLoginWrongPasswordDoesNotLoop(t *testing.T) {
	url, requests, _ := protectedServer(t, "digest", randomData(64<<10))
	d, _ := protectedDownload(t, url, "wrong", 1)
	d.Start(context.Background())
	info := d.Info()
	if info.Status != StatusError || !strings.Contains(info.Error, "401") {
		t.Fatalf("wrong password: %s (%s)", info.Status, info.Error)
	}
	if n := atomic.LoadInt64(requests); n > 2 {
		t.Fatalf("%d requests for a wrong password; one challenge and one answer are enough", n)
	}
}

func TestNoLoginLeavesRequestsAlone(t *testing.T) {
	url, _, _ := protectedServer(t, "basic", randomData(64<<10))
	d := New("auth", url+"/private.bin", t.TempDir(), 1, nil, 0, http.DefaultClient)
	d.SetAuthLookup(func(string) (string, string, bool) { return "", "", false })
	d.Start(context.Background())
	if info := d.Info(); info.Status != StatusError || !strings.Contains(info.Error, "401") {
		t.Fatalf("without a login: %s (%s)", info.Status, info.Error)
	}
}
