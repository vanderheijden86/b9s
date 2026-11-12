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

// footerHintKeys reads the key of every hint the footer shows, so a test
// exercises what the user is told rather than a copy of the hint table.
func footerHintKeys(t *testing.T, m Model) []string {
	t.Helper()
	m.statusMsg = ""
	m.width = 400
	line := strings.TrimSpace(stripANSI(m.renderFooter()))
	var keys []string
	for _, part := range strings.Split(line, "  ") {
		key, _, ok := strings.Cut(strings.TrimSpace(part), ":")
		if !ok || key == "" {
			continue
		}
		keys = append(keys, key)
	}
	return keys
}

// footerKeyPress maps a footer hint key to a key press that exercises it.
// A hint with no entry here fails the guard, so adding a hint forces whoever
// adds it to say how the key is pressed and thereby to prove it works.
var footerKeyPress = map[string]tea.KeyMsg{
	"enter":    {Type: tea.KeyEnter},
	"j/k":      {Type: tea.KeyRunes, Runes: []rune("j")},
	"home/end": {Type: tea.KeyEnd},
	"1-9":      {Type: tea.KeyRunes, Runes: []rune("1")},
	"bksp":     {Type: tea.KeyBackspace},
	"^R":       {Type: tea.KeyCtrlR},
	"^o":       {Type: tea.KeyCtrlO},
	"n/p":      {Type: tea.KeyRunes, Runes: []rune("n")},
	"e":        {Type: tea.KeyRunes, Runes: []rune("e")},
	"s":        {Type: tea.KeyRunes, Runes: []rune("s")},
	"c":        {Type: tea.KeyRunes, Runes: []rune("c")},
	"?":        {Type: tea.KeyRunes, Runes: []rune("?")},
	"esc":      {Type: tea.KeyEsc},
	"L":        {Type: tea.KeyRunes, Runes: []rune("L")},
	"P":        {Type: tea.KeyRunes, Runes: []rune("P")},
	"H":        {Type: tea.KeyRunes, Runes: []rune("H")},
	"F1-4":     {Type: tea.KeyF1},
}

// effectOf is everything a key press may change that the user can see.
// The bottom of the detail counts too: the user scrolls there to read comments.
func effectOf(m Model) string {
	bottom := m.viewport
	bottom.GotoBottom()
	return fmt.Sprintf("%v|%v|%v|%v|%q|%d|%s|%s|%s",
		m.focused, m.showEditModal, m.pickerMode, m.pickerVisible,
		m.statusMsg, m.viewport.YOffset, m.TreeSelectedID(), stripANSI(m.View()),
		stripANSI(bottom.View()))
}

func detailFooterIssues() []model.Issue {
	now := time.Now()
	// Two comments give the comment-order key something visible to reorder.
	comments := func(id string) []*model.Comment {
		return []*model.Comment{
			{ID: id + "-c1", IssueID: id, Author: "a", Text: "First comment", CreatedAt: now.Add(-2 * time.Hour)},
			{ID: id + "-c2", IssueID: id, Author: "b", Text: "Second comment", CreatedAt: now.Add(-time.Hour)},
		}
	}
	child := func(id, parent string, age int) model.Issue {
		return model.Issue{
			ID: id, Title: "Title " + id, Status: model.StatusOpen, IssueType: model.TypeTask,
			Priority: 2, CreatedAt: now.Add(time.Duration(age) * time.Second),
			Description: strings.Repeat("A line of description.\n\n", 60),
			Comments:    comments(id),
			Dependencies: []*model.Dependency{
				{IssueID: id, DependsOnID: parent, Type: model.DepParentChild},
			},
		}
	}
	return []model.Issue{
		{ID: "ep-1", Title: "Epic", Status: model.StatusOpen, IssueType: model.TypeEpic, Priority: 1,
			CreatedAt: now.Add(9 * time.Second), Description: strings.Repeat("Epic line.\n\n", 60),
			Comments: comments("ep-1")},
		child("ch-1", "ep-1", 5),
		child("ch-2", "ep-1", 4),
		child("gc-1", "ch-1", 3),
	}
}

func detailFooterModel(t *testing.T, view string) Model {
	t.Helper()
	m := NewModel(detailFooterIssues(), "")
	m.allProjects = []config.Project{{Name: "one"}, {Name: "two"}}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = updated.(Model)
	switch view {
	case "tree":
		m.treeViewActive = true
		m.tree.SelectByID("ep-1")
		m.syncTreeToDetail()
		m.focused = focusDetail
		// Drill into ch-1 so Backspace has somewhere to go back to.
		m = pressKeys(m, runeKey("1"))
	case "board":
		m.isBoardView = true
		m.board.SelectIssueByID("ch-1")
		m.syncBoardToDetail()
		m.focused = focusDetail
	case "list":
		m.treeViewActive = false
		m.focused = focusDetail
		m.updateViewportContent()
	}
	if m.focused != focusDetail {
		t.Fatalf("%s: setup left focus at %v, want the detail", view, m.focused)
	}
	return m
}

// Every key the detail footer advertises must do something on the detail
// screen, in each view the detail can be reached from. A key that cannot work
// there must not be in the footer.
func TestDetailFooterKeysAllWork(t *testing.T) {
	t.Setenv("B9S_TEST_MODE", "1")
	for _, view := range []string{"tree", "board", "list"} {
		t.Run(view, func(t *testing.T) {
			m := detailFooterModel(t, view)
			keys := footerHintKeys(t, m)
			if len(keys) == 0 {
				t.Fatal("the footer shows no hints")
			}
			for _, key := range keys {
				press, ok := footerKeyPress[key]
				if !ok {
					t.Errorf("footer hint %q has no key press in footerKeyPress; add one so the guard exercises it", key)
					continue
				}
				// The tree's cursor is shared between copies of the model, so each
				// key starts from a model of its own.
				fresh := detailFooterModel(t, view)
				before := effectOf(fresh)
				after := pressKeys(fresh, press)
				if effectOf(after) == before {
					t.Errorf("footer advertises %q on the %s detail screen but pressing it changes nothing", key, view)
				}
			}
		})
	}
}

// Backspace goes back through the issues opened from the detail, so it is
// advertised only once there is somewhere to go back to.
func TestDetailFooterBackspaceHintNeedsHistory(t *testing.T) {
	m := detailFooterModel(t, "tree")
	if !contains(footerHintKeys(t, m), "bksp") {
		t.Error("expected bksp in the footer after opening a child")
	}
	m = pressKeys(m, tea.KeyMsg{Type: tea.KeyBackspace})
	if contains(footerHintKeys(t, m), "bksp") {
		t.Error("bksp is advertised with no issue to go back to")
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func viewFooterModel(t *testing.T, view string, mode int) Model {
	t.Helper()
	issues := detailFooterIssues()
	if view == "graph" {
		for n := 0; n < 25; n++ {
			issues = append(issues, model.Issue{
				ID: fmt.Sprintf("graph-%02d", n), Title: "Graph issue",
				Status: model.StatusOpen, IssueType: model.TypeTask,
			})
		}
	}
	m := NewModel(issues, "")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = updated.(Model)
	m.beadsPath = t.TempDir()
	var entries []ProjectEntry
	for n := 1; n <= 9; n++ {
		project := config.Project{Name: fmt.Sprintf("project-%d", n)}
		m.allProjects = append(m.allProjects, project)
		entries = append(entries, ProjectEntry{Project: project, FavoriteNum: n})
	}
	m.projectPicker.SetEntries(entries)
	m.currentFilter = "all"
	m.applyFilter()
	m.tree.ApplyFilter("all")
	m.tree.ExpandAll()
	m.tree.SelectByID("ep-1")
	m.focused = focusTree
	switch view {
	case "flat tree":
		m.tree.ToggleFlatMode()
	case "type:task", "!type:epic", "F3 type query":
		if view == "F3 type query" {
			m = pressKeys(m, tea.KeyMsg{Type: tea.KeyF3})
		} else {
			m.setQueryText(view)
		}
		if m.tree.IsFlatMode() || !m.tree.IsListView() {
			t.Fatal("type query must force list rows with the saved flat preference off")
		}
		m.tree.SelectByID("ch-1")
	case "board":
		m = pressKeys(m, runeKey("b"))
		m.board.SelectIssueByID("ch-1")
	case "graph":
		m = pressKeys(m, runeKey("g"))
	case "split", "list":
		// These fallback footers depend on list focus, rather than the
		// tree's flat layout (ADR 0027).
		m.treeViewActive = false
		m.focused = focusList
		m.isSplitView = view == "split"
	}
	m.pickerMode = mode
	if mode == pickerModeLabels {
		for n := 1; n <= 9; n++ {
			m.labelEntries = append(m.labelEntries, LabelEntry{
				Number: n, Label: fmt.Sprintf("label-%d", n),
			})
		}
	}
	if mode == pickerModeAssignees {
		for n := 1; n <= 9; n++ {
			m.assigneeEntries = append(m.assigneeEntries, AssigneeEntry{
				Number: n, Assignee: fmt.Sprintf("assignee-%d", n),
			})
		}
	}
	m.statusMsg = ""
	return m
}

// Composite hints promise each of their alternatives, not only the first key.
func viewFooterPresses(hint string) [][]tea.KeyMsg {
	switch hint {
	case "0-9", "1-9":
		first := 1
		if hint == "0-9" {
			first = 0
		}
		var presses [][]tea.KeyMsg
		for n := first; n <= 9; n++ {
			presses = append(presses, []tea.KeyMsg{runeKey(fmt.Sprint(n))})
		}
		return presses
	case "j/k":
		return [][]tea.KeyMsg{{runeKey("j")}, {runeKey("k")}}
	case "home/G":
		return [][]tea.KeyMsg{{{Type: tea.KeyHome}}, {runeKey("G")}}
	case "^d/^u":
		return [][]tea.KeyMsg{{{Type: tea.KeyCtrlD}}, {{Type: tea.KeyCtrlU}}}
	case "g/esc":
		return [][]tea.KeyMsg{{runeKey("g")}, {{Type: tea.KeyEsc}}}
	case "gg/0":
		return [][]tea.KeyMsg{{runeKey("g"), runeKey("g")}, {runeKey("0")}}
	case "</>":
		return [][]tea.KeyMsg{{runeKey("<")}, {runeKey(">")}}
	case "F1-4":
		return [][]tea.KeyMsg{
			{{Type: tea.KeyF1}}, {{Type: tea.KeyF2}},
			{{Type: tea.KeyF3}}, {{Type: tea.KeyF4}},
		}
	case "tab":
		return [][]tea.KeyMsg{{{Type: tea.KeyTab}}}
	case "⇧tab":
		return [][]tea.KeyMsg{{{Type: tea.KeyShiftTab}}}
	case "del":
		return [][]tea.KeyMsg{{{Type: tea.KeyDelete}}}
	case "^n":
		return [][]tea.KeyMsg{{{Type: tea.KeyCtrlN}}}
	}
	if press, ok := footerKeyPress[hint]; ok {
		return [][]tea.KeyMsg{{press}}
	}
	if len([]rune(hint)) == 1 {
		return [][]tea.KeyMsg{{runeKey(hint)}}
	}
	return nil
}

func TestViewFooterKeysAllWork(t *testing.T) {
	t.Setenv("B9S_TEST_MODE", "1")
	for _, view := range []string{
		"tree", "flat tree", "type:task", "!type:epic", "F3 type query",
		"board", "graph", "split", "list",
	} {
		for _, mode := range []int{pickerModeProjects, pickerModeLabels, pickerModeAssignees} {
			t.Run(fmt.Sprintf("%s/picker=%d", view, mode), func(t *testing.T) {
				keys := footerHintKeys(t, viewFooterModel(t, view, mode))
				if len(keys) == 0 {
					t.Fatal("footer shows no shortcuts")
				}
				for _, hint := range keys {
					presses := viewFooterPresses(hint)
					if len(presses) == 0 {
						t.Errorf("footer hint %q needs a key sequence and behavior assertion", hint)
					}
					for _, sequence := range presses {
						t.Run(hint+"/"+sequence[0].String(), func(t *testing.T) {
							m := viewFooterModel(t, view, mode)
							key := sequence[len(sequence)-1].String()
							// Navigation starts away from the relevant boundary.
							if key == "k" || hint == "gg/0" || key == "home" || key == "ctrl+u" {
								m.tree.JumpToBottom()
								m.board.MoveToBottom()
								m.graph.JumpToBottom()
							}
							if key == "0" && hint != "gg/0" {
								if mode == pickerModeLabels {
									m.labelFilter = "label-1"
								} else if mode == pickerModeAssignees {
									m.assigneeFilter = "assignee-1"
								}
							}
							selected := m.getSelectedIssue()
							if selected == nil {
								t.Fatal("fixture has no selected issue")
							}
							wantID, wantTitle := selected.ID, selected.Title
							var copied string
							m.clipboardWrite = func(text string) error { copied = text; return nil }
							advertised := footerText(m)
							oldQuery := m.queryState.Text()
							wasListView := m.tree.IsListView()
							wantNavID := ""
							if hint == "j/k" || hint == "home/G" || hint == "^d/^u" || hint == "gg/0" {
								var ids []string
								index := 0
								switch view {
								case "graph":
									ids, index = m.graph.sortedIDs, m.graph.selectedIdx
								case "board":
									col := m.board.actualFocusedCol()
									for _, issue := range m.board.columns[col] {
										ids = append(ids, issue.ID)
									}
									index = m.board.selectedRow[col]
								default:
									for _, node := range m.tree.flatList {
										ids = append(ids, node.Issue.ID)
									}
									index = m.tree.cursor
								}
								switch key {
								case "j":
									index++
								case "k":
									index--
								case "home", "g", "0":
									index = 0
									// The epic is a lane header, not the first card
									// in a status column (ADR 0030).
									if view == "board" {
										for m.issueMap[ids[index]].IssueType == model.TypeEpic {
											index++
										}
									}
								case "G":
									index = len(ids) - 1
								case "ctrl+d":
									index = min(index+10, len(ids)-1)
								case "ctrl+u":
									index = max(index-10, 0)
								}
								wantNavID = ids[index]
							}
							oldCount := m.tree.NodeCount()
							oldRatio, oldLayout := m.splitPaneRatio, m.splitStacked
							oldEpic, oldSwim := m.board.EpicView(), m.board.GetSwimLaneMode()
							oldHeader := m.pickerVisible
							var cmd tea.Cmd
							for _, press := range sequence {
								updated, next := m.Update(press)
								m, cmd = updated.(Model), next
							}
							ok := false
							switch hint {
							case "0-9", "1-9":
								label := "project"
								if mode == pickerModeLabels {
									label = "label"
								} else if mode == pickerModeAssignees {
									label = "assignee"
								}
								if !strings.Contains(advertised, hint+":"+label) {
									t.Errorf("numeric footer hint %q must describe %s selection", hint, label)
								}
								if key == "0" {
									if mode == pickerModeLabels {
										ok = m.labelFilter == ""
									} else if mode == pickerModeAssignees {
										ok = m.assigneeFilter == ""
									} else {
										// No server is needed to test the unavailable
										// all-projects action on a local fixture.
										ok = m.statusIsError && m.statusMsg == "No Dolt-backed projects found"
									}
								} else if mode == pickerModeLabels {
									ok = m.labelFilter == "label-"+key
								} else if mode == pickerModeAssignees {
									ok = m.assigneeFilter == "assignee-"+key
								} else if cmd != nil {
									msg, isSwitch := cmd().(SwitchProjectMsg)
									ok = isSwitch && msg.Project.Name == "project-"+key
								}
							case "^R":
								ok = m.doltForceFull && !m.lastForceRefresh.IsZero() &&
									m.statusMsg == "Refreshing…" && !m.statusIsError && cmd != nil
								if cmd != nil {
									_, reload := cmd().(FileChangedMsg)
									ok = ok && reload
								}
							case "j/k", "home/G", "^d/^u", "gg/0":
								if view == "graph" {
									ok = m.graph.SelectedIssue().ID == wantNavID
								} else if view == "board" {
									ok = m.board.SelectedIssue().ID == wantNavID
								} else {
									ok = m.TreeSelectedID() == wantNavID
								}
							case "enter":
								detail := m.detailIssue()
								ok = m.focused == focusDetail && detail != nil && detail.ID == wantID
							case "tab", "⇧tab":
								if view == "board" {
									ok = m.board.AnyEpicFolded()
								} else {
									ok = !m.tree.IsListView() && m.tree.NodeCount() < oldCount
								}
							case "g":
								ok = m.isGraphView && m.focused == focusGraph &&
									m.graph.SelectedIssue().ID == wantID
							case "g/esc":
								ok = !m.isGraphView && m.focused == focusTree
							case "s":
								switch view {
								case "tree", "flat tree", "type:task", "!type:epic", "F3 type query":
									ok = m.tree.IsSortPopupOpen()
								case "board":
									ok = m.board.GetSwimLaneMode() != oldSwim
								case "list":
									ok = m.isSplitView
								}
							case "c":
								ok = copied == wantID+" "+wantTitle && !m.statusIsError
							case "|":
								ok = m.tree.IsColumnPopupOpen()
							case "/":
								ok = m.queryState.Mode() == QueryEditing
							case "f":
								if view == "board" {
									ok = m.board.BranchRoot() == "ep-1"
								} else if wasListView {
									ok = m.queryState.Text() == BranchQuery(wantID)
								} else {
									ok = m.queryState.Text() == BranchQuery("ep-1")
								}
							case "e":
								ok = m.showEditModal && m.editModal.issueID == wantID
							case "K":
								ok = m.issueConfirm.action == issueConfirmClose && m.issueConfirm.id == wantID
							case "del":
								ok = m.issueConfirm.action == issueConfirmDelete && m.issueConfirm.id == wantID
							case "esc":
								if oldQuery != "" {
									ok = m.queryState.Text() == "" && !m.showQuitConfirm
								} else if view == "board" {
									ok = !m.isBoardView && m.focused == focusTree
								} else {
									ok = m.showQuitConfirm && m.focused == focusQuitConfirm
								}
							case "v":
								ok = m.board.EpicView() != oldEpic
							case "</>":
								delta := 0.05
								if key == "<" {
									delta = -delta
								}
								ok = m.splitPaneRatio == oldRatio+delta
							case "\\":
								ok = m.splitStacked != oldLayout
							case "t":
								ok = m.focused == focusTree
							case "b":
								ok = m.isBoardView && m.focused == focusBoard
							case "n", "^n":
								ok = m.showEditModal && m.editModal.issueID == ""
							case "?":
								ok = m.showHelp && m.focused == focusHelp
							case "L":
								ok = m.pickerMode == pickerModeLabels
							case "P":
								ok = m.pickerMode == pickerModeProjects && m.projectPicker.Filtering()
							case "F1-4":
								name, exists := typeForFunctionKey(key)
								wantQuery := "type:" + name
								if oldQuery == wantQuery {
									wantQuery = ""
								}
								ok = exists && m.queryState.Text() == wantQuery
							case "H":
								ok = m.pickerVisible != oldHeader
							default:
								t.Fatalf("footer hint %q needs a behavior assertion", hint)
							}
							if !ok {
								t.Errorf("footer hint %q key %q did not perform its advertised action (status %q)", hint, key, m.statusMsg)
							}
						})
					}
				}
			})
		}
	}
}
