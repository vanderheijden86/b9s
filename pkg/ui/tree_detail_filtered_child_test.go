package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/vanderheijden86/b9s/pkg/model"
	tea "github.com/charmbracelet/bubbletea"
)

// A search can keep a branch in the tree whose children do not match the
// query themselves. Enter on such a child must show that child, not the row
// the cursor last passed that the query does match.
func TestTreeEnterOnChildOutsideQueryShowsChild(t *testing.T) {
	t.Setenv("B9S_TEST_MODE", "1")
	now := time.Now()
	issues := []model.Issue{
		{ID: "doc-25s", Title: "Epic: photo defects", Status: model.StatusOpen,
			IssueType: model.TypeEpic, Priority: 1, CreatedAt: now},
		{ID: "doc-25s.1", Title: "Feature: clean photo dataset", Status: model.StatusOpen,
			IssueType: model.TypeFeature, Priority: 2, CreatedAt: now.Add(-time.Second),
			Dependencies: []*model.Dependency{{IssueID: "doc-25s.1", DependsOnID: "doc-25s", Type: model.DepParentChild}}},
		{ID: "doc-3wm.7.8", Title: "Detect and fix photo orientation", Status: model.StatusOpen,
			IssueType: model.TypeTask, Priority: 2, CreatedAt: now.Add(-2 * time.Second),
			Description: "Orientation child body",
			Dependencies: []*model.Dependency{{IssueID: "doc-3wm.7.8", DependsOnID: "doc-25s.1", Type: model.DepParentChild}}},
	}
	m := NewModel(issues, "")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 160, Height: 40})
	m = updated.(Model)
	m.treeViewActive = true
	m.setQueryText("doc-25s")
	m.tree.ExpandAll()

	if !m.tree.SelectByID("doc-25s.1") {
		t.Fatalf("feature not in tree")
	}
	m.syncTreeToDetail()
	if !m.tree.SelectByID("doc-3wm.7.8") {
		t.Fatalf("child outside the query is not in the tree; test premise broken")
	}
	m = pressKeys(m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.focused != focusDetail {
		t.Fatalf("focus = %v, want detail", m.focused)
	}
	got := stripANSI(m.viewport.View())
	if !strings.Contains(got, "Orientation child body") {
		t.Fatalf("detail does not show the selected child:\n%s", got)
	}
	if sel := m.detailIssue(); sel == nil || sel.ID != "doc-3wm.7.8" {
		t.Fatalf("detailIssue = %v, want doc-3wm.7.8", sel)
	}
}
