package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanderheijden86/b9s/pkg/model"
)

func pressKeys(m Model, keys ...tea.KeyMsg) Model {
	for _, k := range keys {
		updated, _ := m.Update(k)
		m = updated.(Model)
	}
	return m
}


// The detail footer advertises e:edit, so e must open the edit modal for the
// issue the detail shows, also when the detail was reached through a search.
func TestDetailViewEKeyOpensEditModal(t *testing.T) {
	enter := tea.KeyMsg{Type: tea.KeyEnter}
	cases := map[string][]tea.KeyMsg{
		"plain":       {enter},
		"from search": {runeKey("/"), runeKey("s"), runeKey("e"), enter, enter},
	}
	for name, path := range cases {
		issues := []model.Issue{
			{ID: "a-1", Title: "First", Status: model.StatusOpen},
			{ID: "a-2", Title: "Second", Status: model.StatusOpen},
		}
		m := NewModel(issues, "")
		updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
		m = pressKeys(updated.(Model), path...)
		if m.focused != focusDetail {
			t.Fatalf("%s: expected the key path to open the detail view, focus %v", name, m.focused)
		}
		want := m.getSelectedIssue()

		m = pressKeys(m, runeKey("e"))

		if !m.showEditModal {
			t.Fatalf("%s: expected e to open the edit modal from the detail view", name)
		}
		if want == nil || m.editModal.issueID != want.ID {
			t.Fatalf("%s: edit modal opened for %q, want the detail's issue %v", name, m.editModal.issueID, want)
		}
	}
}
