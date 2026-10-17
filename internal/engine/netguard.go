package engine

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/net/idna"
)

// errLocalNetwork refuses a request that a public address (a stream's
// playlist, a redirect) made toward the user's own machine or network.
// Retrying cannot change the answer.
var errLocalNetwork = errors.New("a public address led into the local network")

// localGuard keeps a playlist from a public server away from the user's
// machine and network. File downloads go through it too: a public link
// that redirects to a router or localhost is refused the same way.
//
// A playlist names its segments, keys and variants by URL, on any host.
// ODM fetches them from the user's computer, which sits behind the
// firewall and next to routers, NAS boxes and services on localhost that
// a web page cannot reach itself (browsers block public pages from private
// addresses). So when the manifest is on a public address, every request
// of the stream, redirects included, must go to a public address as well.
// A manifest that is itself on the local network (a home media server, an
// intranet) is trusted with local addresses: whoever wrote it is already
// inside.
//
// Where the check happens:
//   - Direct connections are checked in the dialer, on the address
//     actually being connected to. A lookup of our own followed by the
//     transport's would be two answers, and a server that changes its
//     answer in between (DNS rebinding) would pass.
//   - Through a proxy the dialer only sees the proxy, which is often local
//     itself, and the proxy resolves the name. There RoundTrip refuses
//     address literals and names that resolve locally here; a name that
//     does not resolve here is left to the proxy (some networks only
//     resolve through it).
//
// Gotchas:
//   - Trusting the manifest takes every address of its host being local;
//     a host with one public and one local address is public. The reverse
//     rule ("any local answer") would let an attacker switch the guard off
//     by publishing one local record next to the public one.
//   - Hosts are compared in the form the transport sends (lower case, no
//     trailing dot, IDNA-mapped, so "１２７.０.０.１" is 127.0.0.1), and the
//     legacy IPv4 spellings resolvers accept ("2130706433", "127.1",
//     "0x7f.0.0.1") count as literals.
//   - The manifest is judged by the URL ODM was given, not where it
//     redirects to.
type localGuard struct {
	next  http.RoundTripper
	proxy func(*http.Request) (*url.URL, error) // nil: never a proxy
	// checkNames: no dialer of ours sees direct connections (next is not
	// an *http.Transport), so names are looked up here, as for a proxy.
	checkNames bool
	manifest   string // canonical host of the manifest
	lookup     func(ctx context.Context, host string) ([]net.IPAddr, error)

	mu     sync.Mutex
	public *bool // verdict on the manifest host, once known
}

// strictDial marks a request context whose direct connections must not
// reach a local address; the guard's dialer reads it.
type strictDial struct{}

// proxyChoice carries the guard's answer from the proxy function to the
// transport, so both act on the same one. The system proxy is re-read
// periodically; asked twice, it could say "direct" to the guard and
// "proxy" to the transport, or the reverse.
type proxyChoice struct{ u *url.URL }

// lookupIPAddr resolves names for the guard; tests replace it.
var lookupIPAddr = net.DefaultResolver.LookupIPAddr

// guardLocal wraps client for a stream whose manifest is at manifestURL.
// An *http.Transport is cloned, so the guarded connections live in a pool
// of their own instead of being shared with unguarded downloads.
func guardLocal(client *http.Client, manifestURL string) *http.Client {
	if client == nil {
		client = http.DefaultClient
	}
	next := client.Transport
	if next == nil {
		next = http.DefaultTransport
	}
	manifest := ""
	if u, err := url.Parse(manifestURL); err == nil {
		manifest = canonicalHost(u.Hostname())
	}
	g := &localGuard{manifest: manifest, lookup: lookupIPAddr}
	if t, ok := next.(*http.Transport); ok {
		t = t.Clone()
		if t.Proxy != nil {
			g.proxy = t.Proxy
			t.Proxy = func(r *http.Request) (*url.URL, error) {
				if c, ok := r.Context().Value(proxyChoice{}).(proxyChoice); ok {
					return c.u, nil
				}
				return g.proxy(r)
			}
		}
		base := t.DialContext
		if base == nil {
			base = (&net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}).DialContext
		}
		strict := &net.Dialer{
			Timeout: 30 * time.Second, KeepAlive: 30 * time.Second,
			// Control sees each resolved address before connecting, so a
			// name with several answers skips the local ones.
			Control: func(_, address string, _ syscall.RawConn) error {
				ap, err := netip.ParseAddrPort(address)
				if err != nil {
					return fmt.Errorf("%w (%s)", errLocalNetwork, address)
				}
				if localAddr(ap.Addr()) {
					return fmt.Errorf("%w (%s)", errLocalNetwork, ap.Addr())
				}
				return nil
			},
		}
		t.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			if ctx.Value(strictDial{}) != nil {
				return strict.DialContext(ctx, network, addr)
			}
			return base(ctx, network, addr)
		}
		next = t
	} else {
		g.checkNames = true
	}
	g.next = next
	c := *client
	c.Transport = g
	return &c
}

func (g *localGuard) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	viaProxy := false
	if g.proxy != nil {
		u, err := g.proxy(req)
		if err != nil {
			return nil, err
		}
		viaProxy = u != nil
		ctx = context.WithValue(ctx, proxyChoice{u}, proxyChoice{u})
		req = req.WithContext(ctx)
	}
	public, err := g.manifestPublic(ctx, viaProxy)
	if err != nil {
		return nil, err
	}
	if !public {
		return g.next.RoundTrip(req)
	}
	host := canonicalHost(req.URL.Hostname())
	refuse := func() (*http.Response, error) {
		return nil, fmt.Errorf("%w (%s); ODM follows such links only from an address on the local network", errLocalNetwork, host)
	}
	if localName(host) {
		return refuse()
	}
	if ip, ok := literalIP(host); ok {
		if localAddr(ip) {
			return refuse()
		}
	} else if viaProxy || g.checkNames {
		// Best effort: the proxy resolves the name, maybe differently.
		if addrs, err := g.lookup(ctx, host); err == nil {
			for _, a := range addrs {
				if ip, ok := netip.AddrFromSlice(a.IP); ok && localAddr(ip) {
					return refuse()
				}
			}
		}
	}
	if !viaProxy {
		req = req.WithContext(context.WithValue(ctx, strictDial{}, true))
	}
	return g.next.RoundTrip(req)
}

// manifestPublic decides once whether the manifest's host is public: it is
// local only when every address it has is local. Through a proxy a name
// that does not resolve here counts as public (the strict side): only the
// proxy may be able to resolve it.
func (g *localGuard) manifestPublic(ctx context.Context, viaProxy bool) (bool, error) {
	g.mu.Lock()
	if g.public != nil {
		v := *g.public
		g.mu.Unlock()
		return v, nil
	}
	g.mu.Unlock()
	local, err := g.allLocal(ctx, g.manifest)
	if err != nil {
		if !viaProxy {
			return false, err
		}
		local = false
	}
	public := !local
	g.mu.Lock()
	g.public = &public
	g.mu.Unlock()
	return public, nil
}

func (g *localGuard) allLocal(ctx context.Context, host string) (bool, error) {
	if localName(host) {
		return true, nil
	}
	if ip, ok := literalIP(host); ok {
		return localAddr(ip), nil
	}
	addrs, err := g.lookup(ctx, host)
	if err != nil {
		return false, err
	}
	for _, a := range addrs {
		ip, ok := netip.AddrFromSlice(a.IP)
		if !ok || !localAddr(ip) {
			return false, nil
		}
	}
	return len(addrs) > 0, nil
}

// canonicalHost is host as the transport will send it.
func canonicalHost(h string) string {
	h = strings.TrimSuffix(strings.ToLower(h), ".")
	if a, err := idna.Lookup.ToASCII(h); err == nil {
		h = strings.TrimSuffix(a, ".")
	}
	return h
}

func localName(host string) bool {
	return host == "localhost" || strings.HasSuffix(host, ".localhost")
}

// literalIP parses an IP address literal, including the legacy IPv4
// forms inet_aton accepts: 1 to 4 parts, each decimal, octal (leading 0)
// or hex (0x), the last part filling the remaining bytes.
func literalIP(host string) (netip.Addr, bool) {
	if a, err := netip.ParseAddr(host); err == nil {
		return a.WithZone(""), true
	}
	parts := strings.Split(host, ".")
	if len(parts) > 4 {
		return netip.Addr{}, false
	}
	vals := make([]uint64, len(parts))
	for i, p := range parts {
		if p == "" {
			return netip.Addr{}, false
		}
		v, ok := atonPart(p)
		if !ok {
			return netip.Addr{}, false
		}
		vals[i] = v
	}
	var n uint64
	for i, v := range vals[:len(vals)-1] {
		if v > 0xff {
			return netip.Addr{}, false
		}
		n |= v << (24 - 8*i)
	}
	// The last part fills the bytes the others left: 32 bits alone, 24
	// after one part, 16 after two, 8 after three.
	last := vals[len(vals)-1]
	if last >= 1<<(8*(5-len(vals))) && len(vals) > 1 {
		return netip.Addr{}, false
	}
	n |= last
	return netip.AddrFrom4([4]byte{byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n)}), true
}

// atonPart parses one part as inet_aton does: "0x" hex, leading-0 octal,
// else decimal; nothing else (strconv would also take 0b, 0o and "_").
func atonPart(p string) (uint64, bool) {
	base := 10
	switch {
	case len(p) > 2 && (p[:2] == "0x" || p[:2] == "0X"):
		base, p = 16, p[2:]
	case len(p) > 1 && p[0] == '0':
		base, p = 8, p[1:]
	}
	for _, c := range p {
		if !(c >= '0' && c <= '9' || base == 16 && (c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F')) {
			return 0, false
		}
	}
	v, err := strconv.ParseUint(p, base, 32)
	return v, err == nil
}

// localRanges are addresses that are not on the public internet.
var localRanges = func() []netip.Prefix {
	var out []netip.Prefix
	for _, s := range []string{
		"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8",
		"169.254.0.0/16", "172.16.0.0/12", "192.0.0.0/24", "192.168.0.0/16",
		// 198.18.0.0/15 is also where fake-IP proxies (Clash, sing-box)
		// answer every name; with one, every manifest looks local and the
		// guard stays off. It cannot see the real address there anyway.
		"198.18.0.0/15", "224.0.0.0/4", "240.0.0.0/4",
		"::/128", "::1/128", "fc00::/7", "fe80::/10", "fec0::/10", "ff00::/8",
		"64:ff9b:1::/48", // NAT64 for local use
	} {
		out = append(out, netip.MustParsePrefix(s))
	}
	return out
}()

var nat64 = netip.MustParsePrefix("64:ff9b::/96")

// localAddr reports whether ip is not a public address, looking through
// IPv6 forms that carry an IPv4 address (mapped, compatible, NAT64, 6to4).
func localAddr(ip netip.Addr) bool {
	ip = ip.WithZone("").Unmap()
	if ip.Is6() {
		b := ip.As16()
		var v4 [4]byte
		switch {
		case ip == netip.IPv6Unspecified() || ip == netip.IPv6Loopback():
			// in localRanges
		case [12]byte(b[:12]) == [12]byte{}: // ::a.b.c.d (deprecated)
			copy(v4[:], b[12:])
			return localAddr(netip.AddrFrom4(v4))
		case nat64.Contains(ip):
			copy(v4[:], b[12:])
			return localAddr(netip.AddrFrom4(v4))
		case b[0] == 0x20 && b[1] == 0x02: // 2002::/16, 6to4
			copy(v4[:], b[2:6])
			return localAddr(netip.AddrFrom4(v4))
		}
	}
	for _, p := range localRanges {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}
