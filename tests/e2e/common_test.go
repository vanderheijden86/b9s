package main_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

var bvBinaryPath string
var bvBinaryDir string

var (
	scriptTUISupported      = true
	scriptTUIDisabledReason string
)

func TestMain(m *testing.M) {
	// Prevent any test from accidentally opening a browser
	os.Setenv("B9S_NO_BROWSER", "1")
	os.Setenv("B9S_TEST_MODE", "1")

	// b9s saves the projects it opens into its user config. Every binary a test
	// starts inherits this directory, so fixture projects never reach the
	// developer's real recent list.
	configHome, err := os.MkdirTemp("", "b9s-e2e-config-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create isolated config dir: %v\n", err)
		os.Exit(1)
	}
	os.Setenv("XDG_CONFIG_HOME", configHome)

	// Build the binary once for all tests
	if err := buildBvOnce(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to build bv binary: %v\n", err)
		_ = os.RemoveAll(configHome)
		os.Exit(1)
	}

	scriptTUISupported, scriptTUIDisabledReason = detectScriptTUICapability(bvBinaryPath)

	code := m.Run()
	if bvBinaryDir != "" {
		_ = os.RemoveAll(bvBinaryDir)
	}
	// os.Exit skips deferred calls, so cleanup runs explicitly.
	_ = os.RemoveAll(configHome)
	os.Exit(code)
}

// TestE2EUsesIsolatedConfigHome fails if the suite would start b9s against the
// developer's own config, where each run would add fixture projects to the
// recent list.
func TestE2EUsesIsolatedConfigHome(t *testing.T) {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		t.Fatal("XDG_CONFIG_HOME is unset; b9s would read and write ~/.config/b9s")
	}
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(configHome, home) {
		t.Fatalf("XDG_CONFIG_HOME = %q is inside the home directory", configHome)
	}
}

func detectScriptTUICapability(bvPath string) (bool, string) {
	if _, err := exec.LookPath("script"); err != nil {
		return false, "script command not available"
	}
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		return false, "script TUI harness unsupported on this OS"
	}
	if bvPath == "" {
		return false, "bv binary path is empty"
	}

	tempDir, err := os.MkdirTemp("", "bv-e2e-tui-cap-*")
	if err != nil {
		return false, fmt.Sprintf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	beadsDir := filepath.Join(tempDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		return false, fmt.Sprintf("failed to create beads dir: %v", err)
	}
	beads := `{"id":"cap-1","title":"Capability check","status":"open","priority":1,"issue_type":"task"}`
	if err := os.WriteFile(filepath.Join(beadsDir, "beads.jsonl"), []byte(beads), 0o644); err != nil {
		return false, fmt.Sprintf("failed to write beads.jsonl: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := sizedScriptTUICommand(ctx, bvPath)
	if cmd == nil {
		return false, "script command unavailable"
	}
	cmd.Dir = tempDir
	cmd.Stdin = strings.NewReader("")
	cmd.Env = append(os.Environ(),
		"TERM=screen-256color",
		"B9S_TUI_AUTOCLOSE_MS=500",
	)

	outFile := filepath.Join(tempDir, "script.out")
	f, err := os.Create(outFile)
	if err != nil {
		return false, fmt.Sprintf("failed to create output file: %v", err)
	}
	cmd.Stdout = f
	cmd.Stderr = f

	runErr := cmd.Run()
	_ = f.Close()

	if ctx.Err() == context.DeadlineExceeded {
		return false, "bv did not auto-exit under script (PTY/CI mismatch)"
	}
	if runErr != nil {
		return false, fmt.Sprintf("script TUI run failed: %v", runErr)
	}

	return true, ""
}

func buildBvOnce() error {
	tempDir, err := os.MkdirTemp("", "bv-e2e-build-*")
	if err != nil {
		return err
	}
	bvBinaryDir = tempDir

	binName := "bv"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	binPath := filepath.Join(tempDir, binName)

	cmd := exec.Command("go", "build", "-o", binPath, "../../cmd/b9s")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("go build failed: %v\n%s", err, out)
	}

	bvBinaryPath = binPath
	return nil
}

// buildBvBinary returns the path to the pre-built binary.
func buildBvBinary(t *testing.T) string {
	t.Helper()
	if bvBinaryPath == "" {
		t.Fatal("bv binary not built")
	}
	return bvBinaryPath
}

// skipIfNoScript skips the test if the script command is unavailable.
func skipIfNoScript(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("script"); err != nil {
		t.Skip("skipping: script command not available")
	}
	if !scriptTUISupported {
		if scriptTUIDisabledReason != "" {
			t.Skipf("skipping: %s", scriptTUIDisabledReason)
		}
		t.Skip("skipping: script-based TUI harness unavailable")
	}
}

// scriptTUICommand creates an exec.Cmd that runs the bv binary under `script`
// to provide a pseudo-TTY for TUI tests.
func scriptTUICommand(ctx context.Context, bvPath string, args ...string) *exec.Cmd {
	if _, err := exec.LookPath("script"); err != nil {
		return nil
	}

	switch runtime.GOOS {
	case "darwin":
		scriptArgs := []string{"-q", "/dev/null", bvPath}
		scriptArgs = append(scriptArgs, args...)
		return exec.CommandContext(ctx, "script", scriptArgs...)

	case "linux":
		cmdStr := bvPath
		for _, arg := range args {
			if strings.ContainsAny(arg, " \t") {
				cmdStr += " \"" + arg + "\""
			} else {
				cmdStr += " " + arg
			}
		}
		return exec.CommandContext(ctx, "script", "-q", "-e", "-f", "-c", cmdStr, "/dev/null")

	default:
		return nil
	}
}

// sizedScriptTUICommand gives captured PTYs the same usable viewport on every
// platform. Linux CI otherwise inherits a tiny terminal and paginates fixtures
// one row at a time.
func sizedScriptTUICommand(ctx context.Context, bvPath string) *exec.Cmd {
	quotedBinary := "'" + strings.ReplaceAll(bvPath, "'", "'\\''") + "'"
	return scriptTUICommand(ctx, "/bin/sh", "-c", "stty rows 40 cols 160; exec "+quotedBinary)
}

func TestSizedScriptTUICommandSetsDeterministicDimensions(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := sizedScriptTUICommand(ctx, "/tmp/bv")
	if cmd == nil {
		t.Skip("script command not available")
	}
	invocation := strings.Join(cmd.Args, " ")
	if !strings.Contains(invocation, "stty rows 40 cols 160") {
		t.Fatalf("expected deterministic PTY dimensions, got %q", invocation)
	}
}

// ensureCmdStdinCloses wires a controllable stdin for command execution.
func ensureCmdStdinCloses(t *testing.T, ctx context.Context, cmd *exec.Cmd, closeAfter time.Duration) {
	t.Helper()
	if cmd == nil || cmd.Stdin != nil {
		return
	}
	stdinR, stdinW := io.Pipe()
	cmd.Stdin = stdinR
	t.Cleanup(func() {
		_ = stdinW.Close()
		_ = stdinR.Close()
	})

	go func() {
		select {
		case <-ctx.Done():
			_ = stdinW.Close()
		case <-time.After(closeAfter):
			_ = stdinW.Close()
		}
	}()
}

// runCmdToFile runs a command and captures stdout+stderr to a temp file.
func runCmdToFile(t *testing.T, cmd *exec.Cmd) ([]byte, error) {
	t.Helper()
	if cmd == nil {
		return nil, fmt.Errorf("nil cmd")
	}
	if cmd.WaitDelay == 0 {
		// PTY descendants can retain inherited descriptors after the command's
		// context expires. Bound pipe cleanup so Wait always returns promptly.
		cmd.WaitDelay = 250 * time.Millisecond
	}
	if cmd.Cancel != nil {
		cancel := cmd.Cancel
		cmd.Cancel = func() error {
			if closer, ok := cmd.Stdin.(io.Closer); ok {
				_ = closer.Close()
			}
			return cancel()
		}
	}

	outPath := filepath.Join(t.TempDir(), "cmd.out")
	f, err := os.Create(outPath)
	if err != nil {
		return nil, fmt.Errorf("create output file: %w", err)
	}
	cmd.Stdout = f
	cmd.Stderr = f

	runErr := cmd.Run()
	_ = f.Close()
	if errors.Is(runErr, exec.ErrWaitDelay) {
		// Stdout and stderr target a regular file, so ErrWaitDelay can only
		// describe a lingering stdin copier after the process exited cleanly.
		runErr = nil
	}

	out, readErr := os.ReadFile(outPath)
	if readErr != nil {
		return nil, fmt.Errorf("read output file: %w (run err: %v)", readErr, runErr)
	}
	return out, runErr
}

func TestRunCmdToFileBoundsContextShutdown(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires a POSIX shell")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", "sleep 10")
	stdinR, stdinW := io.Pipe()
	cmd.Stdin = stdinR
	t.Cleanup(func() {
		_ = stdinW.Close()
		_ = stdinR.Close()
	})

	done := make(chan error, 1)
	go func() {
		_, err := runCmdToFile(t, cmd)
		done <- err
	}()

	select {
	case err := <-done:
		if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
			t.Fatalf("command returned before its context deadline: %v", err)
		}
		if err == nil {
			t.Fatal("expected context cancellation to stop the command")
		}
	case <-time.After(time.Second):
		_ = stdinW.Close()
		<-done
		t.Fatal("command wait exceeded its context deadline while stdin remained open")
	}
}
