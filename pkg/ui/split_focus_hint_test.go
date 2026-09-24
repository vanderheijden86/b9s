package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func footerText(m Model) string {
	return stripANSI(m.renderFooter())
}

func pressEnter(m Model) Model {
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	return updated.(Model)
}

func TestFooterTellsEnterOpensDetailWhenTreeFocused(t *testing.T) {
	m := newTreeSplitModel(t)
	m.statusMsg = ""

	if footer := footerText(m); !strings.Contains(footer, "enter:detail") {
		t.Errorf("footer = %q, want enter:detail while the tree has focus", footer)
	}
}

func TestFooterTellsEnterReturnsToTreeWhenDetailFocused(t *testing.T) {
	m := pressEnter(newTreeSplitModel(t))
	m.statusMsg = ""
	if m.focused != focusDetail {
		t.Fatalf("focus after enter = %v, want detail", m.focused)
	}

	if footer := footerText(m); !strings.Contains(footer, "enter:back to tree") {
		t.Errorf("footer = %q, want enter:back to tree while the detail pane has focus", footer)
	}
}

func TestFooterShowsScrollKeysWhenDetailFocused(t *testing.T) {
	m := pressEnter(newTreeSplitModel(t))
	m.statusMsg = ""

	if footer := footerText(m); !strings.Contains(footer, "j/k:scroll") {
		t.Errorf("footer = %q, want j/k:scroll while the detail pane has focus", footer)
	}
}

// Both borders are drawn in bright colors unless the unfocused one is muted,
// and then nothing shows which pane Enter has moved to.
func TestUnfocusedPanelBorderIsMuted(t *testing.T) {
	if got := PanelStyle.GetBorderTopForeground(); got != ColorMuted {
		t.Errorf("unfocused panel border = %v, want ColorMuted", got)
	}
}

// The key hints are only useful if the footer is on screen, so the side by
// side panes must fit between the header and the footer.
func TestSideBySideSplitKeepsFooterOnScreen(t *testing.T) {
	m := newTreeSplitModel(t)
	m.statusMsg = ""

	lines := strings.Split(stripANSI(m.View()), "\n")

	if len(lines) != m.height {
		t.Errorf("view has %d lines, want terminal height %d", len(lines), m.height)
	}
	if last := lines[len(lines)-1]; !strings.Contains(last, "enter:detail") {
		t.Errorf("last line = %q, want the footer hints", last)
	}
}

func TestSideBySideSplitFitsBodyHeight(t *testing.T) {
	m := newTreeSplitModel(t)

	lines := strings.Split(stripANSI(m.renderTreeSplitView()), "\n")

	if len(lines) != m.bodyHeight() {
		t.Errorf("side by side split renders %d lines, want body height %d", len(lines), m.bodyHeight())
	}
	if last := lines[len(lines)-1]; !strings.HasPrefix(last, "╰") {
		t.Errorf("last line = %q, want the panes' bottom borders", last)
	}
}
