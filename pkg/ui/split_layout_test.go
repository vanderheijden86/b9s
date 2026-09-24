package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// newTreeSplitModel returns a wide model in tree view with the detail pane shown.
func newTreeSplitModel(t *testing.T) Model {
	t.Helper()
	m := newSearchFilterModel(t, "")
	m = typeKeys(m, "d")
	if m.treeDetailHidden {
		t.Fatal("detail pane hidden after d, want it shown")
	}
	return m
}

func runLayoutCommand(t *testing.T, m Model) Model {
	t.Helper()
	m = typeKeys(m, ":", "l", "a", "y", "o", "u", "t")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	return updated.(Model)
}

// panelTopRows returns, for each rendered line, how many panels start on it.
// Side by side, both panels start on one line; stacked, each starts on its own.
func panelTopRows(view string) []int {
	var rows []int
	for _, line := range strings.Split(stripANSI(view), "\n") {
		if n := strings.Count(line, "╭"); n > 0 {
			rows = append(rows, n)
		}
	}
	return rows
}

func TestSplitViewStartsSideBySide(t *testing.T) {
	m := newTreeSplitModel(t)

	if rows := panelTopRows(m.View()); len(rows) != 1 || rows[0] != 2 {
		t.Errorf("panel tops per line = %v, want [2] (both panels on one line)", rows)
	}
}

func TestLayoutCommandStacksDetailBelowTree(t *testing.T) {
	m := newTreeSplitModel(t)

	m = runLayoutCommand(t, m)

	if rows := panelTopRows(m.View()); len(rows) != 2 || rows[0] != 1 || rows[1] != 1 {
		t.Errorf("panel tops per line = %v, want [1 1] (detail below tree)", rows)
	}
	if !strings.Contains(m.statusMsg, "stacked") {
		t.Errorf("status = %q, want it to name the stacked layout", m.statusMsg)
	}
}

func TestLayoutCommandTwiceRestoresSideBySide(t *testing.T) {
	m := newTreeSplitModel(t)
	m = runLayoutCommand(t, m)

	m = runLayoutCommand(t, m)

	if rows := panelTopRows(m.View()); len(rows) != 1 || rows[0] != 2 {
		t.Errorf("panel tops per line = %v, want [2] (side by side)", rows)
	}
}

func TestStackedLayoutGivesDetailFullWidth(t *testing.T) {
	m := newTreeSplitModel(t)

	m = runLayoutCommand(t, m)

	if want := m.width - 4; m.viewport.Width != want {
		t.Errorf("detail width = %d, want %d (terminal width less borders)", m.viewport.Width, want)
	}
}

func TestStackedLayoutFitsBodyHeight(t *testing.T) {
	m := newTreeSplitModel(t)

	m = runLayoutCommand(t, m)

	lines := strings.Split(stripANSI(m.renderTreeSplitView()), "\n")
	if len(lines) != m.bodyHeight() {
		t.Errorf("stacked split renders %d lines, want body height %d", len(lines), m.bodyHeight())
	}
	if last := lines[len(lines)-1]; !strings.HasPrefix(last, "╰") {
		t.Errorf("last line = %q, want the detail pane's bottom border", last)
	}
}

func TestStackedLayoutGrowKeyGivesTreeMoreRows(t *testing.T) {
	m := newTreeSplitModel(t)
	m = runLayoutCommand(t, m)
	_, before := m.treeLayoutSize()

	m = typeKeys(m, ">")

	if _, after := m.treeLayoutSize(); after <= before {
		t.Errorf("tree height after > = %d, want more than %d", after, before)
	}
}

// A ratio that leaves the side-by-side detail pane too narrow to read still
// leaves the stacked one the full terminal width.
func TestStackedLayoutKeepsDetailWhenSideBySideWouldBeTooNarrow(t *testing.T) {
	m := newTreeSplitModel(t)
	m = runLayoutCommand(t, m)
	m.splitPaneRatio = 0.8

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 104, Height: 40})
	m = updated.(Model)

	if m.treeDetailHidden {
		t.Error("detail pane hidden in stacked layout, want it kept at full width")
	}
}
