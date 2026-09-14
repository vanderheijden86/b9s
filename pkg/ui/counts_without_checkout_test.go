package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vanderheijden86/beadwork/pkg/config"
)

// chdirIntoProjectWithOpenIssue makes the working directory a checkout whose
// issues would be miscounted for any project whose empty path resolves here.
func chdirIntoProjectWithOpenIssue(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	beadsDir := filepath.Join(dir, ".beads")
	if err := os.Mkdir(beadsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	issue := `{"id":"cwd-1","title":"Working directory issue","status":"open","issue_type":"task","priority":2,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}` + "\n"
	if err := os.WriteFile(filepath.Join(beadsDir, "issues.jsonl"), []byte(issue), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
}

func TestProjectCountsSkipProjectWithoutCheckout(t *testing.T) {
	chdirIntoProjectWithOpenIssue(t)

	msg := loadProjectCountsCmd([]config.Project{{Name: "remote", Database: "remote_db", Host: "127.0.0.1:1"}})()

	loaded, ok := msg.(projectCountsLoadedMsg)
	if !ok {
		t.Fatalf("loadProjectCountsCmd produced %T, want projectCountsLoadedMsg", msg)
	}
	if counts, found := loaded.counts[""]; found {
		t.Errorf("counts for a project without a checkout came from the working directory: %+v", counts)
	}
}

func TestHeaderEntrySeedsNoCountsForProjectWithoutCheckout(t *testing.T) {
	chdirIntoProjectWithOpenIssue(t)
	startup := t.TempDir()
	cfg := config.Config{RecentProjects: []config.RecentProject{
		{Name: "remote", Database: "remote_db", Host: "127.0.0.1:1"},
	}}
	m := NewModel(nil, "").WithConfig(cfg, "startup", startup)

	for _, entry := range m.buildProjectEntries() {
		if entry.Project.Name == "remote" && entry.OpenCount != 0 {
			t.Errorf("remote entry OpenCount = %d; it was seeded from the working directory", entry.OpenCount)
		}
	}
}

func TestOpenInEditorWithoutCheckoutRefusesWorkingDirectoryFile(t *testing.T) {
	chdirIntoProjectWithOpenIssue(t)
	// A terminal editor makes openInEditor return before launching anything,
	// so this test can never open a real editor window.
	t.Setenv("EDITOR", "vim")
	t.Setenv("VISUAL", "")
	m := NewModel(nil, "").WithConfig(config.Config{}, "remote", "")

	m.openInEditor()

	if !strings.Contains(m.statusMsg, "no local checkout") {
		t.Errorf("status = %q; opening the editor for a project without a checkout must refuse, not open the working directory's issues file", m.statusMsg)
	}
}
