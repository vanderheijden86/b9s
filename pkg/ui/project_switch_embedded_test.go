package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vanderheijden86/b9s/internal/datasource"
	"github.com/vanderheijden86/b9s/pkg/config"
)

// writeEmbeddedCheckout lays out a project as `bd init` creates it by
// default: an embedded Dolt store and no issues.jsonl.
func writeEmbeddedCheckout(t *testing.T, root, name string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	noms := filepath.Join(dir, ".beads", "embeddeddolt", name, ".dolt", "noms")
	if err := os.MkdirAll(noms, 0o755); err != nil {
		t.Fatal(err)
	}
	meta := `{"database":"dolt","backend":"dolt","dolt_mode":"embedded","dolt_database":"` + name + `"}`
	if err := os.WriteFile(filepath.Join(dir, ".beads", "metadata.json"), []byte(meta), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(noms, "manifest"), []byte("m"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestOpenedEmbeddedProjectWatchesTheStore(t *testing.T) {
	m, _, _ := switchModel(t)
	gamma := config.Project{Name: "gamma", Path: writeEmbeddedCheckout(t, t.TempDir(), "gamma")}
	m, _ = requestSwitch(t, m, gamma)

	updated, _ := m.Update(projectOpenedMsg{generation: m.projectSwitch.generation, project: gamma})
	m = updated.(Model)
	if m.doltWatcher != nil {
		defer m.doltWatcher.Stop()
	}

	if m.activeProjectName != "gamma" {
		t.Fatalf("active = %q (status %q), want gamma: an embedded project needs no issues.jsonl", m.activeProjectName, m.statusMsg)
	}
	if m.sourceType != datasource.SourceTypeDoltEmbedded {
		t.Errorf("sourceType = %s, want %s", m.sourceType, datasource.SourceTypeDoltEmbedded)
	}
	if m.doltWatcher == nil {
		t.Error("no watcher on the embedded store: bd writes would never reload")
	}
	if m.backgroundWorker != nil {
		t.Error("a JSONL background worker runs for an embedded project")
	}
	if !strings.Contains(m.sourceInfo, "embedded") {
		t.Errorf("sourceInfo = %q, want it to name embedded Dolt", m.sourceInfo)
	}
	h := m.buildDatabaseHealth()
	if !strings.Contains(h.Backend, "embedded") || h.Database != "gamma" {
		t.Errorf("health = backend %q database %q, want embedded Dolt gamma", h.Backend, h.Database)
	}
}
