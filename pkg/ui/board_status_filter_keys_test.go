package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanderheijden86/beadwork/pkg/model"
)

func statusFilterBoard(t *testing.T) Model {
	t.Helper()
	issues := []model.Issue{
		{ID: "sf-1", Title: "Open one", Status: model.StatusOpen, IssueType: model.TypeTask},
		{ID: "sf-2", Title: "Working one", Status: model.StatusInProgress, IssueType: model.TypeTask},
		{ID: "sf-3", Title: "Done one", Status: model.StatusClosed, IssueType: model.TypeTask},
	}
	m := NewModel(issues, "")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 40})
	return pressBoard(t, updated.(Model), runeKey("b"))
}

func boardIssueIDs(m Model) map[string]bool {
	ids := map[string]bool{}
	for _, item := range m.list.Items() {
		ids[item.(IssueItem).Issue.ID] = true
	}
	return ids
}

func TestBoardStatusKeysWriteTheQueryBar(t *testing.T) {
	cases := []struct {
		key, query, only string
	}{
		{"o", "status:open", "sf-1"},
		{"i", "status:in_progress", "sf-2"},
		{"C", "status:closed", "sf-3"},
	}
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			m := pressBoard(t, statusFilterBoard(t), runeKey(tc.key))
			if got := m.queryState.Text(); got != tc.query {
				t.Fatalf("%s must put %q in the query bar, got %q", tc.key, tc.query, got)
			}
			ids := boardIssueIDs(m)
			if len(ids) != 1 || !ids[tc.only] {
				t.Fatalf("%s must show only %s, got %v", tc.key, tc.only, ids)
			}
		})
	}
}

func TestBoardStatusKeysCombineAndToggle(t *testing.T) {
	m := pressBoard(t, statusFilterBoard(t), runeKey("o"), runeKey("i"))
	if got := m.queryState.Text(); got != "status:open status:in_progress" {
		t.Fatalf("o then i must combine both statuses, got %q", got)
	}
	if ids := boardIssueIDs(m); len(ids) != 2 || ids["sf-3"] {
		t.Fatalf("open plus in progress must hide closed work, got %v", ids)
	}

	m = pressBoard(t, m, runeKey("o"))
	if got := m.queryState.Text(); got != "status:in_progress" {
		t.Fatalf("a second o must remove its term, got %q", got)
	}
}

func TestBoardStatusKeysKeepOtherQueryTerms(t *testing.T) {
	m := statusFilterBoard(t)
	m.setQueryText("one status:open")
	m = pressBoard(t, m, runeKey("i"))
	if got := m.queryState.Text(); got != "one status:open status:in_progress" {
		t.Fatalf("i must keep the rest of the query, got %q", got)
	}
}

func TestBoardClosedKeyShowsTheClosedColumn(t *testing.T) {
	m := statusFilterBoard(t)
	if m.board.ShowsClosedColumn() {
		t.Fatal("the closed column starts hidden")
	}
	m = pressBoard(t, m, runeKey("C"))
	if !m.board.ShowsClosedColumn() {
		t.Fatal("C must show the closed column, or the filtered board is empty")
	}
	if m.board.SelectedIssue() == nil || m.board.SelectedIssue().ID != "sf-3" {
		t.Fatalf("C must select the closed card, got %v", m.board.SelectedIssue())
	}
}
