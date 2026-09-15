package main_test

import (
	"fmt"
	"testing"
	"time"
)

// TestTreeViewJumpToBottomDrawsLastChild verifies that G scrolls far enough to
// draw the final row under a sticky parent line. The fixture overflows the 40
// row terminal, so one end of the child list is drawn only after the jump, and
// the stream contains both first and last child titles only when that row is
// actually rendered.
func TestTreeViewJumpToBottomDrawsLastChild(t *testing.T) {
	tempDir := t.TempDir()
	now := time.Now()
	issues := []treeFixtureIssue{
		{ID: "epic-s", Title: "Scroll Epic", Status: "open", Priority: 1, IssueType: "epic", CreatedAt: now.Format(time.RFC3339)},
	}
	for i := 1; i <= 60; i++ {
		id := fmt.Sprintf("child-%02d", i)
		issues = append(issues, treeFixtureIssue{
			ID: id, Title: fmt.Sprintf("Scroll Child %02d", i), Status: "open", Priority: 2, IssueType: "task",
			CreatedAt:    now.Add(time.Duration(i) * time.Second).Format(time.RFC3339),
			Dependencies: []*treeFixtureDep{{IssueID: id, DependsOnID: "epic-s", Type: "parent-child"}},
		})
	}
	writeTreeFixture(t, tempDir, issues)

	out, err := runTreeTUI(t, tempDir, 3000, []keyStep{
		k("G"),
	})
	if err != nil {
		t.Fatalf("TUI run failed: %v\noutput:\n%s", err, out)
	}

	containsAll(t, out, []string{"Scroll Child 01", "Scroll Child 60"})
}
