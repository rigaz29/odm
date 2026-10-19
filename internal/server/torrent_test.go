package server

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
