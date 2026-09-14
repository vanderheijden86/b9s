package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func recent(name string) RecentProject {
	return RecentProject{Name: name, Database: name, Host: "127.0.0.1:3306"}
}

func recentNames(projects []RecentProject) []string {
	names := make([]string, 0, len(projects))
	for _, p := range projects {
		names = append(names, p.Name)
	}
	return names
}

func TestTouchRecent_PrependsNewProject(t *testing.T) {
	cfg := DefaultConfig()
	cfg.RecentProjects = []RecentProject{recent("a"), recent("b")}

	changed := cfg.TouchRecent(recent("c"))

	if !changed {
		t.Error("expected TouchRecent to report a change")
	}
	if got, want := recentNames(cfg.RecentProjects), []string{"c", "a", "b"}; !reflect.DeepEqual(got, want) {
		t.Errorf("recent = %v, want %v", got, want)
	}
}

func TestTouchRecent_KeepsSlotOfExistingProject(t *testing.T) {
	cfg := DefaultConfig()
	cfg.RecentProjects = []RecentProject{recent("a"), recent("b"), recent("c")}

	changed := cfg.TouchRecent(recent("c"))

	if changed {
		t.Error("expected no change for a project already in the list")
	}
	if got, want := recentNames(cfg.RecentProjects), []string{"a", "b", "c"}; !reflect.DeepEqual(got, want) {
		t.Errorf("recent = %v, want %v", got, want)
	}
}

func TestTouchRecent_IdentifiesProjectByHostAndDatabase(t *testing.T) {
	cfg := DefaultConfig()
	cfg.RecentProjects = []RecentProject{{Name: "a", Database: "a", Host: "127.0.0.1:3306"}}

	cfg.TouchRecent(RecentProject{Name: "a", Database: "a", Host: "10.0.0.5:3306"})

	if got := len(cfg.RecentProjects); got != 2 {
		t.Errorf("expected the same database on another host to be a new entry, got %d entries", got)
	}
}

func TestTouchRecent_DropsProjectBeyondCap(t *testing.T) {
	cfg := DefaultConfig()
	for i := 1; i <= MaxRecentProjects; i++ {
		cfg.RecentProjects = append(cfg.RecentProjects, recent(fmt.Sprintf("p%d", i)))
	}

	cfg.TouchRecent(recent("new"))

	if got := len(cfg.RecentProjects); got != MaxRecentProjects {
		t.Fatalf("expected %d entries, got %d", MaxRecentProjects, got)
	}
	if first, last := cfg.RecentProjects[0].Name, cfg.RecentProjects[MaxRecentProjects-1].Name; first != "new" || last != "p8" {
		t.Errorf("expected new first and p8 last, got %s ... %s", first, last)
	}
}

func TestTouchRecent_LockedListIgnoresNewProject(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LockRecent = true
	cfg.RecentProjects = []RecentProject{recent("a")}

	changed := cfg.TouchRecent(recent("b"))

	if changed {
		t.Error("expected a locked list not to change")
	}
	if got, want := recentNames(cfg.RecentProjects), []string{"a"}; !reflect.DeepEqual(got, want) {
		t.Errorf("recent = %v, want %v", got, want)
	}
}

func TestLoadFrom_MigratesFavoritesIntoRecentProjects(t *testing.T) {
	dir := t.TempDir()
	alpha := writeDoltProject(t, dir, "alpha", "alpha_db")
	beta := writeDoltProject(t, dir, "beta", "beta_db")
	cfgPath := filepath.Join(dir, "config.yaml")
	writeFile(t, cfgPath, fmt.Sprintf(`projects:
  - name: alpha
    path: %s
  - name: beta
    path: %s
favorites:
  2: alpha
  1: beta
`, alpha, beta))

	cfg, err := LoadFrom(cfgPath)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}

	want := []RecentProject{
		{Name: "beta", Database: "beta_db", Host: "127.0.0.1:3306", Path: beta},
		{Name: "alpha", Database: "alpha_db", Host: "127.0.0.1:3306", Path: alpha},
	}
	if !reflect.DeepEqual(cfg.RecentProjects, want) {
		t.Errorf("recent = %+v, want %+v", cfg.RecentProjects, want)
	}
}

func TestLoadFrom_KeepsExistingRecentProjectsOverFavorites(t *testing.T) {
	dir := t.TempDir()
	alpha := writeDoltProject(t, dir, "alpha", "alpha_db")
	cfgPath := filepath.Join(dir, "config.yaml")
	writeFile(t, cfgPath, fmt.Sprintf(`projects:
  - name: alpha
    path: %s
favorites:
  1: alpha
recent_projects:
  - name: gamma
    database: gamma
    host: 127.0.0.1:3306
`, alpha))

	cfg, err := LoadFrom(cfgPath)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}

	if got, want := recentNames(cfg.RecentProjects), []string{"gamma"}; !reflect.DeepEqual(got, want) {
		t.Errorf("recent = %v, want %v", got, want)
	}
}

func TestSaveTo_MergesRecentProjectsWrittenByAnotherWindow(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	cfg, err := LoadFrom(cfgPath)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}

	other := DefaultConfig()
	other.TouchRecent(recent("other"))
	if err := SaveTo(other, cfgPath); err != nil {
		t.Fatalf("SaveTo other: %v", err)
	}

	cfg.TouchRecent(recent("mine"))
	if err := SaveTo(cfg, cfgPath); err != nil {
		t.Fatalf("SaveTo mine: %v", err)
	}

	saved, err := LoadFrom(cfgPath)
	if err != nil {
		t.Fatalf("LoadFrom saved: %v", err)
	}
	if got, want := recentNames(saved.RecentProjects), []string{"mine", "other"}; !reflect.DeepEqual(got, want) {
		t.Errorf("recent = %v, want %v", got, want)
	}
}

func TestSaveTo_LeavesOnlyTheConfigFile(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultConfig()
	cfg.TouchRecent(recent("a"))

	if err := SaveTo(cfg, filepath.Join(dir, "config.yaml")); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "config.yaml" {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("expected only config.yaml, found %v", names)
	}
}

func writeDoltProject(t *testing.T, root, name, database string) string {
	t.Helper()
	path := filepath.Join(root, name)
	writeFile(t, filepath.Join(path, ".beads", "metadata.json"), fmt.Sprintf(
		`{"backend":"dolt","dolt_mode":"server","dolt_server_host":"127.0.0.1","dolt_server_port":3306,"dolt_server_user":"root","dolt_database":%q}`,
		database))
	return path
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}
