package engine

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestClientBoundsNetworkWaits(t *testing.T) {
	for _, mode := range []string{ProxyNone, ProxySystem, ProxyHTTP, ProxySOCKS5} {
		s := DefaultSettings(t.TempDir())
		s.Proxy = ProxyConfig{Mode: mode, Host: "127.0.0.1", Port: 1}
		tr := buildClient(s).Transport.(*http.Transport)
		if tr.DialContext == nil || tr.TLSHandshakeTimeout <= 0 || tr.ResponseHeaderTimeout <= 0 {
			t.Fatalf("%s: dial %v, TLS %v, headers %v", mode, tr.DialContext != nil, tr.TLSHandshakeTimeout, tr.ResponseHeaderTimeout)
		}
	}
}

// A server that accepts the connection and never answers used to keep the
// probe waiting forever.
func TestProbeGivesUpOnSilentServer(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			defer c.Close() // hold it open, say nothing
		}
	}()

	old := headerTimeout
	headerTimeout = 200 * time.Millisecond
	defer func() { headerTimeout = old }()
	client := buildClient(Settings{Proxy: ProxyConfig{Mode: ProxyNone}})

	done := make(chan error, 1)
	go func() {
		_, err := Inspect(context.Background(), client, "http://"+ln.Addr().String()+"/f.bin", nil)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("probe of a silent server succeeded")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("probe of a silent server still waiting after 5s")
	}
}
