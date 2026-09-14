package ui

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanderheijden86/beadwork/pkg/debug"
)

// BdOperation represents the type of bd operation performed
type BdOperation int

const (
	BdOpUpdate BdOperation = iota
	BdOpCreate
	BdOpClose
	BdOpDelete
	BdOpSetStatus
	BdOpSetPriority
	BdOpDefer // bd-j7mx
)

// BdResultMsg is returned after a bd CLI operation completes
type BdResultMsg struct {
	Operation BdOperation
	IssueID   string
	Success   bool
	Error     error
	Output    string
	// IssueIDs lists every issue a batch operation targeted; nil for
	// single-issue operations.
	IssueIDs []string
}

// IssueWriter wraps the bd CLI for mutating issues
type IssueWriter struct {
	bdPath    string
	available bool
	checkout  Checkout // Where bd runs; none makes every write refuse
}

// NewIssueWriter creates a new IssueWriter, detecting bd availability
func NewIssueWriter() *IssueWriter {
	path, err := exec.LookPath("bd")
	if err != nil {
		return &IssueWriter{available: false}
	}
	return &IssueWriter{bdPath: path, available: true}
}

// IsAvailable returns whether the bd CLI was found
func (w *IssueWriter) IsAvailable() bool {
	return w.available
}

// SetCheckout sets the checkout bd commands run in. The zero Checkout makes
// the project read-only.
func (w *IssueWriter) SetCheckout(checkout Checkout) {
	w.checkout = checkout
}

// UpdateIssue runs bd update <id> with the given field values
func (w *IssueWriter) UpdateIssue(id string, fields map[string]string) tea.Cmd {
	if !w.available {
		return w.unavailableCmd(BdOpUpdate, id)
	}
	args := w.buildUpdateArgs(id, fields)
	return w.runBdCmd(BdOpUpdate, id, args)
}

// CreateIssue runs bd create with the given field values
func (w *IssueWriter) CreateIssue(fields map[string]string) tea.Cmd {
	if !w.available {
		return w.unavailableCmd(BdOpCreate, "")
	}
	args := w.buildCreateArgs(fields)
	return w.runBdCmd(BdOpCreate, "", args)
}

// CloseIssue runs bd close <id> with optional reason
func (w *IssueWriter) CloseIssue(id, reason string) tea.Cmd {
	if !w.available {
		return w.unavailableCmd(BdOpClose, id)
	}
	args := w.buildCloseArgs(id, reason)
	return w.runBdCmd(BdOpClose, id, args)
}

// DeleteIssue runs bd delete <id> --force
func (w *IssueWriter) DeleteIssue(id string) tea.Cmd {
	if !w.available {
		return w.unavailableCmd(BdOpDelete, id)
	}
	args := []string{"delete", id, "--force"}
	return w.runBdCmd(BdOpDelete, id, args)
}

// CloseIssues closes every id with a single bd close invocation.
func (w *IssueWriter) CloseIssues(ids []string) tea.Cmd {
	return w.runBatch(BdOpClose, ids, append([]string{"close"}, ids...))
}

// DeleteIssues deletes every id with a single bd delete --force invocation.
func (w *IssueWriter) DeleteIssues(ids []string) tea.Cmd {
	args := append([]string{"delete"}, ids...)
	return w.runBatch(BdOpDelete, ids, append(args, "--force"))
}

// runBatch runs one bd command over several issues and reports all of them in
// the result's IssueIDs.
func (w *IssueWriter) runBatch(op BdOperation, ids []string, args []string) tea.Cmd {
	ids = append([]string(nil), ids...)
	label := strings.Join(ids, " ")
	cmd := w.unavailableCmd(op, label)
	if w.available {
		cmd = w.runBdCmd(op, label, args)
	}
	return func() tea.Msg {
		msg := cmd()
		if result, ok := msg.(BdResultMsg); ok {
			result.IssueIDs = ids
			return result
		}
		return msg
	}
}

// DeferIssue runs bd defer <id> with optional --until flag (bd-j7mx).
func (w *IssueWriter) DeferIssue(id, until string) tea.Cmd {
	if !w.available {
		return w.unavailableCmd(BdOpDefer, id)
	}
	args := []string{"defer", id}
	if until != "" {
		args = append(args, fmt.Sprintf("--until=%s", until))
	}
	return w.runBdCmd(BdOpDefer, id, args)
}

// SetStatus is a convenience wrapper for updating just the status
func (w *IssueWriter) SetStatus(id, status string) tea.Cmd {
	return w.UpdateIssue(id, map[string]string{"status": status})
}

// SetPriority is a convenience wrapper for updating just the priority
func (w *IssueWriter) SetPriority(id string, priority int) tea.Cmd {
	return w.UpdateIssue(id, map[string]string{"priority": fmt.Sprintf("%d", priority)})
}

// buildUpdateArgs constructs the argument list for bd update
func (w *IssueWriter) buildUpdateArgs(id string, fields map[string]string) []string {
	args := []string{"update", id}
	for key, val := range fields {
		args = append(args, fmt.Sprintf("--%s=%s", key, val))
	}
	return args
}

// buildCreateArgs constructs the argument list for bd create
func (w *IssueWriter) buildCreateArgs(fields map[string]string) []string {
	args := []string{"create"}
	for key, val := range fields {
		args = append(args, fmt.Sprintf("--%s=%s", key, val))
	}
	return args
}

// buildCloseArgs constructs the argument list for bd close
func (w *IssueWriter) buildCloseArgs(id, reason string) []string {
	args := []string{"close", id}
	if reason != "" {
		args = append(args, fmt.Sprintf("--reason=%s", reason))
	}
	return args
}

// runBdCmd executes a bd command asynchronously in the checkout and returns
// the result. Every write passes through here, so this is the one place that
// refuses writes for a project opened without a checkout: bd resolves the
// project from its working directory and would otherwise write elsewhere.
func (w *IssueWriter) runBdCmd(op BdOperation, issueID string, args []string) tea.Cmd {
	dir := w.checkout.Dir()
	if dir == "" {
		return w.readOnlyCmd(op, issueID)
	}
	bdPath := w.bdPath
	return func() tea.Msg {
		debug.Log("bd-cmd: exec %s %s (dir=%s)", bdPath, strings.Join(args, " "), dir)
		start := time.Now()

		cmd := exec.Command(bdPath, args...)
		cmd.Dir = dir
		output, err := cmd.CombinedOutput()
		outStr := strings.TrimSpace(string(output))
		elapsed := time.Since(start)

		if err != nil {
			debug.Log("bd-cmd: FAILED in %v: %v | output: %s", elapsed, err, outStr)
			return BdResultMsg{
				Operation: op,
				IssueID:   issueID,
				Success:   false,
				Error:     fmt.Errorf("%s: %w", outStr, err),
				Output:    outStr,
			}
		}

		debug.Log("bd-cmd: OK in %v | output: %s", elapsed, outStr)

		// For create operations, try to extract the new issue ID from output
		if op == BdOpCreate && issueID == "" {
			issueID = extractCreatedID(outStr)
			debug.Log("bd-cmd: extracted created ID: %q", issueID)
		}

		return BdResultMsg{
			Operation: op,
			IssueID:   issueID,
			Success:   true,
			Output:    outStr,
		}
	}
}

// unavailableCmd returns a command that immediately reports bd is not available
func (w *IssueWriter) unavailableCmd(op BdOperation, id string) tea.Cmd {
	return func() tea.Msg {
		return BdResultMsg{
			Operation: op,
			IssueID:   id,
			Success:   false,
			Error:     fmt.Errorf("bd CLI not found in PATH; install beads to edit issues"),
		}
	}
}

// readOnlyCmd reports that the active project has no checkout to write through.
func (w *IssueWriter) readOnlyCmd(op BdOperation, id string) tea.Cmd {
	return func() tea.Msg {
		return BdResultMsg{
			Operation: op,
			IssueID:   id,
			Success:   false,
			Error:     fmt.Errorf("read-only: no local checkout for this project"),
		}
	}
}

// extractCreatedID parses bd create output to find the new issue ID
// Expected format: "Created issue: bd-xxx" or similar
func extractCreatedID(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		// Look for "Created issue: <id>" pattern
		if strings.Contains(line, "Created issue:") {
			parts := strings.SplitAfter(line, "Created issue:")
			if len(parts) > 1 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}
