package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanderheijden86/b9s/internal/bdrun"
	"github.com/vanderheijden86/b9s/pkg/debug"
)

// bdRunTimeout bounds a single bd invocation from IssueWriter (bd-t8j5.20): a
// hung bd must fail the write rather than leave the TUI's footer busy
// forever. attach_add.go's attachAddTimeout plays the same role for uploads,
// which can legitimately take longer than a plain bd write. A var, not a
// const, so a test can shrink it rather than waiting out the real deadline.
var bdRunTimeout = 60 * time.Second

// batchPerIDBudget extends a batch invocation's timeout beyond bdRunTimeout
// by this much per id: bd's one process call covers every id in the batch,
// so a batch of N takes longer than a single-id write in the ordinary case,
// and bdRunTimeout alone would flag a merely-large batch as hung. A var, not
// a const, so a test can shrink it rather than waiting out the real budget.
var batchPerIDBudget = 2 * time.Second

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
	BdOpComment
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
	opening   string   // Project being switched to; writes wait until it has opened
}

// NewIssueWriter creates a new IssueWriter, detecting bd availability
func NewIssueWriter() *IssueWriter {
	path, ok := bdrun.Resolve()
	if !ok {
		return &IssueWriter{available: false}
	}
	return &IssueWriter{bdPath: path, available: true}
}

// IsAvailable returns whether the bd CLI was found
func (w *IssueWriter) IsAvailable() bool {
	return w.available
}

// Dir is the checkout bd runs in, or "" when writes are refused.
func (w *IssueWriter) Dir() string {
	if w == nil {
		return ""
	}
	return w.checkout.Dir()
}

// SetCheckout sets the checkout bd commands run in. The zero Checkout makes
// the project read-only.
func (w *IssueWriter) SetCheckout(checkout Checkout) {
	w.checkout = checkout
}

// SetOpening refuses writes while project is being opened, because the
// checkout still belongs to the project on screen and the user may think the
// edit lands in the new one. An empty name allows writes again.
func (w *IssueWriter) SetOpening(project string) {
	w.opening = project
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
// the result's IssueIDs. The timeout scales with the batch (bdRunTimeout plus
// batchPerIDBudget per id): bd's one invocation covers every id in the same
// call, so a timeout partway through leaves no way to tell which ids it
// reached before the deadline, and the result's error says so.
func (w *IssueWriter) runBatch(op BdOperation, ids []string, args []string) tea.Cmd {
	ids = append([]string(nil), ids...)
	label := strings.Join(ids, " ")
	cmd := w.unavailableCmd(op, label)
	if w.available {
		budget := bdRunTimeout + batchPerIDBudget*time.Duration(len(ids))
		cmd = w.runBdCmdWithTimeout(op, label, args, budget)
	}
	return func() tea.Msg {
		msg := cmd()
		result, ok := msg.(BdResultMsg)
		if !ok {
			return msg
		}
		result.IssueIDs = ids
		if result.Error != nil && errors.Is(result.Error, bdrun.ErrTimeout) {
			result.Error = fmt.Errorf("%w; the batch may be partly written", result.Error)
		}
		return result
	}
}

// AddComment runs bd comments add <id> -- <text>. The separator keeps text
// that starts with a dash from being read as a flag.
func (w *IssueWriter) AddComment(id, text string) tea.Cmd {
	if !w.available {
		return w.unavailableCmd(BdOpComment, id)
	}
	return w.runBdCmd(BdOpComment, id, []string{"comments", "add", id, "--", text})
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

// SetStatuses sets status on every id with a single bd update invocation.
func (w *IssueWriter) SetStatuses(ids []string, status string) tea.Cmd {
	args := append([]string{"update"}, ids...)
	return w.runBatch(BdOpSetStatus, ids, append(args, "--status="+status))
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
// the result, bound to bdRunTimeout. Every write passes through here or
// through runBatch, so this is the one place that refuses writes for a
// project opened without a checkout: bd resolves the project from its
// working directory and would otherwise write elsewhere.
func (w *IssueWriter) runBdCmd(op BdOperation, issueID string, args []string) tea.Cmd {
	return w.runBdCmdWithTimeout(op, issueID, args, bdRunTimeout)
}

// runBdCmdWithTimeout is runBdCmd with an explicit timeout, so runBatch can
// give a multi-id invocation more time than a single-id write without
// changing bdRunTimeout itself.
func (w *IssueWriter) runBdCmdWithTimeout(op BdOperation, issueID string, args []string, timeout time.Duration) tea.Cmd {
	if w.opening != "" {
		return w.openingCmd(op, issueID)
	}
	dir := w.checkout.Dir()
	if dir == "" {
		return w.readOnlyCmd(op, issueID)
	}
	bdPath := w.bdPath
	return func() tea.Msg {
		debug.Log("bd-cmd: exec %s %s (dir=%s)", bdPath, strings.Join(args, " "), dir)
		start := time.Now()

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		outStr, err := bdrun.Run(ctx, bdPath, dir, args...)
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

// openingCmd reports that writes wait for the project switch to settle.
func (w *IssueWriter) openingCmd(op BdOperation, id string) tea.Cmd {
	project := w.opening
	return func() tea.Msg {
		return BdResultMsg{
			Operation: op,
			IssueID:   id,
			Success:   false,
			Error:     fmt.Errorf("edits wait until %s has opened", project),
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
		// bd prints "Created issue: <id>", and from 1.3 on appends " — <title>".
		if _, rest, ok := strings.Cut(line, "Created issue:"); ok {
			if fields := strings.Fields(rest); len(fields) > 0 {
				return fields[0]
			}
		}
	}
	return ""
}
