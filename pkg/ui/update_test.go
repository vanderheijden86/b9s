package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/vanderheijden86/b9s/pkg/model"
)

type badItem struct{}

func (badItem) Title() string       { return "bad" }
func (badItem) Description() string { return "bad" }
func (badItem) FilterValue() string { return "bad" }

func followModeIssues(includeNewChild bool) []model.Issue {
	now := time.Unix(1_700_000_000, 0)
	issues := []model.Issue{
		{ID: "epic", Title: "Epic", Status: model.StatusOpen, IssueType: model.TypeEpic, CreatedAt: now},
		{
			ID: "nested-epic", Title: "Nested epic", Status: model.StatusOpen, IssueType: model.TypeEpic,
			CreatedAt: now.Add(time.Minute),
			Dependencies: []*model.Dependency{
				{IssueID: "nested-epic", DependsOnID: "epic", Type: model.DepParentChild},
			},
		},
	}
	if includeNewChild {
		issues = append(issues, model.Issue{
			ID: "nested-bug", Title: "Nested bug", Status: model.StatusClosed, IssueType: model.TypeBug,
			CreatedAt: now.Add(2 * time.Minute),
			Dependencies: []*model.Dependency{
				{IssueID: "nested-bug", DependsOnID: "nested-epic", Type: model.DepParentChild},
			},
		})
	}
	return issues
}

func TestSnapshotReadyMsgFollowRevealsNewNestedIssue(t *testing.T) {
	m := NewModel(followModeIssues(false), "")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("F")})
	m = updated.(Model)

	snapshot := NewSnapshotBuilder(followModeIssues(true)).Build()
	updated, _ = m.Update(SnapshotReadyMsg{Snapshot: snapshot})
	m = updated.(Model)

	if got := m.tree.GetSelectedID(); got != "nested-bug" {
		t.Fatalf("follow mode selected %q after snapshot reload, want nested-bug", got)
	}
	if got := m.tree.NodeCount(); got != 3 {
		t.Fatalf("follow mode left the nested parent collapsed: visible nodes = %d, want 3", got)
	}
}

func TestSnapshotReadyMsgFollowOffPreservesCollapsedSelection(t *testing.T) {
	m := NewModel(followModeIssues(false), "")
	if !m.tree.SelectByID("nested-epic") {
		t.Fatal("select nested-epic")
	}

	snapshot := NewSnapshotBuilder(followModeIssues(true)).Build()
	updated, _ := m.Update(SnapshotReadyMsg{Snapshot: snapshot})
	m = updated.(Model)

	if got := m.tree.GetSelectedID(); got != "nested-epic" {
		t.Fatalf("follow-off reload moved selection to %q, want nested-epic", got)
	}
	if got := m.tree.NodeCount(); got != 2 {
		t.Fatalf("follow-off reload expanded the nested parent: visible nodes = %d, want 2", got)
	}
}

func TestFileChangedMsgFollowRevealsNewNestedIssue(t *testing.T) {
	tmp := t.TempDir()
	beads := filepath.Join(tmp, "beads.jsonl")
	initial := "" +
		`{"id":"epic","title":"Epic","status":"open","issue_type":"epic","created_at":"2023-11-14T22:13:20Z"}` + "\n" +
		`{"id":"nested-epic","title":"Nested epic","status":"open","issue_type":"epic","created_at":"2023-11-14T22:14:20Z","dependencies":[{"issue_id":"nested-epic","depends_on_id":"epic","type":"parent-child"}]}` + "\n"
	if err := os.WriteFile(beads, []byte(initial), 0o644); err != nil {
		t.Fatalf("write initial beads: %v", err)
	}

	m := NewModel(followModeIssues(false), beads)
	if m.watcher != nil {
		defer m.watcher.Stop()
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("F")})
	m = updated.(Model)

	withChild := initial + `{"id":"nested-bug","title":"Nested bug","status":"closed","issue_type":"bug","created_at":"2023-11-14T22:15:20Z","dependencies":[{"issue_id":"nested-bug","depends_on_id":"nested-epic","type":"parent-child"}]}` + "\n"
	if err := os.WriteFile(beads, []byte(withChild), 0o644); err != nil {
		t.Fatalf("write updated beads: %v", err)
	}

	updated, _ = m.Update(FileChangedMsg{})
	m = updated.(Model)
	if got := m.tree.GetSelectedID(); got != "nested-bug" {
		t.Fatalf("follow mode selected %q after file reload, want nested-bug", got)
	}
	if got := m.tree.NodeCount(); got != 3 {
		t.Fatalf("follow mode left the nested parent collapsed: visible nodes = %d, want 3", got)
	}
}

func TestCopyIssueToClipboardInvalidItem(t *testing.T) {
	m := NewModel(nil, "")
	m.list.SetItems([]list.Item{badItem{}})
	m.list.Select(0)
	m.copyIssueToClipboard()
	if !m.statusIsError || m.statusMsg == "" {
		t.Fatalf("expected error copying invalid item, got %q", m.statusMsg)
	}
}

func TestDetailCKeyCopiesSelectedIssue(t *testing.T) {
	issue := model.Issue{
		ID:                 "bd-copy",
		Title:              "Copy this ticket",
		Description:        "Full description",
		Design:             "Design rationale",
		AcceptanceCriteria: "Copy is complete",
		Notes:              "Progress notes",
		Status:             model.StatusOpen,
		IssueType:          model.TypeTask,
		Comments: []*model.Comment{{
			Author: "alex",
			Text:   "A useful comment",
		}},
	}
	m := NewModel([]model.Issue{issue}, "")
	m.focused = focusDetail
	var copied string
	m.clipboardWrite = func(text string) error {
		copied = text
		return nil
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = updated.(Model)

	if !strings.Contains(m.statusMsg, "Copied bd-copy to clipboard") {
		t.Fatalf("expected c in detail view to copy the selected issue, got status %q", m.statusMsg)
	}
	for _, want := range []string{
		"# ✔ Copy this ticket",
		"### Description\nFull description",
		"### Design Notes\nDesign rationale",
		"### Acceptance Criteria\nCopy is complete",
		"### Notes\nProgress notes",
		"### Comments (1)",
		"A useful comment",
	} {
		if !strings.Contains(copied, want) {
			t.Errorf("copied Markdown missing %q:\n%s", want, copied)
		}
	}
}

func TestUpdateFileChangedReloadsSelection(t *testing.T) {
	data := `{"id":"ONE","title":"One","status":"open"}`
	tmp := t.TempDir()
	beads := filepath.Join(tmp, "beads.jsonl")
	if err := os.WriteFile(beads, []byte(data), 0644); err != nil {
		t.Fatalf("write beads: %v", err)
	}
	m := NewModel(nil, beads)
	m.list.SetItems([]list.Item{IssueItem{Issue: model.Issue{ID: "ONE", Title: "One", Status: model.StatusOpen}}})
	m.list.Select(0)

	updated, cmd := m.Update(FileChangedMsg{})
	_ = cmd
	m2 := updated.(Model)
	if m2.statusIsError {
		t.Fatalf("expected successful reload, got error %q", m2.statusMsg)
	}
}

func TestCtrlRRequestsImmediateRefresh(t *testing.T) {
	tmp := t.TempDir()
	beads := filepath.Join(tmp, "beads.jsonl")
	if err := os.WriteFile(beads, []byte(`{"id":"ONE","title":"One","status":"open"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write beads: %v", err)
	}

	m := NewModel(nil, beads)
	if m.watcher != nil {
		defer m.watcher.Stop()
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	m2 := updated.(Model)
	if cmd == nil {
		t.Fatal("expected Ctrl+R to request a refresh")
	}
	if m2.statusIsError || m2.statusMsg != "Refreshing…" {
		t.Fatalf("expected refreshing status, got error=%v message=%q", m2.statusIsError, m2.statusMsg)
	}
}

func TestFileChangedMsg_RebuildsTreeWhenFocused(t *testing.T) {
	// Start with one issue on disk and in the model.
	initialIssues := []model.Issue{
		{ID: "parent", Title: "Parent", Status: model.StatusOpen, IssueType: model.TypeTask, Priority: 1},
	}
	data := `{"id":"parent","title":"Parent","status":"open","issue_type":"task","priority":1}` + "\n"
	tmp := t.TempDir()
	beadsDir := filepath.Join(tmp, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatalf("mkdir beads: %v", err)
	}
	beads := filepath.Join(tmp, "beads.jsonl")
	if err := os.WriteFile(beads, []byte(data), 0644); err != nil {
		t.Fatalf("write beads: %v", err)
	}

	m := NewModel(initialIssues, beads)
	m.width, m.height = 120, 40
	m.tree.SetBeadsDir(beadsDir)

	// Model starts in tree view by default.
	if m.focused != focusTree {
		t.Fatalf("expected focusTree on launch, got %v", m.focused)
	}

	// Initial tree should have 1 node.
	initialCount := m.tree.NodeCount()
	if initialCount != 1 {
		t.Fatalf("expected 1 tree node initially, got %d", initialCount)
	}

	// Write updated data with a second issue to disk.
	data2 := data + `{"id":"child","title":"Child","status":"open","issue_type":"task","priority":2,"dependencies":[{"issue_id":"child","depends_on_id":"parent","type":"parent-child"}]}` + "\n"
	if err := os.WriteFile(beads, []byte(data2), 0644); err != nil {
		t.Fatalf("write beads: %v", err)
	}

	// Process FileChangedMsg (sync path, no background worker).
	updated, _ := m.Update(FileChangedMsg{})
	m2 := updated.(Model)

	if m2.statusIsError {
		t.Fatalf("expected successful reload, got error %q", m2.statusMsg)
	}

	// Tree should now have 2 nodes reflecting the updated file.
	if got := m2.tree.NodeCount(); got != 2 {
		t.Fatalf("expected tree rebuilt with 2 nodes after FileChangedMsg, got %d (tree not auto-updated)", got)
	}
}

func TestNewModel_SetsTreeBeadsDirFromBeadsPath(t *testing.T) {
	tmp := t.TempDir()
	beads := filepath.Join(tmp, "beads.jsonl")
	if err := os.WriteFile(beads, []byte(`{"id":"ONE","title":"One","status":"open"}`+"\n"), 0644); err != nil {
		t.Fatalf("write beads: %v", err)
	}

	m := NewModel(nil, beads)
	if m.watcher != nil {
		m.watcher.Stop()
	}

	if got, want := m.tree.beadsDir, filepath.Dir(beads); got != want {
		t.Fatalf("expected tree beadsDir %q, got %q", want, got)
	}
}

func TestMouseWheelDownMovesTreeSelection(t *testing.T) {
	issues := []model.Issue{
		{ID: "issue-1", Title: "First", Status: model.StatusOpen, IssueType: model.TypeTask, Priority: 1},
		{ID: "issue-2", Title: "Second", Status: model.StatusOpen, IssueType: model.TypeTask, Priority: 2},
	}
	m := NewModel(issues, "")
	initialID := m.tree.GetSelectedID()

	updated, _ := m.Update(tea.MouseMsg{
		Button: tea.MouseButtonWheelDown,
		Action: tea.MouseActionPress,
	})
	m = updated.(Model)

	if got := m.tree.GetSelectedID(); got == initialID {
		t.Fatalf("mouse wheel down left tree selection on %q", got)
	}
}

func TestMouseWheelUpMovesTreeSelection(t *testing.T) {
	issues := []model.Issue{
		{ID: "issue-1", Title: "First", Status: model.StatusOpen, IssueType: model.TypeTask, Priority: 1},
		{ID: "issue-2", Title: "Second", Status: model.StatusOpen, IssueType: model.TypeTask, Priority: 2},
	}
	m := NewModel(issues, "")
	m.tree.MoveDown()
	initialID := m.tree.GetSelectedID()

	updated, _ := m.Update(tea.MouseMsg{
		Button: tea.MouseButtonWheelUp,
		Action: tea.MouseActionPress,
	})
	m = updated.(Model)

	if got := m.tree.GetSelectedID(); got == initialID {
		t.Fatalf("mouse wheel up left tree selection on %q", got)
	}
}

func TestMouseWheelScrollsDetailViewport(t *testing.T) {
	m := NewModel(nil, "")
	m.focused = focusDetail
	m.viewport.Height = 2
	m.viewport.SetContent("one\ntwo\nthree\nfour\nfive")
	m.viewport.LineDown(3)
	initialOffset := m.viewport.YOffset

	updated, _ := m.Update(tea.MouseMsg{
		Button: tea.MouseButtonWheelUp,
		Action: tea.MouseActionPress,
	})
	m = updated.(Model)

	if got := m.viewport.YOffset; got >= initialOffset {
		t.Fatalf("mouse wheel up left detail offset at %d, want less than %d", got, initialOffset)
	}
}

func TestMouseWheelDownMovesBoardSelection(t *testing.T) {
	issues := []model.Issue{
		{ID: "issue-1", Title: "First", Status: model.StatusOpen, IssueType: model.TypeTask, Priority: 1},
		{ID: "issue-2", Title: "Second", Status: model.StatusOpen, IssueType: model.TypeTask, Priority: 2},
	}
	m := NewModel(issues, "")
	m.focused = focusBoard
	initial := m.board.SelectedIssue()
	if initial == nil {
		t.Fatal("expected an initial board selection")
	}

	updated, _ := m.Update(tea.MouseMsg{
		Button: tea.MouseButtonWheelDown,
		Action: tea.MouseActionPress,
	})
	m = updated.(Model)

	selected := m.board.SelectedIssue()
	if selected == nil || selected.ID == initial.ID {
		t.Fatalf("mouse wheel down left board selection on %q", initial.ID)
	}
}
