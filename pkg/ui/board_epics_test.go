package ui

import (
	"io"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

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
	b := newEpicBoard(BoardEpicRail)
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
	b := newEpicBoard(BoardEpicRail)
	for _, want := range []BoardEpicView{BoardEpicRows, BoardEpicRail} {
		b.CycleEpicView()
		if b.EpicView() != want {
			t.Fatalf("cycle reached %s, want %s", b.EpicView(), want)
		}
	}
	for in, want := range map[string]BoardEpicView{"rail": BoardEpicRail, "Rows": BoardEpicRows, "1": BoardEpicRail, "2": BoardEpicRows} {
		if got, ok := ParseBoardEpicView(in); !ok || got != want {
			t.Errorf("ParseBoardEpicView(%q) = %s, %v", in, got, ok)
		}
	}
	for _, gone := range []string{"lanes", "chips", "groups", "inspector"} {
		if got, ok := ParseBoardEpicView(gone); ok || got != BoardEpicRail {
			t.Errorf("%q is no longer a design and must fall back to the rail, got %s, %v", gone, got, ok)
		}
	}
}

func TestBoardEpics_OrderEachColumnByEpicLane(t *testing.T) {
	for _, view := range []BoardEpicView{BoardEpicRail, BoardEpicRows} {
		b := newEpicBoard(view)
		// k3s holds the only P0, so its lane comes first; issues without an epic come last.
		got := strings.Join(columnIDs(b, ColOpen), " ")
		want := "k3s.1 eg0 eg0.1 eg0.2.1 x1"
		if got != want {
			t.Fatalf("%s: open column order = %q, want %q", view, got, want)
		}
	}
}

func TestBoardEpicRail_EpicSitsInALeftRail(t *testing.T) {
	b := newEpicBoard(BoardEpicRail)
	view := stripANSI(b.View(200, 40))
	for _, want := range []string{"EPIC", "▾ ◆ eg0", "Stream capture", "1/4", "▾ ◆ k3s", "0/1", "No epic", "Epic rail 1/2", "issues"} {
		if !strings.Contains(view, want) {
			t.Fatalf("rail view misses %q:\n%s", want, view)
		}
	}
	if strings.Index(view, "Local cluster fixtures") > strings.Index(view, "Stream capture") {
		t.Fatalf("the P0 epic lane must come first:\n%s", view)
	}
	// The rail holds the epic, so its title starts each lane at the left edge.
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "◆ eg0") && !strings.HasPrefix(strings.TrimLeft(line, " ┃"), "▾ ◆ eg0") {
			t.Fatalf("the epic must sit in the left rail, got %q", line)
		}
	}
}

func TestBoardEpicRail_CardsAreBoxes(t *testing.T) {
	b := newEpicBoard(BoardEpicRail)
	view := stripANSI(b.View(200, 40))
	for _, want := range []string{"╭", "╰", "Parse capture headers", "eg0.1", "P2"} {
		if !strings.Contains(view, want) {
			t.Fatalf("rail view misses %q:\n%s", want, view)
		}
	}
}

func TestBoardEpicRows_HeaderRowAboveTheCards(t *testing.T) {
	b := newEpicBoard(BoardEpicRows)
	view := stripANSI(b.View(200, 40))
	head := strings.Index(view, "▾ ◆ eg0 Stream capture pipeline")
	card := strings.Index(view, "Parse capture headers")
	if head < 0 || card < 0 || head > card {
		t.Fatalf("the epic row must come before its cards (head %d, card %d):\n%s", head, card, view)
	}
	for _, want := range []string{"Epic rows 2/2", "issues", "1/4", "╭"} {
		if !strings.Contains(view, want) {
			t.Fatalf("rows view misses %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "EPIC") {
		t.Fatalf("the rows design has no rail column:\n%s", view)
	}
}

func TestBoardEpics_NoLanesWithoutEpics(t *testing.T) {
	for _, view := range []BoardEpicView{BoardEpicRail, BoardEpicRows} {
		b := newSparseBoard()
		b.SetEpicView(view)
		if out := stripANSI(b.View(200, 30)); strings.Contains(out, "No epic") || strings.Contains(out, "EPIC") {
			t.Fatalf("%s: a board without epics must not draw lanes:\n%s", view, out)
		}
	}
}

func TestBoardEpics_TabFoldsTheSelectedEpicInBothDesigns(t *testing.T) {
	for _, view := range []BoardEpicView{BoardEpicRail, BoardEpicRows} {
		b := newEpicBoard(view)
		b.SelectIssueByID("spectroscope-eg0.1")
		if !b.ToggleEpicFold() {
			t.Fatalf("%s: tab must fold the epic", view)
		}
		if ids := strings.Join(columnIDs(b, ColOpen), " "); strings.Contains(ids, "eg0.1") || !strings.Contains(ids, "eg0") {
			t.Fatalf("%s: folding hides the children and keeps the epic, got %q", view, ids)
		}
		if sel := b.SelectedIssue(); sel == nil || sel.ID != "spectroscope-eg0" {
			t.Fatalf("%s: folding moves the selection to the epic, got %+v", view, sel)
		}
		out := stripANSI(b.View(200, 40))
		if !strings.Contains(out, "▸ ◆ eg0") {
			t.Fatalf("%s: a folded lane shows a closed marker:\n%s", view, out)
		}
		if strings.Contains(out, "Parse capture headers") {
			t.Fatalf("%s: a folded lane hides its cards:\n%s", view, out)
		}
		b.ToggleEpicFold()
		if ids := strings.Join(columnIDs(b, ColOpen), " "); !strings.Contains(ids, "eg0.1") {
			t.Fatalf("%s: a second fold expands the lane again, got %q", view, ids)
		}
	}
}

func TestBoardEpicRail_FoldedLaneCountsItsHiddenCards(t *testing.T) {
	b := newEpicBoard(BoardEpicRail)
	b.SelectIssueByID("spectroscope-eg0.1")
	b.ToggleEpicFold()
	if out := stripANSI(b.View(200, 40)); !strings.Contains(out, "2 hidden") {
		t.Fatalf("the folded lane must count the open cards it hides:\n%s", out)
	}
}

func TestBoardEpics_FoldAllAndExpandAll(t *testing.T) {
	b := newEpicBoard(BoardEpicRows)
	b.ToggleAllEpicFolds()
	if got := strings.Join(columnIDs(b, ColOpen), " "); got != "eg0 x1" {
		t.Fatalf("fold all leaves epics and issues without an epic, got %q", got)
	}
	b.ToggleAllEpicFolds()
	if got := len(b.columns[ColOpen]); got != 5 {
		t.Fatalf("expand all restores every open issue, got %d", got)
	}
}

func TestBoardEpics_FoldNeedsTheEpicOnTheBoard(t *testing.T) {
	var noEpicRow []model.Issue
	for _, is := range epicBoardIssues() {
		if is.ID != "spectroscope-eg0" {
			noEpicRow = append(noEpicRow, is)
		}
	}
	b := NewBoardModel(noEpicRow, DefaultTheme(lipgloss.DefaultRenderer()))
	b.SetEpicUniverse(epicBoardIssues())
	b.SelectIssueByID("spectroscope-eg0.1")
	if b.ToggleEpicFold() {
		t.Fatal("a lane whose epic is filtered out has nothing left to select once folded")
	}
	if ids := strings.Join(columnIDs(b, ColOpen), " "); !strings.Contains(ids, "eg0.1") {
		t.Fatalf("the refused fold must keep the cards, got %q", ids)
	}
}

func TestBoardEpics_SelectingTheEpicHighlightsItsLane(t *testing.T) {
	renderer := lipgloss.NewRenderer(io.Discard)
	renderer.SetColorProfile(termenv.TrueColor)
	theme := DefaultTheme(renderer)
	bg := bgSeqFromColor(theme.Highlight, renderer)
	for _, view := range []BoardEpicView{BoardEpicRail, BoardEpicRows} {
		b := NewBoardModel(epicBoardIssues(), theme)
		b.SetActiveProjectName("spectroscope")
		b.SetEpicView(view)
		b.SelectIssueByID("spectroscope-eg0")
		found := false
		for _, line := range strings.Split(b.View(200, 40), "\n") {
			if strings.Contains(stripANSI(line), "◆ eg0") && strings.Contains(line, bg) {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s: the selected epic must highlight its lane header", view)
		}
	}
}

func TestBoardEpicViews_FitEveryWidth(t *testing.T) {
	for _, view := range []BoardEpicView{BoardEpicRail, BoardEpicRows} {
		for _, width := range []int{40, 60, 80, 110, 160, 220} {
			for _, height := range []int{6, 8, 30} {
				for _, closed := range []bool{false, true} {
					b := newEpicBoard(view)
					if closed {
						b.ToggleClosedColumn()
					}
					b.SelectIssueByID("spectroscope-eg0.2.1")
					lines := strings.Split(b.View(width, height), "\n")
					if len(lines) != height {
						t.Fatalf("%s %dx%d: %d lines", view, width, height, len(lines))
					}
					for i, line := range lines {
						if w := lipgloss.Width(line); w != width {
							t.Fatalf("%s %dx%d: line %d is %d wide: %q", view, width, height, i, w, stripANSI(line))
						}
					}
				}
			}
		}
	}
}

func TestBoardEpics_SelectionStaysVisibleWhenScrolled(t *testing.T) {
	for _, view := range []BoardEpicView{BoardEpicRail, BoardEpicRows} {
		for _, id := range []string{"spectroscope-x1", "spectroscope-eg0.2.1", "spectroscope-k3s.1"} {
			b := newEpicBoard(view)
			b.SelectIssueByID(id)
			sel := b.SelectedIssue()
			out := stripANSI(b.View(120, 12))
			if !strings.Contains(out, sel.Title[:12]) {
				t.Fatalf("%s: the selected card %s must be in view:\n%s", view, id, out)
			}
		}
	}
}

func TestBoardClosedColumn_HiddenUntilToggled(t *testing.T) {
	b := newEpicBoard(BoardEpicRail)
	for _, col := range b.activeColIdx {
		if col == ColClosed {
			t.Fatal("the closed column must be hidden by default")
		}
	}
	out := stripANSI(b.View(200, 40))
	if strings.Contains(out, "Pick the capture format") || !strings.Contains(out, "closed 1 hidden") {
		t.Fatalf("the bar names the hidden closed column and its cards stay off the board:\n%s", out)
	}
	b.ToggleClosedColumn()
	out = stripANSI(b.View(200, 40))
	if !strings.Contains(out, "CLOSED") || !strings.Contains(out, "Pick the capture format") {
		t.Fatalf("c shows the closed column:\n%s", out)
	}
	b.ToggleClosedColumn()
	if strings.Contains(stripANSI(b.View(200, 40)), "Pick the capture format") {
		t.Fatal("a second c hides it again")
	}
}

func TestBoardClosedColumn_SelectionLeavesAHiddenColumn(t *testing.T) {
	b := newEpicBoard(BoardEpicRail)
	b.ToggleClosedColumn()
	b.SelectIssueByID("spectroscope-eg0.3")
	b.ToggleClosedColumn()
	if b.actualFocusedCol() == ColClosed {
		t.Fatal("hiding the closed column must move the focus off it")
	}
	if b.SelectIssueByID("spectroscope-eg0.3") {
		t.Fatal("a card in the hidden closed column cannot be selected")
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
	if m.board.EpicView() != BoardEpicRows || !strings.Contains(m.statusMsg, "Epic rows") {
		t.Fatalf("v must switch to the rows design, got %s %q", m.board.EpicView(), m.statusMsg)
	}
	if sel := m.board.SelectedIssue(); sel == nil || sel.ID != "spectroscope-eg0.1" {
		t.Fatalf("switching design keeps the selection, got %+v", sel)
	}
	m = pressBoard(t, m, runeKey("v"))
	if m.board.EpicView() != BoardEpicRail {
		t.Fatalf("v cycles back to the rail, got %s", m.board.EpicView())
	}
}

func TestBoardKeyC_TogglesTheClosedColumn(t *testing.T) {
	m := epicKeyModel(t)
	filter := m.currentFilter
	m = pressBoard(t, m, runeKey("c"))
	if !m.board.ShowsClosedColumn() {
		t.Fatal("c must show the closed column")
	}
	if m.currentFilter != filter {
		t.Fatalf("c on the board must not change the filter, got %q", m.currentFilter)
	}
	m = pressBoard(t, m, runeKey("c"))
	if m.board.ShowsClosedColumn() {
		t.Fatal("a second c must hide the closed column")
	}
}

func TestBoardKeyTab_FoldsEpicInBothDesigns(t *testing.T) {
	for _, keys := range [][]tea.KeyMsg{nil, {runeKey("v")}} {
		m := pressBoard(t, epicKeyModel(t), keys...)
		m = pressBoard(t, m, tea.KeyMsg{Type: tea.KeyTab})
		if sel := m.board.SelectedIssue(); sel == nil || sel.ID != "spectroscope-eg0" {
			t.Fatalf("%s: tab folds the epic and selects it, got %+v", m.board.EpicView(), sel)
		}
		m = pressBoard(t, m, tea.KeyMsg{Type: tea.KeyShiftTab})
		if m.board.AnyEpicFolded() {
			t.Fatalf("%s: shift+tab with a folded epic must expand all", m.board.EpicView())
		}
	}
}

func TestBoardEpicConfig_SetsStartupDesign(t *testing.T) {
	cfg := config.Config{}
	cfg.UI.BoardEpics = "rows"
	m := NewModel(epicBoardIssues(), "").WithConfig(cfg, "spectroscope", "")
	if m.board.EpicView() != BoardEpicRows {
		t.Fatalf("ui.board_epics rows must start the rows design, got %s", m.board.EpicView())
	}
}

func TestBoardEpics_BarCountsFoldedIssues(t *testing.T) {
	b := newEpicBoard(BoardEpicRail)
	b.ToggleAllEpicFolds()
	view := stripANSI(b.View(200, 30))
	if !strings.Contains(view, "7 issues") || !strings.Contains(view, "4 folded") {
		t.Fatalf("the bar must count every shown issue and name the folded ones:\n%s", view)
	}
}

// railCells returns the rail part of each rendered line of a rail view.
func railCells(view string, width int) []string {
	var out []string
	for _, line := range strings.Split(stripANSI(view), "\n") {
		r := []rune(line)
		out = append(out, string(r[:min(width, len(r))]))
	}
	return out
}

func TestBoardEpicRail_EpicCellIsABoxSpanningItsLane(t *testing.T) {
	b := newEpicBoard(BoardEpicRail)
	view := b.View(200, 40)
	railW := b.railWidth(200)
	rail := railCells(view, railW)
	lines := strings.Split(stripANSI(view), "\n")

	head := -1
	for i, r := range rail {
		if strings.Contains(r, "◆ eg0") {
			head = i
			break
		}
	}
	if head < 1 || !strings.HasPrefix(rail[head-1], "╭") {
		t.Fatalf("the eg0 cell must open with a box top above its ID:\n%s", strings.Join(rail, "\n"))
	}
	rule := -1
	for i := head; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "─") {
			rule = i
			break
		}
	}
	if rule < 0 {
		t.Fatalf("no rule after the eg0 lane:\n%s", stripANSI(view))
	}
	if !strings.HasPrefix(rail[rule-1], "╰") {
		t.Fatalf("the eg0 cell must close on the line above the lane rule, got %q:\n%s", rail[rule-1], strings.Join(rail, "\n"))
	}
	for i := head; i < rule-1; i++ {
		if !strings.ContainsAny(string([]rune(rail[i])[:1]), "┃│") || !strings.HasSuffix(strings.TrimRight(rail[i], " "), "│") {
			t.Fatalf("line %d of the eg0 cell must have both box sides, got %q", i, rail[i])
		}
	}
	if rule-head < 6 {
		t.Fatalf("the eg0 lane holds two stacked cards, so its cell must be taller than its text:\n%s", strings.Join(rail, "\n"))
	}
}

func TestBoardEpicRail_SelectedEpicHasAHighlightedBorder(t *testing.T) {
	renderer := lipgloss.NewRenderer(io.Discard)
	renderer.SetColorProfile(termenv.TrueColor)
	theme := DefaultTheme(renderer)
	probe := renderer.NewStyle().Foreground(theme.Primary).Render("x")
	primary := probe[:strings.Index(probe, "x")]

	topOfEg0 := func(b BoardModel) string {
		lines := strings.Split(b.View(200, 40), "\n")
		for i, line := range lines {
			if strings.Contains(stripANSI(line), "◆ eg0") {
				return lines[i-1]
			}
		}
		t.Fatal("no eg0 cell")
		return ""
	}
	b := NewBoardModel(epicBoardIssues(), theme)
	b.SetActiveProjectName("spectroscope")
	b.SelectIssueByID("spectroscope-eg0")
	if top := topOfEg0(b); !strings.HasPrefix(top, primary) {
		t.Fatalf("the selected epic's box must use the primary border color, got %q", top)
	}
	b.SelectIssueByID("spectroscope-x1")
	if top := topOfEg0(b); strings.HasPrefix(top, primary) {
		t.Fatalf("an unselected epic's box must not use the primary border color, got %q", top)
	}
}

// The lane layout places cards by cardHeight and draws only the visible ones,
// so a height that disagrees with the drawn card shifts every card below it.
func TestBoardCardHeight_MatchesDrawnCard(t *testing.T) {
	b := newEpicBoard(BoardEpicRail)
	long := model.Issue{ID: "spectroscope-z1", Title: "A very long title that wraps over more than two lines on a narrow card for sure",
		IssueType: model.TypeTask, Status: model.StatusOpen, Priority: 1,
		Labels:       []string{"lane:review"},
		Dependencies: []*model.Dependency{{IssueID: "spectroscope-z1", DependsOnID: "spectroscope-eg0.1", Type: model.DepBlocks}}}
	issues := append(epicBoardIssues(), long)
	for _, is := range issues {
		for w := 8; w <= 48; w += 5 {
			for _, sel := range []bool{false, true} {
				got := b.cardHeight(is, w)
				want := len(b.cardLines(is, w, sel, 0, 0))
				if got != want {
					t.Fatalf("%s at width %d: cardHeight %d, drawn %d lines", is.ID, w, got, want)
				}
			}
		}
	}
}

// In the rows design the epic header is a full-width box, so it stands out
// from the cards under it, and a selected epic gets the selected-card border.
func TestBoardEpicRows_EpicHeaderIsAFullWidthBox(t *testing.T) {
	renderer := lipgloss.NewRenderer(io.Discard)
	renderer.SetColorProfile(termenv.TrueColor)
	theme := DefaultTheme(renderer)
	probe := renderer.NewStyle().Foreground(theme.Primary).Render("x")
	primary := probe[:strings.Index(probe, "x")]

	b := NewBoardModel(epicBoardIssues(), theme)
	b.SetActiveProjectName("spectroscope")
	b.SetEpicView(BoardEpicRows)
	const width = 160
	for _, sel := range []string{"spectroscope-x1", "spectroscope-eg0"} {
		b.SelectIssueByID(sel)
		raw := strings.Split(b.View(width, 40), "\n")
		head := -1
		for i, line := range raw {
			if strings.Contains(stripANSI(line), "◆ eg0") {
				head = i
				break
			}
		}
		if head < 1 || head+1 >= len(raw) {
			t.Fatalf("no eg0 header:\n%s", stripANSI(strings.Join(raw, "\n")))
		}
		top, mid, bottom := stripANSI(raw[head-1]), stripANSI(raw[head]), stripANSI(raw[head+1])
		if !strings.HasPrefix(top, "╭") || !strings.HasSuffix(top, "╮") {
			t.Fatalf("the header box must open over the full width, got %q", top)
		}
		if !strings.HasPrefix(mid, "┃") || !strings.HasSuffix(mid, "│") {
			t.Fatalf("the header row must have both box sides, got %q", mid)
		}
		if !strings.HasPrefix(bottom, "╰") || !strings.HasSuffix(bottom, "╯") {
			t.Fatalf("the header box must close under the row, got %q", bottom)
		}
		if selected := sel == "spectroscope-eg0"; selected != strings.HasPrefix(raw[head-1], primary) {
			t.Fatalf("selected=%v: the header border must be primary only when the epic is selected, got %q", selected, raw[head-1])
		}
	}
}

func selectedID(b BoardModel) string {
	if sel := b.SelectedIssue(); sel != nil {
		return strings.TrimPrefix(sel.ID, "spectroscope-")
	}
	return ""
}

// h and l move across the lane the selection is in, so the selection stays
// on the same band of the screen instead of jumping to another epic.
func TestBoardEpics_SideMovesStayInTheLane(t *testing.T) {
	for _, view := range []BoardEpicView{BoardEpicRail, BoardEpicRows} {
		b := newEpicBoard(view)
		b.SelectIssueByID("spectroscope-eg0.2.1")
		b.MoveRight()
		if got := selectedID(b); got != "eg0.2" {
			t.Fatalf("%s: l from eg0.2.1 must land on eg0.2 in the same lane, got %q", view, got)
		}
		b.MoveLeft()
		if got := selectedID(b); got != "eg0.2.1" {
			t.Fatalf("%s: h back must return to the card it came from, got %q", view, got)
		}
		b.SelectIssueByID("spectroscope-k3s.1")
		b.MoveRight()
		if got := selectedID(b); got != "k3s" {
			t.Fatalf("%s: with no k3s card in progress, l must land on the k3s epic there, got %q", view, got)
		}
		b.SelectIssueByID("spectroscope-x1")
		b.MoveRight()
		if got := selectedID(b); got != "eg0.2" {
			t.Fatalf("%s: with no lane card to the right, l takes the nearest card above, got %q", view, got)
		}
	}
}

// } and { step through the epics: to the next epic, and to the start of the
// current lane, then to the epic before it.
func TestBoardEpics_BracesJumpBetweenEpics(t *testing.T) {
	for _, view := range []BoardEpicView{BoardEpicRail, BoardEpicRows} {
		m := epicKeyModel(t)
		m.board.SetEpicView(view)
		m.board.SelectIssueByID("spectroscope-k3s.1")
		steps := []struct{ key, want string }{
			{"}", "eg0"}, {"}", "x1"}, {"}", "x1"},
			{"{", "eg0"}, {"{", "k3s"}, {"{", "k3s"},
		}
		for i, s := range steps {
			u, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s.key)})
			m = u.(Model)
			if got := selectedID(m.board); got != s.want {
				t.Fatalf("%s step %d (%s): want %q, got %q", view, i, s.key, s.want, got)
			}
		}
		m.board.SelectIssueByID("spectroscope-eg0.2")
		u, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("{")})
		if got := selectedID(u.(Model).board); got != "eg0" {
			t.Fatalf("%s: { inside a lane goes to its epic first, got %q", view, got)
		}
	}
}

// j and k move over cards only. An epic is a lane header, reached with { and },
// so whether its status matches the column never makes it a stop.
func TestBoardEpics_UpDownSkipLaneHeaders(t *testing.T) {
	for _, view := range []BoardEpicView{BoardEpicRail, BoardEpicRows} {
		b := newEpicBoard(view)
		b.SelectIssueByID("spectroscope-k3s.1")
		for _, want := range []string{"eg0.1", "eg0.2.1", "x1", "x1"} {
			b.MoveDown()
			if got := selectedID(b); got != want {
				t.Fatalf("%s: j wants %q, got %q", view, want, got)
			}
		}
		for _, want := range []string{"eg0.2.1", "eg0.1", "k3s.1", "k3s.1"} {
			b.MoveUp()
			if got := selectedID(b); got != want {
				t.Fatalf("%s: k wants %q, got %q", view, want, got)
			}
		}
		b.SelectIssueByID("spectroscope-eg0")
		b.MoveDown()
		if got := selectedID(b); got != "eg0.1" {
			t.Fatalf("%s: j from an epic goes to the next card, got %q", view, got)
		}
		b.SelectIssueByID("spectroscope-eg0")
		b.MoveUp()
		if got := selectedID(b); got != "k3s.1" {
			t.Fatalf("%s: k from an epic goes to the card above, got %q", view, got)
		}
		b.SelectIssueByID("spectroscope-eg0.2")
		b.MoveToTop()
		if got := selectedID(b); got != "eg0.2" {
			t.Fatalf("%s: top of in progress is its first card, not the k3s epic, got %q", view, got)
		}
		b.PageUp(40)
		if got := selectedID(b); got != "eg0.2" {
			t.Fatalf("%s: page up must not land on an epic, got %q", view, got)
		}
	}
}
