package libtorrent

// The tests run odm-libtorrent, the companion program built from this
// folder, the way ODM does: commands on its stdin, replies and events on its
// stdout (PROTOCOL.md). They skip unless ODM_LIBTORRENT names the program:
//
//	ODM_LIBTORRENT=$PWD/build/odm-libtorrent go test ./helper/libtorrent/
//
// Every peer is a second odm-libtorrent on 127.0.0.1, so no test needs the
// internet.

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func program(t *testing.T) string {
	t.Helper()
	p := os.Getenv("ODM_LIBTORRENT")
	if p == "" {
		t.Skip("ODM_LIBTORRENT is not set")
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

// msg is a command, a reply or an event.
type msg map[string]any

func (m msg) str(k string) string { s, _ := m[k].(string); return s }
func (m msg) num(k string) int64  { f, _ := m[k].(float64); return int64(f) }
func (m msg) flag(k string) bool  { b, _ := m[k].(bool); return b }

func (m msg) list(k string) []msg {
	var out []msg
	items, _ := m[k].([]any)
	for _, it := range items {
		v, _ := it.(map[string]any)
		out = append(out, msg(v))
	}
	return out
}

// helper is a running odm-libtorrent.
type helper struct {
	t      *testing.T
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	lines  chan msg
	exited chan struct{}
	// queue holds what was read while waiting for something else.
	queue []msg
	id    int
}

func start(t *testing.T, args ...string) *helper {
	t.Helper()
	cmd := exec.Command(program(t), args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	h := &helper{t: t, cmd: cmd, stdin: stdin, lines: make(chan msg, 256), exited: make(chan struct{})}
	go func() {
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 64<<10), 16<<20)
		for sc.Scan() {
			var m msg
			if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
				m = msg{"event": "not_json", "line": sc.Text()}
			}
			h.lines <- m
		}
		close(h.lines)
		cmd.Wait()
		close(h.exited)
	}()
	t.Cleanup(func() {
		h.stdin.Close()
		go func() {
			for range h.lines {
			}
		}()
		select {
		case <-h.exited:
		case <-time.After(20 * time.Second):
			cmd.Process.Kill()
			<-h.exited
		}
	})
	hello := h.event("hello", 15*time.Second, nil)
	if hello.num("protocol") != 1 || hello.str("libtorrent") == "" {
		t.Fatalf("hello: %v", hello)
	}
	return h
}

func (h *helper) send(m msg) {
	h.t.Helper()
	b, err := json.Marshal(m)
	if err != nil {
		h.t.Fatal(err)
	}
	if _, err := h.stdin.Write(append(b, '\n')); err != nil {
		h.t.Fatalf("send %v: %v", m, err)
	}
}

// read returns the first message match takes, from the queue or the
// program.
func (h *helper) read(timeout time.Duration, what string, match func(msg) bool) msg {
	h.t.Helper()
	for i, m := range h.queue {
		if match(m) {
			h.queue = append(h.queue[:i], h.queue[i+1:]...)
			return m
		}
	}
	deadline := time.After(timeout)
	for {
		select {
		case m, ok := <-h.lines:
			switch {
			case !ok:
				h.t.Fatalf("odm-libtorrent exited while waiting for %s", what)
			case m.str("event") == "not_json":
				h.t.Fatalf("not JSON: %q", m.str("line"))
			case match(m):
				return m
			case m.str("event") == "status":
				// Status events come every second; the tests ask instead.
			default:
				if m.str("event") == "error" {
					h.t.Logf("error event: %v", m)
				}
				h.queue = append(h.queue, m)
			}
		case <-deadline:
			h.t.Fatalf("no %s in %v", what, timeout)
		}
	}
}

func (h *helper) event(name string, timeout time.Duration, match func(msg) bool) msg {
	h.t.Helper()
	return h.read(timeout, name+" event", func(m msg) bool {
		return m.str("event") == name && (match == nil || match(m))
	})
}

// try sends a command and returns its reply, ok or not.
func (h *helper) try(m msg) msg {
	h.t.Helper()
	h.id++
	m["id"] = h.id
	h.send(m)
	want := float64(h.id)
	return h.read(30*time.Second, fmt.Sprintf("reply to %v", m["cmd"]), func(r msg) bool {
		id, ok := r["id"].(float64)
		return ok && id == want
	})
}

// call sends a command that must succeed.
func (h *helper) call(m msg) msg {
	h.t.Helper()
	r := h.try(m)
	if !r.flag("ok") {
		h.t.Fatalf("%v: %v", m["cmd"], r["error"])
	}
	return r
}

func (h *helper) status(key string) msg {
	h.t.Helper()
	list := h.call(msg{"cmd": "status", "key": key}).list("torrents")
	if len(list) != 1 {
		h.t.Fatalf("status of %s: %v", key, list)
	}
	return list[0]
}

// waitStatus asks for key's status until ok takes it.
func (h *helper) waitStatus(key string, timeout time.Duration, what string, ok func(msg) bool) msg {
	h.t.Helper()
	until := time.Now().Add(timeout)
	for {
		st := h.status(key)
		if ok(st) {
			return st
		}
		if time.Now().After(until) {
			h.t.Fatalf("%s is not %s after %v: %v", key, what, timeout, st)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// listen sets the program up on 127.0.0.1 alone and returns its TCP port.
func (h *helper) listen() int {
	h.t.Helper()
	h.call(msg{"cmd": "settings", "listen": "127.0.0.1:0", "dht": false, "lsd": false,
		"port_forwarding": false, "status_interval": 0})
	ev := h.event("listening", 15*time.Second, func(m msg) bool { return m.str("type") == "tcp" })
	return int(ev.num("port"))
}

func (h *helper) waitExit(timeout time.Duration) {
	h.t.Helper()
	go func() {
		for range h.lines {
		}
	}()
	select {
	case <-h.exited:
	case <-time.After(timeout):
		h.t.Fatalf("odm-libtorrent did not exit in %v", timeout)
	}
	if code := h.cmd.ProcessState.ExitCode(); code != 0 {
		h.t.Errorf("exit code %d", code)
	}
}

func peer(port int) []string { return []string{"127.0.0.1:" + strconv.Itoa(port)} }

func makeFile(t *testing.T, path string, size int) []byte {
	t.Helper()
	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return b
}

func sameFile(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s: its %d bytes are not the %d bytes seeded", path, len(got), len(want))
	}
}

func missing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("%s is there (%v)", path, err)
	}
}

// createTorrent makes a .torrent of src with "odm-libtorrent create".
func createTorrent(t *testing.T, src, out string, pieceSize int) msg {
	t.Helper()
	cmd := exec.Command(program(t), "create", src, out, "--piece-size", strconv.Itoa(pieceSize))
	cmd.Stderr = os.Stderr
	b, err := cmd.Output()
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	var m msg
	if err := json.Unmarshal(bytes.TrimSpace(b), &m); err != nil || m.str("info_hash") == "" {
		t.Fatalf("create printed %q", b)
	}
	return m
}

// seed starts a program that seeds torrent from dir, and returns its port.
func seed(t *testing.T, torrent, dir string, extra msg) int {
	t.Helper()
	h := start(t)
	port := h.listen()
	add := msg{"cmd": "add", "key": "seed", "torrent": torrent, "save_path": dir}
	for k, v := range extra {
		add[k] = v
	}
	h.call(add)
	h.waitStatus("seed", 30*time.Second, "seeding", func(s msg) bool { return s.flag("seeding") })
	return port
}

func TestVersion(t *testing.T) {
	out, err := exec.Command(program(t), "--version").Output()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "(protocol 1, libtorrent 2.") {
		t.Fatalf("--version: %q", out)
	}
}

func TestDownloadRenameAndQuit(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "seed", "data.bin")
	want := makeFile(t, src, 3<<20+12345)
	tf := filepath.Join(dir, "data.torrent")
	createTorrent(t, src, tf, 32<<10)
	port := seed(t, tf, filepath.Dir(src), nil)

	h := start(t)
	h.listen()
	got := filepath.Join(dir, "got")
	state := filepath.Join(dir, "state")
	resume := filepath.Join(state, "l.resume")
	r := h.call(msg{"cmd": "add", "key": "l", "torrent": tf, "save_path": got, "part_dir": state,
		"names": msg{"0": "data.bin.odmpart"}, "peers": peer(port), "resume": resume})
	if r.str("info_hash") == "" || r.flag("resumed") {
		t.Fatalf("add: %v", r)
	}
	md := h.event("metadata", 10*time.Second, nil)
	if files := md.list("files"); len(files) != 1 || files[0].str("path") != "data.bin" ||
		files[0].num("size") != int64(len(want)) || !strings.HasPrefix(md.str("magnet"), "magnet:?") {
		t.Fatalf("metadata: %v", md)
	}
	h.event("finished", 60*time.Second, nil)
	sameFile(t, filepath.Join(got, "data.bin.odmpart"), want)

	h.call(msg{"cmd": "rename", "key": "l", "names": msg{"0": "data.bin"}})
	if ev := h.event("renamed", 10*time.Second, nil); ev.num("index") != 0 || ev.str("path") != "data.bin" {
		t.Errorf("renamed: %v", ev)
	}
	sameFile(t, filepath.Join(got, "data.bin"), want)
	missing(t, filepath.Join(got, "data.bin.odmpart"))
	if st := h.status("l"); st.num("done") != int64(len(want)) || st.num("size") != int64(len(want)) ||
		!st.flag("seeding") {
		t.Errorf("status: %v", st)
	}
	pieces := h.call(msg{"cmd": "pieces", "key": "l"})
	if pieces.num("count") != int64((len(want)+32<<10-1)/(32<<10)) || pieces.str("have") == "" {
		t.Errorf("pieces: %v", pieces)
	}
	progress := h.call(msg{"cmd": "file_progress", "key": "l"})
	if files, _ := progress["files"].([]any); len(files) != 1 || files[0] != float64(len(want)) {
		t.Errorf("file_progress: %v", progress)
	}

	h.call(msg{"cmd": "quit"})
	h.event("bye", 20*time.Second, nil)
	h.waitExit(20 * time.Second)
	if fi, err := os.Stat(resume); err != nil || fi.Size() == 0 {
		t.Errorf("resume data: %v", err)
	}
}

func TestSelectFilesAndMove(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "seed", "Album")
	a := makeFile(t, filepath.Join(root, "a.bin"), 200000)
	makeFile(t, filepath.Join(root, "sub", "b.bin"), 300000)
	c := makeFile(t, filepath.Join(root, "c.bin"), 100001)
	tf := filepath.Join(dir, "album.torrent")
	createTorrent(t, root, tf, 32<<10)
	port := seed(t, tf, filepath.Join(dir, "seed"), nil)

	h := start(t)
	h.listen()
	info := h.call(msg{"cmd": "inspect", "torrent": tf})
	if info.str("name") != "Album" || info.str("info_hash") == "" {
		t.Fatalf("inspect: %v", info)
	}
	// ODM's layout: the files under <name>.odmpart without the torrent's
	// own folder, b.bin left out.
	names, prio := msg{}, []int{}
	for i, f := range info.list("files") {
		if f.flag("pad") {
			prio = append(prio, 0)
			continue
		}
		rel := strings.TrimPrefix(f.str("path"), "Album/")
		names[strconv.Itoa(i)] = rel
		if rel == "sub/b.bin" {
			prio = append(prio, 0)
		} else {
			prio = append(prio, 4)
		}
	}
	part := filepath.Join(dir, "got", "Album.odmpart")
	h.call(msg{"cmd": "add", "key": "l", "torrent": tf, "save_path": part, "names": names,
		"priorities": prio, "peers": peer(port), "part_dir": filepath.Join(dir, "state")})
	h.event("finished", 60*time.Second, nil)
	sameFile(t, filepath.Join(part, "a.bin"), a)
	sameFile(t, filepath.Join(part, "c.bin"), c)
	missing(t, filepath.Join(part, "sub", "b.bin"))
	if st := h.status("l"); st.num("size") != int64(len(a)+len(c)) || st.num("done") != st.num("size") {
		t.Errorf("status: %v, want size %d", st, len(a)+len(c))
	}

	final := filepath.Join(dir, "got", "Album")
	h.call(msg{"cmd": "move", "key": "l", "save_path": final})
	if ev := h.event("moved", 20*time.Second, nil); filepath.Clean(filepath.FromSlash(ev.str("path"))) != final {
		t.Errorf("moved: %v", ev)
	}
	sameFile(t, filepath.Join(final, "a.bin"), a)
	sameFile(t, filepath.Join(final, "c.bin"), c)
	if left, err := os.ReadDir(part); err == nil {
		t.Logf("Album.odmpart is still there after the move, with %d entries", len(left))
	}
}

func TestPauseResumeAndRestart(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "seed", "big.bin")
	want := makeFile(t, src, 8<<20)
	tf := filepath.Join(dir, "big.torrent")
	createTorrent(t, src, tf, 64<<10)
	// The seeder sends 1 MiB/s, so the download takes about 8 seconds.
	port := seed(t, tf, filepath.Dir(src), msg{"upload_limit": 1 << 20})

	got := filepath.Join(dir, "got")
	resume := filepath.Join(dir, "state", "l.resume")
	add := func() msg {
		return msg{"cmd": "add", "key": "l", "torrent": tf, "save_path": got, "peers": peer(port), "resume": resume}
	}

	h := start(t)
	h.listen()
	h.call(add())
	st := h.waitStatus("l", 30*time.Second, "past 1 MiB", func(s msg) bool { return s.num("done") >= 1<<20 })
	if st.num("done") == st.num("size") {
		t.Fatalf("the seeder's upload limit did not hold: done at once (%v)", st)
	}
	h.call(msg{"cmd": "pause", "key": "l"})
	paused := h.waitStatus("l", 10*time.Second, "paused", func(s msg) bool { return s.flag("paused") })
	time.Sleep(1500 * time.Millisecond)
	if st := h.status("l"); st.num("done") > paused.num("done")+(64<<10) {
		t.Errorf("went on while paused: %d → %d", paused.num("done"), st.num("done"))
	}
	h.call(msg{"cmd": "resume", "key": "l"})
	before := h.waitStatus("l", 30*time.Second, "past 3 MiB", func(s msg) bool {
		return s.num("done") >= 3<<20 && !s.flag("paused")
	})
	if before.num("done") == before.num("size") {
		t.Fatalf("finished before the restart: %v", before)
	}
	// ODM going away closes the program's input: it keeps its resume data
	// and exits.
	h.stdin.Close()
	h.waitExit(20 * time.Second)

	h2 := start(t)
	h2.listen()
	if r := h2.call(add()); !r.flag("resumed") {
		t.Fatalf("add after the restart: %v", r)
	}
	first := h2.waitStatus("l", 30*time.Second, "checked", func(s msg) bool {
		state := s.str("state")
		return state == "downloading" || state == "finished" || state == "seeding"
	})
	if first.num("done") < 3<<20-(128<<10) {
		t.Errorf("started over after the restart: %d of %d bytes kept", first.num("done"), before.num("done"))
	}
	h2.event("finished", 60*time.Second, nil)
	sameFile(t, filepath.Join(got, "big.bin"), want)
}

func TestMagnetHeldUntilFilesAreChosen(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "seed", "data.bin")
	want := makeFile(t, src, 1<<20+7)
	tf := filepath.Join(dir, "data.torrent")
	ih := createTorrent(t, src, tf, 32<<10).str("info_hash")
	port := seed(t, tf, filepath.Dir(src), nil)

	h := start(t)
	h.listen()
	got := filepath.Join(dir, "got")
	magnet := "magnet:?xt=urn:btih:" + ih + "&dn=data.bin"
	if info := h.call(msg{"cmd": "inspect", "magnet": magnet}); info.str("info_hash") != ih ||
		info.str("name") != "data.bin" {
		t.Errorf("inspect: %v", info)
	}
	h.call(msg{"cmd": "add", "key": "m", "magnet": magnet, "hold": true, "save_path": got, "peers": peer(port)})
	if md := h.event("metadata", 30*time.Second, nil); md.str("name") != "data.bin" {
		t.Fatalf("metadata: %v", md)
	}
	time.Sleep(time.Second)
	if st := h.status("m"); st.num("done") != 0 || st.num("size") != 0 {
		t.Errorf("downloaded while held: %v", st)
	}
	missing(t, filepath.Join(got, "data.bin"))

	h.call(msg{"cmd": "rename", "key": "m", "names": msg{"0": "data.bin.odmpart"}})
	h.event("renamed", 10*time.Second, nil)
	h.call(msg{"cmd": "priorities", "key": "m", "priorities": []int{4}})
	h.event("finished", 60*time.Second, nil)
	sameFile(t, filepath.Join(got, "data.bin.odmpart"), want)
}

func TestRemoveDeletesFiles(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "seed", "data.bin")
	want := makeFile(t, src, 300000)
	tf := filepath.Join(dir, "data.torrent")
	createTorrent(t, src, tf, 32<<10)
	port := seed(t, tf, filepath.Dir(src), nil)

	h := start(t)
	h.listen()
	got := filepath.Join(dir, "got")
	resume := filepath.Join(dir, "state", "l.resume")
	h.call(msg{"cmd": "add", "key": "l", "torrent": tf, "save_path": got, "peers": peer(port), "resume": resume})
	h.event("finished", 60*time.Second, nil)
	sameFile(t, filepath.Join(got, "data.bin"), want)
	h.event("resume_saved", 10*time.Second, nil)

	h.call(msg{"cmd": "remove", "key": "l", "delete_files": true})
	if ev := h.event("removed", 20*time.Second, nil); ev.str("key") != "l" || ev.str("error") != "" {
		t.Errorf("removed: %v", ev)
	}
	missing(t, filepath.Join(got, "data.bin"))
	missing(t, resume)
	if r := h.try(msg{"cmd": "status", "key": "l"}); r.flag("ok") {
		t.Errorf("status after remove: %v", r)
	}
}

func TestSettings(t *testing.T) {
	h := start(t)
	h.call(msg{"cmd": "settings", "listen": "127.0.0.1:0", "incoming": false, "port_forwarding": false,
		"dht": false, "lsd": false, "download_limit": 1 << 20, "upload_limit": 0, "user_agent": "ODM test",
		"proxy":           msg{"type": "socks5", "host": "127.0.0.1", "port": 1080, "user": "u", "password": "p", "peers": true},
		"status_interval": 500})
	h.call(msg{"cmd": "settings", "proxy": msg{"type": "none"}})
	if r := h.try(msg{"cmd": "settings", "proxy": msg{"type": "socks4"}}); r.flag("ok") {
		t.Errorf("a socks4 proxy was taken: %v", r)
	}
}

func TestBadCommands(t *testing.T) {
	h := start(t)
	if _, err := h.stdin.Write([]byte("not json\n")); err != nil {
		t.Fatal(err)
	}
	r := h.read(10*time.Second, "reply to a line that is not JSON", func(m msg) bool {
		id, has := m["id"]
		return has && id == nil
	})
	if r.flag("ok") || r.str("error") == "" {
		t.Errorf("not JSON: %v", r)
	}
	for _, m := range []msg{
		{"cmd": "nonsense"},
		{"cmd": "add"},
		{"cmd": "add", "key": "x", "save_path": "x"},
		{"cmd": "add", "key": "x", "magnet": "magnet:?xt=urn:btih:zz", "save_path": "x"},
		{"cmd": "pause", "key": "nope"},
		{"cmd": "connect", "key": "nope", "peer": "127.0.0.1:1"},
	} {
		if r := h.try(m); r.flag("ok") || r.str("error") == "" {
			t.Errorf("%v: %v", m, r)
		}
	}
	// Still answering.
	h.call(msg{"cmd": "status"})
}
