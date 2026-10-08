package datasource

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestParseGraphPreviewKeepsMemoriesAndOwnedLinks(t *testing.T) {
	input := `{"preview":true,"schemaVersion":1,"result":{"items":[{"id":"https://example.test/team/beads/policy","type":"https://example.test/team/types/preview-memory-v2","version":"v2","properties":{"title":"Code flow policy","body":"Use integration."},"owned":[{"id":"https://example.test/team/links/policy-work","type":"https://example.test/team/types/preview-related-v2","source":"https://example.test/team/beads/policy","target":"https://example.test/team/beads/work","properties":{"note":"applies to"}}]},{"id":"https://example.test/team/beads/work","type":"https://example.test/team/types/preview-issue-v2","properties":{"title":"Adopt policy","status":"open"},"owned":[]}],"hasMore":false}}`
	graph, err := ParseGraphPreview(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Beads) != 2 || graph.Beads[0].Kind != "memory" || graph.Beads[0].Properties.Body != "Use integration." {
		t.Fatalf("unexpected beads: %+v", graph.Beads)
	}
	if got := graph.Beads[0].Owned[0].Target; got != "https://example.test/team/beads/work" {
		t.Fatalf("link target = %q", got)
	}
}

func TestParseGraphPreviewRejectsTruncatedInventory(t *testing.T) {
	_, err := ParseGraphPreview(strings.NewReader(`{"preview":true,"result":{"items":[],"hasMore":true}}`))
	if err == nil || !strings.Contains(err.Error(), "limit") {
		t.Fatalf("expected limit error, got %v", err)
	}
}

func TestParseGraphPreviewRejectsOrdinaryBeadsOutput(t *testing.T) {
	_, err := ParseGraphPreview(strings.NewReader(`{"result":{"items":[]}}`))
	if err == nil {
		t.Fatal("expected preview format error")
	}
}

func TestParseGraphPreviewLinksReadsIncomingIssueLink(t *testing.T) {
	input := `{"preview":true,"result":[{"id":"links/work-policy","type":"types/preview-related-v2","source":"beads/work","target":"beads/policy","properties":{"note":"cites policy"}}]}`
	links, err := ParseGraphPreviewLinks(strings.NewReader(input))
	if err != nil || len(links) != 1 || links[0].Source != "beads/work" {
		t.Fatalf("links = %+v, err = %v", links, err)
	}
}

// fakeGraphBd writes a bd script that answers each preview read from a fixture
// and records every argument list in calls.log.
func fakeGraphBd(t *testing.T, script string) (bdPath, logPath string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake bd script needs a POSIX shell")
	}
	dir := t.TempDir()
	logPath = filepath.Join(dir, "calls.log")
	body := "#!/bin/sh\necho \"$*\" >> '" + logPath + "'\n" + script
	bdPath = filepath.Join(dir, "bd")
	if err := os.WriteFile(bdPath, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return bdPath, logPath
}

func graphWorkspace(t *testing.T) string {
	t.Helper()
	project := t.TempDir()
	if err := os.MkdirAll(filepath.Join(project, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	meta := `{"graph_mode":"link","graph_ready":true}`
	if err := os.WriteFile(filepath.Join(project, ".beads", "metadata.json"), []byte(meta), 0o644); err != nil {
		t.Fatal(err)
	}
	return project
}

func TestGraphPreviewClientSearchesMemorySummaries(t *testing.T) {
	bd, log := fakeGraphBd(t, `echo '{"preview":true,"result":{"items":[{"id":"s/beads/adr-0025","title":"ADR 0025","version":"v1","matchedFields":["title","body"],"excerpt":{"field":"body","text":"embedded Dolt","truncated":true},"details":{"ownedLinkCount":1}}]}}'`)
	client, err := newGraphPreviewClient(graphWorkspace(t), bd)
	if err != nil {
		t.Fatal(err)
	}
	got, err := client.SearchMemories(context.Background(), "embedded Dolt")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Title != "ADR 0025" || got[0].MatchedFields[1] != "body" || got[0].Excerpt.Text != "embedded Dolt" || got[0].Details.OwnedLinkCount != 1 {
		t.Fatalf("summaries = %+v", got)
	}
	calls, _ := os.ReadFile(log)
	if want := "memories embedded Dolt --all --details --format records-json\n"; string(calls) != want {
		t.Fatalf("calls = %q, want %q", calls, want)
	}
}

func TestGraphPreviewClientListsAllMemoriesWithoutQuery(t *testing.T) {
	bd, log := fakeGraphBd(t, `echo '{"preview":true,"result":{"items":[]}}'`)
	client, _ := newGraphPreviewClient(graphWorkspace(t), bd)
	if _, err := client.SearchMemories(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	calls, _ := os.ReadFile(log)
	if want := "memories --all --details --format records-json\n"; string(calls) != want {
		t.Fatalf("calls = %q, want %q", calls, want)
	}
}

func TestGraphPreviewClientRejectsTruncatedSearch(t *testing.T) {
	bd, _ := fakeGraphBd(t, `echo '{"preview":true,"result":{"items":[],"hasMore":true}}'`)
	client, _ := newGraphPreviewClient(graphWorkspace(t), bd)
	if _, err := client.SearchMemories(context.Background(), "x"); err == nil || !strings.Contains(err.Error(), "limit") {
		t.Fatalf("expected limit error, got %v", err)
	}
}

func TestGraphPreviewClientReadsRetainedMemoryVersion(t *testing.T) {
	bd, log := fakeGraphBd(t, `echo '{"preview":true,"result":{"id":"s/beads/adr-0030","version":"old","properties":{"title":"ADR 0030","body":"first body"},"owned":[{"id":"s/links/seed-1","type":"s/types/example-cites","source":"s/beads/adr-0030","target":"s/beads/adr-0025"}]}}'`)
	client, _ := newGraphPreviewClient(graphWorkspace(t), bd)
	bead, err := client.Memory(context.Background(), "s/beads/adr-0030", "old")
	if err != nil {
		t.Fatal(err)
	}
	if bead.Version != "old" || bead.Properties.Body != "first body" || len(bead.Owned) != 1 || bead.Kind != "" {
		t.Fatalf("bead = %+v", bead)
	}
	calls, _ := os.ReadFile(log)
	if want := "show s/beads/adr-0030 --json --version old\n"; string(calls) != want {
		t.Fatalf("calls = %q, want %q", calls, want)
	}
}

func TestGraphPreviewClientListsVersionsNewestFirst(t *testing.T) {
	bd, _ := fakeGraphBd(t, `echo '{"preview":true,"result":{"kind":"memory","versions":[{"actor":"a","change_at":"2026-10-02T13:05:22Z","ordinal":2,"version":"new"},{"actor":"a","change_at":"2026-10-02T13:04:05Z","ordinal":1,"version":"old"}]}}'`)
	client, _ := newGraphPreviewClient(graphWorkspace(t), bd)
	versions, err := client.Versions(context.Background(), "s/beads/adr-0030")
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 2 || versions[0].Version != "new" || versions[1].Ordinal != 1 || versions[0].ChangeAt.IsZero() {
		t.Fatalf("versions = %+v", versions)
	}
}

func TestGraphPreviewClientReportsPreviewErrorCode(t *testing.T) {
	bd, _ := fakeGraphBd(t, `echo '{"code":"revision_unknown","message":"graph preview retained version is unknown","retryable":false}'; exit 3`)
	client, _ := newGraphPreviewClient(graphWorkspace(t), bd)
	_, err := client.Memory(context.Background(), "s/beads/adr-0030", "nope")
	if err == nil || !strings.Contains(err.Error(), "revision_unknown") || !strings.Contains(err.Error(), "retained version is unknown") {
		t.Fatalf("expected preview error code, got %v", err)
	}
}

func TestGraphPreviewClientRefusesOrdinaryWorkspace(t *testing.T) {
	project := t.TempDir()
	os.MkdirAll(filepath.Join(project, ".beads"), 0o755)
	os.WriteFile(filepath.Join(project, ".beads", "metadata.json"), []byte(`{"backend":"dolt"}`), 0o644)
	if _, err := newGraphPreviewClient(project, "/bin/false"); !errors.Is(err, ErrMemoryNotEnabled) {
		t.Fatalf("expected workspace error, got %v", err)
	}
}

func TestGraphPreviewClientNamesAStableBdThatLacksGraphCommands(t *testing.T) {
	bd, _ := fakeGraphBd(t, `echo 'Error: unknown flag: --all' >&2; exit 1`)
	client, _ := newGraphPreviewClient(graphWorkspace(t), bd)
	_, err := client.SearchMemories(context.Background(), "")
	if !errors.Is(err, ErrBDNotGraphPreview) || !strings.Contains(err.Error(), bd) {
		t.Fatalf("expected ErrBDNotGraphPreview naming %s, got %v", bd, err)
	}
}

func TestMemoryReadFailureCategories(t *testing.T) {
	for _, tt := range []struct {
		err            error
		category, hint string
	}{
		{ErrBDNotFound, "bd missing", "PATH"},
		{ErrMemoryNotEnabled, "Memories not enabled", "workspace"},
		{ErrBDNotGraphPreview, "Memory commands unavailable", "bd"},
		{errors.New("Access denied for user bd_test"), "Access denied", "credentials"},
		{errors.New("dial tcp: connection refused"), "Database unreachable", "connection"},
		{context.DeadlineExceeded, "Read timed out", "retry"},
		{errors.New("schema version mismatch"), "Beads schema mismatch", "compatible"},
		{errors.New("unexpected failure"), "Memory read failed", "details"},
	} {
		t.Run(tt.category, func(t *testing.T) {
			got := ExplainMemoryFailure(tt.err)
			if !strings.Contains(got, tt.category) || !strings.Contains(got, tt.hint) || !strings.Contains(got, tt.err.Error()) {
				t.Fatalf("explanation = %s", got)
			}
		})
	}
}

// probeHelpAnswer makes a fake bd answer the Memory capability probe the way
// the preview build does. Prepend it to a fake's script.
const probeHelpAnswer = `case "$*" in *--help) echo "--format string   Graph Memory summaries: table or records-json"; exit 0 ;; esac
`

// stockHelpAnswer answers the probe the way a released bd does.
const stockHelpAnswer = `case "$*" in versions*|links*) echo "Error: unknown command \"$1\" for \"bd\"" >&2; exit 1 ;; *--help) exit 0 ;; esac
`

// onPath puts a bd script on PATH, with an isolated probe cache, and returns
// the file that logs every call that is not a capability probe.
func onPath(t *testing.T, script string) (logPath string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake bd script needs a POSIX shell")
	}
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	bin := t.TempDir()
	logPath = filepath.Join(bin, "calls.log")
	head := "#!/bin/sh\ncase \"$*\" in *--help) ;; *) echo \"$*\" >> '" + logPath + "' ;; esac\n"
	if err := os.WriteFile(filepath.Join(bin, "bd"), []byte(head+script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+"/usr/bin:/bin")
	return logPath
}

func TestDetectMemory_ReadyWithPreviewBdAndGraphWorkspace(t *testing.T) {
	onPath(t, probeHelpAnswer)

	got := DetectMemory(context.Background(), graphWorkspace(t))

	if got.State != MemoryReady || got.Err != nil {
		t.Fatalf("DetectMemory = %+v, want ready", got)
	}
}

func TestDetectMemory_StockBdIsUnsupported(t *testing.T) {
	onPath(t, stockHelpAnswer)

	got := DetectMemory(context.Background(), graphWorkspace(t))

	if got.State != MemoryUnsupported || !errors.Is(got.Err, ErrBDNotGraphPreview) {
		t.Fatalf("DetectMemory = %+v, want unsupported with ErrBDNotGraphPreview", got)
	}
}

func TestDetectMemory_MissingBdIsUnsupported(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	got := DetectMemory(context.Background(), graphWorkspace(t))

	if got.State != MemoryUnsupported || !errors.Is(got.Err, ErrBDNotFound) {
		t.Fatalf("DetectMemory = %+v, want unsupported with ErrBDNotFound", got)
	}
}

// An ordinary project is the common case: it must cost no bd call at all.
func TestDetectMemory_OrdinaryWorkspaceIsNotEnabledWithoutRunningBd(t *testing.T) {
	ran := filepath.Join(t.TempDir(), "ran")
	onPath(t, "touch '"+ran+"'\n"+probeHelpAnswer)
	project := t.TempDir()
	os.MkdirAll(filepath.Join(project, ".beads"), 0o755)
	writeFile(t, filepath.Join(project, ".beads", "metadata.json"), `{"backend":"dolt"}`)

	got := DetectMemory(context.Background(), project)

	if got.State != MemoryNotEnabled || !errors.Is(got.Err, ErrMemoryNotEnabled) {
		t.Fatalf("DetectMemory = %+v, want not enabled", got)
	}
	if _, err := os.Stat(ran); err == nil {
		t.Error("bd ran for an ordinary workspace, want no bd call")
	}
}

func TestDetectMemory_NewerGraphSchemaIsNotEnabled(t *testing.T) {
	onPath(t, probeHelpAnswer)
	project := graphWorkspace(t)
	writeFile(t, filepath.Join(project, ".beads", "metadata.json"),
		`{"graph_mode":"link","graph_ready":true,"graph_schema_version":99}`)

	got := DetectMemory(context.Background(), project)

	if got.State != MemoryNotEnabled || !strings.Contains(got.Err.Error(), "graph schema 99") {
		t.Fatalf("DetectMemory = %+v, want not enabled naming graph schema 99", got)
	}
}

func TestOpenGraphPreview_RefusesStockBd(t *testing.T) {
	onPath(t, stockHelpAnswer)

	_, err := OpenGraphPreview(graphWorkspace(t))

	if !errors.Is(err, ErrBDNotGraphPreview) {
		t.Fatalf("OpenGraphPreview err = %v, want ErrBDNotGraphPreview", err)
	}
}

// B9S_MEMORY=off can only take Memory away, and it must do so before bd runs.
func TestDetectMemory_LeverTurnsMemoryOffWithoutRunningBd(t *testing.T) {
	ran := filepath.Join(t.TempDir(), "ran")
	onPath(t, "touch '"+ran+"'\n"+probeHelpAnswer)
	t.Setenv("B9S_MEMORY", "off")

	got := DetectMemory(context.Background(), graphWorkspace(t))

	if got.State != MemoryOff {
		t.Errorf("State = %q, want %q", got.State, MemoryOff)
	}
	if reason, _ := MemoryUnavailableReason(got.Err); !strings.Contains(reason, "B9S_MEMORY=off") {
		t.Errorf("reason = %q, want it to name the lever", reason)
	}
	if _, err := os.Stat(ran); !os.IsNotExist(err) {
		t.Error("bd ran although Memory was turned off")
	}
}

func TestDetectMemory_LeverCannotForceMemoryOn(t *testing.T) {
	onPath(t, stockHelpAnswer)
	t.Setenv("B9S_MEMORY", "on")

	got := DetectMemory(context.Background(), graphWorkspace(t))

	if got.State != MemoryUnsupported {
		t.Errorf("State = %q, want %q: the lever may only subtract", got.State, MemoryUnsupported)
	}
}
