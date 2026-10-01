package main_test

import (
	"strings"
	"testing"
	"time"
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
}
