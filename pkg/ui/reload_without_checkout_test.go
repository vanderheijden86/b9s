package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanderheijden86/beadwork/internal/datasource"
	"github.com/vanderheijden86/beadwork/pkg/config"
)

// A project opened from its database has no checkout path. Its reload must use
// that database, never a .beads directory resolved against the working
// directory, which belongs to the project b9s was started in.
func TestReloadWithoutCheckoutNeverReadsWorkingDirectoryBeads(t *testing.T) {
	startupProject := t.TempDir()
	beadsDir := filepath.Join(startupProject, ".beads")
	if err := os.Mkdir(beadsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	startupIssue := `{"id":"startup-1","title":"Wrong startup database","status":"open","issue_type":"task","priority":2,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}` + "\n"
	if err := os.WriteFile(filepath.Join(beadsDir, "issues.jsonl"), []byte(startupIssue), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(startupProject)

	m := NewModel(nil, "").
		WithConfig(config.Config{}, "remote", "").
		WithSourceType(datasource.SourceTypeDolt)
	m.doltSource = datasource.DataSource{Type: datasource.SourceTypeDolt, Path: "127.0.0.1:1", Database: "remote", User: "reader"}
	// The reload skips work when it has neither a file nor a live Dolt watcher.
	// A connected project has a watcher; this path stands in for it so the
	// Dolt reload branch runs. The Dolt branch itself never reads beadsPath.
	m.beadsPath = filepath.Join(t.TempDir(), "stands-in-for-watcher.jsonl")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = updated.(Model)

	updated, _ = m.Update(FileChangedMsg{})
	m = updated.(Model)

	if strings.Contains(m.View(), "Wrong startup database") {
		t.Fatal("reload of a project without a checkout showed the startup project's issues")
	}
}
