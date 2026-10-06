package ui

import (
	"fmt"
	"os"
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
	"c":        {Type: tea.KeyRunes, Runes: []rune("c")},
	"O":        {Type: tea.KeyRunes, Runes: []rune("O")},
	"?":        {Type: tea.KeyRunes, Runes: []rune("?")},
	"esc":      {Type: tea.KeyEsc},
	"L":        {Type: tea.KeyRunes, Runes: []rune("L")},
	"P":        {Type: tea.KeyRunes, Runes: []rune("P")},
	"H":        {Type: tea.KeyRunes, Runes: []rune("H")},
	"Y":        {Type: tea.KeyRunes, Runes: []rune("Y")},
}

// effectOf is everything a key press may change that the user can see.
func effectOf(m Model) string {
	return fmt.Sprintf("%v|%v|%v|%v|%q|%d|%s|%s",
		m.focused, m.showEditModal, m.pickerMode, m.pickerVisible,
		m.statusMsg, m.viewport.YOffset, m.TreeSelectedID(), stripANSI(m.View()))
}

func detailFooterIssues() []model.Issue {
	now := time.Now()
	child := func(id, parent string, age int) model.Issue {
		return model.Issue{
			ID: id, Title: "Title " + id, Status: model.StatusOpen, IssueType: model.TypeTask,
			Priority: 2, CreatedAt: now.Add(time.Duration(age) * time.Second),
			Description: strings.Repeat("A line of description.\n\n", 60),
			Dependencies: []*model.Dependency{
				{IssueID: id, DependsOnID: parent, Type: model.DepParentChild},
			},
		}
	}
	return []model.Issue{
		{ID: "ep-1", Title: "Epic", Status: model.StatusOpen, IssueType: model.TypeEpic, Priority: 1,
			CreatedAt: now.Add(9 * time.Second), Description: strings.Repeat("Epic line.\n\n", 60)},
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

// O on the detail screen opens the beads file, like O in the list view. Under
// B9S_TEST_MODE it reports the file it would open instead of starting an editor.
// O opens the issue the detail shows, written to a temp Markdown file, so it
// works the same whatever the project is read from. The regression case is a
// Dolt-server project with no issues.jsonl and no local checkout.
func TestDetailOKeyOpensIssueWithoutBeadsFile(t *testing.T) {
	t.Setenv("B9S_TEST_MODE", "1")
	t.Setenv("TMPDIR", t.TempDir())
	for _, view := range []string{"tree", "board", "list"} {
		t.Run(view, func(t *testing.T) {
			m := detailFooterModel(t, view)
			m.beadsPath = ""
			m.activeProjectPath = ""
			want := m.list.SelectedItem().(IssueItem).Issue
			if shown := map[string]string{"tree": "ch-1", "board": "ch-1"}[view]; shown != "" && want.ID != shown {
				t.Fatalf("the detail shows %s but the selection is %s", shown, want.ID)
			}

			m = pressKeys(m, runeKey("O"))

			if m.statusIsError || !strings.Contains(m.statusMsg, "Would open "+want.ID) {
				t.Fatalf("expected O to open %s, status %q (error %v)", want.ID, m.statusMsg, m.statusIsError)
			}
			file := m.OpenedIssueFile()
			if !strings.HasSuffix(file, ".md") {
				t.Fatalf("expected a Markdown file, got %q", file)
			}
			body, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(body), want.Title) || !strings.Contains(string(body), want.ID) {
				t.Errorf("file does not hold %s:\n%s", want.ID, body)
			}
			if m.focused != focusDetail {
				t.Errorf("expected O to leave the detail focused, got %v", m.focused)
			}
		})
	}
}
