package ui_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanderheijden86/b9s/pkg/config"
	"github.com/vanderheijden86/b9s/pkg/model"
	"github.com/vanderheijden86/b9s/pkg/ui"
)

func recentFromProjects(projects ...config.Project) []config.RecentProject {
	recent := make([]config.RecentProject, 0, len(projects))
	for _, p := range projects {
		recent = append(recent, config.RecentProject{Name: p.Name, Path: p.Path})
	}
	return recent
}

func headerModel(t *testing.T, cfg config.Config, active config.Project) ui.Model {
	t.Helper()
	issues := []model.Issue{
		{ID: "x-1", Title: "Task", Status: "open", IssueType: "task", Priority: 2, CreatedAt: time.Now()},
	}
	m := ui.NewModel(issues, "").WithConfig(cfg, active.Name, active.Path)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	return updated.(ui.Model)
}

func switchTargetForKey(t *testing.T, m ui.Model, key string) string {
	t.Helper()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	if cmd == nil {
		t.Fatalf("key %s produced no command", key)
	}
	switchMsg, ok := cmd().(ui.SwitchProjectMsg)
	if !ok {
		t.Fatalf("key %s produced %T, want SwitchProjectMsg", key, cmd())
	}
	return switchMsg.Project.Name
}

func TestWithConfig_NumberKeysFollowStoredRecentOrder(t *testing.T) {
	_, projects := createSampleProjects(t)
	api, web, data := projects[0], projects[1], projects[2]
	cfg := config.Config{RecentProjects: recentFromProjects(data, api, web)}

	m := headerModel(t, cfg, api)

	if got := switchTargetForKey(t, m, "1"); got != "data-pipeline" {
		t.Errorf("key 1 switches to %q, want data-pipeline", got)
	}
	if got := switchTargetForKey(t, m, "3"); got != "web-frontend" {
		t.Errorf("key 3 switches to %q, want web-frontend", got)
	}
}

func TestWithConfig_PrependsStartupProjectMissingFromRecent(t *testing.T) {
	_, projects := createSampleProjects(t)
	api, web, data := projects[0], projects[1], projects[2]
	cfg := config.Config{RecentProjects: recentFromProjects(web, data)}

	m := headerModel(t, cfg, api)

	if got := m.ProjectPickerFilteredCount(); got != 3 {
		t.Fatalf("header has %d projects, want 3", got)
	}
	if got := switchTargetForKey(t, m, "2"); got != "web-frontend" {
		t.Errorf("key 2 switches to %q, want web-frontend", got)
	}
}

func TestWithConfig_SwitchCarriesDatabaseOfProjectWithoutCheckout(t *testing.T) {
	_, projects := createSampleProjects(t)
	api := projects[0]
	cfg := config.Config{RecentProjects: []config.RecentProject{
		{Name: api.Name, Path: api.Path},
		{Name: "remote", Database: "remote_db", Host: "10.0.0.5:3306"},
	}}
	m := headerModel(t, cfg, api)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	if cmd == nil {
		t.Fatal("key 2 produced no command")
	}
	switchMsg, ok := cmd().(ui.SwitchProjectMsg)
	if !ok {
		t.Fatalf("key 2 produced %T, want SwitchProjectMsg", cmd())
	}

	want := config.Project{Name: "remote", Database: "remote_db", Host: "10.0.0.5:3306"}
	if switchMsg.Project != want {
		t.Errorf("switch project = %+v, want %+v", switchMsg.Project, want)
	}
}

func TestHeaderProjects_StartupMatchesRecentEntryWithoutPathByDatabase(t *testing.T) {
	recent := []config.RecentProject{
		{Name: "LP_Team", Database: "LP_Team", Host: "127.0.0.1:3306"},
		{Name: "b9s", Database: "b9s", Host: "127.0.0.1:3306"},
	}
	startup := config.RecentProject{Name: "b9s", Path: "/src/b9s", Database: "b9s", Host: "127.0.0.1:3306"}

	got := ui.HeaderProjects(recent, startup)

	want := []config.Project{
		{Name: "LP_Team", Database: "LP_Team", Host: "127.0.0.1:3306"},
		{Name: "b9s", Path: "/src/b9s", Database: "b9s", Host: "127.0.0.1:3306"},
	}
	if len(got) != len(want) {
		t.Fatalf("header = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("row %d = %+v, want %+v", i+1, got[i], want[i])
		}
	}
}

func TestWithConfig_ListsDoltStartupOnceWhenItsRecentEntryHasNoPath(t *testing.T) {
	checkout := filepath.Join(t.TempDir(), "b9s")
	if err := os.MkdirAll(filepath.Join(checkout, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	meta := `{"backend":"dolt","dolt_mode":"server","dolt_server_host":"127.0.0.1","dolt_server_port":3306,"dolt_server_user":"root","dolt_database":"b9s"}`
	if err := os.WriteFile(filepath.Join(checkout, ".beads", "metadata.json"), []byte(meta), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{RecentProjects: []config.RecentProject{
		{Name: "b9s", Database: "b9s", Host: "127.0.0.1:3306"},
		{Name: "LP_Team", Database: "LP_Team", Host: "127.0.0.1:3306"},
	}}

	m := headerModel(t, cfg, config.Project{Name: "b9s", Path: checkout})

	if got := m.ProjectPickerFilteredCount(); got != 2 {
		t.Fatalf("header has %d projects, want 2", got)
	}
	if got := switchTargetForKey(t, m, "2"); got != "LP_Team" {
		t.Errorf("key 2 switches to %q, want LP_Team", got)
	}
}
