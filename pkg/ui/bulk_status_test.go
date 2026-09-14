package ui

import (
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestShiftSOpensStatusPickerForTreeRow(t *testing.T) {
	m := newBulkMarkModel(t)

	m, cmd := pressBulkKey(t, m, runeKey("S"))

	if cmd != nil {
		t.Fatal("opening the status picker must not run a command")
	}
	if !m.showStatusPicker {
		t.Fatal("S must open the status picker in tree view")
	}
	if !strings.Contains(m.View(), "Change Status") {
		t.Fatalf("expected status picker, got:\n%s", m.View())
	}
}

func TestStatusPickerShowsMarkedCount(t *testing.T) {
	m := newBulkMarkModel(t)
	m, _ = pressBulkKey(t, m, tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")})
	m.tree.SelectByID("bd-3")
	m, _ = pressBulkKey(t, m, tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")})

	m, _ = pressBulkKey(t, m, runeKey("S"))

	if !strings.Contains(m.View(), "Change Status (2 issues)") {
		t.Fatalf("status picker must state how many issues it changes, got:\n%s", m.View())
	}
}

func TestStatusPickerAppliesToAllMarkedIssues(t *testing.T) {
	m := newBulkMarkModel(t)
	m, _ = pressBulkKey(t, m, tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")})
	m.tree.SelectByID("bd-3")
	m, _ = pressBulkKey(t, m, tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")})
	m.tree.SelectByID("bd-2") // cursor on an unmarked row must not be targeted

	m, _ = pressBulkKey(t, m, runeKey("S"))
	m, _ = pressBulkKey(t, m, runeKey("j")) // open -> in_progress
	m, cmd := pressBulkKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if cmd == nil {
		t.Fatal("applying a status must return a command")
	}
	result, ok := findBdResult(cmd)
	if !ok {
		t.Fatal("expected a BdResultMsg from the status change")
	}
	if result.Operation != BdOpSetStatus || !result.Success {
		t.Fatalf("unexpected result: %#v", result)
	}
	if !reflect.DeepEqual(result.IssueIDs, []string{"bd-1", "bd-3"}) {
		t.Fatalf("result must carry every updated ID, got %v", result.IssueIDs)
	}
	// /bin/echo prints its argv, proving a single bd invocation carried both IDs.
	if result.Output != "update bd-1 bd-3 --status=in_progress" {
		t.Fatalf("expected one bd update call with both IDs, got %q", result.Output)
	}
	if m.showStatusPicker {
		t.Fatal("status picker must close after applying")
	}
	if got := m.tree.MarkedCount(); got != 0 {
		t.Fatalf("updated issues must be unmarked, %d remain", got)
	}
}

func TestStatusPickerWithoutMarksUpdatesCursorRow(t *testing.T) {
	m := newBulkMarkModel(t)
	m.tree.SelectByID("bd-2")

	m, _ = pressBulkKey(t, m, runeKey("S"))
	m, _ = pressBulkKey(t, m, runeKey("j"))
	_, cmd := pressBulkKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	result, ok := findBdResult(cmd)
	if !ok {
		t.Fatal("expected a BdResultMsg from the status change")
	}
	if result.IssueID != "bd-2" || !strings.HasPrefix(result.Output, "update bd-2 ") {
		t.Fatalf("expected the cursor row to be updated, got %#v", result)
	}
}

func TestCancelledStatusPickerKeepsMarks(t *testing.T) {
	m := newBulkMarkModel(t)
	m, _ = pressBulkKey(t, m, tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")})
	m, _ = pressBulkKey(t, m, runeKey("S"))

	m, _ = pressBulkKey(t, m, tea.KeyMsg{Type: tea.KeyEsc})

	if m.showStatusPicker {
		t.Fatal("esc must close the status picker")
	}
	if got := m.tree.MarkedCount(); got != 1 {
		t.Fatalf("cancelling must keep marks, got %d", got)
	}
}

func TestBulkStatusReportsCount(t *testing.T) {
	m := newBulkMarkModel(t)

	updated, _ := m.Update(BdResultMsg{Operation: BdOpSetStatus, IssueIDs: []string{"bd-1", "bd-3"}, Success: true})
	m = updated.(Model)

	if m.statusMsg != "Updated 2 issues" {
		t.Fatalf("expected count in status, got %q", m.statusMsg)
	}
}

// findBdResult runs cmd, unwrapping tea.Batch, and returns the first
// BdResultMsg it produces.
func findBdResult(cmd tea.Cmd) (BdResultMsg, bool) {
	if cmd == nil {
		return BdResultMsg{}, false
	}
	switch msg := cmd().(type) {
	case BdResultMsg:
		return msg, true
	case tea.BatchMsg:
		for _, c := range msg {
			if result, ok := findBdResult(c); ok {
				return result, true
			}
		}
	}
	return BdResultMsg{}, false
}
