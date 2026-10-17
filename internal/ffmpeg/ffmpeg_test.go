package ffmpeg

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestMain lets the test binary stand in for ffmpeg: with FAKE_FFMPEG set
// it behaves like a tiny ffmpeg instead of running the tests.
func TestMain(m *testing.M) {
	switch os.Getenv("FAKE_FFMPEG") {
	case "":
		os.Exit(m.Run())
	case "ok":
		args := os.Args[1:]
		if len(args) == 1 && args[0] == "-version" {
			os.Stdout.WriteString("ffmpeg version 7.1-fake Copyright\nbuilt with gcc\n")
			os.Exit(0)
		}
		// Concatenate the two inputs into the last argument, so the test
		// can see which files were used.
		var in []string
		for i, a := range args {
			if a == "-i" {
				in = append(in, args[i+1])
			}
		}
		var data []byte
		for _, p := range in {
			b, _ := os.ReadFile(p)
			data = append(data, b...)
		}
		_ = os.WriteFile(args[len(args)-1], data, 0o644)
		os.Exit(0)
	case "fail":
		os.WriteFile(os.Args[len(os.Args)-1], []byte("partial"), 0o644)
		os.Stderr.WriteString("Could not find tag for codec vp9 in stream #0\n")
		os.Exit(1)
	case "hang":
		time.Sleep(time.Minute)
	}
}

// fake copies the test binary to dir under ffmpeg's name.
func fake(t *testing.T, dir, mode string) string {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, exeName())
	if err := os.WriteFile(p, data, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FAKE_FFMPEG", mode)
	return p
}

func TestLocate(t *testing.T) {
	t.Setenv("ODM_TOOLS_DIR", t.TempDir()) // not the copy ODM keeps for this user
	dir := t.TempDir()
	bin := fake(t, dir, "ok")
	if got, err := Locate(bin); err != nil || got != bin {
		t.Fatalf("file: %q %v", got, err)
	}
	if got, err := Locate(dir); err != nil || got != bin {
		t.Fatalf("folder: %q %v", got, err)
	}
	if _, err := Locate(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("missing path accepted")
	}
	t.Setenv("PATH", dir)
	if got, err := Locate(""); err != nil || got == "" {
		t.Fatalf("PATH: %q %v", got, err)
	}
	t.Setenv("PATH", t.TempDir())
	if _, err := Locate(""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("empty PATH: %v", err)
	}
}

func TestVersion(t *testing.T) {
	bin := fake(t, t.TempDir(), "ok")
	v, err := Version(context.Background(), bin)
	if err != nil || v != "ffmpeg version 7.1-fake Copyright" {
		t.Fatalf("%q %v", v, err)
	}
}

func TestMergeArgs(t *testing.T) {
	got := strings.Join(mergeArgs("v.webm", "a.webm", "out.webm.merging", "webm"), " ")
	want := "-hide_banner -loglevel error -nostdin -y -i v.webm -i a.webm -map 0:v:0 -map 1:a:0 -c copy -f webm out.webm.merging"
	if got != want {
		t.Fatalf("args\n got %s\nwant %s", got, want)
	}
	for out, m := range map[string]string{"a.mp4": "mp4", "a.M4A": "mp4", "a.webm": "webm", "a.mkv": "matroska", "a": "matroska"} {
		if muxer(out) != m {
			t.Errorf("muxer(%q) = %q, want %q", out, muxer(out), m)
		}
	}
}

func TestMerge(t *testing.T) {
	dir := t.TempDir()
	video, audio := filepath.Join(dir, "v"), filepath.Join(dir, "a")
	os.WriteFile(video, []byte("VIDEO"), 0o644)
	os.WriteFile(audio, []byte("AUDIO"), 0o644)
	out := filepath.Join(dir, "clip.mp4")

	bin := fake(t, t.TempDir(), "ok")
	if err := Merge(context.Background(), bin, video, audio, out); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(out); string(b) != "VIDEOAUDIO" {
		t.Fatalf("output %q", b)
	}
	if _, err := os.Stat(out + ".merging"); !os.IsNotExist(err) {
		t.Fatal("temporary file left behind")
	}

	// A failure reports ffmpeg's message and leaves no output.
	os.Remove(out)
	bin = fake(t, t.TempDir(), "fail")
	err := Merge(context.Background(), bin, video, audio, out)
	if err == nil || !strings.Contains(err.Error(), "Could not find tag") {
		t.Fatalf("failure: %v", err)
	}
	for _, p := range []string{out, out + ".merging"} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("%s left behind", p)
		}
	}

	// Cancelling stops ffmpeg and reports the cancellation.
	bin = fake(t, t.TempDir(), "hang")
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if err := Merge(ctx, bin, video, audio, out); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cancel: %v", err)
	}
}

// TestRealFFmpeg merges two generated streams when a real ffmpeg is on PATH.
func TestRealFFmpeg(t *testing.T) {
	bin, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("no ffmpeg on PATH")
	}
	dir := t.TempDir()
	video, audio := filepath.Join(dir, "v.mp4"), filepath.Join(dir, "a.m4a")
	gen := func(args ...string) {
		if out, err := exec.Command(bin, append([]string{"-hide_banner", "-loglevel", "error", "-y"}, args...)...).CombinedOutput(); err != nil {
			t.Skipf("cannot generate test input: %v %s", err, out)
		}
	}
	gen("-f", "lavfi", "-i", "testsrc=duration=1:size=160x120:rate=10", "-c:v", "libx264", video)
	gen("-f", "lavfi", "-i", "sine=duration=1", "-c:a", "aac", audio)
	out := filepath.Join(dir, "clip.mp4")
	if err := Merge(context.Background(), bin, video, audio, out); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		probe, err := exec.Command(bin, "-hide_banner", "-i", out).CombinedOutput()
		if s := string(probe); !strings.Contains(s, "Video: h264") || !strings.Contains(s, "Audio: aac") {
			t.Fatalf("merged file: %v\n%s", err, s)
		}
	}
}

// Parts of a live FLV whose timestamps run on from the broadcast's start
// (TikTok's), written as a live server sends them (no duration in the
// file), are joined back to back: joined as they came, each was moved on
// by its first timestamp, and three seconds of picture lasted over an hour.
func TestRealConcatLiveParts(t *testing.T) {
	bin, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("no ffmpeg on PATH")
	}
	dir := t.TempDir()
	var parts []string
	for i, at := range []string{"1500", "3100", "3100"} {
		p := filepath.Join(dir, fmt.Sprintf("p%d.flv", i+1))
		f, err := os.Create(p)
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(bin, "-hide_banner", "-loglevel", "error",
			"-f", "lavfi", "-i", "testsrc=duration=1:size=160x120:rate=10",
			"-f", "lavfi", "-i", "sine=duration=1",
			"-c:v", "libx264", "-g", "10", "-c:a", "aac", "-output_ts_offset", at, "-f", "flv", "pipe:1")
		cmd.Stdout = f // a pipe: no duration written, as from a live server
		err = cmd.Run()
		f.Close()
		if err != nil {
			t.Skipf("cannot generate test input: %v", err)
		}
		parts = append(parts, p)
	}
	out := filepath.Join(dir, "joined.mp4")
	if err := Concat(context.Background(), bin, parts, out, "mp4"); err != nil {
		t.Fatal(err)
	}
	probe, _ := exec.Command(bin, "-hide_banner", "-i", out).CombinedOutput()
	m := regexp.MustCompile(`Duration: (\d+):(\d+):(\d+\.\d+)`).FindStringSubmatch(string(probe))
	if m == nil {
		t.Fatalf("no duration in:\n%s", probe)
	}
	h, _ := strconv.Atoi(m[1])
	mins, _ := strconv.Atoi(m[2])
	secs, _ := strconv.ParseFloat(m[3], 64)
	if total := float64(h*3600+mins*60) + secs; total < 2.5 || total > 4 {
		t.Fatalf("three one-second parts last %.1fs", total)
	}
	if left, _ := filepath.Glob(filepath.Join(dir, "*.mkv")); len(left) != 0 {
		t.Fatalf("copies left behind: %v", left)
	}
}
