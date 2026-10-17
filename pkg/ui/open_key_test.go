package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// Occur lives on & in the tree, as less's &pattern shows only the matching
// lines: it shows only the matches of the last search, and neither O nor
// ctrl+o touches it.
func TestTreeAmpersandTogglesOccur(t *testing.T) {
	t.Setenv("B9S_TEST_MODE", "1")
	t.Setenv("TMPDIR", t.TempDir())
	m := detailFooterModel(t, "list")
	m.treeViewActive = true
	m.focused = focusTree
	m.tree.EnterSearchMode()
	for _, r := range "ch-2" {
		m.tree.SearchAddChar(r)
	}
	m.tree.ExitSearchMode()
	if m.tree.SearchQuery() == "" {
		t.Fatal("setup: closing the search box dropped the query")
	}

	m = pressKeys(m, runeKey("O"))
	if m.tree.IsOccurMode() {
		t.Fatal("O entered occur mode; occur is on &")
	}

	m = pressKeys(m, tea.KeyMsg{Type: tea.KeyCtrlO})
	if m.tree.IsOccurMode() {
		t.Fatal("ctrl+o entered occur mode; occur is on &")
	}

	m = pressKeys(m, runeKey("&"))
	if !m.tree.IsOccurMode() {
		t.Fatalf("& did not enter occur mode, status %q", m.statusMsg)
	}
	m = pressKeys(m, runeKey("&"))
	if m.tree.IsOccurMode() {
		t.Fatal("& again did not leave occur mode")
	}
}
