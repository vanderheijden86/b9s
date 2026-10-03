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

// memoryMode is the view the Memory browser shows. The two graph views read
// the whole graph, which costs one traversal per component, so they load it
// on first entry and share it (ADR 0032).
type memoryMode uint8

const (
	memoryModeList memoryMode = iota
	memoryModeConstellation
	memoryModeShores
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

type shoresState struct {
	side   int
	cursor [2]int
	offset int
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
	left, right := shoresRows(msg.graph)
	b.shore.cursor = [2]int{firstShoreRow(left), firstShoreRow(right)}
	return b
}

func (b MemoryBrowser) handleGraphKey(key string) (MemoryBrowser, tea.Cmd) {
	if b.graphLoad != graphRead || b.graphErr != nil {
		return b, nil
	}
	if b.mode == memoryModeConstellation {
		return b.handleConstellationKey(key)
	}
	return b.handleShoresKey(key)
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
	}
	return b.shoresView(width, height)
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
	name := "constellation"
	if b.mode == memoryModeShores {
		name = "two shores"
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
// Two shores

// shoreRow is a line of one shore: a group header or a node.
type shoreRow struct {
	id     string
	header string
	depth  int
}

func firstShoreRow(rows []shoreRow) int {
	for i, r := range rows {
		if r.id != "" {
			return i
		}
	}
	return 0
}

// shoresRows lays out work on the left as the epic tree, then loose work,
// then empty epics; and decisions on the right by status, most used first.
func shoresRows(g datasource.MemoryGraph) (left, right []shoreRow) {
	kids := map[string][]string{}
	parent := map[string]string{}
	for _, e := range g.Edges {
		if e.Kind == datasource.EdgeChildOf {
			kids[e.Target] = append(kids[e.Target], e.Source)
			parent[e.Source] = e.Target
		}
	}
	var walk func(id string, depth int)
	walk = func(id string, depth int) {
		left = append(left, shoreRow{id: id, depth: depth})
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
		left = append(left, shoreRow{header: "No epic"})
		for _, id := range loose {
			walk(id, 0)
		}
	}
	if len(empty) > 0 {
		left = append(left, shoreRow{header: "Epics with no Links"})
		for _, id := range empty {
			left = append(left, shoreRow{id: id})
		}
	}
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
		right = append(right, shoreRow{header: header})
		for _, id := range ids {
			right = append(right, shoreRow{id: id})
		}
	}
	return left, right
}

func knownMemoryStatus(status string) bool {
	return status == "active" || status == "proposed" || status == "superseded"
}

// shoresTraced is the selected row, everything under it in the epic tree,
// and the far end of every Link those rows own.
func shoresTraced(g datasource.MemoryGraph, id string) map[string]bool {
	on := map[string]bool{}
	if id == "" {
		return on
	}
	var descend func(string)
	descend = func(x string) {
		on[x] = true
		for _, e := range g.In(x) {
			if e.Kind == datasource.EdgeChildOf && !on[e.Source] {
				descend(e.Source)
			}
		}
	}
	descend(id)
	owners := make([]string, 0, len(on))
	for x := range on {
		owners = append(owners, x)
	}
	for _, x := range owners {
		for _, nb := range g.Neighbours(x) {
			if nb.Edge.Kind != datasource.EdgeChildOf {
				on[nb.Other] = true
			}
		}
	}
	return on
}

func (b MemoryBrowser) shoresSelected() string {
	left, right := shoresRows(b.graph)
	rows := [2][]shoreRow{left, right}[b.shore.side]
	if c := b.shore.cursor[b.shore.side]; c < len(rows) {
		return rows[c].id
	}
	return ""
}

// shoresLayout splits the pane height into the two shores and the strip that
// shows the selected row's neighbourhood.
func (b MemoryBrowser) shoresLayout() (listHeight, stripHeight int) {
	height := max(1, b.height-4)
	stripHeight = min(12, max(4, height/3))
	return max(1, height-stripHeight-2), stripHeight
}

func (b MemoryBrowser) handleShoresKey(key string) (MemoryBrowser, tea.Cmd) {
	left, right := shoresRows(b.graph)
	sides := [2][]shoreRow{left, right}
	sh := &b.shore
	rows := sides[sh.side]
	step := func(from, dir int) int {
		for i := from + dir; i >= 0 && i < len(rows); i += dir {
			if rows[i].id != "" {
				return i
			}
		}
		return from
	}
	switch key {
	case "j", "down":
		sh.cursor[sh.side] = step(sh.cursor[sh.side], 1)
	case "k", "up":
		sh.cursor[sh.side] = step(sh.cursor[sh.side], -1)
	case "g", "home":
		sh.cursor[sh.side] = firstShoreRow(rows)
	case "G", "end":
		sh.cursor[sh.side] = step(len(rows), -1)
	case "tab":
		sh.side = 1 - sh.side
	case "h", "left":
		sh.side = 0
	case "l", "right":
		sh.side = 1
	case "enter":
		return b.openMemory(b.shoresSelected())
	}
	listHeight, _ := b.shoresLayout()
	c := sh.cursor[sh.side]
	if c < sh.offset {
		sh.offset = c
	} else if c >= sh.offset+listHeight {
		sh.offset = c - listHeight + 1
	}
	return b, nil
}

// lanesNeeded is the most wires that overlap on any row, which is the lane
// count routeWires needs to draw them all.
func lanesNeeded(wires []shoreWire) int {
	ends := make([]int, 0, len(wires))
	sorted := append([]shoreWire(nil), wires...)
	sort.SliceStable(sorted, func(i, j int) bool { return min(sorted[i].from, sorted[i].to) < min(sorted[j].from, sorted[j].to) })
	for _, w := range sorted {
		lo, hi := min(w.from, w.to), max(w.from, w.to)
		placed := false
		for i, end := range ends {
			if end < lo {
				ends[i], placed = hi, true
				break
			}
		}
		if !placed {
			ends = append(ends, hi)
		}
	}
	return len(ends)
}

type wireCell struct {
	bits   uint8 // 1 up, 2 down, 4 left, 8 right
	marker rune
	style  int
	prio   int
}

type shoreWire struct {
	from, to int
	style    int
	prio     int
	traced   bool
}

var boxRunes = map[uint8]rune{
	1: '│', 2: '│', 3: '│', 4: '─', 8: '─', 12: '─',
	10: '┌', 6: '┐', 9: '└', 5: '┘', 11: '├', 7: '┤', 14: '┬', 13: '┴', 15: '┼',
}

// routeWires gives each wire its own vertical lane, reusing a lane once the
// wire before it has ended, and returns how many wires found no lane. A wire
// enters at column 0 on its first row and leaves at column exit on its last.
func routeWires(grid [][]wireCell, wires []shoreWire, lanes []int, exit int, marker rune) int {
	sort.SliceStable(wires, func(i, j int) bool { return min(wires[i].from, wires[i].to) < min(wires[j].from, wires[j].to) })
	free := make([]int, len(lanes))
	for i := range free {
		free[i] = -1
	}
	dropped := 0
	set := func(x, y int, bits uint8, w shoreWire) {
		if y < 0 || y >= len(grid) || x < 0 || x >= len(grid[y]) {
			return
		}
		c := &grid[y][x]
		c.bits |= bits
		if w.prio >= c.prio {
			c.style, c.prio = w.style, w.prio
		}
	}
	for _, w := range wires {
		lo, hi := min(w.from, w.to), max(w.from, w.to)
		lane := -1
		for i, end := range free {
			if end < lo {
				lane = i
				break
			}
		}
		if lane < 0 {
			dropped++
			continue
		}
		free[lane] = hi
		x := lanes[lane]
		for c := 0; c <= x; c++ {
			bits := uint8(4)
			if c < x {
				bits |= 8
			}
			set(c, w.from, bits, w)
		}
		for y := lo; y <= hi; y++ {
			var bits uint8
			if y > lo {
				bits |= 1
			}
			if y < hi {
				bits |= 2
			}
			set(x, y, bits, w)
		}
		from, to := x, exit
		if to < from {
			from, to = to, from
		}
		for c := from; c <= to; c++ {
			var bits uint8
			if c > from {
				bits |= 4
			}
			if c < to {
				bits |= 8
			}
			set(c, w.to, bits, w)
		}
		if w.to >= 0 && w.to < len(grid) && exit < len(grid[w.to]) {
			grid[w.to][exit].marker = marker
		}
	}
	return dropped
}

func (b MemoryBrowser) shoresView(width, height int) string {
	t := b.theme
	g := b.graph
	s := newGraphStyles(t)
	styles := []lipgloss.Style{s.text, s.muted, s.dead}
	const (
		stText = iota
		stMuted
		stDead
	)
	left, right := shoresRows(g)
	listHeight, stripHeight := b.shoresLayout()
	selected := b.shoresSelected()
	traced := shoresTraced(g, selected)

	total := max(len(left), len(right))
	leftAt, rightAt := map[string]int{}, map[string]int{}
	for i, r := range left {
		if r.id != "" {
			leftAt[r.id] = i
		}
	}
	for i, r := range right {
		if r.id != "" {
			rightAt[r.id] = i
		}
	}
	var middle, side []shoreWire
	for _, e := range g.Edges {
		target, _ := g.Node(e.Target)
		w := shoreWire{style: stMuted, prio: 1, traced: traced[e.Source] && traced[e.Target]}
		switch {
		case e.Kind == datasource.EdgeSupersedes || (e.Kind == datasource.EdgeFollows && target.Status == "superseded"):
			w.style, w.prio = stDead, 3
		case e.Kind == datasource.EdgeFollows:
			w.style, w.prio = stText, 2
		}
		if selected != "" && !w.traced {
			w.style, w.prio = stMuted, 0
		}
		ya, inLeft := leftAt[e.Source]
		yb, inRight := rightAt[e.Target]
		if inLeft && inRight && (e.Kind == datasource.EdgeFollows || e.Kind == datasource.EdgeCites) {
			w.from, w.to = ya, yb
			middle = append(middle, w)
			continue
		}
		ra, srcRight := rightAt[e.Source]
		if srcRight && inRight {
			w.from, w.to = ra, yb
			side = append(side, w)
		}
	}
	gutter := max(6, min(width/4, lanesNeeded(middle)+2))
	rightGutter := max(3, min(10, lanesNeeded(side)+2))
	leftWidth := (width - gutter - rightGutter) / 2
	rightWidth := width - gutter - rightGutter - leftWidth
	draw := func(wires []shoreWire, cols, exit int, marker rune) ([][]wireCell, int) {
		var lanes []int
		for x := 1; x < cols-1; x++ {
			lanes = append(lanes, x)
		}
		grid := make([][]wireCell, total)
		for y := range grid {
			grid[y] = make([]wireCell, cols)
		}
		probe := make([][]wireCell, total)
		for y := range probe {
			probe[y] = make([]wireCell, cols)
		}
		if routeWires(probe, append([]shoreWire(nil), wires...), lanes, exit, marker) == 0 {
			routeWires(grid, wires, lanes, exit, marker)
			return grid, 0
		}
		var keep []shoreWire
		for _, w := range wires {
			if w.traced && selected != "" {
				keep = append(keep, w)
			}
		}
		routeWires(grid, keep, lanes, exit, marker)
		return grid, len(wires) - len(keep)
	}
	midGrid, hidden := draw(middle, gutter, gutter-1, '▸')
	sideGrid, hiddenSide := draw(side, rightGutter, 0, '◂')
	hidden += hiddenSide

	renderWires := func(row []wireCell) string {
		var sb strings.Builder
		for _, c := range row {
			r := c.marker
			if r == 0 {
				r = boxRunes[c.bits]
			}
			if r == 0 {
				sb.WriteByte(' ')
				continue
			}
			sb.WriteString(styles[c.style].Render(string(r)))
		}
		return sb.String()
	}
	rowText := func(rows []shoreRow, i, w int, isLeft bool) string {
		if i >= len(rows) {
			return strings.Repeat(" ", w)
		}
		r := rows[i]
		if r.id == "" {
			style := s.muted
			if !isLeft {
				style = s.memoryStatus(r.header)
			}
			return padRight(style.Render(truncate(r.header, w)), w)
		}
		n, _ := g.Node(r.id)
		var plain, glyph string
		var glyphStyle lipgloss.Style
		if isLeft {
			h := g.Health(r.id)
			glyph, glyphStyle = healthGlyph(h), s.health(h)
			plain = strings.Repeat("  ", r.depth) + glyph + " " + r.id + " " + graphTitle(n)
		} else {
			f, c := len(g.Followers(r.id)), len(g.Citers(r.id))
			badge := ""
			if f > 0 || c > 0 {
				badge = fmt.Sprintf(" %df %dc", f, c)
			}
			glyph, glyphStyle = "●", s.memoryStatus(n.Status)
			number := n.Number
			if number == "" {
				number = r.id
			}
			body := truncate("● "+number+" "+graphTitle(n), max(1, w-len(badge)))
			plain = padRight(body, w-len(badge)) + badge
		}
		plain = padRight(truncate(plain, w), w)
		side := 0
		if !isLeft {
			side = 1
		}
		switch {
		case b.shore.side == side && b.shore.cursor[side] == i:
			return s.selected.Render(plain)
		case selected != "" && !traced[r.id]:
			return s.muted.Render(plain)
		}
		at := strings.Index(plain, glyph)
		if at < 0 {
			return plain
		}
		return plain[:at] + glyphStyle.Render(glyph) + plain[at+len(glyph):]
	}

	head := padRight(s.strong.Render("Work · Issues"), leftWidth+gutter) + s.strong.Render("Knowledge · Memories")
	if hidden > 0 {
		head += t.MutedText.Render(fmt.Sprintf("  %d wires hidden, select a row to trace", hidden))
	}
	lines := []string{head}
	for i := b.shore.offset; i < b.shore.offset+listHeight; i++ {
		mid := strings.Repeat(" ", gutter)
		rg := strings.Repeat(" ", rightGutter)
		if i < total {
			mid, rg = renderWires(midGrid[i]), renderWires(sideGrid[i])
		}
		lines = append(lines, rowText(left, i, leftWidth, true)+mid+rowText(right, i, rightWidth, false)+rg)
	}
	lines = append(lines, t.MutedText.Render(strings.Repeat("─", width)))
	if selected != "" {
		strip := renderNeighbourhood(t, g, selected, width)
		if len(strip) > stripHeight {
			strip = strip[:stripHeight]
		}
		lines = append(lines, strip...)
	}
	return strings.Join(lines, "\n")
}
