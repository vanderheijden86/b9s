package datasource

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vanderheijden86/b9s/pkg/model"
)

// graphFixtureScript answers one inventory and two traversals. The inventory
// owns only the child-of Link, as in the preview, where `bd link` writes
// Links that no Bead owns. The traversal is the only source of those.
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
 {"id":"$S/beads/sample-e.2","type":"$S/types/preview-issue-v2","version":"i2","properties":{"title":"Proposed work","status":"open","priority":2,"issue_type":"task"},"owned":[]},
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
	graph, err := client.Graph(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	calls, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	return graph, string(calls)
}

func TestGraphReadsEveryComponentOnceAndNoBeadRecord(t *testing.T) {
	graph, calls := readFixtureGraph(t, 1)
	if got := strings.Count(calls, "graph "); got != 3 {
		t.Fatalf("traversals = %d, want one per component (3):\n%s", got, calls)
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

func TestGraphKeepsLinksOnlyTheTraversalReturns(t *testing.T) {
	graph, _ := readFixtureGraph(t, 4)
	kinds := map[string]int{}
	for _, e := range graph.Edges {
		kinds[e.Kind]++
	}
	want := map[string]int{EdgeChildOf: 2, EdgeFollows: 2, EdgeCites: 1, EdgeSupersedes: 1, EdgeBlocks: 1}
	for k, n := range want {
		if kinds[k] != n {
			t.Fatalf("edge kinds = %v, want %v", kinds, want)
		}
	}
	if len(graph.Edges) != 7 {
		t.Fatalf("edges = %d, want 7 without duplicates", len(graph.Edges))
	}
}

func TestGraphReadsMemoryStatusFromBodyFrontmatter(t *testing.T) {
	graph, _ := readFixtureGraph(t, 1)
	cases := map[string]struct{ status, number string }{
		"adr-0009": {"superseded", "0009"},
		"adr-0027": {"active", "0027"},
		"adr-0026": {"proposed", "0026"},
		"note":     {"", ""},
	}
	for id, want := range cases {
		n, ok := graph.Node(id)
		if !ok {
			t.Fatalf("missing node %s", id)
		}
		if n.Status != want.status || n.Number != want.number {
			t.Fatalf("%s: status %q number %q, want %q %q", id, n.Status, n.Number, want.status, want.number)
		}
	}
	if got := graph.Label("adr-0009"); got != "ADR 0009" {
		t.Fatalf("label = %q", got)
	}
	if got := graph.Label("note"); got != "note" {
		t.Fatalf("label without frontmatter id = %q", got)
	}
}

func TestGraphTurnsFrontmatterSupersessionIntoTypedEdge(t *testing.T) {
	graph, _ := readFixtureGraph(t, 1)
	var found bool
	for _, e := range graph.Edges {
		if e.Kind == EdgeSupersedes {
			found = true
			if e.Source != "adr-0027" || e.Target != "adr-0009" {
				t.Fatalf("supersedes edge %s -> %s, want adr-0027 -> adr-0009", e.Source, e.Target)
			}
		}
		if e.Kind == EdgeRelated {
			t.Fatalf("the related Link between 0009 and 0027 should read as supersedes: %+v", e)
		}
	}
	if !found {
		t.Fatal("no supersedes edge")
	}
}

func TestGraphHealthFlagsEachIssueOnce(t *testing.T) {
	graph, _ := readFixtureGraph(t, 1)
	want := map[string]Health{
		"sample-e":    HealthEpic,
		"sample-e.1":  HealthDead,
		"sample-e.2":  HealthProposed,
		"sample-lone": HealthNone,
		"adr-0027":    HealthNotIssue,
	}
	for id, h := range want {
		if got := graph.Health(id); got != h {
			t.Fatalf("health(%s) = %v, want %v", id, got, h)
		}
	}
	if f := graph.Followers("adr-0009"); len(f) != 1 || f[0] != "sample-e.1" {
		t.Fatalf("followers = %v", f)
	}
	if c := graph.Citers("adr-0027"); len(c) != 1 {
		t.Fatalf("citers = %v", c)
	}
}

func TestGraphIssuesConvertToModelWithHierarchy(t *testing.T) {
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
	if !parent || !blocks {
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
	if _, err := client.Graph(context.Background()); err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("want incomplete traversal error, got %v", err)
	}
}

func TestParseMemoryFrontmatterIgnoresBodyRules(t *testing.T) {
	fm := parseMemoryFrontmatter("intro\n\n---\nstatus: active\nid: \"0001\"\n---\n\ntext\n\n---\nstatus: superseded\n---\n")
	if fm.Status != "active" || fm.ID != "0001" {
		t.Fatalf("frontmatter = %+v; only the first block counts", fm)
	}
	if fm := parseMemoryFrontmatter("no block\nstatus: active"); fm.Status != "" {
		t.Fatalf("a status line outside a block must not count: %+v", fm)
	}
}

func TestLoadFromSourceReadsAGraphWorkspaceThroughTheGraph(t *testing.T) {
	bd, calls := fakeGraphBd(t, graphFixtureScript)
	t.Setenv("PATH", filepath.Dir(bd)+string(os.PathListSeparator)+"/usr/bin:/bin")
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
	graph, ok := MemoryGraphFor(beadsDir)
	if !ok || graph.Health("sample-e.1") != HealthDead {
		t.Fatalf("the graph read for the Issues must stay available for the decision views (ok=%v)", ok)
	}
}

func TestMemoryGraphForIsEmptyOutsideAGraphWorkspace(t *testing.T) {
	if _, ok := MemoryGraphFor(t.TempDir()); ok {
		t.Fatal("no graph was read for this directory")
	}
}
