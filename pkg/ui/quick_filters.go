package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/vanderheijden86/b9s/pkg/model"
)

// typeFunctionKeys bind one issue type each to a function key, the way k9s
// binds resources to function keys. The header's type legend shows them.
var typeFunctionKeys = []struct {
	key  string
	name string
	typ  model.IssueType
}{
	{"f1", "epic", model.TypeEpic},
	{"f2", "feature", model.TypeFeature},
	{"f3", "task", model.TypeTask},
	{"f4", "bug", model.TypeBug},
}

// typeForFunctionKey returns the type name a function key selects.
func typeForFunctionKey(key string) (string, bool) {
	for _, tk := range typeFunctionKeys {
		if tk.key == key {
			return tk.name, true
		}
	}
	return "", false
}

// showOnlyType replaces every type term in the shared query with type:<name>
// and keeps the other terms. When the query already shows only that type, it
// removes the type term, so the same key shows every type again.
func (m *Model) showOnlyType(name string) {
	prefix := string(QueryFieldType) + ":"
	var types []string
	for _, token := range strings.Fields(strings.ToLower(m.queryState.Text())) {
		if strings.HasPrefix(strings.TrimPrefix(token, "!"), prefix) {
			types = append(types, token)
		}
	}
	typ := model.IssueType(name)
	if len(types) == 1 && types[0] == prefix+name {
		typ = ""
	}
	text := withTypeFilter(m.queryState.Text(), typ)
	m.setQueryText(text)
	if text == "" {
		m.queryState.Clear()
	} else {
		m.queryState.Accept()
	}
	m.afterQuickFilter()
}

func (m *Model) afterQuickFilter() {
	m.statusMsg = "Filter: " + m.queryState.Text()
	if m.queryState.Text() == "" {
		m.statusMsg = "Filter cleared"
	}
	m.statusIsError = false
	if m.isBoardView {
		m.refreshBoardAndGraphForCurrentFilter()
		m.syncBoardToDetail()
	}
}

// QueryText returns the accepted or in-progress text of the shared query.
func (m Model) QueryText() string {
	return m.queryState.Text()
}

// hotkeyFor returns the user hotkey for a key press, but only where a
// keystroke is a command: never while text entry, a popup or an overlay owns it.
func (m Model) hotkeyFor(msg tea.KeyMsg) (Hotkey, bool) {
	if len(m.hotkeys.keys) == 0 || m.showHelp || m.showTutorial ||
		m.focused == focusLabelPicker || m.tree.IsSortPopupOpen() || m.tree.IsColumnPopupOpen() ||
		m.list.FilterState() == list.Filtering {
		return Hotkey{}, false
	}
	return m.hotkeys.Lookup(msg.String())
}

// hotkeyHelpRows lists the user hotkeys for the help overlay's Hotkeys panel.
// It is empty without a hotkeys.yaml, and the overlay skips an empty panel.
func (m Model) hotkeyHelpRows() []struct{ key, desc string } {
	rows := make([]struct{ key, desc string }, 0, len(m.hotkeys.keys))
	for _, h := range m.hotkeys.keys {
		desc := ":" + h.CommandText
		if h.Description != "" {
			desc = h.Description + " (" + desc + ")"
		}
		rows = append(rows, struct{ key, desc string }{h.ShortCut, desc})
	}
	return rows
}
