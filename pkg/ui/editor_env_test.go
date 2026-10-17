package ui

import "testing"

func TestRemovedEditorActionIgnoresEnvironment(t *testing.T) {
	t.Setenv("VISUAL", "emacsclient -n")
	t.Setenv("EDITOR", "vim")
	m := newK9sKeysModel(t)
	updated, cmd := m.Update(runeMsg("O"))
	m = updated.(Model)
	if cmd != nil || m.statusMsg != "" || m.focused != focusTree {
		t.Fatalf("O must not run an environment editor: cmd=%v status=%q", cmd, m.statusMsg)
	}
}
