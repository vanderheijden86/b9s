package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vanderheijden86/beadwork/pkg/model"
)

func TestGraphViewOpensFromTree(t *testing.T) {
	issues := []model.Issue{
		{
			ID:        "bd-blocker",
			Title:     "Finish prerequisite",
			Status:    model.StatusOpen,
			IssueType: model.TypeTask,
			CreatedAt: time.Now().Add(-time.Hour),
		},
		{
			ID:        "bd-work",
			Title:     "Dependent work",
			Status:    model.StatusOpen,
			IssueType: model.TypeFeature,
			CreatedAt: time.Now(),
			Dependencies: []*model.Dependency{
				{IssueID: "bd-work", DependsOnID: "bd-blocker", Type: model.DepBlocks},
			},
		},
	}
	m := NewModel(issues, "")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	m = updated.(Model)
	view := stripANSI(m.View())

	if got := m.currentViewName(); got != "graph" {
		t.Fatalf("pressing g from tree opened %q view, want graph", got)
	}
	if !strings.Contains(view, "Dependency Graph") {
		t.Fatalf("graph view missing heading: %q", view)
	}
}

func TestGraphViewShowsBlockingNeighborhood(t *testing.T) {
	issues := []model.Issue{
		{
			ID:        "bd-blocker",
			Title:     "Finish prerequisite",
			Status:    model.StatusOpen,
			IssueType: model.TypeTask,
		},
		{
			ID:        "bd-work",
			Title:     "Dependent work",
			Status:    model.StatusInProgress,
			IssueType: model.TypeFeature,
			Dependencies: []*model.Dependency{
				{IssueID: "bd-work", DependsOnID: "bd-blocker", Type: model.DepBlocks},
				{IssueID: "bd-work", DependsOnID: "external-review", Type: model.DepBlocks},
				{IssueID: "bd-work", DependsOnID: "bd-reference", Type: model.DepRelated},
			},
		},
		{
			ID:        "bd-child",
			Title:     "Waits on dependent work",
			Status:    model.StatusOpen,
			IssueType: model.TypeTask,
			Dependencies: []*model.Dependency{
				{IssueID: "bd-child", DependsOnID: "bd-work", Type: model.DepBlocks},
			},
		},
	}
	graph := NewGraphModel(issues, newTreeTestTheme())
	graph.SelectIssue("bd-work")

	view := stripANSI(graph.View(100, 24))
	for _, want := range []string{
		"Dependency Graph",
		"bd-work",
		"BLOCKED BY (2)",
		"bd-blocker",
		"external-review (not loaded)",
		"BLOCKS (1)",
		"bd-child",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("graph view missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "bd-reference") {
		t.Fatalf("non-blocking related edge shown as a blocker:\n%s", view)
	}
}

func TestGraphViewNavigationAndEscape(t *testing.T) {
	issues := []model.Issue{
		{ID: "bd-a", Title: "A", Status: model.StatusOpen, IssueType: model.TypeTask},
		{ID: "bd-b", Title: "B", Status: model.StatusOpen, IssueType: model.TypeTask},
	}
	m := NewModel(issues, "")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyHome})
	m = updated.(Model)
	first := m.graph.SelectedIssue()
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = updated.(Model)
	second := m.graph.SelectedIssue()
	if first == nil || second == nil || first.ID == second.ID {
		t.Fatalf("j did not move graph selection: first=%v second=%v", first, second)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if got := m.currentViewName(); got != "tree" {
		t.Fatalf("escape returned to %q view, want tree", got)
	}
}

func TestGraphSetIssuesPreservesSelectionAndRefreshesEdges(t *testing.T) {
	graph := NewGraphModel([]model.Issue{
		{ID: "bd-a", Title: "A", Status: model.StatusOpen, IssueType: model.TypeTask},
		{ID: "bd-b", Title: "B", Status: model.StatusOpen, IssueType: model.TypeTask},
	}, newTreeTestTheme())
	graph.SelectIssue("bd-b")

	graph.SetIssues([]model.Issue{
		{ID: "bd-a", Title: "A", Status: model.StatusClosed, IssueType: model.TypeTask},
		{
			ID:           "bd-b",
			Title:        "B",
			Status:       model.StatusOpen,
			IssueType:    model.TypeTask,
			Dependencies: []*model.Dependency{{IssueID: "bd-b", DependsOnID: "bd-a", Type: model.DepBlocks}},
		},
	})

	if selected := graph.SelectedIssue(); selected == nil || selected.ID != "bd-b" {
		t.Fatalf("selection not preserved after refresh: %v", selected)
	}
	view := stripANSI(graph.View(100, 20))
	if !strings.Contains(view, "BLOCKED BY (1)") || !strings.Contains(view, "bd-a") {
		t.Fatalf("dependency index not refreshed:\n%s", view)
	}
}

func TestGraphDetailReturnsToGraph(t *testing.T) {
	m := NewModel([]model.Issue{
		{ID: "bd-a", Title: "A", Status: model.StatusOpen, IssueType: model.TypeTask},
	}, "")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if got := m.FocusState(); got != "detail" {
		t.Fatalf("enter moved focus to %q, want detail", got)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if got := m.FocusState(); got != "graph" {
		t.Fatalf("escape returned focus to %q, want graph", got)
	}
}

func TestGraphViewSwitchesToTreeWithE(t *testing.T) {
	m := NewModel(nil, "")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("E")})
	m = updated.(Model)

	if got := m.currentViewName(); got != "tree" {
		t.Fatalf("E switched to %q view, want tree", got)
	}
}
