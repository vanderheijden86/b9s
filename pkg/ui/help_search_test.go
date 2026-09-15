package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbletea"
	"github.com/vanderheijden86/beadwork/pkg/model"
)

func openHelpOverlay(t *testing.T) Model {
	t.Helper()
	m := NewModel([]model.Issue{{ID: "1", Title: "One", Status: model.StatusOpen}}, "")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 120})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	m = updated.(Model)
	if !m.showHelp || m.focused != focusHelp {
		t.Fatalf("expected help overlay shown")
	}
	return m
}

func sendKeys(m Model, msgs ...tea.KeyMsg) Model {
	for _, msg := range msgs {
		updated, _ := m.Update(msg)
		m = updated.(Model)
	}
	return m
}

func typeText(m Model, text string) Model {
	for _, r := range text {
		m = sendKeys(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return m
}

func TestHelpSearch_CtrlSOpensSearchInput(t *testing.T) {
	m := openHelpOverlay(t)
	m = sendKeys(m, tea.KeyMsg{Type: tea.KeyCtrlS})

	if !m.showHelp || m.focused != focusHelp {
		t.Fatalf("ctrl+s must keep the help overlay open")
	}
	if !m.helpSearching {
		t.Fatalf("ctrl+s must start help search")
	}
}

func TestHelpSearch_FiltersShortcutsByDescription(t *testing.T) {
	m := openHelpOverlay(t)
	m = sendKeys(m, tea.KeyMsg{Type: tea.KeyCtrlS})
	m = typeText(m, "graph")

	view := m.View()
	if !strings.Contains(view, "Dependency graph") {
		t.Fatalf("expected matching shortcut in view, got:\n%s", view)
	}
	if strings.Contains(view, "Kanban board") {
		t.Fatalf("expected non-matching shortcut hidden, got:\n%s", view)
	}
}

func TestHelpSearch_FiltersShortcutsByKey(t *testing.T) {
	m := openHelpOverlay(t)
	m = sendKeys(m, tea.KeyMsg{Type: tea.KeyCtrlS})
	m = typeText(m, "ctrl+r")

	view := m.View()
	if !strings.Contains(view, "Force refresh") {
		t.Fatalf("expected shortcut matched by key in view, got:\n%s", view)
	}
	if strings.Contains(view, "Dependency graph") {
		t.Fatalf("expected non-matching shortcut hidden, got:\n%s", view)
	}
}

func TestHelpSearch_TypingHelpToggleKeyDoesNotCloseHelp(t *testing.T) {
	m := openHelpOverlay(t)
	m = sendKeys(m, tea.KeyMsg{Type: tea.KeyCtrlS})
	m = typeText(m, "?`:x")

	if !m.showHelp || m.focused != focusHelp {
		t.Fatalf("keys typed into the search must not close help")
	}
	if got := m.helpSearchInput.Value(); got != "?`:x" {
		t.Fatalf("expected query %q, got %q", "?`:x", got)
	}
}

func TestHelpSearch_NoMatchesSaysSo(t *testing.T) {
	m := openHelpOverlay(t)
	m = sendKeys(m, tea.KeyMsg{Type: tea.KeyCtrlS})
	m = typeText(m, "zzzqqq")

	if view := m.View(); !strings.Contains(view, "No shortcuts match") {
		t.Fatalf("expected no-match notice, got:\n%s", view)
	}
}

func TestHelpSearch_EnterKeepsFilterAndStopsTyping(t *testing.T) {
	m := openHelpOverlay(t)
	m = sendKeys(m, tea.KeyMsg{Type: tea.KeyCtrlS})
	m = typeText(m, "graph")
	m = sendKeys(m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.helpSearching {
		t.Fatalf("enter must stop editing the search")
	}
	if !m.showHelp {
		t.Fatalf("enter must keep help open")
	}
	m = sendKeys(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if !m.showHelp {
		t.Fatalf("j must scroll, not dismiss, a filtered help overlay")
	}
	if view := m.View(); strings.Contains(view, "Kanban board") {
		t.Fatalf("filter must survive enter, got:\n%s", view)
	}
}

func TestHelpSearch_EscClearsFilterBeforeClosingHelp(t *testing.T) {
	m := openHelpOverlay(t)
	m = sendKeys(m, tea.KeyMsg{Type: tea.KeyCtrlS})
	m = typeText(m, "graph")

	m = sendKeys(m, tea.KeyMsg{Type: tea.KeyEsc})
	if !m.showHelp || m.helpSearching || m.helpSearchInput.Value() != "" {
		t.Fatalf("first esc must clear the search and keep help open")
	}
	if view := m.View(); !strings.Contains(view, "Kanban board") {
		t.Fatalf("clearing the search must show every shortcut again")
	}

	m = sendKeys(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.showHelp {
		t.Fatalf("second esc must close help")
	}
}

func TestHelpSearch_ReopeningHelpStartsUnfiltered(t *testing.T) {
	m := openHelpOverlay(t)
	m = sendKeys(m, tea.KeyMsg{Type: tea.KeyCtrlS})
	m = typeText(m, "graph")
	m = sendKeys(m, tea.KeyMsg{Type: tea.KeyEnter}, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	if m.showHelp {
		t.Fatalf("? must close a help overlay that is not being searched")
	}

	m = sendKeys(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	if m.helpSearchInput.Value() != "" {
		t.Fatalf("reopened help must start without a filter")
	}
}
