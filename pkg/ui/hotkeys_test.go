package ui

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vanderheijden86/b9s/pkg/config"
	"github.com/vanderheijden86/b9s/pkg/model"
)

func TestShortCutSpellsKeysAsK9sDoes(t *testing.T) {
	for in, want := range map[string]string{
		"F1": "f1", "f12": "f12", "Ctrl-T": "ctrl+t", "ctrl-t": "ctrl+t", "Alt-x": "alt+x",
		"Shift-E": "E", "Shift-0": ")", "Shift-1": "!", "x": "x", "E": "E", "|": "|",
	} {
		got, err := keyForShortCut(in)
		if err != nil || got != want {
			t.Errorf("keyForShortCut(%q) = %q, %v, want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", " ", "F13", "Ctrl-1", "Ctrl-Shift-T", "Shift-;", "ctrl+t", "Hyper-x"} {
		if got, err := keyForShortCut(in); err == nil {
			t.Errorf("keyForShortCut(%q) = %q, want an error", in, got)
		}
	}
}

func TestResolveHotkeysRunsCommands(t *testing.T) {
	hk, errs := ResolveHotkeys(config.Hotkeys{
		{Name: "open-epics", ShortCut: "Ctrl-T", Description: "Open epics", Command: "epic status:open"},
		{Name: "layout", ShortCut: "Ctrl-G", Command: "layout"},
	})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	h, ok := hk.Lookup("ctrl+t")
	if !ok || h.Label() != "Open epics" || h.command.Query != "status:open" || h.command.IssueType != model.TypeEpic {
		t.Errorf("ctrl+t = %+v ok=%v", h, ok)
	}
	if h, _ := hk.Lookup("ctrl+g"); h.Label() != "layout" {
		t.Errorf("a hotkey without a description is labelled by its command, got %q", h.Label())
	}
	if len(hk.All()) != 2 {
		t.Errorf("All() = %d, want 2", len(hk.All()))
	}
}

func TestResolveHotkeysRejectsLoudly(t *testing.T) {
	tests := []struct {
		name   string
		hotkey config.Hotkey
		want   string
	}{
		{"unknown command", config.Hotkey{ShortCut: "Ctrl-T", Command: "explode"}, "unknown command: explode"},
		{"no command", config.Hotkey{ShortCut: "Ctrl-T"}, "needs a command"},
		{"no shortCut", config.Hotkey{Command: "epic"}, "needs a shortCut"},
		{"bad shortCut", config.Hotkey{ShortCut: "Ctrl-Shift-X", Command: "epic"}, "not a valid shortCut"},
		{"unknown query field", config.Hotkey{ShortCut: "Ctrl-T", Command: "epic typ:x"}, `unknown query field "typ"`},
		{"args on a command without them", config.Hotkey{ShortCut: "Ctrl-T", Command: "mouse on"}, "takes no arguments"},
		{"built-in key without override", config.Hotkey{ShortCut: "x", Command: "epic"}, "built-in"},
		{"reserved key even with override", config.Hotkey{ShortCut: "Shift-G", Command: "epic", Override: true}, "reserved"},
		{"digit is reserved", config.Hotkey{ShortCut: "3", Command: "epic", Override: true}, "reserved"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.hotkey.Name = "h"
			hk, errs := ResolveHotkeys(config.Hotkeys{tt.hotkey})
			if len(errs) != 1 || !strings.Contains(errs[0].Error(), tt.want) || !strings.Contains(errs[0].Error(), `hotKeys.h`) {
				t.Fatalf("errs = %v, want one naming hotKeys.h and containing %q", errs, tt.want)
			}
			if len(hk.All()) != 0 {
				t.Errorf("a rejected hotkey must not load")
			}
		})
	}
}

func TestResolveHotkeysOverrideShadowsBuiltIn(t *testing.T) {
	hk, errs := ResolveHotkeys(config.Hotkeys{{Name: "e", ShortCut: "F1", Command: "bug", Override: true}})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if _, ok := hk.Lookup("f1"); !ok {
		t.Error("override hotkey missing")
	}
}

func TestResolveHotkeysRejectsTwoNamesOnOneKey(t *testing.T) {
	hk, errs := ResolveHotkeys(config.Hotkeys{
		{Name: "a", ShortCut: "Ctrl-T", Command: "epic"},
		{Name: "b", ShortCut: "ctrl-t", Command: "bug"},
	})
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), `already bound by hotKeys.a`) {
		t.Fatalf("errs = %v, want one duplicate error", errs)
	}
	if h, _ := hk.Lookup("ctrl+t"); h.Name != "a" {
		t.Errorf("the first hotkey must win, got %+v", h)
	}
}

// Guard: a key handled by a built-in switch must be known to the collision
// check, so a user binding can never shadow it without override: true.
func TestEveryHandledKeyIsKnownToCollisionCheck(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	literal := regexp.MustCompile(`(?:case\s+|String\(\)\s*==\s*)((?:"[^"]*"(?:\s*,\s*)?)+)`)
	quoted := regexp.MustCompile(`"([^"]*)"`)
	keyLike := regexp.MustCompile(`^(.|ctrl\+.+|alt\+.+|f[0-9]+|shift\+tab|enter|esc|tab|space|backspace|delete|up|down|left|right|home|end|pgup|pgdown)$`)
	var missing []string
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range literal.FindAllStringSubmatch(string(src), -1) {
			for _, q := range quoted.FindAllStringSubmatch(m[1], -1) {
				k, err := strconv.Unquote(q[0])
				if err != nil || k == "" || k == " " || !keyLike.MatchString(k) || isKnownKey(k) {
					continue
				}
				missing = append(missing, f+": "+k)
			}
		}
	}
	if len(missing) > 0 {
		t.Errorf("keys handled in the UI but absent from builtinKeys or reservedKeys in keybindings.go:\n%s", strings.Join(missing, "\n"))
	}
}

func keybindingTestModel(t *testing.T, hotkeys config.Hotkeys) Model {
	t.Helper()
	issues := []model.Issue{
		{ID: "t-1", Title: "Epic one", Status: model.StatusOpen, IssueType: model.TypeEpic, Priority: 1},
		{ID: "t-2", Title: "Bug two", Status: model.StatusOpen, IssueType: model.TypeBug, Priority: 1},
		{ID: "t-3", Title: "Task three", Status: model.StatusOpen, IssueType: model.TypeTask, Priority: 1},
	}
	resolved, errs := ResolveHotkeys(hotkeys)
	if len(errs) != 0 {
		t.Fatalf("hotkeys: %v", errs)
	}
	m := NewModel(issues, "").WithConfig(config.DefaultConfig(), "p", t.TempDir()).WithHotkeys(resolved)
	m.width, m.height = 120, 40
	return m
}

func pressNamedKey(m Model, key string) Model {
	msg, ok := keyMsgFor(key)
	if !ok {
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
	updated, _ := m.Update(msg)
	return updated.(Model)
}

// A hotkey runs its : command, as a k9s hotkey does. With query terms the
// command replaces the shared query.
func TestHotkeyRunsItsCommand(t *testing.T) {
	m := keybindingTestModel(t, config.Hotkeys{{Name: "oe", ShortCut: "Ctrl-T", Command: "epic status:open"}})
	m.setQueryText("label:ui")
	m.queryState.Accept()
	m = pressNamedKey(m, "ctrl+t")
	if got := m.QueryText(); got != "status:open type:epic" {
		t.Fatalf("query = %q, want status:open type:epic", got)
	}
	if n := len(m.FilteredIssues()); n != 1 {
		t.Errorf("shows %d issues, want the one open epic", n)
	}
}

func TestHotkeyOverridesBuiltInKey(t *testing.T) {
	m := keybindingTestModel(t, config.Hotkeys{{Name: "bugs", ShortCut: "b", Command: "bug", Override: true}})
	m = pressNamedKey(m, "b")
	if m.IsBoardView() {
		t.Error("overridden b must not open the board")
	}
	if got := m.QueryText(); got != "type:bug" {
		t.Errorf("query = %q, want type:bug", got)
	}
}

func TestHotkeyOverridesFunctionKey(t *testing.T) {
	m := keybindingTestModel(t, config.Hotkeys{{Name: "chores", ShortCut: "F1", Command: "chore", Override: true}})
	m = pressFKey(m, 1)
	if got := m.QueryText(); got != "type:chore" {
		t.Errorf("query = %q, want type:chore", got)
	}
}

func TestHotkeysAppearInFooterAndHelp(t *testing.T) {
	m := keybindingTestModel(t, config.Hotkeys{{Name: "oe", ShortCut: "Ctrl-T", Command: "epic status:open", Description: "Open epics"}})
	if footer := m.renderFooter(); !strings.Contains(footer, "Open epics") || !strings.Contains(footer, "^t") {
		t.Errorf("footer misses the hotkey hint: %q", footer)
	}
	m = pressNamedKey(m, "?")
	if help := m.View(); !strings.Contains(help, "Hotkeys") || !strings.Contains(help, "Open epics") || !strings.Contains(help, ":epic") {
		t.Errorf("help overlay misses the hotkeys section:\n%s", stripANSI(help))
	}
}
