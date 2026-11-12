package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/vanderheijden86/b9s/pkg/model"
)

// Moving the cursor must not pay for a detail pane nobody can see: on an issue
// with many comments one render costs hundreds of milliseconds (bd-mlet.1).
func commentedIssues(n, comments int) []model.Issue {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	issues := make([]model.Issue, 0, n)
	for i := 0; i < n; i++ {
		issue := model.Issue{
			ID:        fmt.Sprintf("c-%d", i+1),
			Title:     fmt.Sprintf("Commented %d", i+1),
			Status:    model.StatusOpen,
			IssueType: model.TypeTask,
			Priority:  2,
			CreatedAt: base.Add(time.Duration(n-i) * time.Hour),
			UpdatedAt: base.Add(time.Duration(n-i) * time.Hour),
		}
		for c := 0; c < comments; c++ {
			issue.Comments = append(issue.Comments, &model.Comment{
				ID:        fmt.Sprintf("%s-c%d", issue.ID, c),
				IssueID:   issue.ID,
				Author:    "Claude",
				Text:      fmt.Sprintf("Progress **%d** on `%s`", c, issue.ID),
				CreatedAt: base.Add(time.Duration(c) * time.Minute),
			})
		}
		issues = append(issues, issue)
	}
	return issues
}

func sendDetailKey(t *testing.T, m Model, key tea.KeyMsg) Model {
	t.Helper()
	next, _ := m.Update(key)
	return next.(Model)
}

func TestHiddenDetailIsNotRenderedOnCursorMove(t *testing.T) {
	m := NewModel(commentedIssues(6, 3), "")
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	m = next.(Model)
	if !m.treeDetailHidden {
		t.Fatal("narrow window should hide the tree detail pane")
	}

	before := m.detailRenders
	for i := 0; i < 4; i++ {
		m = sendDetailKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	}
	if m.detailRenders != before {
		t.Fatalf("detail rendered %d times while hidden, want 0", m.detailRenders-before)
	}

	selected := m.TreeSelectedID()
	m = sendDetailKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.focused != focusDetail {
		t.Fatalf("Enter should open the detail, focus is %v", m.focused)
	}
	if !strings.Contains(m.viewport.View(), selected) {
		t.Fatalf("opened detail does not show %s:\n%s", selected, m.viewport.View())
	}
}

func TestVisibleDetailRendersOnCursorMove(t *testing.T) {
	m := NewModel(commentedIssues(6, 3), "")
	next, _ := m.Update(tea.WindowSizeMsg{Width: 160, Height: 40})
	m = next.(Model)
	m = sendDetailKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if m.treeDetailHidden || !m.isSplitView {
		t.Fatal("wide window should show the detail beside the tree")
	}
	m = sendDetailKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if !strings.Contains(m.viewport.View(), m.TreeSelectedID()) {
		t.Fatalf("visible detail does not follow the cursor to %s", m.TreeSelectedID())
	}
}

func TestRenderAtCachesByWidthAndText(t *testing.T) {
	theme := DefaultTheme(lipgloss.NewRenderer(nil))
	md := NewMarkdownRendererWithTheme(80, theme)

	first, err := md.RenderAt(40, "some **bold** text")
	if err != nil {
		t.Fatal(err)
	}
	if md.width != 80 {
		t.Fatalf("RenderAt changed the renderer width to %d", md.width)
	}
	if md.renderMisses != 1 {
		t.Fatalf("first render: misses = %d, want 1", md.renderMisses)
	}
	again, _ := md.RenderAt(40, "some **bold** text")
	if again != first || md.renderMisses != 1 {
		t.Fatalf("same width and text should hit the cache: misses = %d", md.renderMisses)
	}

	fresh := NewMarkdownRendererWithTheme(40, theme)
	want, _ := fresh.Render("some **bold** text")
	if first != want {
		t.Fatalf("RenderAt(40) differs from a renderer built at width 40:\n%q\n%q", first, want)
	}

	md.RenderAt(50, "some **bold** text")
	if md.renderMisses != 2 {
		t.Fatalf("another width must render again: misses = %d", md.renderMisses)
	}
	md.SetWidthWithTheme(80, theme)
	md.RenderAt(40, "some **bold** text")
	if md.renderMisses != 3 {
		t.Fatalf("a theme change must clear the cache: misses = %d", md.renderMisses)
	}
}

// BenchmarkDetailCursorMove measures one j/k keypress with the detail pane
// open on issues that carry 200 comments each, the shape lane agents leave.
func BenchmarkDetailCursorMove(b *testing.B) {
	m := NewModel(commentedIssues(20, 200), "")
	next, _ := m.Update(tea.WindowSizeMsg{Width: 160, Height: 50})
	m = next.(Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = next.(Model)
	keys := []tea.KeyMsg{{Type: tea.KeyRunes, Runes: []rune{'j'}}, {Type: tea.KeyRunes, Runes: []rune{'k'}}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		next, _ = m.Update(keys[i%2])
		m = next.(Model)
	}
}

func TestDetailRerenderHitsTheCommentCache(t *testing.T) {
	theme := DefaultTheme(lipgloss.NewRenderer(nil))
	md := NewMarkdownRendererWithTheme(80, theme)
	issue := commentedIssues(1, 50)[0]
	first := renderIssueDetail(issue, nil, nil, theme, 80, md, nil, nil, false)
	misses := md.renderMisses
	second := renderIssueDetail(issue, nil, nil, theme, 80, md, nil, nil, false)
	if md.renderMisses != misses {
		t.Fatalf("re-rendering the same issue missed the cache %d times", md.renderMisses-misses)
	}
	if first != second {
		t.Fatal("cached render differs from the first render")
	}
}
