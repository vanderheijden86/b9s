package ui

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vanderheijden86/b9s/internal/attachref"
	"github.com/vanderheijden86/b9s/pkg/model"
)

func TestIssueDetailExtraSections(t *testing.T) {
	for _, tc := range []struct{ name, field, value, heading string }{
		{"close reason", "close_reason", `"Finished after review"`, "Close Reason"},
		{"metadata", "metadata", `{"branch":"feature/planning","worktree":"/tmp/planning","nested":{"count":9007199254740993}}`, "Metadata"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var issue model.Issue
			if err := json.Unmarshal([]byte(`{"id":"bd-39","title":"Planning","status":"closed","`+tc.field+`":`+tc.value+`}`), &issue); err != nil {
				t.Fatal(err)
			}
			theme := DefaultTheme(lipgloss.NewRenderer(nil))
			outputs := map[string]string{
				"TUI":      stripANSI(renderIssueDetail(issue, nil, theme, 100, NewMarkdownRendererWithTheme(100, theme), nil, nil, false)),
				"Markdown": formatIssueMarkdown(issue, nil),
			}
			for name, out := range outputs {
				heading := tc.heading
				if name == "TUI" {
					heading = strings.ToUpper(heading)
				}
				if !strings.Contains(out, heading) {
					t.Errorf("%s missing %q:\n%s", name, heading, out)
				}
				wants := []string{"Finished after review"}
				if tc.field == "metadata" {
					wants = []string{"feature/planning", "/tmp/planning", "9007199254740993"}
				}
				for _, want := range wants {
					if !strings.Contains(out, want) {
						t.Errorf("%s missing %q:\n%s", name, want, out)
					}
				}
			}
		})
	}
}

func TestIssueDetailOmitsEmptyExtraSections(t *testing.T) {
	for _, metadata := range []string{"null", "{}"} {
		var issue model.Issue
		if err := json.Unmarshal([]byte(`{"id":"bd-39","metadata":`+metadata+`}`), &issue); err != nil {
			t.Fatal(err)
		}
		theme := DefaultTheme(lipgloss.NewRenderer(nil))
		for _, out := range []string{
			renderIssueDetail(issue, nil, theme, 80, nil, nil, nil, false),
			formatIssueMarkdown(issue, nil),
		} {
			if strings.Contains(strings.ToLower(out), "close reason") || strings.Contains(strings.ToLower(out), "metadata") {
				t.Errorf("empty sections rendered:\n%s", out)
			}
		}
	}
}

func TestDetailCopyIncludesExtraDetailSections(t *testing.T) {
	var issue model.Issue
	if err := json.Unmarshal([]byte(`{"id":"bd-39","title":"Planning","status":"open","close_reason":"Finished after review","metadata":{"branch":"feature/planning"}}`), &issue); err != nil {
		t.Fatal(err)
	}
	m := NewModel([]model.Issue{issue}, "")
	m.focused = focusDetail
	var copied string
	m.clipboardWrite = func(text string) error { copied = text; return nil }
	m = pressTreeKey(m, 'c')
	for _, want := range []string{"### Close Reason", "Finished after review", "### Metadata", "feature/planning"} {
		if !strings.Contains(copied, want) {
			t.Errorf("detail copy missing %q:\n%s", want, copied)
		}
	}
}

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
	return stripANSI(renderIssueDetail(issue, issueMap, nil, theme, 70, md, update, nil, false))
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

func TestDetailCommentsNewestFirst(t *testing.T) {
	issue, _ := detailFixture()
	issue.Comments = []*model.Comment{
		{Text: "oldest-message", CreatedAt: time.Unix(1, 0)},
		{Text: "newest-message", CreatedAt: time.Unix(3, 0)},
		{Text: "middle-message", CreatedAt: time.Unix(2, 0)},
	}
	m := NewModel([]model.Issue{issue}, "")
	m.viewport.Width = 100
	m.viewport.Height = 100
	m.focused = focusDetail
	m.updateViewportContent()
	out := stripANSI(m.viewport.View())
	for _, text := range []string{"newest-message", "middle-message", "oldest-message"} {
		if !strings.Contains(out, text) {
			t.Fatalf("missing comment %q: %s", text, out)
		}
	}
	if strings.Index(out, "newest-message") >= strings.Index(out, "middle-message") || strings.Index(out, "middle-message") >= strings.Index(out, "oldest-message") {
		t.Fatalf("comments must be newest first: %s", out)
	}
	for _, want := range []bool{true, false} {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
		m = next.(Model)
		out = stripANSI(m.viewport.View())
		if got := strings.Index(out, "oldest-message") < strings.Index(out, "newest-message"); got != want {
			t.Fatalf("oldest first = %v, want %v: %s", got, want, out)
		}
	}
	if issue.Comments[0].Text != "oldest-message" {
		t.Fatal("rendering changed source comment order")
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
			out := stripANSI(renderIssueDetail(issue, issueMap, nil, theme, 70, NewMarkdownRendererWithTheme(70, theme), nil, nil, false))
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
	out := stripANSI(renderIssueDetail(issue, issueMap, nil, theme, 70, NewMarkdownRendererWithTheme(70, theme), nil, nil, false))
	if !strings.Contains(out, "┌") || strings.Contains(out, "sequenceDiagram") {
		t.Fatalf("comment diagram not rendered:\n%s", out)
	}
}

func TestDetailCKeyKeepsMermaidSequenceSource(t *testing.T) {
	source := "```mermaid\nsequenceDiagram\n    participant User\n    participant b9s\n    User->>b9s: Open detail\n    b9s-->>User: Render diagram\n```"
	issue := model.Issue{
		ID: "bd-uv2t", Title: "Sequence proof", Status: model.StatusOpen,
		IssueType: model.TypeTask, Description: source,
	}
	m := NewModel([]model.Issue{issue}, "")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 160, Height: 40})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.focused != focusDetail {
		t.Fatal("Enter must open detail before copying")
	}
	if out := stripANSI(m.viewport.View()); strings.Contains(out, "sequenceDiagram") ||
		!strings.Contains(out, "Render diagram") {
		t.Fatalf("detail must render the diagram before copying:\n%s", out)
	}

	var copied string
	m.clipboardWrite = func(text string) error {
		copied = text
		return nil
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = updated.(Model)
	if m.statusIsError || !strings.Contains(m.statusMsg, "Copied bd-uv2t to clipboard") {
		t.Fatalf("copy failed: %s", m.statusMsg)
	}
	if !strings.Contains(copied, source) {
		t.Fatalf("copy must preserve the original Mermaid fence:\n%s", copied)
	}
	if strings.Contains(copied, "┌") {
		t.Fatalf("copy must contain source without rendered boxes:\n%s", copied)
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
	out := renderIssueDetail(issue, issueMap, nil, theme, 40, NewMarkdownRendererWithTheme(40, theme), nil, nil, false)
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
	out := stripANSI(renderIssueDetail(issue, issueMap, nil, theme, 70, NewMarkdownRendererWithTheme(70, theme), nil, nil, false))
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
	out := stripANSI(renderIssueDetail(issue, issueMap, nil, theme, 70, NewMarkdownRendererWithTheme(70, theme), nil, nil, false))
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
	lines := strings.Split(stripANSI(renderIssueDetail(issue, issueMap, nil, theme, 70, nil, nil, nil, false)), "\n")
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
	out := stripANSI(renderIssueDetail(issue, issueMap, nil, theme, 70, nil, nil, nil, false))
	if exact := issue.UpdatedAt.Local().Format("2006-01-02 15:04"); !strings.Contains(out, exact) {
		t.Errorf("exact time %q missing with a long title:\n%s", exact, out)
	}
	if !strings.Contains(strings.ReplaceAll(out, "\n", " "), "very long title") {
		t.Errorf("title missing:\n%s", out)
	}
}
