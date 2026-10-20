package server

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"odm/internal/engine"
)

// upload posts a .torrent file and the form's fields to POST /api/torrents.
func upload(t *testing.T, h http.Handler, name string, data []byte, fields map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for k, v := range fields {
		mw.WriteField(k, v)
	}
	if data != nil {
		fw, _ := mw.CreateFormFile("torrent", name)
		fw.Write(data)
	}
	mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/torrents", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func TestUploadTorrent(t *testing.T) {
	prev := engine.TorrentDir
	engine.TorrentDir = t.TempDir()
	t.Cleanup(func() { engine.TorrentDir = prev })
	mgr := engine.NewManager(engine.Settings{Dir: t.TempDir()}, "")
	h := New(mgr).Handler()

	data := []byte("d4:infod4:name6:Seasonee")
	w := upload(t, h, `C:\Users\me\Season 1.torrent`, data, map[string]string{"start": "false", "description": "box set", "file_name": "My Season"})
	if w.Code != http.StatusCreated {
		t.Fatalf("upload: %d %s", w.Code, w.Body)
	}
	var info engine.Info
	json.Unmarshal(w.Body.Bytes(), &info)
	if !info.Torrent || info.URL != "Season 1.torrent" || info.Status != engine.StatusPaused || info.Description != "box set" || info.Category != "" && info.Category != "Other" {
		t.Fatalf("added %+v", info)
	}
	if got, err := os.ReadFile(filepath.Join(engine.TorrentDir, info.ID+".torrent")); err != nil || !bytes.Equal(got, data) {
		t.Fatalf("kept %q, %v", got, err)
	}
	if tj, _ := mgr.Torrent(info.ID); tj.Name != "My Season" {
		t.Fatalf("name %q", tj.Name)
	}

	for _, c := range []struct {
		data []byte
		want int
	}{
		{nil, http.StatusBadRequest},
		{[]byte("<html>"), http.StatusBadRequest},
		{append([]byte("d"), bytes.Repeat([]byte("x"), engine.MaxTorrentFile)...), http.StatusBadRequest},
	} {
		if w := upload(t, h, "x.torrent", c.data, nil); w.Code != c.want && !(len(c.data) > engine.MaxTorrentFile && w.Code == http.StatusRequestEntityTooLarge) {
			t.Errorf("%d bytes: %d %s", len(c.data), w.Code, w.Body)
		}
	}
}

func TestTorrentFilesAPI(t *testing.T) {
	mgr := engine.NewManager(engine.Settings{Dir: t.TempDir()}, "")
	info := mgr.Create(engine.AddOptions{URL: "https://example.com/s.torrent", Torrent: &engine.TorrentJob{Files: []engine.TorrentFile{
		{Path: "S/a.mkv", Size: 10}, {Path: ".pad/0", Size: 2, Pad: true}, {Path: "S/b.srt", Size: 1},
	}}})
	h := New(mgr).Handler()
	do := func(method, path, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
		return w
	}
	base := "/api/downloads/" + info.ID + "/files"
	if w := do(http.MethodPost, base, `{"priorities":{"0":7,"2":0}}`); w.Code != http.StatusOK {
		t.Fatalf("set: %d %s", w.Code, w.Body)
	}
	w := do(http.MethodGet, base, "")
	var files []torrentFile
	if err := json.Unmarshal(w.Body.Bytes(), &files); err != nil || w.Code != http.StatusOK {
		t.Fatalf("list: %d %s", w.Code, w.Body)
	}
	if len(files) != 2 || files[0].Index != 0 || files[0].Priority != 7 || files[1].Index != 2 || files[1].Priority != 0 {
		t.Fatalf("files %+v", files)
	}
	if w := do(http.MethodPost, base, `{"priorities":{"0":0}}`); w.Code != http.StatusBadRequest {
		t.Fatalf("every file left out: %d %s", w.Code, w.Body)
	}
	if w := do(http.MethodGet, "/api/downloads/nope/files", ""); w.Code != http.StatusNotFound {
		t.Fatalf("unknown id: %d", w.Code)
	}
}

// The same torrent sent again is the one in the list (200), found by its
// info hash; another file of the same name is not.
func TestUploadTorrentTwice(t *testing.T) {
	prev := engine.TorrentDir
	engine.TorrentDir = t.TempDir()
	t.Cleanup(func() { engine.TorrentDir = prev })
	mgr := engine.NewManager(engine.Settings{Dir: t.TempDir()}, "")
	h := New(mgr).Handler()
	torrent := func(name string) []byte {
		return []byte("d4:infod6:lengthi5e4:name" + strconv.Itoa(len(name)) + ":" + name + "12:piece lengthi16384e6:pieces20:" + strings.Repeat("p", 20) + "ee")
	}
	var first engine.Info
	w := upload(t, h, "a.torrent", torrent("a.bin"), nil)
	json.Unmarshal(w.Body.Bytes(), &first)
	if w.Code != http.StatusCreated {
		t.Fatalf("first: %d %s", w.Code, w.Body)
	}
	var again engine.Info
	w = upload(t, h, "renamed.torrent", torrent("a.bin"), nil)
	json.Unmarshal(w.Body.Bytes(), &again)
	if w.Code != http.StatusOK || again.ID != first.ID {
		t.Fatalf("again: %d %s", w.Code, w.Body)
	}
	if w := upload(t, h, "a.torrent", torrent("b.bin"), nil); w.Code != http.StatusCreated {
		t.Fatalf("another torrent of the same file name: %d %s", w.Code, w.Body)
	}
	if n := len(mgr.List()); n != 2 {
		t.Fatalf("%d downloads", n)
	}
}

func TestTorrentSeedAPI(t *testing.T) {
	mgr := engine.NewManager(engine.Settings{Dir: t.TempDir()}, "")
	info := mgr.Create(engine.AddOptions{URL: "magnet:?xt=urn:btih:" + strings.Repeat("a", 40)})
	h := New(mgr).Handler()
	post := func(body string) int {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/downloads/"+info.ID+"/seed", strings.NewReader(body)))
		return w.Code
	}
	if c := post(`{"mode":"","ratio":250,"minutes":90}`); c != http.StatusOK {
		t.Fatalf("set: %d", c)
	}
	if tj, _ := mgr.Torrent(info.ID); tj.Seed == nil || tj.Seed.Ratio != 250 || tj.Seed.Minutes != 90 {
		t.Fatalf("seed %+v", tj.Seed)
	}
	if c := post(`{"mode":"options"}`); c != http.StatusOK {
		t.Fatalf("options: %d", c)
	}
	if tj, _ := mgr.Torrent(info.ID); tj.Seed != nil {
		t.Fatalf("seed %+v", tj.Seed)
	}
	if c := post(`{"mode":"often"}`); c != http.StatusBadRequest {
		t.Fatalf("bad mode: %d", c)
	}
}
