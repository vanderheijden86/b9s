// board_branch_search_test.go - A text query keeps each hit's descendants on
// the board, as the tree does (bd-xkxb), so a child whose own ID and title do
// not match still shows under its matched epic (bd-e5u3.19).
package ui

import (
	"sort"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanderheijden86/b9s/pkg/model"
)

func branchSearchBoard(t *testing.T) Model {
	t.Helper()
	m := newBranchSearchModel(branchSearchIssues())
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 40})
	return pressBoard(t, updated.(Model), runeKey("b"))
}

func assertBoardIDs(t *testing.T, m Model, want ...string) {
	t.Helper()
	got := make([]string, 0, len(m.board.allIssues))
	for _, is := range m.board.allIssues {
		got = append(got, is.ID)
	}
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("board issue IDs = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("board issue IDs = %v, want %v", got, want)
		}
	}
}

func TestBoardQueryHitOnEpicKeepsItsWholeSubtree(t *testing.T) {
	m := branchSearchBoard(t)

	m.setQueryText("acceptance")

	assertBoardIDs(t, m, "epic-a", "feat-a1", "task-a1", "feat-a2", "task-a2")
}

func TestBoardQueryRevealedDescendantsRespectStatusFilter(t *testing.T) {
	m := branchSearchBoard(t)
	m.currentFilter = "open"

	m.setQueryText("quokka")

	assertBoardIDs(t, m, "epic-b", "feat-b1")
}

// A watcher reload must keep both the query and the branch it reveals.
func TestBoardQueryBranchSurvivesReload(t *testing.T) {
	m := branchSearchBoard(t)
	m.setQueryText("acceptance")

	issues := append(branchSearchIssues(), childOf("task-a3", "Measure the models", "epic-a", model.TypeTask, model.StatusInProgress))
	m, _ = sendMsg(t, m, SnapshotReadyMsg{Snapshot: NewSnapshotBuilder(issues).Build()})

	assertBoardIDs(t, m, "epic-a", "feat-a1", "task-a1", "feat-a2", "task-a2", "task-a3")
}

func TestBoardQueryHitOnNestedTaskShowsNoSiblings(t *testing.T) {
	m := branchSearchBoard(t)

	m.setQueryText("tunnel")

	assertBoardIDs(t, m, "task-a1")
}
