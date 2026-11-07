package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vanderheijden86/b9s/pkg/model"
)

// findModeIssues holds two epics, so a query that hits one child of the first
// hides the whole second branch in filter mode.
func findModeIssues() []model.Issue {
	now := time.Now()
	issue := func(id, title, parent string, typ model.IssueType, age int) model.Issue {
		i := model.Issue{ID: id, Title: title, Status: model.StatusOpen, IssueType: typ,
			Priority: 2, CreatedAt: now.Add(time.Duration(age) * time.Second)}
		if parent != "" {
			i.Dependencies = []*model.Dependency{{IssueID: id, DependsOnID: parent, Type: model.DepParentChild}}
		}
		return i
	}
	return []model.Issue{
		issue("ep-a", "Alpha epic", "", model.TypeEpic, 9),
		issue("ta-1", "Wire the tunnel", "ep-a", model.TypeTask, 8),
		issue("ta-2", "Plain chore", "ep-a", model.TypeTask, 7),
		issue("ep-b", "Beta epic", "", model.TypeEpic, 6),
		issue("tb-1", "Other work", "ep-b", model.TypeTask, 5),
		issue("tb-2", "Second tunnel", "ep-b", model.TypeTask, 4),
	}
}

func findModeModel(t *testing.T) Model {
	t.Helper()
	m := NewModel(findModeIssues(), "")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = updated.(Model)
	m.treeViewActive = true
	m.focused = focusTree
	m.tree.ExpandAll()
	m.tree.SelectByID("ep-a")
	return m
}

func typeQuery(m Model, query string) Model {
	m = pressKeys(m, runeKey("/"))
	for _, r := range query {
		m = pressKeys(m, runeKey(string(r)))
	}
	return pressKeys(m, tea.KeyMsg{Type: tea.KeyEnter})
}

// / filters the tree as a k9s table does; Ctrl-F switches the same query to
// find, which keeps every row, as a k9s YAML or describe view does, and n/N
// still step through the hits.
func TestCtrlFSwitchesSearchBetweenFilterAndFind(t *testing.T) {
	t.Setenv("B9S_TEST_MODE", "1")
	m := findModeModel(t)
	all := m.TreeNodeCount()

	m = typeQuery(m, "tunnel")
	filtered := m.TreeNodeCount()
	if filtered >= all {
		t.Fatalf("filter mode shows %d of %d rows; / must hide branches without a hit", filtered, all)
	}

	m = pressKeys(m, tea.KeyMsg{Type: tea.KeyCtrlF})
	if !m.queryState.Find() {
		t.Fatal("ctrl+f did not switch the query to find mode")
	}
	if got := m.TreeNodeCount(); got != all {
		t.Fatalf("find mode shows %d rows, want all %d", got, all)
	}
	if got := m.tree.SearchMatchCount(); got != 2 {
		t.Fatalf("find mode has %d hits, want 2", got)
	}
	first := m.TreeSelectedID()
	m = pressKeys(m, runeKey("n"))
	second := m.TreeSelectedID()
	if first == second || (first != "ta-1" && first != "tb-2") || (second != "ta-1" && second != "tb-2") {
		t.Fatalf("n moved %q -> %q, want the two tunnel hits", first, second)
	}

	m = pressKeys(m, tea.KeyMsg{Type: tea.KeyCtrlF})
	if m.queryState.Find() {
		t.Fatal("ctrl+f again did not switch back to filter mode")
	}
	if got := m.TreeNodeCount(); got != filtered {
		t.Fatalf("back in filter mode the tree shows %d rows, want %d", got, filtered)
	}
}

// Find mode survives the query being edited and cleared, so a user who
// chose find keeps it for the next search.
func TestFindModeKeepsAcrossQueries(t *testing.T) {
	t.Setenv("B9S_TEST_MODE", "1")
	m := findModeModel(t)
	all := m.TreeNodeCount()
	m = pressKeys(m, tea.KeyMsg{Type: tea.KeyCtrlF})
	for _, r := range "epic" {
		m = pressKeys(m, runeKey(string(r)))
	}
	m = pressKeys(m, tea.KeyMsg{Type: tea.KeyEnter})
	if got := m.queryState.Text(); got != "epic" {
		t.Fatalf("query text = %q, want %q", got, "epic")
	}
	if got := m.TreeNodeCount(); got != all {
		t.Fatalf("find mode shows %d rows, want all %d", got, all)
	}
	m = pressKeys(m, tea.KeyMsg{Type: tea.KeyEsc})
	m = typeQuery(m, "tunnel")
	if !m.queryState.Find() || m.TreeNodeCount() != all {
		t.Fatalf("after Esc and a new query: find %v, rows %d of %d", m.queryState.Find(), m.TreeNodeCount(), all)
	}
}

// Ctrl-F is no longer page down in the tree: it must not move the cursor.
func TestCtrlFDoesNotPageTheTree(t *testing.T) {
	t.Setenv("B9S_TEST_MODE", "1")
	m := findModeModel(t)
	before := m.TreeSelectedID()
	m = pressKeys(m, tea.KeyMsg{Type: tea.KeyCtrlF})
	if got := m.TreeSelectedID(); got != before {
		t.Fatalf("ctrl+f moved the cursor from %s to %s", before, got)
	}
}

// The query bar names find mode, since a query that hides nothing would
// otherwise look like a filter that matched everything.
func TestQueryBarNamesFindMode(t *testing.T) {
	t.Setenv("B9S_TEST_MODE", "1")
	m := typeQuery(findModeModel(t), "tunnel")
	if bar := m.renderUnifiedTitleBar(100); strings.Contains(bar, "find /") {
		t.Fatalf("filter mode bar says find:\n%s", bar)
	}
	m = pressKeys(m, tea.KeyMsg{Type: tea.KeyCtrlF})
	if bar := m.renderUnifiedTitleBar(100); !strings.Contains(bar, "find / tunnel") {
		t.Fatalf("find mode bar does not name the mode:\n%s", bar)
	}
}

// Ctrl-F with no query opens the query bar in find mode, as / opens it in
// filter mode, so the letters typed next are query text. Without the open bar,
// t, u and n would run their tree commands instead (bd-8umy).
func TestCtrlFWithoutQueryOpensQueryBarInFindMode(t *testing.T) {
	t.Setenv("B9S_TEST_MODE", "1")
	m := findModeModel(t)
	all := m.TreeNodeCount()

	m = pressKeys(m, tea.KeyMsg{Type: tea.KeyCtrlF})
	for _, r := range "tunnel" {
		m = pressKeys(m, runeKey(string(r)))
	}

	if got := m.queryState.Text(); got != "tunnel" {
		t.Fatalf("query text = %q, want %q: typed keys ran commands instead", got, "tunnel")
	}
	if !m.queryState.Find() {
		t.Fatal("ctrl+f with no query did not choose find mode")
	}
	if m.tree.IsFlatMode() || !m.treeViewActive || m.focused != focusTree {
		t.Fatal("typed query text changed the view")
	}
	if got := m.TreeNodeCount(); got != all {
		t.Fatalf("find mode shows %d rows, want all %d", got, all)
	}
	if got := m.tree.SearchMatchCount(); got != 2 {
		t.Fatalf("find mode has %d hits, want 2", got)
	}
}
