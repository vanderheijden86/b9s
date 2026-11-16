package agents

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// fixtureHome builds a home directory with one Claude transcript whose cwd is
// a project inside the same temporary tree.
func fixtureHome(t *testing.T) (home, project, transcriptPath string) {
	home = t.TempDir()
	project = filepath.Join(home, "work", "b9s")
	data, err := os.ReadFile("testdata/claude.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	body := strings.ReplaceAll(string(data), "/work/b9s/.worktrees/feat-x", project)
	body = strings.ReplaceAll(body, "2026-11-15T09:", "2026-11-15T10:")
	transcriptPath = filepath.Join(home, ".claude", "projects", ClaudeProjectDir(project), "s-claude-1.jsonl")
	writeFile(t, transcriptPath, body)
	writeFile(t, filepath.Join(home, ".claude", "CLAUDE.md"), "global rules\n@~/.config/agents/AGENTS.md\n")
	writeFile(t, filepath.Join(home, ".config", "agents", "AGENTS.md"), strings.Repeat("baseline ", 100))
	writeFile(t, filepath.Join(project, "CLAUDE.md"), "project rules")
	writeFile(t, ClaudeMemoryFile(home, project), "- [x](x.md)")
	old := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	for _, p := range []string{filepath.Join(home, ".claude", "CLAUDE.md"), filepath.Join(home, ".config", "agents", "AGENTS.md"), filepath.Join(project, "CLAUDE.md"), ClaudeMemoryFile(home, project)} {
		os.Chtimes(p, old, old)
	}
	return home, project, transcriptPath
}

func TestCollectorScanStatesAndPanes(t *testing.T) {
	home, project, tp := fixtureHome(t)
	now := time.Date(2026, 11, 15, 10, 12, 0, 0, time.UTC)
	os.Chtimes(tp, now, now)
	c := NewCollector(Options{Home: home, Now: func() time.Time { return now }, Panes: func(context.Context) []Pane {
		return []Pane{{ID: "%7", Target: "main:3.0", Window: "review-thing", Path: project, Command: "2.1.325"}}
	}})
	snap := c.Scan(context.Background())
	if len(snap.Sessions) != 1 {
		t.Fatalf("sessions = %+v", snap.Sessions)
	}
	s := snap.Sessions[0]
	if s.State != StateWaiting || s.Where.Ref != "%7" || s.Project != "b9s" {
		t.Errorf("state/where/project = %s/%+v/%s", s.State, s.Where, s.Project)
	}

	// A later scan with nothing appended keeps the parse; without a pane and
	// long after the last activity the session is done.
	later := now.Add(2 * time.Hour)
	c.opts.Now = func() time.Time { return later }
	c.opts.Panes = func(context.Context) []Pane { return nil }
	s = c.Scan(context.Background()).Sessions[0]
	if s.State != StateDone || s.Compactions != 1 {
		t.Errorf("later state/compactions = %s/%d", s.State, s.Compactions)
	}
}

func TestCollectorReadsOnlyCompleteAppendedLines(t *testing.T) {
	home, _, tp := fixtureHome(t)
	now := time.Date(2026, 11, 15, 10, 12, 0, 0, time.UTC)
	c := NewCollector(Options{Home: home, Now: func() time.Time { return now }, Panes: func(context.Context) []Pane { return nil }})
	c.Scan(context.Background())
	f, _ := os.OpenFile(tp, os.O_APPEND|os.O_WRONLY, 0)
	f.WriteString(`{"type":"assistant","timestamp":"2026-11-15T10:11:30.000Z","sessionId":"s-claude-1","message":{"stop_reason":"tool_use","content":[{"type":"text","text":"more"}]}}` + "\n" + `{"type":"assistant","timest`)
	f.Close()
	os.Chtimes(tp, now, now)
	c.Scan(context.Background())
	ev, _ := c.Events(context.Background(), "s-claude-1")
	if last := ev[len(ev)-1]; last.Text != "more" {
		t.Errorf("last event = %+v", last)
	}
	s, _ := c.Session("s-claude-1")
	if s.State != StateRunning {
		t.Errorf("state after tool_use = %s", s.State)
	}
}

func TestCollectorContextReport(t *testing.T) {
	home, project, tp := fixtureHome(t)
	now := time.Date(2026, 11, 15, 10, 12, 0, 0, time.UTC)
	os.Chtimes(tp, now, now)
	// The project CLAUDE.md changes after the session loaded it.
	os.Chtimes(filepath.Join(project, "CLAUDE.md"), now, now)
	c := NewCollector(Options{Home: home, Now: func() time.Time { return now }, Panes: func(context.Context) []Pane { return nil }})
	c.Scan(context.Background())
	rep, ok := c.Context("s-claude-1")
	if !ok {
		t.Fatal("no report")
	}
	names := map[string]Source{}
	for _, s := range rep.Sources {
		names[s.Name] = s
	}
	for _, want := range []string{"~/.claude/CLAUDE.md", "~/.config/agents/AGENTS.md", "~/work/b9s/CLAUDE.md", "MEMORY.md (auto memory)", "run-tests-with-p2"} {
		if _, ok := names[want]; !ok {
			t.Errorf("missing source %q in %v", want, rep.Sources)
		}
	}
	if !names["~/work/b9s/CLAUDE.md"].Stale {
		t.Errorf("changed CLAUDE.md not stale")
	}
	if names["~/.claude/CLAUDE.md"].Stale {
		t.Errorf("unchanged CLAUDE.md stale")
	}
	total := 0
	for _, l := range rep.Layers {
		total += l.Tokens
	}
	if rep.Window != 30005 || total != rep.Window {
		t.Errorf("window %d, layer total %d", rep.Window, total)
	}
}

func TestCollectorActivityBuckets(t *testing.T) {
	home, _, tp := fixtureHome(t)
	now := time.Date(2026, 11, 15, 10, 12, 0, 0, time.UTC)
	os.Chtimes(tp, now, now)
	c := NewCollector(Options{Home: home, Now: func() time.Time { return now }, Panes: func(context.Context) []Pane { return nil }})
	c.Scan(context.Background())
	a := c.Activity(now, 3, 10*time.Minute)
	// Buckets end at now: [09:42,09:52) [09:52,10:02) [10:02,10:12].
	if len(a.Active) != 3 || a.Active[0] != 0 || a.Active[1] != 1 || a.Active[2] != 1 {
		t.Errorf("active = %v", a.Active)
	}
	if a.Errors[1] != 1 || a.Tokens[1] != 400 || a.Tokens[2] != 50 {
		t.Errorf("errors = %v tokens = %v", a.Errors, a.Tokens)
	}
}

// BenchmarkScanRealHome measures a rescan of a real home directory, the cost
// the fleet pays every refresh. It reads the transcripts and never writes;
// set B9S_FLEET_BENCH_HOME to run it.
func BenchmarkScanRealHome(b *testing.B) {
	home := os.Getenv("B9S_FLEET_BENCH_HOME")
	if home == "" {
		b.Skip("B9S_FLEET_BENCH_HOME is not set")
	}
	c := NewCollector(Options{Home: home})
	start := time.Now()
	snap := c.Scan(context.Background())
	b.Logf("first scan: %d sessions in %v", len(snap.Sessions), time.Since(start))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Scan(context.Background())
	}
}

// BenchmarkActivityRealHome measures the activity buckets the pulse view
// builds after every scan. It runs on the UI goroutine.
func BenchmarkActivityRealHome(b *testing.B) {
	home := os.Getenv("B9S_FLEET_BENCH_HOME")
	if home == "" {
		b.Skip("B9S_FLEET_BENCH_HOME is not set")
	}
	c := NewCollector(Options{Home: home})
	snap := c.Scan(context.Background())
	events := 0
	for _, st := range c.files {
		events += len(st.tr.events)
	}
	b.Logf("%d sessions, %d events", len(snap.Sessions), events)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Activity(time.Now(), 36, 10*time.Minute)
	}
}

// TestScanReadsEveryTranscriptOfAManyFileHome pins what a scan of many
// transcripts returns: each file is its own session with all its events. The
// files are read side by side, so run it with -race.
func TestScanReadsEveryTranscriptOfAManyFileHome(t *testing.T) {
	home, project, _ := fixtureHome(t)
	data, err := os.ReadFile("testdata/claude.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 15, 10, 12, 0, 0, time.UTC)
	const n = 40
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("s-many-%02d", i)
		body := strings.ReplaceAll(string(data), "/work/b9s/.worktrees/feat-x", project)
		body = strings.ReplaceAll(body, "s-claude-1", id)
		p := filepath.Join(home, ".claude", "projects", ClaudeProjectDir(project), id+".jsonl")
		writeFile(t, p, body)
		os.Chtimes(p, now, now)
	}
	c := NewCollector(Options{Home: home, Now: func() time.Time { return now }, Panes: func(context.Context) []Pane { return nil }})
	snap := c.Scan(context.Background())
	seen := map[string]bool{}
	for _, s := range snap.Sessions {
		seen[s.ID] = true
	}
	want, _ := c.Events(context.Background(), "s-many-00")
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("s-many-%02d", i)
		if !seen[id] {
			t.Fatalf("session %s missing from %d sessions", id, len(snap.Sessions))
		}
		got, _ := c.Events(context.Background(), id)
		if len(got) == 0 || len(got) != len(want) {
			t.Errorf("%s has %d events, want %d", id, len(got), len(want))
		}
	}
}
