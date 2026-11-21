package ipc

import (
	"errors"
	"net"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"
)

// Starts in the same instant, as the browser makes them when the extension
// asks ODM two things while it is closed (one native host, and one ODM
// start, per message): exactly one takes the lock and binds the socket, and
// every other one is told ODM runs and reaches that socket. Before the
// lock, the check and the bind were apart, and both starts of a pair ran.
func TestOneOfConcurrentStartsRuns(t *testing.T) {
	const starts = 4
	for trial := 0; trial < 50; trial++ {
		dir := t.TempDir()
		path := SocketPath(dir)
		var wg sync.WaitGroup
		begin := make(chan struct{})
		locks := make([]*Lock, starts)
		lns := make([]net.Listener, starts)
		errs := make([]error, starts)
		reached := make([]bool, starts)
		for i := 0; i < starts; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-begin
				// What cmd/odm does at start.
				locks[i], errs[i] = TakeLock(dir)
				switch {
				case errs[i] == nil:
					lns[i], errs[i] = Listen(path)
				case errors.Is(errs[i], ErrRunning):
					reached[i] = Wait(path, 5*time.Second)
				}
			}(i)
		}
		close(begin)
		wg.Wait()

		running := 0
		for i := 0; i < starts; i++ {
			switch {
			case locks[i] != nil:
				running++
				if errs[i] != nil {
					t.Fatalf("trial %d: the lock holder could not bind the socket: %v", trial, errs[i])
				}
			case !errors.Is(errs[i], ErrRunning):
				t.Fatalf("trial %d: start %d: %v, want ErrRunning", trial, i, errs[i])
			case !reached[i]:
				t.Fatalf("trial %d: start %d never reached the running ODM's socket", trial, i)
			}
		}
		for i := 0; i < starts; i++ {
			if lns[i] != nil {
				lns[i].Close()
			}
			_ = locks[i].Release()
		}
		if running != 1 {
			t.Fatalf("trial %d: %d of %d starts took the lock, want 1", trial, running, starts)
		}
	}
}

// The lock is ODM's while it runs, and the next start's once it is
// released, as it is when ODM ends.
func TestTakeLockAgainAfterRelease(t *testing.T) {
	dir := t.TempDir() + "/odm"
	first, err := TakeLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := TakeLock(dir); !errors.Is(err, ErrRunning) {
		t.Fatalf("second TakeLock while held: %v, want ErrRunning", err)
	}
	if runtime.GOOS != "windows" {
		if fi, err := os.Stat(dir); err != nil || fi.Mode().Perm() != 0o700 {
			t.Fatalf("config folder made with %v (%v), want 0700", fi.Mode().Perm(), err)
		}
	}
	if err := first.Release(); err != nil {
		t.Fatal(err)
	}
	if err := first.Release(); err != nil {
		t.Fatalf("second Release: %v", err)
	}
	next, err := TakeLock(dir)
	if err != nil {
		t.Fatalf("TakeLock after release: %v", err)
	}
	next.Release()
	var none *Lock
	if err := none.Release(); err != nil {
		t.Fatalf("Release of no lock: %v", err)
	}
}

// Wait keeps dialing until the socket is bound, and gives up after the
// time it is given.
func TestWait(t *testing.T) {
	path := SocketPath(t.TempDir())
	start := time.Now()
	if Wait(path, 200*time.Millisecond) {
		t.Fatal("Wait found a socket nobody bound")
	}
	if d := time.Since(start); d < 200*time.Millisecond || d > 5*time.Second {
		t.Fatalf("Wait gave up after %v, want about 200ms", d)
	}

	bound := make(chan net.Listener, 1)
	go func() {
		time.Sleep(300 * time.Millisecond)
		ln, err := net.Listen("unix", path)
		if err != nil {
			t.Error(err)
		}
		bound <- ln
	}()
	if !Wait(path, 5*time.Second) {
		t.Fatal("Wait did not find the socket bound after it started")
	}
	if ln := <-bound; ln != nil {
		ln.Close()
	}
}
