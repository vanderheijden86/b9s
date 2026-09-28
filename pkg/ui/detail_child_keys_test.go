package ui_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// With the detail focused, 1-9 open the issue's children in the order the
// CHILDREN section numbers them, and Backspace returns to the issue before.

func TestDetailDigitOpensChild(t *testing.T) {
	m := openDetailOn(t, 0) // epic-1

	m = sendKey(t, m, "2")

	if m.TreeSelectedID() != "task-2" {
		t.Fatalf("expected task-2 after '2' in the epic's detail, got %q", m.TreeSelectedID())
	}
	if m.FocusState() != "detail" {
		t.Errorf("expected focus to stay 'detail', got %q", m.FocusState())
	}
	if view := m.View(); !strings.Contains(view, "Task 2") {
		t.Errorf("expected the detail to render Task 2, view:\n%s", view)
	}
}

func TestDetailNumbersChildren(t *testing.T) {
	m := openDetailOn(t, 0)

	for n, title := range []string{"Task 1", "Task 2", "Task 3"} {
		found := false
		for _, line := range strings.Split(m.View(), "\n") {
			if !strings.Contains(line, title) || strings.Contains(line, "Epic One") {
				continue
			}
			// The short IDs are 1-3 as well, so the number must come before the ID.
			num := string(rune('1' + n))
			if f := strings.Fields(line); len(f) > 1 && f[0] == num && f[1] == num {
				found = true
			}
		}
		if !found {
			t.Errorf("expected the child %q to carry the number %d, view:\n%s", title, n+1, m.View())
		}
	}
}

func TestDetailBackspaceReturnsToParent(t *testing.T) {
	m := openDetailOn(t, 0)
	m = sendKey(t, m, "3")

	m = sendSpecialKey(t, m, tea.KeyBackspace)

	if m.TreeSelectedID() != "epic-1" {
		t.Errorf("expected Backspace to return to epic-1, got %q", m.TreeSelectedID())
	}
	if m.FocusState() != "detail" {
		t.Errorf("expected focus to stay 'detail', got %q", m.FocusState())
	}
}

func TestDetailDigitReachesChildOfFoldedIssue(t *testing.T) {
	cleanTreeState(t)
	m := openDetailOn(t, 0)
	m = sendSpecialKey(t, m, tea.KeyEnter) // back to the tree
	m = sendKey(t, m, "h")                 // fold epic-1
	m = sendSpecialKey(t, m, tea.KeyEnter)

	m = sendKey(t, m, "1")

	if m.TreeSelectedID() != "task-1" {
		t.Errorf("expected task-1 under the folded epic, got %q", m.TreeSelectedID())
	}
}

func TestDetailDigitWithoutSuchChildKeepsIssue(t *testing.T) {
	m := openDetailOn(t, 1) // task-1 has no children

	m = sendKey(t, m, "1")

	if m.TreeSelectedID() != "task-1" {
		t.Errorf("expected task-1 to stay selected, got %q", m.TreeSelectedID())
	}
	if m.FocusState() != "detail" {
		t.Errorf("expected focus to stay 'detail', got %q", m.FocusState())
	}
}
