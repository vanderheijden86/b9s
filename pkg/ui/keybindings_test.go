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

func TestResolveKeybindingsAcceptsQueryAndAction(t *testing.T) {
	km, errs := ResolveKeybindings(config.Keybindings{
		{Key: "ctrl+t", Query: "type:epic", Description: "Epics"},
		{Key: "ctrl+g", Action: "view_board"},
	})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	b, ok := km.Lookup("ctrl+t")
	if !ok || b.Query != "type:epic" || b.Label() != "Epics" {
		t.Errorf("ctrl+t = %+v ok=%v", b, ok)
	}
	b, ok = km.Lookup("ctrl+g")
	if !ok || b.DefaultKey != "b" {
		t.Errorf("ctrl+g = %+v ok=%v", b, ok)
	}
	if len(km.Bindings()) != 2 {
		t.Errorf("Bindings() = %d, want 2", len(km.Bindings()))
	}
}

func TestResolveKeybindingsRejectsLoudly(t *testing.T) {
	tests := []struct {
		name    string
		binding config.Keybinding
		want    string
	}{
		{"unknown action", config.Keybinding{Key: "ctrl+t", Action: "explode"}, `unknown action "explode"`},
		{"neither action nor query", config.Keybinding{Key: "ctrl+t"}, "needs an action or a query"},
		{"both action and query", config.Keybinding{Key: "ctrl+t", Action: "help", Query: "x"}, "not both"},
		{"missing key", config.Keybinding{Query: "type:epic"}, "needs a key"},
		{"bad key syntax", config.Keybinding{Key: "ctrl+shift+x", Query: "type:epic"}, "not a valid key"},
		{"unknown query field", config.Keybinding{Key: "ctrl+t", Query: "typ:epic"}, `unknown query field "typ"`},
		{"built-in key without override", config.Keybinding{Key: "x", Query: "type:epic"}, "built-in"},
		{"reserved key even with override", config.Keybinding{Key: "esc", Query: "type:epic", Override: true}, "reserved"},
		{"digit is reserved", config.Keybinding{Key: "3", Query: "type:epic", Override: true}, "reserved"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			km, errs := ResolveKeybindings(config.Keybindings{tt.binding})
			if len(errs) != 1 || !strings.Contains(errs[0].Error(), tt.want) {
				t.Fatalf("errs = %v, want one containing %q", errs, tt.want)
			}
			if len(km.Bindings()) != 0 {
				t.Errorf("a rejected binding must not load")
			}
		})
	}
}

func TestResolveKeybindingsOverrideShadowsBuiltIn(t *testing.T) {
	km, errs := ResolveKeybindings(config.Keybindings{{Key: "x", Query: "type:epic", Override: true}})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if _, ok := km.Lookup("x"); !ok {
		t.Error("override binding missing")
	}
}

func TestResolveKeybindingsRejectsDuplicateKeys(t *testing.T) {
	km, errs := ResolveKeybindings(config.Keybindings{
		{Key: "ctrl+t", Query: "type:epic"},
		{Key: "ctrl+t", Query: "type:bug"},
	})
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "duplicate") {
		t.Fatalf("errs = %v, want one duplicate error", errs)
	}
	if b, _ := km.Lookup("ctrl+t"); b.Query != "type:epic" {
		t.Errorf("the first binding must win, got %+v", b)
	}
}

func TestEveryActionDefaultKeyBuildsAKeyMsg(t *testing.T) {
	for _, a := range KeyActions() {
		msg, ok := keyMsgFor(a.Key)
		if !ok {
			t.Errorf("action %s: no key message for %q", a.Name, a.Key)
			continue
		}
		if msg.String() != a.Key {
			t.Errorf("action %s: %q round-trips to %q", a.Name, a.Key, msg.String())
		}
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

func keybindingTestModel(t *testing.T, bindings config.Keybindings) Model {
	t.Helper()
	issues := []model.Issue{
		{ID: "t-1", Title: "Epic one", Status: model.StatusOpen, IssueType: model.TypeEpic, Priority: 1},
		{ID: "t-2", Title: "Bug two", Status: model.StatusOpen, IssueType: model.TypeBug, Priority: 1},
		{ID: "t-3", Title: "Task three", Status: model.StatusOpen, IssueType: model.TypeTask, Priority: 1},
	}
	cfg := config.DefaultConfig()
	cfg.Keybindings = bindings
	m := NewModel(issues, "").WithConfig(cfg, "p", t.TempDir())
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

func TestCustomQueryBindingTogglesSharedQuery(t *testing.T) {
	m := keybindingTestModel(t, config.Keybindings{{Key: "ctrl+t", Query: "type:epic"}})
	m = pressNamedKey(m, "ctrl+t")
	if got := m.QueryText(); got != "type:epic" {
		t.Fatalf("query = %q, want type:epic", got)
	}
	m = pressNamedKey(m, "ctrl+t")
	if got := m.QueryText(); got != "" {
		t.Fatalf("second press must clear the query, got %q", got)
	}
}

func TestCustomActionBindingActsAsDefaultKey(t *testing.T) {
	m := keybindingTestModel(t, config.Keybindings{{Key: "ctrl+g", Action: "view_board"}})
	m = pressNamedKey(m, "ctrl+g")
	if !m.IsBoardView() {
		t.Fatal("ctrl+g bound to view_board should open the board")
	}
}

func TestCustomBindingOverridesBuiltInKey(t *testing.T) {
	m := keybindingTestModel(t, config.Keybindings{{Key: "b", Query: "type:bug", Override: true}})
	m = pressNamedKey(m, "b")
	if m.IsBoardView() {
		t.Error("overridden b must not open the board")
	}
	if got := m.QueryText(); got != "type:bug" {
		t.Errorf("query = %q, want type:bug", got)
	}
}

func TestCustomBindingsAppearInFooterAndHelp(t *testing.T) {
	m := keybindingTestModel(t, config.Keybindings{{Key: "ctrl+t", Query: "type:epic", Description: "Epics"}})
	if footer := m.renderFooter(); !strings.Contains(footer, "Epics") || !strings.Contains(footer, "^t") {
		t.Errorf("footer misses the custom hint: %q", footer)
	}
	m = pressNamedKey(m, "?")
	if help := m.View(); !strings.Contains(help, "Custom") || !strings.Contains(help, "Epics") {
		t.Errorf("help overlay misses the custom section")
	}
}

func TestInvalidBindingsLoadNothingButKeepErrors(t *testing.T) {
	m := keybindingTestModel(t, config.Keybindings{{Key: "ctrl+t", Action: "explode"}})
	if len(m.KeybindingErrors()) != 1 {
		t.Fatalf("KeybindingErrors() = %v", m.KeybindingErrors())
	}
	m = pressNamedKey(m, "ctrl+t")
	if m.QueryText() != "" {
		t.Error("a rejected binding must do nothing")
	}
}
