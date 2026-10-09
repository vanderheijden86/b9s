package ui

import (
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/vanderheijden86/b9s/pkg/model"
)

// foldFixture holds an open epic with an open child, and a closed issue.
func foldFixture() []model.Issue {
	base := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	return []model.Issue{
		{ID: "f-1", Title: "Epic", Status: model.StatusOpen, IssueType: model.TypeEpic, CreatedAt: base},
		{ID: "f-2", Title: "Child", Status: model.StatusOpen, IssueType: model.TypeTask, CreatedAt: base,
			Dependencies: []*model.Dependency{{IssueID: "f-2", DependsOnID: "f-1", Type: model.DepParentChild}}},
		{ID: "f-3", Title: "Done", Status: model.StatusClosed, IssueType: model.TypeTask, CreatedAt: base},
	}
}

func visibleIDs(tree *TreeModel) map[string]bool {
	ids := map[string]bool{}
	for _, node := range tree.flatList {
		if node != nil && node.Issue != nil {
			ids[node.Issue.ID] = true
		}
	}
	return ids
}

func TestTreeReapplyingSameFilterKeepsFolds(t *testing.T) {
	tree := NewTreeModel(newTreeTestTheme())
	tree.SetSize(120, 12)
	tree.Build(foldFixture())
	tree.ApplyFilter("open")
	tree.CollapseAll()
	if visibleIDs(&tree)["f-2"] {
		t.Fatal("fixture expects the child folded away")
	}

	tree.SetAssigneeFilter("")
	tree.SetLabelFilter("")

	if visibleIDs(&tree)["f-2"] {
		t.Error("reapplying the same filter reopened a folded branch")
	}
}

func TestTreeRebuildUnderFilterKeepsFoldsAndRefreshesMatches(t *testing.T) {
	tree := NewTreeModel(newTreeTestTheme())
	tree.SetSize(120, 12)
	tree.Build(foldFixture())
	tree.ApplyFilter("open")
	tree.CollapseAll()

	issues := foldFixture()
	issues[2].Status = model.StatusOpen
	tree.Build(issues)

	ids := visibleIDs(&tree)
	if ids["f-2"] {
		t.Error("rebuild reopened a folded branch")
	}
	if !ids["f-3"] {
		t.Error("rebuild kept the old filter matches: a reopened issue stays hidden")
	}
}

func TestTreeChangingFilterOpensContextAncestors(t *testing.T) {
	tree := NewTreeModel(newTreeTestTheme())
	tree.SetSize(120, 12)
	tree.Build(foldFixture())
	tree.CollapseAll()

	tree.ApplyFilter("open")

	if !visibleIDs(&tree)["f-2"] {
		t.Error("a new filter must open the ancestors of its matches")
	}
}

func TestSetListItemsKeepsSelectedIssue(t *testing.T) {
	m := NewModel(refreshIssues(6), "")
	m.list.Select(4)
	want := m.list.SelectedItem().(IssueItem).Issue.ID

	var items []list.Item
	for _, issue := range refreshIssues(6)[2:] {
		items = append(items, IssueItem{Issue: issue})
	}
	m.setListItems(items)

	if got := m.list.SelectedItem().(IssueItem).Issue.ID; got != want {
		t.Errorf("selection moved from %s to %s", want, got)
	}
}
