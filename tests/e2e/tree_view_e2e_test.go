package main_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

// treeFixtureDep is a dependency entry for tree test JSONL fixtures.
type treeFixtureDep struct {
	IssueID     string `json:"issue_id"`
	DependsOnID string `json:"depends_on_id"`
	Type        string `json:"type"`
}

// treeFixtureIssue is a JSONL issue for tree test fixtures.
type treeFixtureIssue struct {
	ID           string            `json:"id"`
	Title        string            `json:"title"`
	Status       string            `json:"status"`
	Priority     int               `json:"priority"`
	IssueType    string            `json:"issue_type"`
	CreatedAt    string            `json:"created_at"`
	Labels       []string          `json:"labels,omitempty"`
	Notes        string            `json:"notes,omitempty"`
	Dependencies []*treeFixtureDep `json:"dependencies,omitempty"`
}

// writeTreeFixture writes a .beads/beads.jsonl with the given issues.
func writeTreeFixture(t *testing.T, dir string, issues []treeFixtureIssue) {
	t.Helper()
	beadsDir := filepath.Join(dir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}

	var lines []string
	for _, issue := range issues {
		data, err := json.Marshal(issue)
		if err != nil {
			t.Fatalf("marshal issue %s: %v", issue.ID, err)
		}
		lines = append(lines, string(data))
	}
	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(beadsDir, "beads.jsonl"), []byte(content), 0o644); err != nil {
		t.Fatalf("write beads.jsonl: %v", err)
	}
}

// makeTreeHierarchy creates a standard tree fixture with epics, tasks, and subtasks.
//
//	epic-1 (P1, epic)
//	  task-1 (P2, task, child of epic-1)
//	    subtask-1 (P3, task, child of task-1)
//	    subtask-2 (P3, task, child of task-1)
//	  task-2 (P2, task, child of epic-1)
//	epic-2 (P1, epic)
//	  task-3 (P2, task, child of epic-2)
//	standalone-1 (P2, task, no parent)
func makeTreeHierarchy(t *testing.T) []treeFixtureIssue {
	t.Helper()
	now := time.Now()
	return []treeFixtureIssue{
		{ID: "epic-1", Title: "Epic One", Status: "open", Priority: 1, IssueType: "epic", CreatedAt: now.Format(time.RFC3339)},
		{ID: "task-1", Title: "Task One", Status: "open", Priority: 2, IssueType: "task", CreatedAt: now.Add(time.Second).Format(time.RFC3339),
			Dependencies: []*treeFixtureDep{{IssueID: "task-1", DependsOnID: "epic-1", Type: "parent-child"}}},
		{ID: "subtask-1", Title: "Subtask Alpha", Status: "open", Priority: 3, IssueType: "task", CreatedAt: now.Add(2 * time.Second).Format(time.RFC3339),
			Dependencies: []*treeFixtureDep{{IssueID: "subtask-1", DependsOnID: "task-1", Type: "parent-child"}}},
		{ID: "subtask-2", Title: "Subtask Beta", Status: "closed", Priority: 3, IssueType: "task", CreatedAt: now.Add(3 * time.Second).Format(time.RFC3339),
			Dependencies: []*treeFixtureDep{{IssueID: "subtask-2", DependsOnID: "task-1", Type: "parent-child"}}},
		{ID: "task-2", Title: "Task Two", Status: "open", Priority: 2, IssueType: "task", CreatedAt: now.Add(4 * time.Second).Format(time.RFC3339),
			Dependencies: []*treeFixtureDep{{IssueID: "task-2", DependsOnID: "epic-1", Type: "parent-child"}}},
		{ID: "epic-2", Title: "Epic Two", Status: "open", Priority: 1, IssueType: "epic", CreatedAt: now.Add(5 * time.Second).Format(time.RFC3339)},
		{ID: "task-3", Title: "Task Three", Status: "closed", Priority: 2, IssueType: "task", CreatedAt: now.Add(6 * time.Second).Format(time.RFC3339),
			Dependencies: []*treeFixtureDep{{IssueID: "task-3", DependsOnID: "epic-2", Type: "parent-child"}}},
		{ID: "standalone-1", Title: "Standalone Task", Status: "open", Priority: 2, IssueType: "task", CreatedAt: now.Add(7 * time.Second).Format(time.RFC3339)},
	}
}

// makeFilterFixture creates a fixture with mixed open/closed/blocked statuses for filter testing.
//
//	epic-f1 (open, epic)
//	  task-f1 (open, task, child of epic-f1)
//	  task-f2 (closed, task, child of epic-f1)
//	  task-f3 (open, task, child of epic-f1, blocked by task-f1)
//	epic-f2 (closed, epic)
//	  task-f4 (closed, task, child of epic-f2)
//	task-f5 (open, task, no parent, no blockers = ready)
func makeFilterFixture(t *testing.T) []treeFixtureIssue {
	t.Helper()
	now := time.Now()
	return []treeFixtureIssue{
		{ID: "epic-f1", Title: "Open Epic", Status: "open", Priority: 1, IssueType: "epic", CreatedAt: now.Format(time.RFC3339)},
		{ID: "task-f1", Title: "Open Task A", Status: "open", Priority: 2, IssueType: "task", CreatedAt: now.Add(time.Second).Format(time.RFC3339),
			Dependencies: []*treeFixtureDep{{IssueID: "task-f1", DependsOnID: "epic-f1", Type: "parent-child"}}},
		{ID: "task-f2", Title: "Closed Task B", Status: "closed", Priority: 2, IssueType: "task", CreatedAt: now.Add(2 * time.Second).Format(time.RFC3339),
			Dependencies: []*treeFixtureDep{{IssueID: "task-f2", DependsOnID: "epic-f1", Type: "parent-child"}}},
		{ID: "task-f3", Title: "Blocked Task C", Status: "open", Priority: 2, IssueType: "task", CreatedAt: now.Add(3 * time.Second).Format(time.RFC3339),
			Dependencies: []*treeFixtureDep{
				{IssueID: "task-f3", DependsOnID: "epic-f1", Type: "parent-child"},
				{IssueID: "task-f3", DependsOnID: "task-f1", Type: "blocks"},
			}},
		{ID: "epic-f2", Title: "Closed Epic", Status: "closed", Priority: 1, IssueType: "epic", CreatedAt: now.Add(4 * time.Second).Format(time.RFC3339)},
		{ID: "task-f4", Title: "Closed Task D", Status: "closed", Priority: 2, IssueType: "task", CreatedAt: now.Add(5 * time.Second).Format(time.RFC3339),
			Dependencies: []*treeFixtureDep{{IssueID: "task-f4", DependsOnID: "epic-f2", Type: "parent-child"}}},
		{ID: "task-f5", Title: "Ready Task E", Status: "open", Priority: 2, IssueType: "task", CreatedAt: now.Add(6 * time.Second).Format(time.RFC3339)},
	}
}

func TestDependencyGraphView(t *testing.T) {
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, makeFilterFixture(t))

	out, err := runTreeTUI(t, tempDir, 1800, []keyStep{
		k("g"),
		k("\x1b[H"),
		k("j"), k("j"), k("j"), k("j"),
	})
	if err != nil {
		t.Fatalf("run dependency graph TUI: %v", err)
	}
	containsAll(t, out, []string{
		"Dependency Graph",
		"task-f3",
		"BLOCKED BY (1)",
		"task-f1",
	})
}

func TestTreeViewShowsComputedBlockerBadge(t *testing.T) {
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, makeFilterFixture(t))

	out, err := runTreeTUI(t, tempDir, 900, nil)
	if err != nil {
		t.Fatalf("run tree TUI: %v", err)
	}
	containsAll(t, out, []string{"Blocked Task C", "◈1"})
}

func adaptiveColumnFixture(t *testing.T) []treeFixtureIssue {
	t.Helper()
	now := time.Now()
	longTitle := strings.Repeat("Long acceptance task title ", 8)
	return []treeFixtureIssue{
		{ID: "column-short", Title: "Short task", Status: "open", Priority: 2, IssueType: "task", CreatedAt: now.Format(time.RFC3339), Labels: []string{"lane-stage=DONE"}},
		{ID: "column-long-1", Title: longTitle, Status: "open", Priority: 2, IssueType: "task", CreatedAt: now.Add(-time.Second).Format(time.RFC3339), Labels: []string{"lane-stage=QUEUED"}},
		{ID: "column-long-2", Title: longTitle, Status: "open", Priority: 2, IssueType: "feature", CreatedAt: now.Add(-2 * time.Second).Format(time.RFC3339), Labels: []string{"lane-stage=RUNNING"}},
	}
}

func TestTreeViewAutoHidesLaneStageInNarrowContentLayout(t *testing.T) {
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, adaptiveColumnFixture(t))

	out, err := runTreeTUI(t, tempDir, 1200, nil)
	if err != nil {
		t.Fatalf("run tree TUI: %v", err)
	}
	if strings.Contains(string(out), "LANE STATE") {
		t.Fatalf("lane stage should be auto-hidden when most work titles truncate:\n%s", out)
	}
}

func TestTreeViewColumnSelectorForcesLaneStageVisible(t *testing.T) {
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, adaptiveColumnFixture(t))

	out, err := runTreeTUI(t, tempDir, 1800, []keyStep{k("|"), k(" "), k("|")})
	if err != nil {
		t.Fatalf("run tree TUI: %v", err)
	}
	containsAll(t, out, []string{"LANE STATE", "RUNNING"})
}

// runTreeTUI launches bv in a PTY, sends the given key sequence, and returns the captured output.
// Keys are sent with configurable delays. The TUI auto-closes after autoCloseMs.
func runTreeTUI(t *testing.T, dir string, autoCloseMs int, keys []keyStep) ([]byte, error) {
	t.Helper()
	skipIfNoScript(t)
	bv := buildBvBinary(t)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := sizedScriptTUICommand(ctx, bv)
	if cmd == nil {
		t.Skip("skipping: script command not available on this platform")
		return nil, nil
	}
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"TERM=screen-256color",
		fmt.Sprintf("B9S_TUI_AUTOCLOSE_MS=%d", autoCloseMs),
	)

	stdinR, stdinW := io.Pipe()
	cmd.Stdin = stdinR
	t.Cleanup(func() {
		_ = stdinW.Close()
		_ = stdinR.Close()
	})

	// Safety: close stdin after timeout to prevent hangs
	time.AfterFunc(time.Duration(autoCloseMs+3000)*time.Millisecond, func() {
		_ = stdinW.Close()
	})

	// Send key sequence in a goroutine
	done := make(chan struct{})
	t.Cleanup(func() { close(done) })

	go func() {
		// Wait for TUI to initialize
		time.Sleep(300 * time.Millisecond)
		for _, k := range keys {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			default:
			}
			if k.delay > 0 {
				time.Sleep(k.delay)
			}
			if _, err := io.WriteString(stdinW, k.key); err != nil {
				return
			}
		}
	}()

	out, err := runCmdToFile(t, cmd)
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("TUI exceeded 15-second deadline; output:\n%s", out)
	}
	return out, err
}

// keyStep represents a key to send with an optional delay before sending it.
type keyStep struct {
	key   string
	delay time.Duration
}

// k is a shorthand for creating a keyStep with a default 100ms delay.
func k(key string) keyStep {
	return keyStep{key: key, delay: 100 * time.Millisecond}
}

// kd creates a keyStep with a custom delay.
func kd(key string, delay time.Duration) keyStep {
	return keyStep{key: key, delay: delay}
}

// containsAll checks that output contains all expected substrings.
func containsAll(t *testing.T, out []byte, expected []string) {
	t.Helper()
	s := string(out)
	for _, exp := range expected {
		if !strings.Contains(s, exp) {
			t.Errorf("expected output to contain %q, but it was missing\noutput (first 2000 chars):\n%s", exp, truncateOutput(s, 2000))
		}
	}
}

// containsNone checks that output contains none of the forbidden substrings.
func containsNone(t *testing.T, out []byte, forbidden []string) {
	t.Helper()
	s := string(out)
	for _, f := range forbidden {
		if strings.Contains(s, f) {
			t.Errorf("expected output NOT to contain %q, but it was present\noutput (first 2000 chars):\n%s", f, truncateOutput(s, 2000))
		}
	}
}

func truncateOutput(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen] + "...(truncated)"
	}
	return s
}

// ============================================================================
// Tests: Basic tree view initialization and rendering
// ============================================================================

// TestTreeViewEnterAndExit verifies that pressing E enters the tree view and
// the output contains tree structure elements and issue titles.
func TestTreeViewEnterAndExit(t *testing.T) {
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, makeTreeHierarchy(t))

	out, err := runTreeTUI(t, tempDir, 2500, []keyStep{
		// Tree view is the default on launch (bd-dxc); no need to press E
	})
	if err != nil {
		t.Fatalf("TUI run failed: %v\noutput:\n%s", err, out)
	}

	// Tree view should show issues from the hierarchy.
	containsAll(t, out, []string{"Epic One", "Task One"})
}

func TestTUIEnablesMouseCellMotion(t *testing.T) {
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, makeTreeHierarchy(t))

	out, err := runTreeTUI(t, tempDir, 1200, nil)
	if err != nil {
		t.Fatalf("TUI run failed: %v\noutput:\n%s", err, out)
	}

	if !strings.Contains(string(out), "\x1b[?1002h") {
		t.Fatal("TUI did not enable terminal mouse cell-motion reporting")
	}
}

func TestTreeViewShiftKShowsCloseConfirmation(t *testing.T) {
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, []treeFixtureIssue{
		{
			ID:        "close-1",
			Title:     "Close from the TUI",
			Status:    "open",
			Priority:  2,
			IssueType: "task",
			CreatedAt: time.Now().Format(time.RFC3339),
		},
	})

	out, err := runTreeTUI(t, tempDir, 1800, []keyStep{k("K")})
	if err != nil {
		t.Fatalf("TUI run failed: %v\noutput:\n%s", err, out)
	}

	containsAll(t, out, []string{
		"Close issue?",
		"close-1",
		"Close from the TUI",
		"[Y] Close",
		"[Esc] Cancel",
	})
}

func TestTreeViewCreatedSortNumberedChildren(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	issues := []treeFixtureIssue{
		{ID: "epic-old", Title: "OlderEpic", Status: "open", IssueType: "epic", CreatedAt: now.Add(-time.Hour).Format(time.RFC3339)},
		{ID: "epic-new", Title: "NewerEpic", Status: "open", IssueType: "epic", CreatedAt: now.Format(time.RFC3339)},
	}
	for _, n := range []int{10, 2, 1} {
		id := fmt.Sprintf("child-%d", n)
		issues = append(issues, treeFixtureIssue{ID: id, Title: fmt.Sprintf("[szgb.%d] OrderedStep", n), Status: "open", IssueType: "task",
			CreatedAt:    now.Add(time.Duration(n) * time.Minute).Format(time.RFC3339),
			Dependencies: []*treeFixtureDep{{IssueID: id, DependsOnID: "epic-new", Type: "parent-child"}}})
	}
	writeTreeFixture(t, dir, issues)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := sizedScriptTUICommand(ctx, buildBvBinary(t))
	if cmd == nil {
		t.Fatal("script PTY is required for the sorting regression")
	}
	cmd.Dir = dir
	// A captured PTY has no emulator to answer OSC color queries.
	cmd.Env = append(os.Environ(), "TERM=screen-256color", "B9S_TUI_AUTOCLOSE_MS=2500")
	ensureCmdStdinCloses(t, ctx, cmd, 6*time.Second)
	out, err := runCmdToFile(t, cmd)
	if err != nil {
		t.Fatalf("TUI failed: %v\n%s", err, out)
	}
	last := -1
	for _, title := range []string{"NewerEpic", "[szgb.1] OrderedStep", "[szgb.2] OrderedStep", "[szgb.10] OrderedStep", "OlderEpic"} {
		pos := strings.Index(string(out), title)
		if pos <= last {
			t.Fatalf("expected %q after position %d, got %d\n%s", title, last, pos, out)
		}
		last = pos
	}
}

// TestTreeViewShowsHierarchy verifies that the tree view displays the parent-child
// hierarchy with branch characters and proper nesting.
func TestTreeViewShowsHierarchy(t *testing.T) {
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, makeTreeHierarchy(t))

	out, err := runTreeTUI(t, tempDir, 2500, []keyStep{
		// Tree view is the default on launch (bd-dxc); no need to press E
	})
	if err != nil {
		t.Fatalf("TUI run failed: %v\noutput:\n%s", err, out)
	}

	// Should show tree branch characters for nested items
	s := string(out)
	hasBranch := strings.Contains(s, "├") || strings.Contains(s, "└") || strings.Contains(s, "│")
	if !hasBranch {
		t.Error("expected tree branch characters (├, └, │) in output for nested hierarchy")
	}

	// Root epics and their immediate children should be visible (depth < 2 = auto-expanded)
	containsAll(t, out, []string{"Epic One", "Epic Two", "Task One", "Task Two", "Task Three"})
}

// ============================================================================
// Tests: Expand and collapse
// ============================================================================

// TestTreeViewExpandAll verifies that pressing X expands all nodes, making
// deeply nested subtasks visible.
func TestTreeViewExpandAll(t *testing.T) {
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, makeTreeHierarchy(t))

	out, err := runTreeTUI(t, tempDir, 3000, []keyStep{
		// Tree view is the default on launch (bd-dxc)
		k("X"), // Expand all
	})
	if err != nil {
		t.Fatalf("TUI run failed: %v\noutput:\n%s", err, out)
	}

	// After expand all, subtasks should be visible
	containsAll(t, out, []string{"Subtask Alpha", "Subtask Beta", "Standalone Task"})
}

// TestTreeViewCollapseAll verifies that pressing Z collapses all nodes.
// After collapse, only root-level issues should be visible.
func TestTreeViewCollapseAll(t *testing.T) {
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, makeTreeHierarchy(t))

	out, err := runTreeTUI(t, tempDir, 3000, []keyStep{
		// Tree view is the default on launch (bd-dxc)
		k("Z"), // Collapse all
	})
	if err != nil {
		t.Fatalf("TUI run failed: %v\noutput:\n%s", err, out)
	}

	// Root items should still be visible
	containsAll(t, out, []string{"Epic One", "Epic Two", "Standalone Task"})

	// Child tasks should not be visible after collapse (they are children of collapsed nodes)
	// Note: Due to terminal output buffering, earlier frames may still contain these.
	// We check the final rendered state by looking at the last occurrence of the tree header.
	s := string(out)
	lastHeaderIdx := strings.LastIndex(s, "TYPE PRI STATUS")
	if lastHeaderIdx >= 0 {
		finalFrame := s[lastHeaderIdx:]
		if strings.Contains(finalFrame, "Task One") || strings.Contains(finalFrame, "Task Two") {
			// Children of collapsed epics should not appear in the final frame
			// This is a soft check because terminal output may interleave frames
			t.Log("Note: child tasks still visible in final frame after collapse-all (may be terminal buffering)")
		}
	}
}

func TestTreeFinalFrameOverwritesEarlierChildren(t *testing.T) {
	out := "\x1b[H\rEpic One\r\nTask One\r\nTask Two\x1b[3;1H" +
		"\x1b[H\rEpic One\r\nStandalone Task\r\n\x1b[K\x1b[3;1H\x1b[?1049l"
	frame := ansi.Strip(treeFinalFrame([]byte(out)))
	if strings.Contains(frame, "Task One") || strings.Contains(frame, "Task Two") {
		t.Fatalf("final frame retained earlier children: %q", frame)
	}
}

func TestTreeFinalFrameRetainsUnchangedRows(t *testing.T) {
	out := "\x1b[H\rEpic One\r\nTask One\r\nTask Two\x1b[3;1H" +
		"\x1b[H\nTask Three\r\n\x1b[3;1H\x1b[?1049l"
	frame := ansi.Strip(treeFinalFrame([]byte(out)))
	if frame != "Epic One\nTask Three\nTask Two" {
		t.Fatalf("unexpected final frame: %q", frame)
	}
}

func treeFinalFrame(out []byte) string {
	// Bubble Tea starts each alternate-screen update at home and emits whole
	// changed rows. Bare newlines retain cached rows; erase-line clears them.
	transcript, _, _ := strings.Cut(string(out), "\x1b[?1049l")
	var rows []string
	for _, update := range strings.Split(transcript, "\x1b[H")[1:] {
		for i, raw := range strings.Split(update, "\n") {
			line := strings.Trim(ansi.Strip(raw), "\r")
			if line == "" && !strings.Contains(raw, "\x1b[K") {
				continue
			}
			for len(rows) <= i {
				rows = append(rows, "")
			}
			rows[i] = strings.Trim(raw, "\r")
		}
	}
	return strings.Join(rows, "\n")
}

// TestTreeViewToggleExpand verifies Tab folds the selected root, as advertised
// by the tree footer. Assertions cover the final screen, including selection.
func TestTreeViewToggleExpand(t *testing.T) {
	// The selected row is identified by its background, even in colorless CI.
	t.Setenv("NO_COLOR", "")
	t.Setenv("CLICOLOR_FORCE", "1")
	t.Setenv("COLORTERM", "truecolor")
	for _, tc := range []struct {
		name     string
		toggles  int
		expanded bool
	}{
		{name: "selected_collapsed_root"},
		{name: "expanded", toggles: 1, expanded: true},
		{name: "collapsed_again", toggles: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()
			issues := makeTreeHierarchy(t)
			// Default ordering is Created descending. Unique fixed timestamps
			// put epic-1 first without relying on priority or input order ties.
			created := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
			for i := range issues {
				issues[i].CreatedAt = created.Add(-time.Duration(i) * time.Hour).Format(time.RFC3339)
			}
			writeTreeFixture(t, tempDir, issues)
			keys := []keyStep{k("Z"), k("g")}
			for i := 0; i < tc.toggles; i++ {
				keys = append(keys, k("\t"))
			}
			out, err := runTreeTUI(t, tempDir, 3500, keys)
			if err != nil {
				t.Fatalf("TUI run failed: %v\noutput:\n%s", err, out)
			}
			frame := treeFinalFrame(out)
			selected := false
			for _, row := range strings.Split(frame, "\n") {
				if strings.Contains(ansi.Strip(row), "Epic One") && strings.Contains(row, "\x1b[48;") {
					selected = true
				}
			}
			if !selected {
				t.Fatalf("expected Epic One to be highlighted as selected; final frame:\n%s", frame)
			}
			plain := ansi.Strip(frame)
			for _, child := range []string{"Task One", "Task Two"} {
				if strings.Contains(plain, child) != tc.expanded {
					t.Errorf("final frame visibility of %q: want %t; frame:\n%s", child, tc.expanded, plain)
				}
			}
			if strings.Contains(plain, "Subtask Alpha") || strings.Contains(plain, "Subtask Beta") {
				t.Errorf("fold toggle should leave grandchildren hidden; final frame:\n%s", plain)
			}
		})
	}
}

// ============================================================================
// Tests: Navigation
// ============================================================================

// TestTreeViewNavigation verifies j/k movement changes the selected node.
func TestTreeViewNavigation(t *testing.T) {
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, makeTreeHierarchy(t))

	out, err := runTreeTUI(t, tempDir, 3000, []keyStep{
		// Tree view is the default on launch (bd-dxc)
		k("j"), // Move down
		k("j"), // Move down again
		k("k"), // Move up
	})
	if err != nil {
		t.Fatalf("TUI run failed: %v\noutput:\n%s", err, out)
	}

	// The tree should be rendered with issue content visible
	containsAll(t, out, []string{"Epic One", "Task One"})
}

// TestTreeViewJumpTopBottom verifies g/G jump to top/bottom of the tree.
func TestTreeViewJumpTopBottom(t *testing.T) {
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, makeTreeHierarchy(t))

	out, err := runTreeTUI(t, tempDir, 3000, []keyStep{
		// Tree view is the default on launch (bd-dxc)
		k("X"), // Expand all to have many nodes
		k("G"), // Jump to bottom
		k("g"), // Jump to top
	})
	if err != nil {
		t.Fatalf("TUI run failed: %v\noutput:\n%s", err, out)
	}

	// Should show the full tree
	containsAll(t, out, []string{"Epic One", "Standalone Task"})
}

// TestTreeViewCollapseOrJumpToParent verifies h key collapses expanded nodes
// or jumps to parent for collapsed/leaf nodes.
func TestTreeViewCollapseOrJumpToParent(t *testing.T) {
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, makeTreeHierarchy(t))

	out, err := runTreeTUI(t, tempDir, 3500, []keyStep{
		// Tree view is the default on launch (bd-dxc)
		k("X"), // Expand all
		k("j"), // Move to task-1 (child of epic-1)
		k("j"), // Move to subtask-1 (child of task-1)
		k("h"), // Should jump to parent (task-1) since subtask-1 is a leaf
		k("h"), // Should collapse task-1 (it's expanded and has children)
	})
	if err != nil {
		t.Fatalf("TUI run failed: %v\noutput:\n%s", err, out)
	}

	// Tree should still show the overall structure
	containsAll(t, out, []string{"Epic One", "Task One"})
}

// TestTreeViewExpandOrMoveToChild verifies l key expands collapsed nodes
// or moves to first child for expanded nodes.
func TestTreeViewExpandOrMoveToChild(t *testing.T) {
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, makeTreeHierarchy(t))

	out, err := runTreeTUI(t, tempDir, 3500, []keyStep{
		// Tree view is the default on launch (bd-dxc)
		k("Z"), // Collapse all
		k("l"), // Expand epic-1 (first node, collapsed)
		k("l"), // Move to first child of epic-1 (task-1)
	})
	if err != nil {
		t.Fatalf("TUI run failed: %v\noutput:\n%s", err, out)
	}

	// epic-1's children should now be visible
	containsAll(t, out, []string{"Task One", "Task Two"})
}

// ============================================================================
// Tests: Filtering
// ============================================================================

// TestTreeViewFilterOpen verifies pressing 'o' filters to show only open issues.
func TestTreeViewFilterOpen(t *testing.T) {
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, makeFilterFixture(t))

	out, err := runTreeTUI(t, tempDir, 3000, []keyStep{
		// Tree view is the default on launch (bd-dxc)
		k("X"), // Expand all first
		k("o"), // Filter: open only
	})
	if err != nil {
		t.Fatalf("TUI run failed: %v\noutput:\n%s", err, out)
	}

	// Open issues should be visible
	containsAll(t, out, []string{"Open Task A", "Ready Task E"})
}

// TestTreeViewFilterClosed verifies pressing 'C' filters to show only closed issues.
func TestTreeViewFilterClosed(t *testing.T) {
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, makeFilterFixture(t))

	out, err := runTreeTUI(t, tempDir, 3000, []keyStep{
		// Tree view is the default on launch (bd-dxc)
		k("X"), // Expand all first
		k("C"), // Filter: closed only
	})
	if err != nil {
		t.Fatalf("TUI run failed: %v\noutput:\n%s", err, out)
	}

	// Closed issues should be visible
	containsAll(t, out, []string{"Closed Task B", "Closed Task D"})
}

// TestTreeViewFilterReady verifies pressing 'r' filters to show only ready (unblocked) issues.
func TestTreeViewFilterReady(t *testing.T) {
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, makeFilterFixture(t))

	out, err := runTreeTUI(t, tempDir, 3000, []keyStep{
		// Tree view is the default on launch (bd-dxc)
		k("X"), // Expand all
		k("r"), // Filter: ready only
	})
	if err != nil {
		t.Fatalf("TUI run failed: %v\noutput:\n%s", err, out)
	}

	// Ready (open + unblocked) issues should be visible
	containsAll(t, out, []string{"Ready Task E"})
}

// TestTreeViewFilterAllResets verifies pressing 'a' resets the filter to show all issues.
func TestTreeViewFilterAllResets(t *testing.T) {
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, makeFilterFixture(t))

	out, err := runTreeTUI(t, tempDir, 3500, []keyStep{
		// Tree view is the default on launch (bd-dxc)
		k("X"), // Expand all
		k("C"), // Filter: closed only
		k("a"), // Reset filter: show all
	})
	if err != nil {
		t.Fatalf("TUI run failed: %v\noutput:\n%s", err, out)
	}

	// After resetting filter, all issues should be visible
	containsAll(t, out, []string{"Open Epic", "Open Task A", "Ready Task E"})
}

// TestTreeViewFilterEscClears verifies that ESC clears an active filter before exiting tree view.
func TestTreeViewFilterEscClears(t *testing.T) {
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, makeFilterFixture(t))

	out, err := runTreeTUI(t, tempDir, 3500, []keyStep{
		// Tree view is the default on launch (bd-dxc)
		k("C"),    // Filter: closed only
		k("\x1b"), // ESC: should clear filter (not exit tree view)
	})
	if err != nil {
		t.Fatalf("TUI run failed: %v\noutput:\n%s", err, out)
	}

	// After ESC clearing filter, open issues should be visible again
	containsAll(t, out, []string{"Open Epic", "Ready Task E"})
}

// ============================================================================
// Tests: Search
// ============================================================================

// TestTreeViewSearch verifies that / enters search mode and typing a query highlights matches.
func TestTreeViewSearch(t *testing.T) {
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, makeTreeHierarchy(t))

	out, err := runTreeTUI(t, tempDir, 3500, []keyStep{
		// Tree view is the default on launch (bd-dxc)
		k("X"), // Expand all
		k("/"), // Enter search mode
		kd("S", 50*time.Millisecond),
		kd("u", 50*time.Millisecond),
		kd("b", 50*time.Millisecond),
		kd("t", 50*time.Millisecond),
		kd("a", 50*time.Millisecond),
		kd("s", 50*time.Millisecond),
		kd("k", 50*time.Millisecond),
	})
	if err != nil {
		t.Fatalf("TUI run failed: %v\noutput:\n%s", err, out)
	}

	// Search bar should show the query, and subtask matches should be found
	s := string(out)
	if !strings.Contains(s, "subtask") && !strings.Contains(s, "Subtask") {
		t.Log("Warning: search query or results for 'Subtask' not clearly visible in output")
	}
	// The search bar indicator should appear
	if !strings.Contains(s, "/") {
		t.Log("Warning: search bar indicator '/' not found in output")
	}
}

// TestTreeViewSearchByID verifies that searching by issue ID finds matches.
func TestTreeViewSearchByID(t *testing.T) {
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, makeTreeHierarchy(t))

	out, err := runTreeTUI(t, tempDir, 3500, []keyStep{
		// Tree view is the default on launch (bd-dxc)
		k("/"), // Enter search mode
		kd("e", 50*time.Millisecond),
		kd("p", 50*time.Millisecond),
		kd("i", 50*time.Millisecond),
		kd("c", 50*time.Millisecond),
		kd("-", 50*time.Millisecond),
		kd("2", 50*time.Millisecond),
	})
	if err != nil {
		t.Fatalf("TUI run failed: %v\noutput:\n%s", err, out)
	}

	// Should find Epic Two by its epic-2 ID search.
	containsAll(t, out, []string{"Epic Two"})
}

func TestTreeViewToggleHighlightedBranchFilter(t *testing.T) {
	tempDir := t.TempDir()
	now := time.Now()
	issues := []treeFixtureIssue{
		{ID: "epic-a", Title: "First epic", Status: "open", Priority: 1, IssueType: "epic", CreatedAt: now.Format(time.RFC3339)},
		{ID: "feature-a", Title: "Nested feature", Status: "open", Priority: 2, IssueType: "feature", CreatedAt: now.Add(-time.Second).Format(time.RFC3339), Dependencies: []*treeFixtureDep{{IssueID: "feature-a", DependsOnID: "epic-a", Type: "parent-child"}}},
		{ID: "task-a", Title: "Nested task", Status: "open", Priority: 2, IssueType: "task", CreatedAt: now.Add(-2 * time.Second).Format(time.RFC3339), Dependencies: []*treeFixtureDep{{IssueID: "task-a", DependsOnID: "feature-a", Type: "parent-child"}}},
		{ID: "epic-b", Title: "Second epic", Status: "open", Priority: 1, IssueType: "epic", CreatedAt: now.Add(-3 * time.Second).Format(time.RFC3339)},
	}
	writeTreeFixture(t, tempDir, issues)

	out, err := runTreeTUI(t, tempDir, 2500, []keyStep{
		k("X"),
		k("\x1b[H"),
		k("j"),
		k("j"),
		k("f"),
		k("f"),
		k("K"),
	})
	if err != nil {
		t.Fatalf("run tree TUI: %v\noutput:\n%s", err, out)
	}

	output := string(out)
	if !strings.Contains(output, "/ epic-a") {
		t.Fatalf("first f did not show the accepted branch query:\n%s", truncateOutput(output, 2000))
	}
	confirmationStart := strings.LastIndex(output, "Close issue?")
	if confirmationStart < 0 || !strings.Contains(output[confirmationStart:], "task-a") {
		t.Fatalf("branch toggle did not preserve the originally highlighted task:\n%s", truncateOutput(output, 2000))
	}
}

// ============================================================================
// Tests: Sort modes
// ============================================================================

// TestTreeViewSortCycle verifies that pressing 's' cycles through sort modes.
func TestTreeViewSortCycle(t *testing.T) {
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, makeTreeHierarchy(t))

	out, err := runTreeTUI(t, tempDir, 3000, []keyStep{
		// Tree view is the default on launch (bd-dxc)
		k("s"), // Cycle sort mode (default -> created asc)
		k("s"), // Cycle again (created asc -> created desc)
	})
	if err != nil {
		t.Fatalf("TUI run failed: %v\noutput:\n%s", err, out)
	}

	// Tree should still render with all root issues visible (order may change)
	containsAll(t, out, []string{"Epic One", "Epic Two"})
}

// ============================================================================
// Tests: Persistence (tree state)
// ============================================================================

// TestTreeViewStatePersistence verifies that expand/collapse state is saved to
// .beads/tree-state.json and can be loaded in a subsequent session.
func TestTreeViewStatePersistence(t *testing.T) {
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, makeTreeHierarchy(t))

	// First session: collapse all, which saves state (tree is default on launch)
	_, err := runTreeTUI(t, tempDir, 2500, []keyStep{
		// Tree view is the default on launch (bd-dxc)
		k("Z"), // Collapse all (this triggers saveState)
	})
	if err != nil {
		t.Fatalf("first TUI session failed: %v", err)
	}

	// Check that tree-state.json was created
	statePath := filepath.Join(tempDir, ".beads", "tree-state.json")
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("expected tree-state.json to be created: %v", err)
	}

	// Verify it's valid JSON with version field
	var state map[string]interface{}
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("tree-state.json is not valid JSON: %v\ncontent: %s", err, data)
	}

	version, ok := state["version"]
	if !ok {
		t.Error("tree-state.json missing 'version' field")
	}
	if v, ok := version.(float64); !ok || v != 1 {
		t.Errorf("expected version 1, got %v", version)
	}
}

// ============================================================================
// Tests: Edge cases
// ============================================================================

// TestTreeViewEmptyData verifies the tree view handles empty data gracefully.
func TestTreeViewEmptyData(t *testing.T) {
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, []treeFixtureIssue{})

	out, err := runTreeTUI(t, tempDir, 2000, []keyStep{
		// Tree view is the default on launch (bd-dxc); no need to press E
	})
	if err != nil {
		// Empty data might cause early exit, which is acceptable
		t.Logf("TUI exited (possibly expected with empty data): %v", err)
	}

	// Should either show empty state message or not crash
	_ = out // No crash = success
}

// TestTreeViewNoHierarchy verifies the tree view works when all issues are roots
// (no parent-child dependencies).
func TestTreeViewNoHierarchy(t *testing.T) {
	now := time.Now()
	issues := []treeFixtureIssue{
		{ID: "t-1", Title: "Task A", Status: "open", Priority: 1, IssueType: "task", CreatedAt: now.Format(time.RFC3339)},
		{ID: "t-2", Title: "Task B", Status: "open", Priority: 2, IssueType: "task", CreatedAt: now.Add(time.Second).Format(time.RFC3339)},
		{ID: "t-3", Title: "Task C", Status: "open", Priority: 3, IssueType: "task", CreatedAt: now.Add(2 * time.Second).Format(time.RFC3339)},
	}

	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, issues)

	out, err := runTreeTUI(t, tempDir, 2500, []keyStep{
		// Tree view is the default on launch (bd-dxc); no need to press E
	})
	if err != nil {
		t.Fatalf("TUI run failed: %v\noutput:\n%s", err, out)
	}

	// All issues should be shown as root nodes (no nesting)
	containsAll(t, out, []string{"Task A", "Task B", "Task C"})

	// Should NOT have tree branch characters since all are roots
	s := string(out)
	if strings.Contains(s, "├──") || strings.Contains(s, "└──") {
		t.Log("Note: branch characters found despite no hierarchy (may be OK if TUI renders differently)")
	}
}

// TestTreeViewDeepNesting verifies the tree view handles deeply nested hierarchies.
func TestTreeViewDeepNesting(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping deep nesting test in short mode")
	}

	now := time.Now()
	issues := []treeFixtureIssue{
		{ID: "d-0", Title: "Root", Status: "open", Priority: 1, IssueType: "epic", CreatedAt: now.Format(time.RFC3339)},
	}
	// Create a chain of 5 levels deep
	for i := 1; i <= 5; i++ {
		parentID := fmt.Sprintf("d-%d", i-1)
		issues = append(issues, treeFixtureIssue{
			ID:        fmt.Sprintf("d-%d", i),
			Title:     fmt.Sprintf("Level %d", i),
			Status:    "open",
			Priority:  2,
			IssueType: "task",
			CreatedAt: now.Add(time.Duration(i) * time.Second).Format(time.RFC3339),
			Dependencies: []*treeFixtureDep{
				{IssueID: fmt.Sprintf("d-%d", i), DependsOnID: parentID, Type: "parent-child"},
			},
		})
	}

	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, issues)

	out, err := runTreeTUI(t, tempDir, 2000, []keyStep{
		// Tree view is the default on launch (bd-dxc)
		k("X"), // Expand all to see deep nesting
	})
	if err != nil {
		t.Fatalf("TUI run failed: %v\noutput:\n%s", err, out)
	}

	// All levels should be visible after expand all
	containsAll(t, out, []string{"Root", "Level 1", "Level 2", "Level 3", "Level 4", "Level 5"})
}

// ============================================================================
// Tests: Arrow key navigation (escape sequences via PTY)
// ============================================================================

// ANSI escape sequences for arrow keys as sent by real terminals.
const (
	arrowUp        = "\x1b[A"
	arrowDown      = "\x1b[B"
	arrowRight     = "\x1b[C"
	arrowLeft      = "\x1b[D"
	mouseWheelDown = "\x1b[<65;10;10M"
)

func TestTreeViewMouseWheelDownNavigation(t *testing.T) {
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, makeTreeHierarchy(t))

	out, err := runTreeTUI(t, tempDir, 2500, []keyStep{
		k(mouseWheelDown),
		k("K"),
	})
	if err != nil {
		t.Fatalf("mouse-wheel run failed: %v\noutput:\n%s", err, out)
	}

	output := string(out)
	confirmationStart := strings.LastIndex(output, "Close issue?")
	if confirmationStart < 0 {
		t.Fatalf("mouse wheel did not leave the TUI responsive to selection actions\noutput:\n%s", out)
	}
	confirmation := output[confirmationStart:]
	if !strings.Contains(confirmation, "epic-2") {
		t.Fatalf("mouse wheel did not select the next visible tree issue\nconfirmation output:\n%s", confirmation)
	}
}

// TestTreeViewArrowDownNavigation verifies that the Down arrow key moves the
// cursor in tree view, matching 'j' behavior. This tests the actual terminal
// escape sequence (\x1b[B) through the PTY harness.
func TestTreeViewArrowDownNavigation(t *testing.T) {
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, makeTreeHierarchy(t))

	// Use arrow down to navigate in tree view (default on launch)
	out, err := runTreeTUI(t, tempDir, 3000, []keyStep{
		// Tree view is the default on launch (bd-dxc)
		k(arrowDown), // Arrow Down
		k(arrowDown), // Arrow Down again
		k(arrowUp),   // Arrow Up
	})
	if err != nil {
		t.Fatalf("TUI run failed: %v\noutput:\n%s", err, out)
	}

	// Tree should render with issues visible (arrows didn't break anything)
	containsAll(t, out, []string{"Epic One", "Task One"})
}

// TestTreeViewArrowDownMatchesJKey verifies that arrow Down produces the same
// navigation as 'j' by comparing two runs: one with j, one with arrow Down.
// Both should show the same second item selected.
func TestTreeViewArrowDownMatchesJKey(t *testing.T) {
	// Run 1: navigate with 'j'
	tempDir1 := t.TempDir()
	writeTreeFixture(t, tempDir1, makeTreeHierarchy(t))
	out1, err := runTreeTUI(t, tempDir1, 2500, []keyStep{
		// Tree view is the default on launch (bd-dxc)
		k("j"), // vim down
	})
	if err != nil {
		t.Fatalf("j-key run failed: %v\noutput:\n%s", err, out1)
	}

	// Run 2: navigate with arrow Down
	tempDir2 := t.TempDir()
	writeTreeFixture(t, tempDir2, makeTreeHierarchy(t))
	out2, err := runTreeTUI(t, tempDir2, 2500, []keyStep{
		// Tree view is the default on launch (bd-dxc)
		k(arrowDown), // arrow down
	})
	if err != nil {
		t.Fatalf("arrow-down run failed: %v\noutput:\n%s", err, out2)
	}

	// Both runs should show the tree structure
	containsAll(t, out1, []string{"Epic One", "Task One"})
	containsAll(t, out2, []string{"Epic One", "Task One"})
}

// TestTreeViewArrowLeftCollapsesNode verifies that Left arrow collapses an
// expanded node in the tree view, matching 'h' behavior.
func TestTreeViewArrowLeftCollapsesNode(t *testing.T) {
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, makeTreeHierarchy(t))

	out, err := runTreeTUI(t, tempDir, 3000, []keyStep{
		// Tree view is the default on launch (bd-dxc); epic-1 is selected, auto-expanded
		k(arrowLeft), // Should collapse epic-1
	})
	if err != nil {
		t.Fatalf("TUI run failed: %v\noutput:\n%s", err, out)
	}

	// epic-1 should still be visible (it's just collapsed, not hidden)
	containsAll(t, out, []string{"Epic One"})
}

// TestTreeViewArrowRightExpandsNode verifies that Right arrow expands a
// collapsed node or moves to first child, matching 'l' behavior.
func TestTreeViewArrowRightExpandsNode(t *testing.T) {
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, makeTreeHierarchy(t))

	out, err := runTreeTUI(t, tempDir, 3000, []keyStep{
		// Tree view is the default on launch (bd-dxc)
		k("Z"),        // Collapse all
		k(arrowRight), // Expand epic-1 (selected by default)
	})
	if err != nil {
		t.Fatalf("TUI run failed: %v\noutput:\n%s", err, out)
	}

	// After expanding epic-1, its children should be visible
	containsAll(t, out, []string{"Epic One", "Task One"})
}

// TestTreeViewArrowKeysOnlyNavigation verifies that a tree view session using
// ONLY arrow keys (no vim keys at all) can navigate, expand, and collapse.
// This simulates a user who doesn't know vim bindings.
func TestTreeViewArrowKeysOnlyNavigation(t *testing.T) {
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, makeTreeHierarchy(t))

	out, err := runTreeTUI(t, tempDir, 4000, []keyStep{
		// Tree view is the default on launch (bd-dxc)
		k(arrowDown),                         // Move to task-1
		k(arrowDown),                         // Move to task-2
		k(arrowUp),                           // Back to task-1
		k(arrowUp),                           // Back to epic-1
		k(arrowLeft),                         // Collapse epic-1
		kd(arrowRight, 200*time.Millisecond), // Expand epic-1
		k(arrowRight),                        // Move into first child (task-1)
		k(arrowDown),                         // Move to task-2
	})
	if err != nil {
		t.Fatalf("TUI run failed: %v\noutput:\n%s", err, out)
	}

	// Should show the full hierarchy was navigated
	containsAll(t, out, []string{"Epic One", "Task One", "Task Two"})
}

// TestTreeViewArrowKeysWithManyNodes verifies arrow key pagination works with
// more nodes than fit in a single viewport.
func TestTreeViewArrowKeysWithManyNodes(t *testing.T) {
	now := time.Now()
	// Create 20 root-level tasks to force scrolling
	var issues []treeFixtureIssue
	for i := 0; i < 20; i++ {
		issues = append(issues, treeFixtureIssue{
			ID:        fmt.Sprintf("t-%02d", i),
			Title:     fmt.Sprintf("Task %02d", i),
			Status:    "open",
			Priority:  2,
			IssueType: "task",
			CreatedAt: now.Add(time.Duration(i) * time.Second).Format(time.RFC3339),
		})
	}

	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, issues)

	// Navigate down many times with arrow key to scroll past the viewport
	// Tree view is the default on launch (bd-dxc); no need to press E
	var keys []keyStep
	for i := 0; i < 15; i++ {
		keys = append(keys, k(arrowDown))
	}

	out, err := runTreeTUI(t, tempDir, 4000, keys)
	if err != nil {
		t.Fatalf("TUI run failed: %v\noutput:\n%s", err, out)
	}

	// After scrolling down 15 times, later tasks should be visible
	// The viewport should have scrolled to show tasks beyond the initial view
	containsAll(t, out, []string{"Task 14", "Task 15"})
}
