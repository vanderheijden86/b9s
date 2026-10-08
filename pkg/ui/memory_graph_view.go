package ui

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/vanderheijden86/b9s/internal/datasource"
	"github.com/vanderheijden86/b9s/pkg/model"
)

type memoryMode uint8

const (
	memoryModeList memoryMode = iota
	memoryModeFocus
)
const memoryGraphTimeout = 2 * time.Minute

type graphLoad uint8

const (
	graphNotRead graphLoad = iota
	graphReading
	graphRead
)

type memoryGraphMsg struct {
	graph datasource.MemoryGraph
	err   error
}

// memoryGraphProgressMsg reports how many Issue components a graph read has
// done. next waits for the read's following message, so a read keeps one
// command in flight until its memoryGraphMsg arrives.
type memoryGraphProgressMsg struct {
	done, total int
	next        tea.Cmd
}

// Cursors index the complete lists, so collapsing context cannot change identity.
// The zero value shows all context: unrelated records stay visible until shift+tab hides them.
type decisionViewState struct {
	side         int
	cursor       [2]int
	collapsed    bool
	page         int
	detailOffset int
}

// graphCmd reads the graph and streams its progress. Only the latest count
// matters, so the read replaces a count nobody has taken yet and never waits
// for the view.
func (b MemoryBrowser) graphCmd() tea.Cmd {
	source := b.source
	progress := make(chan memoryGraphProgressMsg, 1)
	result := make(chan memoryGraphMsg, 1)
	var start sync.Once
	var wait tea.Cmd
	wait = func() tea.Msg {
		start.Do(func() {
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), memoryGraphTimeout)
				defer cancel()
				g, err := source.Graph(ctx, func(done, total int) {
					select {
					case <-progress:
					default:
					}
					progress <- memoryGraphProgressMsg{done: done, total: total}
				})
				result <- memoryGraphMsg{g, err}
			}()
		})
		select {
		case msg := <-result:
			return msg
		case msg := <-progress:
			msg.next = wait
			return msg
		}
	}
	return wait
}
func (b MemoryBrowser) enterMode(mode memoryMode) (MemoryBrowser, tea.Cmd) {
	b.mode = mode
	b.selectGraphID(b.graphSelected())
	if mode == memoryModeList {
		return b, nil
	}
	return b.readGraph()
}

// readGraph starts the graph read unless one has started.
func (b MemoryBrowser) readGraph() (MemoryBrowser, tea.Cmd) {
	if b.graphLoad != graphNotRead {
		return b, nil
	}
	b.graphLoad, b.graphDone, b.graphTotal = graphReading, 0, 0
	return b, b.graphCmd()
}

// refreshGraph re-reads a graph that is already shown, after the workspace
// changed. The shown graph stays until the new one arrives. A change during
// a read marks it stale, and the read repeats once it ends.
func (b MemoryBrowser) refreshGraph() (MemoryBrowser, tea.Cmd) {
	switch {
	case b.graphLoad == graphReading || b.graphRefreshing:
		b.graphStale = true
		return b, nil
	case b.graphLoad == graphRead && b.graphErr == nil:
		b.graphRefreshing = true
		return b, b.graphCmd()
	}
	return b, nil
}
func (b MemoryBrowser) applyGraph(msg memoryGraphMsg) MemoryBrowser {
	if b.graphRefreshing {
		b.graphRefreshing = false
		if msg.err != nil {
			b.status = "Memory Links not refreshed: " + datasource.ExplainMemoryFailure(msg.err)
			return b
		}
	}
	b.graphLoad = graphRead
	b.graphErr = msg.err
	if msg.err == nil {
		b.fullGraph = msg.graph
		b.filterGraph()
	}
	return b
}

const memoryProgressBarWidth = 20

// memoryGraphProgressText names the step of a graph read: the inventory
// first, then the Issue components, each one traversal.
func memoryGraphProgressText(done, total int) string {
	if total <= 0 {
		return "Reading the Memory inventory…"
	}
	filled := min(memoryProgressBarWidth, done*memoryProgressBarWidth/total)
	bar := strings.Repeat("▰", filled) + strings.Repeat("▱", memoryProgressBarWidth-filled)
	return fmt.Sprintf("Reading Memory Links %s %d/%d", bar, done, total)
}
func (b MemoryBrowser) handleGraphKey(key string) (MemoryBrowser, tea.Cmd) {
	switch key {
	case "/":
		b.prior, b.pane, b.input = b.pane, memoryPaneSearch, b.graphQuery
		return b, nil
	case "A", "a", "O", "o", "C", "c":
		b.graphStatus = ""
		if key == "O" || key == "o" {
			b.graphStatus = model.StatusOpen
		}
		if key == "C" || key == "c" {
			b.graphStatus = model.StatusClosed
		}
		b.filterGraph()
		return b, nil
	case "J", "K":
		if b.expandedDetail || b.wiresDetail {
			if key == "J" {
				b.detailScroll++
			} else {
				b.detailScroll = max(0, b.detailScroll-1)
			}
			return b, nil
		}
	}
	if b.graphLoad != graphRead || b.graphErr != nil {
		return b, nil
	}
	return b.handleDecisionKey(key)
}
func (b MemoryBrowser) graphView(width, height int) string {
	if b.sourceErr != nil {
		return b.errorStyle().Render(memoryText(datasource.ExplainMemoryFailure(b.sourceErr)) + "\nr retry · q back")
	}
	if b.graphLoad == graphReading {
		return b.theme.MutedText.Render(memoryGraphProgressText(b.graphDone, b.graphTotal))
	}
	if b.graphErr != nil {
		return b.errorStyle().Render("Graph unavailable: " + memoryText(datasource.ExplainMemoryFailure(b.graphErr)))
	}
	if len(b.graph.Nodes) == 0 {
		return "No matching beads. A all · / search"
	}
	return b.focusView(width, height)
}
func (b MemoryBrowser) graphHeaderText() string {
	if b.graphLoad != graphRead || b.graphErr != nil {
		return b.project + " · graph"
	}
	view := "Memory wires"
	status := string(b.graphStatus)
	if status == "" {
		status = "all"
	}
	return fmt.Sprintf("%s · %s · %d Issues, %d Memories, %d Links · %s · /%s", b.project, view, len(workRows(b.graph)), len(decisionRows(b.graph)), len(b.graph.Edges), status, memoryText(b.graphQuery))
}

func (b *MemoryBrowser) filterGraph() {
	selected := b.graphSelected()
	nodes := []datasource.GraphNode{}
	visible := map[string]bool{}
	query := strings.ToLower(b.graphQuery)
	for _, n := range b.fullGraph.Nodes {
		if n.Kind == "issue" && b.graphStatus != "" && n.Status != string(b.graphStatus) {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(n.ID+" "+n.Title+" "+n.Body), query) {
			continue
		}
		nodes = append(nodes, n)
		visible[n.ID] = true
	}
	edges := []datasource.GraphEdge{}
	for _, e := range b.fullGraph.Edges {
		if visible[e.Source] && visible[e.Target] {
			edges = append(edges, e)
		}
	}
	b.graph = datasource.NewMemoryGraph(nodes, edges)
	b.decide, b.detailScroll = decisionViewState{}, 0
	b.selectGraphID(selected)
}

// selectGraphID moves the wires cursor to id and reports whether the filtered
// graph holds it.
func (b *MemoryBrowser) selectGraphID(id string) bool {
	for side, rows := range [][]graphRow{workRows(b.graph), decisionRows(b.graph)} {
		for i, row := range rows {
			if row.id == id {
				b.decide.side, b.decide.cursor[side] = side, i
				b.decide.page, b.decide.detailOffset, b.detailScroll = 0, 0, 0
				return true
			}
		}
	}
	return false
}

// followWireLink selects the record at the other end of the selected record's
// nth Link, as numbered in the detail panel.
func (b MemoryBrowser) followWireLink(n int) MemoryBrowser {
	current := b.graphSelected()
	neighbours := b.fullGraph.Neighbours(current)
	if n >= len(neighbours) {
		return b
	}
	other := neighbours[n].Other
	if !b.selectGraphID(other) {
		b.status = graphPath(other) + " is outside the current filter"
		return b
	}
	b.wiresBack = append(b.wiresBack, current)
	return b
}

// handleWiresPanelKey takes the keys that act on the detail panel before the
// wires see them, and reports whether it consumed key.
func (b MemoryBrowser) handleWiresPanelKey(key string) (MemoryBrowser, bool) {
	switch {
	case key == "esc":
		b.wiresDetail, b.wiresBack, b.detailScroll = false, nil, 0
	case key == "backspace":
		if len(b.wiresBack) > 0 {
			previous := b.wiresBack[len(b.wiresBack)-1]
			b.wiresBack = b.wiresBack[:len(b.wiresBack)-1]
			b.selectGraphID(previous)
		}
	case len(key) == 1 && key >= "1" && key <= "9":
		return b.followWireLink(int(key[0] - '1')), true
	default:
		return b, false
	}
	return b, true
}

// openWiresDetail opens the panel only on a record, so an empty graph keeps
// its message in place of an empty panel.
func (b MemoryBrowser) openWiresDetail() MemoryBrowser {
	if _, ok := b.fullGraph.Node(b.decisionSelected()); ok {
		b.wiresDetail, b.detailScroll = true, 0
	}
	return b
}

func (b MemoryBrowser) graphSelected() string {
	return b.decisionSelected()
}

// graphDetail shows the selected record with its Links numbered for 1-9, in
// the panel beside the wires and in the expanded detail.
func (b MemoryBrowser) graphDetail(width, height int) string {
	s := newGraphStyles(b.theme)
	id := b.graphSelected()
	n, ok := b.fullGraph.Node(id)
	if !ok {
		return s.muted.Render("No record selected")
	}
	lines := []string{s.strong.Render(memoryText(id) + " · " + n.Kind + " · " + graphTitle(n))}
	if n.Kind == "issue" {
		lines = append(lines, s.muted.Render(strings.ReplaceAll(n.Status, "_", " ")))
	}
	neighbours := b.fullGraph.Neighbours(id)
	lines = append(lines, s.muted.Render(fmt.Sprintf("─ Links %d", len(neighbours))))
	if len(neighbours) == 0 {
		lines = append(lines, s.muted.Render("No Memory Links recorded."))
	}
	for i, nb := range neighbours {
		key := " "
		if i < 9 {
			key = strconv.Itoa(i + 1)
		}
		conn := connectorFor(nb.Edge.Kind).label(nb.Out)
		lines = append(lines, s.strong.Render(key)+" "+s.edge(nb.Edge.Kind).Render(conn+" ")+graphNodeLine(s, b.fullGraph, nb.Other))
		if nb.Edge.Note != "" {
			lines = append(lines, s.muted.Render("    note: "+memoryText(nb.Edge.Note)))
		}
	}
	if n.Body != "" {
		body := memoryText(n.Body)
		if b.renderer != nil {
			if rendered, err := b.renderer.Render(body); err == nil {
				body = strings.Trim(rendered, "\n")
			}
		}
		lines = append(lines, s.muted.Render("─ Body"), body)
	}
	lines = strings.Split(ansi.Wrap(strings.Join(lines, "\n"), width, ""), "\n")
	offset := min(b.detailScroll, max(0, len(lines)-height))
	return clipBlock(strings.Join(lines[offset:], "\n"), width, height)
}

type graphStyles struct{ memory, issue, cites, related, blocks, muted, text, strong, selected lipgloss.Style }

func newGraphStyles(t Theme) graphStyles {
	fg := func(light, dark string) lipgloss.Style {
		return t.Renderer.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: light, Dark: dark})
	}
	return graphStyles{memory: fg("#2457c5", "#88baff"), issue: fg("#92600b", "#dfbc89"), cites: fg("#6b47d9", "#bf9ffa"), related: fg("#187568", "#6ccabd"), blocks: fg("#92600b", "#e9b375"), muted: t.MutedText, text: t.Renderer.NewStyle(), strong: t.Renderer.NewStyle().Bold(true), selected: t.Selected}
}
func (s graphStyles) edge(kind string) lipgloss.Style {
	switch kind {
	case datasource.EdgeFollows:
		return s.memory
	case datasource.EdgeCites:
		return s.cites
	case datasource.EdgeRelated:
		return s.related
	case datasource.EdgeBlocks:
		return s.blocks
	}
	return s.muted
}
func graphTitle(n datasource.GraphNode) string { return memoryText(n.Title) }
func memoryShortTitle(title string) string     { return memoryText(title) }

// umlConnector draws a Link kind as the UML connector with the same meaning,
// so the wires read like the web graph (ADRs 0048 and 0049). head points at
// the target and back at the source; related, an association, has neither.
type umlConnector struct {
	name, out, in string
	stroke        rune
	head, back    rune
}

var umlConnectors = []struct {
	kind string
	umlConnector
}{
	{datasource.EdgeFollows, umlConnector{"realization", "follows", "followed by", '╌', '▷', '◁'}},
	{datasource.EdgeBlocks, umlConnector{"dependency", "depends on", "blocks", '╌', '>', '<'}},
	{datasource.EdgeCites, umlConnector{"directed association", "cites", "cited by", '─', '>', '<'}},
	{datasource.EdgeRelated, umlConnector{"association", "related", "related", '─', 0, 0}},
}

// A kind b9s does not recognise claims no UML meaning, but keeps its stored
// direction with a plain arrow.
func connectorFor(kind string) umlConnector {
	for _, c := range umlConnectors {
		if c.kind == kind {
			return c.umlConnector
		}
	}
	label := memoryText(kind)
	return umlConnector{"", label, label, '─', '>', '<'}
}

// ends returns the glyphs at the target and source ends, falling back to the
// line itself where the connector has no head.
func (c umlConnector) ends() (head, back string) {
	head, back = string(c.stroke), string(c.stroke)
	if c.head != 0 {
		head, back = string(c.head), string(c.back)
	}
	return head, back
}

func (c umlConnector) label(out bool) string {
	line := strings.Repeat(string(c.stroke), 2)
	head, back := c.ends()
	if out {
		return line + " " + c.out + " " + string(c.stroke) + head
	}
	return back + string(c.stroke) + " " + c.in + " " + line
}

// heavy draws a wire's stroke in heavy box glyphs, so the selected record's
// Links stand out from the light rules of the panes and the Link labels.
func heavy(stroke rune) rune {
	switch stroke {
	case '─':
		return '━'
	case '╌':
		return '┅'
	}
	return stroke
}

func umlLegend() string {
	parts := make([]string, len(umlConnectors))
	for i, c := range umlConnectors {
		head := string(heavy(c.stroke))
		if c.head != 0 {
			head = string(c.head)
		}
		parts[i] = c.out + " " + string(heavy(c.stroke)) + head + " " + c.name
	}
	return strings.Join(parts, " · ") + " · depends on: target blocks source"
}

func graphNodeLine(s graphStyles, g datasource.MemoryGraph, id string) string {
	n, _ := g.Node(id)
	if n.Kind == "memory" {
		return s.memory.Render("▤ ") + graphTitle(n) + " " + s.muted.Render("["+memoryText(id)+"]")
	}
	return s.issue.Render("○ ") + graphTitle(n) + " " + s.muted.Render("["+memoryText(id)+"] "+strings.ReplaceAll(n.Status, "_", " "))
}
func renderNeighbourhood(t Theme, g datasource.MemoryGraph, id string, width int) []string {
	n, ok := g.Node(id)
	if !ok {
		return []string{t.MutedText.Render("No record selected")}
	}
	lines := []string{newGraphStyles(t).strong.Render(memoryText(id) + " · " + n.Kind + " · " + graphTitle(n))}
	return append(lines, neighbourhoodBody(t, g, id, width, nil)...)
}

// The main Issue detail includes incoming and outgoing Memory Links of every Type.
func decisionSection(t Theme, g datasource.MemoryGraph, id string, width int) []string {
	n, ok := g.Node(id)
	if !ok || n.Kind != "issue" {
		return nil
	}
	return neighbourhoodBody(t, g, id, width, func(e datasource.GraphEdge) bool {
		other := e.Target
		if other == id {
			other = e.Source
		}
		n, ok := g.Node(other)
		return ok && n.Kind == "memory"
	})
}
func neighbourhoodBody(t Theme, g datasource.MemoryGraph, id string, width int, keep func(datasource.GraphEdge) bool) []string {
	s := newGraphStyles(t)
	var neighbours []datasource.Neighbour
	for _, n := range g.Neighbours(id) {
		if keep == nil || keep(n.Edge) {
			neighbours = append(neighbours, n)
		}
	}
	if len(neighbours) == 0 {
		return []string{s.muted.Render("No Memory Links recorded.")}
	}
	var lines []string
	for i, n := range neighbours {
		branch := "├"
		if i == len(neighbours)-1 {
			branch = "└"
		}
		conn := connectorFor(n.Edge.Kind).label(n.Out)
		lines = append(lines, s.muted.Render(branch)+s.edge(n.Edge.Kind).Render(conn+" ")+graphNodeLine(s, g, n.Other))
		if n.Edge.Note != "" {
			lines = append(lines, s.muted.Render("   note: "+memoryText(n.Edge.Note)))
		}
	}
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, max(1, width), "…")
	}
	return lines
}

type graphRow struct {
	id, header string
	depth      int
}

func firstGraphRow(rows []graphRow) int {
	for i, r := range rows {
		if r.id != "" {
			return i
		}
	}
	return 0
}
func workRows(g datasource.MemoryGraph) []graphRow     { return rowsOfKind(g, "issue") }
func decisionRows(g datasource.MemoryGraph) []graphRow { return rowsOfKind(g, "memory") }
func rowsOfKind(g datasource.MemoryGraph, kind string) []graphRow {
	var rows []graphRow
	for _, n := range g.Nodes {
		if n.Kind == kind {
			rows = append(rows, graphRow{id: n.ID})
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		a, _ := g.Node(rows[i].id)
		b, _ := g.Node(rows[j].id)
		if a.Title == b.Title {
			return a.ID < b.ID
		}
		return a.Title < b.Title
	})
	return rows
}
func (b MemoryBrowser) decisionWorkRows() []graphRow { return workRows(b.graph) }
func (b MemoryBrowser) decisionSelected() string {
	rows := workRows(b.graph)
	if b.decide.side == 1 {
		rows = decisionRows(b.graph)
	}
	c := b.decide.cursor[b.decide.side]
	if c >= 0 && c < len(rows) {
		return rows[c].id
	}
	return ""
}
func decisionOffset(offset, cursor, total, capacity int) int {
	offset = max(0, min(offset, max(0, total-capacity)))
	if cursor < offset {
		return max(0, cursor)
	}
	if cursor >= offset+capacity {
		return max(0, cursor-capacity+1)
	}
	return offset
}
func (b MemoryBrowser) handleDecisionKey(key string) (MemoryBrowser, tea.Cmd) {
	d := &b.decide
	rows := workRows(b.graph)
	if d.side == 1 {
		rows = decisionRows(b.graph)
	}
	before := b.decisionSelected()
	switch key {
	case "j", "down":
		d.cursor[d.side] = min(max(0, len(rows)-1), d.cursor[d.side]+1)
	case "k", "up":
		d.cursor[d.side] = max(0, d.cursor[d.side]-1)
	case "g", "home":
		d.cursor[d.side] = 0
	case "G", "end":
		d.cursor[d.side] = max(0, len(rows)-1)
	case "tab":
		d.side = 1 - d.side
	case "h", "left":
		d.side = 0
	case "l", "right":
		d.side = 1
	case "shift+tab":
		d.collapsed = !d.collapsed
		d.page = 0
	case "]", "pgdown":
		before := b.focusView(b.width-2, b.height-4)
		d.page++
		if b.focusView(b.width-2, b.height-4) == before {
			d.page--
		}
	case "[", "pgup":
		d.page = max(0, d.page-1)
	case "J":
		stripHeight := min(8, max(3, (b.height-4)/3))
		lines := renderNeighbourhood(b.theme, b.graph, b.decisionSelected(), b.width-2)
		d.detailOffset = min(d.detailOffset+1, max(0, len(lines)-stripHeight))
	case "K":
		d.detailOffset = max(0, d.detailOffset-1)
	case "enter":
		b = b.openWiresDetail()
	case "d":
		if b.wiresDetail {
			b.wiresDetail, b.wiresBack, b.detailScroll = false, nil, 0
		} else {
			b = b.openWiresDetail()
		}
	}
	if before != b.decisionSelected() {
		d.page = 0
		d.detailOffset = 0
		b.detailScroll, b.wiresBack = 0, nil
	}
	return b, nil
}

// compactGraphRows preserves relevant endpoints before spending rows on context.
// If relevant endpoints alone overflow, every page carries the selected record.
func compactGraphRows(rows []graphRow, keep map[string]bool, selected string, capacity, page int, expanded bool) []graphRow {
	capacity = max(1, capacity)
	if len(rows) <= capacity {
		return rows
	}
	if expanded {
		cursor := 0
		for i, r := range rows {
			if r.id == selected {
				cursor = i
			}
		}
		size := max(1, capacity-2)
		start := max(0, min(len(rows)-size, cursor-size/2+page*size))
		var out []graphRow
		if start > 0 {
			out = append(out, graphRow{header: fmt.Sprintf("… %d above", start)})
		}
		out = append(out, rows[start:min(len(rows), start+size)]...)
		if end := start + size; end < len(rows) {
			out = append(out, graphRow{header: fmt.Sprintf("… %d below", len(rows)-end)})
		}
		return out[:min(capacity, len(out))]
	}
	var compact, important []graphRow
	hidden := 0
	flush := func() {
		if hidden > 0 {
			compact = append(compact, graphRow{header: fmt.Sprintf("… %d unrelated hidden · shift+tab expand", hidden)})
			hidden = 0
		}
	}
	for _, r := range rows {
		if keep[r.id] {
			flush()
			compact = append(compact, r)
			important = append(important, r)
		} else {
			hidden++
		}
	}
	flush()
	if len(compact) <= capacity {
		return compact
	}
	if len(important) < capacity {
		return append(important, graphRow{header: fmt.Sprintf("… %d unrelated hidden · shift+tab expand", len(rows)-len(important))})
	}
	var pinned []graphRow
	var rest []graphRow
	for _, r := range important {
		if r.id == selected {
			pinned = append(pinned, r)
		} else {
			rest = append(rest, r)
		}
	}
	size := max(1, capacity-len(pinned)-1)
	pages := max(1, (len(rest)+size-1)/size)
	page = min(page, pages-1)
	start := page * size
	out := append(pinned, rest[start:min(len(rest), start+size)]...)
	if len(out) < capacity {
		out = append(out, graphRow{header: fmt.Sprintf("Links %d/%d · [ ] page · %d hidden", page+1, pages, len(rows)-len(out))})
	}
	return out[:min(capacity, len(out))]
}
func workRowText(s graphStyles, g datasource.MemoryGraph, r graphRow, width int, selected, dim bool) string {
	return nativeRowText(s, g, r, width, selected, dim)
}
func decisionRowText(s graphStyles, g datasource.MemoryGraph, r graphRow, width int, selected, dim bool) string {
	return nativeRowText(s, g, r, width, selected, dim)
}
func nativeRowText(s graphStyles, g datasource.MemoryGraph, r graphRow, width int, selected, dim bool) string {
	if r.id == "" {
		return s.muted.Render(padRight(truncate(r.header, width), width))
	}
	n, _ := g.Node(r.id)
	glyph := "○"
	style := s.issue
	if n.Kind == "memory" {
		glyph = "▤"
		style = s.memory
	} else if n.Issue.IssueType == model.TypeEpic {
		glyph = "◆"
	}
	plain := padRight(truncate(glyph+" "+memoryText(r.id)+" "+graphTitle(n), width), width)
	if selected {
		return s.selected.Render(plain)
	}
	if dim {
		return s.muted.Render(plain)
	}
	return style.Render(plain)
}
func junction(up, down, left, right bool) rune {
	switch {
	case up && down && left && right:
		return '╋'
	case up && down && left:
		return '┫'
	case up && down && right:
		return '┣'
	case up && down:
		return '┃'
	case down && left && right:
		return '┳'
	case up && left && right:
		return '┻'
	case down && left:
		return '┓'
	case down && right:
		return '┏'
	case up && left:
		return '┛'
	case up && right:
		return '┗'
	}
	return '━'
}

type focusWire struct {
	work, decision int
	kind           string
	reverse        bool
}

// labelLaneWidth fits the longest kind word, "followed by", between two
// strokes and its head, plus the trunk and the stub into the selected record.
const labelLaneWidth = 20

type cell struct {
	r     rune
	style lipgloss.Style
}

// privateRun draws the part of a wire that belongs to one Link: its kind
// word, its stroke and its head, so wires that meet in the trunk keep their
// kind. back says whether this run ends at the Link's target when the Link
// points from the Memory side to the Issue side.
func privateRun(s graphStyles, w focusWire, n int, labels, back bool) []cell {
	c := connectorFor(w.kind)
	style := s.edge(w.kind)
	run := make([]cell, n)
	for x := range run {
		run[x] = cell{heavy(c.stroke), style}
	}
	word := []rune(" " + c.out + " ")
	if w.reverse {
		word = []rune(" " + c.in + " ")
	}
	if labels && len(word)+4 <= n {
		for x, r := range word {
			run[2+x] = cell{r, style}
		}
	}
	if !w.reverse && c.head != 0 && n > 0 {
		run[n-1] = cell{c.head, style}
	}
	if w.reverse && back && c.back != 0 && n > 0 {
		run[0] = cell{c.back, style}
	}
	return run
}

// focusLanes draws the wires between the columns. Every wire touches the
// selected record, so they meet in one trunk next to it: fanIn puts the
// selected record on the Memory side, and each Link's own run on the Issue side.
func focusLanes(s graphStyles, wires []focusWire, rows, width int, fanIn, labels bool) []string {
	grid := make([][]cell, rows)
	for y := range grid {
		grid[y] = make([]cell, width)
	}
	if width < 5 {
		wires = nil
	}
	trunk, hub := 1, -1
	if fanIn {
		trunk = width - 3
	}
	lo, hi := rows, -1
	far := map[int]focusWire{}
	for _, w := range wires {
		own, shared := w.decision, w.work
		if fanIn {
			own, shared = w.work, w.decision
		}
		far[own], hub = w, shared
		lo, hi = min(lo, own, shared), max(hi, own, shared)
	}
	for y := max(0, lo); y <= min(hi, rows-1); y++ {
		w, own := far[y]
		left, right := y == hub, own
		if fanIn {
			left, right = own, y == hub
		}
		grid[y][trunk] = cell{junction(y > lo, y < hi, left, right), s.text}
		if y == hub {
			stub := 0
			if fanIn {
				stub = trunk + 1
			}
			grid[y][stub] = cell{'━', s.text}
		}
		if own {
			from, to := trunk+1, width-1
			if fanIn {
				from, to = 0, trunk
			}
			copy(grid[y][from:to], privateRun(s, w, to-from, labels, !fanIn))
		}
	}
	out := make([]string, rows)
	for y, row := range grid {
		var sb strings.Builder
		for _, c := range row {
			if c.r == 0 {
				sb.WriteByte(' ')
			} else {
				sb.WriteString(c.style.Render(string(c.r)))
			}
		}
		out[y] = sb.String()
	}
	return out
}

type wiresLayout struct {
	work, memories                 []graphRow
	linked                         map[string]bool
	leftWidth, laneWidth, capacity int
	stripHeight                    int
	labels                         bool
}

// wiresLayout places the rows and columns of the wires, for drawing them and
// for finding the record under a click.
func (b MemoryBrowser) wiresLayout(width, height int) wiresLayout {
	g, selected := b.graph, b.decisionSelected()
	l := wiresLayout{stripHeight: min(8, max(3, height/3)), linked: map[string]bool{selected: true}}
	// The header, the legend and the strip's heading take three rows; the
	// panel replaces the strip.
	l.capacity = max(1, height-l.stripHeight-3)
	if b.wiresDetail {
		l.stripHeight, l.capacity = 0, max(1, height-2)
	}
	for _, n := range g.Neighbours(selected) {
		l.linked[n.Other] = true
	}
	l.work = compactGraphRows(workRows(g), l.linked, selected, l.capacity, b.decide.page, !b.decide.collapsed)
	l.memories = compactGraphRows(decisionRows(g), l.linked, selected, l.capacity, b.decide.page, !b.decide.collapsed)
	l.labels = width >= 4*labelLaneWidth
	l.laneWidth = max(6, min(14, width/8))
	if l.labels {
		l.laneWidth = labelLaneWidth
	}
	l.leftWidth = (width - l.laneWidth) / 2
	return l
}

func (b MemoryBrowser) focusView(width, height int) string {
	width = max(1, width)
	height = max(1, height)
	if width < 48 || height < 10 {
		return ansi.Truncate("Wires need 48 columns and 10 rows. Press 1 for the list.", width, "…")
	}
	t, g := b.theme, b.graph
	s := newGraphStyles(t)
	selected := b.decisionSelected()
	l := b.wiresLayout(width, height)
	at := [2]map[string]int{{}, {}}
	for i, r := range l.work {
		if r.id != "" {
			at[0][r.id] = i
		}
	}
	for i, r := range l.memories {
		if r.id != "" {
			at[1][r.id] = i
		}
	}
	var wires []focusWire
	for _, e := range g.Edges {
		if e.Source != selected && e.Target != selected {
			continue
		}
		wy, wok := at[0][e.Source]
		my, mok := at[1][e.Target]
		reverse := false
		if !wok || !mok {
			wy, wok = at[0][e.Target]
			my, mok = at[1][e.Source]
			reverse = true
		}
		if wok && mok {
			wires = append(wires, focusWire{wy, my, e.Kind, reverse})
		}
	}
	fanIn := false
	if n, ok := g.Node(selected); ok && n.Kind == "memory" {
		fanIn = true
	}
	rightWidth := width - l.laneWidth - l.leftWidth
	lanes := focusLanes(s, wires, l.capacity, l.laneWidth, fanIn, l.labels)
	issueEnds := map[int]focusWire{}
	for _, wire := range wires {
		issueEnds[wire.work] = wire
	}
	lines := []string{padRight(s.strong.Render("Work · Issues"), l.leftWidth+l.laneWidth) + s.strong.Render("Knowledge · Memories")}
	for i := 0; i < l.capacity; i++ {
		left, right := strings.Repeat(" ", l.leftWidth), strings.Repeat(" ", rightWidth)
		if i < len(l.work) {
			r := l.work[i]
			left = workRowText(s, g, r, l.leftWidth, r.id != "" && r.id == selected, !l.linked[r.id])
			if wire, ok := issueEnds[i]; ok {
				left = issueRun(s, left, wire, l.leftWidth, fanIn, &lanes[i], l.laneWidth)
			}
		}
		if i < len(l.memories) {
			r := l.memories[i]
			right = decisionRowText(s, g, r, rightWidth, r.id != "" && r.id == selected, !l.linked[r.id])
		}
		lines = append(lines, left+lanes[i]+right)
	}
	lines = append(lines, s.muted.Render(ansi.Truncate(umlLegend(), width, "…")))
	if !b.wiresDetail {
		detail := renderNeighbourhood(t, g, selected, width)
		start := min(b.decide.detailOffset, max(0, len(detail)-l.stripHeight))
		lines = append(lines, s.muted.Render(ansi.Truncate(fmt.Sprintf("─ Links %d–%d/%d · enter open · J/K scroll details · shift+tab context · [ ] linked page", start+1, min(len(detail), start+l.stripHeight), len(detail)), width, "…")))
		lines = append(lines, detail[start:min(len(detail), start+l.stripHeight)]...)
	}
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, width, "…")
	}
	return strings.Join(lines[:min(height, len(lines))], "\n")
}

// issueRun extends a wire from the end of the Issue's text to the lane. With
// a Memory selected the run belongs to one Link, so it takes the Link's colour
// and, for a Link that points at the Issue, the head; with the Issue selected
// the run is shared by all its Links.
func issueRun(s graphStyles, text string, wire focusWire, width int, fanIn bool, lane *string, laneWidth int) string {
	c := connectorFor(wire.kind)
	textWidth := ansi.StringWidth(strings.TrimRight(ansi.Strip(text), " "))
	padding := width - textWidth
	style, stroke := s.text, "━"
	if fanIn {
		style, stroke = s.edge(wire.kind), string(heavy(c.stroke))
	}
	back := fanIn && wire.reverse && c.back != 0
	if padding <= 1 {
		if back {
			*lane = style.Render(string(c.back)) + ansi.Cut(*lane, 1, laneWidth)
		}
		return text
	}
	tail := strings.Repeat(stroke, padding-1)
	if back {
		tail = string(c.back) + strings.Repeat(stroke, padding-2)
	}
	return ansi.Truncate(text, textWidth, "") + " " + style.Render(tail)
}

// wiresPanelShown reports whether the detail panel takes part of the screen:
// a graph that failed to read has no record to show.
func (b MemoryBrowser) wiresPanelShown() bool {
	return b.wiresDetail && b.graphLoad == graphRead && b.graphErr == nil && b.sourceErr == nil
}

// wiresPaneWidth is the inner width of the wires pane, and whether the wires
// are on screen: a terminal under 100 columns gives the panel the full width,
// as the tree does with its detail.
func (b MemoryBrowser) wiresPaneWidth() (int, bool) {
	if !b.wiresPanelShown() {
		return max(20, b.width-2), true
	}
	if b.width < 100 {
		return max(20, b.width-2), false
	}
	return b.width*11/20 - 2, true
}

// handleMouse opens the record under a left click in the wires, as enter does.
func (b MemoryBrowser) handleMouse(msg tea.MouseMsg) MemoryBrowser {
	if b.mode != memoryModeFocus || msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft || b.graphLoad != graphRead || b.graphErr != nil || len(b.graph.Nodes) == 0 {
		return b
	}
	width, shown := b.wiresPaneWidth()
	height := max(1, b.height-4)
	if !shown || width < 48 || height < 10 {
		return b
	}
	// The view header and the pane border sit above the wires, whose first
	// row holds the column headings.
	l := b.wiresLayout(width, height)
	row, x := msg.Y-3, msg.X-1
	if row < 0 || row >= l.capacity || x < 0 || x >= width {
		return b
	}
	rows := l.memories
	if x < l.leftWidth {
		rows = l.work
	} else if x < l.leftWidth+l.laneWidth {
		return b
	}
	if row >= len(rows) || rows[row].id == "" || !b.selectGraphID(rows[row].id) {
		return b
	}
	b.wiresDetail, b.wiresBack = true, nil
	return b
}
