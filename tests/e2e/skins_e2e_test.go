package main_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// :skins must reach the picker through the command prompt, apply the chosen
// skin and save it, so the next start and b9s web use it (ADR 0034).
func TestSkinsCommandAppliesAndSavesASkinFile(t *testing.T) {
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, makeFilterFixture(t))

	configHome := t.TempDir()
	skinsDir := filepath.Join(configHome, "b9s", "skins")
	if err := os.MkdirAll(skinsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile(filepath.Join("..", "..", "pkg", "skin", "testdata", "gruvbox-light.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	skinFile := filepath.Join(skinsDir, "gruvbox-light.yaml")
	if err := os.WriteFile(skinFile, src, 0o644); err != nil {
		t.Fatal(err)
	}

	// The picker lists dracula, sepia, then the skin files, and opens on the
	// skin in use, so three steps down always end on gruvbox-light.
	out, err := runTreeTUIWithEnv(t, tempDir, 3000, []keyStep{
		kd(":", 300*time.Millisecond),
		kd("skins", 200*time.Millisecond),
		kd("\r", 400*time.Millisecond),
		kd("j", 150*time.Millisecond),
		kd("j", 150*time.Millisecond),
		kd("j", 150*time.Millisecond),
		kd("\r", 400*time.Millisecond),
	}, "XDG_CONFIG_HOME="+configHome)
	if err != nil {
		t.Fatalf("run tree TUI: %v", err)
	}
	containsAll(t, out, []string{"Skins", "gruvbox-light", "Skin: gruvbox-light"})

	saved, err := os.ReadFile(filepath.Join(configHome, "b9s", "config.yaml"))
	if err != nil {
		t.Fatalf("config not saved: %v", err)
	}
	if !strings.Contains(string(saved), "skin: "+skinFile) {
		t.Fatalf("config.yaml does not name the skin file:\n%s", saved)
	}
}
