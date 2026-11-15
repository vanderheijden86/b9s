package agents

import (
	"testing"
	"time"
)

func TestParsePanes(t *testing.T) {
	p := ParsePanes("%1|main:1.0|cross-project-agents|123|/work/b9s|2.1.325\n%2|main:2.0|zsh|124|/work/b9s|zsh\nbad line\n")
	if len(p) != 2 || p[0].ID != "%1" || p[0].Target != "main:1.0" || p[0].Window != "cross-project-agents" || p[0].Path != "/work/b9s" {
		t.Fatalf("panes = %+v", p)
	}
}

func TestMatchPanesPrefersWindowName(t *testing.T) {
	now := time.Now()
	recent := &Session{Name: "other", Tool: ToolClaude, Cwd: "/work/b9s", LastActivity: now}
	named := &Session{Name: "cross-project-agents", Tool: ToolClaude, Cwd: "/work/b9s", LastActivity: now.Add(-time.Hour)}
	codex := &Session{Name: "x", Tool: ToolCodex, Cwd: "/work/b9s", LastActivity: now}
	panes := []Pane{
		{ID: "%1", Target: "main:1.0", Window: "cross-project-agents", Path: "/work/b9s", Command: "2.1.325"},
		{ID: "%2", Target: "main:2.0", Window: "shell", Path: "/work/b9s", Command: "zsh"},
		{ID: "%3", Target: "main:3.0", Window: "codex", Path: "/work/b9s", Command: "codex"},
	}
	MatchPanes([]*Session{recent, codex, named}, panes)
	if named.Where.Ref != "%1" {
		t.Errorf("named session pane = %q, want %%1", named.Where.Ref)
	}
	if recent.Where.Kind != WhereNone {
		t.Errorf("zsh pane matched %+v", recent.Where)
	}
	if codex.Where.Ref != "%3" {
		t.Errorf("codex pane = %q", codex.Where.Ref)
	}
}
