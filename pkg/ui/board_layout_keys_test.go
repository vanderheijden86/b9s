package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanderheijden86/beadwork/pkg/model"
)

func boardKeyModel(t *testing.T) Model {
	t.Helper()
	m := NewModel(sparseBoardIssues(), "")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 40})
	m = updated.(Model)
	m.isBoardView = true
	m.focused = focusBoard
	m.board.SelectIssueByID("spectroscope-eg0.4.2")
	return m
}

func pressBoard(t *testing.T, m Model, keys ...tea.KeyMsg) Model {
	t.Helper()
	for _, k := range keys {
		updated, _ := m.Update(k)
		m = updated.(Model)
	}
	return m
}

func TestBoardKeyL_MovesRightInsteadOfOpeningLabels(t *testing.T) {
	m := boardKeyModel(t)
	m = pressBoard(t, m, runeKey("l"))
	if m.showLabelPicker {
		t.Fatal("l on the board must move to the next column, not open the label picker")
	}
	if m.board.actualFocusedCol() != ColInProgress {
		t.Fatalf("l must focus the next column, got %d", m.board.actualFocusedCol())
	}
}

func TestBoardKeyB_OpensOnFirstPopulatedColumn(t *testing.T) {
	var issues []model.Issue
	for _, is := range sparseBoardIssues() {
		if is.Status != model.StatusOpen {
			issues = append(issues, is)
		}
	}
	m := NewModel(issues, "")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 40})
	m = pressBoard(t, updated.(Model), runeKey("b"))
	if !m.isBoardView {
		t.Fatal("b must open the board")
	}
	if got := m.board.actualFocusedCol(); got != ColInProgress {
		t.Fatalf("an empty OPEN column must not take the focus, got column %d", got)
	}
	if m.board.SelectedIssue() == nil {
		t.Fatal("opening the board must select an issue")
	}
}
