package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vanderheijden86/beadwork/pkg/model"
)

func showBranchTreeIssues() []model.Issue {
	return []model.Issue{
		{ID: "epic-a", Title: "First epic", IssueType: model.TypeEpic, Status: model.StatusOpen},
		childOf("feature-a", "Nested feature", "epic-a", model.TypeFeature, model.StatusOpen),
		childOf("task-a", "Nested task", "feature-a", model.TypeTask, model.StatusOpen),
		{ID: "epic-b", Title: "Second epic", IssueType: model.TypeEpic, Status: model.StatusOpen},
		childOf("task-b", "Other task", "epic-b", model.TypeTask, model.StatusOpen),
	}
}

func showBranchTreeModel(t *testing.T) Model {
	t.Helper()
	m := NewModel(showBranchTreeIssues(), "")
	m.tree.Build(showBranchTreeIssues())
	m.focused = focusTree
	m.treeViewActive = true
	return m
}

func sendMsg(t *testing.T, m Model, msg tea.Msg) (Model, tea.Cmd) {
	t.Helper()
	updated, cmd := m.Update(msg)
	return updated.(Model), cmd
}

func TestShowBranchMsg_TreeSelectsCollapsedIssueAndFiltersToItsBranch(t *testing.T) {
	m := showBranchTreeModel(t)

	m, _ = sendMsg(t, m, ShowBranchMsg{ID: "task-a"})

	if got := m.queryState.Text(); got != "epic-a" {
		t.Fatalf("branch query = %q, want epic-a", got)
	}
	if got := m.queryState.Mode(); got != QueryIdle {
		t.Fatalf("query mode = %v, want accepted query", got)
	}
	if got := m.tree.GetSelectedID(); got != "task-a" {
		t.Fatalf("selected %q, want task-a", got)
	}
	if got := strings.Join(treeVisibleIDs(&m.tree), ","); got != "epic-a,feature-a,task-a" {
		t.Fatalf("visible = %s, want the epic-a branch", got)
	}
}

func TestShowBranchMsg_RepeatKeepsTheFilterOn(t *testing.T) {
	m := showBranchTreeModel(t)

	m, _ = sendMsg(t, m, ShowBranchMsg{ID: "task-a"})
	m, _ = sendMsg(t, m, ShowBranchMsg{ID: "feature-a"})

	if got := m.queryState.Text(); got != "epic-a" {
		t.Fatalf("second request toggled the branch off: query = %q", got)
	}
	if got := m.tree.GetSelectedID(); got != "feature-a" {
		t.Fatalf("selected %q, want feature-a", got)
	}

	m, _ = sendMsg(t, m, ShowBranchMsg{ID: "task-b"})
	if got := m.queryState.Text(); got != "epic-b" {
		t.Fatalf("query = %q, want the other branch epic-b", got)
	}
}

func TestShowBranchMsg_BoardSetsBranchAndSelectsCard(t *testing.T) {
	m := branchBoardModel(t, "spectroscope-x1")

	m, _ = sendMsg(t, m, ShowBranchMsg{ID: "spectroscope-eg0.2.1"})
	m, _ = sendMsg(t, m, ShowBranchMsg{ID: "spectroscope-eg0.1"})

	if got := m.board.BranchRoot(); got != "spectroscope-eg0" {
		t.Fatalf("board branch = %q, want spectroscope-eg0", got)
	}
	if sel := m.board.SelectedIssue(); sel == nil || sel.ID != "spectroscope-eg0.1" {
		t.Fatalf("board selected %v, want spectroscope-eg0.1", sel)
	}
}

func TestShowBranchMsg_UnknownIDWaitsForTheReloadThatAddsIt(t *testing.T) {
	m := showBranchTreeModel(t)

	m, cmd := sendMsg(t, m, ShowBranchMsg{ID: "task-new"})
	if cmd == nil {
		t.Fatal("pending request scheduled no deadline")
	}
	if got := m.queryState.Text(); got != "" {
		t.Fatalf("unknown id changed the query to %q", got)
	}

	issues := append(showBranchTreeIssues(), childOf("task-new", "Fresh task", "epic-b", model.TypeTask, model.StatusOpen))
	m, _ = sendMsg(t, m, SnapshotReadyMsg{Snapshot: NewSnapshotBuilder(issues).Build()})

	if got := m.queryState.Text(); got != "epic-b" {
		t.Fatalf("query after reload = %q, want epic-b", got)
	}
	if got := m.tree.GetSelectedID(); got != "task-new" {
		t.Fatalf("selected %q after reload, want task-new", got)
	}
}

func TestShowBranchMsg_PendingRequestExpiresAtItsDeadline(t *testing.T) {
	m := showBranchTreeModel(t)

	m, _ = sendMsg(t, m, ShowBranchMsg{ID: "task-never"})
	m, _ = sendMsg(t, m, showBranchDeadlineMsg{ID: "task-never"})

	if !m.statusIsError || !strings.Contains(m.statusMsg, "task-never") {
		t.Fatalf("expiry status = %q (error %v), want an error naming the id", m.statusMsg, m.statusIsError)
	}
	issues := append(showBranchTreeIssues(), childOf("task-never", "Late task", "epic-b", model.TypeTask, model.StatusOpen))
	m, _ = sendMsg(t, m, SnapshotReadyMsg{Snapshot: NewSnapshotBuilder(issues).Build()})
	if got := m.queryState.Text(); got != "" {
		t.Fatalf("expired request still applied: query = %q", got)
	}
}

func TestBranchCommandShowsTheNamedIssuesBranch(t *testing.T) {
	m := showBranchTreeModel(t)

	m = typeKeys(m, ":", "b", "r", "a", "n", "c", "h", " ", "t", "a", "s", "k", "-", "a", "enter")

	if got := m.queryState.Text(); got != "epic-a" {
		t.Fatalf(":branch task-a query = %q, want epic-a", got)
	}
	if got := m.tree.GetSelectedID(); got != "task-a" {
		t.Fatalf(":branch selected %q, want task-a", got)
	}
}

func TestShowBranchMsg_AcceptsTheShortIDShownOnScreen(t *testing.T) {
	m := branchBoardModel(t, "spectroscope-x1")

	m, _ = sendMsg(t, m, ShowBranchMsg{ID: "k3s.1"})

	if got := m.board.BranchRoot(); got != "spectroscope-k3s" {
		t.Fatalf("board branch = %q, want spectroscope-k3s", got)
	}
}

func TestShowKnownBranchMsg_ShowsTheFirstLoadedIDAndIgnoresTheRest(t *testing.T) {
	m := showBranchTreeModel(t)

	m, cmd := sendMsg(t, m, ShowKnownBranchMsg{IDs: []string{"follow-up", "task-b", "task-a"}})

	if got := m.queryState.Text(); got != "epic-b" {
		t.Fatalf("branch query = %q, want epic-b", got)
	}
	if got := m.tree.GetSelectedID(); got != "task-b" {
		t.Fatalf("selected %q, want task-b", got)
	}
	if cmd != nil {
		t.Fatal("a known id must not start a wait")
	}
}

func TestShowKnownBranchMsg_NoLoadedIDChangesNothing(t *testing.T) {
	m := showBranchTreeModel(t)
	m.statusMsg = "before"

	m, cmd := sendMsg(t, m, ShowKnownBranchMsg{IDs: []string{"follow-up", "other-x1"}})

	if cmd != nil || m.pendingBranchID != "" {
		t.Fatalf("unknown ids started a wait: pending %q", m.pendingBranchID)
	}
	if m.statusMsg != "before" || m.statusIsError {
		t.Fatalf("status = %q (error %v), want it untouched", m.statusMsg, m.statusIsError)
	}
	if got := m.queryState.Text(); got != "" {
		t.Fatalf("query = %q, want none", got)
	}
}
