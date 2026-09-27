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

// writeFakeBd installs a shell script named bd in a fresh PATH-only
// directory that echoes its args (one per line, so a caller can tell a
// space-containing argument was received intact) and exits with code.
func writeFakeBd(t *testing.T, code int) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake bd script needs a POSIX shell")
	}
	dir := t.TempDir()
	script := "#!/bin/sh\nfor a in \"$@\"; do printf '%s\\n' \"$a\"; done\nexit " + itoa(code) + "\n"
	path := filepath.Join(dir, "bd")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}

func TestResolve_FindsBdOnPath(t *testing.T) {
	binDir := writeFakeBd(t, 0)
	t.Setenv("PATH", binDir)

	path, ok := Resolve()

	if !ok {
		t.Fatal("Resolve() ok = false, want true")
	}
	if filepath.Dir(path) != binDir {
		t.Errorf("Resolve() path = %q, want it under %q", path, binDir)
	}
}

func TestResolve_NotFoundWhenBdIsMissing(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	_, ok := Resolve()

	if ok {
		t.Fatal("Resolve() ok = true, want false: no bd on PATH")
	}
}

func TestRun_PassesEachArgIntact(t *testing.T) {
	binDir := writeFakeBd(t, 0)
	bdPath := filepath.Join(binDir, "bd")
	dir := t.TempDir()

	out, err := Run(context.Background(), bdPath, dir, "comments", "add", "bd-1", "line one\nline two with spaces")

	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	want := "comments\nadd\nbd-1\nline one\nline two with spaces"
	if out != want {
		t.Errorf("Run output = %q, want %q", out, want)
	}
}

func TestRun_RunsInDir(t *testing.T) {
	binDir := t.TempDir()
	script := "#!/bin/sh\npwd\n"
	if err := os.WriteFile(filepath.Join(binDir, "bd"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()

	out, err := Run(context.Background(), filepath.Join(binDir, "bd"), dir, "list")

	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	resolvedDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	resolvedOut, err := filepath.EvalSymlinks(strings.TrimSpace(out))
	if err != nil {
		t.Fatal(err)
	}
	if resolvedOut != resolvedDir {
		t.Errorf("Run pwd = %q, want %q", resolvedOut, resolvedDir)
	}
}

func TestRun_ReturnsErrorOnNonZeroExit(t *testing.T) {
	binDir := writeFakeBd(t, 1)
	bdPath := filepath.Join(binDir, "bd")

	_, err := Run(context.Background(), bdPath, t.TempDir(), "close", "bd-1")

	if err == nil {
		t.Fatal("Run() err = nil, want an error for exit code 1")
	}
}

// TestRun_TimesOutOnAHungProcess proves the deadline itself, not only the
// process-group kill below: a fake bd that sleeps past ctx's deadline must
// return a clear timeout error within the deadline plus WaitDelay, never
// hang for the sleep's full duration.
func TestRun_TimesOutOnAHungProcess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake bd script needs a POSIX shell")
	}
	binDir := t.TempDir()
	script := "#!/bin/sh\nsleep 30\n"
	if err := os.WriteFile(filepath.Join(binDir, "bd"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := Run(ctx, filepath.Join(binDir, "bd"), t.TempDir(), "list")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Run() err = nil, want a timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("err = %v, want it to name the timeout clearly", err)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("Run took %v, want it bounded by the 200ms deadline plus WaitDelay, not the 30s sleep", elapsed)
	}
}

// TestRun_KillsWholeProcessGroupOnTimeout proves the process-group half of
// the fix: a fake bd that forks a grandchild holding stdout open (a
// backgrounded sh that never exits) must not block Run past its deadline.
// Killing only the direct bd process would leave that grandchild holding the
// CombinedOutput pipe open, and Wait would then block on it indefinitely.
func TestRun_KillsWholeProcessGroupOnTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake bd script needs a POSIX shell")
	}
	binDir := t.TempDir()
	// The backgrounded child inherits bd's stdout pipe and outlives bd
	// itself; only a process-group kill reaches it.
	script := "#!/bin/sh\n(sleep 30) &\nsleep 30\n"
	if err := os.WriteFile(filepath.Join(binDir, "bd"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := Run(ctx, filepath.Join(binDir, "bd"), t.TempDir(), "list")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Run() err = nil, want a timeout error")
	}
	if elapsed > 5*time.Second {
		t.Fatalf("Run took %v, want the grandchild's pipe hold to be bounded by WaitDelay, not the 30s sleep", elapsed)
	}
}

// TestRun_ContextCancellationIsReportedAsTimeout guards the error-shaping
// branch specifically: errors.Is(ctx.Err(), context.DeadlineExceeded) must
// be the check Run uses, not a bare ctx.Err() != nil, since a deliberate
// cancellation (not a deadline) should surface as the underlying exec error
// rather than "bd timed out".
func TestRun_ContextCancellationIsReportedAsTimeout(t *testing.T) {
	binDir := writeFakeBd(t, 0)
	bdPath := filepath.Join(binDir, "bd")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Run(ctx, bdPath, t.TempDir(), "list")

	if err == nil {
		t.Fatal("Run() err = nil, want an error for an already-cancelled context")
	}
	if strings.Contains(err.Error(), "timed out") {
		t.Errorf("err = %v, want cancellation (not a deadline) to not claim a timeout", err)
	}
	if !errors.Is(err, context.Canceled) && !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("err = %v, want it to reflect context cancellation", err)
	}
}
