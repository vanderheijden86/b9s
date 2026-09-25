package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanderheijden86/beadwork/pkg/model"
)

func foldColumnBoard(t *testing.T) Model {
	t.Helper()
	issues := []model.Issue{
		{ID: "fc-1", Title: "Open card one", Status: model.StatusOpen, IssueType: model.TypeTask},
		{ID: "fc-2", Title: "Open card two", Status: model.StatusOpen, IssueType: model.TypeTask},
		{ID: "fc-3", Title: "Working card", Status: model.StatusInProgress, IssueType: model.TypeTask},
		{ID: "fc-4", Title: "Blocked card", Status: model.StatusBlocked, IssueType: model.TypeTask},
	}
	m := NewModel(issues, "")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 40})
	return pressBoard(t, updated.(Model), runeKey("b"))
}

func boardRegionFor(b *BoardModel, col int) (boardRegion, bool) {
	for _, r := range planBoardRegions(200, b.regionInputs(), b.actualFocusedCol(), adaptiveBreakpoints) {
		if r.col == col {
			return r, true
		}
	}
	return boardRegion{}, false
}

func TestBoardZFoldsTheFocusedColumnIntoARail(t *testing.T) {
	m := foldColumnBoard(t)
	if m.board.actualFocusedCol() != ColOpen {
		t.Fatalf("the board must start on the open column, got %d", m.board.actualFocusedCol())
	}
	m = pressBoard(t, m, runeKey("z"))

	if !m.board.ColumnFolded(ColOpen) {
		t.Fatal("z must fold the focused open column")
	}
	if got := m.board.actualFocusedCol(); got == ColOpen {
		t.Fatal("the focus must leave the folded column")
	}
	if sel := m.board.SelectedIssue(); sel == nil || sel.Status == model.StatusOpen {
		t.Fatalf("the selection must move to a card outside the folded column, got %+v", sel)
	}
	r, ok := boardRegionFor(&m.board, ColOpen)
	if !ok || !r.collapsed {
		t.Fatalf("the folded open column must render as a rail, got %+v (shown %v)", r, ok)
	}
	view := m.board.View(200, 40)
	if strings.Contains(view, "Open card one") {
		t.Fatal("a folded column must not render its cards")
	}
	if !strings.Contains(view, "Working card") {
		t.Fatal("the other columns must still render their cards")
	}
}

func TestBoardArrowsSkipAFoldedColumn(t *testing.T) {
	m := pressBoard(t, foldColumnBoard(t), runeKey("z"))
	for range 4 {
		m = pressBoard(t, m, tea.KeyMsg{Type: tea.KeyLeft})
		if m.board.actualFocusedCol() == ColOpen && !m.board.onEpicColumn {
			t.Fatal("Left must never focus the folded column")
		}
	}
}

func TestBoardShiftZUnfoldsEveryColumn(t *testing.T) {
	m := pressBoard(t, foldColumnBoard(t), runeKey("z"))
	selID := m.board.SelectedIssue().ID
	m = pressBoard(t, m, runeKey("Z"))
	if m.board.ColumnFolded(ColOpen) {
		t.Fatal("Z must unfold the open column")
	}
	if got := m.board.SelectedIssue(); got == nil || got.ID != selID {
		t.Fatalf("Z must keep the selection on %s, got %+v", selID, got)
	}
	if r, _ := boardRegionFor(&m.board, ColOpen); r.collapsed {
		t.Fatal("an unfolded open column with cards must render in full")
	}
}

func TestBoardZKeepsOneColumnUnfolded(t *testing.T) {
	m := pressBoard(t, foldColumnBoard(t), runeKey("z"), runeKey("z"), runeKey("z"), runeKey("z"), runeKey("z"))
	unfolded := 0
	for col := range 4 {
		if !m.board.columnHidden(col) && !m.board.ColumnFolded(col) {
			unfolded++
		}
	}
	if unfolded < 1 {
		t.Fatal("z must leave at least one column unfolded")
	}
	if m.board.SelectedIssue() == nil && !m.board.onEpicColumn {
		t.Fatal("the focus must stay on a column that renders")
	}
}

func TestBoardSwimlaneChangeClearsFoldedColumns(t *testing.T) {
	m := pressBoard(t, foldColumnBoard(t), runeKey("z"))
	m.board.CycleSwimLaneMode()
	for col := range 4 {
		if m.board.ColumnFolded(col) {
			t.Fatalf("a swimlane change regroups the columns, so column %d must not stay folded", col)
		}
	}
}

func TestBoardZFoldsTheOpenColumnInEpicLanes(t *testing.T) {
	issues := []model.Issue{
		{ID: "ep", Title: "Epic lane", Status: model.StatusOpen, IssueType: model.TypeEpic},
		{ID: "ep.1", Title: "Open child card", Status: model.StatusOpen, IssueType: model.TypeTask,
			Dependencies: []*model.Dependency{{IssueID: "ep.1", DependsOnID: "ep", Type: model.DepParentChild}}},
		{ID: "ep.2", Title: "Working child card", Status: model.StatusInProgress, IssueType: model.TypeTask,
			Dependencies: []*model.Dependency{{IssueID: "ep.2", DependsOnID: "ep", Type: model.DepParentChild}}},
	}
	m := NewModel(issues, "")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 40})
	m = pressBoard(t, updated.(Model), runeKey("b"))
	if !m.board.hasEpicLanes() {
		t.Fatal("the fixture must render epic lanes")
	}
	m.board.JumpToColumn(ColOpen)
	m = pressBoard(t, m, runeKey("z"))
	view := m.board.View(200, 40)
	if strings.Contains(view, "Open child card") {
		t.Fatal("a folded column must not render its cards in a lane")
	}
	if !strings.Contains(view, "Working child card") {
		t.Fatal("the lane must still render the unfolded columns")
	}
}
