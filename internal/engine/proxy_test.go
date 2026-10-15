package engine

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync/atomic"
	"testing"
)

// A download added before the proxy changed used to keep the old client,
// and with it the old proxy, until ODM restarted.
func TestProxyChangeReachesExistingDownloads(t *testing.T) {
	data := randomBytes(256 << 10)
	var hits int64
	proxy := rangeServer(t, data, &hits) // answers any absolute URL it is sent
	defer proxy.Close()
	pu, _ := url.Parse(proxy.URL)
	host, portText, _ := net.SplitHostPort(pu.Host)
	port, _ := strconv.Atoi(portText)

	m := NewManager(Settings{Dir: t.TempDir(), Proxy: ProxyConfig{Mode: ProxyNone}}, "")
	// The host does not resolve, so only a request sent through the proxy
	// can complete.
	info := m.Create(AddOptions{URL: "http://odm-test.invalid/file.bin"})
	d := m.get(info.ID)

	next := m.Settings()
	next.Proxy = ProxyConfig{Mode: ProxyHTTP, Host: host, Port: port}
	m.UpdateSettings(next)
	if d.httpClient() != m.HTTPClient() {
		t.Fatal("the existing download kept the client from before the proxy change")
	}

	d.Start(context.Background())
	if got := d.Info(); got.Status != StatusComplete {
		t.Fatalf("status %s (%s)", got.Status, got.Error)
	}
	if atomic.LoadInt64(&hits) == 0 {
		t.Fatal("no request went through the new proxy")
	}
}

// Ranged connections use an HTTP/1.1 copy of the client. A copy made before
// the proxy changed must not outlive the change.
func TestProxyChangeRebuildsTheRangedClient(t *testing.T) {
	m := NewManager(Settings{Dir: t.TempDir(), Proxy: ProxyConfig{Mode: ProxyNone}}, "")
	info := m.Create(AddOptions{URL: "http://h/file.bin", Connections: 4})
	d := m.get(info.ID)
	before := d.segmentClient(true, 4)
	if before == d.httpClient() {
		t.Fatal("several connections should use the HTTP/1.1 copy")
	}

	next := m.Settings()
	next.Proxy = ProxyConfig{Mode: ProxyHTTP, Host: "127.0.0.1", Port: 3128}
	m.UpdateSettings(next)
	after := d.segmentClient(true, 4)
	if after == before {
		t.Fatal("the HTTP/1.1 copy from before the proxy change is still used")
	}
	tr := after.Transport.(*http.Transport)
	req, _ := http.NewRequest(http.MethodGet, "http://h/file.bin", nil)
	if pu, err := tr.Proxy(req); err != nil || pu == nil || pu.Host != "127.0.0.1:3128" {
		t.Fatalf("ranged client proxy = %v, %v", pu, err)
	}
	if tr.ResponseHeaderTimeout <= 0 {
		t.Fatal("the HTTP/1.1 copy lost the header timeout")
	}
}

func randomBytes(n int) []byte {
	b := randomData(n)
	return b
}
