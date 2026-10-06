package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/vanderheijden86/b9s/internal/attachref"
	"github.com/vanderheijden86/b9s/pkg/model"
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

func TestRenderIssueDetail_MermaidInTextSections(t *testing.T) {
	for _, section := range []string{"description", "design", "acceptance", "notes"} {
		t.Run(section, func(t *testing.T) {
			issue, issueMap := detailFixture()
			source := "```mermaid\ngraph LR\nA --> B\n```"
			switch section {
			case "description":
				issue.Description = source
			case "design":
				issue.Design = source
			case "acceptance":
				issue.AcceptanceCriteria = source
			case "notes":
				issue.Notes = source
			}
			theme := DefaultTheme(lipgloss.NewRenderer(nil))
			out := stripANSI(renderIssueDetail(issue, issueMap, theme, 70, NewMarkdownRendererWithTheme(70, theme), nil))
			if !strings.Contains(out, "┌") || strings.Contains(out, "graph LR") {
				t.Fatalf("diagram not rendered in %s:\n%s", section, out)
			}
		})
	}
}

func TestRenderIssueDetail_MermaidInComment(t *testing.T) {
	issue, issueMap := detailFixture()
	issue.Comments[0].Text = "```mermaid\nsequenceDiagram\nAlice->>Bob: Hello\n```"
	theme := DefaultTheme(lipgloss.NewRenderer(nil))
	out := stripANSI(renderIssueDetail(issue, issueMap, theme, 70, NewMarkdownRendererWithTheme(70, theme), nil))
	if !strings.Contains(out, "┌") || strings.Contains(out, "sequenceDiagram") {
		t.Fatalf("comment diagram not rendered:\n%s", out)
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

func TestRenderIssueDetail_ShowsAttachmentsBlockWithMachineLinesStripped(t *testing.T) {
	theme := DefaultTheme(lipgloss.NewRenderer(nil))
	issue, issueMap := detailFixture()
	text, err := attachref.Format(attachref.Ref{
		SHA256: strings.Repeat("a", 64), Size: 2048, Type: "image/png", Name: "diagram.png",
	})
	if err != nil {
		t.Fatalf("attachref.Format: %v", err)
	}
	issue.Comments = append(issue.Comments, &model.Comment{
		ID: "cmt-1", Author: "carol", Text: text, CreatedAt: time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC),
	})
	out := stripANSI(renderIssueDetail(issue, issueMap, theme, 70, NewMarkdownRendererWithTheme(70, theme), nil))
	for _, want := range []string{"ATTACHMENTS · 1", "diagram.png", "image/png", "2.0 KB", "added by carol"} {
		if !strings.Contains(out, want) {
			t.Errorf("attachments block missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "beads-attachment/v1") {
		t.Errorf("machine line should be stripped from the comment body:\n%s", out)
	}
}

func TestRenderIssueDetail_NoAttachmentsShowsNoBlock(t *testing.T) {
	out := renderDetailFixture(t, nil)
	if strings.Contains(out, "ATTACHMENTS") {
		t.Errorf("issue with no attachments should show no ATTACHMENTS block:\n%s", out)
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

func TestRenderIssueDetail_ExactUpdatedTimeBelowRelativeAge(t *testing.T) {
	theme := DefaultTheme(lipgloss.NewRenderer(nil))
	issue, issueMap := detailFixture()
	issue.UpdatedAt = time.Now().Add(-5 * time.Hour)
	exact := issue.UpdatedAt.Local().Format("2006-01-02 15:04")
	lines := strings.Split(stripANSI(renderIssueDetail(issue, issueMap, theme, 70, nil, nil)), "\n")
	rel := -1
	for i, l := range lines {
		if strings.Contains(l, "updated 5h ago") {
			rel = i
			break
		}
	}
	if rel < 0 || rel+1 >= len(lines) {
		t.Fatalf("relative age not found:\n%s", strings.Join(lines, "\n"))
	}
	below := lines[rel+1]
	if !strings.Contains(below, exact) {
		t.Fatalf("line below the relative age should hold %q, got %q", exact, below)
	}
	relEnd := strings.Index(lines[rel], "ago") + len("ago")
	exactEnd := strings.Index(below, exact) + len(exact)
	if lipgloss.Width(lines[rel][:relEnd]) != lipgloss.Width(below[:exactEnd]) {
		t.Errorf("exact time should be right-aligned under the relative age:\n%s\n%s", lines[rel], below)
	}
}

func TestRenderIssueDetail_ExactUpdatedTimeSurvivesLongTitle(t *testing.T) {
	theme := DefaultTheme(lipgloss.NewRenderer(nil))
	issue, issueMap := detailFixture()
	issue.UpdatedAt = time.Now().Add(-5 * time.Hour)
	issue.Title = strings.Repeat("very long title ", 12)
	out := stripANSI(renderIssueDetail(issue, issueMap, theme, 70, nil, nil))
	if exact := issue.UpdatedAt.Local().Format("2006-01-02 15:04"); !strings.Contains(out, exact) {
		t.Errorf("exact time %q missing with a long title:\n%s", exact, out)
	}
	if !strings.Contains(strings.ReplaceAll(out, "\n", " "), "very long title") {
		t.Errorf("title missing:\n%s", out)
	}
}
