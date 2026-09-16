package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanderheijden86/beadwork/pkg/model"
)

// spaceKey is the message bubbletea delivers for the space bar: a KeySpace
// event carrying the rune, not a KeyRunes event.
var spaceKey = tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}

func typeInto(m Model, text string) Model {
	for _, r := range text {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
		if r == ' ' {
			msg = spaceKey
		}
		next, _ := m.Update(msg)
		m = next.(Model)
	}
	return m
}

func TestQueryBarAcceptsTypedSpace(t *testing.T) {
	m := NewModel([]model.Issue{{ID: "rws-sync-inh", Title: "Epic"}}, "")

	m = typeInto(m, "/rws sync")

	if got := m.queryState.Text(); got != "rws sync" {
		t.Errorf("query text = %q, want %q", got, "rws sync")
	}
}

func TestCommandPromptAcceptsTypedSpace(t *testing.T) {
	m := NewModel([]model.Issue{{ID: "rws-sync-inh", Title: "Epic"}}, "")

	m = typeInto(m, ":type bug")

	if got := m.commandPrompt.Text(); got != "type bug" {
		t.Errorf("prompt text = %q, want %q", got, "type bug")
	}
}

func TestIssueQueryPlainTermsMatchAcrossFields(t *testing.T) {
	issue := model.Issue{ID: "rws-sync-inh", Title: "Close the remaining RWS interface field mapping"}

	if !ParseIssueQuery("inh mapping").Matches(issue) {
		t.Error("terms from ID and title did not match together")
	}
}

func TestIssueQueryPlainTermsMatchFuzzilyEach(t *testing.T) {
	issue := model.Issue{ID: "rws-sync-inh", Title: "Close the remaining RWS interface field mapping"}

	if !ParseIssueQuery("rws mppng").Matches(issue) {
		t.Error("abbreviated term did not fuzzy-match alongside an exact term")
	}
}

func TestIssueQueryPlainTermsRequireEveryTerm(t *testing.T) {
	issue := model.Issue{ID: "rws-sync-inh", Title: "Close the remaining RWS interface field mapping"}

	if ParseIssueQuery("inh zzzzzz").Matches(issue) {
		t.Error("query matched although one term matches nothing")
	}
}
