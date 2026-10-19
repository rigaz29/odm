//go:build gui

package gui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"odm/internal/tools"
)

func TestToolQuestions(t *testing.T) {
	if !tools.Libtorrent().Supported() || !tools.YtDlp().Supported() {
		t.Skip("no companion programs for this system")
	}
	svc, _ := newTestService(t)
	w := &stubWindows{}
	svc.windows = w
	mode := tools.ModeAsk
	k := tools.NewKeeper(&tools.Installer{Dir: t.TempDir()}, func() string { return mode })
	(&desk{svc: svc}).hookTools(k)
	wait := func(want string) {
		t.Helper()
		for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
			if strings.Contains(w.got(), want) {
				return
			}
		}
		t.Fatalf("asked %q, not %q", w.got(), want)
	}
	// yt-dlp and ffmpeg share a question; a torrent asks about
	// odm-libtorrent, even after yt-dlp and ffmpeg were declined.
	k.Ensure(context.Background(), "yt-dlp")
	wait("notice kind=tools")
	mode = tools.ModeOff
	if _, err := k.Ensure(context.Background(), "odm-libtorrent"); !errors.Is(err, tools.ErrNeedsInstall) {
		t.Fatalf("odm-libtorrent: %v", err)
	}
	wait("notice kind=program&name=odm-libtorrent")
}
