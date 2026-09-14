package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vanderheijden86/beadwork/internal/datasource"
	"github.com/vanderheijden86/beadwork/pkg/config"
)

// chdirIntoDoltCheckout makes the working directory a Dolt checkout whose
// database must never stand in for a project that has no checkout.
func chdirIntoDoltCheckout(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	beadsDir := filepath.Join(dir, ".beads")
	if err := os.Mkdir(beadsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	metadata := `{"dolt_mode":"server","dolt_server_host":"127.0.0.1","dolt_server_port":3306,"dolt_server_user":"cwd_user","dolt_database":"cwd_db"}`
	if err := os.WriteFile(filepath.Join(beadsDir, "metadata.json"), []byte(metadata), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
}

func TestProjectSourcesWithoutCheckoutUseDatabaseAndStartupUser(t *testing.T) {
	chdirIntoDoltCheckout(t)
	m := NewModel(nil, "").WithDoltFailure(&DoltFailure{User: "bd_startup"})

	sources, err := m.projectSources(config.Project{Name: "remote", Database: "remote_db", Host: "10.0.0.5:3306"})

	if err != nil {
		t.Fatalf("projectSources: %v", err)
	}
	want := datasource.DataSource{Type: datasource.SourceTypeDolt, Path: "10.0.0.5:3306", Database: "remote_db", User: "bd_startup"}
	if len(sources) != 1 || sources[0] != want {
		t.Errorf("sources = %+v, want only %+v", sources, want)
	}
}

func TestProjectSourcesWithoutCheckoutOrDatabaseAreEmpty(t *testing.T) {
	chdirIntoDoltCheckout(t)
	m := NewModel(nil, "")

	sources, err := m.projectSources(config.Project{Name: "nothing"})

	if err != nil || len(sources) != 0 {
		t.Errorf("sources = %+v, %v; want none rather than the working directory's", sources, err)
	}
}

func TestActiveProjectSlotFollowsHeaderOrder(t *testing.T) {
	cfg := config.Config{RecentProjects: []config.RecentProject{
		{Name: "first", Database: "first_db", Host: "10.0.0.5:3306"},
		{Name: "second", Database: "second_db", Host: "10.0.0.5:3306"},
	}}
	m := NewModel(nil, "").WithConfig(cfg, "second", "")

	if got := m.activeProjectSlot(); got != 2 {
		t.Errorf("active project slot = %d, want 2 (its position in the header)", got)
	}

	m.activeProjectName = "unlisted"
	if got := m.activeProjectSlot(); got != 0 {
		t.Errorf("slot for a project missing from the header = %d, want 0", got)
	}
}

func TestActiveProjectCarriesDatabaseOfProjectWithoutCheckout(t *testing.T) {
	cfg := config.Config{RecentProjects: []config.RecentProject{
		{Name: "remote", Database: "remote_db", Host: "10.0.0.5:3306"},
	}}
	m := NewModel(nil, "").WithConfig(cfg, "startup", t.TempDir())
	m.activeProjectName = "remote"
	m.activeProjectPath = ""

	got := m.activeProject()

	want := config.Project{Name: "remote", Database: "remote_db", Host: "10.0.0.5:3306"}
	if got != want {
		t.Errorf("activeProject = %+v, want %+v", got, want)
	}
}
