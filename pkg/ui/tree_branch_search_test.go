// tree_branch_search_test.go - A text query keeps the whole branch of every
// hit visible (ancestors and descendants) and hides branches without a hit
// (bd-xkxb).
package ui

import (
	"io"
	"sort"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/vanderheijden86/beadwork/pkg/model"
)

func childOf(id, title, parent string, typ model.IssueType, status model.Status) model.Issue {
	return model.Issue{
		ID:        id,
		Title:     title,
		IssueType: typ,
		Status:    status,
		Dependencies: []*model.Dependency{
			{IssueID: id, DependsOnID: parent, Type: model.DepParentChild},
		},
	}
}

// branchSearchIssues is two epics, each with two features, plus two
// standalone tasks. Only titles containing "zephyr" or "quokka" can match.
func branchSearchIssues() []model.Issue {
	return []model.Issue{
		{ID: "epic-a", Title: "Acceptance testing", IssueType: model.TypeEpic, Status: model.StatusOpen},
		childOf("feat-a1", "Verify first implementation", "epic-a", model.TypeFeature, model.StatusOpen),
		childOf("task-a1", "Connect over the zephyr tunnel", "feat-a1", model.TypeTask, model.StatusOpen),
		childOf("feat-a2", "Deploy to the cloud", "epic-a", model.TypeFeature, model.StatusOpen),
		childOf("task-a2", "Provision the app service", "feat-a2", model.TypeTask, model.StatusClosed),
		{ID: "epic-b", Title: "Quokka migration", IssueType: model.TypeEpic, Status: model.StatusOpen},
		childOf("feat-b1", "Move the storage layer", "epic-b", model.TypeFeature, model.StatusOpen),
		childOf("task-b1", "Copy the archive tables", "feat-b1", model.TypeTask, model.StatusClosed),
		{ID: "solo-1", Title: "Standalone zephyr cleanup", IssueType: model.TypeTask, Status: model.StatusOpen},
		{ID: "solo-2", Title: "Rotate release keys", IssueType: model.TypeTask, Status: model.StatusOpen},
	}
}

func newBranchSearchModel(issues []model.Issue) Model {
	m := NewModel(issues, "")
	m.tree.Build(issues)
	return m
}

func sortedVisibleIDs(tree *TreeModel) []string {
	ids := treeVisibleIDs(tree)
	sort.Strings(ids)
	return ids
}

func assertVisibleIDs(t *testing.T, tree *TreeModel, want ...string) {
	t.Helper()
	sort.Strings(want)
	got := sortedVisibleIDs(tree)
	if len(got) != len(want) {
		t.Fatalf("visible tree IDs = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("visible tree IDs = %v, want %v", got, want)
		}
	}
}

func TestTreeQueryHitOnNestedTaskHidesSiblingBranches(t *testing.T) {
	m := newBranchSearchModel(branchSearchIssues()[:8])

	m.setQueryText("zephyr")

	assertVisibleIDs(t, &m.tree, "epic-a", "feat-a1", "task-a1")
}

func TestTreeQueryHitOnEpicShowsItsWholeSubtree(t *testing.T) {
	m := newBranchSearchModel(branchSearchIssues()[:8])

	m.setQueryText("quokka")

	assertVisibleIDs(t, &m.tree, "epic-b", "feat-b1", "task-b1")
}

func TestTreeQueryHitOnStandaloneTaskShowsOnlyThatTask(t *testing.T) {
	issues := branchSearchIssues()
	m := newBranchSearchModel(append(issues[5:8:8], issues[8], issues[9]))

	m.setQueryText("zephyr")

	assertVisibleIDs(t, &m.tree, "solo-1")
}

func TestTreeQueryKeepsBranchOfEveryHit(t *testing.T) {
	m := newBranchSearchModel(branchSearchIssues())

	m.setQueryText("zephyr")

	assertVisibleIDs(t, &m.tree, "epic-a", "feat-a1", "task-a1", "solo-1")
}

func TestTreeQueryDimsRevealedDescendants(t *testing.T) {
	m := newBranchSearchModel(branchSearchIssues())

	m.setQueryText("quokka")

	if m.tree.IsFilterDimmed(m.tree.issueMap["epic-b"]) {
		t.Error("the hit itself must not be dimmed")
	}
	if !m.tree.IsFilterDimmed(m.tree.issueMap["feat-b1"]) {
		t.Error("a descendant revealed as branch context must be dimmed")
	}
}

func TestTreeQueryRevealedDescendantsRespectStatusFilter(t *testing.T) {
	m := newBranchSearchModel(branchSearchIssues())
	m.tree.ApplyFilter("open")

	m.setQueryText("quokka")

	assertVisibleIDs(t, &m.tree, "epic-b", "feat-b1")
}

func TestTreeQueryRevealedBranchCollapsesWithTab(t *testing.T) {
	m := newBranchSearchModel(branchSearchIssues())
	m.setQueryText("quokka")

	m.tree.SelectByID("epic-b")
	m.tree.ToggleExpand()

	assertVisibleIDs(t, &m.tree, "epic-b")
}

func TestTreeQueryHitRowKeepsItsNormalColours(t *testing.T) {
	renderer := lipgloss.NewRenderer(io.Discard)
	renderer.SetColorProfile(termenv.TrueColor)
	issues := branchSearchIssues()

	plain := NewTreeModel(DefaultTheme(renderer))
	plain.Build(issues)
	plain.SetColumnPreference(TreeColumnLaneStage, ColumnHide)
	searched := NewTreeModel(DefaultTheme(renderer))
	searched.Build(issues)
	searched.SetColumnPreference(TreeColumnLaneStage, ColumnHide)
	searched.SetIssueQuery(ParseIssueQuery("zephyr"))

	want := plain.renderNode(plain.issueMap["solo-1"], false, 10)
	got := searched.renderNode(searched.issueMap["solo-1"], false, 10)
	if got != want {
		t.Errorf("hit row is restyled by search:\n got %q\nwant %q", got, want)
	}
}

func TestTreeStatusFilterAloneDoesNotRevealDescendants(t *testing.T) {
	issues := []model.Issue{
		{ID: "epic-c", Title: "Closed epic", IssueType: model.TypeEpic, Status: model.StatusClosed},
		childOf("task-c1", "Open leftover", "epic-c", model.TypeTask, model.StatusOpen),
	}
	m := newBranchSearchModel(issues)

	m.tree.ApplyFilter("closed")

	assertVisibleIDs(t, &m.tree, "epic-c")
}
