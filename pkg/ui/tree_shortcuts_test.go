package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vanderheijden86/beadwork/pkg/model"
)

func newTreeShortcutModel(t *testing.T) Model {
	t.Helper()
	issues := []model.Issue{
		{ID: "bd-open", Title: "Open task", IssueType: model.TypeTask, Status: model.StatusOpen},
		{ID: "bd-done", Title: "Finished task", IssueType: model.TypeTask, Status: model.StatusClosed},
	}
	m := NewModel(issues, "")
	m.tree.Build(issues)
	m.focused = focusTree
	return m
}

func pressTreeKey(m Model, r rune) Model {
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	return updated.(Model)
}

func TestTreeLowercaseCCopiesSelectedIDAndTitle(t *testing.T) {
	m := newTreeShortcutModel(t)
	selected := m.tree.SelectedIssue()
	if selected == nil {
		t.Fatal("test setup should select an issue")
	}
	var copied string
	m.clipboardWrite = func(text string) error {
		copied = text
		return nil
	}

	m = pressTreeKey(m, 'c')

	want := selected.ID + " " + selected.Title
	if copied != want {
		t.Fatalf("c in tree view copied %q, want %q", copied, want)
	}
	if m.currentFilter == "closed" {
		t.Fatal("c in tree view must not apply the closed filter")
	}
	if !strings.Contains(m.statusMsg, selected.ID) {
		t.Fatalf("status should confirm the copied issue, got %q", m.statusMsg)
	}
}

func TestTreeCapitalCFiltersClosedIssues(t *testing.T) {
	m := newTreeShortcutModel(t)

	m = pressTreeKey(m, 'C')

	if m.currentFilter != "closed" {
		t.Fatalf("C in tree view should filter closed issues, got filter %q", m.currentFilter)
	}
	if m.tree.IsColumnPopupOpen() {
		t.Fatal("C in tree view must not open the column popup")
	}
}

func TestTreePipeOpensAndClosesColumnPopup(t *testing.T) {
	m := newTreeShortcutModel(t)

	m = pressTreeKey(m, '|')
	if !m.tree.IsColumnPopupOpen() {
		t.Fatal("| in tree view should open the column popup")
	}

	m = pressTreeKey(m, '|')
	if m.tree.IsColumnPopupOpen() {
		t.Fatal("| should close the column popup it opened")
	}
}
