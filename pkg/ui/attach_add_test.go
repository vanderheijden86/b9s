package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanderheijden86/b9s/internal/attachref"
	"github.com/vanderheijden86/b9s/internal/datasource"
	"github.com/vanderheijden86/b9s/pkg/config"
	"github.com/vanderheijden86/b9s/pkg/model"
)

// writeFakeBdRecorder installs a shell script named bd that appends every
// invocation's args to recordPath, following cmd/b9s/attach_test.go's
// separator scheme: RS (\036) within one call, GS (\035) between calls,
// since a reference comment's machine line contains a real newline and so
// cannot itself be used as a separator.
func writeFakeBdRecorder(t *testing.T, recordPath string, exitCode int) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake bd script needs a POSIX shell")
	}
	binDir := t.TempDir()
	script := "#!/bin/sh\n" +
		"{\n" +
		"  for a in \"$@\"; do printf '%s\\036' \"$a\"; done\n" +
		"  printf '\\035'\n" +
		"} >> " + shellQuoteUI(recordPath) + "\n" +
		"exit " + strconv.Itoa(exitCode) + "\n"
	if err := os.WriteFile(filepath.Join(binDir, "bd"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(binDir, "bd")
}

func shellQuoteUI(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// readBdCallsUI parses recordPath into one []string per bd invocation, in order.
func readBdCallsUI(t *testing.T, recordPath string) [][]string {
	t.Helper()
	data, err := os.ReadFile(recordPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	var calls [][]string
	for _, rawCall := range strings.Split(string(data), "\035") {
		if rawCall == "" {
			continue
		}
		args := strings.Split(rawCall, "\036")
		if len(args) > 0 && args[len(args)-1] == "" {
			args = args[:len(args)-1]
		}
		calls = append(calls, args)
	}
	return calls
}

// attachAddTestModel builds a Model over one issue ("bd-1"), wired to a
// project directory with a real .beads directory, an attachments.local_dir
// config when configured is true, and an IssueWriter bound to a fake bd
// script that records every invocation to recordPath.
func attachAddTestModel(t *testing.T, configured bool, bdExitCode int) (m Model, recordPath, localDir string) {
	t.Helper()
	projectDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(projectDir, ".beads"), 0o700); err != nil {
		t.Fatal(err)
	}
	localDir = filepath.Join(t.TempDir(), "blobs")

	xdgHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdgHome)
	if configured {
		cfgDir := filepath.Join(xdgHome, "b9s")
		if err := os.MkdirAll(cfgDir, 0o755); err != nil {
			t.Fatal(err)
		}
		yaml := fmt.Sprintf("attachments:\n  backend: local\n  local_dir: %q\n", localDir)
		if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(yaml), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	recordPath = filepath.Join(t.TempDir(), "bd-calls")
	fakeBdPath := writeFakeBdRecorder(t, recordPath, bdExitCode)

	m = NewModel([]model.Issue{
		{ID: "bd-1", Title: "Has attachments", Status: model.StatusOpen, IssueType: model.TypeTask, CreatedAt: time.Now()},
	}, "")
	m = m.WithConfig(config.Config{}, "proj", projectDir)
	m = m.WithSourceType(datasource.SourceTypeJSONLLocal)
	checkout, ok := NewCheckout(projectDir)
	if !ok {
		t.Fatalf("NewCheckout(%q) rejected a directory with .beads", projectDir)
	}
	m.issueWriter = &IssueWriter{bdPath: fakeBdPath, available: true, checkout: checkout}
	m.tree.SelectByID("bd-1")
	return m, recordPath, localDir
}

// typeIntoAttachAddForm opens the form with "I" and types raw into it, one
// rune at a time, the way TestCreateModal_TypedTitleVisibleInBuildCreateArgs
// exercises the edit modal's own huh field bindings.
func typeIntoAttachAddForm(t *testing.T, m Model, raw string) Model {
	t.Helper()
	m, _ = pressBulkKey(t, m, runeKey("I"))
	if !m.showAttachAddModal {
		t.Fatal("I must open the attach-files form")
	}
	for _, r := range raw {
		m, _ = pressBulkKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return m
}

// submitAttachAddForm opens the form, types raw, presses ctrl+s, and runs
// the resulting tea.Cmd synchronously, feeding its result back into Update
// the same way the real event loop would (mirrors runAttachmentOpenCmd in
// attachments_test.go).
func submitAttachAddForm(t *testing.T, m Model, raw string) Model {
	t.Helper()
	m = typeIntoAttachAddForm(t, m, raw)
	m, cmd := pressBulkKey(t, m, tea.KeyMsg{Type: tea.KeyCtrlS})
	if m.showAttachAddModal {
		t.Fatal("ctrl+s must close the attach-files form")
	}
	if cmd == nil {
		t.Fatal("ctrl+s with a non-empty path list must dispatch a Cmd")
	}
	updated, _ := m.Update(cmd())
	return updated.(Model)
}

func TestAttachAddKeyOpensFormAndEscCloses(t *testing.T) {
	m, _, _ := attachAddTestModel(t, true, 0)

	m, _ = pressBulkKey(t, m, runeKey("I"))
	if !m.showAttachAddModal {
		t.Fatal("I must open the attach-files form")
	}

	m, cmd := pressBulkKey(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.showAttachAddModal {
		t.Fatal("esc must close the attach-files form")
	}
	if cmd != nil {
		t.Fatal("esc must not dispatch a submit command")
	}
}

func TestAttachAddSubmitTwoFilesRecordsTwoBdCommentsAdd(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	m, recordPath, _ := attachAddTestModel(t, true, 0)

	srcDir := t.TempDir()
	file1 := filepath.Join(srcDir, "one.txt")
	file2 := filepath.Join(srcDir, "two.txt")
	if err := os.WriteFile(file1, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file2, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}

	m = submitAttachAddForm(t, m, file1+" "+file2)

	if m.statusIsError {
		t.Fatalf("expected a non-error status after two successful attaches, got %q", m.statusMsg)
	}
	if !strings.Contains(m.statusMsg, "2 attached") {
		t.Fatalf("statusMsg = %q, want it to report 2 attached", m.statusMsg)
	}

	calls := readBdCallsUI(t, recordPath)
	var commentAdds [][]string
	for _, c := range calls {
		if len(c) >= 2 && c[0] == "comments" && c[1] == "add" {
			commentAdds = append(commentAdds, c)
		}
	}
	if len(commentAdds) != 2 {
		t.Fatalf("expected 2 `bd comments add` calls, got %d: %v", len(commentAdds), calls)
	}
	for _, c := range commentAdds {
		if len(c) != 4 || c[2] != "bd-1" {
			t.Fatalf("bd comments add call = %v, want [comments add bd-1 <text>]", c)
		}
		refs, _ := attachref.Parse(c[3])
		if len(refs) != 1 {
			t.Fatalf("attachref.Parse(%q) found %d refs, want 1", c[3], len(refs))
		}
	}
}

func TestAttachAddMissingFileFailsOtherStillAttaches(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	m, recordPath, _ := attachAddTestModel(t, true, 0)

	srcDir := t.TempDir()
	present := filepath.Join(srcDir, "present.txt")
	if err := os.WriteFile(present, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(srcDir, "missing.txt")

	m = submitAttachAddForm(t, m, missing+" "+present)

	if !m.statusIsError {
		t.Fatalf("expected an error status when one of two files is missing, got %q", m.statusMsg)
	}
	if !strings.Contains(m.statusMsg, "1 attached") || !strings.Contains(m.statusMsg, "1 failed") {
		t.Fatalf("statusMsg = %q, want it to report 1 attached and 1 failed", m.statusMsg)
	}

	calls := readBdCallsUI(t, recordPath)
	var commentAdds int
	for _, c := range calls {
		if len(c) >= 2 && c[0] == "comments" && c[1] == "add" {
			commentAdds++
		}
	}
	if commentAdds != 1 {
		t.Fatalf("expected 1 `bd comments add` call for the file that exists, got %d: %v", commentAdds, calls)
	}
}

func TestAttachAddWithoutAttachmentsConfigShowsHint(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	m, _, _ := attachAddTestModel(t, false, 0)

	srcDir := t.TempDir()
	file := filepath.Join(srcDir, "notes.txt")
	if err := os.WriteFile(file, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	m = submitAttachAddForm(t, m, file)

	if !m.statusIsError {
		t.Fatalf("expected an error status, got %q", m.statusMsg)
	}
	if m.statusMsg != notConfiguredHint {
		t.Fatalf("statusMsg = %q, want %q", m.statusMsg, notConfiguredHint)
	}
}

func TestAttachAddKeyInAllProjectsModeRefusesWithoutOpeningForm(t *testing.T) {
	m, _, _ := attachAddTestModel(t, true, 0)
	m.allProjectsMode = true

	m, _ = pressBulkKey(t, m, runeKey("I"))

	if m.showAttachAddModal {
		t.Fatal("I in all-projects mode must not open the attach-files form")
	}
	if m.statusIsError {
		t.Fatalf("expected a non-error status message, got isError=true (%q)", m.statusMsg)
	}
	if !strings.Contains(m.statusMsg, "all-projects mode") {
		t.Fatalf("statusMsg = %q, want it to mention all-projects mode", m.statusMsg)
	}
}

// TestAttachAddKeyInLabelPickerGoesToInputNotForm mirrors
// TestRInLabelPickerGoesToInputNotAttachmentPicker in attachments_test.go: I
// must not steal the letter from the label picker's fuzzy-search input.
func TestAttachAddKeyInLabelPickerGoesToInputNotForm(t *testing.T) {
	m, _, _ := attachAddTestModel(t, true, 0)
	m.labelPicker.SetLabels([]string{"bug", "chore"}, map[string]int{"bug": 1, "chore": 1})
	m.showLabelPicker = true
	m.focused = focusLabelPicker

	m, _ = pressBulkKey(t, m, runeKey("I"))

	if m.showAttachAddModal {
		t.Fatal("I while the label picker is focused must not open the attach-files form")
	}
	if got := m.labelPicker.InputValue(); got != "I" {
		t.Fatalf("labelPicker.InputValue() = %q, want %q (I must reach the fuzzy-search input)", got, "I")
	}
}

// TestAttachAddResultDroppedAfterProjectSwitch is the staleness guard
// attachmentOpenResultMsg also applies: a :project switch that lands between
// dispatch and the result arriving must not apply a result naming the wrong
// project's blob store or overwrite the new project's footer message.
func TestAttachAddResultDroppedAfterProjectSwitch(t *testing.T) {
	m, _, _ := attachAddTestModel(t, true, 0)
	dispatchedFrom := m.activeProjectPath

	m.activeProjectPath = "/some/other/project"
	m.statusMsg = "unrelated status from the new project"
	m.statusIsError = false

	updated, _ := m.Update(attachAddResultMsg{
		dispatchProjectPath: dispatchedFrom,
		issueID:             "bd-1",
		statusMsg:           "3 attached",
		reload:              true,
	})
	m = updated.(Model)

	if m.statusMsg != "unrelated status from the new project" {
		t.Fatalf("statusMsg = %q, want the stale result dropped and the new project's message left alone", m.statusMsg)
	}
	if m.attachHandle != nil {
		t.Fatal("a stale result must not populate attachHandle")
	}
}

func TestParseAttachPaths(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory available")
	}
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{name: "empty", raw: "", want: nil},
		{name: "blank lines skipped", raw: "\n \n\t\n", want: nil},
		{name: "one per line", raw: "/a/b.txt\n/c/d.txt", want: []string{"/a/b.txt", "/c/d.txt"}},
		{name: "space separated", raw: "/a/b.txt /c/d.txt", want: []string{"/a/b.txt", "/c/d.txt"}},
		{name: "double-quoted path with space", raw: `"/a/my file.txt" /c/d.txt`, want: []string{"/a/my file.txt", "/c/d.txt"}},
		{name: "single-quoted path with space", raw: `'/a/my file.txt'`, want: []string{"/a/my file.txt"}},
		{name: "tilde expands", raw: "~/notes.txt", want: []string{filepath.Join(home, "notes.txt")}},
		{name: "bare tilde expands", raw: "~", want: []string{home}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseAttachPaths(tc.raw)
			if err != nil {
				t.Fatalf("parseAttachPaths(%q) error: %v", tc.raw, err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("parseAttachPaths(%q) = %v, want %v", tc.raw, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("parseAttachPaths(%q)[%d] = %q, want %q", tc.raw, i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestParseAttachPathsUnterminatedQuoteErrors(t *testing.T) {
	if _, err := parseAttachPaths(`"/a/unterminated`); err == nil {
		t.Fatal("expected an error for an unterminated quote")
	}
}

// TestParseAttachPathsCapsAtMax guards the 50-path cap (bd-t8j5.10 review):
// each accepted file runs its own hash/upload/bd-comments-add against a
// single attachAddTimeout deadline, so an unbounded list must be rejected
// with a clear error rather than accepted and left to run long.
func TestParseAttachPathsCapsAtMax(t *testing.T) {
	var raw strings.Builder
	for i := 0; i < maxAttachPaths+1; i++ {
		fmt.Fprintf(&raw, "/tmp/file%d.txt\n", i)
	}

	_, err := parseAttachPaths(raw.String())
	if err == nil {
		t.Fatal("expected an error for 51 paths (cap is 50)")
	}
	if !strings.Contains(err.Error(), "50") {
		t.Errorf("error = %v, want it to name the 50-path cap", err)
	}
}

func TestParseAttachPathsAtMaxSucceeds(t *testing.T) {
	var raw strings.Builder
	for i := 0; i < maxAttachPaths; i++ {
		fmt.Fprintf(&raw, "/tmp/file%d.txt\n", i)
	}

	got, err := parseAttachPaths(raw.String())
	if err != nil {
		t.Fatalf("parseAttachPaths with exactly %d paths: %v", maxAttachPaths, err)
	}
	if len(got) != maxAttachPaths {
		t.Fatalf("got %d paths, want %d", len(got), maxAttachPaths)
	}
}

func TestResolveAttachPaths(t *testing.T) {
	tests := []struct {
		name    string
		paths   []string
		baseDir string
		want    []string
	}{
		{
			name:    "relative path resolves against baseDir",
			paths:   []string{"notes.txt", "sub/dir/file.txt"},
			baseDir: "/proj",
			want:    []string{"/proj/notes.txt", "/proj/sub/dir/file.txt"},
		},
		{
			name:    "absolute path is left unchanged",
			paths:   []string{"/elsewhere/file.txt"},
			baseDir: "/proj",
			want:    []string{"/elsewhere/file.txt"},
		},
		{
			name:    "empty baseDir leaves every path unchanged",
			paths:   []string{"notes.txt", "/abs/file.txt"},
			baseDir: "",
			want:    []string{"notes.txt", "/abs/file.txt"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveAttachPaths(tc.paths, tc.baseDir)
			if len(got) != len(tc.want) {
				t.Fatalf("resolveAttachPaths(%v, %q) = %v, want %v", tc.paths, tc.baseDir, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("resolveAttachPaths(%v, %q)[%d] = %q, want %q", tc.paths, tc.baseDir, i, got[i], tc.want[i])
				}
			}
		})
	}
}

// TestAttachAddSubmitRelativePathResolvesAgainstProjectDir is the
// through-the-model regression test for relative-path resolution: a relative
// path typed into the form must resolve against the project's checkout
// directory (the same directory bd runs in), not the test process's cwd.
func TestAttachAddSubmitRelativePathResolvesAgainstProjectDir(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	m, recordPath, _ := attachAddTestModel(t, true, 0)

	relName := "relative-notes.txt"
	fullPath := filepath.Join(m.activeProjectPath, relName)
	if err := os.WriteFile(fullPath, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	m = submitAttachAddForm(t, m, relName)

	if m.statusIsError {
		t.Fatalf("expected a successful attach of a project-relative path, got %q", m.statusMsg)
	}
	calls := readBdCallsUI(t, recordPath)
	var commentAdds int
	for _, c := range calls {
		if len(c) >= 2 && c[0] == "comments" && c[1] == "add" {
			commentAdds++
		}
	}
	if commentAdds != 1 {
		t.Fatalf("expected 1 `bd comments add` call for the project-relative path, got %d: %v", commentAdds, calls)
	}
}
