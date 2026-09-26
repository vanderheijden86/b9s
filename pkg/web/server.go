package web

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/vanderheijden86/beadwork/internal/datasource"
	"github.com/vanderheijden86/beadwork/pkg/config"
	"github.com/vanderheijden86/beadwork/pkg/ui"
)

// Options configures a Server.
type Options struct {
	Store *Store
	// Writer runs bd. It is the TUI's writer, so both UIs send bd the same
	// commands.
	Writer *ui.IssueWriter
	// Auth is nil only for a loopback server started with --no-token.
	Auth *Auth
	// Projects lists the project sheet's rows, recent first, as the TUI
	// header does. It is called on every request, so it may reread config.
	Projects func() []config.Project
	// StartupUser is the Dolt user projects without a checkout are read as.
	StartupUser string
	// Assets is the built SPA. Nil serves the embedded bundle.
	Assets fs.FS
	// Heartbeat is the SSE keep-alive interval; zero means 15 seconds.
	Heartbeat time.Duration
	// Opened is told about every project the browser opened, so it joins the
	// recent list as it does when the TUI opens it.
	Opened func(config.Project)
	// InitialQuery is the query the browser starts with (--filter).
	InitialQuery string
}

// Server is the b9s web HTTP handler.
type Server struct {
	opts    Options
	assets  fs.FS
	etags   map[string]string
	mux     *http.ServeMux
	etagMu  sync.Mutex
	writeMu sync.Mutex
	idem    *idempotency
}

// NewServer builds the handler.
func NewServer(opts Options) (*Server, error) {
	if opts.Store == nil {
		return nil, fmt.Errorf("web: a store is required")
	}
	if opts.Writer == nil {
		opts.Writer = ui.NewIssueWriter()
	}
	if opts.Projects == nil {
		opts.Projects = func() []config.Project { return nil }
	}
	if opts.Heartbeat <= 0 {
		opts.Heartbeat = 15 * time.Second
	}
	assets := opts.Assets
	if assets == nil {
		var err error
		assets, err = EmbeddedAssets()
		if err != nil {
			return nil, err
		}
	}
	s := &Server{opts: opts, assets: assets, etags: map[string]string{}, idem: newIdempotency(512, 10*time.Minute)}
	s.mux = http.NewServeMux()
	s.routes()
	return s, nil
}

// ServeHTTP applies the headers every response carries, then routes.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h := w.Header()
	h.Set("X-Content-Type-Options", "nosniff")
	// The pairing URL carries the token; no page may leak it in a Referer.
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /pair", s.opts.Auth.pair)
	s.mux.HandleFunc("GET /api/session", s.paired(s.handleSession))
	s.mux.HandleFunc("GET /api/snapshot", s.paired(s.handleSnapshot))
	s.mux.HandleFunc("GET /api/query", s.paired(s.handleQuery))
	s.mux.HandleFunc("GET /api/health", s.paired(s.handleHealth))
	s.mux.HandleFunc("GET /api/events", s.paired(s.handleEvents))
	s.mux.HandleFunc("GET /api/projects", s.paired(s.handleProjects))
	s.mux.HandleFunc("POST /api/projects/open", s.paired(s.csrf(s.handleOpenProject)))
	s.mux.HandleFunc("POST /api/reload", s.paired(s.csrf(s.handleReload)))
	s.mux.HandleFunc("POST /api/write", s.paired(s.csrf(s.handleWrite)))
	noAPI := func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "no such API endpoint")
	}
	s.mux.HandleFunc("GET /api/", noAPI)
	s.mux.HandleFunc("POST /api/", noAPI)
	s.mux.HandleFunc("GET /", s.handleAsset)
}

func (s *Server) paired(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.opts.Auth.paired(r) {
			writeError(w, http.StatusUnauthorized, "this browser is not paired: open the pairing link that b9s web printed")
			return
		}
		next(w, r)
	}
}

func (s *Server) csrf(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.opts.Auth.csrfOK(r) {
			writeError(w, http.StatusForbidden, "missing or wrong "+csrfHeader+" header")
			return
		}
		next(w, r)
	}
}

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	csrf := ""
	if s.opts.Auth != nil {
		csrf = s.opts.Auth.CSRF()
	}
	writeJSON(w, r, Session{CSRF: csrf, Query: s.opts.InitialQuery})
}

func (s *Server) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, r, s.opts.Store.Snapshot(s.opts.Writer.IsAvailable()))
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, r, s.opts.Store.Health(s.opts.Writer.IsAvailable()))
}

// handleQuery evaluates the TUI query language on the server, so a query
// cannot match differently in the browser than it does in the terminal.
func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	query := ui.ParseIssueQuery(r.URL.Query().Get("q"))
	issues, version := s.opts.Store.Issues()
	ids := make([]string, 0, len(issues))
	for i := range issues {
		if query.Matches(issues[i]) {
			ids = append(ids, issues[i].ID)
		}
	}
	writeJSON(w, r, QueryResult{Version: version, IDs: ids})
}

func (s *Server) handleReload(w http.ResponseWriter, r *http.Request) {
	s.opts.Store.Reload()
	writeJSON(w, r, s.opts.Store.Health(s.opts.Writer.IsAvailable()))
}

// handleEvents streams Server-Sent Events until the client goes away.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming is not supported")
		return
	}
	events, cancel := s.opts.Store.Subscribe()
	defer cancel()

	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-store")
	// Proxies such as nginx buffer a response unless told not to.
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	send := func(e Event) bool {
		data, _ := json.Marshal(e)
		if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", e.Type, data); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}
	if !send(Event{Type: "hello", Version: s.opts.Store.Version()}) {
		return
	}

	heartbeat := time.NewTicker(s.opts.Heartbeat)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case e := <-events:
			if !send(e) {
				return
			}
		case <-heartbeat.C:
			if _, err := io.WriteString(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (s *Server) handleProjects(w http.ResponseWriter, r *http.Request) {
	projects := s.opts.Projects()
	counts, reach := ui.ProbeProjects(projects, s.opts.StartupUser)
	active := s.opts.Store.Info().Key

	entries := make([]ProjectEntry, 0, len(projects))
	for _, p := range projects {
		key := ui.ProjectKey(p)
		reachability, probed := reach[key]
		// A project the startup user may not read is hidden, unless it is
		// the one on screen (ADR 0015).
		if probed && reachability == datasource.ReachDenied && key != active {
			continue
		}
		location := p.ResolvedPath()
		if location == "" {
			location = p.Host + "/" + p.Database
		}
		entry := ProjectEntry{Key: key, Name: p.Name, Location: location, Active: key == active, Reach: "unknown"}
		if probed {
			entry.Reach = reachability.String()
		}
		if c, ok := counts[key]; ok {
			entry.Open, entry.InProgress, entry.Ready, entry.Blocked = c.Open, c.InProgress, c.Ready, c.Blocked
		}
		if len(entries) < config.MaxRecentProjects {
			entry.Slot = len(entries) + 1
		}
		entries = append(entries, entry)
	}
	writeJSON(w, r, ProjectList{Projects: entries})
}

func (s *Server) handleOpenProject(w http.ResponseWriter, r *http.Request) {
	var req OpenProjectRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	projects := s.opts.Projects()
	if req.Key == AllProjectsKey {
		if err := s.opts.Store.OpenAll(ui.AllProjectsDBs(projects, s.opts.StartupUser)); err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		writeJSON(w, r, s.opts.Store.Info())
		return
	}
	for _, p := range projects {
		if ui.ProjectKey(p) != req.Key {
			continue
		}
		if failure := s.opts.Store.Open(ui.ProjectOpenTarget(p, s.opts.StartupUser), req.Key); failure != nil {
			writeError(w, http.StatusBadGateway, failure.Message())
			return
		}
		if s.opts.Opened != nil {
			s.opts.Opened(p)
		}
		writeJSON(w, r, s.opts.Store.Info())
		return
	}
	writeError(w, http.StatusNotFound, "no project with key "+req.Key)
}

// handleAsset serves the embedded SPA. index.html is revalidated on every
// load so a new binary's bundle takes effect at once.
func (s *Server) handleAsset(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if name == "" {
		name = "index.html"
	}
	data, err := fs.ReadFile(s.assets, name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	etag := s.etag(name, data)
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "no-cache")
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	if ct := contentType(name); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	writeMaybeGzip(w, r, data)
}

func (s *Server) etag(name string, data []byte) string {
	s.etagMu.Lock()
	defer s.etagMu.Unlock()
	if tag, ok := s.etags[name]; ok {
		return tag
	}
	sum := sha256.Sum256(data)
	tag := `"` + hex.EncodeToString(sum[:8]) + `"`
	s.etags[name] = tag
	return tag
}

func contentType(name string) string {
	switch path.Ext(name) {
	case ".html":
		return "text/html; charset=utf-8"
	case ".js":
		return "text/javascript; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".json", ".webmanifest":
		return "application/manifest+json"
	case ".svg":
		return "image/svg+xml"
	case ".png":
		return "image/png"
	}
	return ""
}

func decodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("invalid request body: %w", err)
	}
	return nil
}

func writeJSON(w http.ResponseWriter, r *http.Request, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	writeMaybeGzip(w, r, data)
}

// writeJSONStatus writes v uncompressed with status: a write result is small
// and its status matters more than its size.
func writeJSONStatus(w http.ResponseWriter, status int, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	data, _ := json.Marshal(map[string]string{"error": message})
	_, _ = w.Write(data)
}

// writeMaybeGzip compresses bodies large enough to matter: a snapshot of a
// thousand issues shrinks about eight times, which counts on a phone link.
func writeMaybeGzip(w http.ResponseWriter, r *http.Request, data []byte) {
	w.Header().Add("Vary", "Accept-Encoding")
	if len(data) < 1024 || !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		_, _ = w.Write(data)
		return
	}
	w.Header().Set("Content-Encoding", "gzip")
	gz := gzip.NewWriter(w)
	_, _ = gz.Write(data)
	_ = gz.Close()
}
