package ui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/vanderheijden86/b9s/internal/datasource"
	"github.com/vanderheijden86/b9s/pkg/model"
)

// fixtureMemoryGraph is one epic whose feature follows a superseded decision,
// a task that follows a proposed one, and a task with no decision at all.
func fixtureMemoryGraph() datasource.MemoryGraph {
	issue := func(id, title string, typ model.IssueType, status model.Status) datasource.GraphNode {
		return datasource.GraphNode{ID: id, Kind: "issue", Title: title, Status: string(status),
			Issue: model.Issue{ID: id, Title: title, IssueType: typ, Status: status, Priority: 2}}
	}
	memory := func(id, number, status, title string) datasource.GraphNode {
		return datasource.GraphNode{ID: id, Kind: "memory", Number: number, Status: status, Date: "2026-09-14",
			Title: "[" + id + "] ADR " + number + " (" + status + "): " + title}
	}
	nodes := []datasource.GraphNode{
		memory("adr-0009", "0009", "superseded", "Colon prompt"),
		memory("adr-0027", "0027", "active", "Flat list"),
		memory("adr-0026", "0026", "proposed", "Graph board"),
		issue("sample-e", "Epic E", model.TypeEpic, model.StatusOpen),
		issue("sample-e.1", "Dead work", model.TypeFeature, model.StatusInProgress),
		issue("sample-e.2", "Proposed work", model.TypeTask, model.StatusOpen),
		issue("sample-lone", "Lone work", model.TypeTask, model.StatusOpen),
	}
	edges := []datasource.GraphEdge{
		{ID: "c1", Kind: datasource.EdgeChildOf, Source: "sample-e.1", Target: "sample-e"},
		{ID: "c2", Kind: datasource.EdgeChildOf, Source: "sample-e.2", Target: "sample-e"},
		{ID: "f1", Kind: datasource.EdgeFollows, Source: "sample-e.1", Target: "adr-0009", Note: "built on the colon prompt"},
		{ID: "x1", Kind: datasource.EdgeCites, Source: "sample-e.1", Target: "adr-0027"},
		{ID: "f2", Kind: datasource.EdgeFollows, Source: "sample-e.2", Target: "adr-0026"},
		{ID: "s1", Kind: datasource.EdgeSupersedes, Source: "adr-0027", Target: "adr-0009"},
		{ID: "b1", Kind: datasource.EdgeBlocks, Source: "sample-e.2", Target: "sample-e.1"},
	}
	return datasource.NewMemoryGraph(nodes, edges)
}

func newGraphFakeSource() *fakeMemorySource {
	f := newFakeMemorySource()
	f.graph = fixtureMemoryGraph()
	for _, id := range []string{"adr-0009", "adr-0027", "adr-0026"} {
		n, _ := f.graph.Node(id)
		f.summaries = append(f.summaries, datasource.MemorySummary{ID: memID(id), Title: n.Title, Version: "g1"})
		f.beads[memID(id)+"@"] = datasource.GraphBead{ID: memID(id), Version: "g1", Properties: datasource.GraphProperties{Title: n.Title, Body: "body of " + id}}
	}
	return f
}

func plainTheme() Theme { return DefaultTheme(lipgloss.DefaultRenderer()) }

func neighbourhoodText(g datasource.MemoryGraph, id string) string {
	return ansi.Strip(strings.Join(renderNeighbourhood(plainTheme(), g, id, 100), "\n"))
}

func TestNeighbourhoodNamesTheRetiredDecisionAndItsReplacement(t *testing.T) {
	text := neighbourhoodText(fixtureMemoryGraph(), "sample-e.1")
	for _, want := range []string{
		"sample-e.1  feature",
		"✗ follows a superseded decision",
		"├━━ follows ━━ ADR 0009 ● superseded Colon prompt",
		"note: built on the colon prompt",
		"└ replaced by ADR 0027",
		"┄┄ cites ┄┄┄ ADR 0027 ● active Flat list",
		"── part of ── sample-e Epic E",
		"── blocks ─── sample-e.2",
		"read-at version: not recorded by the preview",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("neighbourhood lacks %q:\n%s", want, text)
		}
	}
	if strings.Index(text, "follows ━━") > strings.Index(text, "blocks ───") {
		t.Fatalf("outgoing Links must come before incoming ones:\n%s", text)
	}
}

func TestNeighbourhoodOfARetiredMemoryCountsItsOpenFollowers(t *testing.T) {
	text := neighbourhoodText(fixtureMemoryGraph(), "adr-0009")
	for _, want := range []string{"ADR 0009  2026-09-14", "✗ superseded, 1 open Issue still follows it", "━━ followed by sample-e.1", "━━ replaced by ADR 0027"} {
		if !strings.Contains(text, want) {
			t.Fatalf("neighbourhood lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "read-at version") {
		t.Fatalf("a Memory reads nothing, so it has no read-at line:\n%s", text)
	}
}

func TestNeighbourhoodSaysWhenWorkHasNoDecision(t *testing.T) {
	text := neighbourhoodText(fixtureMemoryGraph(), "sample-lone")
	if !strings.Contains(text, "· no decision recorded for this work") || !strings.Contains(text, "(no Links)") {
		t.Fatalf("lone Issue:\n%s", text)
	}
}

func TestBrowserReadsTheGraphOnceForEveryGraphView(t *testing.T) {
	source := newGraphFakeSource()
	b := startBrowser(t, source)
	if source.callCount("graph") != 0 {
		t.Fatal("the list must not read the whole graph")
	}
	for _, step := range []struct{ key, want string }{
		{"2", "Knowledge · Memories"},
		{"3", "legend"},
		{"4", "filled = follows"},
		{"5", "Graph health"},
	} {
		b = press(t, b, step.key)
		if got := screen(b); !strings.Contains(got, step.want) {
			t.Fatalf("view %s lacks %q:\n%s", step.key, step.want, got)
		}
	}
	if source.callCount("graph") != 1 {
		t.Fatalf("graph reads = %d, want 1", source.callCount("graph"))
	}
	b = press(t, b, "1")
	if got := screen(b); !strings.Contains(got, "Memories") || strings.Contains(got, "Work · Issues") {
		t.Fatalf("list view:\n%s", got)
	}
}

func TestBrowserReportsAGraphReadFailure(t *testing.T) {
	source := newGraphFakeSource()
	source.graphErr = errors.New("graph traversal from x is incomplete")
	b := press(t, startBrowser(t, source), "2")
	if got := screen(b); !strings.Contains(got, "graph traversal from x is incomplete") {
		t.Fatalf("error not shown:\n%s", got)
	}
}

func TestGraphProblemsPutRetiredDecisionsFirst(t *testing.T) {
	got := graphProblems(fixtureMemoryGraph())
	want := []string{"sample-e.1", "sample-e.2", "sample-lone"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("problems = %v, want %v", got, want)
	}
}

func TestConstellationDrawsEveryNodeAndTheHealthList(t *testing.T) {
	b := press(t, startBrowser(t, newGraphFakeSource()), "5")
	got := screen(b)
	for _, want := range []string{"●0009", "●0027", "●0026", "◆e", "✗e.1", "?e.2", "·lone", "3 Issues need a look"} {
		if !strings.Contains(got, want) {
			t.Fatalf("constellation lacks %q:\n%s", want, got)
		}
	}
}

func TestConstellationLayoutIsDeterministic(t *testing.T) {
	a := layoutConstellation(fixtureMemoryGraph())
	b := layoutConstellation(fixtureMemoryGraph())
	for id, p := range a {
		if b[id] != p {
			t.Fatalf("%s placed at %v then %v", id, p, b[id])
		}
	}
}

func TestConstellationWalksProblemsAndOpensTheDecision(t *testing.T) {
	source := newGraphFakeSource()
	b := press(t, startBrowser(t, source), "5", "n")
	if got := screen(b); !strings.Contains(got, "✗ follows a superseded decision") {
		t.Fatalf("n must select the first problem:\n%s", got)
	}
	b = press(t, b, "n")
	if got := screen(b); !strings.Contains(got, "? follows a decision that is still proposed") {
		t.Fatalf("second n must select the next problem:\n%s", got)
	}
	b = press(t, b, "N", "enter")
	if got := screen(b); !strings.Contains(got, "ADR 0009  2026-09-14") {
		t.Fatalf("enter on an Issue must select the decision it follows:\n%s", got)
	}
	b = press(t, b, "enter")
	if b.selected.id != memID("adr-0009") || b.mode != memoryModeList {
		t.Fatalf("enter on a Memory must open it in the list: mode %v selected %q", b.mode, b.selected.id)
	}
	if got := screen(b); !strings.Contains(got, "body of adr-0009") {
		t.Fatalf("Memory detail not shown:\n%s", got)
	}
	b = press(t, b, "5", "esc")
	if got := screen(b); !strings.Contains(got, "Graph health") {
		t.Fatalf("esc must clear the selection:\n%s", got)
	}
}

// screenLine is the first screen line that contains marker.
func screenLine(t *testing.T, b MemoryBrowser, marker string) string {
	t.Helper()
	for _, line := range strings.Split(screen(b), "\n") {
		if strings.Contains(line, marker) {
			return line
		}
	}
	t.Fatalf("no line contains %q:\n%s", marker, screen(b))
	return ""
}

func TestFocusWiresDrawOnlyTheSelectedIssuesLinks(t *testing.T) {
	b := press(t, startBrowser(t, newGraphFakeSource()), "2")
	if strings.Contains(screen(b), "▶") {
		t.Fatalf("the epic owns no decision, so no wire may show:\n%s", screen(b))
	}
	b = press(t, b, "j")
	for marker, want := range map[string]string{
		"Dead work":     "┬┄",    // the cites wire leaves the trunk dotted
		"Colon prompt":  "╰─",    // the follows wire ends the trunk with a rounded corner
		"Proposed work": "1→",    // another row shows only a count
		"Graph board":   "Graph", // and its decision draws no wire
	} {
		if line := screenLine(t, b, marker); !strings.Contains(line, want) {
			t.Fatalf("line %q lacks %q", line, want)
		}
	}
	if n := strings.Count(screen(b), "▶"); n != 2 {
		t.Fatalf("the feature owns two Links, got %d arrowheads:\n%s", n, screen(b))
	}
	if line := screenLine(t, b, "Graph board"); strings.Contains(line, "▶") {
		t.Fatalf("an unselected Issue's wire is drawn: %q", line)
	}
}

func TestFocusWiresFollowTheCursorToTheMemorySide(t *testing.T) {
	b := press(t, startBrowser(t, newGraphFakeSource()), "2", "tab")
	if got := screen(b); !strings.Contains(got, "ADR 0027  2026-09-14") {
		t.Fatalf("tab must select the first Memory and show its neighbourhood:\n%s", got)
	}
	if line := screenLine(t, b, "Dead work"); !strings.Contains(line, "┄") {
		t.Fatalf("the Issue citing the selected Memory must be wired: %q", line)
	}
	b = press(t, b, "enter")
	if b.mode != memoryModeList {
		t.Fatal("enter on a Memory must open it in the list")
	}
}

func TestMatrixMarksFollowsAndCitesPerDecisionColumn(t *testing.T) {
	b := press(t, startBrowser(t, newGraphFakeSource()), "3")
	if line := screenLine(t, b, "Work · Issues"); !strings.Contains(line, "27 26 09") {
		t.Fatalf("columns must be the linked decisions by status: %q", line)
	}
	if line := screenLine(t, b, "Dead work"); !strings.Contains(line, "○  ·  ●") {
		t.Fatalf("the feature cites 0027 and follows 0009: %q", line)
	}
	if line := screenLine(t, b, "Proposed work"); !strings.Contains(line, "·  ●  ·") {
		t.Fatalf("the task follows 0026: %q", line)
	}
	b = press(t, b, "j")
	if line := screenLine(t, b, "cell"); !strings.Contains(line, "sample-e.1 cites ADR 0027") {
		t.Fatalf("cell line: %q", line)
	}
	b = press(t, b, "l", "l")
	if line := screenLine(t, b, "cell"); !strings.Contains(line, "sample-e.1 follows ADR 0009") {
		t.Fatalf("cell line after moving right: %q", line)
	}
	if line := screenLine(t, b, "column"); !strings.Contains(line, "followed by 1, cited by 0") {
		t.Fatalf("column line: %q", line)
	}
	b = press(t, b, "enter")
	if b.mode != memoryModeList {
		t.Fatal("enter must open the column's Memory")
	}
}

func TestMatrixSortKeepsTheSelectedDecision(t *testing.T) {
	f := newGraphFakeSource()
	edges := append([]datasource.GraphEdge{}, f.graph.Edges...)
	edges = append(edges, datasource.GraphEdge{ID: "extra", Kind: datasource.EdgeCites, Source: "sample-lone", Target: "adr-0009"})
	f.graph = datasource.NewMemoryGraph(f.graph.Nodes, edges)
	b := press(t, startBrowser(t, f), "3", "s")
	if line := screenLine(t, b, "Work · Issues"); !strings.Contains(line, "09 27 26") {
		t.Fatalf("usage order: %s", line)
	}
	if line := screenLine(t, b, "cell"); !strings.Contains(line, "ADR 0027") {
		t.Fatalf("sort changed selection: %s", line)
	}
	b = press(t, b, "s")
	if line := screenLine(t, b, "Work · Issues"); !strings.Contains(line, "27 26 09") {
		t.Fatalf("status order: %s", line)
	}
}

func TestMatrixProblemsFilterKeepsSelectedIssue(t *testing.T) {
	b := press(t, startBrowser(t, newGraphFakeSource()), "3", "j", "p")
	if strings.Contains(screen(b), "Epic E") {
		t.Fatalf("problem filter retained healthy epic:\n%s", screen(b))
	}
	if line := screenLine(t, b, "cell"); !strings.Contains(line, "sample-e.1") {
		t.Fatalf("filter changed selected issue: %s", line)
	}
	b = press(t, b, "p")
	if !strings.Contains(screen(b), "Epic E") {
		t.Fatal("second p must restore all issues")
	}
}

func TestMatrixEmptyGraphExplainsMissingLinks(t *testing.T) {
	f := newGraphFakeSource()
	f.graph = datasource.NewMemoryGraph(nil, nil)
	b := press(t, startBrowser(t, f), "3", "p", "s", "G", "l", "enter")
	if !strings.Contains(screen(b), "No decision Links") {
		t.Fatalf("empty matrix needs an explanation:\n%s", screen(b))
	}
}

func TestMatrixLastSelectedIssueRemainsVisible(t *testing.T) {
	f := newGraphFakeSource()
	nodes := append([]datasource.GraphNode{}, f.graph.Nodes...)
	for i := 0; i < 30; i++ {
		id := fmt.Sprintf("extra-%02d", i)
		nodes = append(nodes, datasource.GraphNode{ID: id, Kind: "issue", Title: id, Issue: model.Issue{ID: id}})
	}
	f.graph = datasource.NewMemoryGraph(nodes, f.graph.Edges)
	b := press(t, startBrowser(t, f), "3")
	b, _ = b.Update(tea.WindowSizeMsg{Width: 80, Height: 18})
	b = press(t, b, "G")
	if line := screenLine(t, b, "extra-29"); strings.Contains(line, "cell") {
		t.Fatalf("selected issue appears only in detail: %s", line)
	}
}

func TestDecisionRowTruncationBeforeHealthGlyph(t *testing.T) {
	text := workRowText(newGraphStyles(plainTheme()), fixtureMemoryGraph(), graphRow{id: "sample-e.1", depth: 20}, 20, false, false)
	if ansi.StringWidth(text) > 20 {
		t.Fatalf("deep row exceeds width: %q", text)
	}
}

func TestChipsFitsVeryNarrowTerminals(t *testing.T) {
	for _, width := range []int{20, 24, 26, 40} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			b := press(t, startBrowser(t, newGraphFakeSource()), "4")
			b, _ = b.Update(tea.WindowSizeMsg{Width: width, Height: 16})
			for _, line := range strings.Split(screen(b), "\n") {
				if ansi.StringWidth(line) > width {
					t.Fatalf("line exceeds terminal: %q", line)
				}
			}
		})
	}
}

func TestMatrixExplainsMinimumTerminalWidth(t *testing.T) {
	b := press(t, startBrowser(t, newGraphFakeSource()), "3")
	b, _ = b.Update(tea.WindowSizeMsg{Width: 20, Height: 16})
	if !strings.Contains(screen(b), "Widen terminal") {
		t.Fatalf("invisible cells need a width notice:\n%s", screen(b))
	}
}

func TestFocusKeepsSelectionVisibleAfterResize(t *testing.T) {
	b := press(t, startBrowser(t, newGraphFakeSource()), "4", "G", "2")
	b, _ = b.Update(tea.WindowSizeMsg{Width: 100, Height: 10})
	view := ansi.Strip(b.focusView(98, 6))
	list := strings.Split(view, strings.Repeat("─", 98))[0]
	if !strings.Contains(list, "Lone work") {
		t.Fatalf("selection is absent from list: %q", list)
	}
}

func TestChipsShowsSelectedDecisionAfterHorizontalNavigation(t *testing.T) {
	f := newGraphFakeSource()
	nodes := append([]datasource.GraphNode{}, f.graph.Nodes...)
	edges := append([]datasource.GraphEdge{}, f.graph.Edges...)
	for i := 0; i < 20; i++ {
		id := fmt.Sprintf("adr-%04d", 100+i)
		nodes = append(nodes, datasource.GraphNode{ID: id, Kind: "memory", Number: fmt.Sprintf("%04d", 100+i), Title: "Decision " + id, Status: "active"})
		edges = append(edges, datasource.GraphEdge{ID: "link-" + id, Kind: datasource.EdgeFollows, Source: "sample-e", Target: id})
	}
	f.graph = datasource.NewMemoryGraph(nodes, edges)
	b := press(t, startBrowser(t, f), "4")
	b, _ = b.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	for i := 0; i < 19; i++ {
		b = press(t, b, "l")
	}
	if line := screenLine(t, b, "Epic E"); !strings.Contains(line, "0119") {
		t.Fatalf("selected chip hidden: %q", line)
	}
	if !strings.Contains(screen(b), "Decision adr-0119") {
		t.Fatalf("selected decision detail hidden:\n%s", screen(b))
	}
}

func TestChipsShowEachIssuesDecisionsAndADetailBox(t *testing.T) {
	b := press(t, startBrowser(t, newGraphFakeSource()), "4", "j")
	for marker, want := range map[string]string{
		"Dead work":     "0009✗  (0027)",
		"Proposed work": "0026?",
		"Lone work":     "no decision",
	} {
		if line := screenLine(t, b, marker); !strings.Contains(line, want) {
			t.Fatalf("line %q lacks %q", line, want)
		}
	}
	got := screen(b)
	for _, want := range []string{"╭", "“built on the colon prompt”", "follows · superseded", "cites · active"} {
		if !strings.Contains(got, want) {
			t.Fatalf("detail box lacks %q:\n%s", want, got)
		}
	}
	b = press(t, b, "l", "enter")
	if b.mode != memoryModeList || !strings.Contains(screen(b), "body of adr-0027") {
		t.Fatalf("l then enter must open the second chip's Memory:\n%s", screen(b))
	}
}

func TestConstellationArrowKeysMoveToANeighbourInThatDirection(t *testing.T) {
	g := fixtureMemoryGraph()
	pos := layoutConstellation(g)
	from := "sample-e"
	next := nearestInDirection(pos, from, 1, 0)
	if next == "" {
		return // nothing lies to the right of the epic in this layout
	}
	if pos[next][0] <= pos[from][0] {
		t.Fatalf("moving right from %v reached %s at %v", pos[from], next, pos[next])
	}
}

func TestGraphRowsKeepTheEpicTreeAndGroupDecisionsByStatus(t *testing.T) {
	g := fixtureMemoryGraph()
	left, right := workRows(g), decisionRows(g)
	var l, r []string
	for _, row := range left {
		l = append(l, row.header+row.id)
	}
	for _, row := range right {
		r = append(r, row.header+row.id)
	}
	if got := strings.Join(l, ","); got != "sample-e,sample-e.1,sample-e.2,No epic,sample-lone" {
		t.Fatalf("work rows = %s", got)
	}
	if got := strings.Join(r, ","); got != "active,adr-0027,proposed,adr-0026,superseded,adr-0009" {
		t.Fatalf("decision rows = %s", got)
	}
}

func TestGraphViewsFitASmallTerminal(t *testing.T) {
	source := newGraphFakeSource()
	b := NewMemoryBrowser(source, "b9s-memory-poc")
	b, _ = b.Update(tea.WindowSizeMsg{Width: 60, Height: 16})
	b = drive(t, b, b.Init())
	for _, keys := range [][]string{{"2"}, {"j", "tab", "j"}, {"3"}, {"j", "l"}, {"4"}, {"j", "l"}, {"5"}, {"n"}, {"t"}} {
		b = press(t, b, keys...)
		lines := strings.Split(screen(b), "\n")
		if len(lines) > 16 {
			t.Fatalf("after %v the view has %d lines for 16 rows", keys, len(lines))
		}
		for _, line := range lines {
			if w := ansi.StringWidth(line); w > 60 {
				t.Fatalf("after %v a line is %d cells wide for 60: %q", keys, w, line)
			}
		}
	}
}

func TestGraphTitleDropsTheIdTag(t *testing.T) {
	n := datasource.GraphNode{Kind: "issue", Title: "[sample-db5q.2] Snapshot: display"}
	if got := graphTitle(n); got != "Snapshot: display" {
		t.Fatalf("title = %q", got)
	}
}

func TestDecisionSectionShowsOnlyDecisionLinksBelowTheIssueCard(t *testing.T) {
	g := fixtureMemoryGraph()
	text := stripANSI(strings.Join(decisionSection(plainTheme(), g, "sample-e.1", 80), "\n"))
	for _, want := range []string{"✗ follows a superseded decision", "follows", "ADR 0009", "note: built on the colon prompt", "replaced by ADR 0027", "cites", "read-at version"} {
		if !strings.Contains(text, want) {
			t.Fatalf("section lacks %q:\n%s", want, text)
		}
	}
	for _, unwanted := range []string{"Dead work", "part of", "blocks", "waits on"} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("section repeats the card or the relations (%q):\n%s", unwanted, text)
		}
	}
	if s := decisionSection(plainTheme(), g, "sample-e", 80); s != nil {
		t.Fatalf("an epic's decisions live on its children: %q", s)
	}
	if s := stripANSI(strings.Join(decisionSection(plainTheme(), g, "sample-lone", 80), "\n")); !strings.Contains(s, "no decision recorded") {
		t.Fatalf("a gap must be visible: %q", s)
	}
}

func TestIssueDetailCarriesTheDecisionSection(t *testing.T) {
	theme := plainTheme()
	g := fixtureMemoryGraph()
	n, _ := g.Node("sample-e.2")
	out := stripANSI(renderIssueDetail(n.Issue, nil, theme, 70, nil, nil, decisionSection(theme, g, "sample-e.2", 70)))
	if !strings.Contains(out, "DECISIONS") || !strings.Contains(out, "ADR 0026") {
		t.Fatalf("detail lacks the decision section:\n%s", out)
	}
	if out := stripANSI(renderIssueDetail(n.Issue, nil, theme, 70, nil, nil, nil)); strings.Contains(out, "DECISIONS") {
		t.Fatalf("no graph, no section:\n%s", out)
	}
}
