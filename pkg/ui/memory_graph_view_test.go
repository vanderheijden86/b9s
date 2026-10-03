package ui

import (
	"errors"
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

func TestBrowserReadsTheGraphOnceForBothGraphViews(t *testing.T) {
	source := newGraphFakeSource()
	b := startBrowser(t, source)
	if source.callCount("graph") != 0 {
		t.Fatal("the list must not read the whole graph")
	}
	b = press(t, b, "2")
	if got := screen(b); !strings.Contains(got, "Graph health") {
		t.Fatalf("constellation view:\n%s", got)
	}
	b = press(t, b, "3")
	if got := screen(b); !strings.Contains(got, "Work · Issues") || !strings.Contains(got, "Knowledge · Memories") {
		t.Fatalf("two shores view:\n%s", got)
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
	b := press(t, startBrowser(t, newGraphFakeSource()), "2")
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
	b := press(t, startBrowser(t, source), "2", "n")
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
	b = press(t, b, "2", "esc")
	if got := screen(b); !strings.Contains(got, "Graph health") {
		t.Fatalf("esc must clear the selection:\n%s", got)
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

func TestShoresRowsKeepTheEpicTreeAndGroupDecisionsByStatus(t *testing.T) {
	left, right := shoresRows(fixtureMemoryGraph())
	var l, r []string
	for _, row := range left {
		l = append(l, row.header+row.id)
	}
	for _, row := range right {
		r = append(r, row.header+row.id)
	}
	if got := strings.Join(l, ","); got != "sample-e,sample-e.1,sample-e.2,No epic,sample-lone" {
		t.Fatalf("left shore = %s", got)
	}
	if got := strings.Join(r, ","); got != "active,adr-0027,proposed,adr-0026,superseded,adr-0009" {
		t.Fatalf("right shore = %s", got)
	}
}

func TestShoresDrawWiresAndTraceTheSelectedRow(t *testing.T) {
	b := press(t, startBrowser(t, newGraphFakeSource()), "3")
	got := screen(b)
	for _, want := range []string{"1f 0c", "0f 1c", "▸", "◂"} {
		if !strings.Contains(got, want) {
			t.Fatalf("shores lack %q:\n%s", want, got)
		}
	}
	b = press(t, b, "j")
	if got := screen(b); !strings.Contains(got, "✗ follows a superseded decision") {
		t.Fatalf("j must select the feature and show its neighbourhood:\n%s", got)
	}
	b = press(t, b, "tab")
	if got := screen(b); !strings.Contains(got, "ADR 0027  2026-09-14") {
		t.Fatalf("tab must move to the Memory shore:\n%s", got)
	}
	traced := shoresTraced(fixtureMemoryGraph(), "sample-e")
	for _, id := range []string{"sample-e", "sample-e.1", "sample-e.2", "adr-0009", "adr-0027", "adr-0026"} {
		if !traced[id] {
			t.Fatalf("tracing the epic must reach %s: %v", id, traced)
		}
	}
	if traced["sample-lone"] {
		t.Fatal("tracing the epic must not reach unrelated work")
	}
}

func TestGraphViewsFitASmallTerminal(t *testing.T) {
	source := newGraphFakeSource()
	b := NewMemoryBrowser(source, "b9s-memory-poc")
	b, _ = b.Update(tea.WindowSizeMsg{Width: 60, Height: 16})
	b = drive(t, b, b.Init())
	for _, keys := range [][]string{{"2"}, {"n"}, {"t"}, {"3"}, {"j", "tab", "j"}} {
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

func TestLanesNeededCountsOverlappingWires(t *testing.T) {
	wires := []shoreWire{{from: 0, to: 4}, {from: 1, to: 2}, {from: 3, to: 3}, {from: 5, to: 6}}
	if got := lanesNeeded(wires); got != 2 {
		t.Fatalf("lanes = %d, want 2: the third wire reuses the second wire's lane", got)
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
