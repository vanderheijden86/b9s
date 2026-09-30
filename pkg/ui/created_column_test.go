// created_column_test.go - The time columns (Created, Updated, Deferred, Due)
// in the tree and the flat list: which show by default, what their headers
// say, the relative and absolute formats T switches between, and the Deferred
// sort.
package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vanderheijden86/b9s/pkg/config"
	"github.com/vanderheijden86/b9s/pkg/model"
)

var createdColumnTime = time.Date(2026, 9, 30, 14, 5, 42, 0, time.Local)

func createdColumnFixture() model.Issue {
	return model.Issue{
		ID: "bd-t1", Title: "Timestamp fixture", Status: model.StatusOpen,
		IssueType: model.TypeTask, Priority: 2,
		CreatedAt: createdColumnTime, UpdatedAt: createdColumnTime.Add(time.Hour),
	}
}

// relativeTimeFixture has a distinct relative value in every time column.
func relativeTimeFixture() model.Issue {
	now := time.Now()
	deferUntil := now.Add(3*24*time.Hour + time.Hour)
	due := now.Add(14*24*time.Hour + time.Hour)
	return model.Issue{
		ID: "bd-t2", Title: "Relative fixture", Status: model.StatusOpen,
		IssueType: model.TypeTask, Priority: 2,
		CreatedAt:  now.Add(-2*time.Hour - time.Minute),
		UpdatedAt:  now.Add(-5*time.Minute - 10*time.Second),
		DeferUntil: &deferUntil,
		DueDate:    &due,
	}
}

func timeColumnsTree(t *testing.T, width int, issues ...model.Issue) *TreeModel {
	t.Helper()
	tree := NewTreeModel(newTreeTestTheme())
	tree.Build(issues)
	tree.SetSize(width, 10)
	return &tree
}

func headerLine(tree *TreeModel) string {
	return stripANSI(tree.RenderHeader())
}

func rowFor(t *testing.T, tree *TreeModel, title string) string {
	t.Helper()
	for _, line := range strings.Split(stripANSI(tree.View()), "\n") {
		if strings.Contains(line, title) {
			return line
		}
	}
	t.Fatalf("no row for %q:\n%s", title, stripANSI(tree.View()))
	return ""
}

func TestTreeColumnPopup_ListsEveryTimeColumn(t *testing.T) {
	tree := NewTreeModel(newTreeTestTheme())
	tree.OpenColumnPopup()
	popup := stripANSI(tree.RenderColumnPopup())
	for _, name := range []string{"Created", "Updated", "Deferred", "Due"} {
		if !strings.Contains(popup, name) {
			t.Errorf("column popup missing %s: %q", name, popup)
		}
	}
}

func TestTreeTimeColumns_OnlyCreatedShowsByDefault(t *testing.T) {
	tree := timeColumnsTree(t, 160, relativeTimeFixture())
	header := headerLine(tree)
	if !strings.Contains(header, "CREATED") {
		t.Fatalf("CREATED should show by default: %q", header)
	}
	for _, hidden := range []string{"UPDATED", "DEFERRED", "DUE"} {
		if strings.Contains(header, hidden) {
			t.Errorf("%s should be hidden until chosen with |: %q", hidden, header)
		}
	}
}

func TestTreeTimeColumns_RelativeByDefault(t *testing.T) {
	tree := timeColumnsTree(t, 160, relativeTimeFixture())
	row := rowFor(t, tree, "Relative fixture")
	if !strings.Contains(row, "2h ago") {
		t.Fatalf("created column should show the age since created_at (2h ago): %q", row)
	}
}

// The column under a header must hold that header's field whatever the sort.
func TestTreeTimeColumns_HeadersNameTheirFieldNotTheSort(t *testing.T) {
	tree := timeColumnsTree(t, 160, relativeTimeFixture())
	tree.SetColumnPreference(TreeColumnUpdated, ColumnShow)
	tree.SetSort(SortFieldCreated, SortDescending)
	header := headerLine(tree)
	if !strings.Contains(header, "UPDATED") {
		t.Fatalf("UPDATED header missing: %q", header)
	}
	if strings.Count(header, "Created")+strings.Count(header, "CREATED") != 2 {
		t.Fatalf("want one CREATED column header and one Created sort badge: %q", header)
	}
	row := rowFor(t, tree, "Relative fixture")
	if !strings.Contains(row, "2h ago") || !strings.Contains(row, "5m ago") {
		t.Fatalf("row should show created (2h ago) and updated (5m ago): %q", row)
	}
}

func TestTreeHeader_SortBadgeFollowsIssueLabel(t *testing.T) {
	tree := timeColumnsTree(t, 160, relativeTimeFixture())
	tree.SetSort(SortFieldPriority, SortAscending)
	if header := headerLine(tree); !strings.Contains(header, "Issue [Priority ▲]") {
		t.Fatalf("sort badge should follow the Issue label: %q", header)
	}
}

func TestTreeTimeColumns_DeferredAndDueShowTheFuture(t *testing.T) {
	tree := timeColumnsTree(t, 160, relativeTimeFixture())
	tree.SetColumnPreference(TreeColumnDeferred, ColumnShow)
	tree.SetColumnPreference(TreeColumnDue, ColumnShow)
	header := headerLine(tree)
	if !strings.Contains(header, "DEFERRED") || !strings.Contains(header, "DUE") {
		t.Fatalf("DEFERRED and DUE headers missing: %q", header)
	}
	row := rowFor(t, tree, "Relative fixture")
	if !strings.Contains(row, "in 3d") || !strings.Contains(row, "in 2w") {
		t.Fatalf("deferred (in 3d) and due (in 2w) missing: %q", row)
	}
}

func TestTreeTimeColumns_EmptyDeferredCellIsBlank(t *testing.T) {
	issue := relativeTimeFixture()
	issue.DeferUntil = nil
	tree := timeColumnsTree(t, 160, issue)
	tree.SetColumnPreference(TreeColumnDeferred, ColumnShow)
	if header := headerLine(tree); !strings.Contains(header, "DEFERRED") {
		t.Fatalf("DEFERRED header missing: %q", header)
	}
	row := rowFor(t, tree, "Relative fixture")
	for _, word := range []string{"unknown", "now"} {
		if strings.Contains(row, word) {
			t.Fatalf("an issue without a defer date should leave the cell blank: %q", row)
		}
	}
}

func TestTreeTimeColumns_AbsoluteIsShortDateAndMinute(t *testing.T) {
	tree := timeColumnsTree(t, 160, createdColumnFixture())
	tree.ToggleTimeFormat()
	row := rowFor(t, tree, "Timestamp fixture")
	if !strings.Contains(row, "30-09 14:05") || strings.Contains(row, "2026-09-30") {
		t.Fatalf("absolute format should be DD-MM HH:MM without the year: %q", row)
	}
}

func TestTreeTimeColumns_MonthFirstLocale(t *testing.T) {
	tree := timeColumnsTree(t, 160, createdColumnFixture())
	tree.SetMonthFirst(true)
	tree.ToggleTimeFormat()
	row := rowFor(t, tree, "Timestamp fixture")
	if !strings.Contains(row, "09-30 14:05") || strings.Contains(row, "30-09") {
		t.Fatalf("a month-first locale should show MM-DD HH:MM: %q", row)
	}
}

func TestWithMonthFirst_ReachesTheTree(t *testing.T) {
	m := NewModel([]model.Issue{createdColumnFixture()}, "").WithMonthFirst(true)
	m.tree.SetSize(160, 10)
	m.focused = focusTree

	m = m.handleTreeKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("T")})
	if view := stripANSI(m.tree.View()); !strings.Contains(view, "09-30 14:05") {
		t.Fatalf("WithMonthFirst(true) should give month-first absolute times:\n%s", view)
	}
}

func TestTreeTimeColumns_RowsFitTheWidth(t *testing.T) {
	for _, absolute := range []bool{false, true} {
		tree := timeColumnsTree(t, 100, relativeTimeFixture())
		if absolute {
			tree.ToggleTimeFormat()
		}
		tree.ToggleWide()
		for _, line := range strings.Split(tree.View(), "\n") {
			if w := lipgloss.Width(line); w > 100 {
				t.Fatalf("absolute=%v: line is %d cells wide, over the 100-cell tree:\n%s", absolute, w, stripANSI(line))
			}
		}
	}
}

func TestTreeKeyT_TogglesTimeFormat(t *testing.T) {
	m := NewModel([]model.Issue{createdColumnFixture()}, "")
	m.tree.SetSize(160, 10)
	m.focused = focusTree

	m = m.handleTreeKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("T")})
	if view := stripANSI(m.tree.View()); !strings.Contains(view, "30-09 14:05") || strings.Contains(view, "2026-09-30") {
		t.Fatalf("T should switch to absolute times without the year:\n%s", view)
	}
	m = m.handleTreeKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("T")})
	view := stripANSI(m.tree.View())
	if strings.Contains(view, "30-09 14:05") || !strings.Contains(view, "CREATED") {
		t.Fatalf("second T should return to relative times and keep the column:\n%s", view)
	}
}

func TestListKeyT_TogglesTimeFormat(t *testing.T) {
	m := NewModel([]model.Issue{createdColumnFixture()}, "")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 160, Height: 40})
	m = updated.(Model)
	m.focused = focusTree

	m = typeKeys(m, "t", "T")
	view := stripANSI(m.tree.View())
	if !strings.Contains(view, "[LIST]") || !strings.Contains(view, "30-09 14:05") || strings.Contains(view, "2026-09-30") {
		t.Fatalf("T in the flat list should switch to absolute times:\n%s", view)
	}
}

func deferredSortFixture() []model.Issue {
	now := time.Now()
	soon, later := now.Add(24*time.Hour), now.Add(10*24*time.Hour)
	return []model.Issue{
		{ID: "bd-none", Title: "No defer", Status: model.StatusOpen, IssueType: model.TypeTask, CreatedAt: now},
		{ID: "bd-later", Title: "Later defer", Status: model.StatusOpen, IssueType: model.TypeTask, CreatedAt: now, DeferUntil: &later},
		{ID: "bd-soon", Title: "Soon defer", Status: model.StatusOpen, IssueType: model.TypeTask, CreatedAt: now, DeferUntil: &soon},
	}
}

func treeOrder(tree *TreeModel) []string {
	ids := make([]string, 0, tree.NodeCount())
	for _, node := range tree.flatList {
		ids = append(ids, node.Issue.ID)
	}
	return ids
}

// Issues without a defer date go last in both directions: they are not
// waiting for anything, so they never belong between the deferred ones.
func TestTreeSortDeferred_UndatedIssuesGoLast(t *testing.T) {
	cases := []struct {
		dir  SortDirection
		want string
	}{
		{SortAscending, "bd-soon bd-later bd-none"},
		{SortDescending, "bd-later bd-soon bd-none"},
	}
	for _, tc := range cases {
		tree := timeColumnsTree(t, 160, deferredSortFixture()...)
		tree.SetSort(SortFieldDeferred, tc.dir)
		if got := strings.Join(treeOrder(tree), " "); got != tc.want {
			t.Errorf("%s: order %q, want %q", tc.dir.Indicator(), got, tc.want)
		}
	}
}

func TestSortFieldDeferred_LabelDirectionAndConfig(t *testing.T) {
	if SortFieldDeferred.String() != "Deferred" {
		t.Errorf("label %q, want Deferred", SortFieldDeferred.String())
	}
	if SortFieldDeferred.DefaultDirection() != SortAscending {
		t.Error("deferred should default to soonest first")
	}
	if field, _ := sortFromConfig(config.SortConfig{Field: "deferred"}); field != SortFieldDeferred {
		t.Errorf("config field deferred resolved to %s", field)
	}
}
