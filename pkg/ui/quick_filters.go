package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vanderheijden86/b9s/pkg/model"
)

// quickFilterTypes are the issue types the Y quick filter offers, numbered 1-5
// in the order of the header's type legend.
var quickFilterTypes = []struct {
	name string
	typ  model.IssueType
}{
	{"bug", model.TypeBug},
	{"feature", model.TypeFeature},
	{"task", model.TypeTask},
	{"epic", model.TypeEpic},
	{"chore", model.TypeChore},
}

// activeTypeFilters returns the type names the shared query currently selects.
func (m Model) activeTypeFilters() map[string]bool {
	active := map[string]bool{}
	prefix := string(QueryFieldType) + ":"
	for _, token := range strings.Fields(strings.ToLower(m.queryState.Text())) {
		if strings.HasPrefix(token, prefix) {
			active[strings.TrimPrefix(token, prefix)] = true
		}
	}
	return active
}

// toggleTypeFilter adds or removes type:<name> in the shared query, so the
// chip composes with every other term and shows in the query bar of every view.
func (m *Model) toggleTypeFilter(name string) {
	text, _ := toggleQueryTerm(m.queryState.Text(), QueryFieldType, name)
	m.setQueryText(text)
	m.queryState.Accept()
	m.afterQuickFilter()
}

// clearTypeFilter removes every type term and leaves the other terms.
func (m *Model) clearTypeFilter() {
	prefix := string(QueryFieldType) + ":"
	var kept []string
	for _, token := range strings.Fields(m.queryState.Text()) {
		if !strings.HasPrefix(strings.ToLower(strings.TrimPrefix(token, "!")), prefix) {
			kept = append(kept, token)
		}
	}
	m.setQueryText(strings.Join(kept, " "))
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

// renderTypeBar renders the Y quick filter panel: the five types with counts
// and their number keys, beside the shortcut column.
func (m Model) renderTypeBar() string {
	t := m.theme
	w := m.width
	if w <= 0 {
		w = 80
	}
	numStyle := t.Renderer.NewStyle().Foreground(lipgloss.Color("#F3F3F3")).Bold(true)
	activeStyle := t.Renderer.NewStyle().Foreground(t.Primary).Bold(true)
	normalStyle := t.Renderer.NewStyle().Foreground(t.Base.GetForeground())
	countStyle := t.Renderer.NewStyle().Foreground(t.Secondary)

	counts := map[model.IssueType]int{}
	for _, issue := range m.issues {
		counts[issue.IssueType]++
	}
	active := m.activeTypeFilters()

	lines := make([]string, panelRows)
	lines[0] = countStyle.Render("      Type                Count")
	for i, qt := range quickFilterTypes {
		icon, color := t.GetTypeIcon(string(qt.typ))
		iconStyled := t.Renderer.NewStyle().Foreground(color).Render(icon)
		if active[qt.name] {
			lines[i+1] = activeStyle.Render(fmt.Sprintf(" <%d> ", i+1)) + iconStyled + activeStyle.Render(fmt.Sprintf(" %-14s  %4d", qt.name, counts[qt.typ]))
			continue
		}
		lines[i+1] = numStyle.Render(fmt.Sprintf(" <%d> ", i+1)) + iconStyled + normalStyle.Render(fmt.Sprintf(" %-14s", qt.name)) + countStyle.Render(fmt.Sprintf("  %4d", counts[qt.typ]))
	}
	if len(active) == 0 {
		lines[0] += "  " + activeStyle.Render("<0> all")
	} else {
		lines[0] += "  " + numStyle.Render("<0> all")
	}

	m.projectPicker.SetSize(w, m.height)
	shortcutLines := m.projectPicker.RenderShortcutsColumn()
	shortcutsWidth := lipgloss.Width(shortcutLines[0])
	if shortcutsWidth < 16 {
		shortcutsWidth = 16
	}
	const gap = 2
	showShortcuts := gap+shortcutsWidth <= w/2
	tableWidth := w
	if showShortcuts {
		tableWidth -= shortcutsWidth + gap
	}
	clipStyle := t.Renderer.NewStyle().MaxWidth(w)
	rows := make([]string, 0, panelRows+1)
	for i := 0; i < panelRows; i++ {
		row := padRight(safeIndex(lines[:], i), tableWidth)
		if showShortcuts {
			row += strings.Repeat(" ", gap) + padRight(safeIndex(shortcutLines[:], i), shortcutsWidth)
		}
		rows = append(rows, clipStyle.Render(row))
	}
	if titleBar := m.renderUnifiedTitleBar(w); titleBar != "" {
		rows = append(rows, titleBar)
	}
	return strings.Join(rows, "\n")
}

// QueryText returns the accepted or in-progress text of the shared query.
func (m Model) QueryText() string {
	return m.queryState.Text()
}

// KeybindingErrors lists the configured bindings that failed validation.
func (m Model) KeybindingErrors() []error {
	return m.keybindingErrors
}

// customBindingFor returns the user binding for a key press, but only where a
// keystroke is a command: never while text entry, a popup or an overlay owns it.
func (m Model) customBindingFor(msg tea.KeyMsg) (CustomBinding, bool) {
	if len(m.keymap.bindings) == 0 || m.showHelp || m.showTutorial ||
		m.focused == focusLabelPicker || m.tree.IsSortPopupOpen() || m.tree.IsColumnPopupOpen() ||
		m.list.FilterState() == list.Filtering {
		return CustomBinding{}, false
	}
	return m.keymap.Lookup(msg.String())
}

// runQueryBinding applies a query binding to the shared query. Pressing it
// again while its query is showing clears the query, like the o, i and C keys.
func (m *Model) runQueryBinding(b CustomBinding) {
	if strings.EqualFold(m.queryState.Text(), b.Query) {
		m.queryState.Clear()
		m.setQueryText("")
	} else {
		m.setQueryText(b.Query)
		m.queryState.Accept()
	}
	m.afterQuickFilter()
}

// customHelpRows lists the user bindings for the help overlay's Custom panel.
// It is empty when the config defines none, and the overlay skips an empty panel.
func (m Model) customHelpRows() []struct{ key, desc string } {
	rows := make([]struct{ key, desc string }, 0, len(m.keymap.bindings))
	for _, b := range m.keymap.bindings {
		desc := b.Label()
		if b.Query != "" && b.Description != "" {
			desc += " (" + b.Query + ")"
		}
		rows = append(rows, struct{ key, desc string }{b.Key, desc})
	}
	return rows
}
