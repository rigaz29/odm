package engine

import (
	"context"
	"net"
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

func randomBytes(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte((i*7 + i/251) % 256)
	}
	return b
}
