package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/vanderheijden86/beadwork/pkg/model"
)

func detailFixture() (model.Issue, map[string]*model.Issue) {
	parent := &model.Issue{ID: "bd-e5u3", Title: "Epic: Redesign the board TUI", Status: model.StatusOpen, IssueType: model.TypeEpic}
	blocker := &model.Issue{ID: "bd-x1", Title: "Blocker task", Status: model.StatusInProgress, IssueType: model.TypeTask}
	issue := model.Issue{
		ID: "bd-e5u3.9", Title: "Build epic rail", Status: model.StatusInProgress,
		IssueType: model.TypeTask, Priority: 1, CreatedAt: time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC),
		CreatedBy: "alice", Owner: "alice@example.com", Assignee: "BrownBear",
		Labels:      []string{"ui", "board"},
		Description: "Cards become boxed.",
		Dependencies: []*model.Dependency{
			{IssueID: "bd-e5u3.9", DependsOnID: "bd-e5u3", Type: model.DepParentChild},
			{IssueID: "bd-e5u3.9", DependsOnID: "bd-x1", Type: model.DepBlocks},
		},
		Comments: []*model.Comment{{Author: "bob", Text: "looks good", CreatedAt: time.Now()}},
	}
	child := &model.Issue{ID: "bd-c1", Title: "Child step", Status: model.StatusClosed, IssueType: model.TypeTask,
		Dependencies: []*model.Dependency{{IssueID: "bd-c1", DependsOnID: "bd-e5u3.9", Type: model.DepParentChild}}}
	issueMap := map[string]*model.Issue{parent.ID: parent, blocker.ID: blocker, child.ID: child, issue.ID: &issue}
	return issue, issueMap
}

func renderDetailFixture(t *testing.T, update *detailUpdateNotice) string {
	t.Helper()
	theme := DefaultTheme(lipgloss.NewRenderer(nil))
	issue, issueMap := detailFixture()
	md := NewMarkdownRendererWithTheme(70, theme)
	return stripANSI(renderIssueDetail(issue, issueMap, theme, 70, md, update))
}

func TestRenderIssueDetail_HeaderShowsMetaWithoutTable(t *testing.T) {
	out := renderDetailFixture(t, nil)
	for _, want := range []string{"e5u3.9", "Build epic rail", "IN PROGRESS", "P1", "creator", "@alice", "assignee", "@BrownBear", "alice@example.com", "2026-09-25", "ui", "board"} {
		if !strings.Contains(out, want) {
			t.Errorf("detail header missing %q:\n%s", want, out)
		}
	}
	for _, unwanted := range []string{"| ID |", "mailto:", "⭐", "⚡"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("detail should not contain %q:\n%s", unwanted, out)
		}
	}
}

func TestRenderIssueDetail_ListsRelationsBothWays(t *testing.T) {
	out := renderDetailFixture(t, nil)
	for _, want := range []string{"RELATIONS", "parent", "Epic: Redesign the board TUI", "blocked by", "Blocker task", "child", "Child step"} {
		if !strings.Contains(out, want) {
			t.Errorf("relations missing %q:\n%s", want, out)
		}
	}
}

func TestRenderIssueDetail_SectionsAndComments(t *testing.T) {
	out := renderDetailFixture(t, nil)
	for _, want := range []string{"DESCRIPTION", "Cards become boxed.", "COMMENTS", "bob", "looks good"} {
		if !strings.Contains(out, want) {
			t.Errorf("detail missing %q:\n%s", want, out)
		}
	}
}

func TestRenderIssueDetail_UpdateNoticeIsOneQuietLine(t *testing.T) {
	out := renderDetailFixture(t, &detailUpdateNotice{Tag: "v1.1.0"})
	if !strings.Contains(out, "v1.1.0 available") {
		t.Fatalf("update notice missing:\n%s", out)
	}
	if strings.Contains(out, "⭐") || strings.Contains(out, "https://") {
		t.Fatalf("update notice should be a single quiet line:\n%s", out)
	}
}

func TestRenderIssueDetail_LinesFitWidth(t *testing.T) {
	theme := DefaultTheme(lipgloss.NewRenderer(nil))
	issue, issueMap := detailFixture()
	issue.Title = strings.Repeat("very long title ", 12)
	out := renderIssueDetail(issue, issueMap, theme, 40, NewMarkdownRendererWithTheme(40, theme), nil)
	for _, line := range strings.Split(out, "\n") {
		if w := lipgloss.Width(line); w > 40 {
			t.Fatalf("line wider than pane (%d > 40): %q", w, stripANSI(line))
		}
	}
}
