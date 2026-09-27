package ui

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewIssueWriter_DetectsAvailability(t *testing.T) {
	// "bd" should be available in the test environment (installed on this machine)
	w := NewIssueWriter()
	// We can't guarantee bd is installed in CI, so just test the struct is created
	if w == nil {
		t.Fatal("NewIssueWriter returned nil")
	}
}

func TestIssueWriter_BuildUpdateArgs(t *testing.T) {
	w := &IssueWriter{bdPath: "/usr/local/bin/bd", available: true}

	fields := map[string]string{
		"title":    "New Title",
		"status":   "in_progress",
		"priority": "1",
	}

	args := w.buildUpdateArgs("bd-123", fields)

	// Should start with "update bd-123"
	if len(args) < 2 {
		t.Fatalf("expected at least 2 args, got %d", len(args))
	}
	if args[0] != "update" {
		t.Errorf("expected first arg 'update', got %q", args[0])
	}
	if args[1] != "bd-123" {
		t.Errorf("expected second arg 'bd-123', got %q", args[1])
	}

	// Should contain --title, --status, --priority flags
	argStr := joinArgs(args)
	for _, flag := range []string{"--title=New Title", "--status=in_progress", "--priority=1"} {
		if !containsArg(args, flag) {
			t.Errorf("expected args to contain %q, got: %s", flag, argStr)
		}
	}
}

func TestIssueWriter_BuildCreateArgs(t *testing.T) {
	w := &IssueWriter{bdPath: "/usr/local/bin/bd", available: true}

	fields := map[string]string{
		"title":    "My New Issue",
		"type":     "task",
		"priority": "2",
	}

	args := w.buildCreateArgs(fields)

	if len(args) < 1 {
		t.Fatalf("expected at least 1 arg, got %d", len(args))
	}
	if args[0] != "create" {
		t.Errorf("expected first arg 'create', got %q", args[0])
	}

	for _, flag := range []string{"--title=My New Issue", "--type=task", "--priority=2"} {
		if !containsArg(args, flag) {
			t.Errorf("expected args to contain %q, got: %s", flag, joinArgs(args))
		}
	}
}

func TestIssueWriter_BuildCloseArgs(t *testing.T) {
	w := &IssueWriter{bdPath: "/usr/local/bin/bd", available: true}

	args := w.buildCloseArgs("bd-456", "done")
	if args[0] != "close" {
		t.Errorf("expected first arg 'close', got %q", args[0])
	}
	if args[1] != "bd-456" {
		t.Errorf("expected second arg 'bd-456', got %q", args[1])
	}
	if !containsArg(args, "--reason=done") {
		t.Errorf("expected --reason=done, got: %s", joinArgs(args))
	}

	// Empty reason should omit --reason flag
	argsNoReason := w.buildCloseArgs("bd-456", "")
	if containsArg(argsNoReason, "--reason=") {
		t.Errorf("expected no --reason flag for empty reason, got: %s", joinArgs(argsNoReason))
	}
}

func TestIssueWriter_NotAvailable(t *testing.T) {
	w := &IssueWriter{bdPath: "", available: false}

	cmd := w.UpdateIssue("bd-123", map[string]string{"title": "test"})
	if cmd == nil {
		t.Fatal("expected a cmd even when unavailable")
	}

	msg := cmd()
	result, ok := msg.(BdResultMsg)
	if !ok {
		t.Fatalf("expected BdResultMsg, got %T", msg)
	}
	if result.Success {
		t.Error("expected failure when bd not available")
	}
	if result.Error == nil {
		t.Error("expected error when bd not available")
	}
}

func TestBdResultMsg_IsTeaMsg(t *testing.T) {
	// Verify BdResultMsg satisfies tea.Msg interface (compile-time check)
	var _ tea.Msg = BdResultMsg{}
}

func TestIssueWriter_SetStatusConvenience(t *testing.T) {
	w := &IssueWriter{bdPath: "/usr/local/bin/bd", available: true}

	// SetStatus should produce an UpdateIssue with status field
	cmd := w.SetStatus("bd-1", "closed")
	if cmd == nil {
		t.Fatal("expected non-nil cmd from SetStatus")
	}
}

func TestIssueWriter_SetPriorityConvenience(t *testing.T) {
	w := &IssueWriter{bdPath: "/usr/local/bin/bd", available: true}

	cmd := w.SetPriority("bd-1", 2)
	if cmd == nil {
		t.Fatal("expected non-nil cmd from SetPriority")
	}
}

func TestIssueWriter_SetCheckout(t *testing.T) {
	w := &IssueWriter{bdPath: "/usr/local/bin/bd", available: true}
	if w.checkout.Dir() != "" {
		t.Errorf("expected no checkout by default, got %q", w.checkout.Dir())
	}

	checkout := testCheckout(t)
	w.SetCheckout(checkout)
	if w.checkout.Dir() != checkout.Dir() {
		t.Errorf("expected checkout %q, got %q", checkout.Dir(), w.checkout.Dir())
	}
}

func TestIssueWriter_RunBdCmd_RunsInCheckout(t *testing.T) {
	checkout := testCheckout(t)
	// Resolve symlinks (macOS /var -> /private/var) so pwd output compares equal.
	wantDir, err := filepath.EvalSymlinks(checkout.Dir())
	if err != nil {
		t.Fatalf("failed to eval symlinks: %v", err)
	}

	// Use "pwd" as the fake "bd" command to report where the command runs
	w := &IssueWriter{bdPath: "/bin/pwd", available: true}
	w.SetCheckout(checkout)

	cmd := w.runBdCmd(BdOpCreate, "", []string{})
	msg := cmd()
	result, ok := msg.(BdResultMsg)
	if !ok {
		t.Fatalf("expected BdResultMsg, got %T", msg)
	}
	if !result.Success {
		t.Fatalf("command failed: %v", result.Error)
	}

	reportedDir, _ := filepath.EvalSymlinks(strings.TrimSpace(result.Output))
	if reportedDir != wantDir {
		t.Errorf("expected command to run in checkout %q, but ran in %q", wantDir, reportedDir)
	}
}

// TestIssueWriter_RunBdCmd_HungBdReturnsErroredResult proves runBdCmd's
// bounded ctx (bd-t8j5.20): a bd that never exits must not leave the footer
// busy forever, but return a BdResultMsg carrying the timeout error within
// bdRunTimeout plus bdrun's WaitDelay.
func TestIssueWriter_RunBdCmd_HungBdReturnsErroredResult(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake bd script needs a POSIX shell")
	}
	binDir := t.TempDir()
	script := "#!/bin/sh\nsleep 30\n"
	if err := os.WriteFile(filepath.Join(binDir, "bd"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	original := bdRunTimeout
	bdRunTimeout = 200 * time.Millisecond
	t.Cleanup(func() { bdRunTimeout = original })

	w := &IssueWriter{bdPath: filepath.Join(binDir, "bd"), available: true}
	w.SetCheckout(testCheckout(t))

	start := time.Now()
	cmd := w.runBdCmd(BdOpUpdate, "bd-1", []string{"update", "bd-1", "--status=closed"})
	msg := cmd()
	elapsed := time.Since(start)

	if elapsed > 5*time.Second {
		t.Fatalf("runBdCmd took %v, want it bounded by bdRunTimeout plus WaitDelay, not the 30s sleep", elapsed)
	}
	result, ok := msg.(BdResultMsg)
	if !ok {
		t.Fatalf("expected BdResultMsg, got %T", msg)
	}
	if result.Success {
		t.Error("expected failure for a hung bd")
	}
	if result.Error == nil || !strings.Contains(result.Error.Error(), "timed out") {
		t.Errorf("result.Error = %v, want it to name the timeout clearly", result.Error)
	}
}

// TestIssueWriter_RunBatch_TimeoutMessageWarnsOfPartialWrite proves runBatch
// scales its budget with the batch (bdRunTimeout plus batchPerIDBudget per
// id) and, when that larger budget still expires, that the result's error
// warns the batch may be partly written: bd's one invocation covers every id
// in the same call, so a timeout partway through leaves no way to tell which
// ids it reached before the deadline.
func TestIssueWriter_RunBatch_TimeoutMessageWarnsOfPartialWrite(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake bd script needs a POSIX shell")
	}
	binDir := t.TempDir()
	script := "#!/bin/sh\nsleep 30\n"
	if err := os.WriteFile(filepath.Join(binDir, "bd"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	originalTimeout, originalPerID := bdRunTimeout, batchPerIDBudget
	bdRunTimeout = 50 * time.Millisecond
	batchPerIDBudget = 50 * time.Millisecond
	t.Cleanup(func() { bdRunTimeout, batchPerIDBudget = originalTimeout, originalPerID })

	w := &IssueWriter{bdPath: filepath.Join(binDir, "bd"), available: true}
	w.SetCheckout(testCheckout(t))

	ids := []string{"bd-1", "bd-2"}
	start := time.Now()
	cmd := w.runBatch(BdOpSetStatus, ids, append([]string{"update"}, append(append([]string(nil), ids...), "--status=closed")...))
	msg := cmd()
	elapsed := time.Since(start)

	if elapsed > 5*time.Second {
		t.Fatalf("runBatch took %v, want it bounded by bdRunTimeout+batchPerIDBudget*len(ids) plus WaitDelay", elapsed)
	}
	result, ok := msg.(BdResultMsg)
	if !ok {
		t.Fatalf("expected BdResultMsg, got %T", msg)
	}
	if result.Success {
		t.Fatal("expected failure for a hung bd")
	}
	if !strings.Contains(result.Error.Error(), "timed out") {
		t.Errorf("result.Error = %v, want it to name the timeout clearly", result.Error)
	}
	if !strings.Contains(result.Error.Error(), "the batch may be partly written") {
		t.Errorf("result.Error = %v, want it to warn the batch may be partly written", result.Error)
	}
	if len(result.IssueIDs) != len(ids) {
		t.Errorf("result.IssueIDs = %v, want %v", result.IssueIDs, ids)
	}
}

func TestIssueWriter_DeleteIssue(t *testing.T) {
	w := &IssueWriter{bdPath: "/usr/local/bin/bd", available: true}

	cmd := w.DeleteIssue("bd-789")
	if cmd == nil {
		t.Fatal("expected non-nil cmd from DeleteIssue")
	}
}

func TestIssueWriter_DeleteIssue_NotAvailable(t *testing.T) {
	w := &IssueWriter{bdPath: "", available: false}

	cmd := w.DeleteIssue("bd-789")
	msg := cmd()
	result, ok := msg.(BdResultMsg)
	if !ok {
		t.Fatalf("expected BdResultMsg, got %T", msg)
	}
	if result.Success {
		t.Error("expected failure when bd not available")
	}
	if result.Operation != BdOpDelete {
		t.Errorf("expected BdOpDelete, got %d", result.Operation)
	}
}

// Test helpers
func containsArg(args []string, target string) bool {
	for _, a := range args {
		if a == target {
			return true
		}
	}
	return false
}

func joinArgs(args []string) string {
	result := ""
	for i, a := range args {
		if i > 0 {
			result += " "
		}
		result += a
	}
	return result
}
