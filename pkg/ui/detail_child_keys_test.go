package ui_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/vanderheijden86/b9s/pkg/model"
	"github.com/vanderheijden86/b9s/pkg/ui"
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

func openManyChildrenDetail(t *testing.T, reverseTitles bool) ui.Model {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	now := time.Now()
	issues := []model.Issue{{
		ID: "many-epic", Title: "Many children", Status: model.StatusOpen,
		IssueType: model.TypeEpic, Priority: 1, CreatedAt: now,
	}}
	for n := 1; n <= 24; n++ {
		id := fmt.Sprintf("many-child-%02d", n)
		titleNumber := n
		if reverseTitles {
			titleNumber = 25 - n
		}
		issues = append(issues, model.Issue{
			ID: id, Title: fmt.Sprintf("Child %02d", titleNumber),
			Description: "Body of " + id, Status: model.StatusOpen,
			IssueType: model.TypeTask, Priority: 2, CreatedAt: now.Add(-time.Duration(n) * time.Second),
			Dependencies: []*model.Dependency{{IssueID: id, DependsOnID: "many-epic", Type: model.DepParentChild}},
		})
	}
	issues = append(issues, model.Issue{
		ID: "many-grandchild", Title: "Nested child", Status: model.StatusOpen,
		IssueType: model.TypeTask, Priority: 2, CreatedAt: now,
		Dependencies: []*model.Dependency{{IssueID: "many-grandchild", DependsOnID: "many-child-09", Type: model.DepParentChild}},
	})
	m := ui.NewModel(issues, "")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	m = enterTreeView(t, updated.(ui.Model))
	m = sendSpecialKey(t, m, tea.KeyEnter)
	if m.TreeSelectedID() != "many-epic" || m.FocusState() != "detail" {
		t.Fatalf("expected parent detail, got %q with focus %q", m.TreeSelectedID(), m.FocusState())
	}
	return m
}

func TestDetailDigitsKeepNumberedChildOrderWithManyChildren(t *testing.T) {
	for n := 1; n <= 9; n++ {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			m := openManyChildrenDetail(t, true)
			m = sendKey(t, m, fmt.Sprint(n))
			want := fmt.Sprintf("many-child-%02d", n)
			if got := m.TreeSelectedID(); got != want {
				t.Fatalf("digit %d selected %q, want numbered child %q", n, got, want)
			}
		})
	}
}

func TestDetailSiblingKeysReachEveryChildBeyondNine(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		t.Run(fmt.Sprintf("reverse_titles=%t", reverse), func(t *testing.T) {
			m := openManyChildrenDetail(t, reverse)
			m = sendKey(t, m, "9")
			visited := map[string]bool{}
			for _, key := range []string{"n", "p"} {
				for step := 0; step <= 24; step++ {
					id := m.TreeSelectedID()
					if !strings.HasPrefix(id, "many-child-") {
						t.Fatalf("%s left the parent's direct children: %q", key, id)
					}
					if m.FocusState() != "detail" || !strings.Contains(ansi.Strip(m.View()), "Body of "+id) {
						t.Fatalf("%s must keep the selected child's detail visible: %q, focus %q:\n%s", key, id, m.FocusState(), m.View())
					}
					visited[id] = true
					m = sendKey(t, m, key)
				}
			}
			for n := 1; n <= 24; n++ {
				id := fmt.Sprintf("many-child-%02d", n)
				if !visited[id] {
					t.Errorf("digit then n/p did not reach child %q", id)
				}
			}
			m = sendSpecialKey(t, m, tea.KeyBackspace)
			if got := m.TreeSelectedID(); got != "many-epic" {
				t.Fatalf("Backspace after sibling navigation selected %q, want parent", got)
			}
		})
	}
}
