package main

import (
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
