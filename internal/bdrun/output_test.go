package bdrun

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// writeScript installs script as an executable named bd and returns its path.
func writeScript(t *testing.T, script string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake bd script needs a POSIX shell")
	}
	path := filepath.Join(t.TempDir(), "bd")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestOutput_KeepsStderrOutOfStdout(t *testing.T) {
	bd := writeScript(t, "echo '{\"id\":\"a-1\"}'\necho 'hint: something' >&2\n")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stdout, stderr, err := Output(ctx, bd, t.TempDir(), nil, "export")
	if err != nil {
		t.Fatalf("Output: %v", err)
	}
	if got := string(stdout); got != "{\"id\":\"a-1\"}\n" {
		t.Fatalf("stdout = %q, want the JSON line only", got)
	}
	if !strings.Contains(stderr, "hint: something") {
		t.Fatalf("stderr = %q, want the hint", stderr)
	}
}

func TestOutput_SetsExtraEnvironment(t *testing.T) {
	bd := writeScript(t, "printf '%s' \"$BEADS_DIR\"\n")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stdout, _, err := Output(ctx, bd, t.TempDir(), []string{"BEADS_DIR=/some/.beads"}, "export")
	if err != nil {
		t.Fatalf("Output: %v", err)
	}
	if string(stdout) != "/some/.beads" {
		t.Fatalf("BEADS_DIR seen by bd = %q, want /some/.beads", stdout)
	}
}

func TestOutput_NonZeroExitCarriesStderr(t *testing.T) {
	bd := writeScript(t, "echo 'Error: database is locked' >&2\nexit 1\n")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, stderr, err := Output(ctx, bd, t.TempDir(), nil, "export")
	if err == nil {
		t.Fatal("Output succeeded on exit 1, want an error")
	}
	if !strings.Contains(stderr, "database is locked") {
		t.Fatalf("stderr = %q, want bd's message", stderr)
	}
}

func TestOutput_TimesOut(t *testing.T) {
	bd := writeScript(t, "sleep 30\n")
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, _, err := Output(ctx, bd, t.TempDir(), nil, "export")
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("err = %v, want ErrTimeout", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("Output returned after %s, want soon after the deadline", elapsed)
	}
}
