package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.UI.DefaultView != "list" {
		t.Errorf("expected default view 'list', got %q", cfg.UI.DefaultView)
	}
	if cfg.UI.SplitRatio != 0.4 {
		t.Errorf("expected split ratio 0.4, got %f", cfg.UI.SplitRatio)
	}
	if got := time.Duration(cfg.Refresh.PollInterval); got != 500*time.Millisecond {
		t.Errorf("expected refresh poll interval 500ms, got %s", got)
	}
}

func TestLoadFrom_NonExistent(t *testing.T) {
	cfg, err := LoadFrom("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
	if cfg.UI.DefaultView != "list" {
		t.Errorf("expected default config, got view %q", cfg.UI.DefaultView)
	}
}

func TestLoadFrom_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := `
recent_projects:
  - name: myproject
    path: ~/work/myproject
lock_recent: true

ui:
  default_view: tree
  split_ratio: 0.5

refresh:
  poll_interval: 2s
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(cfg.RecentProjects) != 1 {
		t.Fatalf("expected 1 recent project, got %d", len(cfg.RecentProjects))
	}
	home, _ := os.UserHomeDir()
	if want := filepath.Join(home, "work/myproject"); cfg.RecentProjects[0].Path != want {
		t.Errorf("expected expanded path %q, got %q", want, cfg.RecentProjects[0].Path)
	}
	if !cfg.LockRecent {
		t.Error("expected lock_recent true")
	}
	if cfg.UI.DefaultView != "tree" {
		t.Errorf("expected default_view 'tree', got %q", cfg.UI.DefaultView)
	}
	if cfg.UI.SplitRatio != 0.5 {
		t.Errorf("expected split_ratio 0.5, got %f", cfg.UI.SplitRatio)
	}
	if got := time.Duration(cfg.Refresh.PollInterval); got != 2*time.Second {
		t.Errorf("expected poll_interval 2s, got %s", got)
	}
}

// Configs written by older versions still carry projects, favorites and a
// discovery scan. They must keep loading; the scan settings are simply unused.
func TestLoadFrom_AcceptsLegacyProjectKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `
projects:
  - name: missing
    path: /no/such/checkout
favorites:
  1: missing
discovery:
  scan_paths:
    - ~/work
  max_depth: 2
ui:
  default_view: board
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if cfg.UI.DefaultView != "board" {
		t.Errorf("expected default_view 'board', got %q", cfg.UI.DefaultView)
	}
	if len(cfg.RecentProjects) != 0 {
		t.Errorf("a favorite without a checkout must not become a recent project, got %+v", cfg.RecentProjects)
	}
}

func TestLoadFrom_InvalidRefreshPollInterval(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("refresh:\n  poll_interval: eventually\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadFrom(path); err == nil {
		t.Fatal("expected invalid refresh poll interval to fail config loading")
	}
}

func TestLoadFrom_RefreshPollIntervalRejectsBusyLoop(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("refresh:\n  poll_interval: 10ms\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadFrom(path); err == nil {
		t.Fatal("expected refresh poll interval below 100ms to fail config loading")
	}
}

func TestLoadFrom_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	if err := os.WriteFile(path, []byte("{{invalid yaml"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadFrom(path)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestSaveAndLoad_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg := Config{
		RecentProjects: []RecentProject{
			{Name: "proj1", Path: "/path/to/proj1"},
			{Name: "proj2", Database: "proj2", Host: "127.0.0.1:3306"},
		},
		UI: UIConfig{
			DefaultView: "board",
			SplitRatio:  0.6,
		},
	}

	if err := SaveTo(cfg, path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("Load after save failed: %v", err)
	}

	if len(loaded.RecentProjects) != 2 {
		t.Fatalf("expected 2 recent projects, got %d", len(loaded.RecentProjects))
	}
	if loaded.RecentProjects[0].Name != "proj1" || loaded.RecentProjects[1].Database != "proj2" {
		t.Errorf("recent projects did not round-trip: %+v", loaded.RecentProjects)
	}
	if loaded.UI.DefaultView != "board" {
		t.Errorf("expected 'board', got %q", loaded.UI.DefaultView)
	}
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"~/foo", filepath.Join(home, "foo")},
		{"~/", filepath.Join(home, "")},
		{"/absolute", "/absolute"},
		{"relative", "relative"},
	}

	for _, tt := range tests {
		got := expandHome(tt.input)
		if got != tt.expected {
			t.Errorf("expandHome(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestConfigDir_XDGOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	got := ConfigDir()
	expected := filepath.Join(dir, "b9s")
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestDataDir_XDGOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dir)

	got := DataDir()
	expected := filepath.Join(dir, "b9s")
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestStateDir_XDGOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)

	got := StateDir()
	expected := filepath.Join(dir, "b9s")
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestExperimentalConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	boolTrue := true
	content := `
experimental:
  background_mode: true
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Experimental.BackgroundMode == nil {
		t.Fatal("expected background_mode to be set")
	}
	if *cfg.Experimental.BackgroundMode != boolTrue {
		t.Error("expected background_mode to be true")
	}
}

func TestDefaultConfig_SortsNewestCreatedFirst(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.UI.Sort.Field != "created" {
		t.Errorf("expected default sort field 'created', got %q", cfg.UI.Sort.Field)
	}
	if cfg.UI.Sort.Direction != "desc" {
		t.Errorf("expected default sort direction 'desc', got %q", cfg.UI.Sort.Direction)
	}
}

func TestLoadFrom_UISort(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("ui:\n  sort:\n    field: updated\n    direction: asc\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if cfg.UI.Sort.Field != "updated" || cfg.UI.Sort.Direction != "asc" {
		t.Errorf("expected sort updated/asc, got %q/%q", cfg.UI.Sort.Field, cfg.UI.Sort.Direction)
	}
}

// A field without a direction takes that field's natural direction, not the
// default's, so `field: title` sorts A-Z rather than inheriting "desc".
func TestLoadFrom_UISortFieldWithoutDirection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("ui:\n  sort:\n    field: title\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if cfg.UI.Sort.Direction != "" {
		t.Errorf("expected empty direction, got %q", cfg.UI.Sort.Direction)
	}
}

func TestLoadFrom_UISortRejectsUnknownField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("ui:\n  sort:\n    field: newest\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadFrom(path); err == nil {
		t.Fatal("expected unknown sort field to fail config loading")
	}
}

func TestLoadFrom_UISortRejectsUnknownDirection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("ui:\n  sort:\n    field: created\n    direction: down\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadFrom(path); err == nil {
		t.Fatal("expected unknown sort direction to fail config loading")
	}
}
