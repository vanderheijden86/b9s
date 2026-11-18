package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vanderheijden86/b9s/pkg/config"
)

func TestPreviewCommandResolves(t *testing.T) {
	got, err := ResolveCommand("preview")
	if err != nil || got.Kind != CommandPreview {
		t.Fatalf("ResolveCommand(preview) = %+v, %v; want CommandPreview", got, err)
	}
	if _, err := ResolveCommand("preview shop-1"); err == nil {
		t.Fatal("preview must reject arguments")
	}
}

// :pr completes to project, the older command, even though preview sorts first.
func TestColonPRStillCompletesToProject(t *testing.T) {
	if got := commandSuggestion("pr"); got != "project" {
		t.Fatalf("commandSuggestion(pr) = %q, want project", got)
	}
	if got := commandSuggestion("pre"); got != "preview" {
		t.Fatalf("commandSuggestion(pre) = %q, want preview", got)
	}
}

func TestColonPreviewOpensQuickViewOnSelectedRow(t *testing.T) {
	m := quickViewModel(t)
	m = pressKeys(m, runeKey(":"))
	for _, r := range "preview" {
		m = pressKeys(m, runeKey(string(r)))
	}
	m = pressKeys(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.quickView == nil || !strings.Contains(stripANSI(m.View()), "Quick view") {
		t.Fatalf(":preview must open the quick view, status %q", m.statusMsg)
	}
}

func TestHotkeyCanBindPreviewToAnyKey(t *testing.T) {
	m := keybindingTestModel(t, config.Hotkeys{{Name: "peek", ShortCut: "Ctrl-T", Command: "preview"}})
	m.focused = focusList
	m = pressNamedKey(m, "ctrl+t")
	if m.quickView == nil {
		t.Fatalf("a hotkey bound to preview must open the quick view, status %q", m.statusMsg)
	}
}

func TestCtrlOAcceptsAUserHotkeyWithoutOverride(t *testing.T) {
	if _, errs := ResolveHotkeys(config.Hotkeys{{Name: "layout", ShortCut: "Ctrl-O", Command: "layout"}}); len(errs) != 0 {
		t.Fatalf("Ctrl-O has no built-in action, so it needs no override: %v", errs)
	}
}

func TestCtrlOOpensNoPreview(t *testing.T) {
	m := quickViewModel(t)
	m = pressKeys(m, tea.KeyMsg{Type: tea.KeyCtrlO})
	if m.quickView != nil {
		t.Fatal("the preview is on Ctrl-P, so Ctrl-O must not open it")
	}
}

func TestPreviewCommandWithoutIssueReportsStatus(t *testing.T) {
	m := keybindingTestModel(t, nil)
	m.issues, m.issueMap = nil, nil
	m.applyFilter()
	next, _ := m.executeCommand(Command{Kind: CommandPreview})
	if next.quickView != nil || !next.statusIsError || next.statusMsg == "" {
		t.Fatalf("preview with nothing selected must report it: quickView=%v status=%q", next.quickView != nil, next.statusMsg)
	}
}

func TestCtrlPDoesNotMoveLabelPicker(t *testing.T) {
	m := quickViewModel(t)
	m.labelPicker.SetLabels([]string{"bug", "chore"}, map[string]int{"bug": 1, "chore": 1})
	m.showLabelPicker = true
	m.focused = focusLabelPicker
	m = pressKeys(m, tea.KeyMsg{Type: tea.KeyDown})
	before := m.labelPicker.SelectedLabel()
	m = pressKeys(m, tea.KeyMsg{Type: tea.KeyCtrlP})
	if got := m.labelPicker.SelectedLabel(); got != before {
		t.Fatalf("Ctrl-P moved the label picker from %q to %q; Up and k move it", before, got)
	}
}
