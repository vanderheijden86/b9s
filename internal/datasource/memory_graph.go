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
	EdgeFollows = "follows"
	EdgeCites   = "cites"
	EdgeBlocks  = "blocks"
	EdgeRelated = "related"
)

// GraphNode preserves stored content. Status belongs only to Issues.
type GraphNode struct {
	ID      string
	Kind    string
	Title   string
	Body    string
	Version string
	Status  string
	Type    string
	// Issue holds the decoded properties of an Issue node.
	Issue model.Issue
}

// GraphEdge is one Link between two nodes, by short Bead id.
type GraphEdge struct {
	ID     string
	Type   string
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

// Graph reads every Bead and its body from the inventory. Memory-owned Links
// are complete there; Issue informational Links require generic traversal,
// one per Issue component. After the inventory and after each component,
// progress, if not nil, receives how many components are done; one reached
// by an earlier traversal counts as done without its own. It is called from
// one goroutine at a time.
func (c *GraphPreviewClient) Graph(ctx context.Context, progress func(done, total int)) (MemoryGraph, error) {
	inv, err := c.Inventory(ctx)
	if err != nil {
		return MemoryGraph{}, err
	}
	seeds := componentSeeds(inv)
	done := 0
	report := func() {
		if progress != nil {
			progress(done, len(seeds))
		}
	}
	report()
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
		if skip {
			done++
			report()
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
			done++
			report()
		}(seed)
	}
	wg.Wait()
	if firstErr != nil {
		return MemoryGraph{}, firstErr
	}
	// Traversals return summaries; the complete owned records carry properties.
	return buildMemoryGraph(inv.Beads, append(inv.Links, links...)), nil
}

// componentSeeds groups roots by known owned Links. The preview-memory-v2
// descriptor owns all outgoing Links, so a Memory-only component is complete
// in the inventory. Issue and unknown Types still require traversal (ADR 0040).
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
		if bead.Kind == "memory" {
			continue
		}
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
			Type:    bead.Type,
		}
		switch bead.Kind {
		case "issue":
			n.Issue = decodeGraphIssue(n.ID, bead.rawProperties)
			n.Status = string(n.Issue.Status)
		}
		g.index[n.ID] = len(g.Nodes)
		g.Nodes = append(g.Nodes, n)
	}

	seen := map[string]bool{}
	for _, l := range links {
		e := GraphEdge{ID: shortBeadID(l.ID), Type: l.Type, Source: shortBeadID(l.Source), Target: shortBeadID(l.Target), Note: l.Properties.Note}
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
		e.Kind = edgeKind(l.Type)
		g.Edges = append(g.Edges, e)
	}
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

// Only exact installed preview Type names receive a display label.
func edgeKind(linkType string) string {
	switch shortBeadID(linkType) {
	case "example-follows":
		return EdgeFollows
	case "example-cites":
		return EdgeCites
	case "preview-blocks-v1":
		return EdgeBlocks
	case "preview-related-v2":
		return EdgeRelated
	}
	return shortBeadID(linkType)
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

// Node returns the node with this short id.
func (g MemoryGraph) Node(id string) (GraphNode, bool) {
	i, ok := g.index[id]
	if !ok {
		return GraphNode{}, false
	}
	return g.Nodes[i], true
}

func (g MemoryGraph) Label(id string) string { return id }

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

// MemoryLinks includes both directions and every stored Type, without ownership assumptions.
func (g MemoryGraph) MemoryLinks(id string) []Neighbour {
	var links []Neighbour
	for _, n := range g.Neighbours(id) {
		if other, ok := g.Node(n.Other); ok && other.Kind == "memory" {
			links = append(links, n)
		}
	}
	return links
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

// Issues converts only the installed blocking Link Type to scheduling dependencies.
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
