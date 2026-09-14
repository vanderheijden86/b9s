package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vanderheijden86/beadwork/pkg/model"
)

// refreshIssues returns n flat tasks, newest first under the default sort.
func refreshIssues(n int) []model.Issue {
	base := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	issues := make([]model.Issue, n)
	for i := range issues {
		issues[i] = model.Issue{
			ID:        fmt.Sprintf("r-%02d", i),
			Title:     fmt.Sprintf("Row %02d", i),
			Status:    model.StatusOpen,
			IssueType: model.TypeTask,
			Priority:  2,
			CreatedAt: base.Add(-time.Duration(i) * time.Minute),
		}
	}
	return issues
}

func withoutIssue(issues []model.Issue, id string) []model.Issue {
	out := make([]model.Issue, 0, len(issues))
	for _, issue := range issues {
		if issue.ID != id {
			out = append(out, issue)
		}
	}
	return out
}

func newScrolledTree(t *testing.T, issues []model.Issue, row int) *TreeModel {
	t.Helper()
	tree := NewTreeModel(newTreeTestTheme())
	tree.SetSize(120, 12)
	tree.Build(issues)
	for i := 0; i < row; i++ {
		tree.MoveDown()
	}
	if tree.cursor != row {
		t.Fatalf("fixture expects cursor on row %d, got %d", row, tree.cursor)
	}
	if tree.viewportOffset == 0 {
		t.Fatal("fixture expects the tree to be scrolled")
	}
	return &tree
}

func TestTreeRebuildKeepsSelectedIssue(t *testing.T) {
	issues := refreshIssues(40)
	tree := newScrolledTree(t, issues, 30)
	selected := tree.GetSelectedID()

	tree.Build(refreshIssues(40))

	if got := tree.GetSelectedID(); got != selected {
		t.Fatalf("rebuild must keep the cursor on %s, got %q (row %d)", selected, got, tree.cursor)
	}
}

func TestTreeRebuildKeepsScrollPosition(t *testing.T) {
	tree := newScrolledTree(t, refreshIssues(40), 30)
	offset := tree.viewportOffset

	tree.Build(refreshIssues(40))

	if tree.viewportOffset != offset {
		t.Fatalf("rebuild must not scroll: offset was %d, now %d", offset, tree.viewportOffset)
	}
	if visible := tree.effectiveVisibleCount(); tree.cursor < tree.viewportOffset || tree.cursor >= tree.viewportOffset+visible {
		t.Fatalf("cursor row %d must stay inside the visible rows %d-%d", tree.cursor, tree.viewportOffset, tree.viewportOffset+visible-1)
	}
}

func TestTreeRebuildKeepsRowWhenSelectedIssueDisappears(t *testing.T) {
	issues := refreshIssues(40)
	tree := newScrolledTree(t, issues, 30)
	gone := tree.GetSelectedID()

	tree.Build(withoutIssue(issues, gone))

	if tree.cursor != 30 {
		t.Fatalf("cursor must stay on row 30 when %s disappears, got row %d", gone, tree.cursor)
	}
}

func TestTreeRebuildAfterEmptyStartsAtTop(t *testing.T) {
	tree := newScrolledTree(t, refreshIssues(40), 30)

	tree.Build(nil) // project switch clears the tree before loading the next project
	tree.Build(refreshIssues(40))

	if tree.cursor != 0 || tree.viewportOffset != 0 {
		t.Fatalf("a tree rebuilt from empty must start at the top, got row %d offset %d", tree.cursor, tree.viewportOffset)
	}
}

func TestFileChangedReloadKeepsTreeCursor(t *testing.T) {
	issues := refreshIssues(40)
	var lines []string
	for _, issue := range issues {
		lines = append(lines, fmt.Sprintf(`{"id":%q,"title":%q,"status":"open","issue_type":"task","priority":2,"created_at":%q}`,
			issue.ID, issue.Title, issue.CreatedAt.Format(time.RFC3339)))
	}
	beads := filepath.Join(t.TempDir(), "beads.jsonl")
	if err := os.WriteFile(beads, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write beads: %v", err)
	}

	m := NewModel(issues, beads)
	if m.watcher != nil {
		defer m.watcher.Stop()
	}
	m.width, m.height = 120, 20
	m.tree.SetSize(m.width, m.bodyHeight())
	if m.focused != focusTree {
		t.Fatalf("expected tree focus, got %v", m.focused)
	}
	if !m.tree.SelectByID("r-30") {
		t.Fatal("fixture issue r-30 not in tree")
	}

	updated, _ := m.Update(FileChangedMsg{})
	m = updated.(Model)

	if m.statusIsError {
		t.Fatalf("reload failed: %s", m.statusMsg)
	}
	if got := m.tree.GetSelectedID(); got != "r-30" {
		t.Fatalf("live reload must keep the tree cursor on r-30, got %q", got)
	}
}
