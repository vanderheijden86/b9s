package config

import (
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"
)

// ErrKeybindings marks a keybindings section that cannot be decoded at all.
// Whether each binding is meaningful (known action, free key) is decided by
// the ui package, which owns the actions and the built-in keys.
var ErrKeybindings = errors.New("invalid keybindings")

// Keybinding is one user-defined shortcut. Exactly one of Action and Query is
// set; ui.ResolveKeybindings enforces that and every other rule (ADR 0033).
type Keybinding struct {
	Key         string `yaml:"key"`
	Action      string `yaml:"action,omitempty"`      // a built-in action name
	Query       string `yaml:"query,omitempty"`       // a query such as type:epic
	Description string `yaml:"description,omitempty"` // footer and help text
	Override    bool   `yaml:"override,omitempty"`    // acknowledge shadowing a built-in key
}

// Keybindings is the ordered keybindings list of config.yaml.
type Keybindings []Keybinding

// UnmarshalYAML tags a decode failure with ErrKeybindings so startup can stop
// on it instead of falling back to defaults without a word.
func (k *Keybindings) UnmarshalYAML(node *yaml.Node) error {
	var decoded []Keybinding
	if err := node.Decode(&decoded); err != nil {
		return fmt.Errorf("%w: %v", ErrKeybindings, err)
	}
	*k = decoded
	return nil
}
