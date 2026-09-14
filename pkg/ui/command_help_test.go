package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestHelpOverlayListsCommandPrompt(t *testing.T) {
	updated, _ := NewModel(nil, "").Update(tea.WindowSizeMsg{Width: 180, Height: 60})
	m := updated.(Model)

	help := stripANSI(m.renderHelpOverlay())

	if !strings.Contains(help, "Command prompt") {
		t.Errorf("help overlay does not list the ':' command prompt:\n%s", help)
	}
}
