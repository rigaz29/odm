// Package torrent runs odm-libtorrent, ODM's companion program for torrents
// (helper/libtorrent), and gives the engine its torrents through Session,
// an engine.TorrentBackend.
package torrent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	"odm/internal/tools"
)

// Client is one running odm-libtorrent, spoken to in its protocol
// (helper/libtorrent/PROTOCOL.md): commands on its standard input, replies
// and events on its standard output.
type Client struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	job     io.Closer // on Windows, the job object that ends it with ODM
	jobOnce sync.Once
	onEvent func(Event)
	stderr  *tail
	hello   Event

	writeMu sync.Mutex
	mu      sync.Mutex
	nextID  int64
	waiting map[int64]chan []byte

	exited chan struct{}
	err    error // why it exited, once exited is closed
}

// Event is an event of odm-libtorrent. Only the fields of its kind are set.
type Event struct {
	Event string `json:"event"`
	Key   string `json:"key"`
	// hello
	Protocol   int    `json:"protocol"`
	Version    string `json:"version"`
	Libtorrent string `json:"libtorrent"`
	// metadata
	Metadata
	// status
	Torrents []Status `json:"torrents"`
	// renamed, rename_failed, moved, move_failed, removed, error
	Index int    `json:"index"`
	Path  string `json:"path"`
	Error string `json:"error"`
	File  string `json:"file"`
	Op    string `json:"op"`
	// listening, listen_failed, portmap, portmap_failed
	Address string `json:"address"`
	Port    int    `json:"port"`
	Type    string `json:"type"`
	Via     string `json:"via"`
}

// Metadata describes a torrent: the metadata event, and inspect's reply.
type Metadata struct {
	Name       string   `json:"name"`
	Size       int64    `json:"size"`
	PieceSize  int      `json:"piece_size"`
	Pieces     int      `json:"pieces"`
	Private    bool     `json:"private"`
	Files      []File   `json:"files"`
	Trackers   []string `json:"trackers"`
	InfoHash   string   `json:"info_hash"`
	InfoHashV2 string   `json:"info_hash_v2"`
	Magnet     string   `json:"magnet"`
}

// File is a file of a torrent.
type File struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
	Pad  bool   `json:"pad"`
}

// Status is how far a torrent is.
type Status struct {
	Key          string `json:"key"`
	State        string `json:"state"`
	Paused       bool   `json:"paused"`
	HasMetadata  bool   `json:"has_metadata"`
	Size         int64  `json:"size"`
	Done         int64  `json:"done"`
	Total        int64  `json:"total"`
	TotalDone    int64  `json:"total_done"`
	DownloadRate int64  `json:"download_rate"`
	UploadRate   int64  `json:"upload_rate"`
	Downloaded   int64  `json:"downloaded"`
	Uploaded     int64  `json:"uploaded"`
	FailedBytes  int64  `json:"failed_bytes"`
	Peers        int    `json:"peers"`
	Seeds        int    `json:"seeds"`
	SwarmSeeds   int    `json:"swarm_seeds"`
	SwarmPeers   int    `json:"swarm_peers"`
	Finished     bool   `json:"finished"`
	Seeding      bool   `json:"seeding"`
	Moving       bool   `json:"moving"`
	ActiveTime   int64  `json:"active_time"`
	SeedingTime  int64  `json:"seeding_time"`
	Error        string `json:"error"`
}

// Options start a Client.
type Options struct {
	Args []string
	// OnEvent is told each event but hello, from the goroutine that reads
	// the program's output: it must not wait on the client.
	OnEvent func(Event)
}

// helloWait bounds the wait for the program to say hello.
var helloWait = 15 * time.Second

// Start runs program and waits for it to say hello in this ODM's protocol.
func Start(ctx context.Context, program string, o Options) (*Client, error) {
	cmd := exec.Command(program, o.Args...)
	hide(cmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	c := &Client{cmd: cmd, stdin: stdin, onEvent: o.OnEvent, stderr: &tail{},
		waiting: map[int64]chan []byte{}, exited: make(chan struct{})}
	cmd.Stderr = c.stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	c.job = bind(cmd.Process)
	hello := make(chan Event, 1)
	go c.read(stdout, hello)

	timer := time.NewTimer(helloWait)
	defer timer.Stop()
	select {
	case h := <-hello:
		if h.Protocol != tools.LibtorrentProtocol {
			c.Kill()
			return nil, fmt.Errorf("odm-libtorrent speaks protocol %d, this ODM %d", h.Protocol, tools.LibtorrentProtocol)
		}
		c.hello = h
		return c, nil
	case <-c.exited:
		return nil, c.exitError()
	case <-ctx.Done():
		c.Kill()
		return nil, ctx.Err()
	case <-timer.C:
		c.Kill()
		return nil, errors.New("odm-libtorrent did not start")
	}
}

// Hello is the program's hello: its protocol and versions.
func (c *Client) Hello() Event { return c.hello }

func (c *Client) read(stdout io.Reader, hello chan<- Event) {
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 64<<10), 64<<20)
	for sc.Scan() {
		line := sc.Bytes()
		var head struct {
			ID    *int64 `json:"id"`
			Event string `json:"event"`
		}
		if json.Unmarshal(line, &head) != nil {
			continue
		}
		if head.ID != nil {
			c.mu.Lock()
			ch := c.waiting[*head.ID]
			delete(c.waiting, *head.ID)
			c.mu.Unlock()
			if ch != nil {
				ch <- append([]byte(nil), line...)
			}
			continue
		}
		if head.Event == "" {
			continue // a reply to a line that was not JSON
		}
		var ev Event
		if json.Unmarshal(line, &ev) != nil {
			continue
		}
		if ev.Event == "hello" {
			select {
			case hello <- ev:
			default:
			}
			continue
		}
		if c.onEvent != nil {
			c.onEvent(ev)
		}
	}
	err := c.cmd.Wait()
	if err == nil {
		err = errors.New("exited")
	}
	c.mu.Lock()
	c.err = err
	for id, ch := range c.waiting {
		close(ch)
		delete(c.waiting, id)
	}
	c.mu.Unlock()
	close(c.exited)
}

// Call sends cmd with fields and waits for its reply, which it decodes into
// reply when reply is not nil.
func (c *Client) Call(ctx context.Context, cmd string, fields map[string]any, reply any) error {
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	ch := make(chan []byte, 1)
	c.waiting[id] = ch
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.waiting, id)
		c.mu.Unlock()
	}()
	msg := map[string]any{"id": id, "cmd": cmd}
	for k, v := range fields {
		msg[k] = v
	}
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	c.writeMu.Lock()
	_, err = c.stdin.Write(append(b, '\n'))
	c.writeMu.Unlock()
	if err != nil {
		select {
		case <-c.exited:
			return c.exitError()
		default:
			return fmt.Errorf("odm-libtorrent: %w", err)
		}
	}
	select {
	case line, ok := <-ch:
		if !ok {
			return c.exitError()
		}
		var r struct {
			OK    bool   `json:"ok"`
			Error string `json:"error"`
		}
		if err := json.Unmarshal(line, &r); err != nil {
			return err
		}
		if !r.OK {
			return errors.New(r.Error)
		}
		if reply != nil {
			return json.Unmarshal(line, reply)
		}
		return nil
	case <-c.exited:
		return c.exitError()
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Done is closed once the program has exited.
func (c *Client) Done() <-chan struct{} { return c.exited }

// Err is why the program exited, once Done is closed.
func (c *Client) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

func (c *Client) exitError() error {
	msg := fmt.Sprint(c.Err())
	if t := c.stderr.String(); t != "" {
		msg += ": " + t
	}
	return fmt.Errorf("odm-libtorrent stopped (%s)", msg)
}

// Close asks the program to quit, which pauses its torrents and saves their
// resume data, and kills it if it has not exited within wait.
func (c *Client) Close(wait time.Duration) {
	c.writeMu.Lock()
	_, _ = c.stdin.Write([]byte(`{"cmd":"quit"}` + "\n"))
	_ = c.stdin.Close()
	c.writeMu.Unlock()
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-c.exited:
	case <-timer.C:
		c.Kill()
	}
	c.closeJob()
}

// Kill ends the program at once and waits for it.
func (c *Client) Kill() {
	_ = c.cmd.Process.Kill()
	<-c.exited
	c.closeJob()
}

func (c *Client) closeJob() {
	c.jobOnce.Do(func() {
		if c.job != nil {
			_ = c.job.Close()
		}
	})
}

// Version runs program --version: "odm-libtorrent 2.1.2-1 (protocol 1,
// libtorrent 2.1.2.0)".
func Version(ctx context.Context, program string) (string, error) {
	cmd := exec.CommandContext(ctx, program, "--version")
	hide(cmd)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// tail keeps the end of what the program wrote to its standard error, for
// the message when it stops.
type tail struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

const tailSize = 2 << 10

func (t *tail) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf.Write(p)
	if n := t.buf.Len(); n > tailSize {
		b := append([]byte(nil), t.buf.Bytes()[n-tailSize:]...)
		t.buf.Reset()
		t.buf.Write(b)
	}
	return len(p), nil
}

func (t *tail) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return strings.TrimSpace(t.buf.String())
}
