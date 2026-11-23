package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/vanderheijden86/b9s/pkg/model"
)

// A row wider than the pane must be cut, not wrapped: a wrapped row pushes
// every row under it down and breaks the viewport's one-line-per-row budget.
func TestTreeRowsNeverWrapInNarrowPane(t *testing.T) {
	now := time.Now()
	issues := []model.Issue{
		{ID: "hl-items", Title: "open items epic", Status: model.StatusOpen, IssueType: model.TypeEpic, CreatedAt: now},
		{ID: "hl-items.5", Title: "does the thing", Status: model.StatusOpen, IssueType: model.TypeTask, CreatedAt: now,
			Dependencies: []*model.Dependency{{IssueID: "hl-items.5", DependsOnID: "hl-items", Type: model.DepParentChild}}},
	}
	tree := NewTreeModel(newTreeTestTheme())
	tree.Build(issues)
	tree.ExpandAll()
	tree.SetSize(63, 20)

	view := stripANSI(tree.View())
	lines := strings.Split(view, "\n")
	for _, line := range lines {
		if w := lipgloss.Width(line); w > 63 {
			t.Fatalf("line is %d cells wide, pane is 63: %q", w, line)
		}
		if strings.TrimSpace(line) == "items.5" {
			t.Fatalf("child ID wrapped onto its own line:\n%s", view)
		}
	}
}
