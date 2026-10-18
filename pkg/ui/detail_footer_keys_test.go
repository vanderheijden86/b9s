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
