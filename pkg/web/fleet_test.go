package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vanderheijden86/b9s/internal/agents"
	"github.com/vanderheijden86/b9s/internal/datasource"
	"github.com/vanderheijden86/b9s/pkg/config"
)

// fleetNow is the fixed clock of the fleet fixtures. The transcript's last
// line is one minute older, so its session is live without a tmux pane.
var fleetNow = time.Date(2026, 11, 15, 10, 12, 0, 0, time.UTC)

const fleetExtraIssues = `{"id":"bd-ab12.3","title":"Review the thing","status":"in_progress","priority":1,"issue_type":"task","created_at":"2026-11-01T10:00:00Z","updated_at":"2026-11-01T10:00:00Z"}
{"id":"t-6","title":"Closed this morning","status":"closed","priority":2,"issue_type":"task","created_at":"2026-11-01T10:00:00Z","updated_at":"2026-11-15T10:10:00Z","closed_at":"2026-11-15T10:10:00Z"}
`

type fleetEnv struct {
	*httptest.Server
	srv     *Server
	client  *http.Client
	home    string
	project string
}

func writeTestFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runGit(t *testing.T, dir string, env []string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=Tester", "-c", "user.email=t@example.test", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Env = append(append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null"), env...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// newFleetEnv serves a project called b9s inside a temporary home that
// holds one Claude transcript for it. Nothing reads the real home.
func newFleetEnv(t *testing.T, withGit bool, tweak func(*Options)) *fleetEnv {
	t.Helper()
	home := t.TempDir()
	project := filepath.Join(home, "work", "b9s")
	writeTestFile(t, filepath.Join(project, ".beads", "issues.jsonl"), fixtureIssues+fleetExtraIssues)

	data, err := os.ReadFile("../../internal/agents/testdata/claude.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	body := strings.ReplaceAll(string(data), "/work/b9s/.worktrees/feat-x", project)
	body = strings.ReplaceAll(body, "2026-11-15T09:", "2026-11-15T10:")
	transcript := filepath.Join(home, ".claude", "projects", agents.ClaudeProjectDir(project), "s-claude-1.jsonl")
	writeTestFile(t, transcript, body)
	writeTestFile(t, filepath.Join(home, ".claude", "CLAUDE.md"), "global rules\n@~/.config/agents/AGENTS.md\n")
	writeTestFile(t, filepath.Join(home, ".config", "agents", "AGENTS.md"), strings.Repeat("baseline ", 100))
	writeTestFile(t, filepath.Join(project, "CLAUDE.md"), "project rules")
	old := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	for _, p := range []string{filepath.Join(home, ".claude", "CLAUDE.md"), filepath.Join(home, ".config", "agents", "AGENTS.md"), filepath.Join(project, "CLAUDE.md")} {
		if err := os.Chtimes(p, old, old); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chtimes(transcript, fleetNow, fleetNow); err != nil {
		t.Fatal(err)
	}
	if withGit {
		date := []string{"GIT_AUTHOR_DATE=2026-11-15T10:05:00Z", "GIT_COMMITTER_DATE=2026-11-15T10:05:00Z"}
		runGit(t, project, nil, "init", "-q")
		runGit(t, project, nil, "add", "CLAUDE.md")
		runGit(t, project, date, "commit", "-q", "-m", "docs: add rules", "-m", "Refs: bd-ab12.3")
	}

	store := NewStore(50 * time.Millisecond)
	t.Cleanup(store.Close)
	if failure := store.Open(datasource.OpenTarget{Name: "b9s", Dir: project}, project); failure != nil {
		t.Fatalf("open fixture: %s", failure.Message())
	}
	collector := agents.NewCollector(agents.Options{Home: home, Now: func() time.Time { return fleetNow },
		Panes: func(context.Context) []agents.Pane { return nil }})
	opts := Options{Store: store, Assets: testAssets, Heartbeat: 20 * time.Millisecond, Agents: collector,
		AgentsInterval: time.Hour, Now: func() time.Time { return fleetNow }}
	if tweak != nil {
		tweak(&opts)
	}
	srv, err := NewServer(opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.Close)
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	return &fleetEnv{Server: ts, srv: srv, client: &http.Client{Timeout: 10 * time.Second}, home: home, project: project}
}

func (e *fleetEnv) get(t *testing.T, path string, v any) int {
	t.Helper()
	resp, err := e.client.Get(e.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if v != nil {
		if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
	return resp.StatusCode
}

func TestAgentsEndpoint(t *testing.T) {
	e := newFleetEnv(t, false, nil)
	var got AgentsResponse
	if code := e.get(t, "/api/agents", &got); code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
	if len(got.Sessions) != 1 {
		t.Fatalf("sessions = %+v", got.Sessions)
	}
	s := got.Sessions[0]
	if s.ID != "s-claude-1" || s.Project != "b9s" || s.Tool != "claude" || s.Role != "reviewer" || s.TaskID != "bd-ab12.3" || !s.Live {
		t.Errorf("session = %+v", s)
	}
	if s.ContextLimit <= 0 || s.ContextPercent < 0 || s.Compactions != 1 {
		t.Errorf("context fields = %+v", s)
	}
	if got.At != "2026-11-15T10:12:00Z" {
		t.Errorf("at = %q", got.At)
	}
}

func TestAgentEndpoint(t *testing.T) {
	e := newFleetEnv(t, false, nil)
	var s AgentSession
	if code := e.get(t, "/api/agent?id=s-claude-1", &s); code != http.StatusOK || s.ID != "s-claude-1" {
		t.Fatalf("known: %d %+v", code, s)
	}
	var errBody map[string]string
	if code := e.get(t, "/api/agent?id=nope", &errBody); code != http.StatusNotFound || errBody["error"] == "" {
		t.Fatalf("unknown: %d %v", code, errBody)
	}
}

func TestAgentLogEndpoint(t *testing.T) {
	e := newFleetEnv(t, false, nil)
	var log AgentLogResponse
	if code := e.get(t, "/api/agent/log?id=s-claude-1", &log); code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
	kinds := map[string]bool{}
	for _, ev := range log.Events {
		kinds[ev.Kind] = true
		if ev.Time == "" {
			t.Errorf("event without time: %+v", ev)
		}
	}
	for _, want := range []string{"TASK", "TOOL", "AI"} {
		if !kinds[want] {
			t.Errorf("no %s event in %v", want, kinds)
		}
	}
	var errBody map[string]string
	if code := e.get(t, "/api/agent/log?id=nope", &errBody); code != http.StatusNotFound {
		t.Fatalf("unknown log: %d", code)
	}
}

func TestAgentContextEndpoint(t *testing.T) {
	e := newFleetEnv(t, false, nil)
	var ctxReport AgentContext
	if code := e.get(t, "/api/agent/context?id=s-claude-1", &ctxReport); code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
	if ctxReport.SessionID != "s-claude-1" || len(ctxReport.Layers) == 0 || len(ctxReport.Sources) == 0 {
		t.Fatalf("report = %+v", ctxReport)
	}
	names := map[string]bool{}
	for _, s := range ctxReport.Sources {
		names[s.Name] = true
	}
	hasClaudeMd := false
	for n := range names {
		hasClaudeMd = hasClaudeMd || strings.HasSuffix(n, "CLAUDE.md")
	}
	if !hasClaudeMd {
		t.Errorf("sources = %v", names)
	}
	var errBody map[string]string
	if code := e.get(t, "/api/agent/context?id=nope", &errBody); code != http.StatusNotFound {
		t.Fatalf("unknown context: %d", code)
	}
}

func stageByName(p FleetProject, name string) FleetStage {
	for _, s := range p.Stages {
		if s.Stage == name {
			return s
		}
	}
	return FleetStage{}
}

func TestFleetEndpointRollsUpTheOpenProject(t *testing.T) {
	e := newFleetEnv(t, false, nil)
	var fleet FleetResponse
	if code := e.get(t, "/api/fleet", &fleet); code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
	if len(fleet.Projects) != 1 || len(fleet.Sessions) != 1 {
		t.Fatalf("projects = %+v sessions = %d", fleet.Projects, len(fleet.Sessions))
	}
	p := fleet.Projects[0]
	if p.Name != "b9s" || !p.Current || p.LiveAgents != 1 {
		t.Errorf("row = %+v", p)
	}
	// Open: t-1, t-1.1, t-2, t-3, t-5, bd-ab12.3. t-4 and t-6 are closed.
	if p.Open != 6 || p.InProgress != 2 || p.Blocked != 2 || p.Ready != 2 || p.ClosedToday != 1 {
		t.Errorf("counts open/progress/blocked/ready/closed-today = %d/%d/%d/%d/%d", p.Open, p.InProgress, p.Blocked, p.Ready, p.ClosedToday)
	}
	if len(p.Stages) != 5 {
		t.Fatalf("stages = %d", len(p.Stages))
	}
	review := stageByName(p, "review")
	if len(review.Cards) != 1 || review.Cards[0].ID != "bd-ab12.3" || len(review.Cards[0].Agents) != 1 || review.Cards[0].Agents[0] != "s-claude-1" {
		t.Errorf("review cell = %+v", review)
	}
	if got := stageByName(p, "in_progress"); got.Count != 1 || got.Cards[0].ID != "t-2" {
		t.Errorf("in progress cell = %+v", got)
	}
	if got := stageByName(p, "ready"); got.Count != 2 {
		t.Errorf("ready cell = %+v", got)
	}
	if got := stageByName(p, "done"); got.Count != 1 || got.Cards[0].ID != "t-6" {
		t.Errorf("done cell = %+v", got)
	}
}

func TestFleetEndpointListsOtherProjectsWithTheirCounts(t *testing.T) {
	other := t.TempDir()
	writeTestFile(t, filepath.Join(other, ".beads", "issues.jsonl"), fixtureIssues)
	e := newFleetEnv(t, false, func(o *Options) {
		o.Projects = func() []config.Project { return []config.Project{{Name: "other", Path: other}} }
	})
	var fleet FleetResponse
	e.get(t, "/api/fleet", &fleet)
	byName := map[string]FleetProject{}
	for _, p := range fleet.Projects {
		byName[p.Name] = p
	}
	if len(byName) != 2 || byName["b9s"].LiveAgents != 1 {
		t.Fatalf("rows = %+v", fleet.Projects)
	}
	if o := byName["other"]; o.Current || o.Open != 3 || o.Key == "" || len(o.Stages) != 5 {
		t.Errorf("other = %+v", o)
	}
	if fleet.Projects[0].Name != "b9s" {
		t.Errorf("the row with a live agent should lead: %+v", fleet.Projects)
	}
}

func TestFleetPlacesASessionOfAnUnknownProject(t *testing.T) {
	sessions := []agents.Session{{ID: "x", Name: "impl-x", Project: "elsewhere", Role: "tester", State: agents.StateRunning}}
	e := newFleetEnv(t, false, nil)
	rows := e.srv.buildFleet(sessions)
	var found FleetProject
	for _, p := range rows {
		if p.Name == "elsewhere" {
			found = p
		}
	}
	if found.LiveAgents != 1 {
		t.Fatalf("rows = %+v", rows)
	}
	if cell := stageByName(found, "test"); len(cell.Cards) != 1 || cell.Cards[0].Title != "impl-x" || cell.Cards[0].Agents[0] != "x" {
		t.Errorf("test cell = %+v", cell)
	}
}

func TestContextMatrixEndpoint(t *testing.T) {
	e := newFleetEnv(t, false, nil)
	var m MatrixResponse
	if code := e.get(t, "/api/context-matrix", &m); code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
	if len(m.Sessions) != 1 || m.Sessions[0].ID != "s-claude-1" || len(m.Rows) == 0 {
		t.Fatalf("matrix = %+v", m)
	}
	for _, row := range m.Rows {
		if len(row.Cells) != len(m.Sessions) {
			t.Errorf("row %q has %d cells for %d sessions", row.Key, len(row.Cells), len(m.Sessions))
		}
	}
	if len(m.Layers) == 0 || len(m.Insights) == 0 {
		t.Errorf("layers/insights = %d/%d", len(m.Layers), len(m.Insights))
	}
}

func TestBuildMatrixPivotsSourcesAndFindsStaleCopies(t *testing.T) {
	live := []agents.Session{{ID: "a", Name: "alpha", State: agents.StateRunning}, {ID: "b", Name: "beta", State: agents.StateWaiting}}
	global := func(tokens int, stale bool) agents.Source {
		return agents.Source{Layer: agents.LayerInstructions, Name: "AGENTS.md", Path: "/h/AGENTS.md", Scope: "global", Tokens: tokens, Stale: stale}
	}
	reports := []agents.ContextReport{
		{SessionID: "a", Layers: []agents.LayerTotal{{Layer: agents.LayerInstructions, Tokens: 100}, {Layer: agents.LayerToolOutput, Tokens: 900}},
			Sources: []agents.Source{global(100, true)}},
		{SessionID: "b", Layers: []agents.LayerTotal{{Layer: agents.LayerInstructions, Tokens: 150}, {Layer: agents.LayerMemories, Tokens: 50}, {Layer: agents.LayerToolOutput, Tokens: 100}},
			Sources: []agents.Source{global(100, false), {Layer: agents.LayerMemories, Name: "memory: x", Path: "/h/x.md", Tokens: 50}}},
	}
	m := buildMatrix("t", live, reports)
	if len(m.Rows) != 3 {
		t.Fatalf("rows = %+v", m.Rows)
	}
	first := m.Rows[0]
	if first.Name != "AGENTS.md" || first.Sessions != 2 || first.Stale != 1 || !first.Cells[0].Stale || first.Cells[1].Stale {
		t.Errorf("first row = %+v", first)
	}
	if mem := m.Rows[1]; mem.Name != "memory: x" || mem.Cells[0].Loaded || !mem.Cells[1].Loaded {
		t.Errorf("memory row = %+v", mem)
	}
	if tool := m.Rows[2]; !tool.Aggregate || tool.Tokens != 1000 {
		t.Errorf("aggregate row = %+v", tool)
	}
	titles := []string{}
	for _, in := range m.Insights {
		titles = append(titles, in.Title)
	}
	joined := strings.Join(titles, "|")
	for _, want := range []string{"AGENTS.md is loaded by 2 of 2 sessions", "1 sessions run on an old copy of AGENTS.md", "Tool output fills 77% of all windows"} {
		if !strings.Contains(joined, want) {
			t.Errorf("insights %q lack %q", joined, want)
		}
	}
}

func TestBuildMatrixWithoutSessions(t *testing.T) {
	m := buildMatrix("t", nil, nil)
	if m.Sessions == nil || m.Rows == nil || m.Insights == nil || m.Layers == nil || len(m.Rows) != 0 {
		t.Errorf("empty matrix must carry empty lists, got %+v", m)
	}
}

func TestBuildLanePhases(t *testing.T) {
	at := func(min, sec int) time.Time {
		return fleetNow.Add(-2 * time.Hour).Add(time.Duration(min)*time.Minute + time.Duration(sec)*time.Second)
	}
	from, to := fleetNow.Add(-2*time.Hour), fleetNow
	sess := agents.Session{ID: "s", Name: "impl", State: agents.StateRunning}
	tests := []struct {
		name   string
		events []agents.Event
		phases []string
		marks  []string
	}{
		{"a lone AI line is planning", []agents.Event{{Time: at(1, 0), Kind: agents.EventAI, Text: "think"}}, []string{"plan", "idle"}, nil},
		{"AI beside a tool call is implementation", []agents.Event{
			{Time: at(1, 0), Kind: agents.EventAI, Text: "edit"}, {Time: at(1, 10), Kind: agents.EventTool, Tool: "Edit"},
		}, []string{"implement", "idle"}, nil},
		{"an error is a failure and a marker", []agents.Event{
			{Time: at(1, 0), Kind: agents.EventTool, Tool: "Bash"}, {Time: at(1, 10), Kind: agents.EventErr, Text: "boom"}, {Time: at(1, 20), Kind: agents.EventTool, Tool: "Bash"},
		}, []string{"implement", "fail", "implement", "idle"}, []string{"fail"}},
		{"a long silence is idle", []agents.Event{
			{Time: at(1, 0), Kind: agents.EventTool, Tool: "Read"}, {Time: at(30, 0), Kind: agents.EventTool, Tool: "Read"},
		}, []string{"implement", "idle", "implement", "idle"}, nil},
		{"bd calls are tracking", []agents.Event{{Time: at(1, 0), Kind: agents.EventBD, Text: "bd comments add"}}, []string{"track", "idle"}, []string{"bd"}},
		{"events outside the window are dropped", []agents.Event{{Time: from.Add(-time.Hour), Kind: agents.EventTool}}, nil, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			lane := buildLane(sess, tc.events, from, to)
			var got []string
			for _, seg := range lane.Segments {
				got = append(got, seg.Phase)
			}
			if strings.Join(got, ",") != strings.Join(tc.phases, ",") {
				t.Errorf("phases = %v, want %v", got, tc.phases)
			}
			var marks []string
			for _, m := range lane.Markers {
				marks = append(marks, m.Kind)
			}
			if strings.Join(marks, ",") != strings.Join(tc.marks, ",") {
				t.Errorf("markers = %v, want %v", marks, tc.marks)
			}
		})
	}
}

func TestTimelineEndpoint(t *testing.T) {
	e := newFleetEnv(t, true, nil)
	var tl TimelineResponse
	if code := e.get(t, "/api/timeline?hours=3", &tl); code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
	if tl.Hours != 3 || tl.To != "2026-11-15T10:12:00Z" || len(tl.Lanes) != 1 {
		t.Fatalf("timeline = %+v", tl)
	}
	lane := tl.Lanes[0]
	if lane.SessionID != "s-claude-1" || len(lane.Segments) == 0 {
		t.Fatalf("lane = %+v", lane)
	}
	phases := map[string]bool{}
	for _, s := range lane.Segments {
		phases[s.Phase] = true
		if s.Start >= s.End {
			t.Errorf("segment runs backwards: %+v", s)
		}
	}
	if !phases["implement"] {
		t.Errorf("phases = %v", phases)
	}
	commit := false
	for _, m := range lane.Markers {
		if m.Kind == "commit" && m.Label == "docs: add rules" {
			commit = true
		}
	}
	if !commit {
		t.Errorf("markers = %+v", lane.Markers)
	}
	for _, bad := range []string{"0", "49", "x"} {
		var errBody map[string]string
		if code := e.get(t, "/api/timeline?hours="+bad, &errBody); code != http.StatusBadRequest {
			t.Errorf("hours=%s: status %d", bad, code)
		}
	}
}

func TestCommitsEndpoint(t *testing.T) {
	e := newFleetEnv(t, true, nil)
	var got CommitsResponse
	if code := e.get(t, "/api/commits?limit=5", &got); code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
	if len(got.Commits) != 1 || got.Commits[0].Subject != "docs: add rules" || got.Commits[0].Time != "2026-11-15T10:05:00Z" {
		t.Fatalf("commits = %+v", got.Commits)
	}
	if c := got.Commits[0]; len(c.Issues) != 1 || c.Issues[0] != "bd-ab12.3" || c.Refs == nil || len(c.Hash) != 40 {
		t.Errorf("commit = %+v", c)
	}
	var errBody map[string]string
	for _, bad := range []string{"0", "401", "x"} {
		if code := e.get(t, "/api/commits?limit="+bad, &errBody); code != http.StatusBadRequest {
			t.Errorf("limit=%s: status %d", bad, code)
		}
	}
}

func TestCommitsEndpointWithoutARepository(t *testing.T) {
	e := newFleetEnv(t, false, nil)
	var errBody map[string]string
	// The temporary project sits outside any work tree, so git has no root.
	if code := e.get(t, "/api/commits", &errBody); code != http.StatusConflict || errBody["error"] == "" {
		t.Fatalf("status = %d body = %v", code, errBody)
	}
}

var fleetPaths = []string{
	"/api/agents", "/api/agent?id=s-claude-1", "/api/agent/log?id=s-claude-1", "/api/agent/context?id=s-claude-1",
	"/api/fleet", "/api/context-matrix", "/api/timeline", "/api/commits",
}

func TestFleetEndpointsRefuseAPublicServer(t *testing.T) {
	e := newFleetEnv(t, false, func(o *Options) { o.Public = true })
	for _, path := range fleetPaths {
		var errBody map[string]string
		if code := e.get(t, path, &errBody); code != http.StatusForbidden {
			t.Errorf("%s: status %d, want 403", path, code)
		}
	}
}

func TestFleetEndpointsRequirePairing(t *testing.T) {
	e := newFleetEnv(t, false, func(o *Options) { o.Auth = testAuth(t) })
	for _, path := range fleetPaths {
		resp, err := e.client.Get(e.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s: status %d, want 401", path, resp.StatusCode)
		}
	}
}

func TestFleetEndpointsAreReadOnly(t *testing.T) {
	e := newFleetEnv(t, false, nil)
	for _, path := range fleetPaths {
		resp, err := e.client.Post(e.URL+path, "application/json", strings.NewReader("{}"))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode < 400 {
			t.Errorf("POST %s: status %d", path, resp.StatusCode)
		}
	}
}

// countingSource counts scans so the feed's lifecycle is observable.
type countingSource struct {
	agents.Snapshot
	scans atomic.Int64
}

func (c *countingSource) Scan(context.Context) agents.Snapshot {
	c.scans.Add(1)
	return c.Snapshot
}
func (c *countingSource) Last() agents.Snapshot {
	if c.scans.Load() == 0 {
		return agents.Snapshot{}
	}
	return c.Snapshot
}
func (c *countingSource) Session(string) (agents.Session, bool) { return agents.Session{}, false }
func (c *countingSource) Events(context.Context, string) ([]agents.Event, error) {
	return nil, nil
}
func (c *countingSource) Context(string) (agents.ContextReport, bool) {
	return agents.ContextReport{}, false
}

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestAgentFeedScansWhileReadAndStopsWhenIdle(t *testing.T) {
	src := &countingSource{Snapshot: agents.Snapshot{At: fleetNow}}
	e := newFleetEnv(t, false, func(o *Options) {
		o.Agents, o.AgentsInterval, o.AgentsIdle = src, 5*time.Millisecond, 60*time.Millisecond
	})
	if src.scans.Load() != 0 || e.srv.feed.isRunning() {
		t.Fatal("the feed must not scan before a request")
	}
	var got AgentsResponse
	e.get(t, "/api/agents", &got)
	waitFor(t, "repeated scans", func() bool { return src.scans.Load() >= 3 })
	waitFor(t, "the loop to stop after the idle period", func() bool { return !e.srv.feed.isRunning() })
	stopped := src.scans.Load()
	time.Sleep(40 * time.Millisecond)
	if src.scans.Load() > stopped+1 {
		t.Errorf("scans went on after the loop stopped: %d then %d", stopped, src.scans.Load())
	}
	e.get(t, "/api/agents", &got)
	waitFor(t, "the loop to restart on the next request", func() bool { return src.scans.Load() > stopped+1 })
}
