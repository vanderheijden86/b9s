package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadKeybindings(t *testing.T) {
	path := writeConfig(t, `keybindings:
  - key: E
    query: "type:epic"
    description: Epics
  - key: ctrl+g
    action: view_board
    override: true
`)
	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Keybindings) != 2 {
		t.Fatalf("got %d bindings, want 2", len(cfg.Keybindings))
	}
	first := cfg.Keybindings[0]
	if first.Key != "E" || first.Query != "type:epic" || first.Description != "Epics" || first.Override {
		t.Errorf("first binding = %+v", first)
	}
	second := cfg.Keybindings[1]
	if second.Key != "ctrl+g" || second.Action != "view_board" || !second.Override {
		t.Errorf("second binding = %+v", second)
	}
}

func TestLoadWithoutKeybindingsHasNone(t *testing.T) {
	cfg, err := LoadFrom(writeConfig(t, "ui:\n  default_view: list\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Keybindings) != 0 {
		t.Errorf("got %d bindings, want none", len(cfg.Keybindings))
	}
}

func TestLoadKeybindingsRejectsWrongShape(t *testing.T) {
	_, err := LoadFrom(writeConfig(t, "keybindings:\n  E: type:epic\n"))
	if !errors.Is(err, ErrKeybindings) {
		t.Fatalf("err = %v, want ErrKeybindings", err)
	}
}
