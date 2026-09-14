// open_failure_e2e_test.go - End-to-end proof that a project which cannot be
// opened is explained in plain words, and that b9s keeps or falls back to a
// project that works (bd-6e8h.8).
package main_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// resolvedTempDir resolves symlinks in a test directory. b9s sees its working
// directory without them (/private/var rather than /var on macOS), so a config
// written with the unresolved path would name a second, different project.
func resolvedTempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// unreachableFixture is a Dolt server project on a local port nothing listens
// on, so opening it fails fast with a refused connection.
func unreachableFixture(t *testing.T, root, name string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Join(dir, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	metadata := `{"dolt_mode":"server","dolt_server_host":"127.0.0.1","dolt_server_port":1,"dolt_server_user":"nobody","dolt_database":"` + name + `"}`
	if err := os.WriteFile(filepath.Join(dir, ".beads", "metadata.json"), []byte(metadata), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// writeRecentConfig seeds the isolated config home with recent projects; the
// first one has opened successfully before.
func writeRecentConfig(t *testing.T, opened string, others ...string) {
	t.Helper()
	dir := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "b9s")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	b.WriteString("recent_projects:\n")
	fmt.Fprintf(&b, "  - name: %s\n    path: %s\n    opened_at: %s\n", filepath.Base(opened), opened, time.Now().Add(-time.Hour).Format(time.RFC3339))
	for _, other := range others {
		fmt.Fprintf(&b, "  - name: %s\n    path: %s\n", filepath.Base(other), other)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestStartupInUnreachableProjectFallsBackWithPopupE2E(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := resolvedTempDir(t)
	good := recentFixture(t, root, "good")
	down := unreachableFixture(t, root, "down")
	writeRecentConfig(t, good)

	screen := runRecentTUI(t, down, 2500)

	for _, want := range []string{"Cannot open down", "Cannot reach the Dolt server at 127.0.0.1:1", "Showing good, the last project that opened successfully"} {
		if !strings.Contains(screen, want) {
			t.Errorf("screen lacks %q\nscreen:\n%s", want, screen)
		}
	}
}

func TestStartupWithoutTerminalExitsWithReasonE2E(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := resolvedTempDir(t)
	good := recentFixture(t, root, "good")
	writeRecentConfig(t, good)
	plain := filepath.Join(root, "plain")
	if err := os.Mkdir(plain, 0o755); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, buildBvBinary(t))
	cmd.Dir = plain
	cmd.Stdin = strings.NewReader("")
	out, err := cmd.CombinedOutput()

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("err = %v, want exit status 1 without a terminal\noutput:\n%s", err, out)
	}
	for _, want := range []string{"b9s: cannot open plain", "is not a Beads project", "Try"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(string(out), "Opening good instead") {
		t.Errorf("b9s fell back without a terminal:\n%s", out)
	}
}

func TestSwitchToUnreachableProjectKeepsCurrentE2E(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := resolvedTempDir(t)
	good := recentFixture(t, root, "good")
	down := unreachableFixture(t, root, "down")
	writeRecentConfig(t, good, down)

	screen := runRecentTUI(t, good, 3500, kd("2", 1000*time.Millisecond))

	for _, want := range []string{"Cannot open down", "Cannot reach the Dolt server", "Still showing good."} {
		if !strings.Contains(screen, want) {
			t.Errorf("screen lacks %q\nscreen:\n%s", want, screen)
		}
	}
}
