package ui

import (
	"reflect"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func markRows(t *testing.T, m Model, ids ...string) Model {
	t.Helper()
	for _, id := range ids {
		m.tree.SelectByID(id)
		m, _ = pressBulkKey(t, m, tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")})
	}
	return m
}

func TestLowercaseUUnmarksCursorRow(t *testing.T) {
	m := markRows(t, newBulkMarkModel(t), "bd-1", "bd-2")
	m.tree.SelectByID("bd-1")

	m, _ = pressBulkKey(t, m, runeKey("u"))

	if got := m.tree.TreeMarkedIDs(); !reflect.DeepEqual(got, []string{"bd-2"}) {
		t.Fatalf("u must unmark only the cursor row, got %v", got)
	}
}

// u is unmark, not toggle: dired users press it over a region expecting every
// row to end up unmarked, whatever state each row started in.
func TestLowercaseUDoesNotMarkUnmarkedRow(t *testing.T) {
	m := newBulkMarkModel(t)

	m, _ = pressBulkKey(t, m, runeKey("u"))

	if got := m.tree.MarkedCount(); got != 0 {
		t.Fatalf("u on an unmarked row must leave it unmarked, %d marked", got)
	}
}

func TestUppercaseUClearsMarks(t *testing.T) {
	m := markRows(t, newBulkMarkModel(t), "bd-1", "bd-3")

	m, _ = pressBulkKey(t, m, runeKey("U"))

	if got := m.tree.MarkedCount(); got != 0 {
		t.Fatalf("U must clear marks, %d remain", got)
	}
}

func TestUppercaseVMarksRangeFromPreviousMark(t *testing.T) {
	m := markRows(t, newBulkMarkModel(t), "bd-1")
	m.tree.SelectByID("bd-3")

	m, _ = pressBulkKey(t, m, runeKey("V"))

	want := []string{"bd-1", "bd-2", "bd-3"}
	if got := m.tree.TreeMarkedIDs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("V must mark the range, want %v got %v", want, got)
	}
}
