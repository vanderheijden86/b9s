package ui_test

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanderheijden86/beadwork/pkg/config"
	"github.com/vanderheijden86/beadwork/pkg/model"
	"github.com/vanderheijden86/beadwork/pkg/ui"
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

func TestWithConfig_IgnoresScanPaths(t *testing.T) {
	root, projects := createSampleProjects(t)
	cfg := config.Config{Discovery: config.DiscoveryConfig{ScanPaths: []string{root}, MaxDepth: 2}}

	m := headerModel(t, cfg, projects[0])

	if got := m.ProjectPickerFilteredCount(); got != 1 {
		t.Errorf("header has %d projects, want only the startup project", got)
	}
}

func TestWithConfig_IgnoresRegisteredProjectsList(t *testing.T) {
	_, projects := createSampleProjects(t)
	cfg := config.Config{Projects: projects}

	m := headerModel(t, cfg, projects[0])

	if got := m.ProjectPickerFilteredCount(); got != 1 {
		t.Errorf("header has %d projects, want only the startup project", got)
	}
}
