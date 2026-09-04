package datasource

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDiscoveryOptionsSkipPreflightValidation(t *testing.T) {
	opts := loadDiscoveryOptions("/repo/.beads", "/repo")

	if opts.ValidateAfterDiscovery {
		t.Fatal("load discovery must not validate a source before LoadFromSource opens and queries it")
	}
	if opts.IncludeInvalid {
		t.Fatal("load discovery should not request invalid-source metadata")
	}
}

func TestLoadSmartLoadsAnUnvalidatedCandidate(t *testing.T) {
	beadsDir := filepath.Join(t.TempDir(), ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	issueData := []byte("{\"id\":\"test-1\",\"title\":\"Fast path\",\"status\":\"open\",\"priority\":2,\"issue_type\":\"task\"}\n")
	if err := os.WriteFile(filepath.Join(beadsDir, "issues.jsonl"), issueData, 0o644); err != nil {
		t.Fatal(err)
	}

	issues, err := loadSmart(beadsDir, "")
	if err != nil {
		t.Fatalf("loadSmart failed: %v", err)
	}
	if len(issues) != 1 || issues[0].ID != "test-1" {
		t.Fatalf("unexpected issues: %#v", issues)
	}
}
