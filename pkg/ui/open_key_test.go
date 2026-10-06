package ui

import (
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// O opens the selected issue in the editor from every view, so the key has one
// meaning wherever the user presses it.
func TestOKeyOpensSelectedIssueInEveryView(t *testing.T) {
	t.Setenv("B9S_TEST_MODE", "1")
	t.Setenv("TMPDIR", t.TempDir())
	for _, view := range []string{"tree", "board", "graph", "list"} {
		t.Run(view, func(t *testing.T) {
			m := detailFooterModel(t, "list")
			m.beadsPath = ""
			m.activeProjectPath = ""
			switch view {
			case "tree":
				m.treeViewActive = true
				m.tree.SelectByID("ch-2")
				m.focused = focusTree
			case "board":
				m.isBoardView = true
				m.board.SelectIssueByID("ch-2")
				m.focused = focusBoard
			case "graph":
				m.graph.SetIssues(m.issues)
				m.graph.SelectIssue("ch-2")
				m.isGraphView = true
				m.focused = focusGraph
			case "list":
				m.focused = focusList
			}
			want := m.getSelectedIssue()
			if want == nil {
				t.Fatal("setup selected no issue")
			}

			m = pressKeys(m, runeKey("O"))

			if m.statusIsError || !strings.Contains(m.statusMsg, "Would open "+want.ID) {
				t.Fatalf("expected O to open %s, status %q (error %v)", want.ID, m.statusMsg, m.statusIsError)
			}
			body, err := os.ReadFile(m.OpenedIssueFile())
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(body), want.Title) {
				t.Errorf("file does not hold %s:\n%s", want.ID, body)
			}
		})
	}
}

// Occur lives on ctrl+o in the tree: it shows only the matches of the last
// search, and O no longer touches it.
func TestTreeCtrlOTogglesOccur(t *testing.T) {
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
		t.Fatal("O entered occur mode; it must only open the editor")
	}

	m = pressKeys(m, tea.KeyMsg{Type: tea.KeyCtrlO})
	if !m.tree.IsOccurMode() {
		t.Fatalf("ctrl+o did not enter occur mode, status %q", m.statusMsg)
	}
	m = pressKeys(m, tea.KeyMsg{Type: tea.KeyCtrlO})
	if m.tree.IsOccurMode() {
		t.Fatal("ctrl+o again did not leave occur mode")
	}
}
