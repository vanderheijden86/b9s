package web

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vanderheijden86/b9s/internal/datasource"
	"github.com/vanderheijden86/b9s/pkg/model"
)

// newMemoryWebProject opens a Memory preview workspace whose bd logs every
// call to calls.
func newMemoryWebProject(t *testing.T) (store *Store, server *Server, calls string) {
	t.Helper()
	dir, _ := newEmbeddedWebProject(t)
	beads := filepath.Join(dir, ".beads")
	meta := `{"backend":"dolt","dolt_mode":"embedded","dolt_database":"emb","graph_mode":"link","graph_ready":true}`
	if err := os.WriteFile(filepath.Join(beads, "metadata.json"), []byte(meta), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := strings.Split(os.Getenv("PATH"), string(os.PathListSeparator))[0]
	// The help branch answers b9s's Memory capability probe as a preview bd does.
	script := `#!/bin/sh
case "$*" in *--help) echo "records-json"; exit 0 ;; esac
echo "$*" >> "$BEADS_DIR/calls"
case "$1" in
list) printf '%s\n' '{"preview":true,"result":{"hasMore":false,"items":[{"id":"s/beads/i","type":"s/types/preview-issue-v2","properties":{"title":"Work","status":"open","issue_type":"task","description":"Issue body"},"owned":[]},{"id":"s/beads/m","type":"s/types/preview-memory-v2","properties":{"title":"Decision","body":"---\nid: \"0042\"\nstatus: proposed\n---\nDecision body"},"owned":[]}]}}' ;;
graph) echo '{"preview":true,"result":{"complete":true,"nodes":[{"id":"s/beads/i"},{"id":"s/beads/m"}],"links":[{"id":"s/links/i-follows-m","source":"s/beads/i","target":"s/beads/m","type":"s/types/example-follows","properties":{"note":"Our contract"}}]}}' ;;
*) exit 3 ;;
esac
`
	if err := os.WriteFile(filepath.Join(bin, "bd"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	store = NewStore(time.Hour)
	t.Cleanup(store.Close)
	if failure := store.Open(datasource.OpenTarget{Name: "graph", Dir: dir}, dir); failure != nil {
		t.Fatal(failure)
	}
	server, err := NewServer(Options{Store: store, Assets: testAssets})
	if err != nil {
		t.Fatal(err)
	}
	return store, server, filepath.Join(beads, "calls")
}

func getMemoryGraph(t *testing.T, server *Server) MemoryGraphResponse {
	t.Helper()
	w := httptest.NewRecorder()
	server.ServeHTTP(w, httptest.NewRequest("GET", "/api/memory-graph", nil))
	var graph MemoryGraphResponse
	if err := json.Unmarshal(w.Body.Bytes(), &graph); err != nil {
		t.Fatal(err)
	}
	return graph
}

// waitForMemoryGraph polls the endpoint as the Memory view does until the
// graph is read.
func waitForMemoryGraph(t *testing.T, server *Server) MemoryGraphResponse {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		graph := getMemoryGraph(t, server)
		if !graph.Loading {
			return graph
		}
		if time.Now().After(deadline) {
			t.Fatalf("graph still loading after 5s: %+v", graph)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func graphCalls(t *testing.T, calls string) int {
	t.Helper()
	data, _ := os.ReadFile(calls)
	return strings.Count(string(data), "graph ")
}

// Opening a project must not wait on the Memory graph (ADR 0051): the first
// request starts one read in the background and reports its progress.
func TestMemoryGraphEndpointReadsTheGraphOnRequest(t *testing.T) {
	store, server, calls := newMemoryWebProject(t)
	if n := graphCalls(t, calls); n != 0 {
		t.Fatalf("opening the project ran %d graph traversals", n)
	}
	first := getMemoryGraph(t, server)
	if !first.Loading || first.Available {
		t.Fatalf("first request = %+v, want a read in progress", first)
	}
	graph := waitForMemoryGraph(t, server)
	if !graph.Available || graph.Version != store.Version() || len(graph.Nodes) != 2 || len(graph.Edges) != 1 {
		t.Fatalf("graph = %+v", graph)
	}
	if graph.Total != 1 || graph.Done != 1 {
		t.Fatalf("progress = %d/%d, want 1/1", graph.Done, graph.Total)
	}
	for _, node := range graph.Nodes {
		if node.Kind == "memory" && (node.Status != "" || !strings.Contains(node.Body, "Decision body")) {
			t.Fatalf("memory = %+v", node)
		}
		if node.Kind == "issue" && (node.Status != "open" || node.Body != "Issue body") {
			t.Fatalf("issue = %+v", node)
		}
	}
	if graph.Edges[0].Note != "Our contract" || graph.Edges[0].Kind != "follows" {
		t.Fatalf("edge = %+v", graph.Edges[0])
	}
	read := graphCalls(t, calls)
	for range 3 {
		if again := getMemoryGraph(t, server); !again.Available {
			t.Fatalf("graph = %+v", again)
		}
	}
	if n := graphCalls(t, calls); n != read {
		t.Fatalf("a read graph was read again: %d traversals, want %d", n, read)
	}
	if failure := store.Open(datasource.OpenTarget{Name: "plain", Dir: newFixtureProject(t)}, "plain"); failure != nil {
		t.Fatal(failure)
	}
	if store.MemoryGraph().Available {
		t.Fatal("project switch retained previous graph")
	}
}

// The Issue detail lists Memory Links, so opening one asks for the graph and
// says it is on its way.
func TestIssueDetailStartsTheMemoryGraphRead(t *testing.T) {
	store, server, calls := newMemoryWebProject(t)
	issues, _ := store.Issues()
	if len(issues) == 0 {
		t.Fatal("no issues loaded")
	}
	issue, ok := store.Issue(issues[0].ID)
	if !ok || !issue.MemoryLoading || issue.MemoryAvailable {
		t.Fatalf("issue = loading %v available %v, want loading", issue.MemoryLoading, issue.MemoryAvailable)
	}
	waitForMemoryGraph(t, server)
	if n := graphCalls(t, calls); n != 1 {
		t.Fatalf("%d graph traversals, want the one the detail started", n)
	}
	issue, _ = store.Issue(issues[0].ID)
	if issue.MemoryLoading || !issue.MemoryAvailable {
		t.Fatalf("issue = loading %v available %v, want the read graph", issue.MemoryLoading, issue.MemoryAvailable)
	}
}

// A reload refreshes a graph that was read, and keeps it on screen meanwhile.
func TestReloadRefreshesAReadMemoryGraph(t *testing.T) {
	store, server, calls := newMemoryWebProject(t)
	waitForMemoryGraph(t, server)
	before := store.Version()
	store.Reload()
	if graph := getMemoryGraph(t, server); !graph.Available {
		t.Fatalf("graph during the refresh = %+v", graph)
	}
	deadline := time.Now().Add(5 * time.Second)
	for graphCalls(t, calls) < 2 || store.Version() < before+2 {
		if time.Now().After(deadline) {
			t.Fatalf("no refresh: %d traversals, version %d after %d", graphCalls(t, calls), store.Version(), before)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// newEmbeddedWebProject lays out an embedded Dolt project and puts a bd on
// PATH that answers `bd export` for it.
func newEmbeddedWebProject(t *testing.T) (projectDir, noms string) {
	t.Helper()
	projectDir = t.TempDir()
	beadsDir := filepath.Join(projectDir, ".beads")
	noms = filepath.Join(beadsDir, "embeddeddolt", "emb", ".dolt", "noms")
	if err := os.MkdirAll(noms, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(path, content string, mode os.FileMode) {
		t.Helper()
		if err := os.WriteFile(path, []byte(content), mode); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(beadsDir, "metadata.json"), `{"backend":"dolt","dolt_mode":"embedded","dolt_database":"emb"}`, 0o644)
	write(filepath.Join(noms, "manifest"), "manifest-1", 0o644)
	write(filepath.Join(noms, "vvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvv"), "journal", 0o644)

	bin := t.TempDir()
	write(filepath.Join(bin, "bd"), `#!/bin/sh
[ "$1" = "export" ] || exit 2
echo '{"id":"emb-1","title":"Live","status":"open","priority":1,"issue_type":"task"}'
`, 0o755)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+"/usr/bin:/bin")
	return projectDir, noms
}

// TestStore_EmbeddedDoltPollsTheStoreFingerprint pins ADR 0025 for b9s web:
// the files bd rewrites while it only reads must not reload the project, and
// a write must.
func TestStore_EmbeddedDoltPollsTheStoreFingerprint(t *testing.T) {
	projectDir, noms := newEmbeddedWebProject(t)
	store := NewStore(20 * time.Millisecond)
	defer store.Close()

	if failure := store.Open(datasource.OpenTarget{Name: "emb", Dir: projectDir}, "emb"); failure != nil {
		t.Fatalf("Open: %v", failure)
	}
	h := store.Health(true)
	if h.Kind != string(datasource.SourceTypeDoltEmbedded) {
		t.Fatalf("kind = %q, want %q", h.Kind, datasource.SourceTypeDoltEmbedded)
	}
	if !strings.Contains(h.Watching, "polls") {
		t.Errorf("watching = %q, want the poll that ADR 0025 decides", h.Watching)
	}
	if h.Source != "dolt embedded/emb" {
		t.Errorf("source = %q, want dolt embedded/emb", h.Source)
	}

	opened := store.Version()
	if err := os.WriteFile(filepath.Join(noms, "journal.idx"), []byte("rebuilt by a read"), 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	if v := store.Version(); v != opened {
		t.Fatalf("a file bd touches while reading reloaded the project (version %d -> %d)", opened, v)
	}

	if err := os.WriteFile(filepath.Join(noms, "manifest"), []byte("manifest-2"), 0o644); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for store.Version() == opened {
		if time.Now().After(deadline) {
			t.Fatal("no reload within 3s of a store write")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestIssueDetailIncludesNativeMemoryLinksInBothDirections(t *testing.T) {
	store := &Store{issues: []model.Issue{{ID: "epic", IssueType: "epic"}}, memoryGraph: MemoryGraphResponse{
		Available: true,
		Nodes:     []MemoryGraphNode{{ID: "epic", Kind: "issue"}, {ID: "work", Kind: "issue"}, {ID: "m", Kind: "memory", Title: "Literal Memory", Type: "s/types/preview-memory-v2", Body: "Body"}},
		Edges: []MemoryGraphEdge{
			{ID: "out", Source: "epic", Target: "m", Kind: "follows", Type: "s/types/example-follows", Note: "Policy"},
			{ID: "in", Source: "m", Target: "epic", Kind: "related", Type: "s/types/preview-related-v2"},
			{ID: "work-link", Source: "epic", Target: "work", Kind: "blocks"},
		},
	}}
	issue, ok := store.Issue("epic")
	encoded, err := json.Marshal(issue)
	if err != nil {
		t.Fatal(err)
	}
	var detail struct {
		Available bool `json:"memory_available"`
		Links     []struct {
			Link     MemoryGraphEdge `json:"link"`
			Memory   MemoryGraphNode `json:"memory"`
			Outgoing bool            `json:"outgoing"`
		} `json:"memory_links"`
	}
	if err := json.Unmarshal(encoded, &detail); err != nil {
		t.Fatal(err)
	}
	if !ok || !detail.Available || len(detail.Links) != 2 {
		t.Fatalf("detail: %s", encoded)
	}
	if !detail.Links[0].Outgoing || detail.Links[1].Outgoing || detail.Links[0].Link.Note != "Policy" || detail.Links[1].Memory.Title != "Literal Memory" {
		t.Fatalf("links: %+v", detail.Links)
	}
}

// The Memory tab stays visible for every project, so the endpoint must say
// why there is no graph rather than only that there is none.
func TestMemoryGraphEndpointExplainsWhyThereIsNoGraph(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	cases := []struct {
		name  string
		graph bool
		want  string
	}{
		{"ordinary project", false, "not a Memory graph workspace"},
		{"graph workspace with stock bd", true, "has no Memory Beads support"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir, _ := newEmbeddedWebProject(t)
			if c.graph {
				meta := `{"backend":"dolt","dolt_mode":"embedded","dolt_database":"emb","graph_mode":"link","graph_ready":true}`
				if err := os.WriteFile(filepath.Join(dir, ".beads", "metadata.json"), []byte(meta), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			store := NewStore(time.Hour)
			defer store.Close()
			if failure := store.Open(datasource.OpenTarget{Name: "p", Dir: dir}, dir); failure != nil {
				t.Fatal(failure)
			}
			server, err := NewServer(Options{Store: store, Assets: testAssets})
			if err != nil {
				t.Fatal(err)
			}
			w := httptest.NewRecorder()
			server.ServeHTTP(w, httptest.NewRequest("GET", "/api/memory-graph", nil))
			var graph MemoryGraphResponse
			if err := json.Unmarshal(w.Body.Bytes(), &graph); err != nil {
				t.Fatal(err)
			}
			if graph.Available || !strings.Contains(graph.Reason, c.want) {
				t.Fatalf("graph = %+v, want unavailable with a reason containing %q", graph, c.want)
			}
		})
	}
}
