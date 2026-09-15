package ui_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanderheijden86/beadwork/pkg/model"
	"github.com/vanderheijden86/beadwork/pkg/ui"
)

// Narrower than SplitViewThreshold, so the detail pane stays hidden and cannot
// show the selected title on the tree's behalf.
const layoutTestWidth = 90

func layoutTestIssues() ([]model.Issue, map[string]string) {
	now := time.Now()
	issues := []model.Issue{
		{ID: "epic-l", Title: "Layout Epic", Status: model.StatusOpen, Priority: 1, IssueType: model.TypeEpic, CreatedAt: now},
	}
	for i := 1; i <= 40; i++ {
		id := fmt.Sprintf("child-%02d", i)
		issues = append(issues, model.Issue{
			ID: id, Title: fmt.Sprintf("Layout Child %02d", i), Status: model.StatusOpen, Priority: 2,
			IssueType: model.TypeTask, CreatedAt: now.Add(time.Duration(i) * time.Second),
			Dependencies: []*model.Dependency{{IssueID: id, DependsOnID: "epic-l", Type: model.DepParentChild}},
		})
	}
	titles := make(map[string]string, len(issues))
	for _, issue := range issues {
		titles[issue.ID] = issue.Title
	}
	return issues, titles
}

func resizeModel(m ui.Model, width, height int) ui.Model {
	next, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	return next.(ui.Model)
}

func newLayoutTestModel(t *testing.T) (ui.Model, map[string]string) {
	t.Helper()
	issues, titles := layoutTestIssues()
	m := ui.NewModel(issues, "")
	m = resizeModel(m, layoutTestWidth, 40)
	m = enterTreeView(t, m)
	return m, titles
}

func assertTreeSelectionRendered(t *testing.T, m ui.Model, titles map[string]string, step string) {
	t.Helper()
	id := m.TreeSelectedID()
	title, ok := titles[id]
	if !ok {
		t.Fatalf("%s: unexpected selection %q", step, id)
	}
	if view := m.View(); !strings.Contains(view, title) {
		t.Fatalf("%s: selected %q is not rendered:\n%s", step, title, view)
	}
}

// The tree must scroll against the height it is drawn with, whatever layout
// change last happened, so the selected row never sits below the pane.
func TestTreeSelectionRenderedAfterLayoutChange(t *testing.T) {
	t.Run("jumping to the bottom after the terminal shrinks", func(t *testing.T) {
		m, titles := newLayoutTestModel(t)
		m = resizeModel(m, layoutTestWidth, 20)
		m = sendKey(t, m, "G")
		assertTreeSelectionRendered(t, m, titles, "G")
	})

	t.Run("moving down after the terminal shrinks", func(t *testing.T) {
		m, titles := newLayoutTestModel(t)
		m = resizeModel(m, layoutTestWidth, 20)
		for i := 1; i < len(titles); i++ {
			m = sendKey(t, m, "j")
			assertTreeSelectionRendered(t, m, titles, fmt.Sprintf("j %d", i))
		}
	})

	t.Run("shrinking the terminal with the bottom row selected", func(t *testing.T) {
		m, titles := newLayoutTestModel(t)
		m = sendKey(t, m, "G")
		m = resizeModel(m, layoutTestWidth, 20)
		assertTreeSelectionRendered(t, m, titles, "resize")
	})
}
