// tree_search_filter_test.go - Free-text search must compose with active
// label/assignee/status filters instead of dropping them (bd-oe1y).
package ui

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanderheijden86/beadwork/pkg/model"
)

// searchFilterIssues returns four issues split across two labels and two
// assignees so filter/search composition is observable.
func searchFilterIssues() []model.Issue {
	return []model.Issue{
		{ID: "bv-1", Title: "Alpha bug work", Priority: 1, IssueType: model.TypeTask, Labels: []string{"bug"}, Assignee: "ann", Status: model.StatusOpen},
		{ID: "bv-2", Title: "Alpha feature work", Priority: 1, IssueType: model.TypeTask, Labels: []string{"feature"}, Assignee: "bob", Status: model.StatusOpen},
		{ID: "bv-3", Title: "Beta bug work", Priority: 1, IssueType: model.TypeTask, Labels: []string{"bug"}, Assignee: "ann", Status: model.StatusOpen},
		{ID: "bv-4", Title: "Beta feature work", Priority: 1, IssueType: model.TypeTask, Labels: []string{"feature"}, Assignee: "bob", Status: model.StatusOpen},
	}
}

// newSearchFilterModel returns a sized Model with a label filter already applied,
// mirroring what the label picker does when a label is selected.
func newSearchFilterModel(t *testing.T, label string) Model {
	t.Helper()
	m := NewModel(searchFilterIssues(), "")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)
	m.tree.SetBeadsDir(filepath.Join(t.TempDir(), ".beads"))

	if label != "" {
		m.labelFilter = label
		m.applyFilter()
		m.tree.SetLabelFilter(label)
	}
	m.focused = focusTree
	return m
}

// typeKeys feeds each string as an individual key press through Update.
func typeKeys(m Model, keys ...string) Model {
	for _, k := range keys {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)})
		m = updated.(Model)
	}
	return m
}

func treeVisibleIDs(t *TreeModel) []string {
	ids := make([]string, 0, len(t.flatList))
	for _, n := range t.flatList {
		if n != nil && n.Issue != nil {
			ids = append(ids, n.Issue.ID)
		}
	}
	return ids
}

func treeMatchIDs(t *TreeModel) []string {
	ids := make([]string, 0, len(t.searchMatches))
	for _, n := range t.searchMatches {
		if n != nil && n.Issue != nil {
			ids = append(ids, n.Issue.ID)
		}
	}
	return ids
}

func TestTreeFilteredXRay(t *testing.T) {
	filters := []struct {
		name  string
		apply func(*TreeModel)
	}{
		{"label", func(tree *TreeModel) { tree.SetLabelFilter("bug") }},
		{"assignee", func(tree *TreeModel) { tree.SetAssigneeFilter("ann") }},
		{"status", func(tree *TreeModel) { tree.ApplyFilter("open") }},
		{"query", func(tree *TreeModel) { tree.SetIssueQuery(ParseIssueQuery("needle")) }},
	}
	for _, filter := range filters {
		t.Run(filter.name, func(t *testing.T) {
			issues := []model.Issue{
				{ID: "root", Title: "Root", Status: model.StatusClosed, IssueType: model.TypeEpic},
				{ID: "root.1", Title: "Branch", Status: model.StatusClosed, IssueType: model.TypeEpic},
				{ID: "root.1.1", Title: "Needle inside", Status: model.StatusOpen, Labels: []string{"bug"}, Assignee: "ann"},
				{ID: "root.1.2", Title: "Hidden child", Status: model.StatusClosed},
				{ID: "root.2", Title: "Needle sibling", Status: model.StatusOpen, Labels: []string{"bug"}, Assignee: "ann"},
				{ID: "other", Title: "Needle other root", Status: model.StatusOpen, Labels: []string{"bug"}, Assignee: "ann"},
			}
			for i, parent := range map[int]string{1: "root", 2: "root.1", 3: "root.1", 4: "root"} {
				issues[i].Dependencies = []*model.Dependency{{IssueID: issues[i].ID, DependsOnID: parent, Type: model.DepParentChild}}
			}
			m := NewModel(issues, "")
			updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
			m = updated.(Model)
			m.tree.SetBeadsDir(t.TempDir())
			m.focused = focusTree
			filter.apply(&m.tree)

			assertIDs := func(want ...string) {
				t.Helper()
				got := treeVisibleIDs(&m.tree)
				slices.Sort(got)
				slices.Sort(want)
				if !slices.Equal(got, want) {
					t.Errorf("visible IDs = %v, want %v", got, want)
				}
			}
			assertIDs("root", "root.1", "root.1.1", "root.2", "other")
			for _, exit := range []string{"toggle", "explicit"} {
				if !m.tree.SelectByID("root.1") {
					t.Fatal("filtered context ancestor is not selectable")
				}
				m = typeKeys(m, "x")
				if !m.tree.IsXRayMode() {
					t.Fatal("x did not enter XRay")
				}
				assertIDs("root.1", "root.1.1")
				if exit == "toggle" {
					m = typeKeys(m, "x")
				} else {
					m.tree.ExitXRay()
				}
				if m.tree.IsXRayMode() {
					t.Fatalf("%s did not exit XRay", exit)
				}
				assertIDs("root", "root.1", "root.1.1", "root.2", "other")
			}
		})
	}
}

// TestTreeSearchTypingKeepsLabelFilter is the regression test for the reported
// bug: typing a query containing "a" fired the global "clear all filters"
// shortcut because handleTreeKeys runs after the global key switch (bd-oe1y).
func TestTreeSearchTypingKeepsLabelFilter(t *testing.T) {
	m := newSearchFilterModel(t, "bug")
	m = typeKeys(m, "/", "a", "l", "p", "h", "a")

	if got := m.tree.SearchQuery(); got != "alpha" {
		t.Errorf("search query = %q, want %q (keys leaked to global shortcuts)", got, "alpha")
	}
	if m.labelFilter != "bug" {
		t.Errorf("model label filter = %q, want %q (filter dropped by search)", m.labelFilter, "bug")
	}
	if got := m.tree.GetLabelFilter(); got != "bug" {
		t.Errorf("tree label filter = %q, want %q (filter dropped by search)", got, "bug")
	}
	if got := m.tree.NodeCount(); got != 1 {
		t.Errorf("visible tree nodes = %d (%v), want the 1 node matching both label and query", got, treeVisibleIDs(&m.tree))
	}
}

// TestTreeSearchTypingKeepsStatusAndAssigneeFilters covers the other filters
// reachable from the global switch ("o"/"c"/"r" status keys, "a" clear-all).
func TestTreeSearchTypingKeepsStatusAndAssigneeFilters(t *testing.T) {
	m := newSearchFilterModel(t, "")
	m.assigneeFilter = "ann"
	m.currentFilter = "open"
	m.applyFilter()
	m.tree.SetAssigneeFilter("ann")
	m.tree.ApplyFilter("open")

	m = typeKeys(m, "/", "c", "o", "r", "a")

	if got := m.tree.SearchQuery(); got != "cora" {
		t.Errorf("search query = %q, want %q", got, "cora")
	}
	if m.assigneeFilter != "ann" {
		t.Errorf("assignee filter = %q, want %q", m.assigneeFilter, "ann")
	}
	if m.currentFilter != "open" {
		t.Errorf("status filter = %q, want %q", m.currentFilter, "open")
	}
}

// TestTreeSearchTypingDigitsDoesNotSwitchFilters verifies number keys go into
// the query instead of toggling label/project selection ("0" cleared filters).
func TestTreeSearchTypingDigitsDoesNotSwitchFilters(t *testing.T) {
	m := newSearchFilterModel(t, "bug")
	m = typeKeys(m, "/", "b", "d", "1", "0")

	if got := m.tree.SearchQuery(); got != "bd10" {
		t.Errorf("search query = %q, want %q", got, "bd10")
	}
	if m.labelFilter != "bug" {
		t.Errorf("label filter = %q, want %q", m.labelFilter, "bug")
	}
}

// TestTreeSearchTypingDoesNotOpenOverlays verifies keys bound to global
// overlays are typed into the query while search mode is active.
func TestTreeSearchTypingDoesNotOpenOverlays(t *testing.T) {
	m := newSearchFilterModel(t, "bug")
	m = typeKeys(m, "/", "?", "H", "P", "D")

	if got := m.tree.SearchQuery(); got != "?HPD" {
		t.Errorf("search query = %q, want %q", got, "?HPD")
	}
	if m.showHelp {
		t.Error("help overlay opened while typing a search query")
	}
	if m.showDBHealth {
		t.Error("database health popup opened while typing a search query")
	}
}

// TestTreeSearchEscapeStillClearsSearch guards the escape hatch: the routing
// guard must not swallow esc/enter handling.
func TestTreeSearchEscapeStillClearsSearch(t *testing.T) {
	m := newSearchFilterModel(t, "bug")
	m = typeKeys(m, "/", "a")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)

	if m.tree.IsSearchMode() {
		t.Error("expected esc to leave search mode")
	}
	if m.tree.SearchQuery() != "" {
		t.Errorf("expected esc to clear query, got %q", m.tree.SearchQuery())
	}
	if m.labelFilter != "bug" {
		t.Errorf("label filter = %q, want %q", m.labelFilter, "bug")
	}
}

// TestTreeSearchEnterExitsSearchMode verifies enter keeps matches but closes
// the input bar.
func TestTreeSearchEnterExitsSearchMode(t *testing.T) {
	m := newSearchFilterModel(t, "bug")
	m = typeKeys(m, "/", "b", "u", "g")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	if m.tree.IsSearchMode() {
		t.Error("expected enter to exit search mode")
	}
	if m.tree.SearchQuery() != "bug" {
		t.Errorf("expected enter to keep the query, got %q", m.tree.SearchQuery())
	}
}

// TestBoardSearchTypingKeepsLabelFilter covers the identical key leak in the
// board's search mode (bd-oe1y).
func TestBoardSearchTypingKeepsLabelFilter(t *testing.T) {
	m := newSearchFilterModel(t, "bug")
	m.isBoardView = true
	m.focused = focusBoard
	m.refreshBoardAndGraphForCurrentFilter()

	m = typeKeys(m, "/", "a", "l", "p", "h", "a")

	if got := m.board.SearchQuery(); got != "alpha" {
		t.Errorf("board search query = %q, want %q", got, "alpha")
	}
	if m.labelFilter != "bug" {
		t.Errorf("label filter = %q, want %q (filter dropped by board search)", m.labelFilter, "bug")
	}
}

// TestTreeSearchMatchesRespectLabelFilter verifies search results are scoped to
// the filtered set: an issue hidden by the label filter must never be reported
// as a match (it is not reachable with n/N either).
func TestTreeSearchMatchesRespectLabelFilter(t *testing.T) {
	tree := NewTreeModel(newTreeTestTheme())
	tree.SetBeadsDir(filepath.Join(t.TempDir(), ".beads"))
	tree.SetSize(120, 40)
	tree.Build(searchFilterIssues())
	tree.SetLabelFilter("bug")

	tree.EnterSearchMode()
	for _, ch := range "alpha" {
		tree.SearchAddChar(ch)
	}

	if got := tree.SearchMatchCount(); got != 1 {
		t.Errorf("match count = %d (%v), want 1 (bv-2 is excluded by the label filter)", got, treeMatchIDs(&tree))
	}
	if ids := treeMatchIDs(&tree); len(ids) != 1 || ids[0] != "bv-1" {
		t.Errorf("match IDs = %v, want [bv-1]", ids)
	}
	// Every reported match must be reachable: the cursor lands on the first one.
	if got := tree.GetSelectedID(); got != "bv-1" {
		t.Errorf("selected ID = %q, want %q", got, "bv-1")
	}
}

// TestTreeSearchMatchesRespectAssigneeFilter is the assignee counterpart.
func TestTreeSearchMatchesRespectAssigneeFilter(t *testing.T) {
	tree := NewTreeModel(newTreeTestTheme())
	tree.SetBeadsDir(filepath.Join(t.TempDir(), ".beads"))
	tree.SetSize(120, 40)
	tree.Build(searchFilterIssues())
	tree.SetAssigneeFilter("bob")

	tree.EnterSearchMode()
	for _, ch := range "work" {
		tree.SearchAddChar(ch)
	}

	if ids := treeMatchIDs(&tree); len(ids) != 2 {
		t.Errorf("match IDs = %v, want the 2 issues assigned to bob", ids)
	}
}

// TestTreeSearchMatchesRefreshWhenFilterChanges verifies matches are recomputed
// when a filter is applied after the search, so stale out-of-filter matches do
// not survive.
func TestTreeSearchMatchesRefreshWhenFilterChanges(t *testing.T) {
	tree := NewTreeModel(newTreeTestTheme())
	tree.SetBeadsDir(filepath.Join(t.TempDir(), ".beads"))
	tree.SetSize(120, 40)
	tree.Build(searchFilterIssues())

	tree.EnterSearchMode()
	for _, ch := range "alpha" {
		tree.SearchAddChar(ch)
	}
	if got := tree.SearchMatchCount(); got != 2 {
		t.Fatalf("unfiltered match count = %d, want 2", got)
	}

	tree.SetLabelFilter("bug")

	if got := tree.SearchMatchCount(); got != 1 {
		t.Errorf("match count after applying label filter = %d (%v), want 1", got, treeMatchIDs(&tree))
	}
}

// TestTreeOccurModeComposesWithLabelFilter verifies occur mode (search used as a
// filter) ANDs with the label filter instead of being ignored.
func TestTreeOccurModeComposesWithLabelFilter(t *testing.T) {
	tree := NewTreeModel(newTreeTestTheme())
	tree.SetBeadsDir(filepath.Join(t.TempDir(), ".beads"))
	tree.SetSize(120, 40)
	tree.Build(searchFilterIssues())
	tree.SetLabelFilter("bug")

	tree.EnterOccurMode("Alpha")

	if ids := treeVisibleIDs(&tree); len(ids) != 1 || ids[0] != "bv-1" {
		t.Errorf("occur + label filter visible = %v, want [bv-1]", ids)
	}

	tree.ExitOccurMode()
	if got := tree.NodeCount(); got != 2 {
		t.Errorf("after exiting occur, visible = %d (%v), want the 2 label-filtered nodes", got, treeVisibleIDs(&tree))
	}
}

func searchViewportIssues(count int, matchIndex int) []model.Issue {
	issues := make([]model.Issue, 0, count)
	base := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < count; i++ {
		title := fmt.Sprintf("Ordinary task %02d", i)
		if i == matchIndex {
			title = "Needle epic"
		}
		issues = append(issues, model.Issue{
			ID:        fmt.Sprintf("bd-%02d", i),
			Title:     title,
			Status:    model.StatusOpen,
			IssueType: model.TypeTask,
			CreatedAt: base.Add(time.Duration(count-i) * time.Minute),
		})
	}
	return issues
}

func TestTreeSearchRevealsMatchWithContext(t *testing.T) {
	tree := NewTreeModel(newTreeTestTheme())
	tree.SetBeadsDir(filepath.Join(t.TempDir(), ".beads"))
	tree.SetSize(100, 12)
	tree.Build(searchViewportIssues(40, 32))

	tree.EnterSearchMode()
	for _, ch := range "needle" {
		tree.SearchAddChar(ch)
	}

	start, _ := tree.visibleRange()
	row := tree.cursor - start
	if maxComfortableRow := tree.effectiveVisibleCount() / 2; row > maxComfortableRow {
		t.Fatalf("search match rendered at viewport row %d, want at or above %d", row, maxComfortableRow)
	}
}

func TestTreeSearchBarReservesViewportLine(t *testing.T) {
	tree := NewTreeModel(newTreeTestTheme())
	tree.SetBeadsDir(filepath.Join(t.TempDir(), ".beads"))
	tree.SetSize(100, 12)
	tree.Build(searchViewportIssues(40, 32))
	tree.EnterSearchMode()

	start, end := tree.visibleRange()
	visibleRows := end - start
	// Header, position indicator, and search bar each consume one line.
	if wantMax := tree.height - 3; visibleRows > wantMax {
		t.Fatalf("visible rows = %d, want at most %d while search bar is shown", visibleRows, wantMax)
	}
}

func TestTreeAcceptedSearchKeepsQueryIndicatorVisible(t *testing.T) {
	tree := NewTreeModel(newTreeTestTheme())
	tree.SetBeadsDir(filepath.Join(t.TempDir(), ".beads"))
	tree.SetSize(100, 12)
	tree.Build(searchViewportIssues(10, 5))
	tree.EnterSearchMode()
	for _, ch := range "needle" {
		tree.SearchAddChar(ch)
	}
	tree.ExitSearchMode()

	if view := tree.View(); !strings.Contains(view, "needle") {
		t.Fatalf("accepted search query is absent from tree view:\n%s", view)
	}
}
