package ui

import (
	"testing"
	"time"

	"github.com/vanderheijden86/beadwork/pkg/model"
)

// refreshExpansionIssues returns a fresh four-level chain on every call, the
// way a reload hands the tree newly allocated issues rather than the ones it
// was built from.
func refreshExpansionIssues() []model.Issue {
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	child := func(id, parent string, offset time.Duration) model.Issue {
		return model.Issue{
			ID: id, Title: id, Priority: 2, IssueType: model.TypeTask, CreatedAt: now.Add(offset),
			Dependencies: []*model.Dependency{{IssueID: id, DependsOnID: parent, Type: model.DepParentChild}},
		}
	}
	return []model.Issue{
		{ID: "epic", Title: "epic", Priority: 1, IssueType: model.TypeEpic, CreatedAt: now},
		child("task", "epic", time.Hour),
		child("subtask", "task", 2*time.Hour),
		child("leaf", "subtask", 3*time.Hour),
	}
}

// expandToLeaf opens every level so the deepest child is visible.
func expandToLeaf(t *testing.T, tree *TreeModel) {
	t.Helper()
	for _, id := range []string{"task", "subtask"} {
		if !tree.SelectByID(id) {
			t.Fatalf("%s not visible while expanding", id)
		}
		tree.ToggleExpand()
	}
	if tree.NodeCount() != 4 {
		t.Fatalf("expected all 4 levels visible after expanding, got %d", tree.NodeCount())
	}
}

func refreshSnapshot(hash string) *DataSnapshot {
	issues := refreshExpansionIssues()
	roots, nodeMap := buildIssueTreeNodes(issues)
	issueMap := make(map[string]*model.Issue, len(issues))
	for i := range issues {
		issueMap[issues[i].ID] = &issues[i]
	}
	return &DataSnapshot{Issues: issues, IssueMap: issueMap, TreeRoots: roots, TreeNodeMap: nodeMap, DataHash: hash}
}

// No beads directory is set: a Dolt project without an issues.jsonl has no
// tree-state.json, so the refresh cannot lean on the persisted state.
func TestTreeBuildKeepsExpansionOnRefresh(t *testing.T) {
	tree := NewTreeModel(newTreeTestTheme())
	tree.Build(refreshExpansionIssues())
	expandToLeaf(t, &tree)

	tree.Build(refreshExpansionIssues())

	if tree.NodeCount() != 4 {
		t.Errorf("expected expansion down to the leaf to survive a refresh, got %d visible nodes", tree.NodeCount())
	}
}

func TestTreeBuildFromSnapshotKeepsExpansionOnRefresh(t *testing.T) {
	tree := NewTreeModel(newTreeTestTheme())
	tree.BuildFromSnapshot(refreshSnapshot("first"))
	expandToLeaf(t, &tree)

	tree.BuildFromSnapshot(refreshSnapshot("second"))

	if tree.NodeCount() != 4 {
		t.Errorf("expected expansion down to the leaf to survive a snapshot refresh, got %d visible nodes", tree.NodeCount())
	}
}

func TestTreeBuildKeepsCollapsedRootOnRefresh(t *testing.T) {
	tree := NewTreeModel(newTreeTestTheme())
	tree.Build(refreshExpansionIssues())
	if !tree.SelectByID("epic") {
		t.Fatal("epic not visible")
	}
	tree.ToggleExpand()
	if tree.NodeCount() != 1 {
		t.Fatalf("expected only the collapsed root visible, got %d", tree.NodeCount())
	}

	tree.Build(refreshExpansionIssues())

	if tree.NodeCount() != 1 {
		t.Errorf("expected the collapsed root to stay collapsed after a refresh, got %d visible nodes", tree.NodeCount())
	}
}

// A project switch empties the tree first; the next project must open with
// default expansion, not with whatever the previous project had open.
func TestTreeBuildAfterEmptyTreeUsesDefaultExpansion(t *testing.T) {
	tree := NewTreeModel(newTreeTestTheme())
	tree.Build(refreshExpansionIssues())
	expandToLeaf(t, &tree)

	tree.Build(nil)
	tree.Build(refreshExpansionIssues())

	if tree.NodeCount() != 2 {
		t.Errorf("expected default expansion (root and its children) after the tree was emptied, got %d visible nodes", tree.NodeCount())
	}
}
