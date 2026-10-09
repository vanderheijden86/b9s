package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vanderheijden86/b9s/pkg/model"
)

func newTreeShortcutModel(t *testing.T) Model {
	t.Helper()
	issues := []model.Issue{
		{ID: "bd-open", Title: "Open task", IssueType: model.TypeTask, Status: model.StatusOpen},
		{ID: "bd-done", Title: "Finished task", IssueType: model.TypeTask, Status: model.StatusClosed},
	}
	m := NewModel(issues, "")
	m.tree.Build(issues)
	m.focused = focusTree
	return m
}

func pressTreeKey(m Model, r rune) Model {
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	return updated.(Model)
}

func TestTreeLowercaseCCopiesSelectedIDAndTitle(t *testing.T) {
	m := newTreeShortcutModel(t)
	selected := m.tree.SelectedIssue()
	if selected == nil {
		t.Fatal("test setup should select an issue")
	}
	var copied string
	m.clipboardWrite = func(text string) error {
		copied = text
		return nil
	}

	m = pressTreeKey(m, 'c')

	want := selected.ID + " " + selected.Title
	if copied != want {
		t.Fatalf("c in tree view copied %q, want %q", copied, want)
	}
	if m.currentFilter == "closed" {
		t.Fatal("c in tree view must not apply the closed filter")
	}
	if !strings.Contains(m.statusMsg, selected.ID) {
		t.Fatalf("status should confirm the copied issue, got %q", m.statusMsg)
	}
}

func TestTreeCapitalCFiltersClosedIssues(t *testing.T) {
	m := newTreeShortcutModel(t)

	m = pressTreeKey(m, 'C')

	if m.currentFilter != "closed" {
		t.Fatalf("C in tree view should filter closed issues, got filter %q", m.currentFilter)
	}
	if m.tree.IsColumnPopupOpen() {
		t.Fatal("C in tree view must not open the column popup")
	}
}

func TestTreePipeOpensAndClosesColumnPopup(t *testing.T) {
	m := newTreeShortcutModel(t)

	m = pressTreeKey(m, '|')
	if !m.tree.IsColumnPopupOpen() {
		t.Fatal("| in tree view should open the column popup")
	}

	m = pressTreeKey(m, '|')
	if m.tree.IsColumnPopupOpen() {
		t.Fatal("| should close the column popup it opened")
	}
}

// A popup sits on top of the filtered tree, so Escape belongs to the popup
// and the accepted branch filter underneath must survive it.
func TestEscapeClosesTreePopupsWithoutClearingBranchFilter(t *testing.T) {
	cases := []struct {
		name   string
		open   func(*Model)
		isOpen func(Model) bool
	}{
		{"columns", func(m *Model) { m.tree.OpenColumnPopup() }, func(m Model) bool { return m.tree.IsColumnPopupOpen() }},
		{"sort", func(m *Model) { m.tree.OpenSortPopup() }, func(m Model) bool { return m.tree.IsSortPopupOpen() }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newTreeShortcutModel(t)
			m = pressTreeKey(m, 'f')
			filter := m.queryState.Text()
			if filter == "" {
				t.Fatal("f should apply a branch filter")
			}
			tc.open(&m)

			updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
			m = updated.(Model)

			if tc.isOpen(m) {
				t.Fatalf("Escape should close the %s popup", tc.name)
			}
			if got := m.queryState.Text(); got != filter {
				t.Fatalf("Escape in the %s popup cleared the branch filter: got %q, want %q", tc.name, got, filter)
			}
		})
	}
}

// The header legend is read as a promise about the tree, the default view.
// Each filter it advertises must be the filter its key applies there (#20).
func TestHeaderLegendFilterKeysMatchTreeBindings(t *testing.T) {
	wantFilter := map[string]string{
		"Open":   "open",
		"Closed": "closed",
		"Ready":  "ready",
		"All":    "all",
	}
	seen := 0
	for _, row := range pickerShortcuts() {
		for _, entry := range row {
			filter, ok := wantFilter[entry.desc]
			if !ok {
				continue
			}
			seen++
			keys := []rune(entry.key)
			if len(keys) != 1 {
				t.Fatalf("legend %q names key %q, want a single key", entry.desc, entry.key)
			}
			m := newTreeShortcutModel(t)
			m.clipboardWrite = func(string) error { return nil }
			if filter == "all" {
				m.currentFilter = "open"
			}

			m = pressTreeKey(m, keys[0])

			if m.currentFilter != filter {
				t.Errorf("legend says %s %s, but %s in the tree set filter %q, want %q",
					entry.key, entry.desc, entry.key, m.currentFilter, filter)
			}
		}
	}
	if seen != len(wantFilter) {
		t.Fatalf("legend lists %d of the %d filters", seen, len(wantFilter))
	}
}
