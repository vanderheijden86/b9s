package ui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/vanderheijden86/b9s/internal/datasource"
	"github.com/vanderheijden86/b9s/pkg/model"
)

func fixtureMemoryGraph() datasource.MemoryGraph {
	issue := func(id, title string, typ model.IssueType) datasource.GraphNode {
		return datasource.GraphNode{ID: id, Kind: "issue", Title: title, Status: "open", Issue: model.Issue{ID: id, Title: title, IssueType: typ, Status: model.StatusOpen, Priority: 2}}
	}
	return datasource.NewMemoryGraph([]datasource.GraphNode{
		{ID: "adr-0009", Kind: "memory", Title: "Colon prompt", Body: "---\nstatus: superseded\n---\nLiteral body"},
		{ID: "adr-0027", Kind: "memory", Title: "Flat list"},
		{ID: "adr-0026", Kind: "memory", Title: "Graph board"},
		issue("sample-e", "Epic E", model.TypeEpic), issue("sample-e.1", "Work one", model.TypeFeature), issue("sample-e.2", "Work two", model.TypeTask), issue("sample-lone", "Lone work", model.TypeTask),
	}, []datasource.GraphEdge{
		{ID: "c1", Kind: datasource.EdgeRelated, Source: "sample-e.1", Target: "sample-e"},
		{ID: "f1", Kind: datasource.EdgeFollows, Source: "sample-e.1", Target: "adr-0009", Note: "built on the colon prompt"},
		{ID: "x1", Kind: datasource.EdgeCites, Source: "sample-e.1", Target: "adr-0027"},
		{ID: "f2", Kind: datasource.EdgeFollows, Source: "sample-e.2", Target: "adr-0026"},
		{ID: "r1", Kind: datasource.EdgeRelated, Source: "adr-0027", Target: "adr-0009"},
		{ID: "in", Kind: datasource.EdgeRelated, Source: "adr-0026", Target: "sample-e"},
		{ID: "b1", Kind: datasource.EdgeBlocks, Source: "sample-e.2", Target: "sample-e.1"},
	})
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

func TestMemoryGraphViewKeysAndExpandedDetail(t *testing.T) {
	b := startBrowser(t, newGraphFakeSource())
	b = press(t, b, "2", "V", "v")
	if !strings.Contains(screen(b), "Memory wires") {
		t.Fatal("view keys must not leave wires for a terminal network")
	}
	b = press(t, b, "\\")
	if !strings.Contains(screen(b), "Expanded detail") {
		t.Fatal("backslash must expand detail")
	}
	b = press(t, b, "\\", "V")
	if b.mode != memoryModeFocus {
		t.Fatal("V must keep wires")
	}
}

func TestMemoryGraphStatusAndSearchFilterNativeRecords(t *testing.T) {
	b := startBrowser(t, newGraphFakeSource())
	b = press(t, b, "2", "C")
	if _, ok := b.graph.Node("sample-e.1"); ok {
		t.Fatal("closed filter retained an open Issue")
	}
	if _, ok := b.graph.Node("adr-0009"); !ok {
		t.Fatal("status filter must retain Memories without status")
	}
	b = press(t, b, "A", "/", "C", "o", "l", "o", "n", "enter")
	if len(b.graph.Nodes) != 1 || b.graph.Nodes[0].ID != "adr-0009" {
		t.Fatalf("literal search must hide nonmatching beads: %+v", b.graph.Nodes)
	}
	b = press(t, b, "/", "backspace", "backspace", "backspace", "backspace", "backspace", "enter")
	if len(b.graph.Nodes) != len(fixtureMemoryGraph().Nodes) {
		t.Fatal("clearing search must restore all beads")
	}
}

func TestIntegratedMemoriesReturnToMainAndKeepState(t *testing.T) {
	m := newBulkMarkModel(t)
	b := startBrowser(t, newGraphFakeSource())
	b = press(t, b, "2", "j")
	m.memoryBrowser = &b
	m, _ = pressBulkKey(t, m, runeKey("M"))
	if !m.memoryVisible || !strings.Contains(m.View(), "Memory wires") {
		t.Fatal("M did not open embedded Memories")
	}
	m, _ = pressBulkKey(t, m, runeKey("\\"))
	if !strings.Contains(m.View(), "Expanded detail") {
		t.Fatal("detail did not expand")
	}
	m, _ = pressBulkKey(t, m, runeKey("\\"))
	m, _ = pressBulkKey(t, m, runeKey("V"))
	if m.memoryBrowser.mode != memoryModeFocus {
		t.Fatal("V did not select wires")
	}
	m, cmd := pressBulkKey(t, m, runeKey("q"))
	if m.memoryVisible || cmd != nil {
		t.Fatal("q must return to main without quitting")
	}
	m, _ = pressBulkKey(t, m, runeKey("M"))
	if m.memoryBrowser.mode != memoryModeFocus {
		t.Fatal("reopening lost the view")
	}
}

func TestIntegratedMemoryDiscardsPreviousProjectReplies(t *testing.T) {
	m := newBulkMarkModel(t)
	b := startBrowser(t, newGraphFakeSource())
	m.memoryBrowser, m.memoryGeneration = &b, 2
	updated, _ := m.Update(embeddedMemoryMsg{generation: 1, msg: memoryGraphMsg{err: errors.New("previous project")}})
	if updated.(Model).memoryBrowser.graphErr != nil {
		t.Fatal("stale project reply applied")
	}
}

func TestMemoryWiresReportIncompatibleCLIWithCachedGraph(t *testing.T) {
	b := startBrowser(t, newGraphFakeSource())
	b = press(t, b, "2")
	b, _ = b.Update(memorySummariesMsg{err: datasource.ErrBDNotGraphPreview})
	if !strings.Contains(screen(b), "Memory commands unavailable") {
		t.Fatal("cached graph concealed unsupported bd")
	}
	b = press(t, b, "\\")
	if !strings.Contains(screen(b), "Memory commands unavailable") {
		t.Fatal("expanded detail concealed capability failure")
	}
}

func TestExpandedMemoryDetailReflowsAndScrolls(t *testing.T) {
	b := startBrowser(t, newGraphFakeSource())
	b.rendered[b.selected] = "cached narrow body"
	b = press(t, b, "\\")
	if len(b.rendered) != 0 {
		t.Fatal("expanded detail must invalidate narrow body cache")
	}
	b = press(t, b, "J")
	if b.scroll == 0 {
		t.Fatal("J must scroll expanded list detail")
	}
}

func TestMemoryWiresIgnoreGraphViewKeys(t *testing.T) {
	b := startBrowser(t, newGraphFakeSource())
	b = press(t, b, "2", "j")
	id := b.graphSelected()
	b = press(t, b, "V")
	if b.graphSelected() != id || b.mode != memoryModeFocus {
		t.Fatal("V must preserve wires and selection")
	}
	b = press(t, b, "V")
	if b.graphSelected() != id || b.mode != memoryModeFocus {
		t.Fatal("repeated V must preserve wires and selection")
	}
}
func neighbourhoodText(g datasource.MemoryGraph, id string) string {
	return ansi.Strip(strings.Join(renderNeighbourhood(plainTheme(), g, id, 120), "\n"))
}
func TestNeighbourhoodUsesStoredRelationships(t *testing.T) {
	text := neighbourhoodText(fixtureMemoryGraph(), "sample-e.1")
	for _, want := range []string{"follows", "cites", "related", "blocks", "Colon prompt", "note: built on the colon prompt"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %s:\n%s", want, text)
		}
	}
	for _, bad := range []string{"superseded", "active", "no status", "part of", "replaced by", "no decision"} {
		if strings.Contains(text, bad) {
			t.Fatalf("invented semantics %s:\n%s", bad, text)
		}
	}
}
func TestMemoryTitleIsLiteral(t *testing.T) {
	title := "[m] ADR 0001 (proposed): Title"
	if memoryShortTitle(title) != title {
		t.Fatal("Memory titles must remain literal")
	}
}
func TestIssueDetailIncludesIncomingMemoryLinksForEpics(t *testing.T) {
	g := fixtureMemoryGraph()
	lines := decisionSection(plainTheme(), g, "sample-e", 100)
	text := stripANSI(strings.Join(lines, "\n"))
	if !strings.Contains(text, "── related ──") || !strings.Contains(text, "Graph board") {
		t.Fatalf("incoming Memory link missing: %s", text)
	}
	n, _ := g.Node("sample-e")
	detail := stripANSI(renderIssueDetail(n.Issue, nil, plainTheme(), 100, nil, nil, lines))
	if !strings.Contains(detail, "MEMORY LINKS") {
		t.Fatalf("section missing: %s", detail)
	}
}
func TestIssueDetailOmitsIssueToIssueEdgesFromMemorySection(t *testing.T) {
	text := stripANSI(strings.Join(decisionSection(plainTheme(), fixtureMemoryGraph(), "sample-e.1", 100), "\n"))
	if strings.Contains(text, "blocks") || strings.Contains(text, "Epic E") {
		t.Fatalf("Issue dependencies leaked: %s", text)
	}
}
func TestIssueWithoutMemoriesHasNeutralEmptySection(t *testing.T) {
	text := stripANSI(strings.Join(decisionSection(plainTheme(), fixtureMemoryGraph(), "sample-lone", 80), "\n"))
	if text != "No Memory Links recorded." {
		t.Fatal(text)
	}
}
func TestBrowserReadsGraphOnceAndHasOnlyListAndWires(t *testing.T) {
	source := newGraphFakeSource()
	b := startBrowser(t, source)
	if source.callCount("graph") != 0 {
		t.Fatal("eager graph read")
	}
	for _, key := range []string{"2", "3", "4", "2"} {
		b = press(t, b, key)
		if b.mode != memoryModeFocus {
			t.Fatalf("%s changed mode", key)
		}
	}
	if source.callCount("graph") != 1 {
		t.Fatal("graph reread")
	}
	if strings.Contains(b.footer(), "matrix") || strings.Contains(b.footer(), "chips") {
		t.Fatal(b.footer())
	}
	b = press(t, b, "1")
	if b.mode != memoryModeList {
		t.Fatal("list unavailable")
	}
}
func TestBrowserReportsGraphReadFailure(t *testing.T) {
	source := newGraphFakeSource()
	source.graphErr = errors.New("incomplete traversal")
	b := press(t, startBrowser(t, source), "2")
	if !strings.Contains(screen(b), "incomplete traversal") {
		t.Fatal(screen(b))
	}
}
func largeFocusBrowser(t *testing.T, allLinked bool) MemoryBrowser {
	nodes := []datasource.GraphNode{{ID: "work", Kind: "issue", Title: "Selected work", Issue: model.Issue{ID: "work", Title: "Selected work", Status: model.StatusOpen}}}
	var edges []datasource.GraphEdge
	for i := 0; i < 80; i++ {
		id := fmt.Sprintf("m%02d", i)
		nodes = append(nodes, datasource.GraphNode{ID: id, Kind: "memory", Title: fmt.Sprintf("Memory %02d", i)})
		if allLinked || i == 0 || i == 79 {
			edges = append(edges, datasource.GraphEdge{ID: id, Kind: datasource.EdgeRelated, Source: "work", Target: id})
		}
	}
	b := startBrowser(t, newGraphFakeSource()).applyGraph(memoryGraphMsg{graph: datasource.NewMemoryGraph(nodes, edges)})
	b.mode = memoryModeFocus
	b.width, b.height = 120, 24
	return b
}
func TestFocusKeepsDistantLinkedMemoriesVisible(t *testing.T) {
	b := largeFocusBrowser(t, false)
	b, _ = b.handleDecisionKey("shift+tab")
	view := ansi.Strip(b.focusView(116, 20))
	if !strings.Contains(view, "Memory 00") || !strings.Contains(view, "Memory 79") || !strings.Contains(view, "78 unrelated hidden · shift+tab expand") {
		t.Fatalf("endpoints must survive compact viewport:\n%s", view)
	}
	if len(strings.Split(view, "\n")) > 20 {
		t.Fatalf("height overflow:\n%s", view)
	}
}
func TestFocusShowsContextByDefault(t *testing.T) {
	b := largeFocusBrowser(t, false)
	view := ansi.Strip(b.focusView(116, 20))
	if b.decide.collapsed || !strings.Contains(view, "Memory 01") || strings.Contains(view, "unrelated hidden") {
		t.Fatalf("context must be expanded by default:\n%s", view)
	}
}
func TestFocusShiftTabTogglesContext(t *testing.T) {
	b := largeFocusBrowser(t, false)
	b, _ = b.handleDecisionKey("shift+tab")
	if !b.decide.collapsed || strings.Contains(ansi.Strip(b.focusView(116, 20)), "Memory 01") {
		t.Fatal("shift+tab must collapse context")
	}
	if !strings.Contains(ansi.Strip(b.focusView(116, 20)), "Memory 79") {
		t.Fatal("compaction lost last endpoint")
	}
	b, _ = b.handleDecisionKey("shift+tab")
	if b.decide.collapsed || !strings.Contains(ansi.Strip(b.focusView(116, 20)), "Memory 01") {
		t.Fatal("shift+tab must expand context again")
	}
}
func TestFocusLinkedOverflowCanBePaged(t *testing.T) {
	b := largeFocusBrowser(t, true)
	seen := map[string]bool{}
	for page := 0; page < 20; page++ {
		b.decide.page = page
		text := ansi.Strip(b.focusView(116, 20))
		for i := 0; i < 80; i++ {
			id := fmt.Sprintf("Memory %02d", i)
			if strings.Contains(text, id) {
				seen[id] = true
			}
		}
	}
	if len(seen) != 80 {
		t.Fatalf("only %d/80 endpoints reachable", len(seen))
	}
}
func TestFocusRenderFitsResizes(t *testing.T) {
	b := largeFocusBrowser(t, true)
	for _, size := range [][2]int{{160, 40}, {80, 24}, {48, 10}, {32, 8}} {
		view := b.focusView(size[0], size[1])
		if lipgloss.Height(view) > size[1] {
			t.Fatalf("height exceeds %v", size)
		}
		for _, line := range strings.Split(view, "\n") {
			if ansi.StringWidth(line) > size[0] {
				t.Fatalf("width exceeds %v: %s", size, line)
			}
		}
	}
}
func TestFocusKeyboardCanReachLastMemory(t *testing.T) {
	b := largeFocusBrowser(t, false)
	b, _ = b.handleDecisionKey("tab")
	b, _ = b.handleDecisionKey("end")
	if b.decisionSelected() != "m79" {
		t.Fatal(b.decisionSelected())
	}
	if !strings.Contains(ansi.Strip(b.focusView(116, 20)), "m79") {
		t.Fatal("cursor hidden")
	}
}

// The wires use the UML connector of each Link kind, as the web graph does,
// in heavy strokes. The head points at the Link's target on either side.
func TestWiresDrawUMLConnectors(t *testing.T) {
	for _, c := range []struct {
		kind              string
		stroke            string
		head, reverseHead string
	}{
		{datasource.EdgeFollows, "┅", "▷", "◁"},
		{datasource.EdgeBlocks, "┅", ">", "<"},
		{datasource.EdgeCites, "━", ">", "<"},
		{datasource.EdgeRelated, "━", "", ""},
		{"supersedes", "━", ">", "<"},
	} {
		for _, reverse := range []bool{false, true} {
			for _, side := range []int{0, 1} {
				source, target := "i", "m"
				if reverse {
					source, target = target, source
				}
				b := startBrowser(t, newGraphFakeSource()).applyGraph(memoryGraphMsg{graph: datasource.NewMemoryGraph(
					[]datasource.GraphNode{{ID: "i", Kind: "issue", Title: "Work"}, {ID: "m", Kind: "memory", Title: "Memory"}},
					[]datasource.GraphEdge{{ID: "link", Source: source, Target: target, Kind: c.kind}},
				)})
				b.decide.side = side
				line := lineWith(t, b.focusView(140, 20), "○ i Work")
				head := c.head
				if reverse {
					head = c.reverseHead
				}
				if !strings.Contains(line, c.stroke+c.stroke) {
					t.Errorf("%s reverse=%v side=%d: %q lacks the %q line", c.kind, reverse, side, line, c.stroke)
				}
				heads := strings.Count(line, "▷") + strings.Count(line, "◁") + strings.Count(line, ">") + strings.Count(line, "<")
				if head == "" && heads != 0 || head != "" && (heads != 1 || !strings.Contains(line, head)) {
					t.Errorf("%s reverse=%v side=%d: %q, want head %q", c.kind, reverse, side, line, head)
				}
			}
		}
	}
}
func TestNeighbourhoodDrawsUMLConnectors(t *testing.T) {
	g := fixtureMemoryGraph()
	text := neighbourhoodText(g, "sample-e.1") + "\n" + neighbourhoodText(g, "adr-0009")
	for _, want := range []string{"╌╌ follows ╌▷", "◁╌ followed by", "── cites ─>", "── related ──", "<╌ blocks ╌╌"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q:\n%s", want, text)
		}
	}
}
func TestFocusLegendNamesUMLConnectors(t *testing.T) {
	view := ansi.Strip(startBrowser(t, newGraphFakeSource()).applyGraph(memoryGraphMsg{graph: fixtureMemoryGraph()}).focusView(140, 24))
	for _, want := range []string{"follows ┅▷ realization", "depends on ┅> dependency", "cites ━> directed association", "related ━━ association"} {
		if !strings.Contains(view, want) {
			t.Errorf("legend lacks %q:\n%s", want, view)
		}
	}
}

func TestFocusWiresConnectToIssueText(t *testing.T) {
	for _, side := range []int{0, 1} {
		for _, reverse := range []bool{false, true} {
			for _, title := range []string{"Short", "界面", strings.Repeat("Long title ", 30)} {
				t.Run(fmt.Sprintf("side=%d/reverse=%v/title=%s", side, reverse, title[:min(len(title), 12)]), func(t *testing.T) {
					source, target := "i", "m"
					if reverse {
						source, target = target, source
					}
					b := startBrowser(t, newGraphFakeSource()).applyGraph(memoryGraphMsg{graph: datasource.NewMemoryGraph(
						[]datasource.GraphNode{{ID: "i", Kind: "issue", Title: title}, {ID: "m", Kind: "memory", Title: "Memory"}},
						[]datasource.GraphEdge{{ID: "link", Source: source, Target: target, Kind: datasource.EdgeCites}},
					)})
					b.decide.side = side
					line := ansi.Strip(strings.Split(b.focusView(120, 20), "\n")[1])
					leftWidth := (120 - 20) / 2
					label := truncate("○ i "+title, leftWidth)
					gap := ansi.Cut(line, ansi.StringWidth(label), leftWidth+2)
					if strings.Contains(strings.TrimPrefix(gap, " "), " ") {
						t.Fatalf("wire detached from Issue: %q", line)
					}
					// With an Issue selected its run is shared, so each Memory run carries the head.
					if reverse && side == 1 && !strings.HasPrefix(strings.TrimPrefix(gap, " "), "<") {
						t.Fatalf("reverse arrow must touch Issue endpoint: %q", line)
					}
					if strings.Count(line, "<")+strings.Count(line, ">") != 1 {
						t.Fatalf("expected one directional arrow: %q", line)
					}
				})
			}
		}
	}
}
func TestMemoryBrowserPointsToWebGraph(t *testing.T) {
	b := startBrowser(t, newGraphFakeSource())
	if !strings.Contains(b.footer(), "b9s web") {
		t.Fatal(b.footer())
	}
	b = press(t, b, "5")
	if b.mode != memoryModeList {
		t.Fatal("unexpected graph mode")
	}
}
func TestFocusEmptyGraph(t *testing.T) {
	b := startBrowser(t, newGraphFakeSource()).applyGraph(memoryGraphMsg{})
	b.mode = memoryModeFocus
	for _, key := range []string{"j", "k", "end", "tab", "]", "enter"} {
		b, _ = b.handleDecisionKey(key)
	}
	view := b.focusView(80, 20)
	if !strings.Contains(view, "No record selected") {
		t.Fatal(view)
	}
}
func TestFocusReceivesWindowSize(t *testing.T) {
	b := largeFocusBrowser(t, false)
	m, _ := b.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	view := m.View()
	if lipgloss.Width(view) > 80 || lipgloss.Height(view) > 24 {
		t.Fatalf("screen overflow %dx%d", lipgloss.Width(view), lipgloss.Height(view))
	}
}

func TestFocusPagingStopsAtLastPage(t *testing.T) {
	b := largeFocusBrowser(t, true)
	for range 100 {
		b, _ = b.handleDecisionKey("]")
	}
	last := b.focusView(b.width-2, b.height-4)
	b, _ = b.handleDecisionKey("[")
	if b.focusView(b.width-2, b.height-4) == last {
		t.Fatal("one previous-page key must leave the last page")
	}
}

func TestFocusDetailScrollStopsAtLastLine(t *testing.T) {
	b := largeFocusBrowser(t, true)
	for range 200 {
		b, _ = b.handleDecisionKey("J")
	}
	last := b.focusView(b.width-2, b.height-4)
	b, _ = b.handleDecisionKey("K")
	if b.focusView(b.width-2, b.height-4) == last {
		t.Fatal("one scroll-up key must leave the final detail line")
	}
}

// Pressing M where Memories cannot open says why in one line and opens
// nothing: the full diagnostic belongs to read failures, not to absence.
func TestMemoryKeyExplainsUnavailableMemoriesInOneLine(t *testing.T) {
	cases := []struct {
		name, meta, bd, want string
	}{
		{"ordinary project", `{"backend":"dolt"}`, "", "not a Memory graph workspace"},
		{"stock bd", `{"graph_mode":"link","graph_ready":true}`,
			"#!/bin/sh\ncase \"$1\" in versions|links) exit 1 ;; esac\nexit 0\n", "has no Memory Beads support"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			project := t.TempDir()
			if err := os.MkdirAll(filepath.Join(project, ".beads"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(project, ".beads", "metadata.json"), []byte(c.meta), 0o644); err != nil {
				t.Fatal(err)
			}
			bin := t.TempDir()
			if c.bd != "" {
				if err := os.WriteFile(filepath.Join(bin, "bd"), []byte(c.bd), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("PATH", bin+string(os.PathListSeparator)+"/usr/bin:/bin")
			t.Setenv("XDG_CACHE_HOME", t.TempDir())
			m := newBulkMarkModel(t)
			m.activeProjectPath = project

			m, _ = pressBulkKey(t, m, runeKey("M"))

			if m.memoryVisible {
				t.Fatal("M opened Memories, want only a status line")
			}
			if !strings.HasPrefix(m.statusMsg, "Memories unavailable: ") || !strings.Contains(m.statusMsg, c.want) {
				t.Errorf("status = %q, want 'Memories unavailable: ...%s'", m.statusMsg, c.want)
			}
			if strings.Contains(m.statusMsg, "Details:") || strings.Contains(m.statusMsg, "\n") {
				t.Errorf("status = %q, want one line without the diagnostic", m.statusMsg)
			}
		})
	}
}
func TestFocusShiftTabKeyReachesWires(t *testing.T) {
	b := largeFocusBrowser(t, false)
	b, _ = b.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if !b.decide.collapsed {
		t.Fatal("Shift+Tab key event must collapse wire context")
	}
}

// wiresFanGraph is the mockup's case: three Issues reach adr-0019 by Links of
// different kinds, and i2 also depends on adr-0020.
func wiresFanGraph() datasource.MemoryGraph {
	issue := func(id, title string) datasource.GraphNode {
		return datasource.GraphNode{ID: id, Kind: "issue", Title: title, Status: "open", Issue: model.Issue{ID: id, Title: title, Status: model.StatusOpen}}
	}
	return datasource.NewMemoryGraph([]datasource.GraphNode{
		{ID: "adr-0019", Kind: "memory", Title: "Serve a mobile web UI", Body: "The phone needs the same project."},
		{ID: "adr-0020", Kind: "memory", Title: "Pair every browser"},
		issue("i1", "Public demo"), issue("i2", "Show releases"), issue("i3", "Design the TUI"), issue("i4", "Unrelated work"),
	}, []datasource.GraphEdge{
		{ID: "l1", Kind: datasource.EdgeCites, Source: "i1", Target: "adr-0019"},
		{ID: "l2", Kind: datasource.EdgeFollows, Source: "i2", Target: "adr-0019"},
		{ID: "l3", Kind: datasource.EdgeCites, Source: "i3", Target: "adr-0019"},
		{ID: "l4", Kind: datasource.EdgeBlocks, Source: "i2", Target: "adr-0020"},
	})
}

func wiresBrowser(t *testing.T, selected string) MemoryBrowser {
	t.Helper()
	b := press(t, startBrowser(t, newGraphFakeSource()), "2")
	b = b.applyGraph(memoryGraphMsg{graph: wiresFanGraph()})
	for side, rows := range [][]graphRow{workRows(b.graph), decisionRows(b.graph)} {
		for i, r := range rows {
			if r.id == selected {
				b.decide.side, b.decide.cursor[side] = side, i
			}
		}
	}
	if b.decisionSelected() != selected {
		t.Fatalf("cannot select %s", selected)
	}
	return b
}

func lineWith(t *testing.T, view, text string) string {
	t.Helper()
	for _, line := range strings.Split(ansi.Strip(view), "\n") {
		if strings.Contains(line, text) {
			return line
		}
	}
	t.Fatalf("no line holds %q:\n%s", text, ansi.Strip(view))
	return ""
}

// Each Link's own run names its kind, so wires that meet in one trunk keep it.
func TestWiresLabelEachLinkWithItsKind(t *testing.T) {
	view := wiresBrowser(t, "adr-0019").focusView(140, 24)
	for _, c := range []struct{ row, word, stroke, head string }{
		{"○ i1 Public demo", "cites", "━━", ">"},
		{"○ i2 Show releases", "follows", "┅┅", "▷"},
		{"○ i3 Design the TUI", "cites", "━━", ">"},
	} {
		line := lineWith(t, view, c.row)
		if !strings.Contains(line, " "+c.word+" ") || !strings.Contains(line, c.stroke) || !strings.Contains(line, c.head) {
			t.Errorf("%s: want %q in %q strokes with head %q, got %q", c.row, c.word, c.stroke, c.head, line)
		}
	}
	if !strings.ContainsAny(ansi.Strip(view), "┃┫┓┛") {
		t.Errorf("trunk is not heavy:\n%s", ansi.Strip(view))
	}
}

// When an Issue is selected the private runs lie on the Memory side.
func TestWiresLabelLinksWhenAnIssueFansOut(t *testing.T) {
	view := wiresBrowser(t, "i2").focusView(140, 24)
	for _, c := range []struct{ row, word, head string }{
		{"adr-0019 Serve a mobile web UI", "follows", "▷"},
		{"adr-0020 Pair every browser", "depends on", ">"},
	} {
		line := lineWith(t, view, c.row)
		run := line[:strings.Index(line, c.row)]
		if !strings.Contains(run, " "+c.word+" ") || !strings.HasSuffix(strings.TrimRight(run, " ▤"), c.head) {
			t.Errorf("%s: want %q ending in %q, got %q", c.row, c.word, c.head, line)
		}
	}
}

// A narrow terminal drops the kind words and keeps the coloured glyphs.
func TestWiresDropKindWordsWhenNarrow(t *testing.T) {
	view := wiresBrowser(t, "adr-0019").focusView(60, 24)
	line := lineWith(t, view, "○ i2")
	if strings.Contains(line, "follows") || !strings.Contains(line, "┅") || !strings.Contains(line, "▷") {
		t.Errorf("narrow wire: %q", line)
	}
}

func TestWiresEnterOpensSidePanelAndEscReturns(t *testing.T) {
	b := wiresBrowser(t, "adr-0019")
	b.width, b.height = 140, 30
	b = press(t, b, "enter")
	if b.mode != memoryModeFocus || !b.wiresDetail {
		t.Fatal("enter must open the side panel in the wires")
	}
	view := screen(b)
	for _, want := range []string{"Work · Issues", "adr-0019 · memory · Serve a mobile web UI", "1 <─ cited by", "The phone needs the same project."} {
		if !strings.Contains(view, want) {
			t.Errorf("panel view lacks %q:\n%s", want, view)
		}
	}
	b = press(t, b, "esc")
	if b.wiresDetail || b.mode != memoryModeFocus || b.decisionSelected() != "adr-0019" {
		t.Fatal("esc must close the panel and keep the wires cursor")
	}
}

func TestWiresDTogglesSidePanel(t *testing.T) {
	b := press(t, wiresBrowser(t, "adr-0019"), "d")
	if b.mode != memoryModeFocus || !b.wiresDetail || !strings.Contains(screen(b), "adr-0019 · memory · Serve a mobile web UI") {
		t.Fatalf("d must open the side panel:\n%s", screen(b))
	}
	b = press(t, b, "d")
	if b.wiresDetail || b.mode != memoryModeFocus || b.decisionSelected() != "adr-0019" {
		t.Fatal("d must close the panel and keep the wires cursor")
	}
}

func TestWiresDOpensNoPanelWithoutARecord(t *testing.T) {
	b := startBrowser(t, newGraphFakeSource()).applyGraph(memoryGraphMsg{})
	b.mode = memoryModeFocus
	if b, _ = b.handleDecisionKey("d"); b.wiresDetail {
		t.Fatal("d opened a panel with no record selected")
	}
}

func TestWiresPanelOpensIssuesToo(t *testing.T) {
	b := press(t, wiresBrowser(t, "i2"), "enter")
	if !b.wiresDetail || !strings.Contains(screen(b), "i2 · issue · Show releases") {
		t.Fatalf("Issue panel:\n%s", screen(b))
	}
}

func TestWiresPanelFollowsTheCursor(t *testing.T) {
	b := press(t, wiresBrowser(t, "adr-0019"), "enter", "k")
	if !b.wiresDetail || !strings.Contains(screen(b), "adr-0020 · memory · Pair every browser") {
		t.Fatalf("panel did not follow the cursor:\n%s", screen(b))
	}
}

func TestWiresPanelFollowsLinksAndGoesBack(t *testing.T) {
	b := press(t, wiresBrowser(t, "adr-0019"), "enter")
	first := b.fullGraph.Neighbours("adr-0019")[0].Other
	b = press(t, b, "1")
	if b.decisionSelected() != first || b.mode != memoryModeFocus {
		t.Fatalf("1 selected %q, want %q", b.decisionSelected(), first)
	}
	b = press(t, b, "backspace")
	if b.decisionSelected() != "adr-0019" || !b.wiresDetail {
		t.Fatalf("backspace selected %q", b.decisionSelected())
	}
}

func TestWiresPanelTakesFullWidthWhenNarrow(t *testing.T) {
	b := wiresBrowser(t, "adr-0019")
	b, _ = b.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	b = press(t, b, "enter")
	view := screen(b)
	if strings.Contains(view, "Work · Issues") || !strings.Contains(view, "Serve a mobile web UI") {
		t.Fatalf("narrow panel:\n%s", view)
	}
}

func TestIntegratedEscClosesWiresPanelBeforeMemories(t *testing.T) {
	m := newBulkMarkModel(t)
	b := press(t, wiresBrowser(t, "adr-0019"), "enter")
	m.memoryBrowser, m.memoryVisible = &b, true
	m, _ = pressBulkKey(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if !m.memoryVisible || m.memoryBrowser.wiresDetail {
		t.Fatal("esc must close the panel and stay in the wires")
	}
	m, _ = pressBulkKey(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.memoryVisible {
		t.Fatal("a second esc leaves the Memory view")
	}
}

func TestIntegratedDTogglesWiresPanel(t *testing.T) {
	m := newBulkMarkModel(t)
	b := wiresBrowser(t, "adr-0019")
	m.memoryBrowser, m.memoryVisible = &b, true
	d := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")}
	m, _ = pressBulkKey(t, m, d)
	if !m.memoryVisible || !m.memoryBrowser.wiresDetail {
		t.Fatal("d must open the wires panel")
	}
	m, _ = pressBulkKey(t, m, d)
	if !m.memoryVisible || m.memoryBrowser.wiresDetail {
		t.Fatal("d must close the wires panel and stay in Memory")
	}
}

func TestIntegratedClickOnWireRecordOpensPanel(t *testing.T) {
	m := newBulkMarkModel(t)
	b := wiresBrowser(t, "adr-0019")
	b, _ = b.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m.memoryBrowser, m.memoryVisible = &b, true
	lines := strings.Split(ansi.Strip(m.View()), "\n")
	for y, line := range lines {
		if i := strings.Index(line, "Pair every browser"); i >= 0 {
			updated, _ := m.Update(tea.MouseMsg{X: ansi.StringWidth(line[:i]), Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
			m = updated.(Model)
			if m.memoryBrowser.decisionSelected() != "adr-0020" || !m.memoryBrowser.wiresDetail {
				t.Fatalf("click selected %q, panel %v", m.memoryBrowser.decisionSelected(), m.memoryBrowser.wiresDetail)
			}
			return
		}
	}
	t.Fatalf("adr-0020 not on screen:\n%s", strings.Join(lines, "\n"))
}

// A slow graph read shows how far it has come instead of a bare wait.
func TestGraphReadShowsProgressBar(t *testing.T) {
	b := NewMemoryBrowser(newGraphFakeSource(), "p")
	b.mode, b.graphLoad = memoryModeFocus, graphReading
	if view := ansi.Strip(b.graphView(140, 24)); !strings.Contains(view, "Reading the Memory inventory") {
		t.Fatalf("before the first count the view must name the inventory read:\n%s", view)
	}
	b, _ = b.Update(memoryGraphProgressMsg{done: 7, total: 23})
	view := ansi.Strip(b.graphView(140, 24))
	if !strings.Contains(view, "Reading Memory Links") || !strings.Contains(view, "7/23") || !strings.Contains(view, "▰") {
		t.Fatalf("progress view = %q, want a bar with 7/23", view)
	}
}

// The read streams its progress: each progress message carries the command
// that waits for the next one, and the last message is the graph.
func TestGraphCmdStreamsProgressUntilTheGraph(t *testing.T) {
	src := newGraphFakeSource()
	src.graphGate = make(chan struct{})
	b := NewMemoryBrowser(src, "p")
	b, cmd := b.enterMode(memoryModeFocus)
	msg := cmd()
	progress, ok := msg.(memoryGraphProgressMsg)
	if !ok || progress.total != 2 || progress.next == nil {
		t.Fatalf("first message = %#v, want progress out of 2 with a next command", msg)
	}
	b, cmd = b.Update(msg)
	if !strings.Contains(ansi.Strip(b.graphView(140, 24)), "/2") {
		t.Fatal("progress did not reach the view")
	}
	close(src.graphGate)
	for steps := 0; b.graphLoad != graphRead; steps++ {
		if steps > 10 || cmd == nil {
			t.Fatal("the read never delivered the graph")
		}
		b, cmd = b.Update(cmd())
	}
	if b.graphErr != nil || len(b.fullGraph.Nodes) == 0 {
		t.Fatalf("graph not applied: err %v", b.graphErr)
	}
}

// Showing an Issue detail in a Memory workspace requests the graph without
// opening the Memory view, shows the read's progress, then the Memory Links.
func TestIssueDetailReadsTheMemoryGraphInTheBackground(t *testing.T) {
	m := newBulkMarkModel(t)
	beadsDir := filepath.Join(t.TempDir(), ".beads")
	m = m.WithDoltSource(datasource.DataSource{Type: datasource.SourceTypeDoltEmbedded, Path: filepath.Join(beadsDir, "embeddeddolt", "x"), Database: "x"})
	src := newGraphFakeSource()
	src.graphGate = make(chan struct{})
	b := NewMemoryBrowser(src, "p")
	m.memoryBrowser, m.memoryGeneration = &b, 1
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 40})
	m = updated.(Model)
	if m.memoryBrowser.graphLoad != graphNotRead {
		t.Fatal("the tree alone must not read the graph")
	}
	m, cmd := pressBulkKey(t, m, runeKey("d"))
	if m.memoryVisible || m.memoryBrowser.graphLoad != graphReading || cmd == nil {
		t.Fatalf("visible %v, load %v; want a hidden read started", m.memoryVisible, m.memoryBrowser.graphLoad)
	}
	read := findMemoryMsg(t, cmd)
	updated, cmd = m.Update(read)
	m = updated.(Model)
	if view := ansi.Strip(m.viewport.View()); !strings.Contains(view, "Reading Memory Links") || !strings.Contains(view, "/2") {
		t.Fatalf("detail does not show the read's progress:\n%s", view)
	}
	close(src.graphGate)
	for steps := 0; ; steps++ {
		if _, ok := m.tree.MemoryGraph(); ok {
			break
		}
		if steps > 10 || cmd == nil {
			t.Fatal("the graph never reached the tree")
		}
		updated, cmd = m.Update(cmd())
		m = updated.(Model)
	}
	if _, ok := datasource.MemoryGraphFor(beadsDir); !ok {
		t.Fatal("the graph read on request must be kept for the next browser")
	}
	if m.memoryVisible {
		t.Fatal("a background read must not open the Memory view")
	}
}

// findMemoryMsg runs cmd and returns the first Memory reply it produces.
func findMemoryMsg(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	queue := []tea.Cmd{cmd}
	for len(queue) > 0 {
		next := queue[0]
		queue = queue[1:]
		if next == nil {
			continue
		}
		switch msg := next().(type) {
		case tea.BatchMsg:
			queue = append(queue, msg...)
		case embeddedMemoryMsg:
			if _, ok := msg.msg.(memoryGraphProgressMsg); ok {
				return msg
			}
		}
	}
	t.Fatal("no Memory graph reply")
	return nil
}

// A workspace change re-reads a shown graph without hiding it, and a change
// during that read repeats it once.
func TestRefreshGraphKeepsTheShownGraphAndRepeatsWhenStale(t *testing.T) {
	src := newGraphFakeSource()
	b := NewMemoryBrowser(src, "p").applyGraph(memoryGraphMsg{graph: fixtureMemoryGraph()})
	b, cmd := b.refreshGraph()
	if cmd == nil || b.graphLoad != graphRead || len(b.fullGraph.Nodes) == 0 {
		t.Fatal("refresh must read again while the graph stays shown")
	}
	b, again := b.refreshGraph()
	if again != nil || !b.graphStale {
		t.Fatal("a change during a read must mark it stale, not start a second read")
	}
	src.graph = datasource.NewMemoryGraph([]datasource.GraphNode{{ID: "only", Kind: "memory"}}, nil)
	for steps := 0; cmd != nil; steps++ {
		if steps > 20 {
			t.Fatal("refresh never settled")
		}
		b, cmd = b.Update(cmd())
	}
	if _, ok := b.fullGraph.Node("only"); !ok || b.graphStale || b.graphRefreshing {
		t.Fatalf("refresh did not apply the new graph: stale %v refreshing %v", b.graphStale, b.graphRefreshing)
	}
	if got := strings.Count(strings.Join(src.calls, " "), "graph"); got != 2 {
		t.Fatalf("graph reads = %d, want the refresh and one repeat", got)
	}
}
