package engine

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSiteKey(t *testing.T) {
	for page, want := range map[string]string{
		"https://www.youtube.com/watch?v=abc":   "youtube.com",
		"https://music.youtube.com/watch?v=abc": "youtube.com",
		"https://youtu.be/abc":                  "youtube.com",
		"https://m.tiktok.com/@a/video/1":       "tiktok.com",
		"https://www.tiktok.com/@a/live":        "tiktok.com",
		"https://vimeo.com/123":                 "vimeo.com",
		"not an address":                        "",
	} {
		if got := siteKey(page); got != want {
			t.Errorf("siteKey(%q) = %q, want %q", page, got, want)
		}
	}
}

func TestWriteCookieFile(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	file, err := writeCookieFile([]BrowserCookie{
		{Name: "SID", Value: "s1", Domain: ".youtube.com", Path: "/", Secure: true, HTTPOnly: true, Expires: 1_900_000_000},
		{Name: "PREF", Value: "f6=40", Domain: "www.youtube.com", Path: "/", HostOnly: true},
		{Name: "bad", Value: "a\tb", Domain: ".youtube.com"},
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(filepath.Dir(file))
	data, _ := os.ReadFile(file)
	want := "# Netscape HTTP Cookie File\n" +
		"#HttpOnly_.youtube.com\tTRUE\t/\tTRUE\t1900000000\tSID\ts1\n" +
		"www.youtube.com\tFALSE\t/\tFALSE\t1800086400\tPREF\tf6=40\n"
	if string(data) != want {
		t.Fatalf("cookie file:\n%s\nwant:\n%s", data, want)
	}
}

// The extension's cookies are kept only while the Options say to sign in
// with them, per site, and yt-dlp gets them in a file removed after.
func TestSiteCookiesFollowTheOptions(t *testing.T) {
	m := NewManager(Settings{Dir: t.TempDir()}, "")
	t.Cleanup(func() {
		setYtDlpCookies("")
		forgetSiteCookies()
	})
	page := "https://www.youtube.com/watch?v=abc"
	cookies := []BrowserCookie{{Name: "SID", Value: "s1", Domain: ".youtube.com", Path: "/", Secure: true}}
	st := m.Settings()
	if st.YtDlpCookies != CookiesFromExtension {
		t.Fatalf("the default is %q", st.YtDlpCookies)
	}

	st.YtDlpCookies = CookiesNone
	m.UpdateSettings(st)
	m.SetSiteCookies(page, cookies) // not asked for: dropped
	if c, _ := siteCookieArgs(page); c.File != "" || c.Browser != "" {
		t.Fatalf("cookies with none chosen: %+v", c)
	}

	st.YtDlpCookies = CookiesFromExtension
	m.UpdateSettings(st)
	m.SetSiteCookies(page, cookies)
	c, done := siteCookieArgs("https://youtu.be/abc")
	if c.File == "" {
		t.Fatal("no cookie file for the site")
	}
	if data, _ := os.ReadFile(c.File); !strings.Contains(string(data), "\tSID\ts1\n") {
		t.Fatalf("cookie file:\n%s", data)
	}
	done()
	if _, err := os.Stat(c.File); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("the cookie file was left behind")
	}
	if c, _ := siteCookieArgs("https://www.tiktok.com/@a/video/1"); c.File != "" {
		t.Fatal("another site got YouTube's cookies")
	}

	st.YtDlpCookies = "firefox"
	m.UpdateSettings(st)
	if c, _ := siteCookieArgs(page); c.Browser != "firefox" || c.File != "" {
		t.Fatalf("with a browser chosen: %+v", c)
	}
	st.YtDlpCookies = CookiesFromExtension
	m.UpdateSettings(st)
	if c, _ := siteCookieArgs(page); c.File != "" {
		t.Fatal("the extension's cookies outlived the switch to a browser")
	}
}
