package ui

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vanderheijden86/b9s/pkg/config"
)

// KeyAction is a built-in action a user binding can name. It resolves to the
// key b9s already binds to it, so the action behaves exactly as that key does
// in the view the user is in (ADR 0033).
type KeyAction struct {
	Name        string
	Key         string
	Description string
}

var keyActions = []KeyAction{
	{"help", "?", "Show the help overlay"},
	{"search", "/", "Open the search bar"},
	{"command_prompt", ":", "Open the command prompt"},
	{"view_tree", "t", "Tree view, or flat list and back"},
	{"view_board", "b", "Board view"},
	{"view_graph", "g", "Dependency graph of the cursor"},
	{"filter_open", "o", "Show open issues"},
	{"filter_closed", "C", "Show closed issues"},
	{"filter_ready", "r", "Show ready issues"},
	{"filter_all", "a", "Show all issues, clearing quick filters"},
	{"sort", "s", "Sort"},
	{"status", "S", "Change status"},
	{"edit", "e", "Edit the issue"},
	{"close_issue", "K", "Close the issue"},
	{"refresh", "ctrl+r", "Reload"},
	{"project_picker", "P", "Show or hide the project picker"},
	{"label_picker", "L", "Label quick filter"},
	{"assignee_picker", "A", "Assignee quick filter"},
	{"type_epic", "f1", "Show only epics"},
	{"type_feature", "f2", "Show only features"},
	{"type_task", "f3", "Show only tasks"},
	{"type_bug", "f4", "Show only bugs"},
	{"columns", "|", "Choose columns"},
	{"copy", "c", "Copy the ID and title"},
	{"branch", "f", "Show only the cursor's branch"},
	{"quit", "q", "Quit"},
}

// KeyActions lists the action names a keybinding may use.
func KeyActions() []KeyAction {
	return slices.Clone(keyActions)
}

// reservedKeys never accept a user binding, even with override: true. They
// leave the program (ctrl+c), back out of a state (esc), carry the text-entry
// and navigation contract every view relies on, or stand for an index (digits).
var reservedKeys = []string{
	"ctrl+c", "esc", "enter", "tab", "shift+tab", "backspace", "space",
	"up", "down", "left", "right", "pgup", "pgdown", "home", "end",
	"j", "k", "h", "l", "G",
	"/", ":", "?", "`",
	"0", "1", "2", "3", "4", "5", "6", "7", "8", "9",
}

// builtinKeys are the other keys some view handles. A user binding on one needs
// override: true. A test scans the UI source so a newly handled key cannot be
// left off this list.
var builtinKeys = []string{
	"a", "A", "b", "B", "c", "C", "d", "D", "e", "E", "f", "F", "g", "H", "i", "I",
	"J", "K", "L", "m", "M", "n", "N", "o", "O", "p", "P", "q", "r", "R", "s", "S",
	"t", "T", "u", "U", "v", "V", "w", "W", "x", "X", "y", "Y", "z", "Z",
	"[", "]", "{", "}", "\\", "<", ">", "|", "$", "delete",
	"ctrl+a", "ctrl+b", "ctrl+d", "ctrl+e", "ctrl+f", "ctrl+n", "ctrl+p", "ctrl+r",
	"ctrl+o", "ctrl+s", "ctrl+u", "ctrl+w", "ctrl+\\", "ctrl+@", "f1", "f2", "f3", "f4", "f5",
}

func isKnownKey(key string) bool {
	return slices.Contains(reservedKeys, key) || slices.Contains(builtinKeys, key)
}

var userKeySyntax = regexp.MustCompile(`^(\S|ctrl\+[a-z]|alt\+\S|f([1-9]|1[0-2]))$`)

// ValidKeyName reports whether name is a key a binding can name: one printable
// character, ctrl+<letter>, alt+<character> or f1 to f12.
func ValidKeyName(name string) bool {
	if name == "" {
		return false
	}
	if len([]rune(name)) == 1 {
		return name != " "
	}
	return userKeySyntax.MatchString(name)
}

// keyMsgFor builds the key message that stringifies to key. It covers the
// forms the action table uses: one character, or ctrl+<letter>.
var functionKeyTypes = []tea.KeyType{
	tea.KeyF1, tea.KeyF2, tea.KeyF3, tea.KeyF4, tea.KeyF5, tea.KeyF6,
	tea.KeyF7, tea.KeyF8, tea.KeyF9, tea.KeyF10, tea.KeyF11, tea.KeyF12,
}

func keyMsgFor(key string) (tea.KeyMsg, bool) {
	if strings.HasPrefix(key, "ctrl+") && len(key) == len("ctrl+")+1 {
		letter := key[len("ctrl+")]
		if letter >= 'a' && letter <= 'z' {
			return tea.KeyMsg{Type: tea.KeyType(1 + letter - 'a')}, true
		}
		return tea.KeyMsg{}, false
	}
	if n, err := strconv.Atoi(strings.TrimPrefix(key, "f")); err == nil && strings.HasPrefix(key, "f") && n >= 1 && n <= len(functionKeyTypes) {
		return tea.KeyMsg{Type: functionKeyTypes[n-1]}, true
	}
	if runes := []rune(key); len(runes) == 1 {
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: runes}, true
	}
	return tea.KeyMsg{}, false
}

// CustomBinding is a validated user shortcut.
type CustomBinding struct {
	Key         string
	Action      string
	Query       string
	Description string
	// DefaultKey is the built-in key an action binding stands for.
	DefaultKey string
}

// Label is the text the footer and the help overlay show for the binding.
func (b CustomBinding) Label() string {
	switch {
	case b.Description != "":
		return b.Description
	case b.Query != "":
		return b.Query
	default:
		return b.Action
	}
}

// Keymap holds the validated user bindings in file order.
type Keymap struct {
	bindings []CustomBinding
}

// Lookup returns the binding for a key name as tea.KeyMsg.String reports it.
func (k Keymap) Lookup(key string) (CustomBinding, bool) {
	for _, b := range k.bindings {
		if b.Key == key {
			return b, true
		}
	}
	return CustomBinding{}, false
}

// Bindings returns the validated bindings in file order.
func (k Keymap) Bindings() []CustomBinding {
	return slices.Clone(k.bindings)
}

// ResolveKeybindings validates the configured bindings. A binding that breaks a
// rule is dropped and reported, so a typo never produces a silently different
// shortcut: the caller shows every returned error before anything else runs.
func ResolveKeybindings(in config.Keybindings) (Keymap, []error) {
	var km Keymap
	var errs []error
	seen := map[string]bool{}
	for i, b := range in {
		fail := func(format string, args ...any) {
			errs = append(errs, fmt.Errorf("keybindings[%d] (key %q): %s", i, b.Key, fmt.Sprintf(format, args...)))
		}
		if b.Key == "" {
			fail("needs a key")
			continue
		}
		if slices.Contains(reservedKeys, b.Key) {
			fail("%q is reserved and cannot be rebound", b.Key)
			continue
		}
		if !ValidKeyName(b.Key) {
			fail("%q is not a valid key: use one character, ctrl+<letter>, alt+<character> or f1 to f12", b.Key)
			continue
		}
		resolved := CustomBinding{Key: b.Key, Action: b.Action, Query: strings.TrimSpace(b.Query), Description: b.Description}
		switch {
		case b.Action != "" && resolved.Query != "":
			fail("set an action or a query, not both")
			continue
		case b.Action == "" && resolved.Query == "":
			fail("needs an action or a query")
			continue
		case b.Action != "":
			idx := slices.IndexFunc(keyActions, func(a KeyAction) bool { return a.Name == b.Action })
			if idx < 0 {
				fail("unknown action %q, want one of %s", b.Action, strings.Join(actionNames(), ", "))
				continue
			}
			resolved.DefaultKey = keyActions[idx].Key
		default:
			if field, ok := unknownQueryField(resolved.Query); ok {
				fail("unknown query field %q, want one of %s", field, strings.Join(queryFieldNames(), ", "))
				continue
			}
		}
		if slices.Contains(builtinKeys, b.Key) && !b.Override {
			fail("%q is a built-in key: set override: true to replace it, or pick a free key such as ctrl+<letter>", b.Key)
			continue
		}
		if seen[b.Key] {
			fail("duplicate binding for %q", b.Key)
			continue
		}
		seen[b.Key] = true
		km.bindings = append(km.bindings, resolved)
	}
	return km, errs
}

func actionNames() []string {
	names := make([]string, len(keyActions))
	for i, a := range keyActions {
		names[i] = a.Name
	}
	return names
}

var queryFields = []QueryField{
	QueryFieldID, QueryFieldTitle, QueryFieldStatus, QueryFieldPriority,
	QueryFieldType, QueryFieldLabel, QueryFieldAssignee, QueryFieldProject,
}

func queryFieldNames() []string {
	names := make([]string, len(queryFields))
	for i, f := range queryFields {
		names[i] = string(f)
	}
	return names
}

// unknownQueryField returns the first field:value token whose field is not a
// query field. ParseIssueQuery accepts any field name and then matches nothing,
// which would leave a shortcut that silently empties the view.
func unknownQueryField(query string) (string, bool) {
	for _, predicate := range ParseIssueQuery(query).predicates {
		if predicate.field != QueryFieldText && !slices.Contains(queryFields, predicate.field) {
			return string(predicate.field), true
		}
	}
	return "", false
}

// footerKeyLabel shortens a key name for the footer: ctrl+e becomes ^e.
func footerKeyLabel(key string) string {
	return strings.Replace(key, "ctrl+", "^", 1)
}
