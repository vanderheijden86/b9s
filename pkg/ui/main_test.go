package ui

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// Prevent any test from accidentally opening a browser
	os.Setenv("B9S_NO_BROWSER", "1")
	os.Setenv("B9S_TEST_MODE", "1")

	// Opening a project saves the recent list to the user config, so no test
	// may reach the developer's real ~/.config/b9s.
	configHome, err := os.MkdirTemp("", "b9s-ui-config-")
	if err != nil {
		panic(err)
	}
	os.Setenv("XDG_CONFIG_HOME", configHome)

	// Clean up any tree-state.json that non-isolated tree tests leave behind
	// in the CWD. Go tests run from the package directory, so expand/collapse
	// operations via ui.NewModel can pollute .beads/tree-state.json here,
	// causing cross-test ordering failures.
	os.RemoveAll(".beads")

	code := m.Run()

	// Post-test cleanup. os.Exit skips deferred calls, so cleanup runs explicitly.
	os.RemoveAll(".beads")
	os.RemoveAll(configHome)

	os.Exit(code)
}
