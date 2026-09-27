package bdrun

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
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

	out, err := Run(bdPath, dir, "comments", "add", "bd-1", "line one\nline two with spaces")

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

	out, err := Run(filepath.Join(binDir, "bd"), dir, "list")

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

	_, err := Run(bdPath, t.TempDir(), "close", "bd-1")

	if err == nil {
		t.Fatal("Run() err = nil, want an error for exit code 1")
	}
}
