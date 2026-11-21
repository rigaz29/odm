package torrent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// TestStandIn stands in for odm-libtorrent in the tests that start it
// (ODM_TORRENT_STAND_IN set): it says hello and answers every command, and
// "inspect" with a line over the maxLine a line may have.
func TestStandIn(t *testing.T) {
	if os.Getenv("ODM_TORRENT_STAND_IN") == "" {
		t.Skip("only run as odm-libtorrent's stand-in")
	}
	fmt.Println(`{"event":"hello","protocol":1,"version":"stand-in","libtorrent":"none"}`)
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		var m struct {
			ID  int64  `json:"id"`
			Cmd string `json:"cmd"`
		}
		_ = json.Unmarshal(sc.Bytes(), &m)
		if m.Cmd != "inspect" {
			fmt.Printf(`{"id":%d,"ok":true}`+"\n", m.ID)
			continue
		}
		line := fmt.Appendf(nil, `{"id":%d,"ok":true,"name":"`, m.ID)
		line = append(line, bytes.Repeat([]byte("a"), maxLine)...)
		os.Stdout.Write(append(line, "\"}\n"...))
	}
	os.Exit(0)
}

// A line over the limit ends the program, and the call waiting for it
// hears why, instead of every call waiting for good.
func TestClientEndsAProgramThatSendsAnOverlongLine(t *testing.T) {
	t.Setenv("ODM_TORRENT_STAND_IN", "1")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	c, err := Start(context.Background(), exe, Options{Args: []string{"-test.run=^TestStandIn$"}})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Kill()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := c.Call(ctx, "status", nil, nil); err != nil {
		t.Fatal(err)
	}
	err = c.Call(ctx, "inspect", map[string]any{"torrent": "x.torrent"}, nil)
	if err == nil || ctx.Err() != nil || !strings.Contains(err.Error(), "over 64 MiB") {
		t.Fatalf("inspect answered with a line over 64 MiB: %v", err)
	}
	select {
	case <-c.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("the program still runs")
	}
	if err := c.Call(ctx, "status", nil, nil); err == nil || ctx.Err() != nil {
		t.Fatalf("a call after it: %v", err)
	}
}
