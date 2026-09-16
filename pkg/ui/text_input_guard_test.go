package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTextInputsUseTypedRunes keeps every text input on typedRunes. Checking
// msg.Type == tea.KeyRunes directly compiles and passes letter-only tests,
// yet drops the space bar, which bubbletea reports as KeySpace.
func TestTextInputsUseTypedRunes(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	const forbidden = "msg.Type == tea.KeyRunes && len(msg.Runes) > 0"
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(src), forbidden) {
			t.Errorf("%s reads text input with %q; use typedRunes(msg) so spaces are kept", file, forbidden)
		}
	}
}
