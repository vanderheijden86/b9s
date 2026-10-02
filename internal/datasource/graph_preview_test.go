package datasource

import (
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
