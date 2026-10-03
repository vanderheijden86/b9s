package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/vanderheijden86/b9s/pkg/model"
)

// sparseIssue returns the sparse board's copy of id, so the card sees the
// board's blocker and downstream indexes.
func sparseIssue(t *testing.T, b BoardModel, id string) model.Issue {
	t.Helper()
	is := b.issueMap[id]
	if is == nil {
		t.Fatalf("no issue %s on the sparse board", id)
	}
	return *is
}

func TestBoardCard_UnselectedIsOneLine(t *testing.T) {
	b := newSparseBoard()
	lines := b.cardLines(sparseIssue(t, b, "spectroscope-eg0.4.1"), 100, false, 0, 0)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1:\n%s", len(lines), strings.Join(lines, "\n"))
	}
	got := strings.TrimRight(stripANSI(lines[0]), " ")
	if !strings.HasPrefix(got, "✔ eg0.4.1 Add packet append API with cursor semantics") {
		t.Fatalf("line must start with icon, ID and title, got %q", got)
	}
	if !strings.HasSuffix(got, "lane: reviewing · blocks 1 · P1") {
		t.Fatalf("tags must be right-aligned in order, got %q", got)
	}
	if w := lipgloss.Width(lines[0]); w != 100 {
		t.Fatalf("line width %d, want 100", w)
	}
	blocked := strings.TrimRight(stripANSI(b.cardLines(sparseIssue(t, b, "spectroscope-eg0.4.2"), 80, false, 0, 0)[0]), " ")
	if !strings.HasSuffix(blocked, "blocked eg0.4.1 · P1") {
		t.Fatalf("a blocked card names its blocker first, got %q", blocked)
	}
}

func TestBoardCard_TagsDropFromTheRightBeforeTheTitle(t *testing.T) {
	b := newSparseBoard()
	is := sparseIssue(t, b, "spectroscope-eg0.4.1")
	got := strings.TrimRight(stripANSI(b.cardLines(is, 44, false, 0, 0)[0]), " ")
	if strings.Contains(got, "P1") || strings.Contains(got, "blocks") {
		t.Fatalf("at 44 cells the later tags go first, got %q", got)
	}
	if !strings.Contains(got, "lane: reviewing") || !strings.Contains(got, "Add packe") || !strings.Contains(got, "…") {
		t.Fatalf("the first tag and a cut title must stay, got %q", got)
	}
	narrow := strings.TrimRight(stripANSI(b.cardLines(is, 14, false, 0, 0)[0]), " ")
	if !strings.HasPrefix(narrow, "✔ eg0.4.1") {
		t.Fatalf("the ID is never dropped, got %q", narrow)
	}
	for w := 4; w <= 90; w += 5 {
		if lw := lipgloss.Width(b.cardLines(is, w, false, 0, 0)[0]); lw != w {
			t.Errorf("card at %d cells is %d wide", w, lw)
		}
	}
}

func TestBoardCard_LowPriorityHasNoPriorityTag(t *testing.T) {
	b := newSparseBoard()
	got := strings.TrimRight(stripANSI(b.cardLines(sparseIssue(t, b, "spectroscope-eg0.3"), 60, false, 0, 0)[0]), " ")
	if got != "✔ eg0.3 Open work item 3" {
		t.Fatalf("a P2 card with no tags is icon, ID and title only, got %q", got)
	}
}

func TestBoardCard_SelectedIsTheBox(t *testing.T) {
	b := newSparseBoard()
	lines := plainLines(b.cardLines(sparseIssue(t, b, "spectroscope-eg0.4.2"), 40, true, 0, 0))
	if len(lines) < 5 || !strings.HasPrefix(lines[0], "╭") || !strings.HasPrefix(lines[len(lines)-1], "╰") {
		t.Fatalf("the selected card must be the rounded box:\n%s", strings.Join(lines, "\n"))
	}
	if !strings.Contains(strings.Join(lines, "\n"), "blocked by eg0.4.1") {
		t.Fatalf("the box keeps the tag line:\n%s", strings.Join(lines, "\n"))
	}
}

func TestBoardCardHeight_FollowsTheSelection(t *testing.T) {
	b := newSparseBoard()
	is := sparseIssue(t, b, "spectroscope-eg0.4.2")
	for w := 8; w <= 80; w += 9 {
		if got := b.cardHeight(is, w, false); got != 1 {
			t.Errorf("unselected at %d: height %d, want 1", w, got)
		}
		if got, want := b.cardHeight(is, w, true), len(b.cardLines(is, w, true, 0, 0)); got != want {
			t.Errorf("selected at %d: height %d, drawn %d", w, got, want)
		}
	}
}

// The selected card's box makes its lane taller, so the lane below starts
// further down than when the selection is elsewhere. The eg0 lane sits
// between the k3s lane (it ranks first, with a P0) and the No epic lane.
func TestBoardLanes_SelectedCardGrowsItsLane(t *testing.T) {
	at := func(lines []string, s string) int {
		for i, l := range lines {
			if strings.Contains(l, s) {
				return i
			}
		}
		return -1
	}
	gap := func(sel string) int {
		b := newEpicBoard(BoardEpicRows)
		if !b.SelectIssueByID(sel) {
			t.Fatalf("cannot select %s", sel)
		}
		lines := strings.Split(stripANSI(b.View(200, 60)), "\n")
		eg0, next := at(lines, "◆ eg0"), at(lines, "◇ No epic")
		if eg0 < 0 || next < eg0 {
			t.Fatalf("lanes not drawn in order:\n%s", strings.Join(lines, "\n"))
		}
		return next - eg0
	}
	if outside, inside := gap("spectroscope-x1"), gap("spectroscope-eg0.1"); inside <= outside {
		t.Fatalf("selecting a card of a lane must make it taller: %d lines, %d with the selection elsewhere", inside, outside)
	}
}

// Side by side, one-line cards would touch across the one-cell column gap,
// so a card keeps a right margin before the next column.
func TestBoardCard_KeepsARightMargin(t *testing.T) {
	b := newSparseBoard()
	got := stripANSI(b.cardLines(sparseIssue(t, b, "spectroscope-eg0.4.1"), 100, false, 0, 0)[0])
	if !strings.HasSuffix(got, "P1"+strings.Repeat(" ", cardLineMargin)) {
		t.Fatalf("tags must end %d cells before the edge, got %q", cardLineMargin, got)
	}
}

func featureBracketIssues() []model.Issue {
	makeIssue := func(id, title, parent string, typ model.IssueType, priority int) model.Issue {
		is := model.Issue{ID: id, Title: title, IssueType: typ, Status: model.StatusClosed, Priority: priority}
		if parent != "" {
			is.Dependencies = epicChildDeps(id, parent)
		}
		return is
	}
	return []model.Issue{
		makeIssue("p-e", "Board", "", model.TypeEpic, 2),
		makeIssue("p-a", "Navigation", "p-e", model.TypeFeature, 2),
		makeIssue("p-b", "Editing", "p-e", model.TypeFeature, 2),
		makeIssue("p-a1", "First navigation task", "p-a", model.TypeTask, 0),
		makeIssue("p-b1", "Edit title", "p-b", model.TypeTask, 1),
		makeIssue("p-sub", "Keyboard", "p-a", model.TypeTask, 2),
		makeIssue("p-key", "Arrow keys", "p-sub", model.TypeTask, 3),
		makeIssue("p-loose", "Unrelated", "p-e", model.TypeTask, 2),
	}
}

func TestBoardFeatureBrackets_NestedGroups(t *testing.T) {
	for _, view := range []BoardEpicView{BoardEpicRail, BoardEpicRows} {
		b := NewBoardModel(featureBracketIssues(), DefaultTheme(lipgloss.DefaultRenderer()))
		b.ToggleClosedColumn()
		b.SetEpicView(view)
		b.SelectIssueByID("p-e")
		got := stripANSI(b.View(240, 60))
		for _, text := range []string{"┌─ p-a Navigation", "┌─ p-b Editing", "│ ┌─ p-sub Keyboard", "│ │", "└─"} {
			if !strings.Contains(got, text) {
				t.Errorf("%s missing %q:\n%s", view, text, got)
			}
		}
	}
}

func TestBoardFeatureBrackets_WithoutEpic(t *testing.T) {
	issues := featureBracketIssues()[1:]
	for i := range issues {
		if issues[i].ID == "p-a" || issues[i].ID == "p-b" || issues[i].ID == "p-loose" {
			issues[i].Dependencies = nil
		}
	}
	b := NewBoardModel(issues, DefaultTheme(lipgloss.DefaultRenderer()))
	b.ToggleClosedColumn()
	if got := stripANSI(b.View(200, 40)); !strings.Contains(got, "┌─ p-a Navigation") {
		t.Fatal("features without an epic need brackets too")
	}
}

func TestBoardFeatureBrackets_WithoutEpicScrolls(t *testing.T) {
	b := NewBoardModel(featureBracketIssues()[1:], DefaultTheme(lipgloss.DefaultRenderer()))
	b.ToggleClosedColumn()
	b.SelectIssueByID("p-loose")
	if got := stripANSI(b.View(120, 12)); !strings.Contains(got, "Unrelated") {
		t.Fatal("scroll lost selected card")
	}
}

func TestBoardFeatureBrackets_ViewportKeepsSelectedCard(t *testing.T) {
	for _, width := range []int{40, 80, 120, 240} {
		for _, view := range []BoardEpicView{BoardEpicRail, BoardEpicRows} {
			b := NewBoardModel(featureBracketIssues(), DefaultTheme(lipgloss.DefaultRenderer()))
			b.ToggleClosedColumn()
			b.SetEpicView(view)
			b.SelectIssueByID("p-key")
			got := b.View(width, 16)
			if !strings.Contains(stripANSI(got), "Arrow keys") {
				t.Errorf("%s at %d loses selected card", view, width)
			}
			for _, line := range strings.Split(got, "\n") {
				if lipgloss.Width(line) > width {
					t.Errorf("%s at %d overflows: %d", view, width, lipgloss.Width(line))
				}
			}
		}
	}
}

func TestBoardFeatureBrackets_NavigationFollowsGroups(t *testing.T) {
	b := NewBoardModel(featureBracketIssues(), DefaultTheme(lipgloss.DefaultRenderer()))
	b.ToggleClosedColumn()
	b.SelectIssueByID("p-a")
	for _, want := range []string{"p-a1", "p-sub", "p-key", "p-b", "p-b1", "p-loose"} {
		b.MoveDown()
		if got := b.SelectedIssue(); got == nil || got.ID != want {
			t.Fatalf("want %s, got %+v", want, got)
		}
	}
}

func TestBoardFeatureBrackets_FilteredParentContext(t *testing.T) {
	all := featureBracketIssues()
	b := NewBoardModel([]model.Issue{all[6]}, DefaultTheme(lipgloss.DefaultRenderer()))
	b.SetEpicUniverse(all)
	b.ToggleClosedColumn()
	got := stripANSI(b.View(200, 30))
	for _, text := range []string{"Navigation", "Keyboard", "Arrow keys"} {
		if !strings.Contains(got, text) {
			t.Fatalf("missing %s:\n%s", text, got)
		}
	}
	if b.TotalCount() != 1 {
		t.Fatalf("context headings must not add issues: %d", b.TotalCount())
	}
}

func TestBoardFeatureBrackets_IgnoreBlockingLinks(t *testing.T) {
	all := featureBracketIssues()
	all[3].Dependencies[0].Type = model.DepBlocks
	b := NewBoardModel(all, DefaultTheme(lipgloss.DefaultRenderer()))
	if len(b.groupPaths["p-a1"]) != 0 {
		t.Fatal("a blocker must not make a task a feature member")
	}
}

func TestBoardFeatureBrackets_CyclicParentsStayBounded(t *testing.T) {
	all := featureBracketIssues()
	all[1].Dependencies = epicChildDeps("p-a", "p-sub")
	b := NewBoardModel(all, DefaultTheme(lipgloss.DefaultRenderer()))
	b.ToggleClosedColumn()
	seen := map[string]bool{}
	for _, is := range b.columns[ColClosed] {
		if seen[is.ID] {
			t.Fatalf("duplicate card %s", is.ID)
		}
		seen[is.ID] = true
		if len(b.groupPaths[is.ID]) > maxEpicDepth {
			t.Fatal("unbounded group ancestry")
		}
	}
	if len(seen) != len(all) {
		t.Fatalf("lost cards: got %d want %d", len(seen), len(all))
	}
	_ = b.View(80, 20)
}

func TestBoardFeatureBrackets_SearchSelectsGroupedCard(t *testing.T) {
	b := NewBoardModel(featureBracketIssues(), DefaultTheme(lipgloss.DefaultRenderer()))
	b.ToggleClosedColumn()
	b.StartSearch()
	for _, r := range "Arrow keys" {
		b.AppendSearchChar(r)
	}
	if b.SearchMatchCount() != 1 {
		t.Fatalf("want one match, got %d", b.SearchMatchCount())
	}
	if is := b.SelectedIssue(); is == nil || is.ID != "p-key" {
		t.Fatalf("wrong search selection: %+v", is)
	}
}
