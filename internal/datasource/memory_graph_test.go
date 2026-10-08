package datasource

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/vanderheijden86/b9s/pkg/model"
)

func TestNativeMemoryBodyDoesNotDefineGraphSemantics(t *testing.T) {
	g, _ := readFixtureGraph(t, 1)
	for _, n := range g.Nodes {
		if n.Kind == "memory" && n.Status != "" {
			t.Fatalf("Memory body became status: %+v", n)
		}
	}
	for _, e := range g.Edges {
		if e.ID == "seed-10" && (e.Kind != EdgeRelated || e.Source != "adr-0009" || e.Target != "adr-0027") {
			t.Fatalf("stored related Link changed: %+v", e)
		}
		if strings.Contains(e.ID, "child-of") && e.Kind != EdgeRelated {
			t.Fatalf("Link ID became hierarchy: %+v", e)
		}
	}
}

func TestMemoryPreviewGraphReadTiming(t *testing.T) {
	workspace := os.Getenv("B9S_MEMORY_PREVIEW_WORKSPACE")
	if workspace == "" {
		t.Skip("set B9S_MEMORY_PREVIEW_WORKSPACE to an isolated embedded graph preview")
	}
	metadata, err := os.ReadFile(filepath.Join(workspace, ".beads", "metadata.json"))
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		Mode  string `json:"dolt_mode"`
		Graph string `json:"graph_mode"`
		Ready bool   `json:"graph_ready"`
	}
	if err := json.Unmarshal(metadata, &config); err != nil || config.Mode != "embedded" || config.Graph != "link" || !config.Ready {
		t.Fatal("timing test requires a ready embedded graph workspace")
	}
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(key, "BEADS_") || strings.HasPrefix(key, "BD_") {
			t.Setenv(key, "")
		}
	}
	t.Setenv("BEADS_DOLT_AUTO_START", "0")
	client, err := newGraphPreviewClient(workspace, filepath.Join(workspace, ".memory-preview", "bin", "bd"))
	if err != nil {
		t.Fatal(err)
	}
	var baseline MemoryGraph
	for _, workers := range []int{0, defaultTraversalWorkers} {
		client.traversalWorkers = workers
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		start := time.Now()
		var graph MemoryGraph
		if workers == 0 {
			graph, err = readEveryGraphComponent(ctx, client)
		} else {
			graph, err = client.Graph(ctx, nil)
		}
		elapsed := time.Since(start)
		cancel()
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("workers=%d elapsed=%s nodes=%d edges=%d", workers, elapsed, len(graph.Nodes), len(graph.Edges))
		slices.SortFunc(graph.Edges, func(a, b GraphEdge) int { return strings.Compare(a.ID, b.ID) })
		if workers == 0 {
			baseline = graph
		} else {
			if !reflect.DeepEqual(baseline.Nodes, graph.Nodes) || !reflect.DeepEqual(baseline.Edges, graph.Edges) {
				t.Fatal("parallel traversal changed the complete graph")
			}
			if elapsed >= 11*time.Second {
				t.Fatalf("graph read took %s, goal is below 11 seconds", elapsed)
			}
		}
	}
}

// The baseline discovers Links without assuming any Type's ownership rules.
func readEveryGraphComponent(ctx context.Context, client *GraphPreviewClient) (MemoryGraph, error) {
	inv, err := client.Inventory(ctx)
	if err != nil {
		return MemoryGraph{}, err
	}
	visited := map[string]bool{}
	links := append([]GraphLink{}, inv.Links...)
	for _, bead := range inv.Beads {
		id := shortBeadID(bead.ID)
		if visited[id] {
			continue
		}
		nodes, found, err := client.traverse(ctx, id)
		if err != nil {
			return MemoryGraph{}, err
		}
		for _, node := range nodes {
			visited[node] = true
		}
		links = append(links, found...)
	}
	return buildMemoryGraph(inv.Beads, links), nil
}

// graphFixtureScript answers one inventory and two traversals. The inventory
// owns the child-of and blocking Links, as in the preview, where `bd link`
// writes informational Links that no Bead owns. The traversal is the only
// source of those.
const graphFixtureScript = `
S=https://x.test/p
case "$1" in
list) cat <<EOF
{"preview":true,"result":{"hasMore":false,"items":[
 {"id":"$S/beads/adr-0009","type":"$S/types/preview-memory-v2","version":"m9","properties":{"title":"[adr-0009] ADR 0009 (superseded): Colon prompt","body":"Source: x\n\n---\ntype: ADR\nid: \"0009\"\nstatus: superseded\nsuperseded_by: \"0027\"\ndate: 2026-09-14\n---\n\n## Context"},"owned":[]},
 {"id":"$S/beads/adr-0027","type":"$S/types/preview-memory-v2","version":"m27","properties":{"title":"[adr-0027] ADR 0027 (active): Flat list","body":"---\nid: \"0027\"\nstatus: active\n---\nBody"},"owned":[]},
 {"id":"$S/beads/adr-0026","type":"$S/types/preview-memory-v2","version":"m26","properties":{"title":"ADR 0026","body":"---\nid: '0026'\nstatus: proposed\n---\n"},"owned":[]},
 {"id":"$S/beads/note","type":"$S/types/preview-memory-v2","version":"mn","properties":{"title":"Plain note","body":"No frontmatter here."},"owned":[]},
 {"id":"$S/beads/sample-e","type":"$S/types/preview-issue-v2","version":"ie","properties":{"id":"legacy-1","title":"Epic E","status":"open","priority":1,"issue_type":"epic"},"owned":[]},
 {"id":"$S/beads/sample-e.1","type":"$S/types/preview-issue-v2","version":"i1","properties":{"title":"Dead work","status":"in_progress","priority":2,"issue_type":"feature","description":"d"},"owned":[
   {"id":"$S/links/sample-e.1-child-of-sample-e","type":"$S/types/preview-related-v2","source":"$S/beads/sample-e.1","target":"$S/beads/sample-e","properties":{"note":"child"}}]},
 {"id":"$S/beads/sample-e.2","type":"$S/types/preview-issue-v2","version":"i2","properties":{"title":"Proposed work","status":"open","priority":2,"issue_type":"task"},"owned":[
   {"id":"$S/links/b1","type":"$S/types/preview-blocks-v1","source":"$S/beads/sample-e.2","target":"$S/beads/sample-e.1"}]},
 {"id":"$S/beads/sample-lone","type":"$S/types/preview-issue-v2","version":"il","properties":{"title":"Lone work","status":"open","priority":3,"issue_type":"task"},"owned":[]}
]}}
EOF
;;
graph)
 case "$2" in
 sample-lone) echo '{"preview":true,"result":{"complete":true,"nodes":[{"id":"'$S'/beads/sample-lone"}],"links":[]}}' ;;
 note) echo '{"preview":true,"result":{"complete":true,"nodes":[{"id":"'$S'/beads/note"}],"links":[]}}' ;;
 *) cat <<EOF
{"preview":true,"result":{"complete":true,
 "nodes":[{"id":"$S/beads/adr-0009"},{"id":"$S/beads/adr-0027"},{"id":"$S/beads/adr-0026"},{"id":"$S/beads/sample-e"},{"id":"$S/beads/sample-e.1"},{"id":"$S/beads/sample-e.2"}],
 "links":[
  {"id":"$S/links/sample-e.1-child-of-sample-e","type":"$S/types/preview-related-v2","source":"$S/beads/sample-e.1","target":"$S/beads/sample-e"},
  {"id":"$S/links/sample-e.2-child-of-sample-e","type":"$S/types/preview-related-v2","source":"$S/beads/sample-e.2","target":"$S/beads/sample-e"},
  {"id":"$S/links/sample-e.1-follows-adr-0009","type":"$S/types/example-follows","source":"$S/beads/sample-e.1","target":"$S/beads/adr-0009"},
  {"id":"$S/links/sample-e.1-cites-adr-0027","type":"$S/types/example-cites","source":"$S/beads/sample-e.1","target":"$S/beads/adr-0027"},
  {"id":"$S/links/sample-e.2-follows-adr-0026","type":"$S/types/example-follows","source":"$S/beads/sample-e.2","target":"$S/beads/adr-0026"},
  {"id":"$S/links/seed-10","type":"$S/types/preview-related-v2","source":"$S/beads/adr-0009","target":"$S/beads/adr-0027"},
  {"id":"$S/links/b1","type":"$S/types/preview-blocks-v1","source":"$S/beads/sample-e.2","target":"$S/beads/sample-e.1"}
 ]}}
EOF
 ;;
 esac ;;
show) echo '{"preview":true,"result":{"id":"'$S'/beads/'$2'","owned":[]}}' ;;
esac
`

func readFixtureGraph(t *testing.T, workers int) (MemoryGraph, string) {
	t.Helper()
	bd, log := fakeGraphBd(t, graphFixtureScript)
	client, err := newGraphPreviewClient(graphWorkspace(t), bd)
	if err != nil {
		t.Fatal(err)
	}
	client.traversalWorkers = workers
	graph, err := client.Graph(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	calls, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	return graph, string(calls)
}

func TestGraphTraversesIssueComponentsAndKeepsIsolatedMemories(t *testing.T) {
	graph, calls := readFixtureGraph(t, 1)
	if got := strings.Count(calls, "graph "); got != 2 {
		t.Fatalf("traversals = %d, want one per Issue component (2):\n%s", got, calls)
	}
	if strings.Contains(calls, "show ") {
		t.Fatalf("status must come from the inventory bodies, not one bd show per Memory:\n%s", calls)
	}
	if !strings.Contains(calls, "--view generic") || !strings.Contains(calls, "--direction both") {
		t.Fatalf("traversal must walk the generic graph both ways:\n%s", calls)
	}
	if len(graph.Nodes) != 8 {
		t.Fatalf("nodes = %d, want 8", len(graph.Nodes))
	}
}

func TestGraphKeepsMemoryOwnedLinksWithoutTraversal(t *testing.T) {
	bd, log := fakeGraphBd(t, `
case "$1" in
list) echo '{"preview":true,"result":{"hasMore":false,"items":[
{"id":"https://x.test/beads/a","type":"https://x.test/types/preview-memory-v2","properties":{"title":"A"},"owned":[{"id":"https://x.test/links/a-b","type":"https://x.test/types/example-cites","source":"https://x.test/beads/a","target":"https://x.test/beads/b"}]},
{"id":"https://x.test/beads/b","type":"https://x.test/types/preview-memory-v2","properties":{"title":"B"},"owned":[]}]}}' ;;
*) exit 42 ;;
esac
`)
	client, err := newGraphPreviewClient(graphWorkspace(t), bd)
	if err != nil {
		t.Fatal(err)
	}
	graph, err := client.Graph(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Nodes) != 2 || len(graph.Edges) != 1 || graph.Edges[0].Kind != EdgeCites {
		t.Fatalf("Memory-owned graph lost data: %+v", graph)
	}
	calls, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(calls), "graph ") {
		t.Fatalf("unexpected traversal: %s", calls)
	}
}

func TestComponentSeedsTraverseUnknownTypes(t *testing.T) {
	inv := GraphPreview{Beads: []GraphBead{
		{ID: "memory", Kind: "memory"},
		{ID: "unknown", Kind: "other"},
		{ID: "issue", Kind: "issue"},
	}}
	if got := componentSeeds(inv); !reflect.DeepEqual(got, []string{"unknown", "issue"}) {
		t.Fatalf("seeds = %v, want unknown and Issue", got)
	}
}

func TestGraphKeepsLinksOnlyTheTraversalReturns(t *testing.T) {
	graph, _ := readFixtureGraph(t, 4)
	kinds := map[string]int{}
	for _, e := range graph.Edges {
		kinds[e.Kind]++
	}
	want := map[string]int{EdgeRelated: 3, EdgeFollows: 2, EdgeCites: 1, EdgeBlocks: 1}
	for k, n := range want {
		if kinds[k] != n {
			t.Fatalf("edge kinds = %v, want %v", kinds, want)
		}
	}
	if len(graph.Edges) != 7 {
		t.Fatalf("edges = %d, want 7 without duplicates", len(graph.Edges))
	}
}

func TestGraphPrefersCompleteOwnedLinkProperties(t *testing.T) {
	graph, _ := readFixtureGraph(t, 1)
	for _, edge := range graph.Edges {
		if edge.ID == "sample-e.1-child-of-sample-e" {
			if edge.Note != "child" {
				t.Fatalf("owned Link note = %q, want child", edge.Note)
			}
			return
		}
	}
	t.Fatal("owned Link missing")
}

func TestGraphPreservesNativeMemoryTitlesAndBodies(t *testing.T) {
	graph, _ := readFixtureGraph(t, 1)
	for _, id := range []string{"adr-0009", "adr-0027", "adr-0026", "note"} {
		n, ok := graph.Node(id)
		if !ok {
			t.Fatalf("missing node %s", id)
		}
		if n.Status != "" || n.Title == "" || n.Body == "" {
			t.Fatalf("Memory record lost its native content: %+v", n)
		}
	}
	if got := graph.Label("adr-0009"); got != "adr-0009" {
		t.Fatalf("label = %q", got)
	}
	if got := graph.Label("note"); got != "note" {
		t.Fatalf("label without frontmatter id = %q", got)
	}
}

func TestGraphDoesNotInventSupersession(t *testing.T) {
	graph, _ := readFixtureGraph(t, 1)
	var found bool
	for _, e := range graph.Edges {
		if e.ID == "seed-10" {
			found = true
			if e.Kind != EdgeRelated || e.Source != "adr-0009" || e.Target != "adr-0027" {
				t.Fatalf("stored edge changed: %+v", e)
			}
		}
		if e.Kind == "supersedes" || strings.HasPrefix(e.ID, "frontmatter:") {
			t.Fatalf("invented Link: %+v", e)
		}
	}
	if !found {
		t.Fatal("stored related Link missing")
	}
}

func TestGraphPreservesNativeFollowersAndCiters(t *testing.T) {
	graph, _ := readFixtureGraph(t, 1)
	if f := graph.Followers("adr-0009"); len(f) != 1 || f[0] != "sample-e.1" {
		t.Fatalf("followers = %v", f)
	}
	if c := graph.Citers("adr-0027"); len(c) != 1 {
		t.Fatalf("citers = %v", c)
	}
}

func TestGraphIssuesConvertOnlyNativeBlockingDependencies(t *testing.T) {
	graph, _ := readFixtureGraph(t, 1)
	issues := graph.Issues()
	if len(issues) != 4 {
		t.Fatalf("issues = %d, want 4 (Memories are not Issues)", len(issues))
	}
	byID := map[string]model.Issue{}
	for _, i := range issues {
		byID[i.ID] = i
	}
	e1, ok := byID["sample-e.1"]
	if !ok {
		t.Fatalf("issue ids = %v; the Bead id, not the legacy property id, names an Issue", byID)
	}
	if e1.Status != model.StatusInProgress || e1.IssueType != model.TypeFeature || e1.Priority != 2 || e1.Title != "Dead work" {
		t.Fatalf("converted issue = %+v", e1)
	}
	var parent, blocks bool
	for _, d := range byID["sample-e.1"].Dependencies {
		if d.Type == model.DepParentChild && d.DependsOnID == "sample-e" {
			parent = true
		}
	}
	for _, d := range byID["sample-e.2"].Dependencies {
		if d.Type == model.DepBlocks && d.DependsOnID == "sample-e.1" {
			blocks = true
		}
	}
	if parent || !blocks {
		t.Fatalf("dependencies: parent %v blocks %v", parent, blocks)
	}
	if byID["sample-e"].IssueType != model.TypeEpic {
		t.Fatalf("epic type lost: %+v", byID["sample-e"])
	}
}

func TestGraphRejectsIncompleteTraversal(t *testing.T) {
	bd, _ := fakeGraphBd(t, `case "$1" in
list) echo '{"preview":true,"result":{"hasMore":false,"items":[{"id":"s/beads/a","type":"s/types/preview-issue-v2","properties":{"title":"A"},"owned":[]}]}}' ;;
graph) echo '{"preview":true,"result":{"complete":false,"nodes":[],"links":[]}}' ;;
esac`)
	client, err := newGraphPreviewClient(graphWorkspace(t), bd)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Graph(context.Background(), nil); err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("want incomplete traversal error, got %v", err)
	}
}

func TestGraphTypeLabelsRequireExactInstalledNames(t *testing.T) {
	for _, name := range []string{"not-follows", "custom-cites", "preview-blocks-v2", "supersedes"} {
		if got := edgeKind("https://test/types/" + name); got != name {
			t.Fatalf("interpreted unknown Type %s as %s", name, got)
		}
	}
}

func TestLoadFromSourceReadsAGraphWorkspaceThroughTheGraph(t *testing.T) {
	bd, calls := fakeGraphBd(t, probeHelpAnswer+graphFixtureScript)
	t.Setenv("PATH", filepath.Dir(bd)+string(os.PathListSeparator)+"/usr/bin:/bin")
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	project := graphWorkspace(t)
	beadsDir := filepath.Join(project, ".beads")
	storeDir := filepath.Join(beadsDir, "embeddeddolt", "beads_graph_x")
	if err := os.MkdirAll(storeDir, 0o755); err != nil {
		t.Fatal(err)
	}

	issues, err := LoadFromSource(DataSource{Type: SourceTypeDoltEmbedded, Path: storeDir, Database: "beads_graph_x"})
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 4 {
		t.Fatalf("issues = %d, want the 4 Issues of the graph", len(issues))
	}
	log, _ := os.ReadFile(calls)
	if strings.Contains(string(log), "export") {
		t.Fatalf("a graph workspace refuses bd export, so it must not be called:\n%s", log)
	}
	if strings.Contains(string(log), "graph ") {
		t.Fatalf("loading Issues must not traverse the graph; Memory Links load on demand:\n%s", log)
	}
	var blocked bool
	for _, issue := range issues {
		for _, dep := range issue.Dependencies {
			if issue.ID == "sample-e.2" && dep.DependsOnID == "sample-e.1" && dep.Type == model.DepBlocks {
				blocked = true
			}
		}
	}
	if !blocked {
		t.Fatal("the inventory's owned blocking Link must reach the Issues")
	}
	if !MemoryWorkspaceFor(beadsDir) {
		t.Fatal("MemoryWorkspaceFor = false, want the graph workspace recorded")
	}
	if _, ok := MemoryGraphFor(beadsDir); ok {
		t.Fatal("MemoryGraphFor reports a graph that nobody requested")
	}
}

// A reload re-reads only the Issues. A graph read on request stays until the
// views replace it, so the decision columns do not vanish on every write.
func TestLoadFromSourceKeepsARequestedGraph(t *testing.T) {
	bd, _ := fakeGraphBd(t, probeHelpAnswer+graphFixtureScript)
	t.Setenv("PATH", filepath.Dir(bd)+string(os.PathListSeparator)+"/usr/bin:/bin")
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	project := graphWorkspace(t)
	beadsDir := filepath.Join(project, ".beads")
	storeDir := filepath.Join(beadsDir, "embeddeddolt", "beads_graph_x")
	if err := os.MkdirAll(storeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	StoreMemoryGraph(beadsDir, NewMemoryGraph([]GraphNode{{ID: "kept", Kind: "memory"}}, nil))

	if _, err := LoadFromSource(DataSource{Type: SourceTypeDoltEmbedded, Path: storeDir, Database: "beads_graph_x"}); err != nil {
		t.Fatal(err)
	}
	if g, ok := MemoryGraphFor(beadsDir); !ok || len(g.Nodes) != 1 {
		t.Fatalf("MemoryGraphFor = %v, %v; want the stored graph kept", g.Nodes, ok)
	}
}

func TestStoreMemoryGraphKeepsNothingWithMemoryOff(t *testing.T) {
	beadsDir := t.TempDir()
	t.Setenv(MemoryLeverEnv, "off")
	StoreMemoryGraph(beadsDir, MemoryGraph{})
	if _, ok := MemoryGraphFor(beadsDir); ok {
		t.Fatal("a graph was kept with Memory views turned off")
	}
}

// Progress counts Issue components, the unit of slow work: one traversal each.
func TestGraphCountsEachComponent(t *testing.T) {
	bd, _ := fakeGraphBd(t, graphFixtureScript)
	client, err := newGraphPreviewClient(graphWorkspace(t), bd)
	if err != nil {
		t.Fatal(err)
	}
	client.traversalWorkers = 1
	var steps [][2]int
	graph, err := client.Graph(context.Background(), func(done, total int) {
		steps = append(steps, [2]int{done, total})
	})
	if err != nil {
		t.Fatal(err)
	}
	want := [][2]int{{0, 2}, {1, 2}, {2, 2}}
	if !reflect.DeepEqual(steps, want) {
		t.Fatalf("progress = %v, want %v", steps, want)
	}
	if len(graph.Edges) != 7 {
		t.Fatalf("edges = %d, want the full graph", len(graph.Edges))
	}
}

func TestMemoryGraphForIsEmptyOutsideAGraphWorkspace(t *testing.T) {
	if _, ok := MemoryGraphFor(t.TempDir()); ok {
		t.Fatal("no graph was read for this directory")
	}
}
