package ui_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vanderheijden86/beadwork/pkg/config"
	"github.com/vanderheijden86/beadwork/pkg/model"
	"github.com/vanderheijden86/beadwork/pkg/ui"
)

// createSampleProjects creates temp directories with .beads/issues.jsonl for testing.
func createSampleProjects(t *testing.T) (string, []config.Project) {
	t.Helper()
	root := t.TempDir()

	projects := []struct {
		name   string
		issues string
	}{
		{
			name: "api-service",
			issues: `{"id":"api-1","title":"Fix auth bug","status":"open","issue_type":"bug","priority":1,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}
{"id":"api-2","title":"Add rate limiting","status":"in_progress","issue_type":"feature","priority":2,"created_at":"2026-01-02T00:00:00Z","updated_at":"2026-01-02T00:00:00Z"}
{"id":"api-3","title":"Update docs","status":"open","issue_type":"task","priority":3,"created_at":"2026-01-03T00:00:00Z","updated_at":"2026-01-03T00:00:00Z"}
`,
		},
		{
			name: "web-frontend",
			issues: `{"id":"web-1","title":"Dark mode","status":"open","issue_type":"feature","priority":2,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}
{"id":"web-2","title":"Fix CSS grid","status":"blocked","issue_type":"bug","priority":1,"created_at":"2026-01-02T00:00:00Z","updated_at":"2026-01-02T00:00:00Z"}
`,
		},
		{
			name: "data-pipeline",
			issues: `{"id":"dp-1","title":"Optimize ETL","status":"open","issue_type":"task","priority":2,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}
`,
		},
	}

	var cfgProjects []config.Project
	for _, p := range projects {
		dir := filepath.Join(root, p.name)
		beadsDir := filepath.Join(dir, ".beads")
		if err := os.MkdirAll(beadsDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(beadsDir, "issues.jsonl"), []byte(p.issues), 0o644); err != nil {
			t.Fatal(err)
		}
		cfgProjects = append(cfgProjects, config.Project{Name: p.name, Path: dir})
	}

	return root, cfgProjects
}

// createModelWithProjects creates a Model loaded with sample projects.
func createModelWithProjects(t *testing.T) (ui.Model, config.Config) {
	t.Helper()
	_, projects := createSampleProjects(t)

	// Create some issues for the "active" project (api-service)
	issues := []model.Issue{
		{ID: "api-1", Title: "Fix auth bug", Status: "open", IssueType: "bug", Priority: 1, CreatedAt: time.Now()},
		{ID: "api-2", Title: "Add rate limiting", Status: "in_progress", IssueType: "feature", Priority: 2, CreatedAt: time.Now()},
		{ID: "api-3", Title: "Update docs", Status: "open", IssueType: "task", Priority: 3, CreatedAt: time.Now()},
	}

	cfg := config.Config{
		Projects:  projects,
		Favorites: map[int]string{1: "api-service", 3: "data-pipeline"},
		UI:        config.UIConfig{DefaultView: "list", SplitRatio: 0.4},
		Discovery: config.DiscoveryConfig{MaxDepth: 3},
	}

	m := ui.NewModel(issues, "").WithConfig(cfg, "api-service", projects[0].Path)
	// Send a window size so the model is ready
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return newM.(ui.Model), cfg
}

// switchToListView exits tree view (default) into list view.
func switchToListView(t *testing.T, m ui.Model) ui.Model {
	t.Helper()
	// Default is tree view; press 't' to toggle off -> list view
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	m = newM.(ui.Model)
	if m.FocusState() == "tree" {
		t.Fatal("expected to leave tree view after 't'")
	}
	return m
}

func TestProjectPicker_OpenAndClose(t *testing.T) {
	m, _ := createModelWithProjects(t)

	if m.ShowProjectPicker() {
		t.Fatal("picker should be closed initially")
	}

	// Switch to list view (P in tree view = jump-to-parent)
	m = switchToListView(t, m)

	// Press P to open
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("P")})
	m = newM.(ui.Model)

	if !m.ShowProjectPicker() {
		t.Fatal("picker should be open after P")
	}

	// Press esc to close
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = newM.(ui.Model)

	if m.ShowProjectPicker() {
		t.Fatal("picker should be closed after esc")
	}
}

func TestProjectPicker_ShowsAllProjects(t *testing.T) {
	m, _ := createModelWithProjects(t)
	m = switchToListView(t, m)

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("P")})
	m = newM.(ui.Model)

	if !m.ShowProjectPicker() {
		t.Fatal("picker should be open")
	}

	// Should show all 3 projects
	if m.ProjectPickerFilteredCount() != 3 {
		t.Errorf("expected 3 projects in picker, got %d", m.ProjectPickerFilteredCount())
	}
}

func TestProjectPicker_Navigation(t *testing.T) {
	m, _ := createModelWithProjects(t)
	m = switchToListView(t, m)

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("P")})
	m = newM.(ui.Model)

	if m.ProjectPickerCursor() != 0 {
		t.Fatalf("expected cursor at 0, got %d", m.ProjectPickerCursor())
	}

	// Move down with j
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = newM.(ui.Model)

	if m.ProjectPickerCursor() != 1 {
		t.Errorf("expected cursor at 1 after j, got %d", m.ProjectPickerCursor())
	}

	// Move down again
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = newM.(ui.Model)

	if m.ProjectPickerCursor() != 2 {
		t.Errorf("expected cursor at 2, got %d", m.ProjectPickerCursor())
	}

	// Move up with k
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	m = newM.(ui.Model)

	if m.ProjectPickerCursor() != 1 {
		t.Errorf("expected cursor at 1 after k, got %d", m.ProjectPickerCursor())
	}
}

func TestProjectPicker_ActiveProjectHighlighted(t *testing.T) {
	m, _ := createModelWithProjects(t)

	if m.ActiveProjectName() != "api-service" {
		t.Fatalf("expected active project 'api-service', got %q", m.ActiveProjectName())
	}
}

func TestProjectPicker_ViewContainsProjectInfo(t *testing.T) {
	entries := []ui.ProjectEntry{
		{
			Project:      config.Project{Name: "api-service", Path: "/tmp/api-service"},
			FavoriteNum:  1,
			IsActive:     true,
			OpenCount:    3,
			ReadyCount:   2,
			BlockedCount: 1,
		},
		{
			Project:      config.Project{Name: "web-frontend", Path: "/tmp/web-frontend"},
			FavoriteNum:  0,
			IsActive:     false,
			OpenCount:    2,
			ReadyCount:   1,
			BlockedCount: 1,
		},
		{
			Project:      config.Project{Name: "data-pipeline", Path: "/tmp/data-pipeline"},
			FavoriteNum:  3,
			IsActive:     false,
			OpenCount:    1,
			ReadyCount:   1,
			BlockedCount: 0,
		},
	}

	theme := ui.TestTheme()
	picker := ui.NewProjectPicker(entries, theme)
	picker.SetSize(120, 40)

	view := picker.View()

	// Should contain project names
	for _, name := range []string{"api-service", "web-frontend", "data-pipeline"} {
		if !strings.Contains(view, name) {
			t.Errorf("view should contain project name %q", name)
		}
	}

	// Should contain the title bar
	if !strings.Contains(view, "projects") {
		t.Error("view should contain 'projects' title")
	}

	// Should contain column headers
	if !strings.Contains(view, "NAME") {
		t.Error("view should contain NAME column header")
	}
	if !strings.Contains(view, "BLOCKED") {
		t.Error("view should contain BLOCKED column header")
	}

	// Should contain shortcut hints
	if !strings.Contains(view, "Switch") {
		t.Error("view should contain 'Switch' shortcut hint")
	}
	if !strings.Contains(view, "Filter") {
		t.Error("view should contain 'Filter' shortcut hint")
	}
}

func TestProjectPicker_FilterProjects(t *testing.T) {
	entries := []ui.ProjectEntry{
		{Project: config.Project{Name: "api-service", Path: "/tmp/api"}},
		{Project: config.Project{Name: "web-frontend", Path: "/tmp/web"}},
		{Project: config.Project{Name: "data-pipeline", Path: "/tmp/data"}},
	}

	theme := ui.TestTheme()
	picker := ui.NewProjectPicker(entries, theme)
	picker.SetSize(120, 40)

	// Enter filter mode
	picker, _ = picker.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})

	if !picker.Filtering() {
		t.Fatal("should be in filter mode after /")
	}

	// Type "api-" (specific enough to match only api-service)
	for _, ch := range "api-" {
		picker, _ = picker.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}

	// api-service should be top result
	selected := picker.SelectedEntry()
	if selected == nil || selected.Project.Name != "api-service" {
		name := ""
		if selected != nil {
			name = selected.Project.Name
		}
		t.Errorf("expected api-service as top filter result, got %q", name)
	}

	// Esc clears filter
	picker, _ = picker.Update(tea.KeyMsg{Type: tea.KeyEscape})

	if picker.Filtering() {
		t.Error("should not be filtering after esc")
	}
	if picker.FilteredCount() != 3 {
		t.Errorf("expected all 3 projects after filter clear, got %d", picker.FilteredCount())
	}
}

func TestProjectPicker_QuickSwitchByNumber(t *testing.T) {
	entries := []ui.ProjectEntry{
		{Project: config.Project{Name: "api-service", Path: "/tmp/api"}, FavoriteNum: 1},
		{Project: config.Project{Name: "web-frontend", Path: "/tmp/web"}, FavoriteNum: 0},
		{Project: config.Project{Name: "data-pipeline", Path: "/tmp/data"}, FavoriteNum: 3},
	}

	theme := ui.TestTheme()
	picker := ui.NewProjectPicker(entries, theme)
	picker.SetSize(120, 40)

	// Press 3 to quick-switch to data-pipeline (favorite #3)
	_, cmd := picker.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3")})

	if cmd == nil {
		t.Fatal("expected a command from quick-switch")
	}

	msg := cmd()
	switchMsg, ok := msg.(ui.SwitchProjectMsg)
	if !ok {
		t.Fatalf("expected SwitchProjectMsg, got %T", msg)
	}
	if switchMsg.Project.Name != "data-pipeline" {
		t.Errorf("expected data-pipeline, got %q", switchMsg.Project.Name)
	}
}

func TestProjectPicker_EnterSwitches(t *testing.T) {
	entries := []ui.ProjectEntry{
		{Project: config.Project{Name: "api-service", Path: "/tmp/api"}},
		{Project: config.Project{Name: "web-frontend", Path: "/tmp/web"}},
	}

	theme := ui.TestTheme()
	picker := ui.NewProjectPicker(entries, theme)
	picker.SetSize(120, 40)

	// Move to second entry
	picker, _ = picker.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})

	// Press enter
	_, cmd := picker.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if cmd == nil {
		t.Fatal("expected a command from enter")
	}

	msg := cmd()
	switchMsg, ok := msg.(ui.SwitchProjectMsg)
	if !ok {
		t.Fatalf("expected SwitchProjectMsg, got %T", msg)
	}
	if switchMsg.Project.Name != "web-frontend" {
		t.Errorf("expected web-frontend, got %q", switchMsg.Project.Name)
	}
}

func TestProjectPicker_FavoriteToggle(t *testing.T) {
	entries := []ui.ProjectEntry{
		{Project: config.Project{Name: "api-service", Path: "/tmp/api"}, FavoriteNum: 1},
		{Project: config.Project{Name: "web-frontend", Path: "/tmp/web"}, FavoriteNum: 0},
	}

	theme := ui.TestTheme()
	picker := ui.NewProjectPicker(entries, theme)
	picker.SetSize(120, 40)

	// Move to web-frontend (no favorite) and press u
	picker, _ = picker.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	_, cmd := picker.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")})

	if cmd == nil {
		t.Fatal("expected a command from favorite toggle")
	}

	msg := cmd()
	toggleMsg, ok := msg.(ui.ToggleFavoriteMsg)
	if !ok {
		t.Fatalf("expected ToggleFavoriteMsg, got %T", msg)
	}
	if toggleMsg.ProjectName != "web-frontend" {
		t.Errorf("expected web-frontend, got %q", toggleMsg.ProjectName)
	}
	// Slot 1 is taken by api-service, so should assign slot 2
	if toggleMsg.SlotNumber != 2 {
		t.Errorf("expected slot 2 (first available), got %d", toggleMsg.SlotNumber)
	}
}

func TestProjectPicker_GoToTopBottom(t *testing.T) {
	entries := []ui.ProjectEntry{
		{Project: config.Project{Name: "alpha", Path: "/tmp/a"}},
		{Project: config.Project{Name: "beta", Path: "/tmp/b"}},
		{Project: config.Project{Name: "gamma", Path: "/tmp/c"}},
		{Project: config.Project{Name: "delta", Path: "/tmp/d"}},
	}

	theme := ui.TestTheme()
	picker := ui.NewProjectPicker(entries, theme)
	picker.SetSize(120, 40)

	// Go to bottom with G
	picker, _ = picker.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	if picker.Cursor() != 3 {
		t.Errorf("expected cursor at 3 (bottom), got %d", picker.Cursor())
	}

	// Go to top with g
	picker, _ = picker.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	if picker.Cursor() != 0 {
		t.Errorf("expected cursor at 0 (top), got %d", picker.Cursor())
	}
}

func TestProjectPicker_NoProjectsMessage(t *testing.T) {
	theme := ui.TestTheme()
	picker := ui.NewProjectPicker(nil, theme)
	picker.SetSize(120, 40)

	view := picker.View()
	if !strings.Contains(view, "No projects found") {
		t.Error("expected 'No projects found' message when no projects")
	}
}
