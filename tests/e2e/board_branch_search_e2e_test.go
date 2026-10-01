package main_test

import (
	"strings"
	"testing"
	"time"
)

// A query that matches an epic keeps the epic's children on the board, also a
// child whose ID and title do not match, as the tree does (bd-e5u3.19).
func TestBoardSearchHitOnEpicShowsItsChildrenE2E(t *testing.T) {
	tempDir := t.TempDir()
	fixture := makeBranchSearchFixture()
	fixture = append(fixture, treeFixtureIssue{
		ID: "other-9", Title: "Measure models", Status: "in_progress", Priority: 1, IssueType: "task",
		CreatedAt:    time.Now().Format(time.RFC3339),
		Dependencies: []*treeFixtureDep{{IssueID: "other-9", DependsOnID: "epic-1", Type: "parent-child"}},
	})
	writeTreeFixture(t, tempDir, fixture)

	out, err := runTreeTUI(t, tempDir, 3500, []keyStep{
		kd("/", 150*time.Millisecond),
		kd("quokka", 200*time.Millisecond),
		kd("\r", 300*time.Millisecond),
		kd("b", 800*time.Millisecond),
	})
	if err != nil {
		t.Fatalf("TUI run failed: %v\noutput:\n%s", err, out)
	}

	s := string(out)
	start := strings.LastIndex(s, "BOARD")
	if start < 0 {
		t.Fatalf("board was not rendered\noutput:\n%s", s)
	}
	frame := s[start:]
	for _, want := range []string{"Measure models", "in progress 1"} {
		if !strings.Contains(frame, want) {
			t.Errorf("board frame lacks %q\noutput:\n%s", want, frame)
		}
	}
	if strings.Contains(frame, "Rotate release keys") {
		t.Errorf("board shows an issue outside the hit's branch\noutput:\n%s", frame)
	}
}
