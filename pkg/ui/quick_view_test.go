package ui

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanderheijden86/b9s/internal/datasource"
	"github.com/vanderheijden86/b9s/pkg/model"
)

func quickViewModel(t *testing.T) Model {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := NewModel([]model.Issue{
		{ID: "q-origin", Title: "Original context", Status: model.StatusOpen, IssueType: model.TypeTask,
			Description:  strings.Repeat("Original paragraph.\n\n", 40) + "See q-child.2.",
			Dependencies: []*model.Dependency{{DependsOnID: "q-child.1", Type: model.DepRelated}}},
		{ID: "q-parent", Title: "Parent", Status: model.StatusOpen, IssueType: model.TypeEpic},
		{ID: "q-child.1", Title: "First preview", Status: model.StatusClosed, IssueType: model.TypeTask,
			Description: "FIRST BODY", Dependencies: []*model.Dependency{{DependsOnID: "q-parent", Type: model.DepParentChild}}},
		{ID: "q-child.2", Title: "Second preview", Status: model.StatusOpen, IssueType: model.TypeTask,
			Description: "SECOND BODY", Dependencies: []*model.Dependency{{DependsOnID: "q-parent", Type: model.DepParentChild}}},
	}, "")
	m = pressKeys(m, runeKey("a"))
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(Model)
	m.tree.ExpandPathTo("q-origin")
	m.tree.SelectByID("q-origin")
	m.syncTreeToDetail()
	return m
}

func chooseQuickView(m Model, id string) Model {
	return pressKeys(m, tea.KeyMsg{Type: tea.KeyCtrlO}, runeKey(id), tea.KeyMsg{Type: tea.KeyEnter})
}

func TestQuickViewDetailRestoresContext(t *testing.T) {
	m := quickViewModel(t)
	m.focused = focusDetail
	m.updateViewportContent()
	m.viewport.SetYOffset(8)
	m.detailBack = []string{"q-parent"}
	before, scroll, selected, query := stripANSI(m.View()), m.viewport.YOffset, m.TreeSelectedID(), m.queryState
	history := append([]string(nil), m.detailBack...)

	m = chooseQuickView(m, "q-child.1")
	if view := stripANSI(m.View()); !strings.Contains(view, "Quick view") || !strings.Contains(view, "FIRST BODY") {
		t.Fatalf("related issue must open in a preview:\n%s", view)
	}
	m = pressKeys(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.focused != focusDetail || m.viewport.YOffset != scroll || m.TreeSelectedID() != selected ||
		!reflect.DeepEqual(m.detailBack, history) || !reflect.DeepEqual(m.queryState, query) || stripANSI(m.View()) != before {
		t.Fatalf("Esc lost original detail context: focus=%v scroll=%d selected=%s", m.focused, m.viewport.YOffset, m.TreeSelectedID())
	}
}

func TestQuickViewListPreviewsSelectedRow(t *testing.T) {
	for _, view := range []string{"tree", "flat-list", "board"} {
		t.Run(view, func(t *testing.T) {
			m := quickViewModel(t)
			switch view {
			case "flat-list":
				m = pressKeys(m, runeKey("t"))
			case "board":
				m = pressKeys(m, runeKey("b"))
				m.board.SelectIssueByID("q-origin")
			}
			before, focus := stripANSI(m.View()), m.focused
			m = pressKeys(m, tea.KeyMsg{Type: tea.KeyCtrlO})
			if rendered := stripANSI(m.View()); !strings.Contains(rendered, "Quick view") || !strings.Contains(rendered, "Original context") {
				t.Fatalf("Ctrl-O must preview the cursor row:\n%s", rendered)
			}
			m = pressKeys(m, tea.KeyMsg{Type: tea.KeyEsc})
			if m.focused != focus || stripANSI(m.View()) != before {
				t.Fatal("Esc must restore the list and cursor")
			}
		})
	}
}

func TestQuickViewResizeRestoresOriginalScroll(t *testing.T) {
	m := quickViewModel(t)
	m.focused = focusDetail
	m.updateViewportContent()
	m.viewport.SetYOffset(8)
	scroll := m.viewport.YOffset
	m = chooseQuickView(m, "q-child.1")
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = pressKeys(next.(Model), tea.KeyMsg{Type: tea.KeyEsc})
	if m.focused != focusDetail || m.viewport.YOffset != scroll || m.detailIssue().ID != "q-origin" {
		t.Fatalf("resize lost original detail context: focus=%v scroll=%d, want %d", m.focused, m.viewport.YOffset, scroll)
	}
}

func TestQuickViewRestoresOriginalActionTarget(t *testing.T) {
	m := quickViewModel(t)
	m.focused = focusDetail
	m = chooseQuickView(m, "q-child.1")
	m = pressKeys(m, tea.KeyMsg{Type: tea.KeyEsc}, runeKey("e"))
	if !m.showEditModal || m.editModal.issueID != "q-origin" {
		t.Fatal("edit after Esc must target the original issue")
	}
}

func TestQuickViewDoesNotInterruptQueryEntry(t *testing.T) {
	m := quickViewModel(t)
	m = pressKeys(m, runeKey("/"), tea.KeyMsg{Type: tea.KeyCtrlO})
	if strings.Contains(stripANSI(m.View()), "Quick view") || m.queryState.Mode() != QueryEditing {
		t.Fatal("Ctrl-O must stay inside the active query input")
	}
}

func TestQuickViewCyclesSiblingsOutsideFilter(t *testing.T) {
	m := quickViewModel(t)
	m.setQueryText("id:q-origin")
	m.focused = focusDetail
	m.updateViewportContent()
	m = chooseQuickView(m, "q-child.1")
	for _, step := range []struct{ key, body string }{
		{"n", "SECOND BODY"}, {"n", "FIRST BODY"}, {"p", "SECOND BODY"},
	} {
		m = pressKeys(m, runeKey(step.key))
		if view := stripANSI(m.View()); !strings.Contains(view, step.body) || !strings.Contains(view, "Quick view") {
			t.Fatalf("%s must cycle siblings outside the original filter:\n%s", step.key, view)
		}
		if m.TreeSelectedID() != "q-origin" {
			t.Fatal("preview changed the original cursor")
		}
	}
}

func TestQuickViewOwnsAllContextualKeysAndMouse(t *testing.T) {
	for _, key := range []tea.KeyMsg{
		runeKey("e"), runeKey("S"), runeKey("K"), runeKey("C"), runeKey("c"), runeKey("y"),
		runeKey("I"), runeKey("R"), runeKey("1"), runeKey("s"), runeKey("b"), runeKey("M"),
		runeKey("/"), runeKey(":"), runeKey("?"), runeKey("q"),
		{Type: tea.KeyDelete}, {Type: tea.KeyCtrlN}, {Type: tea.KeyCtrlO}, {Type: tea.KeyCtrlR},
		{Type: tea.KeyEnter}, {Type: tea.KeyTab}, {Type: tea.KeyBackspace},
	} {
		t.Run(key.String(), func(t *testing.T) {
			m := quickViewModel(t)
			m.focused = focusDetail
			m = chooseQuickView(m, "q-child.1")
			before := stripANSI(m.View())
			next, cmd := m.Update(key)
			m = next.(Model)
			if cmd != nil {
				t.Fatal("read-only preview issued a contextual command")
			}
			next, _ = m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown})
			m = next.(Model)
			if !strings.Contains(stripANSI(m.View()), "Quick view") || m.TreeSelectedID() != "q-origin" ||
				m.showEditModal || m.showStatusPicker || m.issueConfirm.action != issueConfirmNone || m.commentsOldestFirst {
				t.Fatalf("contextual key escaped preview: %s", key.String())
			}
			// This short preview cannot scroll, so a wheel event changes no content.
			if stripANSI(m.View()) != before {
				t.Fatalf("key %s changed the preview:\n%s", key.String(), m.View())
			}
		})
	}
}

func TestQuickViewChooserOffersRelationsAndMentions(t *testing.T) {
	m := quickViewModel(t)
	m.focused = focusDetail
	m = pressKeys(m, tea.KeyMsg{Type: tea.KeyCtrlO})
	view := stripANSI(m.View())
	for _, text := range []string{"Preview issue", "q-child.1", "q-child.2"} {
		if !strings.Contains(view, text) {
			t.Fatalf("chooser is missing %q:\n%s", text, view)
		}
	}
	m = pressKeys(m, runeKey("no-such-issue"), tea.KeyMsg{Type: tea.KeyEnter})
	if !strings.Contains(stripANSI(m.View()), "No matching issues") {
		t.Fatal("unknown ID must leave an empty chooser")
	}
	m = pressKeys(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.TreeSelectedID() != "q-origin" || m.focused != focusDetail {
		t.Fatal("cancelled chooser lost original context")
	}
}

func TestQuickViewLoadsAbridgedText(t *testing.T) {
	for _, loadErr := range []error{nil, errors.New("offline")} {
		t.Run(fmtErrorName(loadErr), func(t *testing.T) {
			m := quickViewModel(t)
			m.focused = focusDetail
			m.issueMap["q-child.1"].Abridged = true
			m.textLoader = func(_ doltReloadRequest, id string) (datasource.IssueText, error) {
				return datasource.IssueText{Description: "FETCHED PREVIEW BODY"}, loadErr
			}
			m = pressKeys(m, tea.KeyMsg{Type: tea.KeyCtrlO}, runeKey("q-child.1"))
			next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			m = next.(Model)
			if !strings.Contains(stripANSI(m.View()), "Loading the description") {
				t.Fatal("abridged preview must show loading text")
			}
			loaded := fetchedTextMsg(t, cmd)
			next, _ = m.Update(loaded)
			m = next.(Model)
			want := "FETCHED PREVIEW BODY"
			if loadErr != nil {
				want = "offline"
			}
			if !strings.Contains(stripANSI(m.View()), want) {
				t.Fatalf("preview did not show loaded text/error %q:\n%s", want, m.View())
			}
			m = pressKeys(m, tea.KeyMsg{Type: tea.KeyEsc})
			if strings.Contains(m.statusMsg, "q-child.1") || m.detailIssue().ID != "q-origin" {
				t.Fatal("preview text loading leaked status or selection into the original detail")
			}
		})
	}
}

func fmtErrorName(err error) string {
	if err == nil {
		return "success"
	}
	return "failure"
}
