package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/vanderheijden86/b9s/internal/datasource"
)

func TestRenderMemoriesShowsTypedEdgesAndBodies(t *testing.T) {
	graph := datasource.GraphPreview{Beads: []datasource.GraphBead{
		{ID: "https://example.test/team/beads/policy", Kind: "memory", Version: "v2", Properties: datasource.GraphProperties{Title: "Policy", Body: "Use integration."}, Owned: []datasource.GraphLink{{Type: "https://example.test/team/types/preview-related-v2", Source: "https://example.test/team/beads/policy", Target: "https://example.test/team/beads/work", Properties: datasource.GraphProperties{Note: "applies to"}}}},
		{ID: "https://example.test/team/beads/work", Kind: "issue", Properties: datasource.GraphProperties{Title: "Adopt policy"}},
	}, Links: []datasource.GraphLink{
		{Type: "types/preview-related-v2", Source: "https://example.test/team/beads/policy", Target: "https://example.test/team/beads/work", Properties: datasource.GraphProperties{Note: "applies to"}},
		{Type: "types/preview-related-v2", Source: "https://example.test/team/beads/work", Target: "https://example.test/team/beads/policy", Properties: datasource.GraphProperties{Note: "cites policy"}},
	}}
	output, err := renderMemories(graph, "policy")
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"Policy", "Use integration.", "v2", "related", "applies to", "cites policy", "Adopt policy"} {
		if !strings.Contains(output, text) {
			t.Errorf("missing %q in %s", text, output)
		}
	}
}

func TestRenderMemoriesRejectsUnknownSelection(t *testing.T) {
	_, err := renderMemories(datasource.GraphPreview{}, "missing")
	if err == nil {
		t.Fatal("expected missing memory error")
	}
}

func TestPrintOneMemoryReadsOnlyItsLinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake bd script needs a POSIX shell")
	}
	project := t.TempDir()
	os.MkdirAll(filepath.Join(project, ".beads"), 0o755)
	os.WriteFile(filepath.Join(project, ".beads", "metadata.json"), []byte(`{"graph_mode":"link","graph_ready":true}`), 0o644)
	bin := t.TempDir()
	log := filepath.Join(bin, "calls.log")
	script := `#!/bin/sh
echo "$*" >> '` + log + `'
case "$1" in
list) echo '{"preview":true,"result":{"items":[{"id":"s/beads/a","type":"s/types/preview-memory-v2","properties":{"title":"Alpha","body":"A body"}},{"id":"s/beads/b","type":"s/types/preview-memory-v2","properties":{"title":"Beta","body":"B body"}}],"hasMore":false}}' ;;
links) echo '{"preview":true,"result":[]}' ;;
esac
`
	os.WriteFile(filepath.Join(bin, "bd"), []byte(script), 0o755)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	var stdout, stderr strings.Builder
	if code := runMemories([]string{"--project", project, "--id", "b"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "B body") || strings.Contains(stdout.String(), "A body") {
		t.Fatalf("output = %s", stdout.String())
	}
	calls, _ := os.ReadFile(log)
	if strings.Count(string(calls), "links ") != 1 || !strings.Contains(string(calls), "links s/beads/b --json") {
		t.Fatalf("calls = %q, want one links read for b", calls)
	}
}
