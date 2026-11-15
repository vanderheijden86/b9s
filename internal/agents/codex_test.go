package agents

import (
	"strings"
	"testing"
)

func TestCodexTranscriptSummary(t *testing.T) {
	tr := parseFixture(t, ToolCodex, "codex.jsonl")
	s := tr.sess
	if s.ID != "cx-1" || s.Project != "omnigent-config" || s.Branch != "lane/bfe.51" {
		t.Fatalf("id/project/branch = %q/%q/%q", s.ID, s.Project, s.Branch)
	}
	if s.Model != "gpt-6-astra" || s.TaskID != "omnigent-config-bfe.51" || s.Role != "implementer" {
		t.Errorf("model/task/role = %q/%q/%q", s.Model, s.TaskID, s.Role)
	}
	if s.ContextTokens != 77627 || s.ContextLimit != 258400 {
		t.Errorf("context = %d/%d", s.ContextTokens, s.ContextLimit)
	}
	if s.Compactions != 1 || tr.lastStop != "task_complete" {
		t.Errorf("compactions/stop = %d/%q", s.Compactions, tr.lastStop)
	}
	if s.Tools != 2 {
		t.Errorf("tools = %d", s.Tools)
	}
}

func TestCodexTranscriptEventsAndSources(t *testing.T) {
	tr := parseFixture(t, ToolCodex, "codex.jsonl")
	var texts []string
	for _, e := range tr.events {
		texts = append(texts, string(e.Kind)+":"+e.Text)
	}
	all := strings.Join(texts, "\n")
	for _, want := range []string{"TASK:You are the implementer", "BD:bd show omnigent-config-bfe.51", "TOOL:cat /work/omnigent-config/README.md", "AI:Preview job fixed.", "CTX:context compacted"} {
		if !strings.Contains(all, want) {
			t.Errorf("missing %q in\n%s", want, all)
		}
	}
	var instr []string
	for _, s := range tr.sourceList() {
		if s.Layer == LayerInstructions {
			instr = append(instr, s.Name)
		}
	}
	if strings.Join(instr, ",") != "developer message,AGENTS.md (/work/omnigent-config)" {
		t.Errorf("instructions = %v", instr)
	}
}
