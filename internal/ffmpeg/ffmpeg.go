// Package ffmpeg finds an ffmpeg binary and uses it to join a video stream
// and an audio stream into one file without re-encoding.
//
// ODM does not ship ffmpeg; it is optional and only needed for formats that
// come as separate video and audio streams (most adaptive HLS/DASH sources).
//
// Gotchas:
//   - ODM on Windows is a GUI program; a console child flashes a window
//     unless it is started hidden (see hide in ffmpeg_windows.go).
//   - The output is written under a temporary name and renamed when ffmpeg
//     succeeds, so a failed or cancelled merge never leaves a half file
//     with the final name. The muxer is passed with -f because the
//     temporary name has no extension ffmpeg recognises.
//   - "-c copy" only works when the codecs fit the container (H.264/AV1 +
//     AAC in MP4, VP9 + Opus in WebM). The container follows the output
//     name (see muxer); there is no retry in another one, so a pair that
//     does not fit fails with ffmpeg's error. Stream downloads ask for
//     WebM when both tracks are WebM and MP4 otherwise; Matroska is only
//     used for an output name that is neither.
package ffmpeg

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"odm/internal/tools"
)

// ErrNotFound means no ffmpeg was configured, bundled or on PATH.
var ErrNotFound = errors.New("ffmpeg not found: install it or set its path in Options")

func exeName() string {
	if runtime.GOOS == "windows" {
		return "ffmpeg.exe"
	}
	return "ffmpeg"
}

// Locate returns the ffmpeg to use, in order: the configured path (a file,
// or a folder containing ffmpeg), the copy ODM installed (tools.Dir), one
// next to the ODM executable, then PATH.
func Locate(configured string) (string, error) {
	if configured = strings.TrimSpace(configured); configured != "" {
		if st, err := os.Stat(configured); err == nil && st.IsDir() {
			configured = filepath.Join(configured, exeName())
		}
		if st, err := os.Stat(configured); err == nil && !st.IsDir() {
			return configured, nil
		}
		return "", fmt.Errorf("ffmpeg not found at %s", configured)
	}
	if p := tools.Find(exeName()); p != "" {
		return p, nil
	}
	if self, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(self), exeName())
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	if p, err := exec.LookPath("ffmpeg"); err == nil {
		return p, nil
	}
	return "", ErrNotFound
}

// Version returns the first line of "ffmpeg -version", for Options.
func Version(ctx context.Context, bin string) (string, error) {
	cmd := exec.CommandContext(ctx, bin, "-version")
	hide(cmd)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	line, _, _ := strings.Cut(string(out), "\n")
	return strings.TrimSpace(line), nil
}

// muxer maps the output extension to ffmpeg's muxer name.
func muxer(out string) string {
	switch strings.ToLower(filepath.Ext(out)) {
	case ".mp4", ".m4a", ".m4v", ".mov":
		return "mp4"
	case ".webm":
		return "webm"
	}
	return "matroska"
}

func mergeArgs(video, audio, tmp, format string) []string {
	return []string{
		"-hide_banner", "-loglevel", "error", "-nostdin", "-y",
		"-i", video, "-i", audio,
		"-map", "0:v:0", "-map", "1:a:0",
		"-c", "copy",
		"-f", format, tmp,
	}
}

// Merge joins video and audio into out. The inputs are left in place; the
// caller removes them once it no longer needs them for a retry.
func Merge(ctx context.Context, bin, video, audio, out string) error {
	return run(ctx, bin, out, func(tmp string) []string {
		return mergeArgs(video, audio, tmp, muxer(out))
	})
}

func remuxArgs(in, tmp, format string) []string {
	return []string{
		"-hide_banner", "-loglevel", "error", "-nostdin", "-y",
		"-i", in,
		// Video and audio only: MP4 has no place for the ID3 data streams
		// some MPEG-TS recordings carry.
		"-map", "0:v?", "-map", "0:a?",
		"-c", "copy",
		"-f", format, tmp,
	}
}

// Remux copies the video and audio of in into out, in the container named
// by format ("mp4", "matroska"), without re-encoding. An MPEG-TS recording
// becomes MP4 so: ffmpeg rewraps its AAC (aac_adtstoasc) and smooths the
// timestamp jumps a TS stream may have. in is left in place.
func Remux(ctx context.Context, bin, in, out, format string) error {
	return run(ctx, bin, out, func(tmp string) []string { return remuxArgs(in, tmp, format) })
}

// Concat joins the parts (pieces of one recording, each with its own
// header, as a live stream reconnected) into out, in the container named by
// format, copying their video and audio. ffmpeg's concat demuxer moves each
// part's timestamps on after the one before.
func Concat(ctx context.Context, bin string, parts []string, out, format string) error {
	list := out + ".list.txt"
	var b strings.Builder
	for _, p := range parts {
		// The list's own quoting: a ' is closed, escaped and reopened.
		b.WriteString("file '" + strings.ReplaceAll(filepath.ToSlash(p), "'", `'\''`) + "'\n")
	}
	if err := os.WriteFile(list, []byte(b.String()), 0o644); err != nil {
		return err
	}
	defer os.Remove(list)
	return run(ctx, bin, out, func(tmp string) []string {
		return []string{
			"-hide_banner", "-loglevel", "error", "-nostdin", "-y",
			"-f", "concat", "-safe", "0", "-i", list,
			"-map", "0:v?", "-map", "0:a?",
			"-c", "copy",
			"-f", format, tmp,
		}
	})
}

// run runs ffmpeg with the arguments args gives for a temporary output,
// and renames that to out when ffmpeg succeeds.
func run(ctx context.Context, bin, out string, args func(tmp string) []string) error {
	tmp := out + ".merging"
	cmd := exec.CommandContext(ctx, bin, args(tmp)...)
	hide(cmd)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		_ = os.Remove(tmp)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		msg := strings.TrimSpace(stderr.String())
		if len(msg) > 300 { // keep the end: that is where ffmpeg says why
			msg = "…" + msg[len(msg)-300:]
		}
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("merge failed: %s", msg)
	}
	if err := os.Rename(tmp, out); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
