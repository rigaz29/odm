package engine

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
)

// GrabberOptions configures a site crawl.
type GrabberOptions struct {
	URL         string
	Depth       int
	MaxPages    int
	MaxFiles    int
	SameHost    bool
	Dir         string
	Connections int
	Start       bool
}

// GrabberResult reports what a crawl found.
type GrabberResult struct {
	Pages int
	Files []string
}

var hrefRe = regexp.MustCompile(`(?i)href\s*=\s*["']([^"'>]+)["']`)

// Grab walks a site breadth-first and queues links that look like files.
// progress, when non-nil, is called after each page.
func (m *Manager) Grab(ctx context.Context, o GrabberOptions, progress func(pages, files int)) (GrabberResult, error) {
	if o.MaxPages <= 0 {
		o.MaxPages = 100
	}
	if o.MaxFiles <= 0 {
		o.MaxFiles = 200
	}
	if o.Depth <= 0 {
		o.Depth = 1
	}
	start, err := url.Parse(strings.TrimSpace(o.URL))
	if err != nil || start.Hostname() == "" {
		return GrabberResult{}, fmt.Errorf("invalid start URL")
	}
	fileTypes := m.Settings().FileTypes

	type item struct {
		u     *url.URL
		depth int
	}
	seen := map[string]bool{normalizeURL(start): true}
	queue := []item{{u: start, depth: 0}}
	res := GrabberResult{}

	for len(queue) > 0 && res.Pages < o.MaxPages && len(res.Files) < o.MaxFiles {
		if ctx.Err() != nil {
			break
		}
		cur := queue[0]
		queue = queue[1:]

		body, err := m.fetchPage(ctx, cur.u.String())
		if err != nil {
			continue
		}
		res.Pages++

		for _, href := range extractHrefs(body) {
			ref, err := cur.u.Parse(strings.TrimSpace(href))
			if err != nil || (ref.Scheme != "http" && ref.Scheme != "https") {
				continue
			}
			key := normalizeURL(ref)
			if seen[key] {
				continue
			}
			seen[key] = true

			if KnownFileType(path.Base(ref.Path), fileTypes...) {
				if len(res.Files) >= o.MaxFiles {
					break
				}
				res.Files = append(res.Files, ref.String())
				m.AddAuto(AddOptions{
					URL:         ref.String(),
					Dir:         o.Dir,
					Connections: o.Connections,
					Start:       o.Start,
				})
				continue
			}
			if o.SameHost && !strings.EqualFold(ref.Hostname(), start.Hostname()) {
				continue
			}
			if cur.depth+1 > o.Depth {
				continue
			}
			queue = append(queue, item{u: ref, depth: cur.depth + 1})
		}
		if progress != nil {
			progress(res.Pages, len(res.Files))
		}
	}
	return res, nil
}

func (m *Manager) fetchPage(ctx context.Context, raw string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", currentUserAgent())
	resp, err := m.authz.do(req, m.HTTPClient().Do)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// extractHrefs returns every href value in a page.
func extractHrefs(html string) []string {
	matches := hrefRe.FindAllStringSubmatch(html, -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) > 1 {
			out = append(out, m[1])
		}
	}
	return out
}

// normalizeURL drops the fragment so the same page is not fetched twice.
func normalizeURL(u *url.URL) string {
	c := *u
	c.Fragment = ""
	return c.String()
}
