package ui

import (
	"sort"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func branchBoardModel(t *testing.T, selectedID string) Model {
	t.Helper()
	m := NewModel(epicBoardIssues(), "")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 40})
	m = updated.(Model)
	m.isBoardView = true
	m.focused = focusBoard
	m.refreshBoardAndGraphForCurrentFilter()
	if !m.board.SelectIssueByID(selectedID) {
		t.Fatalf("could not select %q", selectedID)
	}
	return m
}

func boardCardIDs(b BoardModel) []string {
	var ids []string
	for col := range b.columns {
		ids = append(ids, columnIDs(b, col)...)
	}
	sort.Strings(ids)
	return ids
}

func TestBoardBranchFilter_FTogglesTheSelectedCardsBranch(t *testing.T) {
	tests := []struct {
		name     string
		selected string
		wantIDs  []string
	}{
		{name: "grandchild", selected: "spectroscope-eg0.2.1", wantIDs: []string{"eg0", "eg0.1", "eg0.2", "eg0.2.1", "eg0.3"}},
		{name: "epic", selected: "spectroscope-k3s", wantIDs: []string{"k3s", "k3s.1"}},
		{name: "standalone", selected: "spectroscope-x1", wantIDs: []string{"x1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := branchBoardModel(t, tt.selected)
			all := boardCardIDs(m.board)

			m = typeKeys(m, "f")

			if got := boardCardIDs(m.board); strings.Join(got, ",") != strings.Join(tt.wantIDs, ",") {
				t.Fatalf("branch cards = %v, want %v", got, tt.wantIDs)
			}
			if sel := m.board.SelectedIssue(); sel == nil || sel.ID != tt.selected {
				t.Fatalf("f moved the selection off %q", tt.selected)
			}
			if header := stripANSI(m.board.View(200, 30)); !strings.Contains(header, "branch "+m.board.displayID("spectroscope-"+tt.wantIDs[0])) {
				t.Fatalf("board header does not name the branch:\n%s", header)
			}

			// A reload regroups the board and must keep the branch.
			m.applyFilter()
			if got := boardCardIDs(m.board); strings.Join(got, ",") != strings.Join(tt.wantIDs, ",") {
				t.Fatalf("branch lost after reload: %v", got)
			}

			m = typeKeys(m, "f")

			if got := boardCardIDs(m.board); strings.Join(got, ",") != strings.Join(all, ",") {
				t.Fatalf("second f shows %v, want the whole board %v", got, all)
			}
			if sel := m.board.SelectedIssue(); sel == nil || sel.ID != tt.selected {
				t.Fatalf("second f moved the selection off %q", tt.selected)
			}
		})
	}
}
