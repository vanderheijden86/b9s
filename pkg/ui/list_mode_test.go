// list_mode_test.go - The flat list view: every issue on its own row, sorted
// across the whole hierarchy, and forced by a type: predicate (bd-9faq).
package ui

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanderheijden86/b9s/pkg/model"
)

// listModeIssues nests the most recently updated issue two levels deep, below
// an epic that has not changed for days, so a tree sorted by Updated desc
// cannot show it first and a flat list must.
func listModeIssues() []model.Issue {
	now := time.Now()
	parent := func(child, parent string) []*model.Dependency {
		return []*model.Dependency{{IssueID: child, DependsOnID: parent, Type: model.DepParentChild}}
	}
	return []model.Issue{
		{ID: "lm-epic", Title: "Old epic", Status: model.StatusOpen, Priority: 2, IssueType: model.TypeEpic,
			CreatedAt: now.Add(-72 * time.Hour), UpdatedAt: now.Add(-72 * time.Hour)},
		{ID: "lm-epic.1", Title: "Middle task", Status: model.StatusOpen, Priority: 2, IssueType: model.TypeTask,
			CreatedAt: now.Add(-71 * time.Hour), UpdatedAt: now.Add(-48 * time.Hour), Dependencies: parent("lm-epic.1", "lm-epic")},
		{ID: "lm-epic.1.1", Title: "Deep fresh task", Status: model.StatusOpen, Priority: 2, IssueType: model.TypeTask,
			CreatedAt: now.Add(-70 * time.Hour), UpdatedAt: now, Dependencies: parent("lm-epic.1.1", "lm-epic.1")},
		{ID: "lm-bug", Title: "Root bug", Status: model.StatusOpen, Priority: 1, IssueType: model.TypeBug,
			CreatedAt: now.Add(-30 * time.Hour), UpdatedAt: now.Add(-24 * time.Hour)},
	}
}

func newListModeModel(t *testing.T) Model {
	t.Helper()
	m := NewModel(listModeIssues(), "")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = updated.(Model)
	m.tree.SetBeadsDir(filepath.Join(t.TempDir(), ".beads"))
	m.tree.ExpandAll()
	m.tree.SetSort(SortFieldUpdated, SortDescending)
	m.focused = focusTree
	return m
}

func TestListModeKeyTSortsAcrossTheHierarchy(t *testing.T) {
	m := newListModeModel(t)
	if got := treeVisibleIDs(&m.tree); got[0] == "lm-epic.1.1" {
		t.Fatalf("precondition: the tree must not put the nested issue first, got %v", got)
	}

	m = typeKeys(m, "t")

	want := []string{"lm-epic.1.1", "lm-bug", "lm-epic.1", "lm-epic"}
	if got := treeVisibleIDs(&m.tree); !slices.Equal(got, want) {
		t.Errorf("list rows = %v, want %v", got, want)
	}
	if !m.TreeListMode() {
		t.Error("TreeListMode() = false after t")
	}
	if view := m.tree.View(); !strings.Contains(view, "[LIST]") {
		t.Error("list mode view lacks the [LIST] badge")
	}

	m = typeKeys(m, "t")

	if got := treeVisibleIDs(&m.tree); got[0] != "lm-epic" && got[0] != "lm-bug" {
		t.Errorf("t again should restore the tree, rows = %v", got)
	}
	if m.TreeListMode() {
		t.Error("TreeListMode() = true after the second t")
	}
}

func TestListModeShowsOnlyQueryHitsWithoutContextRows(t *testing.T) {
	m := newListModeModel(t)
	m = typeKeys(m, "t")

	m.setQueryText("deep")

	if got := treeVisibleIDs(&m.tree); !slices.Equal(got, []string{"lm-epic.1.1"}) {
		t.Errorf("list rows for query 'deep' = %v, want only the hit", got)
	}
}

func TestListModeHonoursLabelAssigneeAndStatusFilters(t *testing.T) {
	m := newListModeModel(t)
	m = typeKeys(m, "t")

	m.tree.ApplyFilter("closed")

	if got := treeVisibleIDs(&m.tree); len(got) != 0 {
		t.Errorf("closed filter on open issues should leave no rows, got %v", got)
	}
}

func TestTypeQueryShowsFlatListWithoutAncestors(t *testing.T) {
	for _, query := range []string{"type:task", "type:bug"} {
		t.Run(query, func(t *testing.T) {
			m := newListModeModel(t)

			m.setQueryText(query)

			for _, id := range treeVisibleIDs(&m.tree) {
				issue := m.tree.issueMap[id].Issue
				if "type:"+string(issue.IssueType) != query {
					t.Errorf("%s shows %s of type %s", query, id, issue.IssueType)
				}
			}
			if query == "type:task" {
				want := []string{"lm-epic.1.1", "lm-epic.1"}
				if got := treeVisibleIDs(&m.tree); !slices.Equal(got, want) {
					t.Errorf("rows = %v, want %v sorted by Updated desc", got, want)
				}
			}
			if !m.TreeListMode() {
				t.Error("a type: predicate must show the list")
			}
		})
	}
}

func TestTypeCommandShowsListAndAllCommandRestoresTree(t *testing.T) {
	m := newListModeModel(t)

	m = typeKeys(m, ":", "t", "a", "s", "k")
	m = pressKey(m, tea.KeyEnter)

	if got := treeVisibleIDs(&m.tree); slices.Contains(got, "lm-epic") {
		t.Errorf(":task shows the parent epic, rows = %v", got)
	}

	m = typeKeys(m, ":", "a", "l", "l")
	m = pressKey(m, tea.KeyEnter)

	if m.TreeListMode() {
		t.Error("clearing the type predicate should return to the tree")
	}
	if got := treeVisibleIDs(&m.tree); !slices.Contains(got, "lm-epic") {
		t.Errorf("tree after :all lacks the epic, rows = %v", got)
	}
}

func TestTypeQueryKeepsListChosenWithT(t *testing.T) {
	m := newListModeModel(t)
	m = typeKeys(m, "t")
	m.setQueryText("type:task")
	m.setQueryText("")

	if !m.TreeListMode() {
		t.Error("list chosen with t must survive a type: query being cleared")
	}
}

func TestNegatedTypeQueryAlsoShowsList(t *testing.T) {
	m := newListModeModel(t)

	m.setQueryText("!type:epic")

	if got := treeVisibleIDs(&m.tree); slices.Contains(got, "lm-epic") {
		t.Errorf("!type:epic shows the epic as a context row, rows = %v", got)
	}
}

func TestListModeIsRememberedPerProject(t *testing.T) {
	beadsDir := filepath.Join(t.TempDir(), ".beads")
	tree := NewTreeModel(newTreeTestTheme())
	tree.SetBeadsDir(beadsDir)
	tree.Build(listModeIssues())
	tree.ToggleFlatMode()

	tree.Build(listModeIssues())
	if !tree.IsFlatMode() {
		t.Fatal("a refresh must keep list mode")
	}

	reopened := NewTreeModel(newTreeTestTheme())
	reopened.SetBeadsDir(beadsDir)
	reopened.Build(listModeIssues())
	if !reopened.IsFlatMode() {
		t.Error("list mode chosen with t must come back when the project opens again")
	}

	other := NewTreeModel(newTreeTestTheme())
	other.SetBeadsDir(filepath.Join(t.TempDir(), ".beads"))
	other.Build(listModeIssues())
	if other.IsFlatMode() {
		t.Error("another project must start in the tree")
	}
}

func TestListModeFollowsTheProjectOnSwitch(t *testing.T) {
	listDir := filepath.Join(t.TempDir(), ".beads")
	tree := NewTreeModel(newTreeTestTheme())
	tree.SetBeadsDir(listDir)
	tree.Build(listModeIssues())
	tree.ToggleFlatMode()

	tree.SetBeadsDir(filepath.Join(t.TempDir(), ".beads"))
	tree.Build(listModeIssues())

	if tree.IsFlatMode() {
		t.Error("switching to a project without saved list mode must show the tree")
	}
}
