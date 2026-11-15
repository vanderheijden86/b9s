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
	return writeConfigFile(t, "config.yaml", body)
}

func writeHotkeysFile(t *testing.T, body string) string {
	t.Helper()
	return writeConfigFile(t, "hotkeys.yaml", body)
}

func writeConfigFile(t *testing.T, name, body string) string {
	t.Helper()
	configHome := t.TempDir()
	dir := filepath.Join(configHome, "b9s")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return configHome
}

// F1-F8 open the fleet screens from the issue list and switch between them
// inside the fleet (ADR 0068). F1 never opens the help overlay. HOME points
// at an empty directory, so no real agent transcript is read.
func TestFleetFunctionKeysE2E(t *testing.T) {
	const f1, f6, f7 = "\x1bOP", "\x1b[17~", "\x1b[18~"
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, makeFilterFixture(t))
	configHome := writeKeybindingConfig(t, "ui:\n  default_view: list\n")

	out, err := runTreeTUIWithEnv(t, tempDir, 4000, []keyStep{
		kd(f1, 700*time.Millisecond),
		kd(f6, 700*time.Millisecond),
		kd(f7, 700*time.Millisecond),
		kd("q", 600*time.Millisecond),
	}, "XDG_CONFIG_HOME="+configHome, "HOME="+t.TempDir())
	if err != nil {
		t.Fatalf("run TUI: %v", err)
	}
	containsAll(t, out, []string{"F1-8", "<agents>", "0 sessions", "<pulse>", "Waiting on you", "<commits>", "F1-8 screens"})
	if strings.Contains(string(out), "This help") {
		t.Error("F1 opened the help overlay")
	}
}

// A hotkey from hotkeys.yaml, written as k9s writes one, runs its command,
// fills the query bar and shows in the footer.
func TestHotkeyE2E(t *testing.T) {
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, makeFilterFixture(t))
	configHome := writeHotkeysFile(t, `hotKeys:
  open-tasks:
    shortCut: Ctrl-T
    description: Open tasks
    command: task status:open
`)

	out, err := runTreeTUIWithEnv(t, tempDir, 3000, []keyStep{
		kd("\x14", 800*time.Millisecond),
	}, "XDG_CONFIG_HOME="+configHome)
	if err != nil {
		t.Fatalf("run TUI: %v", err)
	}
	containsAll(t, out, []string{"^t", "Open tasks", "status:open type:task"})
}

// A hotkey b9s cannot honour stops startup with a message that names it.
func TestInvalidHotkeyStopsStartupE2E(t *testing.T) {
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, makeFilterFixture(t))
	configHome := writeHotkeysFile(t, `hotKeys:
  epics:
    shortCut: x
    command: epic
  boom:
    shortCut: Ctrl-T
    command: explode
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
	for _, want := range []string{"hotkeys.yaml", "hotKeys.epics", "x is a built-in key", "hotKeys.boom", "unknown command: explode"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("output misses %q:\n%s", want, out)
		}
	}
}
