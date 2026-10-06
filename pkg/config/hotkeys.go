package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ErrHotkeys marks a hotkeys.yaml that cannot be read as the k9s hotkeys
// shape. Whether each hotkey is meaningful (a free key, a known command) is
// decided by the ui package, which owns the keys and the commands.
var ErrHotkeys = errors.New("invalid hotkeys")

// Hotkey is one entry of hotkeys.yaml, spelled as k9s spells it (ADR 0035).
type Hotkey struct {
	Name        string `yaml:"-"`
	ShortCut    string `yaml:"shortCut"`
	Description string `yaml:"description"`
	Command     string `yaml:"command"`
	Override    bool   `yaml:"override"`
	KeepHistory bool   `yaml:"keepHistory"`
}

// Hotkeys are the entries of hotkeys.yaml in file order.
type Hotkeys []Hotkey

var hotkeyFields = map[string]bool{
	"shortCut": true, "description": true, "command": true, "override": true, "keepHistory": true,
}

// HotkeysPath returns the full path to hotkeys.yaml, beside config.yaml.
func HotkeysPath() string {
	dir := ConfigDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "hotkeys.yaml")
}

// LoadHotkeys reads hotkeys.yaml from the XDG config directory.
func LoadHotkeys() (Hotkeys, error) {
	path := HotkeysPath()
	if path == "" {
		return nil, nil
	}
	return LoadHotkeysFrom(path)
}

// LoadHotkeysFrom reads a hotkeys file. A missing file defines no hotkeys.
// Unknown keys and fields are errors: k9s spells its fields in camelCase, and
// a misspelt shortCut would otherwise load a hotkey with no key.
func LoadHotkeysFrom(path string) (Hotkeys, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("%w: %v", ErrHotkeys, err)
	}
	fail := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", ErrHotkeys, fmt.Sprintf(format, args...))
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fail("not valid yaml: %v", err)
	}
	if len(doc.Content) == 0 {
		return nil, nil
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, fail("the file must be a mapping with one key, hotKeys")
	}
	var out Hotkeys
	for i := 0; i+1 < len(root.Content); i += 2 {
		key, entries := root.Content[i], root.Content[i+1]
		if key.Value != "hotKeys" {
			return nil, fail("line %d: unknown key %q, want hotKeys", key.Line, key.Value)
		}
		if entries.Kind != yaml.MappingNode {
			return nil, fail("line %d: hotKeys must be a mapping of name to hotkey", entries.Line)
		}
		for j := 0; j+1 < len(entries.Content); j += 2 {
			name, body := entries.Content[j], entries.Content[j+1]
			if body.Kind != yaml.MappingNode {
				return nil, fail("line %d: hotkey %q must be a mapping", body.Line, name.Value)
			}
			for f := 0; f < len(body.Content); f += 2 {
				if field := body.Content[f]; !hotkeyFields[field.Value] {
					return nil, fail("line %d: hotkey %q: unknown field %q, want shortCut, description, command, override or keepHistory", field.Line, name.Value, field.Value)
				}
			}
			var hk Hotkey
			if err := body.Decode(&hk); err != nil {
				return nil, fail("hotkey %q: %v", name.Value, err)
			}
			hk.Name = name.Value
			out = append(out, hk)
		}
	}
	return out, nil
}
