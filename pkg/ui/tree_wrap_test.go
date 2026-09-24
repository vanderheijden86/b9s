package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanderheijden86/beadwork/pkg/model"
)

const wrapTestTitle = "MegaToby cannot read photos and PDFs because media paths are rejected as outside the allowed roots"

func wrapTestTree(t *testing.T, count, width, height int) *TreeModel {
	t.Helper()
	now := time.Now()
	issues := make([]model.Issue, count)
	for i := range issues {
		issues[i] = model.Issue{
			ID:        fmt.Sprintf("wrap-%02d", i),
			Title:     fmt.Sprintf("%02d %s", i, wrapTestTitle),
			Status:    model.StatusOpen,
			IssueType: model.TypeTask,
			CreatedAt: now.Add(-time.Duration(count-i) * time.Minute),
			UpdatedAt: now,
		}
	}
	tree := NewTreeModel(testTheme())
	tree.SetSize(width, height)
	tree.Build(issues)
	return &tree
}

func TestTreeTitlesTruncateUntilWrapIsOn(t *testing.T) {
	tree := wrapTestTree(t, 1, 70, 20)

	if tree.WrapTitles() {
		t.Fatal("wrapping must start off")
	}
	if view := stripANSI(tree.View()); !strings.Contains(view, "…") || strings.Contains(view, "allowed roots") {
		t.Fatalf("unwrapped title should be truncated, got:\n%s", view)
	}

	tree.ToggleWrapTitles()

	view := stripANSI(tree.View())
	if strings.Contains(view, "…") {
		t.Fatalf("wrapped title must not be truncated, got:\n%s", view)
	}
	for _, word := range strings.Fields(wrapTestTitle) {
		if !strings.Contains(view, word) {
			t.Fatalf("wrapped view is missing %q, got:\n%s", word, view)
		}
	}
}

func TestTreeWrappedTitleContinuesUnderTitleColumn(t *testing.T) {
	tree := wrapTestTree(t, 1, 70, 20)
	tree.ToggleWrapTitles()

	lines := strings.Split(stripANSI(tree.View()), "\n")
	// lines[0] is the header, lines[1] the first row of the only node.
	byteCol := strings.Index(lines[1], "00 MegaToby")
	titleCol := len([]rune(lines[1][:max(byteCol, 0)]))
	if byteCol < 0 {
		t.Fatalf("first row has no title start: %q", lines[1])
	}
	cont := lines[2]
	if strings.TrimSpace(cont) == "" {
		t.Fatalf("expected a continuation line, got %q", cont)
	}
	if got := len(cont) - len(strings.TrimLeft(cont, " ")); got != titleCol {
		t.Fatalf("continuation starts at column %d, want title column %d:\n%s\n%s", got, titleCol, lines[1], cont)
	}
	for _, line := range lines {
		if w := len([]rune(line)); w > 70 {
			t.Fatalf("line wider than the pane (%d > 70): %q", w, line)
		}
	}
}

func TestTreeWrappedRowsFitHeightAndKeepCursorVisible(t *testing.T) {
	const height = 12
	tree := wrapTestTree(t, 30, 70, height)
	tree.ToggleWrapTitles()

	for i := 0; i < 29; i++ {
		tree.MoveDown()
		view := stripANSI(tree.View())
		lines := strings.Split(strings.TrimRight(view, "\n"), "\n")
		if len(lines) > height {
			t.Fatalf("cursor %d: view has %d lines, pane is %d:\n%s", tree.cursor, len(lines), height, view)
		}
		want := tree.SelectedNode().Issue.Title[:len("00 MegaToby")]
		if !strings.Contains(view, want) {
			t.Fatalf("cursor %d: selected title %q not on screen:\n%s", tree.cursor, want, view)
		}
	}
}

func TestWrapCommandResolves(t *testing.T) {
	command, err := ResolveCommand("wrap")
	if err != nil || command.Kind != CommandWrap {
		t.Fatalf("ResolveCommand(wrap) = %+v, %v; want CommandWrap", command, err)
	}
}

func TestTreeKeyVTogglesTitleWrap(t *testing.T) {
	m := NewModel([]model.Issue{{ID: "wrap-1", Title: wrapTestTitle, Status: model.StatusOpen, IssueType: model.TypeTask}}, "")
	v := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("v")}

	next, _ := m.Update(v)
	m = next.(Model)
	if !m.tree.WrapTitles() {
		t.Fatalf("v should turn title wrapping on (status %q)", m.statusMsg)
	}
	next, _ = m.Update(v)
	if next.(Model).tree.WrapTitles() {
		t.Fatal("a second v should turn title wrapping off")
	}
}
