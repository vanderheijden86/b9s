package ui

import (
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// BdOperation represents the type of bd operation performed
type BdOperation int

const (
	BdOpUpdate BdOperation = iota
	BdOpCreate
	BdOpClose
	BdOpSetStatus
	BdOpSetPriority
)

// BdResultMsg is returned after a bd CLI operation completes
type BdResultMsg struct {
	Operation BdOperation
	IssueID   string
	Success   bool
	Error     error
	Output    string
}

// IssueWriter wraps the bd CLI for mutating issues
type IssueWriter struct {
	bdPath    string
	available bool
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

// runBdCmd executes a bd command asynchronously and returns the result
func (w *IssueWriter) runBdCmd(op BdOperation, issueID string, args []string) tea.Cmd {
	bdPath := w.bdPath
	return func() tea.Msg {
		cmd := exec.Command(bdPath, args...)
		output, err := cmd.CombinedOutput()
		outStr := strings.TrimSpace(string(output))

		if err != nil {
			return BdResultMsg{
				Operation: op,
				IssueID:   issueID,
				Success:   false,
				Error:     fmt.Errorf("%s: %w", outStr, err),
				Output:    outStr,
			}
		}

		// For create operations, try to extract the new issue ID from output
		if op == BdOpCreate && issueID == "" {
			issueID = extractCreatedID(outStr)
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
