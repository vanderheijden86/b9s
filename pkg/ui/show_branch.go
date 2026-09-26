package ui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// ShowBranchMsg asks b9s to select an issue and limit the view to its
// top-level branch, as f does for the cursor. It arrives from outside the
// program (the control socket) as well as from the :branch command.
type ShowBranchMsg struct{ ID string }

// showBranchDeadlineMsg ends the wait for an issue that was not loaded when
// its ShowBranchMsg arrived.
type showBranchDeadlineMsg struct{ ID string }

// showBranchWait bounds how long a request waits for its issue. The caller is
// usually `bd create`, whose write reaches b9s on the next Dolt poll, so the
// id is normally missing for up to one poll interval.
const showBranchWait = 5 * time.Second

func (m Model) handleShowBranch(id string) (Model, tea.Cmd) {
	id = strings.TrimSpace(id)
	if id == "" {
		m.statusMsg = "branch needs an issue id"
		m.statusIsError = true
		return m, nil
	}
	if m.showBranch(id) {
		m.pendingBranchID = ""
		return m, nil
	}
	m.pendingBranchID = id
	m.statusMsg = "Waiting for " + id + " to load"
	m.statusIsError = false
	return m, tea.Tick(showBranchWait, func(time.Time) tea.Msg { return showBranchDeadlineMsg{ID: id} })
}

func (m Model) expireShowBranch(id string) Model {
	if m.pendingBranchID != id {
		return m
	}
	m.pendingBranchID = ""
	m.statusMsg = "No issue " + id + " to show"
	m.statusIsError = true
	return m
}

// retryPendingBranch applies a waiting request once a reload has brought its
// issue in.
func (m *Model) retryPendingBranch() {
	if m.pendingBranchID != "" && m.showBranch(m.pendingBranchID) {
		m.pendingBranchID = ""
	}
}

// resolveIssueID accepts a full id or the short form shown on screen, which
// drops the project prefix. A short form that matches several projects' issues
// resolves to none.
func (m *Model) resolveIssueID(id string) string {
	if _, ok := m.issueMap[id]; ok {
		return id
	}
	match := ""
	for full := range m.issueMap {
		if strings.HasSuffix(full, "-"+id) {
			if match != "" {
				return ""
			}
			match = full
		}
	}
	return match
}

// showBranch selects id and sets, never toggles, the branch limit in the
// visible view. It reports false when the issue is not loaded.
func (m *Model) showBranch(id string) bool {
	id = m.resolveIssueID(id)
	if id == "" {
		return false
	}
	if m.isBoardView {
		root := m.board.SetBranchFor(id)
		if root == "" || !m.board.SelectIssueByID(id) {
			return false
		}
		m.statusMsg = "Branch " + root + " · f shows the whole board"
		m.statusIsError = false
		m.syncBoardToDetail()
		return true
	}
	root, ok := m.tree.ExpandPathTo(id)
	if !ok {
		return false
	}
	if m.queryState.Text() != root {
		m.queryState.StartEditing()
		m.setQueryText(root)
		m.queryState.Accept()
	}
	if !m.tree.SelectByID(id) {
		return false
	}
	m.tree.ensureCursorVisible()
	m.statusMsg = "Branch " + root
	m.statusIsError = false
	m.syncTreeToDetail()
	return true
}
