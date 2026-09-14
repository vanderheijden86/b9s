package ui

import (
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vanderheijden86/beadwork/pkg/model"
)

func newBulkMarkModel(t *testing.T) Model {
	t.Helper()
	now := time.Now()
	m := NewModel([]model.Issue{
		{ID: "bd-1", Title: "First", Status: model.StatusOpen, IssueType: model.TypeTask, CreatedAt: now.Add(3 * time.Hour)},
		{ID: "bd-2", Title: "Second", Status: model.StatusOpen, IssueType: model.TypeTask, CreatedAt: now.Add(2 * time.Hour)},
		{ID: "bd-3", Title: "Third", Status: model.StatusOpen, IssueType: model.TypeTask, CreatedAt: now.Add(time.Hour)},
	}, "")
	m.issueWriter = &IssueWriter{bdPath: "/bin/echo", available: true}
	m.tree.SelectByID("bd-1")
	return m
}

func pressBulkKey(t *testing.T, m Model, msg tea.KeyMsg) (Model, tea.Cmd) {
	t.Helper()
	updated, cmd := m.Update(msg)
	return updated.(Model), cmd
}

func runeKey(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestSpaceTogglesMarkOnTreeRow(t *testing.T) {
	m := newBulkMarkModel(t)

	m, _ = pressBulkKey(t, m, tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")})
	if got := m.tree.TreeMarkedIDs(); !reflect.DeepEqual(got, []string{"bd-1"}) {
		t.Fatalf("space must mark the cursor row, got %v", got)
	}

	m, _ = pressBulkKey(t, m, tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")})
	if got := m.tree.TreeMarkedIDs(); len(got) != 0 {
		t.Fatalf("second space must unmark the cursor row, got %v", got)
	}
}

func TestCtrlSpaceMarksRangeFromPreviousMark(t *testing.T) {
	m := newBulkMarkModel(t)
	m, _ = pressBulkKey(t, m, tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")})
	m.tree.SelectByID("bd-3")

	// Terminals deliver ctrl+space as NUL, which bubbletea names ctrl+@.
	m, _ = pressBulkKey(t, m, tea.KeyMsg{Type: tea.KeyCtrlAt})

	want := []string{"bd-1", "bd-2", "bd-3"}
	if got := m.tree.TreeMarkedIDs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("ctrl+space must mark the range, want %v got %v", want, got)
	}
}

func TestCtrlBackslashClearsMarks(t *testing.T) {
	m := newBulkMarkModel(t)
	m, _ = pressBulkKey(t, m, tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")})

	m, _ = pressBulkKey(t, m, tea.KeyMsg{Type: tea.KeyCtrlBackslash})

	if got := m.tree.MarkedCount(); got != 0 {
		t.Fatalf("ctrl+\\ must clear marks, %d remain", got)
	}
}

func TestShiftKConfirmsCloseOfAllMarkedIssues(t *testing.T) {
	m := newBulkMarkModel(t)
	m, _ = pressBulkKey(t, m, tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")})
	m.tree.SelectByID("bd-3")
	m, _ = pressBulkKey(t, m, tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")})
	m.tree.SelectByID("bd-2") // cursor on an unmarked row must not be targeted

	m, cmd := pressBulkKey(t, m, runeKey("K"))
	if cmd != nil {
		t.Fatal("K must wait for confirmation before closing")
	}
	view := m.View()
	if !strings.Contains(view, "Close 2 issues?") {
		t.Fatalf("expected count-based close confirmation, got:\n%s", view)
	}
	if !strings.Contains(view, "bd-1") || !strings.Contains(view, "bd-3") {
		t.Fatalf("confirmation must list the marked IDs, got:\n%s", view)
	}

	m, cmd = pressBulkKey(t, m, runeKey("y"))
	if cmd == nil {
		t.Fatal("confirming must return a command")
	}
	result, ok := cmd().(BdResultMsg)
	if !ok {
		t.Fatal("expected BdResultMsg")
	}
	if result.Operation != BdOpClose || !result.Success {
		t.Fatalf("unexpected result: %#v", result)
	}
	if !reflect.DeepEqual(result.IssueIDs, []string{"bd-1", "bd-3"}) {
		t.Fatalf("result must carry every closed ID, got %v", result.IssueIDs)
	}
	// /bin/echo prints its argv, proving a single bd invocation carried both IDs.
	if result.Output != "close bd-1 bd-3" {
		t.Fatalf("expected one bd close call with both IDs, got %q", result.Output)
	}
	if got := m.tree.MarkedCount(); got != 0 {
		t.Fatalf("closed issues must be unmarked, %d remain", got)
	}
}

func TestDeleteConfirmsDeletionOfAllMarkedIssues(t *testing.T) {
	m := newBulkMarkModel(t)
	m, _ = pressBulkKey(t, m, tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")})
	m.tree.SelectByID("bd-2")
	m, _ = pressBulkKey(t, m, tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")})

	m, _ = pressBulkKey(t, m, tea.KeyMsg{Type: tea.KeyDelete})
	if !strings.Contains(m.View(), "Delete 2 issues?") {
		t.Fatalf("expected count-based delete confirmation, got:\n%s", m.View())
	}

	_, cmd := pressBulkKey(t, m, runeKey("y"))
	result := cmd().(BdResultMsg)
	if result.Operation != BdOpDelete || result.Output != "delete bd-1 bd-2 --force" {
		t.Fatalf("expected one bd delete call with both IDs, got %#v", result)
	}
}

func TestCancelledBulkCloseKeepsMarks(t *testing.T) {
	m := newBulkMarkModel(t)
	m, _ = pressBulkKey(t, m, tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")})
	m, _ = pressBulkKey(t, m, runeKey("K"))

	m, _ = pressBulkKey(t, m, tea.KeyMsg{Type: tea.KeyEsc})

	if got := m.tree.MarkedCount(); got != 1 {
		t.Fatalf("cancelling must keep marks, got %d", got)
	}
}

func TestBulkCloseStatusReportsCount(t *testing.T) {
	m := newBulkMarkModel(t)

	updated, _ := m.Update(BdResultMsg{Operation: BdOpClose, IssueIDs: []string{"bd-1", "bd-3"}, Success: true})
	m = updated.(Model)

	if m.statusMsg != "Closed 2 issues" {
		t.Fatalf("expected count in status, got %q", m.statusMsg)
	}
}

func TestTreeFooterShowsMarkedCount(t *testing.T) {
	m := newBulkMarkModel(t)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 240, Height: 40})
	m = updated.(Model)
	m, _ = pressBulkKey(t, m, tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")})

	if !strings.Contains(m.View(), "1 marked") {
		t.Fatalf("footer must show the marked count, got:\n%s", m.View())
	}
}
