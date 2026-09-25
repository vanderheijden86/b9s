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
	for _, want := range []string{"RELATIONS", "parent", "Epic: Redesign the board TUI", "blocked by", "Blocker task"} {
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

func TestRenderIssueDetail_HeaderIsAHeavyBox(t *testing.T) {
	lines := strings.Split(renderDetailFixture(t, nil), "\n")
	top, bottom := -1, -1
	for i, l := range lines {
		if strings.HasPrefix(l, "┏") && top < 0 {
			top = i
		}
		if strings.HasPrefix(l, "┗") && bottom < 0 {
			bottom = i
		}
	}
	if top < 0 || bottom <= top {
		t.Fatalf("header box not found:\n%s", strings.Join(lines, "\n"))
	}
	box := strings.Join(lines[top:bottom+1], "\n")
	for _, want := range []string{"e5u3.9", "Build epic rail", "IN PROGRESS", "P1", "@alice", "@BrownBear"} {
		if !strings.Contains(box, want) {
			t.Errorf("box missing %q:\n%s", want, box)
		}
	}
	for _, l := range lines[top+1 : bottom] {
		if !strings.HasPrefix(l, "┃") || !strings.HasSuffix(strings.TrimRight(l, " "), "┃") {
			t.Errorf("box side missing: %q", l)
		}
	}
	after := strings.Join(lines[bottom+1:], "\n")
	for _, want := range []string{"alice@example.com", "#ui", "DESCRIPTION"} {
		if !strings.Contains(after, want) {
			t.Errorf("%q should follow the box:\n%s", want, after)
		}
	}
}

func TestRenderIssueDetail_ChildrenShowProgress(t *testing.T) {
	theme := DefaultTheme(lipgloss.NewRenderer(nil))
	issue, issueMap := detailFixture()
	issueMap["bd-c2"] = &model.Issue{ID: "bd-c2", Title: "Second step", Status: model.StatusOpen, IssueType: model.TypeTask,
		Dependencies: []*model.Dependency{{IssueID: "bd-c2", DependsOnID: "bd-e5u3.9", Type: model.DepParentChild}}}
	out := stripANSI(renderIssueDetail(issue, issueMap, theme, 70, NewMarkdownRendererWithTheme(70, theme), nil))
	for _, want := range []string{"CHILDREN · 1/2", "━", "Child step", "Second step"} {
		if !strings.Contains(out, want) {
			t.Errorf("children section missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "↓ child") {
		t.Errorf("children should not repeat under RELATIONS:\n%s", out)
	}
}
