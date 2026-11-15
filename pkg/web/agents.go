package web

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/vanderheijden86/b9s/internal/agents"
)

// AgentSource is what the fleet endpoints read. *agents.Collector satisfies
// it, and a handler test passes a collector rooted in a temporary home.
type AgentSource interface {
	Scan(ctx context.Context) agents.Snapshot
	Last() agents.Snapshot
	Session(id string) (agents.Session, bool)
	Events(ctx context.Context, id string) ([]agents.Event, error)
	Context(id string) (agents.ContextReport, bool)
}

const (
	// defaultAgentsInterval is how often a scan runs while a browser looks at
	// a fleet view. A scan after the first reads only appended bytes.
	defaultAgentsInterval = 3 * time.Second
	// defaultAgentsIdle is how long the scan loop outlives the last request.
	defaultAgentsIdle = 30 * time.Second
)

// agentFeed keeps the agent snapshot fresh while someone reads it. The loop
// starts on a request and ends after agentsIdle without one, so a server
// nobody opens a fleet view on never reads a transcript.
type agentFeed struct {
	source   func() AgentSource
	interval time.Duration
	idle     time.Duration
	now      func() time.Time

	ctx    context.Context
	cancel context.CancelFunc

	srcOnce sync.Once
	src     AgentSource

	mu      sync.Mutex
	running bool
	lastUse time.Time
}

func newAgentFeed(opts Options) *agentFeed {
	f := &agentFeed{interval: opts.AgentsInterval, idle: opts.AgentsIdle, now: opts.Now}
	if f.interval <= 0 {
		f.interval = defaultAgentsInterval
	}
	if f.idle <= 0 {
		f.idle = defaultAgentsIdle
	}
	if f.now == nil {
		f.now = time.Now
	}
	f.ctx, f.cancel = context.WithCancel(context.Background())
	f.source = func() AgentSource {
		f.srcOnce.Do(func() {
			if opts.Agents != nil {
				f.src = opts.Agents
			} else {
				f.src = agents.NewCollector(agents.Options{})
			}
		})
		return f.src
	}
	return f
}

// touch records a request and starts the scan loop when it is not running.
func (f *agentFeed) touch() AgentSource {
	src := f.source()
	f.mu.Lock()
	f.lastUse = time.Now()
	start := !f.running
	f.running = true
	f.mu.Unlock()
	if start {
		go f.loop(src)
	}
	return src
}

func (f *agentFeed) loop(src AgentSource) {
	ticker := time.NewTicker(f.interval)
	defer ticker.Stop()
	for {
		src.Scan(f.ctx)
		select {
		case <-f.ctx.Done():
			f.stopped()
			return
		case <-ticker.C:
		}
		f.mu.Lock()
		quiet := time.Since(f.lastUse) > f.idle
		if quiet {
			f.running = false
		}
		f.mu.Unlock()
		if quiet {
			return
		}
	}
}

func (f *agentFeed) stopped() {
	f.mu.Lock()
	f.running = false
	f.mu.Unlock()
}

func (f *agentFeed) isRunning() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.running
}

// snapshot returns the latest scan, scanning once itself when none ran yet.
func (f *agentFeed) snapshot(ctx context.Context) (agents.Snapshot, AgentSource) {
	src := f.touch()
	snap := src.Last()
	if snap.At.IsZero() {
		snap = src.Scan(ctx)
	}
	return snap, src
}

// Close stops the scan loop.
func (s *Server) Close() {
	if s.feed != nil {
		s.feed.cancel()
	}
}

// agentsOnly refuses a request on a public server: transcripts hold local
// paths, prompts and tool output that an anonymous visitor must never read.
func (s *Server) agentsOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.feed == nil {
			writeError(w, http.StatusForbidden, "agent sessions are not served on a public server")
			return
		}
		next(w, r)
	}
}

func convertSession(s agents.Session) AgentSession {
	return AgentSession{
		ID: s.ID, Name: s.Name, Tool: string(s.Tool), Project: s.Project, Cwd: s.Cwd, Branch: s.Branch,
		Role: s.Role, TaskID: s.TaskID, State: string(s.State), Model: s.Model,
		ContextTokens: s.ContextTokens, ContextLimit: s.ContextLimit, ContextPercent: s.ContextPercent(),
		TokensPerMin: s.TokensPerMin, TotalTokens: s.TotalTokens, Compactions: s.Compactions,
		Started: formatTime(s.Started), LastActivity: formatTime(s.LastActivity),
		WhereKind: string(s.Where.Kind), WhereLabel: s.Where.Label,
		ToolCalls: s.Tools, Errors: s.Errors, Live: s.Live(),
	}
}

func convertSessions(list []agents.Session) []AgentSession {
	out := make([]AgentSession, len(list))
	for i := range list {
		out[i] = convertSession(list[i])
	}
	return out
}

func convertContext(r agents.ContextReport) AgentContext {
	out := AgentContext{SessionID: r.SessionID, Window: r.Window, Limit: r.Limit,
		Layers: make([]ContextLayer, len(r.Layers)), Sources: make([]ContextSource, len(r.Sources))}
	for i, l := range r.Layers {
		out.Layers[i] = ContextLayer{Layer: string(l.Layer), Tokens: l.Tokens}
	}
	for i, src := range r.Sources {
		out.Sources[i] = ContextSource{Layer: string(src.Layer), Name: src.Name, Path: src.Path, Scope: src.Scope,
			Tokens: src.Tokens, LoadedAt: formatTime(src.LoadedAt), ChangedAt: formatTime(src.ChangedAt), Stale: src.Stale}
	}
	return out
}

func (s *Server) handleAgents(w http.ResponseWriter, r *http.Request) {
	snap, _ := s.feed.snapshot(r.Context())
	writeJSON(w, r, AgentsResponse{At: formatTime(snap.At), Sessions: convertSessions(snap.Sessions), PodError: snap.PodError})
}

func (s *Server) handleAgent(w http.ResponseWriter, r *http.Request) {
	_, src := s.feed.snapshot(r.Context())
	session, ok := src.Session(r.URL.Query().Get("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "no such agent session")
		return
	}
	writeJSON(w, r, convertSession(session))
}

// maxLogEvents bounds one log response to the newest events.
const maxLogEvents = 2000

func (s *Server) handleAgentLog(w http.ResponseWriter, r *http.Request) {
	_, src := s.feed.snapshot(r.Context())
	id := r.URL.Query().Get("id")
	if _, ok := src.Session(id); !ok {
		writeError(w, http.StatusNotFound, "no such agent session")
		return
	}
	events, err := src.Events(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	if len(events) > maxLogEvents {
		events = events[len(events)-maxLogEvents:]
	}
	out := AgentLogResponse{ID: id, Events: make([]AgentEvent, len(events))}
	for i, e := range events {
		out.Events[i] = AgentEvent{Time: formatTime(e.Time), Kind: string(e.Kind), Tool: e.Tool, Text: e.Text, Detail: e.Detail}
	}
	writeJSON(w, r, out)
}

func (s *Server) handleAgentContext(w http.ResponseWriter, r *http.Request) {
	_, src := s.feed.snapshot(r.Context())
	report, ok := src.Context(r.URL.Query().Get("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "no such agent session")
		return
	}
	writeJSON(w, r, convertContext(report))
}
