package datasource

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// bdInTemp runs the real bd in a throwaway embedded project. Every BEADS_*
// variable of the calling shell is dropped, so no server setting or password
// can point bd at a shared database.
func bdInTemp(t *testing.T, projectDir string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bd", args...)
	cmd.Dir = projectDir
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "BEADS_") || strings.HasPrefix(kv, "BD_") {
			continue
		}
		cmd.Env = append(cmd.Env, kv)
	}
	cmd.Env = append(cmd.Env,
		"BEADS_DIR="+filepath.Join(projectDir, ".beads"),
		"BEADS_DOLT_AUTO_START=0",
		"BD_NON_INTERACTIVE=1",
	)
	out, err := cmd.Output()
	if err != nil {
		stderr := ""
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = string(ee.Stderr)
		}
		t.Fatalf("bd %v: %v\n%s%s", args, err, out, stderr)
	}
	return strings.TrimSpace(string(out))
}

// TestEmbeddedBD_ReadsAndWatchesARealStore pins ADR 0025 against the bd on
// PATH: b9s reads a fresh embedded project, a read does not look like a
// change, and a bd write does.
func TestEmbeddedBD_ReadsAndWatchesARealStore(t *testing.T) {
	if testing.Short() {
		t.Skip("starts the real bd")
	}
	if _, err := exec.LookPath("bd"); err != nil {
		t.Skip("bd not on PATH")
	}
	projectDir := t.TempDir()
	bdInTemp(t, projectDir, "init", "--prefix", "emb", "--quiet", "--non-interactive", "--skip-agents", "--skip-hooks")

	var meta beadsMetadata
	raw, err := os.ReadFile(filepath.Join(projectDir, ".beads", "metadata.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &meta); err != nil || meta.DoltMode != "embedded" {
		t.Fatalf("bd init did not create an embedded project (dolt_mode=%q, err=%v); refusing to write", meta.DoltMode, err)
	}
	id := bdInTemp(t, projectDir, "create", "First", "--type", "task", "--silent")

	opened, failure := OpenProject(OpenTarget{Name: "emb", Dir: projectDir})
	if failure != nil {
		t.Fatalf("OpenProject: %v", failure)
	}
	if opened.Source.Type != SourceTypeDoltEmbedded {
		t.Fatalf("opened %s, want %s", opened.Source.Type, SourceTypeDoltEmbedded)
	}
	if len(opened.Issues) != 1 || opened.Issues[0].ID != id || opened.Issues[0].Title != "First" {
		t.Fatalf("issues = %+v, want only %s First", opened.Issues, id)
	}

	w, err := NewDoltWatcher(opened.Source, 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Start(); err != nil {
		t.Fatal(err)
	}
	defer w.Stop()

	if _, err := LoadFromSource(opened.Source); err != nil {
		t.Fatalf("reload: %v", err)
	}
	select {
	case <-w.Changed():
		t.Fatal("a read through bd export was reported as a change")
	case <-time.After(500 * time.Millisecond):
	}

	bdInTemp(t, projectDir, "update", id, "--title", "Renamed")
	select {
	case <-w.Changed():
	case <-time.After(10 * time.Second):
		t.Fatal("no change notification within 10s of a bd write")
	}
	issues, err := LoadFromSource(opened.Source)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 1 || issues[0].Title != "Renamed" {
		t.Fatalf("after the write: %+v, want the title Renamed", issues)
	}
}
