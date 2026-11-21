package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"odm/internal/ytdlp"
)

// withRefusedCookies is a manager whose yt-dlp fails every run given the
// browser's cookies and works without them, as YouTube can refuse yt-dlp a
// signed-in session and show the same video to anyone; the extension has
// sent YouTube's cookies.
func withRefusedCookies(t *testing.T) *Manager {
	t.Helper()
	m := fakeYtDlp(t, "nocookies")
	t.Cleanup(func() {
		setYtDlpCookies("")
		forgetSiteCookies()
	})
	st := m.Settings()
	st.YtDlpCookies = CookiesFromExtension
	m.UpdateSettings(st)
	m.SetSiteCookies("https://www.youtube.com/watch?v=other", []BrowserCookie{{Name: "SID", Value: "s1", Domain: ".youtube.com", Path: "/", Secure: true}})
	return m
}

// The menu's qualities came back as an error: yt-dlp ran only with the
// cookies. It reads the page once more without them, and the site's next
// runs go without them for a while, the download too.
func TestProbeSiteWithoutRefusedCookies(t *testing.T) {
	m := withRefusedCookies(t)
	page := "https://www.youtube.com/watch?v=abc"
	c, err := m.ProbeSite(context.Background(), page)
	if err != nil || c == nil || len(c.Qualities) != 2 {
		t.Fatalf("choices %+v, error %v", c, err)
	}
	if args, done := siteCookieArgs(page); args != (ytdlp.Cookies{}) {
		done()
		t.Fatalf("the site's next run still takes the cookies: %+v", args)
	}
	forgetSiteCookies() // the Options changed: the cookies are tried again
	if cookiesResting(page) {
		t.Fatal("the rest outlived the Options' change")
	}
}

// A video page's download whose yt-dlp is refused with the cookies before
// a byte comes downloads without them.
func TestSiteDownloadWithoutRefusedCookies(t *testing.T) {
	m := withRefusedCookies(t)
	dir := t.TempDir()
	info := m.Create(AddOptions{
		URL: "https://www.youtube.com/watch?v=abc", Dir: dir, FileName: "A talk.mp4", Start: true,
		Site: &SiteJob{Format: "137+140"},
	})
	waitFor(t, "the download to finish", func() bool {
		i, _ := m.Get(info.ID)
		return i.Status == StatusComplete || i.Status == StatusError
	})
	got, _ := m.Get(info.ID)
	if got.Status != StatusComplete {
		t.Fatalf("status %s, error %q", got.Status, got.Error)
	}
	if data, err := os.ReadFile(filepath.Join(dir, got.FileName)); err != nil || string(data) != "joined video" {
		t.Fatalf("file %q: %q %v", got.FileName, data, err)
	}
}
