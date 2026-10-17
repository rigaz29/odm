package engine

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"testing"
)

// fakeNet answers requests without a network: a redirect where one is
// set, else 200. It records the hosts it was asked for.
type fakeNet struct {
	redirect map[string]string
	asked    []string
}

func (f *fakeNet) RoundTrip(r *http.Request) (*http.Response, error) {
	f.asked = append(f.asked, r.URL.Host)
	resp := &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("ok")), Request: r}
	if to, ok := f.redirect[r.URL.Host]; ok {
		resp.StatusCode = http.StatusFound
		resp.Header.Set("Location", to)
	}
	return resp, nil
}

func fakeDNS(t *testing.T, table map[string][]string) {
	t.Helper()
	old := lookupIPAddr
	lookupIPAddr = func(_ context.Context, host string) ([]net.IPAddr, error) {
		ips, ok := table[host]
		if !ok {
			return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
		}
		var out []net.IPAddr
		for _, ip := range ips {
			out = append(out, net.IPAddr{IP: net.ParseIP(ip)})
		}
		return out, nil
	}
	t.Cleanup(func() { lookupIPAddr = old })
}

// TestLocalGuardRequests covers what RoundTrip itself refuses: names and
// literals in every spelling. Names that only resolve to a local address
// are the dialer's job (TestLocalGuardDial).
func TestLocalGuardRequests(t *testing.T) {
	fakeDNS(t, map[string][]string{
		"cdn.example":   {"93.184.216.34"},
		"split.example": {"93.184.216.34", "fd00::1"},
		"nas.lan":       {"192.168.1.20"},
	})
	pub := "https://cdn.example/m.m3u8"
	cases := []struct {
		manifest, target string
		refused          bool
	}{
		{pub, "https://cdn.example/s1.ts", false},
		{pub, "https://other.example/s1.ts", false},
		{pub, "http://127.0.0.1:8080/admin", true},
		{pub, "http://[::1]/x", true},
		{pub, "http://[::ffff:127.0.0.1]/x", true},
		{pub, "http://[64:ff9b::c0a8:101]/x", true},
		{pub, "http://[2002:c0a8:101::1]/x", true},
		{pub, "http://localhost/x", true},
		{pub, "http://localhost./x", true},
		{pub, "http://app.localhost/x", true},
		{pub, "http://192.168.1.1/reboot", true},
		{pub, "http://169.254.169.254/latest", true},
		{pub, "http://100.100.1.1/x", true},
		{pub, "http://2130706433/x", true},
		{pub, "http://127.1/x", true},
		{pub, "http://0x7f.0.0.1/x", true},
		{pub, "http://１２７.０.０.１/x", true},
		{pub, "http://0.0.0.0/x", true},
		{pub, "http://93.184.216.34/x", false},
		// One public answer makes the manifest public, so the guard
		// stays on (a local record must not switch it off).
		{"https://split.example/m.m3u8", "http://192.168.1.1/x", true},
		// A playlist on the local network may use local addresses.
		{"http://nas.lan/m.m3u8", "http://192.168.1.21/s1.ts", false},
		{"http://127.0.0.1:9000/m.m3u8", "http://127.0.0.1:9000/s1.ts", false},
	}
	for _, c := range cases {
		next := &fakeNet{}
		client := guardLocal(&http.Client{Transport: next}, c.manifest)
		resp, err := client.Get(c.target)
		if resp != nil {
			resp.Body.Close()
		}
		if c.refused {
			if !errors.Is(err, errLocalNetwork) {
				t.Errorf("%s from %s: %v, want refusal", c.target, c.manifest, err)
			}
			if len(next.asked) != 0 {
				t.Errorf("%s from %s: request sent to %v", c.target, c.manifest, next.asked)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s from %s: %v", c.target, c.manifest, err)
		}
	}
}

func TestLocalGuardRedirect(t *testing.T) {
	fakeDNS(t, map[string][]string{"cdn.example": {"93.184.216.34"}})
	next := &fakeNet{redirect: map[string]string{"cdn.example": "http://127.0.0.1/admin"}}
	client := guardLocal(&http.Client{Transport: next}, "https://cdn.example/m.m3u8")
	_, err := client.Get("https://cdn.example/s1.ts")
	if !errors.Is(err, errLocalNetwork) {
		t.Fatalf("redirect into localhost: %v", err)
	}
	if len(next.asked) != 1 {
		t.Fatalf("asked %v, want only the first hop", next.asked)
	}
}

// TestLocalGuardDial checks the dialer: it refuses a local address for a
// marked request (whatever name led there, so DNS rebinding cannot pass),
// and the mark reaches it from RoundTrip.
func TestLocalGuardDial(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	addr := strings.TrimPrefix(srv.URL, "http://")

	fakeDNS(t, map[string][]string{"cdn.example": {"93.184.216.34"}})
	g := guardLocal(&http.Client{Transport: &http.Transport{}}, "https://cdn.example/m.m3u8").Transport.(*localGuard)
	tr := g.next.(*http.Transport)
	strict := context.WithValue(context.Background(), strictDial{}, true)
	if _, err := tr.DialContext(strict, "tcp", addr); !errors.Is(err, errLocalNetwork) {
		t.Fatalf("strict dial to %s: %v", addr, err)
	}
	conn, err := tr.DialContext(context.Background(), "tcp", addr)
	if err != nil {
		t.Fatalf("unmarked dial: %v", err)
	}
	conn.Close()

	// The mark set in RoundTrip arrives at the dialer.
	seen := false
	dial := tr.DialContext
	tr.DialContext = func(ctx context.Context, n, a string) (net.Conn, error) {
		seen = ctx.Value(strictDial{}) != nil
		return dial(ctx, n, addr) // a name that "resolves" to the local server
	}
	_, err = (&http.Client{Transport: g}).Get("http://rebind.example/x")
	if !seen {
		t.Fatal("the dialer did not see the strict mark")
	}
	if !errors.Is(err, errLocalNetwork) {
		t.Fatalf("rebound name: %v", err)
	}
}

func TestLocalGuardProxy(t *testing.T) {
	// Through a proxy, literals are refused, names resolving locally here
	// are refused, and names that do not resolve here go to the proxy.
	fakeDNS(t, map[string][]string{"cdn.example": {"93.184.216.34"}, "nas.example": {"10.0.0.2"}})
	proxy, _ := url.Parse("http://127.0.0.1:3128")
	tr := &http.Transport{Proxy: http.ProxyURL(proxy)}
	g := guardLocal(&http.Client{Transport: tr}, "https://cdn.example/m.m3u8").Transport.(*localGuard)
	next := &fakeNet{}
	g.next = next
	client := &http.Client{Transport: g}
	if _, err := client.Get("https://intranet-only.example/s1.ts"); err != nil {
		t.Fatalf("unresolvable name through proxy: %v", err)
	}
	if _, err := client.Get("http://10.1.2.3/x"); !errors.Is(err, errLocalNetwork) {
		t.Fatalf("literal through proxy: %v", err)
	}
	if _, err := client.Get("http://nas.example/x"); !errors.Is(err, errLocalNetwork) {
		t.Fatalf("local name through proxy: %v", err)
	}
}

func TestLiteralIP(t *testing.T) {
	cases := map[string]string{
		"127.0.0.1": "127.0.0.1", "2130706433": "127.0.0.1", "127.1": "127.0.0.1",
		"127.0.1": "127.0.0.1", "0x7f.0.0.1": "127.0.0.1", "0177.0.0.1": "127.0.0.1",
		"10.65535": "10.0.255.255", "::1": "::1",
	}
	for in, want := range cases {
		got, ok := literalIP(in)
		if !ok || got != netip.MustParseAddr(want) {
			t.Errorf("literalIP(%q) = %v %v, want %s", in, got, ok, want)
		}
	}
	for _, in := range []string{"cdn.example", "1.2.3.4.5", "256.1.1.1", "1.2.65536", "08.1.1.1", "1..2", ""} {
		if got, ok := literalIP(in); ok {
			t.Errorf("literalIP(%q) = %v, want no literal", in, got)
		}
	}
}

func TestLocalGuardProxyOnlyDNS(t *testing.T) {
	// Behind a proxy that alone resolves names, the manifest lookup fails
	// here: the stream must still work, the manifest counting as public.
	fakeDNS(t, nil)
	proxy, _ := url.Parse("http://127.0.0.1:3128")
	g := guardLocal(&http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxy)}}, "https://cdn.example/m.m3u8").Transport.(*localGuard)
	g.next = &fakeNet{}
	client := &http.Client{Transport: g}
	if _, err := client.Get("https://cdn.example/s1.ts"); err != nil {
		t.Fatalf("proxy-only DNS: %v", err)
	}
	if _, err := client.Get("http://192.168.0.1/x"); !errors.Is(err, errLocalNetwork) {
		t.Fatalf("public manifest, local literal: %v", err)
	}
}

func TestLocalGuardOtherTransport(t *testing.T) {
	// Without an *http.Transport there is no dialer to check, so names
	// are looked up in RoundTrip.
	fakeDNS(t, map[string][]string{"cdn.example": {"93.184.216.34"}, "nas.example": {"10.0.0.5"}})
	next := &fakeNet{}
	client := guardLocal(&http.Client{Transport: next}, "https://cdn.example/m.m3u8")
	if _, err := client.Get("http://nas.example/x"); !errors.Is(err, errLocalNetwork) {
		t.Fatalf("local name: %v", err)
	}
}

func TestLocalGuardOneProxyAnswer(t *testing.T) {
	// The transport uses the guard's proxy answer, even if asking the
	// proxy function again would answer differently.
	fakeDNS(t, map[string][]string{"cdn.example": {"93.184.216.34"}})
	calls := 0
	proxy, _ := url.Parse("http://127.0.0.1:3128")
	tr := &http.Transport{Proxy: func(*http.Request) (*url.URL, error) {
		calls++
		if calls == 1 {
			return nil, nil // direct for the guard...
		}
		return proxy, nil // ...a proxy if asked again
	}}
	g := guardLocal(&http.Client{Transport: tr}, "https://cdn.example/m.m3u8").Transport.(*localGuard)
	cloned := g.next.(*http.Transport)
	var seen *url.URL
	g.next = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		seen, _ = cloned.Proxy(r)
		return (&fakeNet{}).RoundTrip(r)
	})
	if _, err := (&http.Client{Transport: g}).Get("https://cdn.example/s1.ts"); err != nil {
		t.Fatal(err)
	}
	if seen != nil || calls != 1 {
		t.Fatalf("transport proxy %v after %d calls, want the guard's direct answer", seen, calls)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestAtonStrict(t *testing.T) {
	for _, in := range []string{"0b1111111.0.0.1", "0o177.0.0.1", "0x7_f.0.0.1", "1_0.0.0.1", "0x", "+1.2.3.4"} {
		if got, ok := literalIP(in); ok {
			t.Errorf("literalIP(%q) = %v, want no literal", in, got)
		}
	}
}
