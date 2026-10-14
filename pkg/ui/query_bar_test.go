package ui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanderheijden86/b9s/internal/datasource"
	"github.com/vanderheijden86/b9s/pkg/model"
)

func TestSearchHistoryKeepsSelectionVisible(t *testing.T) {
	for _, key := range []tea.KeyType{tea.KeyUp, tea.KeyDown} {
		t.Run(tea.KeyMsg{Type: key}.String(), func(t *testing.T) {
			m := NewModel(searchViewportIssues(80, -1), "")
			m.focused = focusTree
			m.tree.SetSize(100, 16)
			m.queryState.SetText("Ordinary")
			m.queryState.saveHistory()
			m.queryState.StartEditing()
			m.queryState.SetText("title:Ordinary")
			if key == tea.KeyDown {
				m.queryState.recallHistory(true)
			}
			m.setQueryText(m.queryState.Text())
			selectedID := m.tree.flatList[len(m.tree.flatList)-1].Issue.ID
			m.tree.SelectByID(selectedID)
			m.tree.ensureCursorVisible()

			updated, _ := m.Update(tea.KeyMsg{Type: key})
			m = updated.(Model)

			if got := m.tree.GetSelectedID(); got != selectedID {
				t.Fatalf("selected issue = %q, want %q", got, selectedID)
			}
			start, end := m.tree.visibleRange()
			if m.tree.cursor < start || m.tree.cursor >= end {
				t.Fatalf("selected cursor %d outside visible rows [%d,%d)", m.tree.cursor, start, end)
			}
		})
	}
}

func TestSearchHistoryCheckoutWithoutJSONLPersists(t *testing.T) {
	for _, source := range []datasource.SourceType{datasource.SourceTypeDolt, datasource.SourceTypeSQLite} {
		t.Run(string(source), func(t *testing.T) {
			checkout := t.TempDir()
			beadsDir, _ := projectBeadsDir(checkout)
			if err := os.Mkdir(beadsDir, 0700); err != nil {
				t.Fatal(err)
			}
			open := func() Model {
				m := newSearchFilterModel(t, "").WithSourceType(source)
				m.activeProjectPath = checkout
				return m
			}
			m := typeKeys(open(), "/", "alpha")
			if m.beadsPath != "" {
				t.Fatal("test requires a checkout with no JSONL path")
			}
			_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

			reopened := typeKeys(open(), "/")
			updated, _ := reopened.Update(tea.KeyMsg{Type: tea.KeyUp})
			reopened = updated.(Model)
			if got := reopened.queryState.Text(); got != "alpha" {
				t.Fatalf("restart recalled %q, want alpha", got)
			}
			if _, err := os.Stat(filepath.Join(beadsDir, "search-history.json")); err != nil {
				t.Fatalf("checkout history file: %v", err)
			}
		})
	}
}

func TestSearchHistoryKeys(t *testing.T) {
	m := newSearchFilterModel(t, "")
	key := func(k tea.KeyType) {
		t.Helper()
		updated, _ := m.Update(tea.KeyMsg{Type: k})
		m = updated.(Model)
	}
	for _, query := range []string{"title:", "label:"} {
		m = typeKeys(m, "/", query)
		key(tea.KeyEnter)
		key(tea.KeyEsc)
	}
	m = typeKeys(m, "/", "status:")
	m.tree.SelectByID("bv-3")
	for _, step := range []struct {
		key  tea.KeyType
		want string
	}{
		{tea.KeyUp, "label:"},
		{tea.KeyUp, "title:"},
		{tea.KeyUp, "title:"},
		{tea.KeyDown, "label:"},
		{tea.KeyDown, "status:"},
		{tea.KeyDown, "status:"},
	} {
		key(step.key)
		if got := m.queryState.Text(); got != step.want {
			t.Fatalf("key %v: query = %q, want %q", step.key, got, step.want)
		}
		if got := m.tree.GetSelectedID(); got != "bv-3" {
			t.Fatalf("history key moved selection to %q", got)
		}
	}
}

func TestSearchHistoryEscapeDoesNotSubmitQuery(t *testing.T) {
	m := newSearchFilterModel(t, "")
	m = typeKeys(m, "/", "alpha")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	m = typeKeys(m, "/", "draft")
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(Model)
	if got := m.queryState.Text(); got != "draft" {
		t.Fatalf("Escape submitted canceled query: %q", got)
	}
}

func TestSlashStartsSharedQueryEditingInEveryView(t *testing.T) {
	tests := []struct {
		name  string
		focus focus
		setup func(*Model)
	}{
		{name: "tree", focus: focusTree, setup: func(m *Model) { m.treeViewActive = true }},
		{name: "list", focus: focusList, setup: func(m *Model) { m.treeViewActive = false }},
		{name: "board", focus: focusBoard, setup: func(m *Model) { m.isBoardView = true }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newSearchFilterModel(t, "")
			m.focused = tt.focus
			tt.setup(&m)

			m = typeKeys(m, "/")

			if got := m.queryState.Mode(); got != QueryEditing {
				t.Fatalf("query mode = %v, want editing", got)
			}
			if m.tree.IsSearchMode() {
				t.Error("tree-local search mode should not own the shared query")
			}
			if m.board.IsSearchMode() {
				t.Error("board-local search mode should not own the shared query")
			}
		})
	}
}

func TestQueryBarIsHiddenUntilSlashStartsEditing(t *testing.T) {
	m := newSearchFilterModel(t, "")

	if bar := m.renderUnifiedTitleBar(80); bar != "" {
		t.Fatalf("idle query bar = %q, want hidden", stripANSI(bar))
	}

	m = typeKeys(m, "/")
	if bar := m.renderUnifiedTitleBar(80); bar == "" {
		t.Fatal("slash did not reveal the query bar")
	}
}

func TestAcceptedQueryStaysVisibleAsChip(t *testing.T) {
	m := newSearchFilterModel(t, "")
	m = typeKeys(m, "/", "l", "a", "n", "e")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	bar := stripANSI(m.renderUnifiedTitleBar(80))
	if !strings.Contains(bar, "/ lane") {
		t.Fatalf("accepted query bar = %q, want a chip showing / lane", bar)
	}
	if strings.Contains(bar, "█") {
		t.Fatalf("accepted query bar = %q, want no editing cursor", bar)
	}
	if got := m.queryState.Text(); got != "lane" {
		t.Fatalf("accepted query = %q, want lane", got)
	}
}

func TestIdleLayoutDoesNotReserveSearchFieldRows(t *testing.T) {
	m := newSearchFilterModel(t, "")
	m.height = 40

	idleHeight := m.bodyHeight()
	m = typeKeys(m, "/")
	editingHeight := m.bodyHeight()

	if got := idleHeight - editingHeight; got != unifiedQueryBarHeight {
		t.Fatalf("idle layout restores %d rows, want %d", got, unifiedQueryBarHeight)
	}
}

func TestQueryBarUsesSubduedDarkGreenBorder(t *testing.T) {
	if got := unifiedQueryBorder.Dark; got != "#1F5E3B" {
		t.Fatalf("dark query border = %q, want subdued dark green", got)
	}
}

func TestSharedQueryFiltersByIDAndPersistsAfterAccept(t *testing.T) {
	m := newSearchFilterModel(t, "")
	m = typeKeys(m, "/", "i", "d", ":", "b", "v", "-", "3")

	if got := m.queryState.Text(); got != "id:bv-3" {
		t.Fatalf("query text = %q, want id:bv-3", got)
	}
	if got := len(m.list.Items()); got != 1 {
		t.Fatalf("filtered list count = %d, want 1", got)
	}
	if got := m.tree.NodeCount(); got != 1 {
		t.Fatalf("filtered tree count = %d, want 1", got)
	}
	if got := m.board.SearchQuery(); got != "id:bv-3" {
		t.Fatalf("board query = %q, want id:bv-3", got)
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if got := m.queryState.Mode(); got != QueryIdle {
		t.Fatalf("query mode = %v, want idle after enter", got)
	}
	if got := m.queryState.Text(); got != "id:bv-3" {
		t.Fatalf("accepted query = %q, want it to persist", got)
	}

	if bar := stripANSI(m.renderUnifiedTitleBar(120)); !strings.Contains(bar, "/ id:bv-3") {
		t.Errorf("accepted query bar %q should show the active filter as a chip", bar)
	}
}

func TestEscapeClearsAcceptedSharedQuery(t *testing.T) {
	m := newSearchFilterModel(t, "")
	m = typeKeys(m, "/", "b", "e", "t", "a")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)

	if !m.queryState.Empty() || m.queryState.Text() != "" {
		t.Fatalf("query was not cleared: %q", m.queryState.Text())
	}
	if got := len(m.list.Items()); got != 4 {
		t.Fatalf("list count after clearing = %d, want 4", got)
	}
	if got := m.tree.NodeCount(); got != 4 {
		t.Fatalf("tree count after clearing = %d, want 4", got)
	}
}

func TestEscapeClearsAcceptedEmptyFacetQuery(t *testing.T) {
	m := newSearchFilterModel(t, "")
	m = typeKeys(m, "/", "l", "a", "b", "e", "l", ":")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)

	if got := m.queryState.Text(); got != "" {
		t.Fatalf("escape left accepted empty facet text %q", got)
	}
}

func TestQueryTabCompletesFieldsAndValues(t *testing.T) {
	m := newSearchFilterModel(t, "")
	m = typeKeys(m, "/", "s", "t", "a")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	if got := m.queryState.Text(); got != "status:" {
		t.Fatalf("field completion = %q, want status:", got)
	}

	m = typeKeys(m, "o")
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	if got := m.queryState.Text(); got != "status:open" {
		t.Fatalf("value completion = %q, want status:open", got)
	}
}

func TestQueryBarKeepsSearchReadableWithLegacyFilters(t *testing.T) {
	m := newSearchFilterModel(t, "ui")
	m.currentFilter = "open"
	m.assigneeFilter = "ann"
	m.setQueryText("id:bv")
	m.queryState.StartEditing()

	bar := stripANSI(m.renderUnifiedTitleBar(160))
	if !strings.Contains(bar, "/ id:bv") {
		t.Errorf("query bar %q does not contain the active search", bar)
	}
	for _, hiddenFilter := range []string{"[status:open]", "[label:ui]", "[assignee:ann]"} {
		if strings.Contains(bar, hiddenFilter) {
			t.Errorf("query bar %q contains legacy filter chip %q", bar, hiddenFilter)
		}
	}
}

func TestQueryBarUsesCleanBorderedSearchField(t *testing.T) {
	m := newSearchFilterModel(t, "ui")
	m.currentFilter = "open"
	m.assigneeFilter = "ann"
	m.setQueryText("lane-a")
	m.queryState.StartEditing()

	bar := stripANSI(m.renderUnifiedTitleBar(80))
	lines := strings.Split(bar, "\n")
	if len(lines) != 3 {
		t.Fatalf("search field height = %d, want 3 lines: %q", len(lines), bar)
	}
	if !strings.HasPrefix(lines[0], "╭") || !strings.HasSuffix(lines[0], "╮") {
		t.Errorf("search field top border is incomplete: %q", lines[0])
	}
	if !strings.HasPrefix(lines[2], "╰") || !strings.HasSuffix(lines[2], "╯") {
		t.Errorf("search field bottom border is incomplete: %q", lines[2])
	}
	if !strings.Contains(lines[1], "/ lane-a█") {
		t.Errorf("search field does not contain query and cursor: %q", lines[1])
	}
	for _, clutter := range []string{"WHERE / FILTER", "ORDER BY", "[status:", "[label:", "[assignee:", "[project:", "⇥"} {
		if strings.Contains(bar, clutter) {
			t.Errorf("search field contains %q clutter: %q", clutter, bar)
		}
	}
}

func TestTreeQuickFilterRemainsActiveWithCleanSearchBar(t *testing.T) {
	m := newSearchFilterModel(t, "")
	m = typeKeys(m, "o")

	if got := m.currentFilter; got != "open" {
		t.Fatalf("model filter = %q, want open", got)
	}
	bar := stripANSI(m.renderUnifiedTitleBar(120))
	if strings.Contains(bar, "[status:open]") {
		t.Fatalf("query bar %q includes status-filter clutter", bar)
	}
}

func TestTreeFilterBranchShortcutCyclesDownTheParentPath(t *testing.T) {
	issues := []model.Issue{
		{ID: "epic-a", Title: "First epic", IssueType: model.TypeEpic, Status: model.StatusOpen},
		childOf("epic-a1", "Child epic", "epic-a", model.TypeEpic, model.StatusOpen),
		childOf("feature-a", "Nested feature", "epic-a1", model.TypeFeature, model.StatusOpen),
		childOf("task-a", "Nested task", "feature-a", model.TypeTask, model.StatusOpen),
		childOf("leaf-x", "Leaf feature", "epic-a1", model.TypeFeature, model.StatusOpen),
		childOf("feature-x", "Sibling feature", "epic-a", model.TypeFeature, model.StatusOpen),
		{ID: "epic-b", Title: "Second epic", IssueType: model.TypeEpic, Status: model.StatusOpen},
		childOf("task-b", "Other task", "epic-b", model.TypeTask, model.StatusOpen),
		{ID: "standalone", Title: "Standalone task", IssueType: model.TypeTask, Status: model.StatusOpen},
	}

	type step struct {
		query   string
		visible []string
	}
	all := []string{"epic-a", "epic-a1", "feature-a", "task-a", "leaf-x", "feature-x", "epic-b", "task-b", "standalone"}
	rootA := step{"branch:epic-a", []string{"epic-a", "epic-a1", "feature-a", "task-a", "leaf-x", "feature-x"}}
	childA := step{"branch:epic-a1", []string{"epic-a", "epic-a1", "feature-a", "task-a", "leaf-x"}}
	featureA := step{"branch:feature-a", []string{"epic-a", "epic-a1", "feature-a", "task-a"}}
	cleared := step{"", all}

	tests := []struct {
		name       string
		selectedID string
		steps      []step
	}{
		{name: "task under feature", selectedID: "task-a", steps: []step{rootA, childA, featureA, cleared}},
		{name: "feature with children", selectedID: "feature-a", steps: []step{rootA, childA, featureA, cleared}},
		{name: "leaf feature", selectedID: "leaf-x", steps: []step{rootA, childA, cleared}},
		{name: "child epic", selectedID: "epic-a1", steps: []step{rootA, childA, cleared}},
		{name: "top-level epic", selectedID: "epic-a", steps: []step{rootA, cleared}},
		{name: "standalone task", selectedID: "standalone", steps: []step{{"branch:standalone", []string{"standalone"}}, cleared}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewModel(issues, "")
			m.tree.Build(issues)
			m.tree.ExpandAll()
			m.focused = focusTree
			m.treeViewActive = true
			if !m.tree.SelectByID(tt.selectedID) {
				t.Fatalf("could not select %q", tt.selectedID)
			}

			for i, want := range tt.steps {
				m = typeKeys(m, "f")

				if got := m.queryState.Text(); got != want.query {
					t.Fatalf("press %d: branch query = %q, want %q", i+1, got, want.query)
				}
				bar := stripANSI(m.renderUnifiedTitleBar(100))
				if want.query == "" {
					if bar != "" {
						t.Fatalf("press %d: query bar still visible: %q", i+1, bar)
					}
				} else {
					if got := m.queryState.Mode(); got != QueryIdle {
						t.Fatalf("press %d: query mode = %v, want accepted query", i+1, got)
					}
					if !strings.Contains(bar, "/ "+want.query) || strings.Contains(bar, "█") {
						t.Fatalf("press %d: query bar = %q, want accepted %q", i+1, bar, want.query)
					}
				}
				gotIDs := treeVisibleIDs(&m.tree)
				wantIDs := append([]string(nil), want.visible...)
				sort.Strings(gotIDs)
				sort.Strings(wantIDs)
				if strings.Join(gotIDs, ",") != strings.Join(wantIDs, ",") {
					t.Fatalf("press %d: visible = %v, want %v", i+1, gotIDs, want.visible)
				}
				if got := m.tree.GetSelectedID(); got != tt.selectedID {
					t.Fatalf("press %d: selected %q, want original row %q", i+1, got, tt.selectedID)
				}
			}
		})
	}
}

// The ids repeat the dell-gb10 tree in which a feature's level kept showing a
// sibling feature: 25s.2.1 is a child of 25s.1, and its id contains 25s.2.
func TestTreeFilterBranchShortcutHidesIssuesWhoseIDContainsTheLevel(t *testing.T) {
	issues := []model.Issue{
		{ID: "doc-25s", Title: "Prefill the defect form", IssueType: model.TypeEpic, Status: model.StatusOpen},
		childOf("doc-25s.1", "Clean photo dataset", "doc-25s", model.TypeFeature, model.StatusOpen),
		childOf("doc-25s.2.1", "Honest defect test set", "doc-25s.1", model.TypeTask, model.StatusClosed),
		childOf("doc-25s.2.2", "Remove non-defect photos", "doc-25s.1", model.TypeTask, model.StatusOpen),
		childOf("doc-25s.2", "Defect test runs", "doc-25s", model.TypeFeature, model.StatusOpen),
		childOf("doc-e21.11", "Test infra defect recognition", "doc-25s.2", model.TypeTask, model.StatusClosed),
		childOf("doc-e21.11.3", "Tune prompt form-v2", "doc-25s.2", model.TypeTask, model.StatusClosed),
	}
	m := NewModel(issues, "")
	m.tree.Build(issues)
	m.tree.ExpandAll()
	m.focused = focusTree
	m.treeViewActive = true
	if !m.tree.SelectByID("doc-e21.11.3") {
		t.Fatal("could not select doc-e21.11.3")
	}

	m = typeKeys(m, "f", "f")

	if got := m.queryState.Text(); got != "branch:doc-25s.2" {
		t.Fatalf("second f query = %q, want branch:doc-25s.2", got)
	}
	gotIDs := treeVisibleIDs(&m.tree)
	sort.Strings(gotIDs)
	want := "doc-25s,doc-25s.2,doc-e21.11,doc-e21.11.3"
	if strings.Join(gotIDs, ",") != want {
		t.Fatalf("second f visible = %v, want %s", gotIDs, want)
	}
}

func TestTreeFilterBranchShortcutRestartsAtRootAfterAnotherQuery(t *testing.T) {
	issues := []model.Issue{
		{ID: "epic-a", Title: "First epic", IssueType: model.TypeEpic, Status: model.StatusOpen},
		childOf("feature-a", "Nested feature", "epic-a", model.TypeFeature, model.StatusOpen),
		childOf("task-a", "Nested task", "feature-a", model.TypeTask, model.StatusOpen),
	}
	m := NewModel(issues, "")
	m.tree.Build(issues)
	m.tree.ExpandAll()
	m.focused = focusTree
	m.treeViewActive = true
	m.setQueryText("Nested")
	if !m.tree.SelectByID("task-a") {
		t.Fatal("could not select task-a")
	}

	m = typeKeys(m, "f")

	if got := m.queryState.Text(); got != "branch:epic-a" {
		t.Fatalf("f after a typed query = %q, want branch:epic-a", got)
	}
}
