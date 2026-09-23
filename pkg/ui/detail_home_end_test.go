package ui_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanderheijden86/beadwork/pkg/model"
	"github.com/vanderheijden86/beadwork/pkg/ui"
)

const (
	detailTopMarker    = "FIRSTLINEMARKER"
	detailBottomMarker = "LASTLINEMARKER"
)

// openLongDetail opens the full-screen tree detail view on an issue whose
// description is far taller than the viewport.
func openLongDetail(t *testing.T) ui.Model {
	t.Helper()
	cleanTreeState(t)
	lines := []string{detailTopMarker}
	for i := 0; i < 200; i++ {
		lines = append(lines, fmt.Sprintf("filler paragraph %d", i), "")
	}
	lines = append(lines, detailBottomMarker)
	issues := []model.Issue{{
		ID: "long-1", Title: "Long Issue", Status: model.StatusOpen, Priority: 2,
		IssueType: model.TypeTask, CreatedAt: time.Now(),
		Description: strings.Join(lines, "\n"),
	}}
	m := ui.NewModel(issues, "")
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	m = enterTreeView(t, newM.(ui.Model))
	m = sendSpecialKey(t, m, tea.KeyEnter)
	if m.FocusState() != "detail" {
		t.Fatalf("expected detail focus, got %q", m.FocusState())
	}
	return m
}

func TestDetailEndScrollsToBottomOfDescription(t *testing.T) {
	m := openLongDetail(t)

	m = sendSpecialKey(t, m, tea.KeyEnd)

	if view := m.View(); !strings.Contains(view, detailBottomMarker) {
		t.Errorf("expected End to show the last line of the description, view:\n%s", view)
	}
}

func TestDetailHomeScrollsToTopOfDescription(t *testing.T) {
	m := openLongDetail(t)
	m = sendSpecialKey(t, m, tea.KeyEnd)

	m = sendSpecialKey(t, m, tea.KeyHome)

	if view := m.View(); !strings.Contains(view, detailTopMarker) {
		t.Errorf("expected Home to show the first line of the description, view:\n%s", view)
	}
}
