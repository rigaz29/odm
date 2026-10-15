// Package tools keeps ODM's own copies of the companion programs it runs:
// yt-dlp, which reads video sites' pages such as YouTube's, and ffmpeg,
// which joins a video track with an audio track. They are downloaded from
// their releases when first needed (once the person has agreed), checked
// against the SHA-256 checksums published with them, and kept up to date
// in a folder of ODM's, so nobody has to install them by hand.
//
// Gotchas:
//   - Nothing is run or kept unless its checksum matches the one in the
//     same release; a release without one is refused.
//   - Windows cannot replace a program that is running but can rename it,
//     so the old copy is moved aside (".old") and removed later.
//   - The folder is per user and not roaming (%LocalAppData%\odm\tools on
//     Windows): the programs are large and machine specific.
package tools

import (
	"archive/zip"
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// Dir is the folder ODM keeps the programs in: ODM_TOOLS_DIR when set,
// else odm/tools in the user's cache folder (%LocalAppData% on Windows).
func Dir() string {
	if d := os.Getenv("ODM_TOOLS_DIR"); d != "" {
		return d
	}
	base, err := os.UserCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	return filepath.Join(base, "odm", "tools")
}

// Find returns the path of program (its file name, such as "ffmpeg.exe")
// in Dir, or "" when ODM has not installed it.
func Find(program string) string {
	p := filepath.Join(Dir(), program)
	if st, err := os.Stat(p); err == nil && st.Mode().IsRegular() {
		return p
	}
	return ""
}

// Tool describes a program ODM can install from a GitHub release.
type Tool struct {
	Name string // "yt-dlp", "ffmpeg"
	Repo string // GitHub "owner/name" whose releases carry it
	// TagPrefix picks the newest release whose tag starts with it (a repo
	// that publishes other things too); "" takes the repo's latest release.
	TagPrefix string
	Asset     string // the asset to download; "" when this OS has none
	Sums      string // the asset listing SHA-256 checksums ("<hex>  <name>")
	// Files are the programs installed: the asset itself saved under
	// Files[0], or for a .zip asset, the files of these names inside it.
	Files []string
}

// Supported reports whether the tool can be installed on this system.
func (t Tool) Supported() bool { return t.Asset != "" }

func exe(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

// FFmpegRepo publishes the small ffmpeg built by scripts/ffmpeg-min.sh
// (see .github/workflows/ffmpeg-min.yml).
var FFmpegRepo = "rigaz29/odm"

// YtDlp is yt-dlp's own build for this system.
func YtDlp() Tool {
	t := Tool{Name: "yt-dlp", Repo: "yt-dlp/yt-dlp", Sums: "SHA2-256SUMS", Files: []string{exe("yt-dlp")}}
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "windows/amd64", "windows/arm64":
		t.Asset = "yt-dlp.exe"
	case "windows/386":
		t.Asset = "yt-dlp_x86.exe"
	case "darwin/amd64", "darwin/arm64":
		t.Asset = "yt-dlp_macos"
	case "linux/amd64":
		t.Asset = "yt-dlp_linux"
	case "linux/arm64":
		t.Asset = "yt-dlp_linux_aarch64"
	}
	return t
}

// FFmpeg is the small ffmpeg ODM builds; Windows only (elsewhere ffmpeg
// comes with the system's package manager).
func FFmpeg() Tool {
	t := Tool{Name: "ffmpeg", Repo: FFmpegRepo, TagPrefix: "ffmpeg-min-", Files: []string{exe("ffmpeg"), exe("ffprobe")}}
	if runtime.GOOS == "windows" && runtime.GOARCH == "amd64" {
		t.Asset = "ffmpeg-min-win64.zip"
		t.Sums = "ffmpeg-min-win64.zip.sha256"
	}
	return t
}

// All are the tools ODM keeps.
func All() []Tool { return []Tool{YtDlp(), FFmpeg()} }

// maxAsset bounds a download.
const maxAsset = 512 << 20

// Installer downloads and installs tools into Dir.
type Installer struct {
	Dir    string
	Client *http.Client
	API    string // GitHub's API, "https://api.github.com" when empty

	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

func (in *Installer) lock(name string) *sync.Mutex {
	in.mu.Lock()
	defer in.mu.Unlock()
	if in.locks == nil {
		in.locks = map[string]*sync.Mutex{}
	}
	if in.locks[name] == nil {
		in.locks[name] = &sync.Mutex{}
	}
	return in.locks[name]
}

// Installed is the release of t in Dir ("2026.08.19"), "" when none.
func (in *Installer) Installed(t Tool) string {
	data, err := os.ReadFile(filepath.Join(in.Dir, t.Name+".version"))
	if err != nil {
		return ""
	}
	for _, f := range t.Files {
		if _, err := os.Stat(filepath.Join(in.Dir, f)); err != nil {
			return "" // a file went missing: install again
		}
	}
	return strings.TrimSpace(string(data))
}

type release struct {
	Tag        string `json:"tag_name"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
	Assets     []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
		Size int64  `json:"size"`
	} `json:"assets"`
}

func (r *release) asset(name string) string {
	for _, a := range r.Assets {
		if a.Name == name {
			return a.URL
		}
	}
	return ""
}

func (in *Installer) get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	client := in.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	return resp, nil
}

// latest finds the release to install.
func (in *Installer) latest(ctx context.Context, t Tool) (*release, error) {
	api := in.API
	if api == "" {
		api = "https://api.github.com"
	}
	url := api + "/repos/" + t.Repo + "/releases/latest"
	if t.TagPrefix != "" {
		url = api + "/repos/" + t.Repo + "/releases?per_page=50"
	}
	resp, err := in.get(ctx, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if t.TagPrefix == "" {
		var r release
		if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&r); err != nil {
			return nil, err
		}
		return &r, nil
	}
	var list []release
	if err := json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(&list); err != nil {
		return nil, err
	}
	for i := range list { // newest first
		r := &list[i]
		if !r.Draft && strings.HasPrefix(r.Tag, t.TagPrefix) && r.asset(t.Asset) != "" {
			return r, nil
		}
	}
	return nil, fmt.Errorf("no %s release in %s", t.Name, t.Repo)
}

// checksum reads the SHA-256 of name from a checksum list.
func checksum(list io.Reader, name string) (string, bool) {
	sc := bufio.NewScanner(list)
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) == 2 && strings.TrimPrefix(f[1], "*") == name && len(f[0]) == 64 {
			if _, err := hex.DecodeString(f[0]); err == nil {
				return strings.ToLower(f[0]), true
			}
		}
	}
	return "", false
}

// Progress reports a download: bytes so far and the total (0 if unknown).
type Progress func(done, total int64)

// Install puts the newest release of t into Dir unless that release is
// already there, and returns its version. progress may be nil.
func (in *Installer) Install(ctx context.Context, t Tool, progress Progress) (string, error) {
	if !t.Supported() {
		return "", fmt.Errorf("ODM cannot install %s on %s/%s", t.Name, runtime.GOOS, runtime.GOARCH)
	}
	l := in.lock(t.Name)
	l.Lock()
	defer l.Unlock()

	r, err := in.latest(ctx, t)
	if err != nil {
		return "", fmt.Errorf("finding the newest %s: %w", t.Name, err)
	}
	if in.Installed(t) == r.Tag {
		return r.Tag, nil
	}
	sumsURL, assetURL := r.asset(t.Sums), r.asset(t.Asset)
	if sumsURL == "" || assetURL == "" {
		return "", fmt.Errorf("%s %s has no %s with its checksum", t.Name, r.Tag, t.Asset)
	}
	resp, err := in.get(ctx, sumsURL)
	if err != nil {
		return "", err
	}
	want, ok := checksum(io.LimitReader(resp.Body, 1<<20), t.Asset)
	resp.Body.Close()
	if !ok {
		return "", fmt.Errorf("%s %s lists no checksum for %s", t.Name, r.Tag, t.Asset)
	}

	if err := os.MkdirAll(in.Dir, 0o755); err != nil {
		return "", err
	}
	in.cleanOld()
	tmp := filepath.Join(in.Dir, ".download-"+t.Asset)
	defer os.Remove(tmp)
	if err := in.fetch(ctx, assetURL, tmp, want, progress); err != nil {
		return "", fmt.Errorf("downloading %s: %w", t.Name, err)
	}

	// The new files are put next to the old ones first, so a failure
	// halfway leaves the old version working.
	staged := map[string]string{}
	defer func() {
		for _, p := range staged {
			os.Remove(p)
		}
	}()
	if strings.HasSuffix(t.Asset, ".zip") {
		if err := unzip(tmp, in.Dir, t.Files, staged); err != nil {
			return "", err
		}
	} else {
		p := filepath.Join(in.Dir, ".new-"+t.Files[0])
		if err := os.Rename(tmp, p); err != nil {
			return "", err
		}
		staged[t.Files[0]] = p
	}
	for _, f := range t.Files {
		dst := filepath.Join(in.Dir, f)
		if err := os.Chmod(staged[f], 0o755); err != nil {
			return "", err
		}
		if _, err := os.Stat(dst); err == nil {
			// A running copy cannot be replaced on Windows, but moved.
			old := dst + ".old"
			os.Remove(old)
			if err := os.Rename(dst, old); err != nil {
				return "", fmt.Errorf("replacing %s: %w", f, err)
			}
		}
		if err := os.Rename(staged[f], dst); err != nil {
			return "", fmt.Errorf("installing %s: %w", f, err)
		}
		delete(staged, f)
	}
	if err := os.WriteFile(filepath.Join(in.Dir, t.Name+".version"), []byte(r.Tag+"\n"), 0o644); err != nil {
		return "", err
	}
	in.cleanOld()
	return r.Tag, nil
}

// cleanOld removes copies replaced earlier, once nothing runs them.
func (in *Installer) cleanOld() {
	old, _ := filepath.Glob(filepath.Join(in.Dir, "*.old"))
	for _, p := range old {
		os.Remove(p)
	}
}

// fetch downloads url to path, refusing it unless its SHA-256 is want.
func (in *Installer) fetch(ctx context.Context, url, path, want string, progress Progress) error {
	resp, err := in.get(ctx, url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.ContentLength > maxAsset {
		return fmt.Errorf("%d bytes is more than expected", resp.ContentLength)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	h := sha256.New()
	w := io.MultiWriter(f, h)
	var done int64
	buf := make([]byte, 256<<10)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				f.Close()
				return werr
			}
			done += int64(n)
			if done > maxAsset {
				f.Close()
				return errors.New("the download is larger than expected")
			}
			if progress != nil {
				progress(done, resp.ContentLength)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			f.Close()
			return rerr
		}
	}
	if err := f.Close(); err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != want {
		return fmt.Errorf("checksum mismatch: the release lists %s, the download is %s", want[:12], got[:12])
	}
	return nil
}

// unzip takes the files named in names (wherever they sit in the
// archive) out of the zip at path into dir, under a temporary name each,
// recorded in staged.
func unzip(path, dir string, names []string, staged map[string]string) error {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer zr.Close()
	for _, name := range names {
		var found *zip.File
		for _, f := range zr.File {
			if !f.FileInfo().IsDir() && strings.EqualFold(filepath.Base(filepath.FromSlash(f.Name)), name) {
				found = f
				break
			}
		}
		if found == nil {
			return fmt.Errorf("%s is not in the download", name)
		}
		rc, err := found.Open()
		if err != nil {
			return err
		}
		p := filepath.Join(dir, ".new-"+name)
		out, err := os.Create(p)
		if err == nil {
			_, err = io.Copy(out, io.LimitReader(rc, maxAsset))
			if cerr := out.Close(); err == nil {
				err = cerr
			}
		}
		rc.Close()
		staged[name] = p
		if err != nil {
			return err
		}
	}
	return nil
}
