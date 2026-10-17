//go:build fyne

package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"fyne.io/fyne/v2"
)

// Hang detection, for "Not Responding" reports that cannot be reproduced
// in tests. Every hangPing the watchdog asks the UI thread to run an empty
// function; if that has not run after hangAfter, it writes the stacks of
// all goroutines to odm-hang-<time>.txt in the config folder, which shows
// where the UI thread is stuck. One file per hang: it waits for the UI to
// answer again before checking further.
const (
	hangPing  = 2 * time.Second
	hangAfter = 5 * time.Second
)

func (a *application) watchHangs(ctx context.Context) {
	dir := a.mgr.ConfigDir()
	if dir == "" {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(hangPing):
		}
		answered := make(chan struct{})
		fyne.Do(func() { close(answered) })
		select {
		case <-answered:
			continue
		case <-ctx.Done():
			return
		case <-time.After(hangAfter):
		}
		buf := make([]byte, 1<<20)
		buf = buf[:runtime.Stack(buf, true)]
		name := filepath.Join(dir, "odm-hang-"+time.Now().Format("20060102-150405")+".txt")
		head := fmt.Sprintf("ODM UI thread did not answer for %v at %s\n\n", hangAfter, time.Now().Format(time.RFC3339))
		_ = os.WriteFile(name, append([]byte(head), buf...), 0o644)
		select {
		case <-answered:
		case <-ctx.Done():
			return
		}
	}
}
