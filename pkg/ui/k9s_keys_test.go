package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanderheijden86/b9s/pkg/config"
	"github.com/vanderheijden86/b9s/pkg/model"
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
	m.projectPicker = NewProjectPicker(nil, m.theme)
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

// Route through Update so a global shortcut cannot silently steal a view key.
func TestViewKeyRouting(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(*Model)
		keys  []tea.Msg
		check func(*testing.T, Model)
	}{
		{"tree bookmark", nil, []tea.Msg{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b"), Alt: true}}, func(t *testing.T, m Model) {
			if !m.tree.IsBookmarked(m.tree.GetSelectedID()) || m.isBoardView {
				t.Fatal("Alt+b must bookmark the tree row")
			}
		}},
		{"tree flat", nil, []tea.Msg{runeMsg("t")}, func(t *testing.T, m Model) {
			if !m.tree.IsFlatMode() || m.showTutorial {
				t.Fatal("t must select the flat layout (ADR 0027)")
			}
		}},
		{"tree tutorial", nil, []tea.Msg{runeMsg("`")}, func(t *testing.T, m Model) {
			if !m.showTutorial {
				t.Fatal("backtick must open the tutorial")
			}
		}},
		{"tree escape occur", func(m *Model) { m.tree.EnterOccurMode("Child") }, []tea.Msg{tea.KeyMsg{Type: tea.KeyEsc}}, func(t *testing.T, m Model) {
			if m.tree.IsOccurMode() || m.showQuitConfirm {
				t.Fatal("Esc must leave occur before considering quit")
			}
		}},
		{"tree escape XRay", func(m *Model) { m.tree.ToggleXRay() }, []tea.Msg{tea.KeyMsg{Type: tea.KeyEsc}}, func(t *testing.T, m Model) {
			if m.tree.IsXRayMode() || m.showQuitConfirm {
				t.Fatal("Esc must leave XRay before considering quit")
			}
		}},
		{"tree escape assignee", func(m *Model) {
			m.assigneeFilter = "alice"
			m.tree.SetAssigneeFilter("alice")
		}, []tea.Msg{tea.KeyMsg{Type: tea.KeyEsc}}, func(t *testing.T, m Model) {
			if m.assigneeFilter != "" || m.tree.GetAssigneeFilter() != "" || m.showQuitConfirm {
				t.Fatal("Esc must clear the assignee filter")
			}
		}},
		{"tree escape status", func(m *Model) {
			m.currentFilter = "open"
			m.tree.ApplyFilter("open")
			m.applyFilter()
		}, []tea.Msg{tea.KeyMsg{Type: tea.KeyEsc}}, func(t *testing.T, m Model) {
			if m.currentFilter != "all" || m.tree.GetFilter() != "all" || m.showQuitConfirm {
				t.Fatal("Esc must clear the status filter in every view")
			}
		}},
		{"tree escape occur before query", func(m *Model) {
			m.setQueryText("Child")
			m.tree.EnterOccurMode("Child")
		}, []tea.Msg{tea.KeyMsg{Type: tea.KeyEsc}}, func(t *testing.T, m Model) {
			if m.tree.IsOccurMode() || m.queryState.Text() != "Child" || m.showQuitConfirm {
				t.Fatal("Esc must leave occur before clearing the shared query")
			}
		}},
		{"board g", routingBoard, []tea.Msg{runeMsg("g")}, func(t *testing.T, m Model) {
			if !m.isBoardView || m.isGraphView || !m.board.IsWaitingForG() {
				t.Fatal("board g must wait for the second g")
			}
		}},
		{"board gg", routingBoard, []tea.Msg{runeMsg("g"), runeMsg("g")}, routingBoardTop},
		{"board zero", routingBoard, []tea.Msg{runeMsg("0")}, routingBoardTop},
		{"board home", routingBoard, []tea.Msg{tea.KeyMsg{Type: tea.KeyHome}}, routingBoardTop},
		{"board end", routingBoard, []tea.Msg{tea.KeyMsg{Type: tea.KeyHome}, runeMsg("G")}, func(t *testing.T, m Model) {
			if !m.isBoardView || m.board.SelectedIssue() == nil || m.board.SelectedIssue().ID != "key-01" {
				t.Fatal("G must select the last card")
			}
		}},
		{"graph enter", func(m *Model) {
			m.isGraphView, m.focused = true, focusGraph
			m.graph.SetIssues(m.issues)
			m.graph.SelectIssue("key-20")
		}, []tea.Msg{tea.KeyMsg{Type: tea.KeyEnter}}, func(t *testing.T, m Model) {
			if m.focused != focusDetail || m.detailIssue() == nil || m.detailIssue().ID != "key-20" {
				t.Fatalf("graph Enter must show key-20, got %v", m.detailIssue())
			}
		}},
		{"project filter", nil, []tea.Msg{runeMsg("P")}, func(t *testing.T, m Model) {
			if !m.projectPicker.Filtering() || !m.pickerVisible {
				t.Fatal("P must open project filtering")
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newK9sKeysModel(t)
			if tc.setup != nil {
				tc.setup(&m)
			}
			m = pressK9sKey(m, tc.keys...)
			tc.check(t, m)
		})
	}
}

func routingBoard(m *Model) {
	m.isBoardView, m.focused = true, focusBoard
	m.board.SetIssues(m.issues)
	m.board.SelectIssueByID("key-20")
	m.board.MoveToBottom()
}

func routingBoardTop(t *testing.T, m Model) {
	t.Helper()
	if !m.isBoardView || m.isGraphView || m.board.SelectedIssue() == nil || m.board.SelectedIssue().ID != "key-60" {
		t.Fatalf("key must select the first board card, got %v", m.board.SelectedIssue())
	}
}

func TestPickerKeyRoutingIsolation(t *testing.T) {
	for _, picker := range []string{"label", "project"} {
		for _, key := range []string{"b", "g", "l", "K", "1", "0", "?", ":", "`", "H", "P", "D"} {
			t.Run(picker+"/"+key, func(t *testing.T) {
				m := newK9sKeysModel(t)
				if picker == "label" {
					m.showLabelPicker, m.focused = true, focusLabelPicker
				} else {
					m.projectPicker, _ = m.projectPicker.Update(runeMsg("/"))
				}
				m = pressK9sKey(m, runeMsg(key))
				input := m.labelPicker.InputValue()
				if picker == "project" {
					input = m.projectPicker.filterInput.Value()
				}
				if input != key || m.isBoardView || m.isGraphView || m.showHelp || m.showTutorial || m.issueConfirm.action != issueConfirmNone {
					t.Fatalf("%s picker lost %q to a global shortcut: input=%q", picker, key, input)
				}
			})
		}
	}
}

func TestRemovedEditorKeyRouting(t *testing.T) {
	for _, view := range []focus{focusTree, focusBoard, focusGraph, focusDetail} {
		t.Run(fmt.Sprint(view), func(t *testing.T) {
			m := newK9sKeysModel(t)
			m.focused = view
			m.isBoardView = view == focusBoard
			m.isGraphView = view == focusGraph
			m = pressK9sKey(m, runeMsg("O"))
			if m.statusMsg != "" || m.focused != view {
				t.Fatalf("removed O action must have no effect: status=%q focus=%v", m.statusMsg, m.focused)
			}
		})
	}
}

func routingKey(key string) tea.KeyMsg {
	special := map[string]tea.KeyType{
		"home": tea.KeyHome, "end": tea.KeyEnd, "pgdown": tea.KeyPgDown, "pgup": tea.KeyPgUp,
		"enter": tea.KeyEnter, "tab": tea.KeyTab, "shift+tab": tea.KeyShiftTab,
		"space": tea.KeySpace, "backspace": tea.KeyBackspace,
		"esc": tea.KeyEsc, "delete": tea.KeyDelete,
	}
	if typ, ok := special[key]; ok {
		return tea.KeyMsg{Type: typ}
	}
	if key == "alt+b" {
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b"), Alt: true}
	}
	if msg, ok := keyMsgFor(key); ok {
		return msg
	}
	panic("routing guard has no key message for " + key)
}

func TestAdvertisedCommonKeysRouteThroughModel(t *testing.T) {
	for _, view := range []focus{focusTree, focusBoard, focusGraph, focusDetail} {
		for _, shortcut := range commonKeyHelp() {
			t.Run(fmt.Sprintf("%v/%s", view, shortcut.key), func(t *testing.T) {
				m := newK9sKeysModel(t)
				m.issueWriter = &IssueWriter{bdPath: "/bin/echo", available: true, checkout: testCheckout(t)}
				m.focused, m.isBoardView, m.isGraphView = view, view == focusBoard, view == focusGraph
				wasBoard := m.isBoardView
				updated, cmd := m.Update(routingKey(shortcut.key))
				m = updated.(Model)
				var ok bool
				switch shortcut.key {
				case "E":
					ok = m.focused == focusTree && !m.isBoardView
				case "b":
					ok = m.isBoardView != wasBoard && !m.isGraphView &&
						((m.isBoardView && m.focused == focusBoard) || (!m.isBoardView && m.focused == focusTree))
				case "?":
					ok = m.focused == focusHelp && m.showHelp
				case ":":
					ok = m.commandPrompt.Mode() == PromptEditing
				case "ctrl+e":
					ok = !m.pickerVisible
				case "P":
					ok = m.projectPicker.Filtering()
				case "`":
					ok = m.showTutorial
				case "esc":
					ok = (view == focusTree && m.showQuitConfirm) ||
						(view != focusTree && m.focused == focusTree && !m.showQuitConfirm)
				case "ctrl+c":
					if cmd != nil {
						_, ok = cmd().(tea.QuitMsg)
					}
				case "K":
					ok = m.issueConfirm.action == issueConfirmClose
				case "delete":
					ok = m.issueConfirm.action == issueConfirmDelete
				case "ctrl+r", "f5":
					ok = !m.lastForceRefresh.IsZero()
				case "ctrl+n":
					ok = m.showEditModal
				default:
					t.Fatalf("add a dispatch assertion before advertising %q", shortcut.key)
				}
				if !ok {
					t.Fatalf("advertised common key %q did not reach its intended handler", shortcut.key)
				}
			})
		}
	}
}

func routingState(m Model) string {
	return fmt.Sprintf("%v|%v|%v|%v|%v|%s|%s|%s|%s|%s|%v|%v|%v|%v|%v|%v|%v|%v",
		m.focused, m.isBoardView, m.isGraphView, m.showTutorial, m.showQuitConfirm,
		m.queryState.Text(), m.currentFilter, m.tree.View(), m.board.View(100, 30), m.graph.View(100, 30),
		m.queryState.Mode(), m.tree.IsSortPopupOpen(), m.tree.IsColumnPopupOpen(), m.treeDetailHidden,
		m.showEditModal, m.showStatusPicker, m.statusTargets, m.helpSearching)
}

func TestAdvertisedViewKeysRouteThroughModel(t *testing.T) {
	for _, view := range []focus{focusTree, focusBoard, focusGraph, focusDetail, focusHelp} {
		for _, shortcut := range viewKeyHelp(view) {
			t.Run(fmt.Sprintf("%v/%s", view, shortcut.key), func(t *testing.T) {
				makeModel := func() Model {
					m := newK9sKeysModel(t)
					m.focused = view
					m.isBoardView, m.isGraphView = view == focusBoard, view == focusGraph
					m.showHelp = view == focusHelp
					m.issueWriter = &IssueWriter{bdPath: "/bin/echo", available: true, checkout: testCheckout(t)}
					m.tree.SelectByID("key-20")
					m.tree.bookmarks = map[string]bool{"key-40": true}
					m.tree.EnterSearchMode()
					for _, r := range "Child" {
						m.tree.SearchAddChar(r)
					}
					m.tree.ExitSearchMode()
					m.tree.SelectByID("key-20")
					m.graph.SetIssues(m.issues)
					m.graph.SelectIssue("key-20")
					m.board.SelectIssueByID("key-20")
					return m
				}
				msg := routingKey(shortcut.key)
				expected := makeModel()
				switch shortcut.key {
				case "e":
					if view == focusTree || view == focusDetail {
						expected.openEditModal(expected.getSelectedIssue())
					}
				case "ctrl+f":
					if view == focusTree || view == focusBoard {
						expected.toggleQueryFind()
					}
				case "g":
					if view == focusTree {
						expected.graph.SelectIssue(expected.getSelectedIssue().ID)
						expected.isGraphView, expected.focused = true, focusGraph
					} else if view == focusGraph {
						expected.isGraphView, expected.focused = false, focusTree
					}
				}
				switch view {
				case focusTree:
					expected = expected.handleTreeKeys(msg)
				case focusBoard:
					expected = expected.handleBoardKeys(msg)
				case focusGraph:
					expected = expected.handleGraphKeys(msg)
				case focusHelp:
					expected = expected.handleHelpKeys(msg)
				}
				expected.syncTreeSize()
				actual := pressK9sKey(makeModel(), msg)
				got, want := routingState(actual), routingState(expected)
				if got != want {
					at := 0
					for at < len(got) && at < len(want) && got[at] == want[at] {
						at++
					}
					t.Fatalf("help key %q differs at %d: got %q want %q. Fix routing or remove this help entry",
						shortcut.key, at, got[max(0, at-50):min(len(got), at+100)], want[max(0, at-50):min(len(want), at+100)])
				}
			})
		}
	}
}

func TestProjectPickerFilterSelectionRouting(t *testing.T) {
	for _, key := range []tea.KeyType{tea.KeyEnter, tea.KeyEsc} {
		t.Run(tea.KeyMsg{Type: key}.String(), func(t *testing.T) {
			m := newK9sKeysModel(t)
			m.projectPicker = NewProjectPicker([]ProjectEntry{
				{Project: config.Project{Name: "alpha"}},
				{Project: config.Project{Name: "gamma"}},
				{Project: config.Project{Name: "gamma-two"}},
			}, m.theme)
			m = pressK9sKey(m, runeMsg("P"), runeMsg("g"), runeMsg("a"))
			if m.projectPicker.FilteredCount() != 2 || m.queryState.Text() != "" {
				t.Fatal("P must filter projects without changing the issue query")
			}
			m = pressK9sKey(m, tea.KeyMsg{Type: tea.KeyDown})
			updated, cmd := m.Update(tea.KeyMsg{Type: key})
			m = updated.(Model)
			if m.projectPicker.Filtering() || m.showQuitConfirm {
				t.Fatal("Enter/Esc must close only the project filter")
			}
			if key == tea.KeyEsc {
				if cmd != nil || m.projectPicker.FilteredCount() != 3 {
					t.Fatal("Esc must restore the full project list without switching")
				}
			} else {
				if cmd == nil {
					t.Fatal("Enter must return a project switch command")
				}
				switchMsg, ok := cmd().(SwitchProjectMsg)
				if !ok || switchMsg.Project.Name != "gamma-two" {
					t.Fatalf("Enter selected %v, want gamma-two", switchMsg)
				}
			}
		})
	}
}

func TestProjectFilterReopensWithAllEntries(t *testing.T) {
	for _, tc := range []struct {
		name    string
		query   string
		matches int
	}{
		{"current project", "gamma", 2},
		{"no matches", "zzzzzz", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newK9sKeysModel(t)
			m.activeProjectName = "gamma"
			m.projectPicker = NewProjectPicker([]ProjectEntry{
				{Project: config.Project{Name: "alpha"}},
				{Project: config.Project{Name: "gamma"}},
				{Project: config.Project{Name: "gamma-two"}},
			}, m.theme)
			m = pressK9sKey(m, runeMsg("P"), runeMsg(tc.query))
			if m.projectPicker.FilteredCount() != tc.matches {
				t.Fatalf("query must produce %d rows", tc.matches)
			}
			next, cmd := m.Update(routingKey("enter"))
			m = next.(Model)
			if cmd != nil {
				m = pressK9sKey(m, cmd())
			}
			m = pressK9sKey(m, runeMsg("P"))
			if !m.projectPicker.Filtering() || m.projectPicker.filterInput.Value() != "" ||
				m.projectPicker.FilteredCount() != 3 {
				t.Fatalf("reopened empty filter must show all three projects, query=%q rows=%d",
					m.projectPicker.filterInput.Value(), m.projectPicker.FilteredCount())
			}
			if entry := m.projectPicker.SelectedEntry(); entry == nil || entry.Project.Name != "alpha" {
				t.Fatalf("reopened filter must select the first project, got %v", entry)
			}
		})
	}
}

func TestProjectFilterEntryRespectsOverlays(t *testing.T) {
	for _, overlay := range []string{"?", "`", "D"} {
		t.Run(overlay, func(t *testing.T) {
			m := pressK9sKey(newK9sKeysModel(t), runeMsg(overlay), runeMsg("P"))
			if m.projectPicker.Filtering() {
				t.Fatal("P must not start an invisible filter behind an overlay")
			}
			if overlay == "?" {
				m = pressK9sKey(m, runeMsg("?"), tea.KeyMsg{Type: tea.KeyCtrlS}, runeMsg("graph"))
				if !m.helpSearching || m.helpSearchInput.Value() != "graph" {
					t.Fatal("help search must retain input ownership")
				}
			}
		})
	}
}

func TestProjectFilterSurvivesRefreshRouting(t *testing.T) {
	for _, refresh := range []string{"counts", "snapshot"} {
		for _, key := range []string{"b", "g", "K", "1", "?", "`"} {
			t.Run(refresh+"/"+key, func(t *testing.T) {
				m := newK9sKeysModel(t)
				m.allProjects = []config.Project{
					{Name: "gamma", Path: "/fixture/gamma"},
					{Name: "gamma-two", Path: "/fixture/gamma-two"},
				}
				m.projectPicker = NewProjectPicker(m.buildProjectEntries(), m.theme)
				m = pressK9sKey(m, runeMsg("P"), runeMsg("g"), runeMsg("a"), tea.KeyMsg{Type: tea.KeyDown})
				cursor := m.projectPicker.Cursor()
				var msg tea.Msg = projectCountsLoadedMsg{counts: map[string]projectCounts{
					"/fixture/gamma": {open: 42}, "/fixture/gamma-two": {open: 42},
				}}
				if refresh == "snapshot" {
					msg = SnapshotReadyMsg{Snapshot: NewSnapshotBuilder(m.issues).Build()}
				}
				m = pressK9sKey(m, msg)
				if !m.projectPicker.Filtering() || m.projectPicker.filterInput.Value() != "ga" ||
					m.projectPicker.Cursor() != cursor || m.projectPicker.FilteredCount() != 2 {
					t.Fatal("refresh must preserve the filter, input and selection")
				}
				if refresh == "counts" && m.projectPicker.SelectedEntry().OpenCount != 42 {
					t.Fatal("refresh must also apply the project counts")
				}
				m = pressK9sKey(m, runeMsg(key))
				if m.projectPicker.filterInput.Value() != "ga"+key || m.showHelp || m.showTutorial ||
					m.isBoardView || m.isGraphView || m.issueConfirm.action != issueConfirmNone {
					t.Fatal("refresh must preserve input ownership against global shortcuts")
				}
			})
		}
	}
}

func TestStatusHelpMatchesSupportedViews(t *testing.T) {
	for _, view := range []focus{focusTree, focusBoard, focusGraph, focusDetail} {
		t.Run(fmt.Sprint(view), func(t *testing.T) {
			advertised := false
			for _, row := range append(commonKeyHelp(), viewKeyHelp(view)...) {
				advertised = advertised || row.key == "S"
			}
			m := newK9sKeysModel(t)
			m.focused, m.isBoardView, m.isGraphView = view, view == focusBoard, view == focusGraph
			m = pressK9sKey(m, runeMsg("S"))
			if advertised != m.showStatusPicker || (view == focusTree && !advertised) {
				t.Fatalf("S help must match dispatch in view %v: advertised=%v handled=%v", view, advertised, m.showStatusPicker)
			}
		})
	}
}

func labelRoutingModel(t *testing.T, view focus) Model {
	t.Helper()
	issues := k9sKeyIssues()
	issues[1].Labels = []string{"keep"}
	m := NewModel(issues, "")
	m.projectPicker = NewProjectPicker(nil, m.theme)
	m.tree.SetBeadsDir(t.TempDir())
	m = pressK9sKey(m, tea.WindowSizeMsg{Width: k9sKeysWidth, Height: 20})
	switch view {
	case focusTree:
		m.tree.ExpandPathTo("key-20")
		m.tree.SelectByID("key-20")
		m.syncTreeToDetail()
	case focusBoard:
		routingBoard(&m)
		m.board.SelectIssueByID("key-20")
	case focusGraph:
		m = pressK9sKey(m, runeMsg("g"))
		m.graph.SelectIssue("key-20")
	}
	return m
}

func TestLabelPickerRestoresOpeningFocus(t *testing.T) {
	for _, view := range []focus{focusGraph, focusTree, focusBoard} {
		for _, detail := range []bool{false, true} {
			if view != focusGraph && !detail {
				continue // l belongs to tree and board navigation.
			}
			for _, dismiss := range []string{"esc", "enter"} {
				t.Run(fmt.Sprintf("%v/detail=%v/%s", view, detail, dismiss), func(t *testing.T) {
					m := labelRoutingModel(t, view)
					if detail {
						m = pressK9sKey(m, routingKey("enter"))
					}
					openingFocus := m.focused
					m = pressK9sKey(m, runeMsg("l"))
					if !m.showLabelPicker {
						t.Fatal("l must open the label picker")
					}
					m = pressK9sKey(m, routingKey(dismiss))
					if m.showLabelPicker || m.focused != openingFocus {
						t.Fatalf("label picker must return to %v, got %v", openingFocus, m.focused)
					}
				})
			}
		}
	}
}

func TestDetailSelectionSurvivesLabelPicker(t *testing.T) {
	for _, view := range []focus{focusTree, focusBoard, focusGraph} {
		for _, dismiss := range []string{"esc", "enter"} {
			t.Run(fmt.Sprintf("%v/%s", view, dismiss), func(t *testing.T) {
				m := labelRoutingModel(t, view)
				m = pressK9sKey(m, routingKey("enter"))
				if shown := m.detailIssue(); shown == nil || shown.ID != "key-20" {
					t.Fatalf("fixture must show key-20, got %v", shown)
				}
				m = pressK9sKey(m, runeMsg("l"), routingKey(dismiss))
				if shown := m.detailIssue(); shown == nil || shown.ID != "key-20" {
					t.Fatalf("label picker must retain displayed key-20, got %v", shown)
				}
				if dismiss == "enter" && m.labelFilter != "keep" {
					t.Fatal("fixture must apply a label excluding the displayed issue")
				}
			})
		}
	}
}

func TestDetailActionsAfterExcludedLabel(t *testing.T) {
	for _, view := range []focus{focusTree, focusBoard, focusGraph} {
		for _, key := range []string{"e", "K", "delete"} {
			t.Run(fmt.Sprintf("%v/%s", view, key), func(t *testing.T) {
				m := labelRoutingModel(t, view)
				m = pressK9sKey(m, routingKey("enter"), runeMsg("l"), routingKey("enter"))
				if shown := m.detailIssue(); m.focused != focusDetail || shown == nil || shown.ID != "key-20" {
					t.Fatalf("label dismissal must retain displayed key-20, got %v", shown)
				}
				if m.labelFilter != "keep" || m.tree.GetSelectedID() == "key-20" {
					t.Fatal("fixture must exclude the displayed issue from the tree")
				}
				m = pressK9sKey(m, routingKey(key))
				if key == "e" {
					if !m.showEditModal || m.editModal.issueID != "key-20" {
						t.Fatalf("edit must target displayed key-20, got %q", m.editModal.issueID)
					}
				} else if m.issueConfirm.id != "key-20" || len(m.issueConfirm.ids) != 0 {
					t.Fatalf("%s must target displayed key-20, got id=%q ids=%v", key, m.issueConfirm.id, m.issueConfirm.ids)
				}
			})
		}
	}
}

func TestDetailActionsIgnoreTreeMarks(t *testing.T) {
	for _, key := range []string{"K", "delete"} {
		t.Run(key, func(t *testing.T) {
			m := labelRoutingModel(t, focusTree)
			m.tree.SelectByID("key-01")
			m.tree.ToggleMark()
			m.tree.SelectByID("key-20")
			m = pressK9sKey(m, routingKey("enter"), routingKey(key))
			if m.issueConfirm.id != "key-20" || len(m.issueConfirm.ids) != 0 {
				t.Fatalf("detail action must target displayed key-20, got id=%q ids=%v", m.issueConfirm.id, m.issueConfirm.ids)
			}
		})
	}
}

func TestGraphKeysAfterLabelPicker(t *testing.T) {
	for _, dismiss := range []string{"esc", "enter"} {
		for _, key := range []string{"enter", "E", "j"} {
			t.Run(dismiss+"/"+key, func(t *testing.T) {
				m := labelRoutingModel(t, focusGraph)
				m = pressK9sKey(m, runeMsg("l"), routingKey(dismiss))
				selected := m.graph.SelectedIssue()
				if selected == nil {
					t.Fatal("graph must have a row after label dismissal")
				}
				id := selected.ID
				m = pressK9sKey(m, routingKey(key))
				switch key {
				case "enter":
					if shown := m.detailIssue(); m.focused != focusDetail || shown == nil || shown.ID != id {
						t.Fatalf("Enter must open graph row %s, got %v", id, shown)
					}
				case "E":
					if m.isGraphView || m.focused != focusTree {
						t.Fatal("E must return from graph to tree")
					}
				case "j":
					if row := m.graph.SelectedIssue(); m.focused != focusGraph || row == nil || row.ID == id {
						t.Fatal("j must move the graph cursor after label dismissal")
					}
				}
			})
		}
	}
}

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

// Right and Left already page the tree by a full viewport. Ctrl-F is not a
// page key: it switches the search between filter and find (ADR 0057).
func TestTreeFullPageKeys(t *testing.T) {
	down := pressK9sKey(newK9sKeysModel(t), tea.KeyMsg{Type: tea.KeyRight})
	if down.tree.cursor == 0 {
		t.Fatal("Right did not page the fixture")
	}
	for _, key := range []tea.KeyType{tea.KeyPgDown} {
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
		TreeColumnDispatch: ColumnAuto,
		TreeColumnUpdated:  ColumnHide,
		TreeColumnID:       ColumnAuto,
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

func TestExpandedPickerKeepsDetailFooterOnLastRow(t *testing.T) {
	m := pressK9sKey(newK9sKeysModel(t), tea.KeyMsg{Type: tea.KeyEnter})
	m.statusMsg = ""

	lines := strings.Split(stripANSI(m.View()), "\n")
	if len(lines) != m.height {
		t.Fatalf("view has %d lines, want terminal height %d", len(lines), m.height)
	}
	if last := lines[len(lines)-1]; !strings.Contains(last, "enter:back to tree") {
		t.Errorf("last line = %q, want the detail footer", last)
	}
}

func TestCollapsedPickerKeepsDetailFooterOnLastRow(t *testing.T) {
	m := pressK9sKey(newK9sKeysModel(t), tea.KeyMsg{Type: tea.KeyEnter}, runeMsg("H"))
	m.statusMsg = ""

	lines := strings.Split(stripANSI(m.View()), "\n")
	if len(lines) != m.height {
		t.Fatalf("view has %d lines, want terminal height %d", len(lines), m.height)
	}
	if last := lines[len(lines)-1]; !strings.Contains(last, "enter:back to tree") {
		t.Errorf("last line = %q, want the detail footer", last)
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
	for _, key := range []tea.KeyType{tea.KeyPgDown} {
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
	for _, key := range []tea.KeyType{tea.KeyPgDown} {
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
