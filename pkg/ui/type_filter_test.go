package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanderheijden86/b9s/pkg/config"
)

func pressFKey(m Model, n int) Model {
	keys := map[int]tea.KeyType{1: tea.KeyF1, 2: tea.KeyF2, 3: tea.KeyF3, 4: tea.KeyF4}
	updated, _ := m.Update(tea.KeyMsg{Type: keys[n]})
	return updated.(Model)
}

// F1-F4 switch the shown type the way k9s function keys switch resources:
// F1 epic, F2 feature, F3 task, F4 bug. A key shows only its type; the same
// key again shows every type.
func TestFunctionKeysSelectOneType(t *testing.T) {
	m := keybindingTestModel(t, nil)
	for _, tc := range []struct {
		key  int
		want string
	}{{1, "type:epic"}, {2, "type:feature"}, {3, "type:task"}, {4, "type:bug"}} {
		m = pressFKey(m, tc.key)
		if got := m.QueryText(); got != tc.want {
			t.Fatalf("F%d: query = %q, want %q", tc.key, got, tc.want)
		}
	}
	if n := len(m.FilteredIssues()); n != 1 {
		t.Errorf("F4 shows %d issues, want only the bug", n)
	}
	m = pressFKey(m, 4)
	if got := m.QueryText(); got != "" {
		t.Fatalf("F4 again must show every type, query = %q", got)
	}
}

func TestFunctionKeysKeepOtherQueryTerms(t *testing.T) {
	m := keybindingTestModel(t, nil)
	m.setQueryText("status:open")
	m.queryState.Accept()
	m = pressFKey(m, 3)
	if got := m.QueryText(); got != "status:open type:task" {
		t.Fatalf("query = %q", got)
	}
	m = pressFKey(m, 1)
	if got := m.QueryText(); got != "status:open type:epic" {
		t.Fatalf("F1 must replace the type and keep the rest, query = %q", got)
	}
	m = pressFKey(m, 1)
	if got := m.QueryText(); got != "status:open" {
		t.Fatalf("F1 again must keep the other terms, query = %q", got)
	}
}

func TestFunctionKeyClearsItsTypeWhereverTheTermStands(t *testing.T) {
	m := keybindingTestModel(t, nil)
	m.setQueryText("type:epic status:open")
	m.queryState.Accept()
	m = pressFKey(m, 1)
	if got := m.QueryText(); got != "status:open" {
		t.Fatalf("F1 on a leading type:epic: query = %q, want status:open", got)
	}
}

func TestFunctionKeysWorkInTreeAndBoard(t *testing.T) {
	m := keybindingTestModel(t, nil)
	m = pressFKey(m, 1)
	if got := m.TreeNodeCount(); got != 1 {
		t.Errorf("tree shows %d nodes, want only the epic", got)
	}
	m = pressNamedKey(m, "b")
	if !m.IsBoardView() {
		t.Fatal("b should open the board")
	}
	if got := m.QueryText(); got != "type:epic" {
		t.Errorf("board lost the shared query: %q", got)
	}
	m = pressFKey(m, 4)
	if got := m.QueryText(); got != "type:bug" {
		t.Errorf("F4 on the board: query = %q", got)
	}
}

// Help is only ?: F1 belongs to the epic type.
func TestF1DoesNotOpenHelp(t *testing.T) {
	m := keybindingTestModel(t, nil)
	m = pressFKey(m, 1)
	if m.showHelp {
		t.Fatal("F1 opened the help overlay")
	}
}

// The header's type legend names the function key of each type.
func TestTypeLegendShowsFunctionKeys(t *testing.T) {
	m := keybindingTestModel(t, nil)
	m = pressFKey(m, 1)
	view := stripANSI(m.View())
	for _, want := range []string{"F1", "F2", "F3", "F4", "type:epic"} {
		if !strings.Contains(view, want) {
			t.Errorf("view misses %q", want)
		}
	}
}

func TestFunctionKeysAreBuiltIn(t *testing.T) {
	_, errs := ResolveHotkeys(config.Hotkeys{{Name: "c", ShortCut: "F2", Command: "chore"}})
	if len(errs) != 1 {
		t.Fatalf("f2 is built in, errs = %v", errs)
	}
}
