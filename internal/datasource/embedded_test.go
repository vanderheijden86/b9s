package datasource

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// doltJournalName is the file name Dolt gives the chunk journal.
const doltJournalName = "vvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvv"

const embeddedMetadata = `{"database":"dolt","backend":"dolt","dolt_mode":"embedded","dolt_database":"emb"}`

// newEmbeddedProject lays out a .beads directory as bd 1.3 writes it for an
// embedded project, with a stale issues.jsonl beside the store that must
// never be read in its place.
func newEmbeddedProject(t *testing.T) (projectDir, beadsDir, storeDir string) {
	t.Helper()
	projectDir = t.TempDir()
	beadsDir = filepath.Join(projectDir, ".beads")
	storeDir = filepath.Join(beadsDir, "embeddeddolt", "emb")
	noms := filepath.Join(storeDir, ".dolt", "noms")
	if err := os.MkdirAll(noms, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(beadsDir, "metadata.json"), embeddedMetadata)
	writeFile(t, filepath.Join(noms, "manifest"), "manifest-1")
	writeFile(t, filepath.Join(noms, doltJournalName), "journal")
	writeFile(t, filepath.Join(beadsDir, "issues.jsonl"),
		`{"id":"emb-stale","title":"Stale export","status":"open","priority":2,"issue_type":"task"}`+"\n")
	return projectDir, beadsDir, storeDir
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// installFakeBD puts a bd on PATH that answers `bd export` for the project in
// wantBeadsDir only, and writes a hint on stderr as the real bd does.
func installFakeBD(t *testing.T, wantBeadsDir string) {
	t.Helper()
	bin := t.TempDir()
	script := `#!/bin/sh
if [ "$1" != "export" ]; then echo "unexpected: $*" >&2; exit 2; fi
if [ "$BEADS_DIR" != "` + wantBeadsDir + `" ]; then echo "wrong BEADS_DIR: $BEADS_DIR" >&2; exit 3; fi
echo "Hint: a newer bd is available" >&2
echo '{"_type":"issue","id":"emb-1","title":"Live","status":"open","priority":1,"issue_type":"task","comments":[{"id":"c1","issue_id":"emb-1","author":"a","text":"hi","created_at":"2026-09-27T21:03:03Z"}]}'
echo '{"_type":"issue","id":"emb-2","title":"Done","status":"closed","priority":2,"issue_type":"task","dependencies":[{"issue_id":"emb-2","depends_on_id":"emb-1","type":"blocks"}]}'
`
	writeFile(t, filepath.Join(bin, "bd"), script)
	if err := os.Chmod(filepath.Join(bin, "bd"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+"/usr/bin:/bin")
}

func TestDiscoverSources_EmbeddedDoltOutranksStaleExport(t *testing.T) {
	projectDir, beadsDir, storeDir := newEmbeddedProject(t)

	sources, err := DiscoverSources(loadDiscoveryOptions(beadsDir, projectDir))
	if err != nil {
		t.Fatalf("DiscoverSources: %v", err)
	}
	opts := DefaultSelectionOptions()
	opts.PreferFreshest = false
	opts.AllowUnvalidated = true
	best, err := SelectBestSourceWithOptions(sources, opts)
	if err != nil {
		t.Fatalf("SelectBestSource: %v", err)
	}
	if best.Type != SourceTypeDoltEmbedded {
		t.Fatalf("best source = %s, want %s", best.Type, SourceTypeDoltEmbedded)
	}
	if best.Path != storeDir || best.Database != "emb" {
		t.Errorf("source = %s/%s, want %s/emb", best.Path, best.Database, storeDir)
	}
	if got := EmbeddedBeadsDir(best); got != beadsDir {
		t.Errorf("EmbeddedBeadsDir = %q, want %q", got, beadsDir)
	}
}

func TestDiscoverSources_ServerModeIsNotEmbedded(t *testing.T) {
	beadsDir := filepath.Join(t.TempDir(), ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(beadsDir, "metadata.json"), `{"dolt_mode":"server","dolt_database":"x"}`)
	sources, err := DiscoverSources(DiscoveryOptions{BeadsDir: beadsDir, SkipWorktreeSources: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range sources {
		if s.Type == SourceTypeDoltEmbedded {
			t.Fatalf("server-mode metadata produced an embedded source: %+v", s)
		}
	}
}

func TestLoadFromSource_EmbeddedDoltReadsBDExport(t *testing.T) {
	_, beadsDir, storeDir := newEmbeddedProject(t)
	installFakeBD(t, beadsDir)

	source := DataSource{Type: SourceTypeDoltEmbedded, Path: storeDir, Database: "emb"}
	issues, err := LoadFromSource(source)
	if err != nil {
		t.Fatalf("LoadFromSource: %v", err)
	}
	if len(issues) != 2 {
		t.Fatalf("got %d issues, want 2: %+v", len(issues), issues)
	}
	byID := map[string]int{}
	for i, is := range issues {
		byID[is.ID] = i
	}
	live, ok := byID["emb-1"]
	if !ok || len(issues[live].Comments) != 1 {
		t.Errorf("emb-1 missing or without its comment: %+v", issues)
	}
	done, ok := byID["emb-2"]
	if !ok || string(issues[done].Status) != "closed" || len(issues[done].Dependencies) != 1 {
		t.Errorf("emb-2 missing, not closed or without its dependency: %+v", issues)
	}

	comments, err := LoadAllComments(source)
	if err != nil {
		t.Fatalf("LoadAllComments: %v", err)
	}
	if len(comments["emb-1"]) != 1 {
		t.Errorf("LoadAllComments = %v, want one comment on emb-1", comments)
	}
}

func TestOpenProject_EmbeddedDoltUsesBDNotTheStaleExport(t *testing.T) {
	projectDir, beadsDir, _ := newEmbeddedProject(t)
	installFakeBD(t, beadsDir)

	opened, failure := OpenProject(OpenTarget{Name: "emb", Dir: projectDir})
	if failure != nil {
		t.Fatalf("OpenProject: %v", failure)
	}
	if opened.Source.Type != SourceTypeDoltEmbedded {
		t.Fatalf("opened %s, want %s", opened.Source.Type, SourceTypeDoltEmbedded)
	}
	for _, is := range opened.Issues {
		if is.ID == "emb-stale" {
			t.Fatal("the stale issues.jsonl was read instead of the store")
		}
	}
}

func TestOpenProject_EmbeddedDoltWithoutBDNamesTheMissingCommand(t *testing.T) {
	projectDir, _, _ := newEmbeddedProject(t)
	t.Setenv("PATH", t.TempDir())

	_, failure := OpenProject(OpenTarget{Name: "emb", Dir: projectDir})
	if failure == nil {
		t.Fatal("OpenProject succeeded without bd; it must not fall back to the stale export")
	}
	if failure.Reason != OpenNoBD {
		t.Fatalf("reason = %d (%s), want OpenNoBD", failure.Reason, failure.Message())
	}
	if !strings.Contains(failure.Message(), "bd") {
		t.Errorf("message %q does not name bd", failure.Message())
	}
	if !errors.Is(failure, ErrBDNotFound) {
		t.Errorf("failure does not wrap ErrBDNotFound: %v", failure)
	}
}

func TestEmbeddedFingerprint_ChangesOnWriteNotOnRead(t *testing.T) {
	_, _, storeDir := newEmbeddedProject(t)
	noms := filepath.Join(storeDir, ".dolt", "noms")

	before, err := embeddedFingerprint(storeDir)
	if err != nil {
		t.Fatal(err)
	}

	// A read only touches file times.
	later := time.Now().Add(time.Minute)
	for _, name := range []string{"manifest", doltJournalName} {
		if err := os.Chtimes(filepath.Join(noms, name), later, later); err != nil {
			t.Fatal(err)
		}
	}
	afterRead, err := embeddedFingerprint(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	if afterRead != before {
		t.Error("fingerprint changed on a time-only change")
	}

	// While bd reads, it rebuilds journal.idx, takes LOCK and writes
	// temporary manifests.
	writeFile(t, filepath.Join(noms, "journal.idx"), "partial index")
	writeFile(t, filepath.Join(noms, "LOCK"), "")
	writeFile(t, filepath.Join(noms, "nbs_manifest_553622487"), "temp")
	duringRead, err := embeddedFingerprint(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	if duringRead != before {
		t.Error("fingerprint changed on the files bd touches while it reads")
	}

	// A write appends to the journal.
	f, err := os.OpenFile(filepath.Join(noms, doltJournalName), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("more")
	_ = f.Close()
	afterJournal, err := embeddedFingerprint(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	if afterJournal == afterRead {
		t.Error("fingerprint unchanged after the journal grew")
	}

	// A write rewrites the manifest, sometimes at the same size.
	writeFile(t, filepath.Join(noms, "manifest"), "manifest-2")
	afterManifest, err := embeddedFingerprint(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	if afterManifest == afterJournal {
		t.Error("fingerprint unchanged after the manifest was rewritten")
	}
}

func TestDoltWatcher_EmbeddedNotifiesOnStoreWrite(t *testing.T) {
	_, _, storeDir := newEmbeddedProject(t)
	source := DataSource{Type: SourceTypeDoltEmbedded, Path: storeDir, Database: "emb"}

	w, err := NewDoltWatcher(source, 20*time.Millisecond)
	if err != nil {
		t.Fatalf("NewDoltWatcher: %v", err)
	}
	if err := w.Start(); err != nil {
		t.Fatal(err)
	}
	defer w.Stop()

	writeFile(t, filepath.Join(storeDir, ".dolt", "noms", "manifest"), "manifest-2")
	select {
	case <-w.Changed():
	case <-time.After(2 * time.Second):
		t.Fatal("no change notification within 2s of a store write")
	}
}

// A graph workspace read by a bd without Memory support loads Issues through
// the ordinary export and records no Memory graph, so no Memory column shows.
// A graph kept from an earlier load under a Memory-capable bd is dropped too.
func TestLoadFromSource_GraphWorkspaceWithStockBdSkipsTheMemoryGraph(t *testing.T) {
	_, beadsDir, storeDir := newEmbeddedProject(t)
	memoryGraphs.Lock()
	memoryGraphs.byDir[beadsDir] = MemoryGraph{}
	memoryGraphs.Unlock()
	writeFile(t, filepath.Join(beadsDir, "metadata.json"),
		`{"backend":"dolt","dolt_mode":"embedded","dolt_database":"emb","graph_mode":"link","graph_ready":true}`)
	log := onPath(t, stockHelpAnswer+`[ "$1" = export ] || exit 2
echo '{"_type":"issue","id":"emb-1","title":"Live","status":"open","priority":1,"issue_type":"task"}'
`)

	issues, err := LoadFromSource(DataSource{Type: SourceTypeDoltEmbedded, Path: storeDir, Database: "emb"})

	if err != nil || len(issues) != 1 {
		t.Fatalf("LoadFromSource = %d issues, err %v; want 1 issue from export", len(issues), err)
	}
	if calls, _ := os.ReadFile(log); strings.Contains(string(calls), "graph") || strings.Contains(string(calls), "records-json") {
		t.Errorf("bd calls = %q, want no graph read", calls)
	}
	if _, ok := MemoryGraphFor(beadsDir); ok {
		t.Error("MemoryGraphFor reports a graph, want none without Memory support")
	}
	if MemoryWorkspaceFor(beadsDir) {
		t.Error("MemoryWorkspaceFor = true, want false without Memory support")
	}
}

// A graph workspace refuses bd export, so its Issues come only from the graph
// read. Turning Memory views off must hide the graph, not the Issues.
func TestLoadFromSource_MemoryLeverKeepsGraphWorkspaceIssues(t *testing.T) {
	_, beadsDir, storeDir := newEmbeddedProject(t)
	writeFile(t, filepath.Join(beadsDir, "metadata.json"),
		`{"backend":"dolt","dolt_mode":"embedded","dolt_database":"emb","graph_mode":"link","graph_ready":true}`)
	onPath(t, probeHelpAnswer+graphFixtureScript)
	t.Setenv(MemoryLeverEnv, "off")

	issues, err := LoadFromSource(DataSource{Type: SourceTypeDoltEmbedded, Path: storeDir, Database: "emb"})

	if err != nil || len(issues) == 0 {
		t.Fatalf("LoadFromSource = %d issues, err %v; want the graph workspace's Issues", len(issues), err)
	}
	if _, ok := MemoryGraphFor(beadsDir); ok {
		t.Error("MemoryGraphFor reports a graph with Memory turned off, want none")
	}
	if MemoryWorkspaceFor(beadsDir) {
		t.Error("MemoryWorkspaceFor = true with Memory turned off, want false")
	}
}
