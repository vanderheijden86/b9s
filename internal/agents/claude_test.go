package agents

import (
	"os"
	"strings"
	"testing"
	"time"
)

func parseFixture(t *testing.T, tool Tool, name string) *transcript {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	tr := newTranscript(tool, "testdata/"+name)
	for _, line := range strings.Split(string(data), "\n") {
		tr.feed([]byte(line))
	}
	return tr
}

func TestClaudeTranscriptSummary(t *testing.T) {
	tr := parseFixture(t, ToolClaude, "claude.jsonl")
	s := tr.sess
	if s.ID != "s-claude-1" || s.Name != "review-thing" {
		t.Fatalf("id/name = %q/%q", s.ID, s.Name)
	}
	if s.Project != "b9s" {
		t.Errorf("project = %q, want b9s (worktree suffix stripped)", s.Project)
	}
	if s.Branch != "feat/bd-ab12.3-thing" || s.Model != "claude-opus-5-5" {
		t.Errorf("branch/model = %q/%q", s.Branch, s.Model)
	}
	if s.TaskID != "bd-ab12.3" {
		t.Errorf("task = %q", s.TaskID)
	}
	if s.Role != "reviewer" {
		t.Errorf("role = %q", s.Role)
	}
	if s.ContextTokens != 30005 {
		t.Errorf("context = %d, want last call input total 30005", s.ContextTokens)
	}
	if s.ContextLimit != 200000 {
		t.Errorf("limit = %d", s.ContextLimit)
	}
	if s.Compactions != 1 {
		t.Errorf("compactions = %d", s.Compactions)
	}
	if s.Tools != 3 || s.Errors != 1 {
		t.Errorf("tools/errors = %d/%d", s.Tools, s.Errors)
	}
	if !s.Started.Equal(time.Date(2026, 11, 15, 9, 0, 0, 0, time.UTC)) {
		t.Errorf("started = %v", s.Started)
	}
	if tr.lastStop != "end_turn" {
		t.Errorf("lastStop = %q", tr.lastStop)
	}
}

func TestClaudeTranscriptEvents(t *testing.T) {
	tr := parseFixture(t, ToolClaude, "claude.jsonl")
	kinds := map[EventKind]int{}
	for _, e := range tr.events {
		kinds[e.Kind]++
	}
	for _, k := range []EventKind{EventCtx, EventTask, EventAI, EventBD, EventTool, EventErr} {
		if kinds[k] == 0 {
			t.Errorf("no %s event in %v", k, kinds)
		}
	}
	var bd Event
	for _, e := range tr.events {
		if e.Kind == EventBD {
			bd = e
		}
	}
	if !strings.Contains(bd.Text, "bd update bd-ab12.3") {
		t.Errorf("bd event text = %q", bd.Text)
	}
}

func TestClaudeTranscriptSources(t *testing.T) {
	tr := parseFixture(t, ToolClaude, "claude.jsonl")
	got := map[Layer][]string{}
	for _, s := range tr.sourceList() {
		got[s.Layer] = append(got[s.Layer], s.Name)
	}
	if strings.Join(got[LayerMemories], ",") != "run-tests-with-p2,no-shared-dolt" {
		t.Errorf("memories = %v", got[LayerMemories])
	}
	if strings.Join(got[LayerSkills], ",") != "superpowers:test-driven-development" {
		t.Errorf("skills = %v", got[LayerSkills])
	}
	// Compaction folds files read and tool output into the summary.
	if len(got[LayerFiles]) != 0 {
		t.Errorf("files after compaction = %v", got[LayerFiles])
	}
	if len(got[LayerConversation]) == 0 {
		t.Errorf("no conversation source")
	}
}

func TestBDClaimNeedsACommandAndTheWholeID(t *testing.T) {
	for cmd, want := range map[string]string{
		"bd update osenco-infra-tmm --claim":                      "osenco-infra-tmm",
		"cd x && bd update bd-m5js.1 --status=in_progress >/dev/null": "bd-m5js.1",
		"cat > f <<'EOF'\n{\"command\":\"bd update bd-ab12.3 --status=in_progress\"}\nEOF": "",
		"bd show bd-ab12": "",
	} {
		tr := newTranscript(ToolClaude, "x")
		tr.bdCommand(cmd)
		if tr.sess.TaskID != want {
			t.Errorf("%q: task = %q, want %q", cmd, tr.sess.TaskID, want)
		}
	}
}
