package main_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeKeybindingConfig(t *testing.T, body string) string {
	t.Helper()
	configHome := t.TempDir()
	dir := filepath.Join(configHome, "b9s")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return configHome
}

// Y opens the type quick filter and a digit writes a type term into the query
// bar, in the tree and on the board.
func TestTypeQuickFilterE2E(t *testing.T) {
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, makeFilterFixture(t))
	configHome := writeKeybindingConfig(t, "ui:\n  default_view: list\n")

	out, err := runTreeTUIWithEnv(t, tempDir, 3500, []keyStep{
		kd("Y", 600*time.Millisecond),
		kd("4", 600*time.Millisecond),
		kd("b", 600*time.Millisecond),
		kd("1", 600*time.Millisecond),
	}, "XDG_CONFIG_HOME="+configHome)
	if err != nil {
		t.Fatalf("run TUI: %v", err)
	}
	containsAll(t, out, []string{"type:epic", "type:epic type:bug"})
}

// A user binding from config.yaml fills the query bar and shows in the footer.
func TestCustomKeybindingE2E(t *testing.T) {
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, makeFilterFixture(t))
	configHome := writeKeybindingConfig(t, `keybindings:
  - key: ctrl+t
    query: "type:task"
    description: Tasks
`)

	out, err := runTreeTUIWithEnv(t, tempDir, 3000, []keyStep{
		kd("\x14", 800*time.Millisecond),
	}, "XDG_CONFIG_HOME="+configHome)
	if err != nil {
		t.Fatalf("run TUI: %v", err)
	}
	containsAll(t, out, []string{"^t", "Tasks", "type:task"})
}

// A binding b9s cannot honour stops startup with a message that names it.
func TestInvalidKeybindingStopsStartupE2E(t *testing.T) {
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, makeFilterFixture(t))
	configHome := writeKeybindingConfig(t, `keybindings:
  - key: x
    query: "type:epic"
  - key: ctrl+t
    action: explode
`)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, buildBvBinary(t))
	cmd.Dir = tempDir
	cmd.Env = append(os.Environ(), "XDG_CONFIG_HOME="+configHome, "B9S_TUI_AUTOCLOSE_MS=1000")
	out, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 2 {
		t.Fatalf("want exit code 2, got err=%v\n%s", err, out)
	}
	for _, want := range []string{`"x" is a built-in key`, `unknown action "explode"`} {
		if !strings.Contains(string(out), want) {
			t.Errorf("output misses %q:\n%s", want, out)
		}
	}
}
