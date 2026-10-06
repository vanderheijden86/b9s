package ui

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vanderheijden86/b9s/pkg/config"
)

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

// shiftedDigits are the characters a US keyboard sends for Shift-0 to
// Shift-9. A terminal reports the character, never the shift, so k9s's
// Shift-0 has to name the character to be pressable.
const shiftedDigits = ")!@#$%^&*("

// keyForShortCut turns a k9s shortCut into the key name tea.KeyMsg.String
// reports: F1 to F12, Ctrl-<letter>, Alt-<character>, Shift-<letter or digit>
// or one character. Modifier names ignore case, as in k9s.
func keyForShortCut(shortCut string) (string, error) {
	invalid := fmt.Errorf("%q is not a valid shortCut: use F1 to F12, Ctrl-<letter>, Alt-<character>, Shift-<letter or digit> or one character", shortCut)
	if runes := []rune(shortCut); len(runes) == 1 {
		if shortCut == " " {
			return "", invalid
		}
		return shortCut, nil
	}
	lower := strings.ToLower(shortCut)
	if n, err := strconv.Atoi(strings.TrimPrefix(lower, "f")); err == nil && strings.HasPrefix(lower, "f") && n >= 1 && n <= len(functionKeyTypes) {
		return "f" + strconv.Itoa(n), nil
	}
	modifier, rest, ok := strings.Cut(shortCut, "-")
	if !ok || len([]rune(rest)) != 1 {
		return "", invalid
	}
	r := []rune(rest)[0]
	switch strings.ToLower(modifier) {
	case "ctrl":
		if unicode.IsLetter(r) && r < unicode.MaxASCII {
			return "ctrl+" + string(unicode.ToLower(r)), nil
		}
	case "alt":
		if r != ' ' {
			return "alt+" + rest, nil
		}
	case "shift":
		switch {
		case r >= '0' && r <= '9':
			return string(shiftedDigits[r-'0']), nil
		case unicode.IsLetter(r):
			return string(unicode.ToUpper(r)), nil
		}
	}
	return "", invalid
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

// Hotkey is a validated entry of hotkeys.yaml.
type Hotkey struct {
	Name        string
	Key         string // as tea.KeyMsg.String reports it
	ShortCut    string // as the user wrote it
	Description string
	CommandText string
	command     Command
}

// Label is the text the footer shows for the hotkey.
func (h Hotkey) Label() string {
	if h.Description != "" {
		return h.Description
	}
	return h.CommandText
}

// Hotkeys holds the validated hotkeys in file order.
type Hotkeys struct {
	keys []Hotkey
}

// Lookup returns the hotkey for a key name as tea.KeyMsg.String reports it.
func (h Hotkeys) Lookup(key string) (Hotkey, bool) {
	for _, k := range h.keys {
		if k.Key == key {
			return k, true
		}
	}
	return Hotkey{}, false
}

// All returns the validated hotkeys in file order.
func (h Hotkeys) All() []Hotkey {
	return slices.Clone(h.keys)
}

// ResolveHotkeys validates the entries of hotkeys.yaml. An entry that breaks a
// rule is dropped and reported, so a typo never leaves a key doing something
// other than the user wrote: the caller shows every error and stops.
func ResolveHotkeys(in config.Hotkeys) (Hotkeys, []error) {
	var out Hotkeys
	var errs []error
	owner := map[string]string{}
	for _, entry := range in {
		fail := func(format string, args ...any) {
			errs = append(errs, fmt.Errorf("hotKeys.%s (shortCut %q): %s", entry.Name, entry.ShortCut, fmt.Sprintf(format, args...)))
		}
		if entry.ShortCut == "" {
			fail("needs a shortCut")
			continue
		}
		key, err := keyForShortCut(entry.ShortCut)
		if err != nil {
			fail("%v", err)
			continue
		}
		if slices.Contains(reservedKeys, key) {
			fail("%s is reserved and cannot be rebound", key)
			continue
		}
		if strings.TrimSpace(entry.Command) == "" {
			fail("needs a command")
			continue
		}
		command, err := ResolveCommand(entry.Command)
		if err != nil {
			fail("%v", err)
			continue
		}
		if slices.Contains(builtinKeys, key) && !entry.Override {
			fail("%s is a built-in key: set override: true to replace it, or pick a free key such as Ctrl-<letter>", key)
			continue
		}
		if first, taken := owner[key]; taken {
			fail("%s is already bound by hotKeys.%s", key, first)
			continue
		}
		owner[key] = entry.Name
		out.keys = append(out.keys, Hotkey{
			Name: entry.Name, Key: key, ShortCut: entry.ShortCut, Description: entry.Description,
			CommandText: strings.Join(strings.Fields(entry.Command), " "), command: command,
		})
	}
	return out, errs
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
