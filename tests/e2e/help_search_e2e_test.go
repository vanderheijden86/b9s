package main_test

import (
	"testing"
	"time"
)

// Ctrl+S is XOFF on a terminal with flow control, so only a real PTY proves
// the key reaches the help overlay. Letters that miss the search input would
// dismiss help instead, and the no-match notice would never render.
func TestHelpOverlaySearchWithCtrlS(t *testing.T) {
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, makeFilterFixture(t))

	out, err := runTreeTUI(t, tempDir, 2500, []keyStep{
		kd("?", 500*time.Millisecond),
		kd("\x13", 300*time.Millisecond),
		kd("zzzqqq", 300*time.Millisecond),
	})
	if err != nil {
		t.Fatalf("run tree TUI: %v", err)
	}
	containsAll(t, out, []string{"Keyboard Shortcuts", "No shortcuts match"})
}
