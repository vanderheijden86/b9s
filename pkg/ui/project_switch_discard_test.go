package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanderheijden86/b9s/pkg/config"
)

// switchModelWithFakeBd is switchModel plus a fake bd recorder wired as the
// active project's IssueWriter, so a test can assert whether a write ran.
func switchModelWithFakeBd(t *testing.T) (m Model, alpha, beta config.Project, recordPath string) {
	t.Helper()
	m, alpha, beta = switchModel(t)
	recordPath = t.TempDir() + "/bd-calls"
	fakeBdPath := writeFakeBdRecorder(t, recordPath, 0)
	checkout, ok := NewCheckout(alpha.Path)
	if !ok {
		t.Fatalf("NewCheckout(%q) rejected alpha's checkout", alpha.Path)
	}
	m.issueWriter = &IssueWriter{bdPath: fakeBdPath, available: true, checkout: checkout}
	m.tree.SelectByID("alpha-1")
	return m, alpha, beta, recordPath
}

// TestOpeningProjectRefusesEditKey is the first half of the bd-grtt fix: a
// projectOpenedMsg landing while the edit modal is open lets its eventual
// save dispatch bd against the newly switched checkout while still carrying
// the old project's issue ID, so the key that would open the modal is
// refused outright while a switch is in flight (the reviewer's proof:
// SwitchProjectMsg{B}, press e while "Opening B…" shows, projectOpenedMsg
// lands, then ctrl+s).
func TestOpeningProjectRefusesEditKey(t *testing.T) {
	m, _, beta, recordPath := switchModelWithFakeBd(t)
	m, _ = requestSwitch(t, m, beta)
	if m.projectSwitch.state != SwitchOpening {
		t.Fatalf("switch state = %v, want opening", m.projectSwitch.state)
	}

	m, _ = pressBulkKey(t, m, runeKey("e"))
	if m.showEditModal {
		t.Fatal("e must be refused while a project switch is opening")
	}
	if !strings.Contains(m.statusMsg, "Opening") {
		t.Fatalf("statusMsg = %q, want it to explain the refusal", m.statusMsg)
	}

	updated, _ := m.Update(projectOpenedMsg{generation: m.projectSwitch.generation, project: beta})
	m = updated.(Model)
	if m.activeProjectName != "beta" {
		t.Fatalf("active = %q, want beta", m.activeProjectName)
	}

	m, _ = pressBulkKey(t, m, tea.KeyMsg{Type: tea.KeyCtrlS})
	if m.showEditModal {
		t.Fatal("edit modal must stay closed after the switch completes")
	}
	if calls := readBdCallsUI(t, recordPath); len(calls) != 0 {
		t.Fatalf("bd calls = %v, want none: the modal never opened", calls)
	}
}

// TestOpeningProjectRefusesAttachKey is the attach-form half of the same
// scenario as TestOpeningProjectRefusesEditKey.
func TestOpeningProjectRefusesAttachKey(t *testing.T) {
	m, _, beta, recordPath := switchModelWithFakeBd(t)
	m, _ = requestSwitch(t, m, beta)

	m, _ = pressBulkKey(t, m, runeKey("I"))
	if m.showAttachAddModal {
		t.Fatal("I must be refused while a project switch is opening")
	}
	if !strings.Contains(m.statusMsg, "Opening") {
		t.Fatalf("statusMsg = %q, want it to explain the refusal", m.statusMsg)
	}

	updated, _ := m.Update(projectOpenedMsg{generation: m.projectSwitch.generation, project: beta})
	m = updated.(Model)

	if calls := readBdCallsUI(t, recordPath); len(calls) != 0 {
		t.Fatalf("bd calls = %v, want none: the form never opened", calls)
	}
}

// TestProjectSwitchDiscardsOpenEditModal is the belt-and-suspenders half of
// the bd-grtt fix: modalRefusedBySwitch only stops a modal from opening once
// a switch is already in flight, so a modal opened before the switch began
// is untouched by that guard. applyProjectSwitch itself must close it the
// moment the new project actually replaces the old one, or its eventual
// save would still dispatch against the new checkout under the old
// project's issue ID.
func TestProjectSwitchDiscardsOpenEditModal(t *testing.T) {
	m, _, beta, recordPath := switchModelWithFakeBd(t)

	m, _ = pressBulkKey(t, m, runeKey("e"))
	if !m.showEditModal {
		t.Fatal("e must open the edit modal while idle")
	}

	m, _ = m.beginProjectSwitch(beta)
	if !m.showEditModal {
		t.Fatal("beginning a switch must not itself close an already-open modal")
	}

	updated, _ := m.Update(projectOpenedMsg{generation: m.projectSwitch.generation, project: beta})
	m = updated.(Model)

	if m.showEditModal {
		t.Fatal("applyProjectSwitch must discard the edit modal left open by the previous project")
	}
	if m.activeProjectName != "beta" {
		t.Fatalf("active = %q, want beta", m.activeProjectName)
	}
	if !strings.Contains(m.statusMsg, "discarded") {
		t.Fatalf("statusMsg = %q, want it to report the discarded edit", m.statusMsg)
	}

	m, _ = pressBulkKey(t, m, tea.KeyMsg{Type: tea.KeyCtrlS})
	if calls := readBdCallsUI(t, recordPath); len(calls) != 0 {
		t.Fatalf("bd calls = %v, want none: no modal was open to submit", calls)
	}
}

// TestProjectSwitchDiscardsOpenAttachModal is the attach-form half of
// TestProjectSwitchDiscardsOpenEditModal.
func TestProjectSwitchDiscardsOpenAttachModal(t *testing.T) {
	m, _, beta, recordPath := switchModelWithFakeBd(t)

	m, _ = pressBulkKey(t, m, runeKey("I"))
	if !m.showAttachAddModal {
		t.Fatal("I must open the attach form while idle")
	}

	m, _ = m.beginProjectSwitch(beta)
	if !m.showAttachAddModal {
		t.Fatal("beginning a switch must not itself close an already-open modal")
	}

	updated, _ := m.Update(projectOpenedMsg{generation: m.projectSwitch.generation, project: beta})
	m = updated.(Model)

	if m.showAttachAddModal {
		t.Fatal("applyProjectSwitch must discard the attach form left open by the previous project")
	}
	if calls := readBdCallsUI(t, recordPath); len(calls) != 0 {
		t.Fatalf("bd calls = %v, want none: no form was open to submit", calls)
	}
}

// TestProjectSwitchDiscardsOpenStatusPicker is the status-picker half of
// TestProjectSwitchDiscardsOpenEditModal: without this, removing the
// m.showStatusPicker = false line from discardOpenModals leaves the suite
// green (bd-l66t).
func TestProjectSwitchDiscardsOpenStatusPicker(t *testing.T) {
	m, _, beta, recordPath := switchModelWithFakeBd(t)

	m, _ = pressBulkKey(t, m, runeKey("S"))
	if !m.showStatusPicker {
		t.Fatal("S must open the status picker while idle")
	}

	m, _ = m.beginProjectSwitch(beta)
	if !m.showStatusPicker {
		t.Fatal("beginning a switch must not itself close an already-open picker")
	}

	updated, _ := m.Update(projectOpenedMsg{generation: m.projectSwitch.generation, project: beta})
	m = updated.(Model)

	if m.showStatusPicker {
		t.Fatal("applyProjectSwitch must discard the status picker left open by the previous project")
	}
	if calls := readBdCallsUI(t, recordPath); len(calls) != 0 {
		t.Fatalf("bd calls = %v, want none: no picker was open to submit", calls)
	}
}

// TestProjectSwitchDiscardsOpenCloseConfirmation is the close-confirmation
// half of TestProjectSwitchDiscardsOpenEditModal: without this, removing the
// m.issueConfirm = issueConfirmation{} line from discardOpenModals leaves
// the suite green (bd-l66t).
func TestProjectSwitchDiscardsOpenCloseConfirmation(t *testing.T) {
	m, _, beta, recordPath := switchModelWithFakeBd(t)

	m, _ = pressBulkKey(t, m, runeKey("K"))
	if m.issueConfirm.action != issueConfirmClose {
		t.Fatal("K must open the close confirmation while idle")
	}

	m, _ = m.beginProjectSwitch(beta)
	if m.issueConfirm.action != issueConfirmClose {
		t.Fatal("beginning a switch must not itself close an already-open confirmation")
	}

	updated, _ := m.Update(projectOpenedMsg{generation: m.projectSwitch.generation, project: beta})
	m = updated.(Model)

	if m.issueConfirm.action != issueConfirmNone {
		t.Fatal("applyProjectSwitch must discard the confirmation left open by the previous project")
	}
	if calls := readBdCallsUI(t, recordPath); len(calls) != 0 {
		t.Fatalf("bd calls = %v, want none: no confirmation was open to submit", calls)
	}
}

// TestOverlayKeysRefusedWhileProjectOpening is the reviewer's scenario from
// TestOpeningProjectRefusesEditKey/AttachKey, run for every overlay that
// writes against a captured issue ID: e, I, S, K and delete must all be
// refused while a switch is opening, so none of their overlays can be left
// open to write into the project the switch lands on (bd-l66t).
func TestOverlayKeysRefusedWhileProjectOpening(t *testing.T) {
	cases := []struct {
		name   string
		key    tea.KeyMsg
		setup  func(m Model) Model
		isOpen func(m Model) bool
	}{
		{"edit", runeKey("e"), nil, func(m Model) bool { return m.showEditModal }},
		{"edit-list-mode", runeKey("e"), func(m Model) Model { m.focused = focusList; return m }, func(m Model) bool { return m.showEditModal }},
		{"create", tea.KeyMsg{Type: tea.KeyCtrlN}, nil, func(m Model) bool { return m.showEditModal }},
		{"attach", runeKey("I"), nil, func(m Model) bool { return m.showAttachAddModal }},
		{"status", runeKey("S"), nil, func(m Model) bool { return m.showStatusPicker }},
		{"close", runeKey("K"), nil, func(m Model) bool { return m.issueConfirm.action == issueConfirmClose }},
		{"delete", tea.KeyMsg{Type: tea.KeyDelete}, nil, func(m Model) bool { return m.issueConfirm.action == issueConfirmDelete }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, _, beta, recordPath := switchModelWithFakeBd(t)
			if tc.setup != nil {
				m = tc.setup(m)
			}
			m, _ = requestSwitch(t, m, beta)
			if m.projectSwitch.state != SwitchOpening {
				t.Fatalf("switch state = %v, want opening", m.projectSwitch.state)
			}

			m, _ = pressBulkKey(t, m, tc.key)
			if tc.isOpen(m) {
				t.Fatalf("%s must be refused while a project switch is opening", tc.name)
			}
			if !strings.Contains(m.statusMsg, "Opening") {
				t.Fatalf("statusMsg = %q, want it to explain the refusal", m.statusMsg)
			}

			updated, _ := m.Update(projectOpenedMsg{generation: m.projectSwitch.generation, project: beta})
			m = updated.(Model)
			if m.activeProjectName != "beta" {
				t.Fatalf("active = %q, want beta", m.activeProjectName)
			}
			if calls := readBdCallsUI(t, recordPath); len(calls) != 0 {
				t.Fatalf("bd calls = %v, want none: the overlay never opened", calls)
			}
		})
	}
}

// TestOverlayWriteRefusedOnGenerationMismatch proves writeAllowedForGeneration
// itself, not only the discard applyProjectSwitch performs: each overlay is
// opened normally, then projectGeneration is bumped directly (without going
// through applyProjectSwitch, so discardOpenModals never runs) before the
// overlay's own write path fires. If writeAllowedForGeneration were removed
// and only discardOpenModals remained, every one of these would still write
// (bd-l66t).
func TestOverlayWriteRefusedOnGenerationMismatch(t *testing.T) {
	t.Run("edit", func(t *testing.T) {
		m, _, _, recordPath := switchModelWithFakeBd(t)
		m, _ = pressBulkKey(t, m, runeKey("e"))
		if !m.showEditModal {
			t.Fatal("e must open the edit modal")
		}
		*m.editModal.title = "changed while opening"
		m.projectGeneration++

		m, cmd := pressBulkKey(t, m, tea.KeyMsg{Type: tea.KeyCtrlS})
		if m.showEditModal {
			t.Fatal("ctrl+s must still close the modal")
		}
		if !strings.Contains(m.statusMsg, "discarded") {
			t.Fatalf("statusMsg = %q, want it to report the discard", m.statusMsg)
		}
		if cmd != nil {
			updated, _ := m.Update(cmd())
			m = updated.(Model)
		}
		if calls := readBdCallsUI(t, recordPath); len(calls) != 0 {
			t.Fatalf("bd calls = %v, want none: the generation mismatch must refuse the save", calls)
		}
	})

	t.Run("attach", func(t *testing.T) {
		m, recordPath, _ := attachAddTestModel(t, true, 0)
		srcDir := t.TempDir()
		file := filepath.Join(srcDir, "one.txt")
		if err := os.WriteFile(file, []byte("one"), 0o644); err != nil {
			t.Fatal(err)
		}

		m = typeIntoAttachAddForm(t, m, file)
		m.projectGeneration++

		m, cmd := pressBulkKey(t, m, tea.KeyMsg{Type: tea.KeyCtrlS})
		if m.showAttachAddModal {
			t.Fatal("ctrl+s must still close the form")
		}
		if !strings.Contains(m.statusMsg, "discarded") {
			t.Fatalf("statusMsg = %q, want it to report the discard", m.statusMsg)
		}
		if cmd != nil {
			updated, _ := m.Update(cmd())
			m = updated.(Model)
		}
		if calls := readBdCallsUI(t, recordPath); len(calls) != 0 {
			t.Fatalf("bd calls = %v, want none: the generation mismatch must refuse the attach", calls)
		}
	})

	t.Run("status", func(t *testing.T) {
		m, _, _, recordPath := switchModelWithFakeBd(t)
		m, _ = pressBulkKey(t, m, runeKey("S"))
		if !m.showStatusPicker {
			t.Fatal("S must open the status picker")
		}
		m.projectGeneration++

		m, cmd := pressBulkKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
		if m.showStatusPicker {
			t.Fatal("enter must still close the picker")
		}
		if !strings.Contains(m.statusMsg, "discarded") {
			t.Fatalf("statusMsg = %q, want it to report the discard", m.statusMsg)
		}
		if cmd != nil {
			updated, _ := m.Update(cmd())
			m = updated.(Model)
		}
		if calls := readBdCallsUI(t, recordPath); len(calls) != 0 {
			t.Fatalf("bd calls = %v, want none: the generation mismatch must refuse the status change", calls)
		}
	})

	t.Run("close", func(t *testing.T) {
		m, _, _, recordPath := switchModelWithFakeBd(t)
		m, _ = pressBulkKey(t, m, runeKey("K"))
		if m.issueConfirm.action != issueConfirmClose {
			t.Fatal("K must open the close confirmation")
		}
		m.projectGeneration++

		m, cmd := pressBulkKey(t, m, runeKey("y"))
		if m.issueConfirm.action != issueConfirmNone {
			t.Fatal("y must still close the confirmation")
		}
		if !strings.Contains(m.statusMsg, "discarded") {
			t.Fatalf("statusMsg = %q, want it to report the discard", m.statusMsg)
		}
		if cmd != nil {
			updated, _ := m.Update(cmd())
			m = updated.(Model)
		}
		if calls := readBdCallsUI(t, recordPath); len(calls) != 0 {
			t.Fatalf("bd calls = %v, want none: the generation mismatch must refuse the close", calls)
		}
	})

	t.Run("delete", func(t *testing.T) {
		m, _, _, recordPath := switchModelWithFakeBd(t)
		m, _ = pressBulkKey(t, m, tea.KeyMsg{Type: tea.KeyDelete})
		if m.issueConfirm.action != issueConfirmDelete {
			t.Fatal("delete must open the delete confirmation")
		}
		m.projectGeneration++

		m, cmd := pressBulkKey(t, m, runeKey("y"))
		if m.issueConfirm.action != issueConfirmNone {
			t.Fatal("y must still close the confirmation")
		}
		if !strings.Contains(m.statusMsg, "discarded") {
			t.Fatalf("statusMsg = %q, want it to report the discard", m.statusMsg)
		}
		if cmd != nil {
			updated, _ := m.Update(cmd())
			m = updated.(Model)
		}
		if calls := readBdCallsUI(t, recordPath); len(calls) != 0 {
			t.Fatalf("bd calls = %v, want none: the generation mismatch must refuse the delete", calls)
		}
	})
}

// TestListModeEditSavesAfterProjectSwitch is the positive counterpart to the
// refusal tests above: a list-mode "e" opened with no switch in flight must
// stamp the current (non-zero) generation and let its save through, proving
// handleListKeys' opener is not left behind by a future change to the other
// two edit-modal openers (bd-l66t regression: handleListKeys used to open
// the modal directly and skip the stamp, so every list-mode edit after any
// project switch was refused as stale even though nothing was).
func TestListModeEditSavesAfterProjectSwitch(t *testing.T) {
	m, _, _, recordPath := switchModelWithFakeBd(t)
	m.projectGeneration = 3
	m.focused = focusList

	m, _ = pressBulkKey(t, m, runeKey("e"))
	if !m.showEditModal {
		t.Fatal("list-mode e must open the edit modal")
	}
	if m.editModalGeneration != 3 {
		t.Fatalf("editModalGeneration = %d, want 3: the opener must stamp the current generation", m.editModalGeneration)
	}
	*m.editModal.title = "edited from list mode"

	m, cmd := pressBulkKey(t, m, tea.KeyMsg{Type: tea.KeyCtrlS})
	if m.showEditModal {
		t.Fatal("ctrl+s must close the modal")
	}
	if strings.Contains(m.statusMsg, "discarded") {
		t.Fatalf("statusMsg = %q, want the save to go through: the generation matches", m.statusMsg)
	}
	if cmd != nil {
		updated, _ := m.Update(cmd())
		m = updated.(Model)
	}
	if calls := readBdCallsUI(t, recordPath); len(calls) == 0 {
		t.Fatal("bd calls = none, want the list-mode edit to write through")
	}
}
