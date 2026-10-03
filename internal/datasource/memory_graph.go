package datasource

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/vanderheijden86/b9s/pkg/model"
)

// Edge kinds of a MemoryGraph. The preview stores Link types as scope URLs;
// these are the meanings the views draw.
const (
	EdgeFollows    = "follows"
	EdgeCites      = "cites"
	EdgeChildOf    = "child-of"
	EdgeSupersedes = "supersedes"
	EdgeBlocks     = "blocks"
	EdgeRelated    = "related"
)

// Health is the one verdict each Issue gets, so every view flags the same
// problems. The order is the order of severity a reader triages in.
type Health uint8

const (
	HealthNotIssue Health = iota
	HealthOK
	HealthEpic
	HealthNone
	HealthProposed
	HealthDead
)

func (h Health) String() string {
	switch h {
	case HealthOK:
		return "Follows active decisions"
	case HealthEpic:
		return "Epic"
	case HealthNone:
		return "No decision recorded"
	case HealthProposed:
		return "Follows a decision that is still proposed"
	case HealthDead:
		return "Follows a superseded decision"
	default:
		return ""
	}
}

// Problem reports whether the verdict needs a reader's attention.
func (h Health) Problem() bool {
	return h == HealthNone || h == HealthProposed || h == HealthDead
}

// GraphNode is one Bead of a MemoryGraph. Status is the frontmatter status
// for a Memory and the Issue status for an Issue.
type GraphNode struct {
	ID      string
	Kind    string
	Title   string
	Body    string
	Version string
	Status  string
	// Number, Date and SupersededBy come from a Memory's frontmatter.
	Number       string
	Date         string
	SupersededBy string
	// Issue holds the decoded properties of an Issue node.
	Issue model.Issue
}

// GraphEdge is one Link between two nodes, by short Bead id.
type GraphEdge struct {
	ID     string
	Kind   string
	Source string
	Target string
	Note   string
}

// Neighbour is one Link seen from one end.
type Neighbour struct {
	Edge  GraphEdge
	Other string
	Out   bool
}

// MemoryGraph is every current Bead and Link of a graph workspace.
type MemoryGraph struct {
	Nodes []GraphNode
	Edges []GraphEdge
	index map[string]int
	out   map[string][]int
	in    map[string][]int
}

// defaultTraversalWorkers bounds concurrent `bd graph` processes. Each one
// opens the embedded store, so the bound protects the machine more than it
// buys speed.
const defaultTraversalWorkers = 6

// Graph reads the whole graph: the inventory for every Bead and its body,
// then one generic traversal per connected component for the Links, since
// a Link written by `bd link` is owned by no Bead and the inventory omits it.
func (c *GraphPreviewClient) Graph(ctx context.Context) (MemoryGraph, error) {
	inv, err := c.Inventory(ctx)
	if err != nil {
		return MemoryGraph{}, err
	}
	seeds := componentSeeds(inv)
	workers := c.traversalWorkers
	if workers <= 0 {
		workers = defaultTraversalWorkers
	}

	var (
		mu       sync.Mutex
		wg       sync.WaitGroup
		firstErr error
		visited  = map[string]bool{}
		links    []GraphLink
		sem      = make(chan struct{}, workers)
	)
	for _, seed := range seeds {
		sem <- struct{}{}
		mu.Lock()
		skip := visited[seed] || firstErr != nil
		if !skip {
			visited[seed] = true
		}
		mu.Unlock()
		if skip {
			<-sem
			continue
		}
		wg.Add(1)
		go func(seed string) {
			defer wg.Done()
			defer func() { <-sem }()
			nodes, found, err := c.traverse(ctx, seed)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				return
			}
			for _, id := range nodes {
				visited[id] = true
			}
			links = append(links, found...)
		}(seed)
	}
	wg.Wait()
	if firstErr != nil {
		return MemoryGraph{}, firstErr
	}
	for _, bead := range inv.Beads {
		links = append(links, bead.Owned...)
	}
	return buildMemoryGraph(inv.Beads, links), nil
}

// componentSeeds orders traversal roots so that Beads already joined by an
// owned Link share one root: the owned Links are a subset of each component.
func componentSeeds(inv GraphPreview) []string {
	parent := map[string]string{}
	var find func(string) string
	find = func(x string) string {
		if p, ok := parent[x]; ok && p != x {
			root := find(p)
			parent[x] = root
			return root
		}
		return x
	}
	for _, bead := range inv.Beads {
		parent[shortBeadID(bead.ID)] = shortBeadID(bead.ID)
	}
	for _, l := range inv.Links {
		a, b := find(shortBeadID(l.Source)), find(shortBeadID(l.Target))
		if _, ok := parent[a]; !ok {
			continue
		}
		if _, ok := parent[b]; !ok {
			continue
		}
		parent[a] = b
	}
	seen := map[string]bool{}
	var seeds []string
	for _, bead := range inv.Beads {
		id := shortBeadID(bead.ID)
		if root := find(id); !seen[root] {
			seen[root] = true
			seeds = append(seeds, id)
		}
	}
	return seeds
}

// traverse returns the short ids and Links of seed's connected component.
func (c *GraphPreviewClient) traverse(ctx context.Context, seed string) ([]string, []GraphLink, error) {
	out, err := c.run(ctx, "graph", seed, "--view", "generic", "--depth", "1000",
		"--max-nodes", "1000", "--max-links", "1000", "--direction", "both", "--json")
	if err != nil {
		return nil, nil, err
	}
	var payload struct {
		Preview bool `json:"preview"`
		Result  struct {
			Complete bool `json:"complete"`
			Nodes    []struct {
				ID string `json:"id"`
			} `json:"nodes"`
			Links []GraphLink `json:"links"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return nil, nil, fmt.Errorf("parse graph traversal from %s: %w", seed, err)
	}
	if !payload.Preview {
		return nil, nil, fmt.Errorf("bd did not return a graph traversal")
	}
	if !payload.Result.Complete {
		return nil, nil, fmt.Errorf("graph traversal from %s is incomplete; the component exceeds the CLI bounds", seed)
	}
	ids := make([]string, 0, len(payload.Result.Nodes))
	for _, n := range payload.Result.Nodes {
		ids = append(ids, shortBeadID(n.ID))
	}
	return ids, payload.Result.Links, nil
}

// shortBeadID strips the scope URL from a Bead or Link id.
func shortBeadID(id string) string {
	if i := strings.LastIndex(id, "/"); i >= 0 {
		return id[i+1:]
	}
	return id
}

func buildMemoryGraph(beads []GraphBead, links []GraphLink) MemoryGraph {
	g := MemoryGraph{index: map[string]int{}, out: map[string][]int{}, in: map[string][]int{}}
	for _, bead := range beads {
		n := GraphNode{
			ID:      shortBeadID(bead.ID),
			Kind:    bead.Kind,
			Title:   bead.Properties.Title,
			Body:    bead.Properties.Body,
			Version: bead.Version,
		}
		switch bead.Kind {
		case "memory":
			fm := parseMemoryFrontmatter(n.Body)
			n.Status, n.Number, n.Date, n.SupersededBy = fm.Status, fm.ID, fm.Date, fm.SupersededBy
		case "issue":
			n.Issue = decodeGraphIssue(n.ID, bead.rawProperties)
			n.Status = string(n.Issue.Status)
		}
		g.index[n.ID] = len(g.Nodes)
		g.Nodes = append(g.Nodes, n)
	}

	seen := map[string]bool{}
	for _, l := range links {
		e := GraphEdge{ID: shortBeadID(l.ID), Source: shortBeadID(l.Source), Target: shortBeadID(l.Target), Note: l.Properties.Note}
		if seen[e.ID] {
			continue
		}
		if _, ok := g.index[e.Source]; !ok {
			continue
		}
		if _, ok := g.index[e.Target]; !ok {
			continue
		}
		seen[e.ID] = true
		e.Kind = g.edgeKind(l.Type, e)
		g.Edges = append(g.Edges, e)
	}
	g.applySupersession()
	g.reindex()
	return g
}

// NewMemoryGraph builds a graph from nodes and edges whose kinds are already
// decided. Edges with an unknown endpoint are dropped.
func NewMemoryGraph(nodes []GraphNode, edges []GraphEdge) MemoryGraph {
	g := MemoryGraph{Nodes: nodes}
	g.reindex()
	kept := g.Edges[:0]
	for _, e := range edges {
		_, src := g.index[e.Source]
		_, dst := g.index[e.Target]
		if src && dst {
			kept = append(kept, e)
		}
	}
	g.Edges = kept
	g.reindex()
	return g
}

func (g *MemoryGraph) reindex() {
	g.index = make(map[string]int, len(g.Nodes))
	g.out = map[string][]int{}
	g.in = map[string][]int{}
	for i, n := range g.Nodes {
		g.index[n.ID] = i
	}
	for i, e := range g.Edges {
		g.out[e.Source] = append(g.out[e.Source], i)
		g.in[e.Target] = append(g.in[e.Target], i)
	}
}

// edgeKind reads a Link's meaning from its type name. The preview has no
// hierarchy type, so an Issue-to-Issue related Link whose id names the
// relation "child-of" stands for one; that is the convention of the POC data.
func (g *MemoryGraph) edgeKind(linkType string, e GraphEdge) string {
	name := shortBeadID(linkType)
	switch {
	case strings.HasSuffix(name, "follows"):
		return EdgeFollows
	case strings.HasSuffix(name, "cites"):
		return EdgeCites
	case strings.HasSuffix(name, "supersedes"):
		return EdgeSupersedes
	case strings.Contains(name, "blocks"):
		return EdgeBlocks
	}
	src, dst := g.Nodes[g.index[e.Source]], g.Nodes[g.index[e.Target]]
	if strings.Contains(e.ID, "-child-of-") && src.Kind == "issue" && dst.Kind == "issue" {
		return EdgeChildOf
	}
	return EdgeRelated
}

// applySupersession draws each frontmatter superseded_by as a supersedes edge
// from the replacing Memory to the replaced one. A related Link that already
// joins the pair becomes that edge, so the pair is not drawn twice.
func (g *MemoryGraph) applySupersession() {
	byNumber := map[string]string{}
	for _, n := range g.Nodes {
		if n.Kind == "memory" && n.Number != "" {
			byNumber[n.Number] = n.ID
		}
	}
	for _, old := range g.Nodes {
		if old.Kind != "memory" || old.SupersededBy == "" {
			continue
		}
		newer, ok := byNumber[old.SupersededBy]
		if !ok {
			continue
		}
		joined := false
		for i, e := range g.Edges {
			if e.Kind == EdgeRelated && ((e.Source == old.ID && e.Target == newer) || (e.Source == newer && e.Target == old.ID)) {
				g.Edges[i].Kind, g.Edges[i].Source, g.Edges[i].Target = EdgeSupersedes, newer, old.ID
				joined = true
			}
		}
		if !joined {
			g.Edges = append(g.Edges, GraphEdge{
				ID: "frontmatter:" + newer + "-supersedes-" + old.ID, Kind: EdgeSupersedes,
				Source: newer, Target: old.ID, Note: "superseded_by in the frontmatter",
			})
		}
	}
}

func decodeGraphIssue(id string, raw json.RawMessage) model.Issue {
	var issue model.Issue
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &issue)
	}
	issue.ID = id
	if issue.Status == "" {
		issue.Status = model.StatusOpen
	}
	return issue
}

// memoryFrontmatter is the part of an ADR's YAML frontmatter the views use.
type memoryFrontmatter struct {
	ID, Status, Date, SupersededBy string
}

// parseMemoryFrontmatter reads the first `---` block of a Memory body. An ADR
// imported as a Memory keeps a provenance preamble above its frontmatter, so
// the block need not open the body. Later `---` lines are Markdown rules.
func parseMemoryFrontmatter(body string) memoryFrontmatter {
	var fm memoryFrontmatter
	lines := strings.Split(body, "\n")
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "---" {
			start = i
			break
		}
	}
	if start < 0 {
		return fm
	}
	for _, line := range lines[start+1:] {
		if strings.TrimSpace(line) == "---" {
			return fm
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		switch strings.TrimSpace(key) {
		case "id":
			fm.ID = value
		case "status":
			fm.Status = value
		case "date":
			fm.Date = value
		case "superseded_by":
			fm.SupersededBy = value
		}
	}
	// An unclosed block is prose, not frontmatter.
	return memoryFrontmatter{}
}

// Node returns the node with this short id.
func (g MemoryGraph) Node(id string) (GraphNode, bool) {
	i, ok := g.index[id]
	if !ok {
		return GraphNode{}, false
	}
	return g.Nodes[i], true
}

// Label names a node the way a reader names it: an ADR by its number.
func (g MemoryGraph) Label(id string) string {
	if n, ok := g.Node(id); ok && n.Kind == "memory" && n.Number != "" {
		return "ADR " + n.Number
	}
	return id
}

// Out and In return the edges leaving and entering a node.
func (g MemoryGraph) Out(id string) []GraphEdge { return g.pick(g.out[id]) }
func (g MemoryGraph) In(id string) []GraphEdge  { return g.pick(g.in[id]) }

func (g MemoryGraph) pick(idx []int) []GraphEdge {
	edges := make([]GraphEdge, 0, len(idx))
	for _, i := range idx {
		edges = append(edges, g.Edges[i])
	}
	return edges
}

// Neighbours lists every Link of a node, outgoing first.
func (g MemoryGraph) Neighbours(id string) []Neighbour {
	var nb []Neighbour
	for _, e := range g.Out(id) {
		nb = append(nb, Neighbour{Edge: e, Other: e.Target, Out: true})
	}
	for _, e := range g.In(id) {
		nb = append(nb, Neighbour{Edge: e, Other: e.Source})
	}
	return nb
}

// DecisionEdges are the follows and cites Links an Issue owns.
func (g MemoryGraph) DecisionEdges(id string) []GraphEdge {
	var d []GraphEdge
	for _, e := range g.Out(id) {
		if e.Kind == EdgeFollows || e.Kind == EdgeCites {
			d = append(d, e)
		}
	}
	return d
}

func (g MemoryGraph) sources(id, kind string) []string {
	var ids []string
	for _, e := range g.In(id) {
		if e.Kind == kind {
			ids = append(ids, e.Source)
		}
	}
	return ids
}

// Followers and Citers list the nodes that follow or cite a decision.
func (g MemoryGraph) Followers(id string) []string { return g.sources(id, EdgeFollows) }
func (g MemoryGraph) Citers(id string) []string    { return g.sources(id, EdgeCites) }

// ReplacedBy returns the Memory that supersedes id, if any.
func (g MemoryGraph) ReplacedBy(id string) (string, bool) {
	for _, e := range g.In(id) {
		if e.Kind == EdgeSupersedes {
			return e.Source, true
		}
	}
	return "", false
}

// Health is the verdict for one Issue. Following a superseded decision
// outranks being an epic, because an epic built on a retired decision is
// the costliest case to miss.
func (g MemoryGraph) Health(id string) Health {
	n, ok := g.Node(id)
	if !ok || n.Kind != "issue" {
		return HealthNotIssue
	}
	d := g.DecisionEdges(id)
	status := func(e GraphEdge) string {
		m, _ := g.Node(e.Target)
		return m.Status
	}
	for _, e := range d {
		if e.Kind == EdgeFollows && status(e) == "superseded" {
			return HealthDead
		}
	}
	if n.Issue.IssueType == model.TypeEpic {
		return HealthEpic
	}
	if len(d) == 0 {
		return HealthNone
	}
	for _, e := range d {
		if e.Kind == EdgeFollows && status(e) == "proposed" {
			return HealthProposed
		}
	}
	return HealthOK
}

// Issues converts the Issue nodes to the model the main TUI reads. Child-of
// edges become parent-child dependencies and blocks Links become blocking
// ones; Memories and their Links stay in the graph.
func (g MemoryGraph) Issues() []model.Issue {
	var issues []model.Issue
	for _, n := range g.Nodes {
		if n.Kind != "issue" {
			continue
		}
		issue := n.Issue
		issue.Dependencies = nil
		for _, e := range g.Out(n.ID) {
			var t model.DependencyType
			switch e.Kind {
			case EdgeChildOf:
				t = model.DepParentChild
			case EdgeBlocks:
				t = model.DepBlocks
			default:
				continue
			}
			issue.Dependencies = append(issue.Dependencies, &model.Dependency{IssueID: n.ID, DependsOnID: e.Target, Type: t})
		}
		issues = append(issues, issue)
	}
	return issues
}

// UnmarshalJSON keeps the raw properties beside the typed ones, because an
// Issue's properties carry the whole Issue record.
func (b *GraphBead) UnmarshalJSON(data []byte) error {
	type plain GraphBead
	var p plain
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	var raw struct {
		Properties json.RawMessage `json:"properties"`
	}
	if err := json.NewDecoder(bytes.NewReader(data)).Decode(&raw); err != nil {
		return err
	}
	*b = GraphBead(p)
	b.rawProperties = raw.Properties
	return nil
}
