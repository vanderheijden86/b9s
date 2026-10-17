package main_test

import (
	"strings"
	"testing"
	"time"
)

// Ctrl+S is XOFF on a terminal with flow control, so only a real PTY proves
// the key reaches the help overlay. Letters that miss the search input would
// dismiss help instead, and the no-match notice would never render.
func TestHelpOverlaySearchWithCtrlS(t *testing.T) {
	for _, entry := range []string{"direct", "after-project-key"} {
		t.Run(entry, func(t *testing.T) {
			tempDir := t.TempDir()
			writeTreeFixture(t, tempDir, makeFilterFixture(t))

			keys := []keyStep{kd("?", 300*time.Millisecond)}
			if entry == "after-project-key" {
				keys = append(keys, kd("P", 200*time.Millisecond), kd("?", 200*time.Millisecond))
			}
			keys = append(keys, kd("\x13", 300*time.Millisecond), kd("zzzqqq", 300*time.Millisecond))
			out, err := runTreeTUI(t, tempDir, 2500, keys)
			if err != nil {
				t.Fatalf("run tree TUI: %v", err)
			}
			frame := plainFinalFrame(out)
			if !strings.Contains(frame, "Keyboard Shortcuts") || !strings.Contains(frame, "No shortcuts match") {
				t.Fatalf("help must own its search keys (%s):\n%s", entry, frame)
			}
		})
	}
}
