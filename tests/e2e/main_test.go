package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestEndToEndBuildAndRun(t *testing.T) {
	binPath := buildBvBinary(t)
	tempDir := t.TempDir()

	// Prepare a fake environment with .beads/beads.jsonl (canonical filename)
	envDir := filepath.Join(tempDir, "env")
	if err := os.MkdirAll(filepath.Join(envDir, ".beads"), 0755); err != nil {
		t.Fatal(err)
	}

	jsonlContent := `{"id": "bd-1", "title": "E2E Test Issue", "status": "open", "priority": 0, "issue_type": "bug"}`
	if err := os.WriteFile(filepath.Join(envDir, ".beads", "beads.jsonl"), []byte(jsonlContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Run bw --version to verify it runs
	runCmd := exec.Command(binPath, "--version")
	runCmd.Dir = envDir
	if out, err := runCmd.CombinedOutput(); err != nil {
		t.Fatalf("Execution failed: %v\n%s", err, out)
	}
}
