package main_test

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

func TestMermaidDetailRenderingE2E(t *testing.T) {
	dir := t.TempDir()
	writeTreeFixture(t, dir, []treeFixtureIssue{{
		ID: "diagram-1", Title: "Diagram issue", Status: "open", Priority: 2, IssueType: "task",
		CreatedAt: time.Now().Format(time.RFC3339),
		Description: "```mermaid\ngraph LR\nA --> B\n```\n\n" +
			"```mermaid\nsequenceDiagram\nAlice->>Bob: Hello\n```\n\n" +
			"```mermaid\nclassDiagram\nAnimal <|-- Duck\n```",
	}})

	out, err := runTreeTUI(t, dir, 2500, []keyStep{kd("\r", 200*time.Millisecond)})
	if err != nil {
		t.Fatalf("detail TUI run failed: %v\n%s", err, out)
	}
	frame := treeFinalFrame(out)
	for _, want := range []string{"DESCRIPTION", "┌", "►", "Hello", "classDiagram", "Animal <|-- Duck"} {
		if !strings.Contains(frame, want) {
			t.Fatalf("detail missing %q:\n%s", want, frame)
		}
	}
	for _, source := range []string{"graph LR", "sequenceDiagram"} {
		if strings.Contains(frame, source) {
			t.Fatalf("rendered diagram retained %q:\n%s", source, frame)
		}
	}
	for _, participant := range []string{"Alice", "Bob"} {
		if strings.Count(frame, participant) != 2 {
			t.Fatalf("sequence participant %q needs top and bottom boxes:\n%s", participant, frame)
		}
	}
}

func TestMermaidSequenceProofE2E(t *testing.T) {
	dir := t.TempDir()
	writeTreeFixture(t, dir, []treeFixtureIssue{{
		ID: "bd-uv2t", Title: "Sequence proof", Status: "open", Priority: 2, IssueType: "task",
		CreatedAt:   time.Now().Format(time.RFC3339),
		Description: "```mermaid\nsequenceDiagram\n    participant User\n    participant b9s\n    User->>b9s: Open detail\n    b9s-->>User: Render diagram\n```",
	}})

	out, err := runTreeTUI(t, dir, 2500, []keyStep{kd("\r", 200*time.Millisecond)})
	if err != nil {
		t.Fatalf("detail TUI run failed: %v\n%s", err, out)
	}
	frame := ansi.Strip(treeFinalFrame(out))
	want := []string{
		"┌──────┐     ┌─────┐",
		"│ User │     │ b9s │",
		"└───┬──┘     └──┬──┘",
		"    │           │",
		"    │ Open detail",
		"    ├──────────►│",
		"    │           │",
		"    │ Render diagram",
		"    │◄┈┈┈┈┈┈┈┈┈┈┤",
		"    │           │",
		"┌───┴──┐     ┌──┴──┐",
		"│ User │     │ b9s │",
		"└──────┘     └─────┘",
	}
	rows := strings.Split(frame, "\n")
	found := false
	for start := 0; start+len(want) <= len(rows); start++ {
		column := strings.Index(rows[start], want[0])
		if column < 0 {
			continue
		}
		found = true
		for i, line := range want {
			if len(rows[start+i]) < column || !strings.HasPrefix(rows[start+i][column:], line) {
				found = false
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		t.Fatalf("detail must show connected participant boxes, lifelines and both messages:\n%s", frame)
	}
	if strings.Contains(frame, "sequenceDiagram") || strings.Contains(frame, "User->>b9s") {
		t.Fatalf("detail retained Mermaid source:\n%s", frame)
	}
	t.Logf("Sequence proof terminal frame:\n%s", frame)
}
