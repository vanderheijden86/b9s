package ui

import (
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/vanderheijden86/beadwork/pkg/model"
)

// sparseBoardIssues mirrors the screenshot case in docs/board-redesign-spec.md:
// many open issues, one in progress, no stored blocked status, a closed backlog.
func sparseBoardIssues() []model.Issue {
	now := time.Now()
	var issues []model.Issue
	for i := 0; i < 12; i++ {
		issues = append(issues, model.Issue{
			ID: fmt.Sprintf("spectroscope-eg0.%d", i), Title: fmt.Sprintf("Open work item %d", i),
			Status: model.StatusOpen, IssueType: model.TypeTask, Priority: 2,
			CreatedAt: now.Add(-48 * time.Hour), UpdatedAt: now.Add(-48 * time.Hour),
		})
	}
	issues = append(issues,
		model.Issue{
			ID: "spectroscope-eg0.4.1", Title: "Add packet append API with cursor semantics",
			Status: model.StatusOpen, IssueType: model.TypeTask, Priority: 1,
			Labels:    []string{"lane-stage=reviewing"},
			CreatedAt: now.Add(-48 * time.Hour), UpdatedAt: now.Add(-48 * time.Hour),
		},
		model.Issue{
			ID: "spectroscope-eg0.4.2", Title: "Wire the flow subscription transport",
			Status: model.StatusOpen, IssueType: model.TypeTask, Priority: 1,
			Description: "Stream packets to subscribers once the append API exists.",
			Dependencies: []*model.Dependency{
				{IssueID: "spectroscope-eg0.4.2", DependsOnID: "spectroscope-eg0.4.1", Type: model.DepBlocks},
			},
			CreatedAt: now.Add(-48 * time.Hour), UpdatedAt: now.Add(-48 * time.Hour),
		},
		model.Issue{
			ID: "spectroscope-eg0.14.1", Title: "Use recorded mac-k3s capture fixture",
			Status: model.StatusInProgress, IssueType: model.TypeTask, Priority: 2,
			Labels:    []string{"lane-stage=implementing"},
			CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Minute),
		},
	)
	for i := 0; i < 20; i++ {
		closed := now.Add(-time.Duration(i) * time.Hour)
		issues = append(issues, model.Issue{
			ID: fmt.Sprintf("spectroscope-jzf.%d", i), Title: fmt.Sprintf("Delivered item %d", i),
			Status: model.StatusClosed, IssueType: model.TypeTask, Priority: 2,
			CreatedAt: now.Add(-72 * time.Hour), UpdatedAt: closed, ClosedAt: &closed,
		})
	}
	return issues
}

func newSparseBoard() BoardModel {
	b := NewBoardModel(sparseBoardIssues(), DefaultTheme(lipgloss.DefaultRenderer()))
	b.SetActiveProjectName("spectroscope")
	return b
}

func regionFor(regions []boardRegion, col int) (boardRegion, bool) {
	for _, r := range regions {
		if r.col == col {
			return r, true
		}
	}
	return boardRegion{}, false
}

func regionsWidth(regions []boardRegion) int {
	total := 0
	for _, r := range regions {
		total += r.width
	}
	if len(regions) > 1 {
		total += len(regions) - 1
	}
	return total
}

func statusInputs(counts [4]int) []boardRegionInput {
	var in []boardRegionInput
	for col, n := range counts {
		in = append(in, boardRegionInput{col: col, count: n})
	}
	return in
}

func TestPlanBoardRegions_WideSparseCollapsesEmptyColumns(t *testing.T) {
	regions := planBoardRegions(220, statusInputs([4]int{29, 1, 0, 76}), ColOpen, adaptiveBreakpoints)

	if got := regionsWidth(regions); got != 220 {
		t.Fatalf("regions use %d columns, want exactly 220: %+v", got, regions)
	}
	open, _ := regionFor(regions, ColOpen)
	inProgress, _ := regionFor(regions, ColInProgress)
	blocked, _ := regionFor(regions, ColBlocked)
	closed, _ := regionFor(regions, ColClosed)
	if open.collapsed || inProgress.collapsed || closed.collapsed {
		t.Fatalf("populated columns must stay full: %+v", regions)
	}
	if !blocked.collapsed || blocked.width > 16 {
		t.Fatalf("an empty column must collapse to a narrow rail: %+v", regions)
	}
}

func TestPlanBoardRegions_FullColumnsShareTheWidthEqually(t *testing.T) {
	for _, focus := range []int{ColOpen, ColInProgress, ColClosed} {
		regions := planBoardRegions(220, statusInputs([4]int{29, 1, 0, 76}), focus, adaptiveBreakpoints)
		lo, hi := 1<<30, 0
		for _, r := range regions {
			if !r.collapsed {
				lo, hi = min(lo, r.width), max(hi, r.width)
			}
		}
		if hi-lo > 1 {
			t.Fatalf("focus %d: full columns must be equally wide, got %+v", focus, regions)
		}
	}
}

func TestPlanBoardRegions_FocusExpandsARail(t *testing.T) {
	regions := planBoardRegions(220, statusInputs([4]int{29, 1, 0, 76}), ColClosed, adaptiveBreakpoints)
	closed, _ := regionFor(regions, ColClosed)
	if closed.collapsed {
		t.Fatalf("the focused column is never a rail: %+v", regions)
	}
}

func TestPlanBoardRegions_EightyColumnsKeepsFocusPlusOneNeighbour(t *testing.T) {
	regions := planBoardRegions(80, statusInputs([4]int{10, 10, 10, 10}), ColInProgress, adaptiveBreakpoints)
	full := 0
	for _, r := range regions {
		if !r.collapsed {
			full++
		}
	}
	if full != 2 {
		t.Fatalf("80 columns shows the focus and one neighbour in full, got %d full: %+v", full, regions)
	}
	if got := regionsWidth(regions); got > 80 {
		t.Fatalf("regions overflow 80 columns: %d", got)
	}
}

func TestPlanBoardRegions_NeverOverflows(t *testing.T) {
	for _, width := range []int{40, 60, 80, 110, 160, 220} {
		for focus := 0; focus < 4; focus++ {
			regions := planBoardRegions(width, statusInputs([4]int{5, 0, 3, 9}), focus, adaptiveBreakpoints)
			if got := regionsWidth(regions); got > width {
				t.Fatalf("width %d focus %d: regions use %d", width, focus, got)
			}
			if _, ok := regionFor(regions, focus); !ok {
				t.Fatalf("width %d: focused column %d is not shown", width, focus)
			}
		}
	}
}

func TestBoardCardView_SeparatesStatusReadinessLaneAndImpact(t *testing.T) {
	b := newSparseBoard()
	waiting := b.issueMap["spectroscope-eg0.4.2"]
	card := b.cardView(*waiting)
	if card.ShortID != "eg0.4.2" {
		t.Fatalf("ShortID = %q, want the path without the project prefix", card.ShortID)
	}
	if card.BlockedBy != "eg0.4.1" {
		t.Fatalf("BlockedBy = %q, want eg0.4.1", card.BlockedBy)
	}

	blocker := b.cardView(*b.issueMap["spectroscope-eg0.4.1"])
	if blocker.LaneStage != "reviewing" || blocker.BlocksCount != 1 {
		t.Fatalf("blocker card = %+v, want lane reviewing and blocks 1", blocker)
	}
}

func TestBoardCardView_ClosedBlockerDoesNotBlock(t *testing.T) {
	now := time.Now()
	issues := []model.Issue{
		{ID: "bd-1", Status: model.StatusClosed, ClosedAt: &now},
		{ID: "bd-2", Status: model.StatusOpen, Dependencies: []*model.Dependency{
			{IssueID: "bd-2", DependsOnID: "bd-1", Type: model.DepBlocks},
		}},
	}
	b := NewBoardModel(issues, DefaultTheme(lipgloss.DefaultRenderer()))
	if got := b.cardView(issues[1]).BlockedBy; got != "" {
		t.Fatalf("a closed dependency must not read as blocking, got %q", got)
	}
}

func TestBoardCardView_AllProjectsKeepsFullID(t *testing.T) {
	b := NewBoardModel(sparseBoardIssues(), DefaultTheme(lipgloss.DefaultRenderer()))
	if got := b.cardView(*b.issueMap["spectroscope-eg0.4.2"]).ShortID; got != "spectroscope-eg0.4.2" {
		t.Fatalf("without an active project the ID must stay whole, got %q", got)
	}
}

func assertFits(t *testing.T, name, view string, width, height int) {
	t.Helper()
	lines := strings.Split(view, "\n")
	if len(lines) > height {
		t.Fatalf("%s: %d lines, height is %d", name, len(lines), height)
	}
	for i, line := range lines {
		if w := lipgloss.Width(line); w > width {
			t.Fatalf("%s: line %d is %d wide, width is %d: %q", name, i, w, width, stripANSI(line))
		}
	}
}

func TestBoardLayouts_FitEveryWidth(t *testing.T) {
	for _, layout := range []BoardEpicView{BoardEpicRail, BoardEpicRows} {
		for _, width := range []int{60, 80, 110, 160, 220} {
			b := newSparseBoard()
			b.SetEpicView(layout)
			for col := 0; col < 4; col++ {
				b.JumpToColumn(col)
				name := fmt.Sprintf("%s/%d/col%d", layout, width, col)
				assertFits(t, name, b.View(width, 30), width, 30)
			}
		}
	}
}

func TestBoardAdaptiveView_ShowsBoxedCardsAndRails(t *testing.T) {
	b := newSparseBoard()
	b.SelectIssueByID("spectroscope-eg0.4.2")
	view := stripANSI(b.View(220, 40))

	for _, want := range []string{
		"Epic rail 1/2",
		"eg0.4.2", "Wire the flow subscription transport", "blocked by eg0.4.1",
		"eg0.4.1", "lane: reviewing", "blocks 1",
		"lane: implementing",
		"closed 20 hidden", "c closed",
		"v design", "{ } epic", "↑↓ in lane",
		"╭", "╰",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("adaptive board is missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "spectroscope-eg0") {
		t.Errorf("single-project board must not repeat the project prefix:\n%s", view)
	}
	if strings.Contains(view, "Delivered item") {
		t.Errorf("the closed column is hidden until c:\n%s", view)
	}
}

func TestBoardView_RemovesTerminalControlPayloads(t *testing.T) {
	issue := model.Issue{
		ID:          "bd-1\x1b[2J",
		Title:       "safe\x1b]52;c;YXR0YWNr\x07title",
		Description: "body\x1b]52;c;YXR0YWNr\x07",
		Status:      model.StatusOpen,
		IssueType:   model.TypeTask,
		Labels:      []string{"lane-stage=ok\u009b31mbad"},
	}
	for _, layout := range []BoardEpicView{BoardEpicRail, BoardEpicRows} {
		b := NewBoardModel([]model.Issue{issue}, DefaultTheme(lipgloss.DefaultRenderer()))
		b.SetEpicView(layout)
		out := b.View(160, 30)
		for _, forbidden := range []string{"\x1b]52", "YXR0YWNr", "[2J", "[31m"} {
			if strings.Contains(out, forbidden) {
				t.Fatalf("%s retained terminal control payload %q", layout, forbidden)
			}
		}
	}
}

func TestBoardView_SelectedRowKeepsBackgroundAfterInnerResets(t *testing.T) {
	renderer := lipgloss.NewRenderer(io.Discard)
	renderer.SetColorProfile(termenv.TrueColor)
	theme := DefaultTheme(renderer)
	for _, layout := range []BoardEpicView{BoardEpicRail, BoardEpicRows} {
		b := NewBoardModel(sparseBoardIssues(), theme)
		b.SetActiveProjectName("spectroscope")
		b.SetEpicView(layout)
		b.SelectIssueByID("spectroscope-eg0.4.2")
		bg := bgSeqFromColor(theme.Highlight, renderer)
		if bg == "" {
			t.Fatal("expected a background sequence in true colour")
		}
		var row string
		for _, line := range strings.Split(b.View(200, 30), "\n") {
			if strings.Contains(stripANSI(line), "eg0.4.2") && strings.Contains(line, bg) {
				row = line
			}
		}
		if row == "" || !strings.Contains(row, "\x1b[0m"+bg) {
			t.Fatalf("%s: selected row must restore its background after inner resets: %q", layout, row)
		}
	}
}

func TestBoardCardView_DropsTheIDPrefixAlreadyInTheTitle(t *testing.T) {
	b := newSparseBoard()
	issue := model.Issue{ID: "spectroscope-f7t.1", Title: "[f7t.1] Patch small-hetzner", Status: model.StatusOpen}
	if got := b.cardView(issue).Title; got != "Patch small-hetzner" {
		t.Fatalf("title must not repeat the ID shown beside it, got %q", got)
	}
	other := model.Issue{ID: "spectroscope-f7t.1", Title: "[f7t.2] Mentions a sibling", Status: model.StatusOpen}
	if got := b.cardView(other).Title; got != "[f7t.2] Mentions a sibling" {
		t.Fatalf("a bracket that is not this issue's ID stays, got %q", got)
	}
}

func TestBoardRail_WrapsTheBlockedNoteInsteadOfClipping(t *testing.T) {
	b := newSparseBoard()
	view := stripANSI(b.View(220, 30))
	for _, want := range []string{"none stored", "1 open", "wait on deps"} {
		if !strings.Contains(view, want) {
			t.Fatalf("blocked rail must show %q on its own line:\n%s", want, view)
		}
	}
}
