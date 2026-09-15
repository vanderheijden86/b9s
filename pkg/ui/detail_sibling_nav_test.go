package ui_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanderheijden86/beadwork/pkg/ui"
)

// openDetailOn opens the full-screen tree detail view for the issue reached
// by pressing j `downs` times from the top of the tree.
func openDetailOn(t *testing.T, downs int) ui.Model {
	t.Helper()
	cleanTreeState(t)
	m := ui.NewModel(createNavTestIssues(), "")
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	m = enterTreeView(t, newM.(ui.Model))
	for i := 0; i < downs; i++ {
		m = sendKey(t, m, "j")
	}
	m = sendSpecialKey(t, m, tea.KeyEnter)
	if m.FocusState() != "detail" {
		t.Fatalf("expected detail focus, got %q", m.FocusState())
	}
	return m
}

func TestDetailNextSiblingN(t *testing.T) {
	m := openDetailOn(t, 1) // task-1

	m = sendKey(t, m, "n")

	if m.TreeSelectedID() != "task-2" {
		t.Errorf("expected task-2 after 'n' in detail, got %q", m.TreeSelectedID())
	}
}

func TestDetailPrevSiblingP(t *testing.T) {
	m := openDetailOn(t, 2) // task-2

	m = sendKey(t, m, "p")

	if m.TreeSelectedID() != "task-1" {
		t.Errorf("expected task-1 after 'p' in detail, got %q", m.TreeSelectedID())
	}
}

func TestDetailSiblingNavStaysInDetail(t *testing.T) {
	m := openDetailOn(t, 1)

	m = sendKey(t, m, "n")

	if m.FocusState() != "detail" {
		t.Errorf("expected focus to stay 'detail', got %q", m.FocusState())
	}
}

func TestDetailSiblingNavRendersNewIssue(t *testing.T) {
	m := openDetailOn(t, 1)

	m = sendKey(t, m, "n")

	if view := m.View(); !strings.Contains(view, "Task 2") {
		t.Errorf("expected detail to render Task 2 after 'n', view:\n%s", view)
	}
}

func TestDetailNextSiblingAtLastSiblingKeepsSelection(t *testing.T) {
	m := openDetailOn(t, 3) // task-3, last child of epic-1

	m = sendKey(t, m, "n")

	if m.TreeSelectedID() != "task-3" {
		t.Errorf("expected task-3 to remain selected, got %q", m.TreeSelectedID())
	}
}
