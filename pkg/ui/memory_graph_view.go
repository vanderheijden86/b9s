package ui

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/vanderheijden86/b9s/internal/datasource"
	"github.com/vanderheijden86/b9s/pkg/model"
)

// memoryMode is the view the Memory browser shows. The graph views read the
// whole graph, which costs one traversal per component, so they load it on
// first entry and share it (ADR 0032).
type memoryMode uint8

const (
	memoryModeList memoryMode = iota
	memoryModeFocus
	memoryModeMatrix
	memoryModeChips
	memoryModeConstellation
)

// memoryGraphTimeout covers every traversal of a large workspace, each of
// which opens the embedded store.
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

type constellationState struct {
	pos       map[string][2]float64
	selected  string
	hideCites bool
	hideHier  bool
	titles    bool
}

// decisionViewState is shared by the focus, matrix and chips views, so the
// selected Issue stays put while the reader switches between them.
type decisionViewState struct {
	side      int // focus view: 0 the Issue list, 1 the Memory list
	cursor    [2]int
	offset    int
	col       int // matrix view: the decision column
	chip      int // chips view: the selected Issue's Link
	problems  bool
	sortByUse bool
}

func (b MemoryBrowser) graphCmd() tea.Cmd {
	source := b.source
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), memoryGraphTimeout)
		defer cancel()
		graph, err := source.Graph(ctx)
		return memoryGraphMsg{graph: graph, err: err}
	}
}

// enterMode switches view and starts the one graph read the views share.
func (b MemoryBrowser) enterMode(mode memoryMode) (MemoryBrowser, tea.Cmd) {
	b.mode = mode
	if mode == memoryModeList || b.graphLoad != graphNotRead {
		return b, nil
	}
	b.graphLoad = graphReading
	return b, b.graphCmd()
}

func (b MemoryBrowser) applyGraph(msg memoryGraphMsg) MemoryBrowser {
	b.graphLoad = graphRead
	b.graphErr = msg.err
	if msg.err != nil {
		return b
	}
	b.graph = msg.graph
	b.star.pos = layoutConstellation(msg.graph)
	b.decide.cursor = [2]int{firstGraphRow(workRows(msg.graph)), firstGraphRow(decisionRows(msg.graph))}
	return b
}

func (b MemoryBrowser) handleGraphKey(key string) (MemoryBrowser, tea.Cmd) {
	if b.graphLoad != graphRead || b.graphErr != nil {
		return b, nil
	}
	if b.mode == memoryModeConstellation {
		return b.handleConstellationKey(key)
	}
	return b.handleDecisionKey(key)
}

// openMemory shows a graph node's Memory in the list, where its body,
// Links and versions are read.
func (b MemoryBrowser) openMemory(id string) (MemoryBrowser, tea.Cmd) {
	if n, ok := b.graph.Node(id); !ok || n.Kind != "memory" {
		b.status = "Issues open in the main b9s view"
		return b, nil
	}
	full := ""
	for i, s := range b.summaries {
		if graphPath(s.ID) == id {
			b.cursor, full = i, s.ID
			break
		}
	}
	if full == "" {
		for endpoint := range b.endpoints {
			if graphPath(endpoint) == id {
				full = endpoint
			}
		}
	}
	if full == "" {
		b.status = id + " is not in the Memory list"
		return b, nil
	}
	b.mode = memoryModeList
	b.pane = memoryPaneDetail
	return b, b.selectCmd(memoryKey{id: full})
}

func (b MemoryBrowser) graphView(width, height int) string {
	t := b.theme
	switch {
	case b.graphLoad == graphReading:
		return t.MutedText.Render("Reading the graph… one traversal per connected component.")
	case b.graphErr != nil:
		return b.errorStyle().Render("Graph unavailable: " + memoryText(b.graphErr.Error()))
	case b.mode == memoryModeConstellation:
		return b.constellationView(width, height)
	case b.mode == memoryModeMatrix:
		return b.matrixView(width, height)
	case b.mode == memoryModeChips:
		return b.chipsView(width, height)
	}
	return b.focusView(width, height)
}

func (b MemoryBrowser) graphHeaderText() string {
	if b.graphLoad != graphRead || b.graphErr != nil {
		return b.project + " · graph"
	}
	issues, memories := 0, 0
	for _, n := range b.graph.Nodes {
		if n.Kind == "memory" {
			memories++
		} else if n.Kind == "issue" {
			issues++
		}
	}
	name := map[memoryMode]string{
		memoryModeFocus:         "decision wires",
		memoryModeMatrix:        "decision matrix",
		memoryModeChips:         "decision chips",
		memoryModeConstellation: "constellation",
	}[b.mode]
	if b.mode == memoryModeMatrix {
		if b.decide.sortByUse {
			name += " · by use"
		} else {
			name += " · by status"
		}
	}
	if b.mode != memoryModeConstellation && b.decide.problems {
		name += " · problems only"
	}
	return fmt.Sprintf("%s · %s · %d Issues, %d Memories, %d Links", b.project, name, issues, memories, len(b.graph.Edges))
}

// ---------------------------------------------------------------------------
// Shared palette and neighbourhood

type graphStyles struct {
	active, proposed, dead, issue, epic, muted, text, strong, selected lipgloss.Style
}

func newGraphStyles(t Theme) graphStyles {
	r := t.Renderer
	fg := func(light, dark string) lipgloss.Style {
		return r.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: light, Dark: dark})
	}
	return graphStyles{
		active:   fg("#1a7f45", "#4cc27f"),
		proposed: fg("#a86a00", "#e9a640"),
		dead:     fg("#c0392b", "#ef6a60"),
		issue:    fg("#2457c5", "#6d9bff"),
		epic:     fg("#6b47d9", "#a687ef"),
		muted:    t.MutedText,
		text:     r.NewStyle(),
		strong:   r.NewStyle().Bold(true),
		selected: t.Selected,
	}
}

func (s graphStyles) memoryStatus(status string) lipgloss.Style {
	switch status {
	case "active":
		return s.active
	case "proposed":
		return s.proposed
	case "superseded":
		return s.dead
	}
	return s.muted
}

func (s graphStyles) health(h datasource.Health) lipgloss.Style {
	switch h {
	case datasource.HealthDead:
		return s.dead
	case datasource.HealthProposed:
		return s.proposed
	case datasource.HealthNone:
		return s.muted
	case datasource.HealthEpic:
		return s.epic
	}
	return s.issue
}

// healthGlyph marks an Issue by its verdict, so the graph reads without colour.
func healthGlyph(h datasource.Health) string {
	switch h {
	case datasource.HealthDead:
		return "✗"
	case datasource.HealthProposed:
		return "?"
	case datasource.HealthNone:
		return "·"
	case datasource.HealthEpic:
		return "◆"
	}
	return "▪"
}

var (
	titleTagPattern = regexp.MustCompile(`^\[[^\]]*\]\s*`)
	adrTitlePattern = regexp.MustCompile(`^ADR \d+\s*\([^)]*\):\s*`)
)

// graphTitle is a node's title without the id tag the imported titles carry,
// and for a Memory without its "ADR n (status):" prefix too.
func graphTitle(n datasource.GraphNode) string {
	if n.Kind == "memory" {
		return memoryShortTitle(n.Title)
	}
	return titleTagPattern.ReplaceAllString(memoryText(n.Title), "")
}

// memoryShortTitle drops the id tag and the "ADR n (status):" prefix the
// imported titles carry, since the status beside it comes from the
// frontmatter and the title copy may be stale.
func memoryShortTitle(title string) string {
	title = titleTagPattern.ReplaceAllString(memoryText(title), "")
	return adrTitlePattern.ReplaceAllString(title, "")
}

var (
	outConnector = map[string]string{
		datasource.EdgeFollows:    "━━ follows ━━",
		datasource.EdgeCites:      "┄┄ cites ┄┄┄",
		datasource.EdgeSupersedes: "━━ supersedes",
		datasource.EdgeChildOf:    "── part of ──",
		datasource.EdgeBlocks:     "── waits on ─",
		datasource.EdgeRelated:    "── related ──",
	}
	inConnector = map[string]string{
		datasource.EdgeFollows:    "━━ followed by",
		datasource.EdgeCites:      "┄┄ cited by ┄",
		datasource.EdgeSupersedes: "━━ replaced by",
		datasource.EdgeChildOf:    "── contains ──",
		datasource.EdgeBlocks:     "── blocks ───",
		datasource.EdgeRelated:    "── related ──",
	}
)

func graphNodeLine(s graphStyles, g datasource.MemoryGraph, id string) string {
	n, _ := g.Node(id)
	if n.Kind == "memory" {
		status := n.Status
		if status == "" {
			status = "no status"
		}
		return s.strong.Render(g.Label(id)) + " " + s.memoryStatus(n.Status).Render("● "+status) + " " + memoryShortTitle(n.Title)
	}
	return s.health(g.Health(id)).Render(id) + " " + graphTitle(n) + " " + s.muted.Render(strings.ReplaceAll(n.Status, "_", " "))
}

// openFollowers counts the Issues still working to a decision.
func openFollowers(g datasource.MemoryGraph, id string) int {
	count := 0
	for _, f := range g.Followers(id) {
		if n, ok := g.Node(f); ok && n.Issue.Status != model.StatusClosed {
			count++
		}
	}
	return count
}

// renderNeighbourhood draws a node and what it touches two hops deep: each
// Link with its note, then who else shares that neighbour, which is the next
// question a reader asks. The main TUI detail pane and both graph views use it.
func renderNeighbourhood(t Theme, g datasource.MemoryGraph, id string, width int) []string {
	n, ok := g.Node(id)
	if !ok {
		return []string{t.MutedText.Render(id + " is not in the graph")}
	}
	kind := string(n.Issue.IssueType)
	if n.Kind == "memory" {
		kind = n.Date
	}
	s := newGraphStyles(t)
	head := []string{s.strong.Render(g.Label(id)) + "  " + s.muted.Render(kind), graphTitle(n), ""}
	return append(head, neighbourhoodBody(t, g, id, width, nil)...)
}

// decisionSection is the neighbourhood as the main TUI detail pane shows it:
// below the Issue card, so without a header, and only the decision Links,
// because the hierarchy and blockers already have their own sections. An epic
// gets none, as its decisions live on its children.
func decisionSection(t Theme, g datasource.MemoryGraph, id string, width int) []string {
	n, ok := g.Node(id)
	if !ok || n.Kind != "issue" || g.Health(id) == datasource.HealthEpic {
		return nil
	}
	return neighbourhoodBody(t, g, id, width, func(e datasource.GraphEdge) bool {
		return e.Kind == datasource.EdgeFollows || e.Kind == datasource.EdgeCites
	})
}

// neighbourhoodBody draws the health callouts and each Link of id that keep
// accepts, or every Link when keep is nil.
func neighbourhoodBody(t Theme, g datasource.MemoryGraph, id string, width int, keep func(datasource.GraphEdge) bool) []string {
	s := newGraphStyles(t)
	n, _ := g.Node(id)
	var lines []string
	callout := func(style lipgloss.Style, text string) { lines = append(lines, style.Render(text), "") }
	switch g.Health(id) {
	case datasource.HealthDead:
		callout(s.dead, "✗ follows a superseded decision")
	case datasource.HealthNone:
		callout(s.muted, "· no decision recorded for this work")
	case datasource.HealthProposed:
		callout(s.proposed, "? follows a decision that is still proposed")
	}
	if n.Kind == "memory" && n.Status == "superseded" {
		if open := openFollowers(g, id); open == 1 {
			callout(s.dead, "✗ superseded, 1 open Issue still follows it")
		} else if open > 1 {
			callout(s.dead, fmt.Sprintf("✗ superseded, %d open Issues still follow it", open))
		}
	}
	var neighbours []datasource.Neighbour
	for _, x := range g.Neighbours(id) {
		if keep == nil || keep(x.Edge) {
			neighbours = append(neighbours, x)
		}
	}
	if len(neighbours) == 0 && keep == nil {
		lines = append(lines, s.muted.Render("(no Links)"))
	}
	for i, x := range neighbours {
		last := i == len(neighbours)-1
		branch, stem := "├", s.muted.Render("│ ")
		if last {
			branch, stem = "└", "  "
		}
		connectors := inConnector
		if x.Out {
			connectors = outConnector
		}
		connStyle := s.muted
		target, _ := g.Node(x.Edge.Target)
		switch {
		case x.Edge.Kind == datasource.EdgeSupersedes || (x.Edge.Kind == datasource.EdgeFollows && target.Status == "superseded"):
			connStyle = s.dead
		case x.Edge.Kind == datasource.EdgeFollows || x.Edge.Kind == datasource.EdgeChildOf:
			connStyle = s.text
		}
		lines = append(lines, s.muted.Render(branch)+connStyle.Render(connectors[x.Edge.Kind]+" ")+graphNodeLine(s, g, x.Other))
		if x.Edge.Note != "" {
			lines = append(lines, stem+s.muted.Render("   note: "+memoryText(x.Edge.Note)))
		}
		other, _ := g.Node(x.Other)
		var verbs []string
		groups := map[string][]string{}
		for _, y := range g.Neighbours(x.Other) {
			if y.Other == id || y.Edge.Kind == datasource.EdgeChildOf {
				continue
			}
			verb := "also uses"
			switch {
			case y.Edge.Kind == datasource.EdgeSupersedes && y.Out:
				verb = "replaces"
			case y.Edge.Kind == datasource.EdgeSupersedes:
				verb = "replaced by"
			case other.Kind == "memory":
				verb = "also used by"
			}
			if _, seen := groups[verb]; !seen {
				verbs = append(verbs, verb)
			}
			groups[verb] = append(groups[verb], g.Label(y.Other))
		}
		for _, verb := range verbs {
			names := groups[verb]
			more := ""
			if len(names) > 5 {
				names, more = names[:5], " …"
			}
			style := s.muted
			if strings.HasPrefix(verb, "replace") {
				style = s.dead
			}
			lines = append(lines, stem+style.Render("   └ "+verb+" "+strings.Join(names, ", ")+more))
		}
	}
	if n.Kind == "issue" && len(g.DecisionEdges(id)) > 0 {
		lines = append(lines, "", s.muted.Render("read-at version: not recorded by the preview"))
	}
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, width, "…")
	}
	return lines
}

// graphProblems lists the Issues that need a reader, worst first.
func graphProblems(g datasource.MemoryGraph) []string {
	rank := map[datasource.Health]int{datasource.HealthDead: 0, datasource.HealthProposed: 1, datasource.HealthNone: 2}
	var ids []string
	for _, n := range g.Nodes {
		if g.Health(n.ID).Problem() {
			ids = append(ids, n.ID)
		}
	}
	sort.SliceStable(ids, func(i, j int) bool { return rank[g.Health(ids[i])] < rank[g.Health(ids[j])] })
	return ids
}

func healthText(h datasource.Health) string {
	switch h {
	case datasource.HealthDead:
		return "follows a superseded decision"
	case datasource.HealthProposed:
		return "follows a decision that is still proposed"
	case datasource.HealthNone:
		return "no decision recorded"
	}
	return ""
}

// ---------------------------------------------------------------------------
// Constellation

const (
	simWidth  = 900.0
	simHeight = 640.0
	simSteps  = 500
)

// springs give each Link kind a rest length and strength: hierarchy pulls
// hardest, so an epic's work clusters, and citations pull least.
var springs = map[string][2]float64{
	datasource.EdgeFollows:    {80, .06},
	datasource.EdgeCites:      {130, .02},
	datasource.EdgeChildOf:    {55, .08},
	datasource.EdgeSupersedes: {90, .05},
	datasource.EdgeBlocks:     {70, .03},
	datasource.EdgeRelated:    {110, .02},
}

// layoutConstellation places every node with a deterministic force
// simulation: pairwise repulsion, typed springs and weak gravity, started on
// a golden-angle spiral so equal input gives an equal picture.
func layoutConstellation(g datasource.MemoryGraph) map[string][2]float64 {
	type body struct{ x, y, vx, vy float64 }
	nodes := make([]body, len(g.Nodes))
	index := make(map[string]int, len(g.Nodes))
	for i, n := range g.Nodes {
		a, r := float64(i)*2.399, 60+9*float64(i)
		nodes[i] = body{x: simWidth/2 + math.Cos(a)*r, y: simHeight/2 + math.Sin(a)*r}
		index[n.ID] = i
	}
	for k := 0; k < simSteps; k++ {
		alpha := 1 - float64(k)/simSteps
		for i := range nodes {
			for j := i + 1; j < len(nodes); j++ {
				a, b := &nodes[i], &nodes[j]
				dx, dy := b.x-a.x, b.y-a.y
				d := math.Sqrt(dx*dx + dy*dy + .01)
				f := math.Min(40, 2200/d) * (.3 + .7*alpha) / d * 6
				dx, dy = dx/d, dy/d
				a.vx -= dx * f
				a.vy -= dy * f
				b.vx += dx * f
				b.vy += dy * f
			}
		}
		for _, e := range g.Edges {
			spring := springs[e.Kind]
			s, t := &nodes[index[e.Source]], &nodes[index[e.Target]]
			dx, dy := t.x-s.x, t.y-s.y
			d := math.Sqrt(dx*dx + dy*dy)
			if d == 0 {
				d = 1
			}
			f := (d - spring[0]) * spring[1] * alpha
			s.vx += dx / d * f
			s.vy += dy / d * f
			t.vx -= dx / d * f
			t.vy -= dy / d * f
		}
		for i := range nodes {
			n := &nodes[i]
			n.vx += (simWidth/2 - n.x) * .004
			n.vy += (simHeight/2 - n.y) * .007
			n.x += n.vx
			n.y += n.vy
			n.vx *= .6
			n.vy *= .6
			n.x = math.Max(50, math.Min(simWidth-50, n.x))
			n.y = math.Max(30, math.Min(simHeight-30, n.y))
		}
	}
	pos := make(map[string][2]float64, len(nodes))
	for i, n := range g.Nodes {
		pos[n.ID] = [2]float64{nodes[i].x, nodes[i].y}
	}
	return pos
}

// nearestInDirection finds the node closest to from along (dx, dy), with
// sideways distance counted double so the move follows the key.
func nearestInDirection(pos map[string][2]float64, from string, dx, dy float64) string {
	origin := pos[from]
	best, bestScore := "", math.Inf(1)
	ids := make([]string, 0, len(pos))
	for id := range pos {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if id == from {
			continue
		}
		vx, vy := pos[id][0]-origin[0], pos[id][1]-origin[1]
		along := vx*dx + vy*dy
		if along <= 0 {
			continue
		}
		side := math.Abs(vx*dy - vy*dx)
		if score := along + 2*side; score < bestScore {
			best, bestScore = id, score
		}
	}
	return best
}

func (b MemoryBrowser) handleConstellationKey(key string) (MemoryBrowser, tea.Cmd) {
	star := &b.star
	move := func(dx, dy float64) {
		if star.selected == "" {
			star.selected = centralNode(star.pos)
			return
		}
		if next := nearestInDirection(star.pos, star.selected, dx, dy); next != "" {
			star.selected = next
		}
	}
	switch key {
	case "h", "left":
		move(-1, 0)
	case "l", "right":
		move(1, 0)
	case "k", "up":
		move(0, -1)
	case "j", "down":
		move(0, 1)
	case "n", "N":
		problems := graphProblems(b.graph)
		if len(problems) == 0 {
			break
		}
		at := -1
		for i, id := range problems {
			if id == star.selected {
				at = i
			}
		}
		switch {
		case key == "n":
			at = (at + 1) % len(problems)
		case at <= 0:
			at = len(problems) - 1
		default:
			at--
		}
		star.selected = problems[at]
	case "esc":
		star.selected = ""
	case "c":
		star.hideCites = !star.hideCites
	case "e":
		star.hideHier = !star.hideHier
	case "t":
		star.titles = !star.titles
	case "enter":
		if star.selected == "" {
			break
		}
		if n, _ := b.graph.Node(star.selected); n.Kind == "memory" {
			return b.openMemory(star.selected)
		}
		if d := b.graph.DecisionEdges(star.selected); len(d) > 0 {
			star.selected = d[0].Target
		} else if nb := b.graph.Neighbours(star.selected); len(nb) > 0 {
			star.selected = nb[0].Other
		}
	}
	return b, nil
}

func centralNode(pos map[string][2]float64) string {
	best, bestD := "", math.Inf(1)
	for id, p := range pos {
		d := math.Hypot(p[0]-simWidth/2, p[1]-simHeight/2)
		if d < bestD || (d == bestD && id < best) {
			best, bestD = id, d
		}
	}
	return best
}

// issueLabelPrefix is the "name-" every Issue id shares, which a crowded
// canvas cannot afford to repeat.
func issueLabelPrefix(g datasource.MemoryGraph) string {
	prefix := ""
	for _, n := range g.Nodes {
		if n.Kind != "issue" {
			continue
		}
		i := strings.Index(n.ID, "-")
		if i < 0 {
			return ""
		}
		if prefix == "" {
			prefix = n.ID[:i+1]
		} else if !strings.HasPrefix(n.ID, prefix) {
			return ""
		}
	}
	return prefix
}

type canvasCell struct {
	r     rune
	dots  uint8
	style int
	prio  int
}

var brailleDot = [4][2]uint8{{0x01, 0x08}, {0x02, 0x10}, {0x04, 0x20}, {0x40, 0x80}}

func (b MemoryBrowser) constellationView(width, height int) string {
	panelWidth := min(46, max(24, width/3))
	canvasWidth := max(10, width-panelWidth-1)
	canvas := b.constellationCanvas(canvasWidth, height)
	panel := b.constellationPanel(panelWidth, height)
	sep := b.theme.MutedText.Render("│")
	lines := make([]string, height)
	for i := range lines {
		left := ""
		if i < len(canvas) {
			left = canvas[i]
		}
		right := ""
		if i < len(panel) {
			right = panel[i]
		}
		lines[i] = padRight(left, canvasWidth) + sep + right
	}
	return strings.Join(lines, "\n")
}

func (b MemoryBrowser) constellationCanvas(width, height int) []string {
	g, star := b.graph, b.star
	s := newGraphStyles(b.theme)
	styles := []lipgloss.Style{s.text, s.muted, s.dead, s.epic, s.active, s.proposed, s.issue, s.strong, s.selected}
	const (
		stText = iota
		stMuted
		stDead
		stEpic
		stActive
		stProposed
		stIssue
		stStrong
		stSelected
	)
	grid := make([][]canvasCell, height)
	for y := range grid {
		grid[y] = make([]canvasCell, width)
	}
	cell := func(id string) (int, int) {
		p := star.pos[id]
		x := 4 + (p[0]-50)/(simWidth-100)*float64(max(1, width-9))
		y := (p[1] - 30) / (simHeight - 60) * float64(max(1, height-1))
		return int(math.Round(x)), int(math.Round(y))
	}
	touches := func(e datasource.GraphEdge) bool {
		return star.selected == "" || e.Source == star.selected || e.Target == star.selected
	}
	for _, e := range g.Edges {
		if (star.hideCites && e.Kind == datasource.EdgeCites) || (star.hideHier && e.Kind == datasource.EdgeChildOf) {
			continue
		}
		target, _ := g.Node(e.Target)
		style, prio := stMuted, 1
		switch {
		case e.Kind == datasource.EdgeSupersedes || (e.Kind == datasource.EdgeFollows && target.Status == "superseded"):
			style, prio = stDead, 4
		case e.Kind == datasource.EdgeFollows:
			style, prio = stText, 3
		case e.Kind == datasource.EdgeChildOf:
			style, prio = stEpic, 2
		}
		if !touches(e) {
			style, prio = stMuted, 0
		}
		x0, y0 := cell(e.Source)
		x1, y1 := cell(e.Target)
		drawBrailleLine(grid, x0*2+1, y0*4+2, x1*2+1, y1*4+2, e.Kind == datasource.EdgeCites, style, prio)
	}
	neighbour := map[string]bool{}
	if star.selected != "" {
		for _, x := range g.Neighbours(star.selected) {
			neighbour[x.Other] = true
		}
	}
	prefix := issueLabelPrefix(g)
	taken := make([][]bool, height)
	for y := range taken {
		taken[y] = make([]bool, width)
	}
	// Selected and problem nodes claim space first, so collisions move the
	// labels a reader is less likely to be looking for.
	order := make([]int, len(g.Nodes))
	for i := range order {
		order[i] = i
	}
	weight := func(id string) int {
		switch {
		case id == star.selected:
			return 0
		case neighbour[id]:
			return 1
		case g.Health(id).Problem():
			return 2
		}
		return 3
	}
	sort.SliceStable(order, func(i, j int) bool { return weight(g.Nodes[order[i]].ID) < weight(g.Nodes[order[j]].ID) })
	for _, i := range order {
		n := g.Nodes[i]
		var label string
		var style int
		if n.Kind == "memory" {
			number := n.Number
			if number == "" {
				number = n.ID
			}
			label = "●" + number
			style = map[string]int{"active": stActive, "proposed": stProposed, "superseded": stDead}[n.Status]
			if n.Status == "" {
				style = stMuted
			}
		} else {
			h := g.Health(n.ID)
			label = healthGlyph(h) + strings.TrimPrefix(n.ID, prefix)
			style = map[datasource.Health]int{datasource.HealthDead: stDead, datasource.HealthProposed: stProposed, datasource.HealthNone: stMuted, datasource.HealthEpic: stEpic}[h]
			if h == datasource.HealthOK {
				style = stIssue
			}
		}
		if star.titles {
			label += " " + truncate(graphTitle(n), 18)
		}
		switch {
		case n.ID == star.selected:
			style = stSelected
		case star.selected != "" && !neighbour[n.ID]:
			style = stMuted
		}
		cx, cy := cell(n.ID)
		placeLabel(grid, taken, []rune(label), cx, cy, style)
	}
	lines := make([]string, height)
	for y, row := range grid {
		var sb strings.Builder
		run, runStyle := []rune{}, -1
		flush := func() {
			if len(run) == 0 {
				return
			}
			if runStyle < 0 {
				sb.WriteString(string(run))
			} else {
				sb.WriteString(styles[runStyle].Render(string(run)))
			}
			run = run[:0]
		}
		for _, c := range row {
			r, st := ' ', -1
			switch {
			case c.r != 0:
				r, st = c.r, c.style
			case c.dots != 0:
				r, st = rune(0x2800+int(c.dots)), c.style
			}
			if st != runStyle {
				flush()
				runStyle = st
			}
			run = append(run, r)
		}
		flush()
		lines[y] = sb.String()
	}
	return lines
}

// drawBrailleLine sets the dots of a line in braille sub-cell space, two dots
// across and four down per cell. A dotted line skips every third dot.
func drawBrailleLine(grid [][]canvasCell, x0, y0, x1, y1 int, dotted bool, style, prio int) {
	dx, dy := absInt(x1-x0), -absInt(y1-y0)
	sx, sy := 1, 1
	if x0 > x1 {
		sx = -1
	}
	if y0 > y1 {
		sy = -1
	}
	err := dx + dy
	for step := 0; ; step++ {
		if !dotted || step%3 != 2 {
			cx, cy := x0/2, y0/4
			if cy >= 0 && cy < len(grid) && cx >= 0 && cx < len(grid[cy]) {
				c := &grid[cy][cx]
				c.dots |= brailleDot[y0%4][x0%2]
				if prio >= c.prio {
					c.style, c.prio = style, prio
				}
			}
		}
		if x0 == x1 && y0 == y1 {
			return
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

// placeLabel centres a label on its node, then tries the rows below and
// above until it overlaps no label already placed.
func placeLabel(grid [][]canvasCell, taken [][]bool, label []rune, cx, cy, style int) {
	height := len(grid)
	if height == 0 {
		return
	}
	width := len(grid[0])
	x := max(0, min(width-len(label), cx-len(label)/2))
	for _, dy := range []int{0, 1, -1, 2, -2, 3, -3} {
		y := cy + dy
		if y < 0 || y >= height {
			continue
		}
		free := true
		for i := -1; i <= len(label); i++ {
			if xi := x + i; xi >= 0 && xi < width && taken[y][xi] {
				free = false
				break
			}
		}
		if !free && dy != 3 && dy != -3 {
			continue
		}
		for i, r := range label {
			if xi := x + i; xi < width {
				grid[y][xi] = canvasCell{r: r, style: style}
				taken[y][xi] = true
			}
		}
		return
	}
}

func (b MemoryBrowser) constellationPanel(width, height int) []string {
	t := b.theme
	if b.star.selected != "" {
		return renderNeighbourhood(t, b.graph, b.star.selected, width)
	}
	s := newGraphStyles(t)
	problems := graphProblems(b.graph)
	lines := []string{t.MutedText.Render("Graph health"), s.strong.Render(fmt.Sprintf("%d Issues need a look", len(problems))), t.MutedText.Render("n walks them · hjkl moves"), ""}
	for _, id := range problems {
		if len(lines) >= height-1 {
			lines = append(lines, t.MutedText.Render("…"))
			break
		}
		h := b.graph.Health(id)
		n, _ := b.graph.Node(id)
		lines = append(lines, ansi.Truncate(s.health(h).Render(healthGlyph(h)+" "+id)+" "+graphTitle(n), width, "…"))
		lines = append(lines, "  "+t.MutedText.Render(healthText(h)))
	}
	legend := s.active.Render("●") + " active " + s.proposed.Render("●") + " proposed " + s.dead.Render("●") + " superseded"
	if len(lines) < height {
		lines = append(lines, "", legend)
	}
	return lines
}

// ---------------------------------------------------------------------------
// Decision views: focus wires, matrix and chips

// graphRow is a line of the work or decision list: a group header or a node.
type graphRow struct {
	id     string
	header string
	depth  int
}

func firstGraphRow(rows []graphRow) int {
	for i, r := range rows {
		if r.id != "" {
			return i
		}
	}
	return 0
}

// workRows lays out the epic tree, then loose work, then empty epics.
func workRows(g datasource.MemoryGraph) []graphRow {
	kids := map[string][]string{}
	parent := map[string]string{}
	for _, e := range g.Edges {
		if e.Kind == datasource.EdgeChildOf {
			kids[e.Target] = append(kids[e.Target], e.Source)
			parent[e.Source] = e.Target
		}
	}
	var rows []graphRow
	var walk func(id string, depth int)
	walk = func(id string, depth int) {
		rows = append(rows, graphRow{id: id, depth: depth})
		for _, k := range kids[id] {
			walk(k, depth+1)
		}
	}
	var loose, empty []string
	for _, n := range g.Nodes {
		if n.Kind != "issue" || parent[n.ID] != "" {
			continue
		}
		switch {
		case n.Issue.IssueType != model.TypeEpic:
			loose = append(loose, n.ID)
		case len(kids[n.ID]) == 0 && len(g.DecisionEdges(n.ID)) == 0:
			empty = append(empty, n.ID)
		default:
			walk(n.ID, 0)
		}
	}
	if len(loose) > 0 {
		rows = append(rows, graphRow{header: "No epic"})
		for _, id := range loose {
			walk(id, 0)
		}
	}
	if len(empty) > 0 {
		rows = append(rows, graphRow{header: "Epics with no Links"})
		for _, id := range empty {
			rows = append(rows, graphRow{id: id})
		}
	}
	return rows
}

// decisionRows groups Memories by status, most used first in each group.
func decisionRows(g datasource.MemoryGraph) []graphRow {
	var rows []graphRow
	use := func(id string) int { return len(g.Followers(id)) + len(g.Citers(id)) }
	for _, status := range []string{"active", "proposed", "superseded", ""} {
		var ids []string
		for _, n := range g.Nodes {
			if n.Kind == "memory" && (n.Status == status || (status == "" && !knownMemoryStatus(n.Status))) {
				ids = append(ids, n.ID)
			}
		}
		if len(ids) == 0 {
			continue
		}
		sort.SliceStable(ids, func(i, j int) bool { return use(ids[i]) > use(ids[j]) })
		header := status
		if header == "" {
			header = "no status"
		}
		rows = append(rows, graphRow{header: header})
		for _, id := range ids {
			rows = append(rows, graphRow{id: id})
		}
	}
	return rows
}

func knownMemoryStatus(status string) bool {
	return status == "active" || status == "proposed" || status == "superseded"
}

// matrixColumns are the decisions some Issue follows or cites, in the order
// of decisionRows. A Memory nothing links to would be an empty column.
func matrixColumns(g datasource.MemoryGraph) []string {
	var cols []string
	for _, r := range decisionRows(g) {
		if r.id != "" && len(g.Followers(r.id))+len(g.Citers(r.id)) > 0 {
			cols = append(cols, r.id)
		}
	}
	return cols
}

func (b MemoryBrowser) decisionWorkRows() []graphRow {
	rows := workRows(b.graph)
	if !b.decide.problems {
		return rows
	}
	var filtered []graphRow
	for _, row := range rows {
		if row.id != "" && b.graph.Health(row.id).Problem() {
			filtered = append(filtered, row)
		}
	}
	return filtered
}

func (b MemoryBrowser) decisionColumns() []string {
	cols := matrixColumns(b.graph)
	if b.decide.sortByUse {
		use := func(id string) int { return len(b.graph.Followers(id)) + len(b.graph.Citers(id)) }
		sort.SliceStable(cols, func(i, j int) bool { return use(cols[i]) > use(cols[j]) })
	}
	return cols
}

// decisionOffset keeps the selected row visible after navigation or resize.
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

// decisionTag is the short name a chip or column carries: the ADR number,
// or the Memory's id when it has none.
func decisionTag(n datasource.GraphNode) string {
	if n.Number != "" {
		return n.Number
	}
	return graphPath(n.ID)
}

// matrixLabel fits a decision into a two-cell column header: the last two
// digits of an ADR number, or the initials of a named Memory.
func matrixLabel(n datasource.GraphNode) string {
	tag := decisionTag(n)
	if n.Number != "" && len(tag) >= 2 {
		return tag[len(tag)-2:]
	}
	initials := ""
	for _, part := range strings.FieldsFunc(tag, func(r rune) bool { return r == '-' || r == '_' || r == ' ' }) {
		initials += string([]rune(part)[:1])
	}
	return truncate(initials, 2)
}

func (b MemoryBrowser) decisionSide() int {
	if b.mode == memoryModeFocus {
		return b.decide.side
	}
	return 0
}

func (b MemoryBrowser) decisionSelected() string {
	rows := b.decisionWorkRows()
	if b.decisionSide() == 1 {
		rows = decisionRows(b.graph)
	}
	if c := b.decide.cursor[b.decisionSide()]; c < len(rows) {
		return rows[c].id
	}
	return ""
}

// selectedLinks are the follows and cites Links of the selected Issue.
func (b MemoryBrowser) selectedLinks() []datasource.GraphEdge {
	rows := b.decisionWorkRows()
	if c := b.decide.cursor[0]; c < len(rows) && rows[c].id != "" {
		return b.graph.DecisionEdges(rows[c].id)
	}
	return nil
}

// decisionLayout splits the pane height into the list and the strip below
// it that explains the selection.
func (b MemoryBrowser) decisionLayout() (listHeight, stripHeight int) {
	height := max(1, b.height-4)
	if b.mode == memoryModeMatrix {
		return max(1, height-6), 4
	}
	if b.mode == memoryModeChips {
		return max(1, height-1), 0
	}
	stripHeight = min(12, max(4, height/3))
	return max(1, height-stripHeight-2), stripHeight
}

func (b MemoryBrowser) handleDecisionKey(key string) (MemoryBrowser, tea.Cmd) {
	d := &b.decide
	side := b.decisionSide()
	rows := b.decisionWorkRows()
	if side == 1 {
		rows = decisionRows(b.graph)
	}
	step := func(from, dir int) int {
		for i := from + dir; i >= 0 && i < len(rows); i += dir {
			if rows[i].id != "" {
				return i
			}
		}
		return from
	}
	before := d.cursor[side]
	cols := b.decisionColumns()
	switch key {
	case "p":
		selected := ""
		work := b.decisionWorkRows()
		if d.cursor[0] < len(work) {
			selected = work[d.cursor[0]].id
		}
		d.problems = !d.problems
		work = b.decisionWorkRows()
		d.cursor[0] = firstGraphRow(work)
		for i, row := range work {
			if row.id == selected {
				d.cursor[0] = i
			}
		}
		d.offset, d.chip = 0, 0
	case "s":
		if b.mode == memoryModeMatrix {
			selected := ""
			if d.col < len(cols) {
				selected = cols[d.col]
			}
			d.sortByUse = !d.sortByUse
			for i, id := range b.decisionColumns() {
				if id == selected {
					d.col = i
				}
			}
		}
	case "j", "down":
		d.cursor[side] = step(d.cursor[side], 1)
	case "k", "up":
		d.cursor[side] = step(d.cursor[side], -1)
	case "g", "home":
		d.cursor[side] = firstGraphRow(rows)
	case "G", "end":
		d.cursor[side] = step(len(rows), -1)
	case "tab", "h", "left", "l", "right":
		next := 1
		if key == "h" || key == "left" {
			next = -1
		}
		switch b.mode {
		case memoryModeFocus:
			if key == "tab" {
				d.side = 1 - d.side
			} else {
				d.side = max(0, next)
			}
		case memoryModeMatrix:
			d.col = max(0, min(len(cols)-1, d.col+next))
		case memoryModeChips:
			d.chip = max(0, min(len(b.selectedLinks())-1, d.chip+next))
		}
	case "enter":
		switch b.mode {
		case memoryModeMatrix:
			if d.col < len(cols) {
				return b.openMemory(cols[d.col])
			}
			return b, nil
		case memoryModeChips:
			if links := b.selectedLinks(); d.chip < len(links) {
				return b.openMemory(links[d.chip].Target)
			}
			return b, nil
		}
		return b.openMemory(b.decisionSelected())
	}
	if d.cursor[0] != before && side == 0 {
		d.chip = 0
	}
	listHeight, _ := b.decisionLayout()
	rows = b.decisionWorkRows()
	if b.decisionSide() == 1 {
		rows = decisionRows(b.graph)
	}
	d.offset = decisionOffset(d.offset, d.cursor[b.decisionSide()], len(rows), listHeight)
	return b, nil
}

// workRowText is an Issue row: indent, health glyph, id and title, cut to
// width cells less the room the caller keeps for what follows it.
func workRowText(s graphStyles, g datasource.MemoryGraph, r graphRow, width int, selected, dim bool) string {
	if r.id == "" {
		return padRight(s.muted.Render(truncate(r.header, width)), width)
	}
	n, _ := g.Node(r.id)
	h := g.Health(r.id)
	glyph := healthGlyph(h)
	plain := padRight(truncate(strings.Repeat("  ", r.depth)+glyph+" "+r.id+" "+graphTitle(n), width), width)
	switch {
	case selected:
		return s.selected.Render(plain)
	case dim:
		return s.muted.Render(plain)
	}
	at := strings.Index(plain, glyph)
	if at < 0 {
		return plain
	}
	return plain[:at] + s.health(h).Render(glyph) + plain[at+len(glyph):]
}

// decisionRowText is a Memory row: status dot, number and title.
func decisionRowText(s graphStyles, g datasource.MemoryGraph, r graphRow, width int, selected, dim bool) string {
	if r.id == "" {
		return padRight(s.memoryStatus(r.header).Render(truncate(r.header, width)), width)
	}
	n, _ := g.Node(r.id)
	plain := padRight(truncate("● "+decisionTag(n)+" "+graphTitle(n), width), width)
	switch {
	case selected:
		return s.selected.Render(plain)
	case dim:
		return s.muted.Render(plain)
	}
	return s.memoryStatus(n.Status).Render("●") + plain[len("●"):]
}

func countChip(s graphStyles, text string, width int) string {
	return s.muted.Render(padRight(text, width))
}

// junction is the box glyph where a trunk meets its branches, with rounded
// corners where the trunk turns.
func junction(up, down, left, right bool) rune {
	switch {
	case up && down && left && right:
		return '┼'
	case up && down && left:
		return '┤'
	case up && down && right:
		return '├'
	case up && down:
		return '│'
	case down && left && right:
		return '┬'
	case up && left && right:
		return '┴'
	case down && left:
		return '╮'
	case down && right:
		return '╭'
	case up && left:
		return '╯'
	case up && right:
		return '╰'
	}
	return '─'
}

// focusWire is one Link the selected node owns, as rows of the two lists.
type focusWire struct {
	work, decision int
	kind           string
	status         string
}

// focusLanes draws the selected node's wires in a lane width cells wide: a
// stub from each Issue row into one trunk, and from the trunk to each
// Memory row, ending in an arrowhead. The selected node's own stub carries
// the accent, each other end the colour of its decision's status, dotted
// for cites.
func focusLanes(s graphStyles, wires []focusWire, side, rows, width int) []string {
	type cell struct {
		r     rune
		style lipgloss.Style
	}
	grid := make([][]cell, rows)
	for y := range grid {
		grid[y] = make([]cell, width)
	}
	if len(wires) == 0 || width < 5 {
		wires = nil
	}
	const trunk = 2
	lo, hi := rows, -1
	stubs := [2]map[int]focusWire{{}, {}}
	for _, w := range wires {
		stubs[0][w.work], stubs[1][w.decision] = w, w
		lo, hi = min(lo, w.work, w.decision), max(hi, w.work, w.decision)
	}
	line := func(y, from, to int, w focusWire, own bool) {
		if y < 0 || y >= rows {
			return
		}
		r, style := '─', s.memoryStatus(w.status)
		if own {
			style = s.issue
		} else if w.kind == datasource.EdgeCites {
			r = '┄'
		}
		for x := from; x < to; x++ {
			grid[y][x] = cell{r, style}
		}
	}
	for y := max(0, lo); y <= min(hi, rows-1); y++ {
		left, hasLeft := stubs[0][y]
		right, hasRight := stubs[1][y]
		grid[y][trunk] = cell{junction(y > lo, y < hi, hasLeft, hasRight), s.issue}
		if hasLeft {
			line(y, 0, trunk, left, side == 0)
		}
		if hasRight {
			line(y, trunk+1, width-2, right, side == 1)
			style := s.memoryStatus(right.status)
			if side == 1 {
				style = s.issue
			}
			grid[y][width-2] = cell{'▶', style}
		}
	}
	out := make([]string, rows)
	for y, row := range grid {
		var sb strings.Builder
		for _, c := range row {
			if c.r == 0 {
				sb.WriteByte(' ')
				continue
			}
			sb.WriteString(c.style.Render(string(c.r)))
		}
		out[y] = sb.String()
	}
	return out
}

// focusView draws only the selected node's Links, so a busy graph stays
// readable; every other Issue shows how many decisions it owns.
func (b MemoryBrowser) focusView(width, height int) string {
	t := b.theme
	g := b.graph
	s := newGraphStyles(t)
	work, decisions := b.decisionWorkRows(), decisionRows(g)
	listHeight, stripHeight := b.decisionLayout()
	side := b.decide.side
	selected := b.decisionSelected()

	workAt, decisionAt := map[string]int{}, map[string]int{}
	for i, r := range work {
		workAt[r.id] = i
	}
	for i, r := range decisions {
		decisionAt[r.id] = i
	}
	var wires []focusWire
	linked := map[string]bool{selected: true}
	for _, e := range g.Edges {
		if e.Kind != datasource.EdgeFollows && e.Kind != datasource.EdgeCites || (e.Source != selected && e.Target != selected) {
			continue
		}
		wy, okWork := workAt[e.Source]
		dy, okDecision := decisionAt[e.Target]
		if !okWork || !okDecision {
			continue
		}
		target, _ := g.Node(e.Target)
		wires = append(wires, focusWire{work: wy, decision: dy, kind: e.Kind, status: target.Status})
		linked[e.Source], linked[e.Target] = true, true
	}

	total := max(len(work), len(decisions))
	laneWidth := max(6, min(14, width/8))
	const chipWidth = 4
	leftWidth := (width - laneWidth) / 2
	rightWidth := width - laneWidth - leftWidth
	lanes := focusLanes(s, wires, side, total, laneWidth)
	dim := selected != "" && len(wires) > 0

	head := padRight(s.strong.Render("Work · Issues"), leftWidth+laneWidth) + s.strong.Render("Knowledge · Memories")
	lines := []string{head}
	selectedRows := len(work)
	if side == 1 {
		selectedRows = len(decisions)
	}
	offset := decisionOffset(b.decide.offset, b.decide.cursor[side], selectedRows, listHeight)
	for i := offset; i < offset+listHeight; i++ {
		left := strings.Repeat(" ", leftWidth)
		right := strings.Repeat(" ", rightWidth)
		lane := strings.Repeat(" ", laneWidth)
		if i < len(work) {
			r := work[i]
			chip := ""
			if n := len(g.DecisionEdges(r.id)); n > 0 && r.id != "" && r.id != selected {
				chip = fmt.Sprintf("%d→", n)
			}
			left = workRowText(s, g, r, leftWidth-chipWidth, side == 0 && i == b.decide.cursor[0], dim && !linked[r.id]) + countChip(s, " "+chip, chipWidth)
		}
		if i < total {
			lane = lanes[i]
		}
		if i < len(decisions) {
			r := decisions[i]
			chip := ""
			if n := len(g.Followers(r.id)) + len(g.Citers(r.id)); n > 0 && r.id != "" && r.id != selected {
				chip = fmt.Sprintf("←%d", n)
			}
			right = decisionRowText(s, g, r, rightWidth-chipWidth, side == 1 && i == b.decide.cursor[1], dim && !linked[r.id]) + countChip(s, " "+chip, chipWidth)
		}
		lines = append(lines, left+lane+right)
	}
	lines = append(lines, t.MutedText.Render(strings.Repeat("─", width)))
	if selected != "" {
		strip := renderNeighbourhood(t, g, selected, width)
		lines = append(lines, strip[:min(len(strip), stripHeight)]...)
	}
	return strings.Join(lines, "\n")
}

// matrixView puts Issues on rows and decisions on columns, so a column
// reads as everyone who works to one decision.
func (b MemoryBrowser) matrixView(width, height int) string {
	t := b.theme
	g := b.graph
	s := newGraphStyles(t)
	work := b.decisionWorkRows()
	cols := b.decisionColumns()
	if len(cols) == 0 {
		return s.muted.Render("No decision Links. Issues must follow or cite a Memory to appear in the matrix.")
	}
	listHeight, _ := b.decisionLayout()
	const cell = 3
	leftWidth := max(20, min(60, width-cell*len(cols)))
	visible := max(1, (width-leftWidth)/cell)
	first := 0
	if b.decide.col >= visible {
		first = b.decide.col - visible + 1
	}
	shown := cols[first:min(len(cols), first+visible)]
	row := b.decide.cursor[0]

	var labels, bars strings.Builder
	labels.WriteString(padRight(s.strong.Render("Work · Issues"), leftWidth))
	bars.WriteString(strings.Repeat(" ", leftWidth))
	for i, id := range shown {
		n, _ := g.Node(id)
		style := s.memoryStatus(n.Status)
		if first+i == b.decide.col {
			style = style.Bold(true).Underline(true)
		}
		labels.WriteString(" " + style.Render(padRight(matrixLabel(n), cell-1)))
		bars.WriteString(s.memoryStatus(n.Status).Render(" ━ "))
	}
	lines := []string{labels.String(), bars.String()}
	offset := decisionOffset(b.decide.offset, row, len(work), listHeight)
	for i := offset; i < offset+listHeight && i < len(work); i++ {
		r := work[i]
		line := workRowText(s, g, r, leftWidth, i == row, false)
		if r.id != "" {
			kinds := map[string]string{}
			for _, e := range g.DecisionEdges(r.id) {
				kinds[e.Target] = e.Kind
			}
			for j, id := range shown {
				n, _ := g.Node(id)
				mark := s.muted.Render(" · ")
				switch kinds[id] {
				case datasource.EdgeFollows:
					mark = s.memoryStatus(n.Status).Render(" ● ")
				case datasource.EdgeCites:
					mark = s.memoryStatus(n.Status).Render(" ○ ")
				}
				if i == row && first+j == b.decide.col {
					mark = s.selected.Render(ansi.Strip(mark))
				}
				line += mark
			}
		}
		lines = append(lines, line)
	}
	if len(work) == 0 {
		lines = append(lines, s.muted.Render("No Issues match the problems filter. Press p to show all Issues."))
	}
	lines = append(lines, t.MutedText.Render(strings.Repeat("─", width)))
	if b.decide.col < len(cols) {
		id := cols[b.decide.col]
		n, _ := g.Node(id)
		issue := ""
		if row < len(work) {
			issue = work[row].id
		}
		kind := " has no Link to "
		for _, e := range g.DecisionEdges(issue) {
			if e.Target == id {
				kind = " " + e.Kind + " "
			}
		}
		status := n.Status
		if status == "" {
			status = "no status"
		}
		lines = append(lines,
			s.strong.Render("cell  ")+issue+s.muted.Render(kind)+s.memoryStatus(n.Status).Render(g.Label(id)+" "+graphTitle(n))+s.muted.Render("  ("+status+")"),
			s.muted.Render(fmt.Sprintf("column: followed by %d, cited by %d", len(g.Followers(id)), len(g.Citers(id)))),
		)
	}
	lines = append(lines, s.muted.Render("legend  ● follows  ○ cites  ")+s.active.Render("━ active")+" "+s.proposed.Render("━ proposed")+" "+s.dead.Render("━ superseded")+" "+s.muted.Render("━ no status"))
	return strings.Join(lines, "\n")
}

// decisionChip is a decision as a badge: filled for follows, bracketed for
// cites, with ✗ for a superseded decision and ? for a proposed one so the
// verdict reads without colour.
func decisionChip(s graphStyles, n datasource.GraphNode, kind string, current bool) string {
	text := decisionTag(n)
	switch n.Status {
	case "superseded":
		text += "✗"
	case "proposed":
		text += "?"
	}
	style := s.memoryStatus(n.Status).Underline(current)
	if kind == datasource.EdgeFollows {
		return style.Reverse(true).Render(" " + text + " ")
	}
	return style.Render("(" + text + ")")
}

// chipsView lists the work with its decisions inline, as the DECISIONS
// column does in the main TUI, and explains the selected Issue in a box.
func (b MemoryBrowser) chipsView(width, height int) string {
	t := b.theme
	g := b.graph
	s := newGraphStyles(t)
	work := b.decisionWorkRows()
	listHeight, _ := b.decisionLayout()
	boxWidth := max(24, min(48, width/3))
	leftWidth := width - boxWidth - 1
	stacked := width < 60
	if stacked {
		boxWidth, leftWidth = width, width
		listHeight = max(1, (height-3)/2)
	}
	row := b.decide.cursor[0]

	left := []string{padRight(s.strong.Render("Work · Issues")+s.muted.Render("  decisions: filled = follows, (n) = cites"), leftWidth)}
	offset := decisionOffset(b.decide.offset, row, len(work), listHeight)
	for i := offset; i < offset+listHeight && i < len(work); i++ {
		r := work[i]
		var chips []string
		for j, e := range g.DecisionEdges(r.id) {
			n, _ := g.Node(e.Target)
			chips = append(chips, decisionChip(s, n, e.Kind, i == row && j == b.decide.chip))
		}
		first := 0
		if i == row {
			for first < b.decide.chip && lipgloss.Width(strings.Join(chips[first:b.decide.chip+1], " ")) > leftWidth/2 {
				first++
			}
		}
		extra := strings.Join(chips[first:], " ")
		if r.id != "" && len(chips) == 0 && g.Health(r.id) == datasource.HealthNone {
			extra = s.muted.Render("no decision")
		}
		extra = ansi.Truncate(extra, leftWidth/2, "…")
		textWidth := leftWidth - lipgloss.Width(extra) - 1
		left = append(left, workRowText(s, g, r, textWidth, i == row, false)+" "+extra)
	}

	var detail []string
	if row < len(work) && work[row].id != "" {
		id := work[row].id
		n, _ := g.Node(id)
		detail = append(detail, s.strong.Render(id), graphTitle(n))
		if text := healthText(g.Health(id)); text != "" {
			detail = append(detail, s.health(g.Health(id)).Render(text))
		}
		links := g.DecisionEdges(id)
		for j := min(b.decide.chip, len(links)); j < len(links); j++ {
			e := links[j]
			mem, _ := g.Node(e.Target)
			status := mem.Status
			if status == "" {
				status = "no status"
			}
			detail = append(detail, "", decisionChip(s, mem, e.Kind, j == b.decide.chip)+" "+s.muted.Render(e.Kind+" · "+status), graphTitle(mem))
			if e.Note != "" {
				detail = append(detail, s.muted.Render("“"+e.Note+"”"))
			}
			var others []string
			for _, other := range append(g.Followers(e.Target), g.Citers(e.Target)...) {
				if other != id {
					others = append(others, other)
				}
			}
			if len(others) > 0 {
				detail = append(detail, s.muted.Render("shared with "+strings.Join(others, ", ")))
			}
		}
	}
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(t.MutedText.GetForeground()).
		Padding(0, 1).Width(boxWidth - 2).Render(strings.Join(detail, "\n"))
	right := strings.Split(box, "\n")
	if stacked {
		return strings.Join(append(left, right...), "\n")
	}
	lines := make([]string, max(len(left), min(len(right), listHeight)))
	for i := range lines {
		l := strings.Repeat(" ", leftWidth)
		if i < len(left) {
			l = padRight(left[i], leftWidth)
		}
		if i < len(right) && i < listHeight {
			l += " " + right[i]
		}
		lines[i] = l
	}
	return strings.Join(lines, "\n")
}
