package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"odm/internal/ipc"
)

// TestMain runs ODM itself, not the tests, when ODM_TEST_MAIN is set, so
// that a test can start this binary as ODM.
func TestMain(m *testing.M) {
	if os.Getenv("ODM_TEST_MAIN") == "1" {
		main()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// odmHome makes folders for ODMs a test starts and returns the environment
// that points their config, cache and tools folders there, and their
// config folder. The path is kept short: a socket's path has to fit in
// about 100 bytes. The settings keep ODM from registering itself with the
// browsers or for torrent files.
func odmHome(t *testing.T) (env []string, baseDir string) {
	t.Helper()
	parent := ""
	if runtime.GOOS != "windows" {
		parent = "/tmp"
	}
	root, err := os.MkdirTemp(parent, "odm")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	env = append(os.Environ(), "ODM_TEST_MAIN=1", "ODM_TOOLS_DIR="+filepath.Join(root, "tools"))
	switch runtime.GOOS {
	case "windows":
		env = append(env, "APPDATA="+root, "LOCALAPPDATA="+filepath.Join(root, "local"))
		baseDir = filepath.Join(root, "odm")
	case "darwin":
		env = append(env, "HOME="+root)
		baseDir = filepath.Join(root, "Library", "Application Support", "odm")
	default:
		env = append(env, "HOME="+root, "XDG_CONFIG_HOME="+root, "XDG_CACHE_HOME="+filepath.Join(root, "cache"))
		baseDir = filepath.Join(root, "odm")
	}
	if err := os.MkdirAll(baseDir, 0o700); err != nil {
		t.Fatal(err)
	}
	settings, _ := json.Marshal(map[string]any{
		"no_browser_register": true,
		"torrent":             map[string]any{"no_associate": true},
		"dir":                 filepath.Join(root, "downloads"),
	})
	if err := os.WriteFile(filepath.Join(baseDir, "settings.json"), settings, 0o600); err != nil {
		t.Fatal(err)
	}
	return env, baseDir
}

// odmProcess is an ODM a test started: what it printed, and when it ended.
type odmProcess struct {
	cmd    *exec.Cmd
	stderr bytes.Buffer
	ended  chan struct{}
}

func startODMProcess(t *testing.T, env []string, args ...string) *odmProcess {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	p := &odmProcess{cmd: exec.Command(exe, args...), ended: make(chan struct{})}
	p.cmd.Env = env
	p.cmd.Stderr = &p.stderr
	if err := p.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() { _ = p.cmd.Wait(); close(p.ended) }()
	t.Cleanup(p.stop)
	return p
}

func (p *odmProcess) running() bool {
	select {
	case <-p.ended:
		return false
	default:
		return true
	}
}

func (p *odmProcess) stop() {
	if p.running() {
		_ = p.cmd.Process.Kill()
	}
	<-p.ended
}

// Two ODMs started in the same instant, as two native hosts start them when
// the extension asks ODM two things while it is closed: one runs and binds
// the socket, the other hands off and ends. Before the instance lock, both
// kept running in most such pairs and resumed the same downloads. A killed
// ODM leaves no lock behind: the next start runs.
func TestConcurrentStartsRunOneODM(t *testing.T) {
	if testing.Short() {
		t.Skip("starts ODM processes")
	}
	const trials = 4
	for trial := 0; trial < trials; trial++ {
		env, baseDir := odmHome(t)
		var pair [2]*odmProcess
		var wg sync.WaitGroup
		begin := make(chan struct{})
		for i := range pair {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-begin
				pair[i] = startODMProcess(t, env, "-no-ui", "-hidden") // as startODM starts it
			}(i)
		}
		close(begin)
		wg.Wait()

		var loser, winner *odmProcess
		select {
		case <-pair[0].ended:
			loser, winner = pair[0], pair[1]
		case <-pair[1].ended:
			loser, winner = pair[1], pair[0]
		case <-time.After(20 * time.Second):
			t.Fatalf("trial %d: both ODMs still run after 20s", trial)
		}
		time.Sleep(300 * time.Millisecond)
		if !winner.running() {
			t.Fatalf("trial %d: both ODMs ended:\n%s\n%s", trial, &pair[0].stderr, &pair[1].stderr)
		}
		if !strings.Contains(loser.stderr.String(), "ODM is already running") {
			t.Fatalf("trial %d: the second start said %q", trial, &loser.stderr)
		}
		// The running ODM takes the browser's requests on its socket.
		sock := ipc.SocketPath(baseDir)
		if !ipc.Wait(sock, 10*time.Second) {
			t.Fatalf("trial %d: the running ODM does not answer on %s:\n%s", trial, sock, &winner.stderr)
		}
		resp, err := ipc.Client(sock, 10*time.Second).Get(ipc.BaseURL + "/api/config")
		if err != nil {
			t.Fatalf("trial %d: %v", trial, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("trial %d: /api/config: %s", trial, resp.Status)
		}
		winner.stop()

		if trial == trials-1 {
			next := startODMProcess(t, env, "-no-ui", "-hidden")
			if !ipc.Wait(sock, 10*time.Second) || !next.running() {
				t.Fatalf("a start after a killed ODM did not run:\n%s", &next.stderr)
			}
			next.stop()
		}
	}
}

// A start that found ODM running hands it its request, waiting for the
// running ODM's socket, which it binds a moment after taking the lock.
func TestHandOffWaitsForTheRunningODM(t *testing.T) {
	sock := ipc.SocketPath(t.TempDir())
	asked := make(chan string, 4)
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		asked <- r.Method + " " + r.URL.Path + " " + string(body)
		w.WriteHeader(http.StatusNoContent)
	})}
	defer srv.Close()
	go func() {
		time.Sleep(300 * time.Millisecond)
		ln, err := net.Listen("unix", sock)
		if err != nil {
			t.Error(err)
			return
		}
		_ = srv.Serve(ln)
	}()

	if err := toRunningODM(sock, "", false, 5*time.Second); err != nil {
		t.Fatalf("show: %v", err)
	}
	if got := <-asked; got != "POST /api/show " {
		t.Fatalf("a second start asked %q, want its window shown", got)
	}
	magnet := "magnet:?xt=urn:btih:abc"
	if err := toRunningODM(sock, magnet, true, 5*time.Second); err != nil {
		t.Fatalf("magnet: %v", err)
	}
	if got := <-asked; !strings.HasPrefix(got, "POST /api/downloads ") || !strings.Contains(got, `"url":"`+magnet+`"`) ||
		!strings.Contains(got, `"prompt":true`) {
		t.Fatalf("a magnet link was handed over as %q", got)
	}
	// A hidden start with nothing to open asks for nothing.
	if err := toRunningODM(sock, "", true, 5*time.Second); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-asked:
		t.Fatalf("a hidden start asked %q", got)
	case <-time.After(100 * time.Millisecond):
	}
	// Nobody answering is an error once the wait is over.
	if err := toRunningODM(ipc.SocketPath(t.TempDir()), "", false, 200*time.Millisecond); err == nil {
		t.Fatal("no error with no ODM answering")
	}
}
