package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vanderheijden86/beadwork/pkg/model"
)

// modalRoutingFixture builds a Model over one on-disk issue, wired the same
// way TestFileChangedMsg_RebuildsTreeWhenFocused is, so a FileChangedMsg
// reload is observable through the tree node count and dispatches a watch
// cmd to re-arm the watcher.
func modalRoutingFixture(t *testing.T) (m Model, beadsPath string) {
	t.Helper()
	initialIssues := []model.Issue{
		{ID: "parent", Title: "Parent", Status: model.StatusOpen, IssueType: model.TypeTask, Priority: 1},
	}
	data := `{"id":"parent","title":"Parent","status":"open","issue_type":"task","priority":1}` + "\n"
	tmp := t.TempDir()
	beadsDir := filepath.Join(tmp, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	beadsPath = filepath.Join(tmp, "beads.jsonl")
	if err := os.WriteFile(beadsPath, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	m = NewModel(initialIssues, beadsPath)
	m.width, m.height = 120, 40
	m.tree.SetBeadsDir(beadsDir)
	if m.focused != focusTree {
		t.Fatalf("expected focusTree on launch, got %v", m.focused)
	}
	if got := m.tree.NodeCount(); got != 1 {
		t.Fatalf("expected 1 tree node initially, got %d", got)
	}
	return m, beadsPath
}

// addSecondIssueToFixture rewrites beadsPath with a second issue so the next
// FileChangedMsg reload is observable as a tree node count change.
func addSecondIssueToFixture(t *testing.T, beadsPath string) {
	t.Helper()
	data := `{"id":"parent","title":"Parent","status":"open","issue_type":"task","priority":1}` + "\n" +
		`{"id":"child","title":"Child","status":"open","issue_type":"task","priority":2,"dependencies":[{"issue_id":"child","depends_on_id":"parent","type":"parent-child"}]}` + "\n"
	if err := os.WriteFile(beadsPath, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestEditModalOpenStillReloadsOnFileChanged is the bd-grtt guard for the
// edit modal: opening it must not swallow FileChangedMsg for the rest of the
// session. The 'e' key from tree focus does not reassign m.focused, so the
// tree rebuild branch of the FileChangedMsg handler still runs with the
// modal open.
func TestEditModalOpenStillReloadsOnFileChanged(t *testing.T) {
	m, beadsPath := modalRoutingFixture(t)
	if m.watcher != nil {
		defer m.watcher.Stop()
	}

	m, _ = pressBulkKey(t, m, runeKey("e"))
	if !m.showEditModal {
		t.Fatal("e from tree focus must open the edit modal")
	}

	addSecondIssueToFixture(t, beadsPath)

	updated, cmd := m.Update(FileChangedMsg{})
	m = updated.(Model)

	if !m.showEditModal {
		t.Fatal("FileChangedMsg must not close the edit modal")
	}
	if got := m.tree.NodeCount(); got != 2 {
		t.Fatalf("tree node count after FileChangedMsg = %d, want 2 (reload swallowed while modal open)", got)
	}
	if cmd == nil {
		t.Fatal("FileChangedMsg must still return a cmd (watch cmd re-arm) while the edit modal is open")
	}
}

// TestAttachAddModalOpenStillReloadsOnFileChanged is the same bd-grtt guard
// for the attach-add modal.
func TestAttachAddModalOpenStillReloadsOnFileChanged(t *testing.T) {
	m, beadsPath := modalRoutingFixture(t)
	if m.watcher != nil {
		defer m.watcher.Stop()
	}
	m.issueWriter = &IssueWriter{bdPath: "/bin/echo", available: true, checkout: testCheckout(t)}
	m.tree.SelectByID("parent")

	m, _ = pressBulkKey(t, m, runeKey("I"))
	if !m.showAttachAddModal {
		t.Fatal("I must open the attach-files form")
	}

	addSecondIssueToFixture(t, beadsPath)

	updated, cmd := m.Update(FileChangedMsg{})
	m = updated.(Model)

	if !m.showAttachAddModal {
		t.Fatal("FileChangedMsg must not close the attach-add modal")
	}
	if got := m.tree.NodeCount(); got != 2 {
		t.Fatalf("tree node count after FileChangedMsg = %d, want 2 (reload swallowed while modal open)", got)
	}
	if cmd == nil {
		t.Fatal("FileChangedMsg must still return a cmd (watch cmd re-arm) while the attach-add modal is open")
	}
}

// TestAttachAddResultAppliesWhileSecondFormOpen covers the other half of
// bd-grtt: an attachAddResultMsg arriving from a first attach while the user
// has already reopened the form for a second attach must still apply its
// status and dispatch the reload, not get swallowed by the newly open modal.
func TestAttachAddResultAppliesWhileSecondFormOpen(t *testing.T) {
	m, _, _ := attachAddTestModel(t, true, 0)
	dispatchedFrom := m.activeProjectPath

	m, _ = pressBulkKey(t, m, runeKey("I"))
	if !m.showAttachAddModal {
		t.Fatal("I must open the attach-files form for the second attach")
	}

	updated, cmd := m.Update(attachAddResultMsg{
		dispatchProjectPath: dispatchedFrom,
		issueID:             "bd-1",
		statusMsg:           "3 attached",
		reload:              true,
	})
	m = updated.(Model)

	if !m.showAttachAddModal {
		t.Fatal("attachAddResultMsg must not close the newly reopened form")
	}
	if m.statusMsg != "3 attached" {
		t.Fatalf("statusMsg = %q, want the result's status applied even with a second form open", m.statusMsg)
	}
	if m.statusIsError {
		t.Fatal("statusIsError = true, want false for a successful result")
	}
	if cmd == nil {
		t.Fatal("attachAddResultMsg with reload=true must dispatch a cmd (the FileChangedMsg reload trigger)")
	}
}
