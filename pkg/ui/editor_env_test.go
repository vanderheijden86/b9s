package ui

import (
	"slices"
	"strings"
	"testing"
)

// O runs the user's own editor, read from $VISUAL and then $EDITOR as git
// reads them, with any arguments the variable carries.
func TestEnvironmentEditorReadsVisualThenEditor(t *testing.T) {
	for _, tc := range []struct {
		visual, editor string
		want           []string
	}{
		{"", "emacsclient -n", []string{"emacsclient", "-n"}},
		{"zed --wait", "vim", []string{"zed", "--wait"}},
		{"", "vim", []string{"vim"}},
		{"", "/opt/homebrew/bin/hx", []string{"/opt/homebrew/bin/hx"}},
	} {
		t.Setenv("VISUAL", tc.visual)
		t.Setenv("EDITOR", tc.editor)
		args, ok, err := environmentEditor()
		if err != nil || !ok {
			t.Fatalf("VISUAL=%q EDITOR=%q: ok=%v err=%v", tc.visual, tc.editor, ok, err)
		}
		if !slices.Equal(args, tc.want) {
			t.Errorf("VISUAL=%q EDITOR=%q: args = %q, want %q", tc.visual, tc.editor, args, tc.want)
		}
	}
}

func TestEnvironmentEditorUnsetFallsBackToPlatformDefault(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	if _, ok, err := environmentEditor(); ok || err != nil {
		t.Fatalf("unset editor: ok=%v err=%v, want the platform default", ok, err)
	}
}

func TestEnvironmentEditorRefusesShells(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "bash -c")
	if _, _, err := environmentEditor(); err == nil {
		t.Fatal("a shell as $EDITOR must be refused")
	}
}

// Outside test mode, O hands the editor to Bubble Tea, which suspends the TUI
// while it runs, so a terminal editor such as vim gets the terminal.
func TestOKeyRunsTheEnvironmentEditor(t *testing.T) {
	t.Setenv("B9S_TEST_MODE", "")
	t.Setenv("TMPDIR", t.TempDir())
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "emacsclient -n")
	m := detailFooterModel(t, "list")
	m.focused = focusList
	want := m.getSelectedIssue()

	updated, cmd := m.Update(runeKey("O"))
	m = updated.(Model)

	if cmd == nil {
		t.Fatal("O returned no command to run the editor")
	}
	if m.statusIsError || !strings.Contains(m.statusMsg, "emacsclient") {
		t.Fatalf("status %q (error %v), want it to name emacsclient", m.statusMsg, m.statusIsError)
	}

	updated, _ = m.Update(editorFinishedMsg{id: want.ID, editor: "emacsclient"})
	m = updated.(Model)
	if m.statusIsError {
		t.Fatalf("a clean editor exit reported an error: %q", m.statusMsg)
	}
}

func TestOKeyNamesTheEditorInTestMode(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "vim")
	m := detailFooterModel(t, "list")
	m.focused = focusList
	m = pressKeys(m, runeKey("O"))
	if !strings.Contains(m.statusMsg, "Would open") || !strings.Contains(m.statusMsg, "vim") {
		t.Fatalf("status %q, want it to name vim", m.statusMsg)
	}
}
