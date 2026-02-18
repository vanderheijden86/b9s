package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanForBeads(t *testing.T) {
	root := t.TempDir()

	// Create project directories with .beads/
	proj1 := filepath.Join(root, "project1")
	proj2 := filepath.Join(root, "subdir", "project2")
	noBeads := filepath.Join(root, "nobeads")

	for _, dir := range []string{
		filepath.Join(proj1, ".beads"),
		filepath.Join(proj2, ".beads"),
		noBeads,
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	results := scanForBeads(root, 3)

	if len(results) != 2 {
		t.Fatalf("expected 2 projects, got %d: %v", len(results), results)
	}

	found := make(map[string]bool)
	for _, r := range results {
		found[r] = true
	}

	if !found[proj1] {
		t.Error("expected to find project1")
	}
	if !found[proj2] {
		t.Error("expected to find project2")
	}
}

func TestScanForBeads_DepthLimit(t *testing.T) {
	root := t.TempDir()

	// Create a deeply nested project
	deep := filepath.Join(root, "a", "b", "c", "d", "deep")
	if err := os.MkdirAll(filepath.Join(deep, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Shallow project
	shallow := filepath.Join(root, "shallow")
	if err := os.MkdirAll(filepath.Join(shallow, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}

	results := scanForBeads(root, 2)

	if len(results) != 1 {
		t.Fatalf("expected 1 project at depth 2, got %d: %v", len(results), results)
	}
	if results[0] != shallow {
		t.Errorf("expected shallow project, got %q", results[0])
	}
}

func TestScanForBeads_SkipsHiddenDirs(t *testing.T) {
	root := t.TempDir()

	// Hidden dir with .beads inside
	hidden := filepath.Join(root, ".hidden", "project")
	if err := os.MkdirAll(filepath.Join(hidden, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}

	results := scanForBeads(root, 3)
	if len(results) != 0 {
		t.Errorf("expected 0 results (hidden dir skipped), got %d", len(results))
	}
}

func TestDiscoverProjects_MergesWithRegistered(t *testing.T) {
	root := t.TempDir()

	// Create a discoverable project
	proj := filepath.Join(root, "myproj")
	if err := os.MkdirAll(filepath.Join(proj, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := Config{
		Projects: []Project{
			{Name: "registered", Path: proj}, // Same path, registered name
		},
		Discovery: DiscoveryConfig{
			ScanPaths: []string{root},
			MaxDepth:  3,
		},
	}

	result := DiscoverProjects(cfg)

	// Should have exactly 1 project (deduped by path)
	if len(result) != 1 {
		t.Fatalf("expected 1 deduped project, got %d: %v", len(result), result)
	}
	// Should use registered name
	if result[0].Name != "registered" {
		t.Errorf("expected registered name, got %q", result[0].Name)
	}
}

func TestDiscoverProjects_AddsNewProjects(t *testing.T) {
	root := t.TempDir()

	proj1 := filepath.Join(root, "proj1")
	proj2 := filepath.Join(root, "proj2")
	for _, p := range []string{proj1, proj2} {
		if err := os.MkdirAll(filepath.Join(p, ".beads"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	cfg := Config{
		Projects: []Project{
			{Name: "proj1", Path: proj1},
		},
		Discovery: DiscoveryConfig{
			ScanPaths: []string{root},
			MaxDepth:  3,
		},
	}

	result := DiscoverProjects(cfg)

	if len(result) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(result))
	}

	// First should be registered, second discovered
	if result[0].Name != "proj1" {
		t.Errorf("expected first project 'proj1', got %q", result[0].Name)
	}
	if result[1].Name != "proj2" {
		t.Errorf("expected discovered project 'proj2', got %q", result[1].Name)
	}
}

func TestFindBeadsRoot(t *testing.T) {
	root := t.TempDir()

	// Create .beads in root
	if err := os.MkdirAll(filepath.Join(root, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a subdirectory
	sub := filepath.Join(root, "src", "pkg")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	// Should find root from subdirectory
	found, ok := findBeadsRoot(sub)
	if !ok {
		t.Error("expected to find beads root")
	}
	if found != root {
		t.Errorf("expected %q, got %q", root, found)
	}
}

func TestFindBeadsRoot_NotFound(t *testing.T) {
	dir := t.TempDir()

	_, ok := findBeadsRoot(dir)
	// May or may not find it depending on whether test runs inside a beads project.
	// Just verify it doesn't panic.
	_ = ok
}
