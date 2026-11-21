package engine

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// otherSite is the test server under another host name: another site to
// sameSite, while the same server answers.
func otherSite(u string) string { return strings.Replace(u, "127.0.0.1", "localhost", 1) }

// A download handed over with a site's cookies and login, whose address
// redirects to another site, sends neither there. The probe follows the
// redirect, where Go drops them itself, but every transfer is a fresh
// request to the final address.
func TestTransfersKeepCredentialsFromAnotherSite(t *testing.T) {
	data := randomData(128 << 10)
	var mu sync.Mutex
	var seen []string
	other := httpServer(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.Header.Get("Range")+" Cookie="+r.Header.Get("Cookie")+" Authorization="+r.Header.Get("Authorization"))
		mu.Unlock()
		serveRange(w, r, data)
	})
	target := otherSite(other.URL) + "/payload.bin"
	site := httpServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target, http.StatusFound)
	})
	d := New("redirect", site.URL+"/out?to=payload", t.TempDir(), 2,
		map[string]string{"Cookie": "session=SECRET", "Authorization": "Bearer TOKEN"}, 0, http.DefaultClient)
	t.Cleanup(func() { closePart(d) })
	d.Start(context.Background())
	if info := d.Info(); info.Status != StatusComplete {
		t.Fatalf("status=%s err=%q", info.Status, info.Error)
	}
	mu.Lock()
	defer mu.Unlock()
	for _, s := range seen {
		if strings.Contains(s, "SECRET") || strings.Contains(s, "TOKEN") {
			t.Fatalf("a request to the other site carried the first one's credentials:\n  %s", strings.Join(seen, "\n  "))
		}
	}
}

// The ZIP preview reads the archive's end with fresh requests to the final
// address too.
func TestZipPreviewKeepsCredentialsFromAnotherSite(t *testing.T) {
	archive := buildZip(t, map[string][]byte{"a.txt": []byte("a")})
	var mu sync.Mutex
	var seen []string
	other := httpServer(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.Header.Get("Cookie"))
		mu.Unlock()
		serveRange(w, r, archive)
	})
	target := otherSite(other.URL) + "/pack.zip"
	site := httpServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target, http.StatusFound)
	})
	m := NewManager(Settings{Dir: t.TempDir()}, "")
	if _, err := m.PreviewZip(context.Background(), site.URL+"/download?id=1",
		map[string]string{"Cookie": "session=site-secret"}); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	for _, c := range seen {
		if strings.Contains(c, "site-secret") {
			t.Fatalf("the first site's cookie reached the other one: %q", seen)
		}
	}
}

// A server that sets a cookie of its own on the file's answer (a load
// balancer's) does not take the login the download was handed with away
// from the transfers.
func TestServerCookiesJoinTheLogin(t *testing.T) {
	data := randomData(256 << 10)
	var refused atomic.Int32
	srv := httpServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "AWSALB", Value: "node7"})
		if !strings.Contains(r.Header.Get("Cookie"), "session=1") {
			refused.Add(1)
			http.Error(w, "login required", http.StatusForbidden)
			return
		}
		serveRange(w, r, data)
	})
	d := New("cookie", srv.URL+"/members/file.bin", t.TempDir(), 2,
		map[string]string{"Cookie": "session=1"}, 0, srv.Client())
	d.retryBase = time.Millisecond
	t.Cleanup(func() { closePart(d) })
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	d.Start(ctx)
	if info := d.Info(); info.Status != StatusComplete {
		t.Fatalf("status=%s err=%q; %d requests refused", info.Status, info.Error, refused.Load())
	}
}

func TestAddCookies(t *testing.T) {
	for _, c := range []struct {
		header string
		set    []*http.Cookie
		want   string
	}{
		{"", []*http.Cookie{{Name: "a", Value: "1"}}, "a=1"},
		{"session=1", nil, "session=1"},
		{"session=1; lb=old", []*http.Cookie{{Name: "lb", Value: "new"}, {Name: "x", Value: "2"}}, "session=1; lb=new; x=2"},
		{" session=1 ;; theme=dark", []*http.Cookie{{Name: "session", Value: "2"}}, "session=2; theme=dark"},
	} {
		req, _ := http.NewRequest(http.MethodGet, "http://example.com/", nil)
		if c.header != "" {
			req.Header.Set("Cookie", c.header)
		}
		addCookies(req, c.set)
		if got := req.Header.Get("Cookie"); got != c.want {
			t.Errorf("addCookies(%q, %v) = %q, want %q", c.header, c.set, got, c.want)
		}
	}
}
