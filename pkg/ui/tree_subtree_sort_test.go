package ui

import (
	"testing"
	"time"

	"github.com/vanderheijden86/b9s/pkg/model"
)

func subtreeSortIssues(now time.Time) []model.Issue {
	child := func(id, parent string, created, updated time.Time) model.Issue {
		return model.Issue{
			ID: id, Title: id, IssueType: model.TypeTask, Status: model.StatusOpen,
			CreatedAt: created, UpdatedAt: updated,
			Dependencies: []*model.Dependency{{IssueID: id, DependsOnID: parent, Type: model.DepParentChild}},
		}
	}
	old := now.Add(-13 * time.Hour)
	return []model.Issue{
		{ID: "epic-old", Title: "epic-old", IssueType: model.TypeEpic, Status: model.StatusOpen, CreatedAt: old, UpdatedAt: old},
		child("task", "epic-old", old, old),
		child("sub", "task", now.Add(-3*time.Minute), now.Add(-3*time.Minute)),
		{ID: "epic-mid", Title: "epic-mid", IssueType: model.TypeEpic, Status: model.StatusOpen,
			CreatedAt: now.Add(-1 * time.Hour), UpdatedAt: now.Add(-1 * time.Hour)},
		{ID: "lone", Title: "lone", IssueType: model.TypeTask, Status: model.StatusOpen,
			CreatedAt: now.Add(-2 * time.Hour), UpdatedAt: now.Add(-2 * time.Hour)},
	}
}

func treeRootIDs(tree *TreeModel) []string {
	var ids []string
	for _, n := range tree.roots {
		ids = append(ids, n.Issue.ID)
	}
	return ids
}

func assertOrder(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

// A grandchild created minutes ago lifts its old epic above newer roots.
func TestTreeSortCreatedRanksRootsByNewestDescendant(t *testing.T) {
	tree := NewTreeModel(newTreeTestTheme())
	tree.Build(subtreeSortIssues(time.Now()))
	tree.SetSort(SortFieldCreated, SortDescending)

	assertOrder(t, treeRootIDs(&tree), "epic-old", "epic-mid", "lone")
}

func TestTreeSortUpdatedRanksRootsByNewestDescendant(t *testing.T) {
	now := time.Now()
	issues := subtreeSortIssues(now)
	for i := range issues {
		if issues[i].ID == "sub" {
			issues[i].CreatedAt = now.Add(-20 * time.Hour)
		}
	}
	tree := NewTreeModel(newTreeTestTheme())
	tree.Build(issues)
	tree.SetSort(SortFieldUpdated, SortDescending)

	assertOrder(t, treeRootIDs(&tree), "epic-old", "epic-mid", "lone")
}

// Ascending puts the stalest subtree first, so a fresh descendant moves the
// epic to the end rather than leaving it at its own old date.
func TestTreeSortAscendingUsesNewestDescendantToo(t *testing.T) {
	tree := NewTreeModel(newTreeTestTheme())
	tree.Build(subtreeSortIssues(time.Now()))
	tree.SetSort(SortFieldUpdated, SortAscending)

	assertOrder(t, treeRootIDs(&tree), "lone", "epic-mid", "epic-old")
}

// Siblings below the root rank by their own subtree as well.
func TestTreeSortUpdatedRanksNestedSiblingsBySubtree(t *testing.T) {
	now := time.Now()
	issues := subtreeSortIssues(now)
	issues = append(issues, model.Issue{
		ID: "task-newer", Title: "task-newer", IssueType: model.TypeTask, Status: model.StatusOpen,
		CreatedAt: now.Add(-1 * time.Hour), UpdatedAt: now.Add(-1 * time.Hour),
		Dependencies: []*model.Dependency{{IssueID: "task-newer", DependsOnID: "epic-old", Type: model.DepParentChild}},
	})
	tree := NewTreeModel(newTreeTestTheme())
	tree.Build(issues)
	tree.SetSort(SortFieldUpdated, SortDescending)

	var kids []string
	for _, n := range tree.roots[0].Children {
		kids = append(kids, n.Issue.ID)
	}
	assertOrder(t, kids, "task", "task-newer")
}

// The flat list has no hierarchy, so each issue ranks by its own date.
func TestFlatListSortsByOwnDate(t *testing.T) {
	tree := NewTreeModel(newTreeTestTheme())
	tree.Build(subtreeSortIssues(time.Now()))
	tree.SetSort(SortFieldCreated, SortDescending)

	var ids []string
	for _, n := range tree.buildFlatNodes() {
		ids = append(ids, n.Issue.ID)
	}
	assertOrder(t, ids, "sub", "epic-mid", "lone", "task", "epic-old")
}
