package main_test

import (
	"testing"
	"time"
)

func makeMarkFixture() []treeFixtureIssue {
	now := time.Now()
	return []treeFixtureIssue{
		{ID: "mark-1", Title: "Mark Alpha", Status: "open", Priority: 2, IssueType: "task", CreatedAt: now.Format(time.RFC3339)},
		{ID: "mark-2", Title: "Mark Beta", Status: "open", Priority: 2, IssueType: "task", CreatedAt: now.Add(time.Second).Format(time.RFC3339)},
		{ID: "mark-3", Title: "Mark Gamma", Status: "open", Priority: 2, IssueType: "task", CreatedAt: now.Add(2 * time.Second).Format(time.RFC3339)},
	}
}

func TestTreeViewSpaceMarksShowCount(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PTY e2e test in -short mode")
	}
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, makeMarkFixture())

	out, err := runTreeTUI(t, tempDir, 1500, []keyStep{
		k(" "), k("j"), k(" "),
	})
	if err != nil {
		t.Fatalf("run tree TUI: %v", err)
	}
	containsAll(t, out, []string{"2 marked"})
}

func TestTreeViewShiftKConfirmsCloseOfMarkedIssues(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PTY e2e test in -short mode")
	}
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, makeMarkFixture())

	// Space, down, ctrl+space (NUL) marks two rows; K must target both.
	out, err := runTreeTUI(t, tempDir, 1800, []keyStep{
		k(" "), k("j"), k("\x00"), k("K"),
	})
	if err != nil {
		t.Fatalf("run tree TUI: %v", err)
	}
	containsAll(t, out, []string{"Close 2 issues?", "[Y] Close 2"})
}
