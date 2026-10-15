package server

import (
	"context"
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"time"

	"odm/internal/engine"
	"odm/internal/media"
	"odm/internal/osutil"
)

//go:embed web
var webFS embed.FS

// Server exposes the manager over a small JSON/SSE API plus the web UI.
type Server struct {
	mgr *engine.Manager

	// OnShow, when set, brings the desktop window forward; a second ODM
	// launch calls it over the control socket instead of starting twice.
	OnShow func()
	// OnPrompt, when set, shows the New Download window for a download the
	// browser handed over, instead of adding it directly. It reports whether
	// it took the request.
	OnPrompt func(engine.AddOptions) bool

	mu   sync.Mutex
	subs map[chan []byte]struct{}
}

// New builds a server for the given manager.
func New(mgr *engine.Manager) *Server {
	return &Server{mgr: mgr, subs: map[chan []byte]struct{}{}}
}

// Handler returns the HTTP routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/config", s.handleConfig)
	mux.HandleFunc("GET /api/settings", s.handleConfig)
	mux.HandleFunc("PUT /api/settings", s.handlePutSettings)
	mux.HandleFunc("PATCH /api/settings", s.handlePutSettings)
	mux.HandleFunc("GET /api/downloads", s.handleList)
	mux.HandleFunc("POST /api/downloads", s.handleAdd)
	mux.HandleFunc("GET /api/probe", s.handleProbe)
	mux.HandleFunc("GET /api/downloads/{id}", s.handleGet)
	mux.HandleFunc("POST /api/downloads/{id}/{action}", s.handleAction)
	mux.HandleFunc("POST /api/downloads/{id}/limit", s.handleLimit)
	mux.HandleFunc("POST /api/downloads/{id}/options", s.handleOptions)
	mux.HandleFunc("DELETE /api/downloads/{id}", s.handleDelete)
	mux.HandleFunc("POST /api/actions/{action}", s.handleBatch)
	mux.HandleFunc("POST /api/queues/{name}/{action}", s.handleQueue)
	mux.HandleFunc("GET /api/events", s.handleEvents)
	mux.HandleFunc("POST /api/show", func(w http.ResponseWriter, _ *http.Request) {
		if s.OnShow == nil {
			http.Error(w, "no window to show", http.StatusNotFound)
			return
		}
		s.OnShow()
		w.WriteHeader(http.StatusNoContent)
	})

	sub, err := fs.Sub(webFS, "web")
	if err == nil {
		mux.Handle("GET /", noStore(http.FileServer(http.FS(sub))))
	}
	return mux
}

// noStore stops the WebView2/browser from caching the embedded assets, so a
// rebuilt binary always shows the current UI.
func noStore(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		h.ServeHTTP(w, r)
	})
}

// Broadcast pushes the current list to every SSE subscriber. It never blocks.
func (s *Server) Broadcast() {
	msg, err := json.Marshal(s.mgr.List())
	if err != nil {
		return
	}
	s.mu.Lock()
	subs := make([]chan []byte, 0, len(s.subs))
	for ch := range s.subs {
		subs = append(subs, ch)
	}
	s.mu.Unlock()
	for _, ch := range subs {
		select {
		case ch <- msg:
		default:
		}
	}
}

func (s *Server) handleConfig(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, struct {
		engine.Settings
		CategoryList []string `json:"category_list"`
	}{Settings: s.mgr.Settings(), CategoryList: engine.Categories()})
}

// handlePutSettings applies the body over the current settings, so a field
// it leaves out keeps its value (a map in it adds or replaces entries). The
// web UI's Options sends a handful of fields; decoded into empty settings,
// they wiped the queues, schedules, logins, proxy and category folders.
func (s *Server) handlePutSettings(w http.ResponseWriter, r *http.Request) {
	next := s.mgr.Settings()
	if err := json.NewDecoder(r.Body).Decode(&next); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	applied := s.mgr.UpdateSettings(next)
	s.Broadcast()
	writeJSON(w, http.StatusOK, applied)
}

func (s *Server) handleList(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.mgr.List())
}

func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	info, ok := s.mgr.Get(r.PathValue("id"))
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, info)
}

type addRequest struct {
	URL          string            `json:"url"`
	Dir          string            `json:"dir"`
	Connections  int               `json:"connections"`
	Headers      map[string]string `json:"headers"`
	SpeedLimit   int64             `json:"speed_limit"`
	Start        *bool             `json:"start"`
	OpenWhenDone bool              `json:"open_when_done"`
	Category     string            `json:"category"`
	FileName     string            `json:"file_name"`
	Queue        string            `json:"queue"`
	Description  string            `json:"description"`
	// Checksum is the hash the file must have: "sha256:<hex>", sha1,
	// sha512, md5, or the hex alone.
	Checksum string `json:"checksum"`
	// Prompt marks a request a person started in the browser: the desktop
	// UI may confirm it in the New Download window (202) rather than add it.
	Prompt bool `json:"prompt"`
	// Stream ("hls" or "dash") makes URL a playlist to download segment
	// by segment instead of a file.
	Stream string `json:"stream"`
}

func (s *Server) handleAdd(w http.ResponseWriter, r *http.Request) {
	var req addRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	req.URL = strings.TrimSpace(req.URL)
	if req.URL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "url is required"})
		return
	}
	sum, err := engine.ParseChecksum(req.Checksum)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	start := true
	if req.Start != nil {
		start = *req.Start
	}
	opts := engine.AddOptions{
		URL: req.URL, Dir: req.Dir, Connections: req.Connections, Headers: req.Headers,
		SpeedLimit: req.SpeedLimit, Start: start, OpenWhenDone: req.OpenWhenDone,
		Category:    strings.TrimSpace(req.Category),
		FileName:    engine.SanitizeName(req.FileName),
		Queue:       strings.TrimSpace(req.Queue),
		Description: strings.TrimSpace(req.Description),
		Checksum:    sum,
	}
	if req.Stream != "" {
		k := media.Kind(req.Stream)
		if k != media.KindHLS && k != media.KindDASH {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "stream must be hls or dash"})
			return
		}
		opts.Stream = &engine.StreamJob{Kind: k, Manifest: req.URL}
	}
	if req.Prompt && s.OnPrompt != nil && !s.mgr.Settings().NoStartDialog && s.OnPrompt(opts) {
		// The person decides in the window (it also handles duplicates).
		writeJSON(w, http.StatusAccepted, map[string]bool{"prompted": true})
		return
	}
	// Requests here come from the browser relay or scripts, so the
	// duplicate policy for automatic adds applies.
	info, added := s.mgr.AddAuto(opts)
	s.Broadcast()
	status := http.StatusCreated
	if !added {
		status = http.StatusOK // already in the list; info is the existing entry
	}
	writeJSON(w, status, info)
}

// handleProbe inspects a URL so the Add dialog can show size and file name.
func (s *Server) handleProbe(w http.ResponseWriter, r *http.Request) {
	raw := strings.TrimSpace(r.URL.Query().Get("url"))
	if raw == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "url is required"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	p, err := s.mgr.Probe(ctx, raw, nil)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"final_url": p.FinalURL, "file_name": p.FileName, "total": p.Total,
		"range_ok": p.RangeOK, "content_type": p.ContentType,
		"category": engine.Category(p.FileName),
	})
}

func (s *Server) handleLimit(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SpeedLimit int64 `json:"speed_limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if !s.mgr.SetSpeedLimit(r.PathValue("id"), body.SpeedLimit) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	s.Broadcast()
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleOptions(w http.ResponseWriter, r *http.Request) {
	var body struct {
		OpenWhenDone bool `json:"open_when_done"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if !s.mgr.SetOpenWhenDone(r.PathValue("id"), body.OpenWhenDone) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	s.Broadcast()
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleAction(w http.ResponseWriter, r *http.Request) {
	id, action := r.PathValue("id"), r.PathValue("action")
	switch action {
	case "pause":
		if !s.mgr.Pause(id) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
	case "resume", "retry":
		if !s.mgr.Resume(id) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
	case "open", "reveal":
		path, ok := s.mgr.Path(id)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		var err error
		if action == "open" {
			err = osutil.Open(path)
		} else {
			err = osutil.Reveal(path)
		}
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown action"})
		return
	}
	s.Broadcast()
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleBatch(w http.ResponseWriter, r *http.Request) {
	switch r.PathValue("action") {
	case "pause-all":
		s.mgr.PauseAll()
	case "stop-all":
		s.mgr.StopAll()
	case "resume-all":
		s.mgr.ResumeAll()
	case "clear-completed":
		s.mgr.ClearCompleted()
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown action"})
		return
	}
	s.Broadcast()
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleQueue starts or stops a queue; the name "*" means every queue.
func (s *Server) handleQueue(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	switch r.PathValue("action") {
	case "start":
		s.mgr.StartQueue(name)
	case "stop":
		s.mgr.StopQueue(name)
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown action"})
		return
	}
	s.Broadcast()
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	withFile := r.URL.Query().Get("file") == "true"
	if !s.mgr.RemoveWithFile(r.PathValue("id"), withFile) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	s.Broadcast()
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	ch := make(chan []byte, 1)
	s.addSub(ch)
	defer s.removeSub(ch)

	if msg, err := json.Marshal(s.mgr.List()); err == nil {
		_, _ = w.Write([]byte("data: "))
		_, _ = w.Write(msg)
		_, _ = w.Write([]byte("\n\n"))
		fl.Flush()
	}
	for {
		select {
		case <-r.Context().Done():
			return
		case msg := <-ch:
			_, _ = w.Write([]byte("data: "))
			_, _ = w.Write(msg)
			_, _ = w.Write([]byte("\n\n"))
			fl.Flush()
		}
	}
}

func (s *Server) addSub(ch chan []byte) {
	s.mu.Lock()
	s.subs[ch] = struct{}{}
	s.mu.Unlock()
}

func (s *Server) removeSub(ch chan []byte) {
	s.mu.Lock()
	delete(s.subs, ch)
	s.mu.Unlock()
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
