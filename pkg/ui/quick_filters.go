package ui

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

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
