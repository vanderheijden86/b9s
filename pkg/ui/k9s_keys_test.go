package ui

import (
	"fmt"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanderheijden86/beadwork/pkg/model"
)

// k9s tables (tview) bind g/Home to the first row, G/End to the last row, and
// Ctrl-F/PgDn and Ctrl-B/PgUp to a full page. b9s keeps g for dependencies and
// Ctrl-D/Ctrl-U for half pages, and otherwise follows those bindings.

// Narrower than SplitViewThreshold, so every key reaches the tree itself.
const k9sKeysWidth = 90

func k9sKeyIssues() []model.Issue {
	now := time.Now()
	issues := []model.Issue{
		{ID: "epic-k", Title: "Keys Epic", Status: model.StatusOpen, Priority: 1, IssueType: model.TypeEpic, CreatedAt: now},
	}
	for i := 1; i <= 60; i++ {
		id := fmt.Sprintf("key-%02d", i)
		issues = append(issues, model.Issue{
			ID: id, Title: fmt.Sprintf("Key Child %02d", i), Status: model.StatusOpen, Priority: 2,
			IssueType: model.TypeTask, CreatedAt: now.Add(time.Duration(i) * time.Second),
			Dependencies: []*model.Dependency{{IssueID: id, DependsOnID: "epic-k", Type: model.DepParentChild}},
		})
	}
	return issues
}

func newK9sKeysModel(t *testing.T) Model {
	t.Helper()
	m := NewModel(k9sKeyIssues(), "")
	m.tree.SetBeadsDir(t.TempDir())
	m = pressK9sKey(m, tea.WindowSizeMsg{Width: k9sKeysWidth, Height: 20})
	if m.focused != focusTree {
		t.Fatalf("expected the tree to be focused, got %v", m.focused)
	}
	if m.tree.NodeCount() <= 20 {
		t.Fatalf("fixture must overflow the viewport, got %d nodes", m.tree.NodeCount())
	}
	return m
}

func pressK9sKey(m Model, msgs ...tea.Msg) Model {
	for _, msg := range msgs {
		next, _ := m.Update(msg)
		m = next.(Model)
	}
	return m
}

func runeMsg(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

func TestTreeEndSelectsLastNode(t *testing.T) {
	m := pressK9sKey(newK9sKeysModel(t), tea.KeyMsg{Type: tea.KeyEnd})
	want := pressK9sKey(newK9sKeysModel(t), runeMsg("G"))
	if got := m.tree.GetSelectedID(); got == "" || got != want.tree.GetSelectedID() {
		t.Fatalf("End selected %q, G selects %q", got, want.tree.GetSelectedID())
	}
}

func TestTreeHomeAfterEndSelectsFirstNode(t *testing.T) {
	m := pressK9sKey(newK9sKeysModel(t), tea.KeyMsg{Type: tea.KeyEnd}, tea.KeyMsg{Type: tea.KeyHome})
	if m.tree.cursor != 0 || m.tree.viewportOffset != 0 {
		t.Fatalf("Home left cursor %d, offset %d; want 0, 0", m.tree.cursor, m.tree.viewportOffset)
	}
}

// Right and Left already page the tree by a full viewport.
func TestTreeFullPageKeys(t *testing.T) {
	down := pressK9sKey(newK9sKeysModel(t), tea.KeyMsg{Type: tea.KeyRight})
	if down.tree.cursor == 0 {
		t.Fatal("Right did not page the fixture")
	}
	for _, key := range []tea.KeyType{tea.KeyCtrlF, tea.KeyPgDown} {
		m := pressK9sKey(newK9sKeysModel(t), tea.KeyMsg{Type: key})
		if m.tree.cursor != down.tree.cursor || m.tree.viewportOffset != down.tree.viewportOffset {
			t.Errorf("%s: cursor %d offset %d, want a full page like Right: cursor %d offset %d",
				tea.KeyMsg{Type: key}, m.tree.cursor, m.tree.viewportOffset, down.tree.cursor, down.tree.viewportOffset)
		}
	}

	bottom := []tea.Msg{runeMsg("G")}
	up := pressK9sKey(newK9sKeysModel(t), append(bottom, tea.KeyMsg{Type: tea.KeyLeft})...)
	for _, key := range []tea.KeyType{tea.KeyCtrlB, tea.KeyPgUp} {
		m := pressK9sKey(newK9sKeysModel(t), append(bottom, tea.KeyMsg{Type: key})...)
		if m.tree.cursor != up.tree.cursor || m.tree.viewportOffset != up.tree.viewportOffset {
			t.Errorf("%s: cursor %d offset %d, want a full page like Left: cursor %d offset %d",
				tea.KeyMsg{Type: key}, m.tree.cursor, m.tree.viewportOffset, up.tree.cursor, up.tree.viewportOffset)
		}
	}
}

func TestTreeCtrlWShowsEveryOptionalColumn(t *testing.T) {
	m := newK9sKeysModel(t)
	m.tree.SetColumnPreference(TreeColumnUpdated, ColumnHide)
	m = pressK9sKey(m, tea.KeyMsg{Type: tea.KeyCtrlW})
	for c := TreeColumn(0); c < treeColumnCount; c++ {
		if got := m.tree.ColumnPreference(c); got != ColumnShow {
			t.Errorf("wide: %s is %s, want Show", c, got)
		}
	}
}

func TestTreeCtrlWAgainRestoresColumnPreferences(t *testing.T) {
	m := newK9sKeysModel(t)
	m.tree.SetColumnPreference(TreeColumnUpdated, ColumnHide)
	m = pressK9sKey(m, tea.KeyMsg{Type: tea.KeyCtrlW}, tea.KeyMsg{Type: tea.KeyCtrlW})
	want := map[TreeColumn]ColumnPreference{
		TreeColumnLaneStage: ColumnAuto,
		TreeColumnUpdated:   ColumnHide,
		TreeColumnID:        ColumnAuto,
	}
	for c, pref := range want {
		if got := m.tree.ColumnPreference(c); got != pref {
			t.Errorf("narrow again: %s is %s, want %s", c, got, pref)
		}
	}
}

func TestCtrlETogglesHeaderLikeH(t *testing.T) {
	m := newK9sKeysModel(t)
	before := m.pickerVisible
	withH := pressK9sKey(m, runeMsg("H"))
	withCtrlE := pressK9sKey(m, tea.KeyMsg{Type: tea.KeyCtrlE})
	if withH.pickerVisible == before {
		t.Fatal("H did not toggle the header")
	}
	if withCtrlE.pickerVisible != withH.pickerVisible {
		t.Fatalf("Ctrl-E left the header visible=%v, H makes it %v", withCtrlE.pickerVisible, withH.pickerVisible)
	}
}

func TestGraphCtrlFAndCtrlBPage(t *testing.T) {
	graph := pressK9sKey(newK9sKeysModel(t), runeMsg("g"), tea.KeyMsg{Type: tea.KeyHome})
	if !graph.isGraphView {
		t.Fatal("g did not open the graph")
	}
	selected := func(m Model) string {
		if issue := m.graph.SelectedIssue(); issue != nil {
			return issue.ID
		}
		return ""
	}
	start := selected(graph)
	for _, tc := range []struct{ key, same tea.KeyType }{
		{tea.KeyCtrlF, tea.KeyPgDown},
		{tea.KeyCtrlB, tea.KeyPgUp},
	} {
		keyed := pressK9sKey(graph, tea.KeyMsg{Type: tea.KeyPgDown}, tea.KeyMsg{Type: tc.key})
		ref := pressK9sKey(graph, tea.KeyMsg{Type: tea.KeyPgDown}, tea.KeyMsg{Type: tc.same})
		if selected(keyed) != selected(ref) {
			t.Errorf("%s selected %q, %s selects %q", tea.KeyMsg{Type: tc.key}, selected(keyed), tea.KeyMsg{Type: tc.same}, selected(ref))
		}
	}
	if paged := pressK9sKey(graph, tea.KeyMsg{Type: tea.KeyCtrlF}); selected(paged) == start {
		t.Error("Ctrl-F did not move the graph selection")
	}
}

func TestBoardCtrlFAndCtrlBPage(t *testing.T) {
	board := pressK9sKey(newK9sKeysModel(t), runeMsg("b"))
	if !board.isBoardView {
		t.Fatal("b did not open the board")
	}
	down := pressK9sKey(board, tea.KeyMsg{Type: tea.KeyCtrlD})
	if down.View() == board.View() {
		t.Fatal("Ctrl-D did not page the board fixture")
	}
	for _, key := range []tea.KeyType{tea.KeyCtrlF, tea.KeyPgDown} {
		if got := pressK9sKey(board, tea.KeyMsg{Type: key}); got.View() != down.View() {
			t.Errorf("%s did not page the board down", tea.KeyMsg{Type: key})
		}
	}
	up := pressK9sKey(down, tea.KeyMsg{Type: tea.KeyCtrlU})
	for _, key := range []tea.KeyType{tea.KeyCtrlB, tea.KeyPgUp} {
		if got := pressK9sKey(down, tea.KeyMsg{Type: key}); got.View() != up.View() {
			t.Errorf("%s did not page the board up", tea.KeyMsg{Type: key})
		}
	}
}

func TestHelpCtrlFAndCtrlBScroll(t *testing.T) {
	help := pressK9sKey(newK9sKeysModel(t), runeMsg("?"))
	if !help.showHelp {
		t.Fatal("? did not open help")
	}
	down := pressK9sKey(help, tea.KeyMsg{Type: tea.KeyCtrlD})
	for _, key := range []tea.KeyType{tea.KeyCtrlF, tea.KeyPgDown} {
		if got := pressK9sKey(help, tea.KeyMsg{Type: key}); got.helpScroll != down.helpScroll {
			t.Errorf("%s scrolled help to %d, Ctrl-D scrolls to %d", tea.KeyMsg{Type: key}, got.helpScroll, down.helpScroll)
		}
	}
	for _, key := range []tea.KeyType{tea.KeyCtrlB, tea.KeyPgUp} {
		if got := pressK9sKey(down, tea.KeyMsg{Type: key}); got.helpScroll != 0 {
			t.Errorf("%s left help scrolled to %d, want 0", tea.KeyMsg{Type: key}, got.helpScroll)
		}
	}
}
