package ui

import (
	"io"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/vanderheijden86/b9s/pkg/identity"
	"github.com/vanderheijden86/b9s/pkg/model"
)

func TestFirstSentence(t *testing.T) {
	cases := map[string]string{
		"Live activity feed from bd events. Covers the TUI and web.": "Live activity feed from bd events.",
		"# Heading\n\nSync invoices from the billing API":            "Sync invoices from the billing API",
		"- first bullet\nsecond line":                                "first bullet second line",
		"Is it done? Not yet.":                                       "Is it done?",
		"No terminator at all":                                       "No terminator at all",
		"Version 2.1 ships soon. Then 3.":                            "Version 2.1 ships soon.",
		"One paragraph\n\nAnother paragraph.":                        "One paragraph",
		"":                                                           "",
		"  \n  ":                                                     "",
	}
	for in, want := range cases {
		if got := FirstSentence(in); got != want {
			t.Errorf("FirstSentence(%q) = %q, want %q", in, got, want)
		}
	}
}

// factsBoardIssues is one epic with every kind of child the epic cell
// counts: a ready issue, one waiting on an open blocker, one in progress,
// one deferred, one closed, one whose blocker is closed, and an urgent (P1)
// one. The epic itself carries the description, labels, owner and due date.
func factsBoardIssues() []model.Issue {
	now := time.Now()
	old := now.Add(-72 * time.Hour)
	recent := now.Add(-2 * time.Hour)
	due := now.Add(48 * time.Hour)
	mk := func(id string, status model.Status, prio int, updated time.Time, deps ...*model.Dependency) model.Issue {
		return model.Issue{ID: id, Title: id, IssueType: model.TypeTask, Status: status, Priority: prio,
			Dependencies: deps, CreatedAt: old, UpdatedAt: updated}
	}
	child := func(id string) *model.Dependency {
		return &model.Dependency{IssueID: id, DependsOnID: "p-e1", Type: model.DepParentChild}
	}
	blocks := func(id, on string) *model.Dependency {
		return &model.Dependency{IssueID: id, DependsOnID: on, Type: model.DepBlocks}
	}
	epic := model.Issue{ID: "p-e1", Title: "Epic: Events", IssueType: model.TypeEpic, Status: model.StatusOpen, Priority: 1,
		Description: "Live activity feed from bd events. Covers the TUI and web.",
		Labels:      []string{"tui", "web"}, Assignee: "andre", DueDate: &due, CreatedAt: old, UpdatedAt: old}
	return []model.Issue{
		epic,
		mk("p-e1.1", model.StatusOpen, 2, old, child("p-e1.1")),                                // ready
		mk("p-e1.2", model.StatusOpen, 1, recent, child("p-e1.2"), blocks("p-e1.2", "p-e1.1")), // waiting, urgent
		mk("p-e1.3", model.StatusInProgress, 2, old, child("p-e1.3")),                          // wip
		mk("p-e1.4", model.StatusDeferred, 3, old, child("p-e1.4")),                            // neither ready nor waiting
		mk("p-e1.5", model.StatusClosed, 2, old, child("p-e1.5")),                              // done
		mk("p-e1.6", model.StatusOpen, 2, old, child("p-e1.6"), blocks("p-e1.6", "p-e1.5")),    // blocker closed: ready
	}
}

func newFactsBoard(issues []model.Issue) BoardModel {
	b := NewBoardModel(issues, DefaultTheme(lipgloss.DefaultRenderer()))
	b.SetActiveProjectName("p")
	return b
}

func TestBoardEpicFacts_CountsOverTheEpicUniverse(t *testing.T) {
	b := newFactsBoard(factsBoardIssues())
	e := b.epics["p-e1"]
	if e == nil {
		t.Fatal("no epic p-e1")
	}
	got := *e
	if got.Done != 1 || got.Total != 6 || got.Urgent != 1 || got.InProgress != 1 || got.Waiting != 1 || got.Ready != 2 {
		t.Fatalf("counts: got done %d/%d urgent %d wip %d waiting %d ready %d, want 1/6 1 1 1 2",
			got.Done, got.Total, got.Urgent, got.InProgress, got.Waiting, got.Ready)
	}
	if got.Description != "Live activity feed from bd events." || got.Owner != "andre" || len(got.Labels) != 2 || got.Due == nil {
		t.Fatalf("facts: got %q owner %q labels %v due %v", got.Description, got.Owner, got.Labels, got.Due)
	}
	if time.Since(got.LastActivity) > 3*time.Hour {
		t.Fatalf("last activity must be the newest child's update, got %v", got.LastActivity)
	}
}

func TestBoardEpicFacts_OwnerFallsBackToOwnerField(t *testing.T) {
	issues := factsBoardIssues()
	issues[0].Assignee = ""
	issues[0].Owner = "polly"
	b := newFactsBoard(issues)
	if got := b.epics["p-e1"].Owner; got != "polly" {
		t.Fatalf("owner = %q, want polly", got)
	}
}

func TestBoardEpicFacts_AFilterDoesNotChangeThem(t *testing.T) {
	all := factsBoardIssues()
	b := newFactsBoard(all[:2]) // the epic and one ready child on the board
	b.SetEpicUniverse(all)
	if got := b.epics["p-e1"]; got.Total != 6 || got.Waiting != 1 || got.Ready != 2 {
		t.Fatalf("facts over the universe: total %d waiting %d ready %d, want 6 1 2", got.Total, got.Waiting, got.Ready)
	}
}

func TestBoardEpicFacts_OwnerShowsThroughTheIdentityRegistry(t *testing.T) {
	b := newFactsBoard(factsBoardIssues())
	reg, err := identity.Parse(`[{"name":"André","kind":"human","aliases":["andre"]}]`, "")
	if err != nil {
		t.Fatal(err)
	}
	if got := b.ownerLabel("andre"); got != "@andre" {
		t.Fatalf("without a registry ownerLabel = %q, want @andre", got)
	}
	b.SetIdentities(reg)
	if got := b.ownerLabel("andre"); got != "@André" {
		t.Fatalf("ownerLabel = %q, want @André", got)
	}
	if got := b.ownerLabel(""); got != "" {
		t.Fatalf("ownerLabel of nobody = %q, want empty", got)
	}
}

func TestSetIdentities_ReachesTheBoard(t *testing.T) {
	m := NewModel(factsBoardIssues(), "")
	reg, _ := identity.Parse(`[{"name":"André","kind":"human","aliases":["andre"]}]`, "")
	m.setIdentities(reg)
	if got := m.board.ownerLabel("andre"); got != "@André" {
		t.Fatalf("board ownerLabel = %q, want @André", got)
	}
}

func plainLines(lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = strings.TrimRight(stripANSI(l), " ")
	}
	return out
}

// The exact content is checked in a cell wide enough to hold every fact; the
// width checks below cover the cut at the rail's real widths.
func TestBoardEpicCell_FiveLinesWithEveryFact(t *testing.T) {
	b := newFactsBoard(factsBoardIssues())
	got := plainLines(b.railLines("p-e1", 44, false, false))
	joined := strings.Join(got, "\n")
	if len(got) != 5 {
		t.Fatalf("got %d lines, want title, description, bar, counts, meta:\n%s", len(got), joined)
	}
	if got[0] != "▾ ◆ e1 Epic: Events" {
		t.Errorf("title line = %q", got[0])
	}
	if got[1] != "Live activity feed from bd events." {
		t.Errorf("description = %q", got[1])
	}
	if !strings.HasPrefix(got[2], "━") || !strings.HasSuffix(got[2], " 1/6 · P1 1") {
		t.Errorf("bar line = %q, want a bar ending in 1/6 · P1 1", got[2])
	}
	if got[3] != "wip 1 · waiting 1 · ready 2" {
		t.Errorf("counts line = %q", got[3])
	}
	due := time.Now().Add(48 * time.Hour).Format("2 Jan")
	if want := "#tui #web · @andre · due " + due + " · 2h"; got[4] != want {
		t.Errorf("meta line = %q, want %q", got[4], want)
	}
	for _, w := range []int{16, 20, 34} {
		lines := b.railLines("p-e1", w, false, false)
		for i, l := range lines {
			if lw := lipgloss.Width(l); lw > railInner(w) {
				t.Errorf("cell %d: line %d is %d cells, wider than %d", w, i, lw, railInner(w))
			}
		}
		if last := strings.TrimRight(stripANSI(lines[len(lines)-1]), " "); !strings.HasSuffix(last, " · 2h") {
			t.Errorf("cell %d: the activity must survive the cut, got %q", w, last)
		}
	}
}

func TestBoardEpicCell_LeavesOutEmptyLines(t *testing.T) {
	issues := factsBoardIssues()
	issues[0].Description, issues[0].Labels, issues[0].Assignee, issues[0].DueDate = "", nil, "", nil
	b := newFactsBoard(issues)
	got := plainLines(b.railLines("p-e1", 44, false, false))
	if len(got) != 3 {
		t.Fatalf("got %d lines, want title, bar, counts:\n%s", len(got), strings.Join(got, "\n"))
	}
	if got[2] != "wip 1 · waiting 1 · ready 2 · 2h" {
		t.Fatalf("without a meta line the activity ends the counts line, got %q", got[2])
	}
	// Where the activity does not fit beside the counts, it takes its own
	// line rather than cutting the ready count.
	got = plainLines(b.railLines("p-e1", 34, false, false))
	if len(got) != 4 || got[2] != "wip 1 · waiting 1 · ready 2" || got[3] != "2h" {
		t.Fatalf("narrow cell = %q, want the counts whole and the activity below", got)
	}
}

func TestBoardEpicCell_NoUrgentNoP1(t *testing.T) {
	issues := factsBoardIssues()
	issues[2].Priority = 2
	b := newFactsBoard(issues)
	got := plainLines(b.railLines("p-e1", 44, false, false))
	if strings.Contains(got[2], "P1") || !strings.HasSuffix(got[2], " 1/6") {
		t.Fatalf("no open P0/P1: the bar line ends at done/total, got %q", got[2])
	}
}

func TestBoardEpicCell_FoldedShowsTitleAndBar(t *testing.T) {
	b := newFactsBoard(factsBoardIssues())
	got := plainLines(b.railLines("p-e1", 34, true, false))
	if len(got) != 2 || got[0] != "▸ ◆ e1 Epic: Events" || !strings.Contains(got[1], "1/6") {
		t.Fatalf("folded cell = %q, want the folded title line and the bar", got)
	}
}

func TestBoardEpicCell_NarrowCellCutsTheMetaLineNotTheActivity(t *testing.T) {
	b := newFactsBoard(factsBoardIssues())
	lines := b.railLines("p-e1", 20, false, false)
	for i, l := range lines {
		if w := lipgloss.Width(l); w > railInner(20) {
			t.Errorf("line %d is %d cells, wider than %d: %q", i, w, railInner(20), stripANSI(l))
		}
	}
	last := strings.TrimRight(stripANSI(lines[len(lines)-1]), " ")
	if !strings.HasPrefix(last, "#tui") || !strings.HasSuffix(last, "… · 2h") {
		t.Fatalf("meta line = %q, want labels cut with an ellipsis before the activity", last)
	}
}

func TestBoardEpicCell_PastDueIsRed(t *testing.T) {
	renderer := lipgloss.NewRenderer(io.Discard)
	renderer.SetColorProfile(termenv.TrueColor)
	issues := factsBoardIssues()
	past := time.Now().Add(-24 * time.Hour)
	issues[0].DueDate = &past
	b := NewBoardModel(issues, DefaultTheme(renderer))
	b.SetActiveProjectName("p")
	lines := b.railLines("p-e1", 44, false, false)
	red := renderer.NewStyle().Foreground(overdueColor).Render("due")
	if !strings.Contains(lines[4], red[:strings.Index(red, "due")]+"due") {
		t.Fatalf("a past due date must be red: %q", lines[4])
	}
}

func TestBoardRailWidth_IsAFifthBetween16And34(t *testing.T) {
	b := newEpicBoard(BoardEpicRail)
	for _, c := range []struct{ width, want int }{{60, 16}, {100, 20}, {170, 34}, {240, 34}} {
		if got := b.railWidth(c.width); got != c.want {
			t.Errorf("railWidth(%d) = %d, want %d", c.width, got, c.want)
		}
	}
}

func TestBoardEpicRows_HeaderHasTheFactsOnTwoLines(t *testing.T) {
	b := newFactsBoard(factsBoardIssues())
	b.SetEpicView(BoardEpicRows)
	lines := plainLines(b.laneBox("p-e1", 140, false, false))
	if len(lines) != 4 {
		t.Fatalf("got %d lines, want top, two content lines, bottom:\n%s", len(lines), strings.Join(lines, "\n"))
	}
	for _, want := range []string{"▾ ◆ e1 Epic: Events", "wip 1 · waiting 1 · ready 2", "#tui #web", "@andre", "2h", "1/6 · P1 1"} {
		if !strings.Contains(lines[1], want) {
			t.Errorf("line one misses %q: %q", want, lines[1])
		}
	}
	if !strings.Contains(lines[2], "Live activity feed from bd events.") {
		t.Errorf("line two is the description, got %q", lines[2])
	}
	for _, w := range []int{40, 80, 140} {
		for i, l := range b.laneBox("p-e1", w, false, false) {
			if lw := lipgloss.Width(l); lw != w {
				t.Errorf("header at %d: line %d is %d cells", w, i, lw)
			}
		}
	}
	if got := b.laneBox("p-e1", 140, true, false); len(got) != 3 {
		t.Fatalf("a folded row is one content line, got %d lines", len(got))
	}
	issues := factsBoardIssues()
	issues[0].Description = ""
	b = newFactsBoard(issues)
	b.SetEpicView(BoardEpicRows)
	if got := b.laneBox("p-e1", 140, false, false); len(got) != 3 {
		t.Fatalf("without a description the header is one content line, got %d lines", len(got))
	}
}

// Beads stores an issue's owner as an email address; the cell shows its
// local part so the address does not fill the line.
func TestBoardEpicFacts_OwnerEmailShowsItsLocalPart(t *testing.T) {
	b := newFactsBoard(factsBoardIssues())
	if got := b.ownerLabel("andre@example.com"); got != "@andre" {
		t.Fatalf("ownerLabel = %q, want @andre", got)
	}
	reg, _ := identity.Parse(`[{"name":"André","kind":"human","aliases":["andre@example.com"]}]`, "")
	b.SetIdentities(reg)
	if got := b.ownerLabel("andre@example.com"); got != "@André" {
		t.Fatalf("a registered address shows its name, got %q", got)
	}
}
