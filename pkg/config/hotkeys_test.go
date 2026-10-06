package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeHotkeys(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "hotkeys.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// The file has the k9s hotkeys.yaml shape, and the order of the file is kept
// so the footer lists hotkeys as the user wrote them.
func TestLoadHotkeysK9sShape(t *testing.T) {
	path := writeHotkeys(t, `hotKeys:
  open-epics:
    shortCut: Ctrl-T
    description: Open epics
    command: epic status:open
  bugs:
    shortCut: F4
    override: true
    keepHistory: true
    command: bug
`)
	hk, err := LoadHotkeysFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(hk) != 2 {
		t.Fatalf("got %d hotkeys, want 2", len(hk))
	}
	want := Hotkey{Name: "open-epics", ShortCut: "Ctrl-T", Description: "Open epics", Command: "epic status:open"}
	if hk[0] != want {
		t.Errorf("first = %+v, want %+v", hk[0], want)
	}
	if hk[1].Name != "bugs" || !hk[1].Override || !hk[1].KeepHistory || hk[1].Command != "bug" {
		t.Errorf("second = %+v", hk[1])
	}
}

func TestLoadHotkeysMissingFileHasNone(t *testing.T) {
	hk, err := LoadHotkeysFrom(filepath.Join(t.TempDir(), "hotkeys.yaml"))
	if err != nil || len(hk) != 0 {
		t.Fatalf("hk=%v err=%v, want none and no error", hk, err)
	}
}

// A typo in a field name would otherwise load a hotkey without its command.
func TestLoadHotkeysRejectsUnknownShapes(t *testing.T) {
	for name, tc := range map[string]struct{ body, want string }{
		"unknown field":   {"hotKeys:\n  e:\n    shortcut: F6\n    command: epic\n", `unknown field "shortcut"`},
		"unknown top key": {"hotkeys:\n  e:\n    shortCut: F6\n", `unknown key "hotkeys"`},
		"list not map":    {"hotKeys:\n  - shortCut: F6\n", "mapping"},
		"not yaml":        {"hotKeys: [", "yaml"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := LoadHotkeysFrom(writeHotkeys(t, tc.body))
			if !errors.Is(err, ErrHotkeys) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want ErrHotkeys containing %q", err, tc.want)
			}
		})
	}
}

func TestHotkeysPathSitsBesideConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/x")
	if got := HotkeysPath(); got != filepath.Join("/x", "b9s", "hotkeys.yaml") {
		t.Errorf("HotkeysPath() = %q", got)
	}
}
