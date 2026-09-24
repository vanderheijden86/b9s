package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/vanderheijden86/beadwork/pkg/config"
	"github.com/vanderheijden86/beadwork/pkg/model"
)

func epicChildDeps(id, parent string) []*model.Dependency {
	return []*model.Dependency{{IssueID: id, DependsOnID: parent, Type: model.DepParentChild}}
}

// epicBoardIssues holds two epics and one issue outside any epic. eg0.2.1 is a
// grandchild through a task, so its epic is the nearest epic ancestor.
func epicBoardIssues() []model.Issue {
	now := time.Now()
	at := now.Add(-24 * time.Hour)
	closed := now.Add(-time.Hour)
	mk := func(id, title string, typ model.IssueType, status model.Status, prio int, deps []*model.Dependency) model.Issue {
		return model.Issue{ID: id, Title: title, IssueType: typ, Status: status, Priority: prio,
			Dependencies: deps, CreatedAt: at, UpdatedAt: at}
	}
	done := mk("spectroscope-eg0.3", "Pick the capture format", model.TypeTask, model.StatusClosed, 2, epicChildDeps("spectroscope-eg0.3", "spectroscope-eg0"))
	done.ClosedAt = &closed
	return []model.Issue{
		mk("spectroscope-eg0", "Stream capture pipeline", model.TypeEpic, model.StatusOpen, 1, nil),
		mk("spectroscope-eg0.1", "Parse capture headers", model.TypeTask, model.StatusOpen, 2, epicChildDeps("spectroscope-eg0.1", "spectroscope-eg0")),
		mk("spectroscope-eg0.2", "Wire the subscription transport", model.TypeTask, model.StatusInProgress, 1, epicChildDeps("spectroscope-eg0.2", "spectroscope-eg0")),
		mk("spectroscope-eg0.2.1", "Backpressure for slow readers", model.TypeTask, model.StatusOpen, 2, epicChildDeps("spectroscope-eg0.2.1", "spectroscope-eg0.2")),
		done,
		mk("spectroscope-k3s", "Local cluster fixtures", model.TypeEpic, model.StatusInProgress, 2, nil),
		mk("spectroscope-k3s.1", "Record a mac-k3s capture", model.TypeTask, model.StatusOpen, 0, epicChildDeps("spectroscope-k3s.1", "spectroscope-k3s")),
		mk("spectroscope-x1", "Tidy the README", model.TypeChore, model.StatusOpen, 3, nil),
	}
}

func newEpicBoard(view BoardEpicView) BoardModel {
	b := NewBoardModel(epicBoardIssues(), DefaultTheme(lipgloss.DefaultRenderer()))
	b.SetActiveProjectName("spectroscope")
	b.SetEpicView(view)
	return b
}

func columnIDs(b BoardModel, col int) []string {
	var ids []string
	for _, is := range b.columns[col] {
		ids = append(ids, strings.TrimPrefix(is.ID, "spectroscope-"))
	}
	return ids
}

func TestBoardEpics_NearestEpicAncestorAndCompletion(t *testing.T) {
	b := newEpicBoard(BoardEpicLanes)
	for id, want := range map[string]string{
		"spectroscope-eg0.2.1": "spectroscope-eg0",
		"spectroscope-eg0":     "spectroscope-eg0",
		"spectroscope-k3s.1":   "spectroscope-k3s",
		"spectroscope-x1":      "",
	} {
		if got := b.epicFor(id); got != want {
			t.Errorf("epicFor(%s) = %q, want %q", id, got, want)
		}
	}
	info := b.epics["spectroscope-eg0"]
	if info == nil || info.Done != 1 || info.Total != 4 {
		t.Fatalf("eg0 completion must be 1/4 (three open children and one closed), got %+v", info)
	}
}

func TestBoardEpics_CompletionCountsChildrenOutsideTheFilter(t *testing.T) {
	var open []model.Issue
	for _, is := range epicBoardIssues() {
		if !isClosedLikeStatus(is.Status) {
			open = append(open, is)
		}
	}
	b := NewBoardModel(open, DefaultTheme(lipgloss.DefaultRenderer()))
	b.SetEpicUniverse(epicBoardIssues())
	if info := b.epics["spectroscope-eg0"]; info == nil || info.Done != 1 || info.Total != 4 {
		t.Fatalf("a filter must not hide closed children from the completion, got %+v", info)
	}
}

func TestBoardEpicView_CycleAndParse(t *testing.T) {
	b := newEpicBoard(BoardEpicLanes)
	for _, want := range []BoardEpicView{BoardEpicChips, BoardEpicGroups, BoardEpicLanes} {
		b.CycleEpicView()
		if b.EpicView() != want {
			t.Fatalf("cycle reached %s, want %s", b.EpicView(), want)
		}
	}
	for in, want := range map[string]BoardEpicView{"lanes": BoardEpicLanes, "chips": BoardEpicChips, "Groups": BoardEpicGroups, "2": BoardEpicChips} {
		if got, ok := ParseBoardEpicView(in); !ok || got != want {
			t.Errorf("ParseBoardEpicView(%q) = %s, %v", in, got, ok)
		}
	}
	if _, ok := ParseBoardEpicView("inspector"); ok {
		t.Error("layout E is gone and must not parse")
	}
}

func TestBoardEpicLanes_OrderEachColumnByEpicBand(t *testing.T) {
	b := newEpicBoard(BoardEpicLanes)
	// k3s holds the only P0, so its band comes first; issues without an epic come last.
	got := strings.Join(columnIDs(b, ColOpen), " ")
	want := "k3s.1 eg0 eg0.1 eg0.2.1 x1"
	if got != want {
		t.Fatalf("open column order = %q, want %q", got, want)
	}
}

func TestBoardEpicLanes_ViewShowsBandsWithCompletion(t *testing.T) {
	b := newEpicBoard(BoardEpicLanes)
	view := stripANSI(b.View(200, 40))
	for _, want := range []string{"Stream capture pipeline", "1/4", "Local cluster fixtures", "0/1", "No epic", "Epic lanes"} {
		if !strings.Contains(view, want) {
			t.Fatalf("lanes view misses %q:\n%s", want, view)
		}
	}
	if strings.Index(view, "Local cluster fixtures") > strings.Index(view, "Stream capture pipeline") {
		t.Fatalf("the P0 epic band must come first:\n%s", view)
	}
}

func TestBoardEpicLanes_NoBandsWithoutEpics(t *testing.T) {
	b := newSparseBoard()
	if view := stripANSI(b.View(200, 30)); strings.Contains(view, "No epic") {
		t.Fatalf("a board without epics must not draw a lone No epic band:\n%s", view)
	}
}

func TestBoardEpicLanes_TabFoldsTheSelectedEpic(t *testing.T) {
	b := newEpicBoard(BoardEpicLanes)
	b.SelectIssueByID("spectroscope-eg0.1")
	b.ToggleEpicFold()
	if ids := strings.Join(columnIDs(b, ColOpen), " "); strings.Contains(ids, "eg0.1") || !strings.Contains(ids, "eg0") {
		t.Fatalf("folding hides the children and keeps the epic row, got %q", ids)
	}
	if sel := b.SelectedIssue(); sel == nil || sel.ID != "spectroscope-eg0" {
		t.Fatalf("folding moves the selection to the epic, got %+v", sel)
	}
	if view := stripANSI(b.View(200, 40)); !strings.Contains(view, "▸ ◆ eg0") {
		t.Fatalf("a folded band shows a closed marker:\n%s", view)
	}
	b.ToggleEpicFold()
	if ids := strings.Join(columnIDs(b, ColOpen), " "); !strings.Contains(ids, "eg0.1") {
		t.Fatalf("a second fold expands the band again, got %q", ids)
	}
}

func TestBoardEpicLanes_FoldAllAndExpandAll(t *testing.T) {
	b := newEpicBoard(BoardEpicLanes)
	b.ToggleAllEpicFolds()
	if got := strings.Join(columnIDs(b, ColOpen), " "); got != "eg0 x1" {
		t.Fatalf("fold all leaves epics and issues without an epic, got %q", got)
	}
	b.ToggleAllEpicFolds()
	if got := len(b.columns[ColOpen]); got != 5 {
		t.Fatalf("expand all restores every open issue, got %d", got)
	}
}

func TestBoardEpicChips_RowCarriesEpicTag(t *testing.T) {
	b := newEpicBoard(BoardEpicChips)
	view := stripANSI(b.View(200, 40))
	for _, want := range []string{"◆ eg0 Stream capture", "◆ k3s Local cluster", "Epic chips"} {
		if !strings.Contains(view, want) {
			t.Fatalf("chips view misses %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "No epic") {
		t.Fatalf("chips draw no bands:\n%s", view)
	}
}

func TestBoardEpicGroups_SubheaderPrecedesItsRows(t *testing.T) {
	b := newEpicBoard(BoardEpicGroups)
	view := stripANSI(b.View(200, 40))
	head := strings.Index(view, "◆ eg0 Stream capture pipeline")
	row := strings.Index(view, "[eg0.1]")
	if head < 0 || row < 0 || head > row {
		t.Fatalf("group subheader must come before its rows (head %d, row %d):\n%s", head, row, view)
	}
	if !strings.Contains(view, "Epic groups") {
		t.Fatalf("board bar must name the design:\n%s", view)
	}
}

func TestBoardEpicViews_FitEveryWidth(t *testing.T) {
	for _, view := range []BoardEpicView{BoardEpicLanes, BoardEpicChips, BoardEpicGroups} {
		for _, width := range []int{60, 80, 110, 160, 220} {
			for _, height := range []int{8, 30} {
				b := newEpicBoard(view)
				b.SelectIssueByID("spectroscope-eg0.2.1")
				lines := strings.Split(b.View(width, height), "\n")
				if len(lines) != height {
					t.Fatalf("%s %dx%d: %d lines", view, width, height, len(lines))
				}
				for i, line := range lines {
					if w := lipgloss.Width(line); w > width {
						t.Fatalf("%s %dx%d: line %d is %d wide", view, width, height, i, w)
					}
				}
			}
		}
	}
}

func TestBoardEpicLanes_SelectionStaysVisibleWhenScrolled(t *testing.T) {
	b := newEpicBoard(BoardEpicLanes)
	b.SelectIssueByID("spectroscope-x1")
	view := stripANSI(b.View(200, 12))
	if !strings.Contains(view, "[x1]") {
		t.Fatalf("the selected row must be in view:\n%s", view)
	}
}

func epicKeyModel(t *testing.T) Model {
	t.Helper()
	m := NewModel(epicBoardIssues(), "")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 40})
	m = pressBoard(t, updated.(Model), runeKey("b"))
	m.board.SelectIssueByID("spectroscope-eg0.1")
	return m
}

func TestBoardKeyV_CyclesEpicDesigns(t *testing.T) {
	m := epicKeyModel(t)
	m = pressBoard(t, m, runeKey("v"))
	if m.board.EpicView() != BoardEpicChips || !strings.Contains(m.statusMsg, "Epic chips") {
		t.Fatalf("v must switch to the chips design, got %s %q", m.board.EpicView(), m.statusMsg)
	}
	if sel := m.board.SelectedIssue(); sel == nil || sel.ID != "spectroscope-eg0.1" {
		t.Fatalf("switching design keeps the selection, got %+v", sel)
	}
}

func TestBoardKeyTab_FoldsEpicInLanes(t *testing.T) {
	m := epicKeyModel(t)
	m = pressBoard(t, m, tea.KeyMsg{Type: tea.KeyTab})
	if sel := m.board.SelectedIssue(); sel == nil || sel.ID != "spectroscope-eg0" {
		t.Fatalf("tab folds the epic and selects it, got %+v", sel)
	}
	m = pressBoard(t, m, tea.KeyMsg{Type: tea.KeyShiftTab})
	if !m.board.AnyEpicFolded() {
		// eg0 was folded, so shift+tab expands everything.
		return
	}
	t.Fatal("shift+tab with a folded epic must expand all")
}

func TestBoardEpicConfig_SetsStartupDesign(t *testing.T) {
	cfg := config.Config{}
	cfg.UI.BoardEpics = "groups"
	m := NewModel(epicBoardIssues(), "").WithConfig(cfg, "spectroscope", "")
	if m.board.EpicView() != BoardEpicGroups {
		t.Fatalf("ui.board_epics groups must start the groups design, got %s", m.board.EpicView())
	}
}

func TestBoardEpicLanes_BarCountsFoldedIssues(t *testing.T) {
	b := newEpicBoard(BoardEpicLanes)
	b.ToggleAllEpicFolds()
	view := stripANSI(b.View(200, 30))
	if !strings.Contains(view, "8 issues") || !strings.Contains(view, "5 folded") {
		t.Fatalf("the bar must count every issue and name the folded ones:\n%s", view)
	}
}
