// Package quarantine marks a downloaded file as coming from the internet,
// as browsers do, so the system treats it with the care such a file needs:
// on Windows, SmartScreen checks a program before it runs and Office opens a
// document in Protected View.
package quarantine

import (
	"net"
	"net/url"
	"strings"
)

// sourceURL is raw without user name and password, which must not be
// written next to the file, or "about:internet" when raw is no URL.
func sourceURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "about:internet"
	}
	u.User = nil
	return u.String()
}

// local reports whether raw names this computer or a private network: a
// file from there is not marked as coming from the internet.
func local(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast())
}
