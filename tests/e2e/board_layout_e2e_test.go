package main_test

import (
	"testing"
	"time"
)

// The board opens in layout A and v switches to layout E through a real PTY.
// Layout E is the only one that renders the inspector's Ready field, so its
// text proves the switch reached the renderer, not just the model.
func TestBoardLayoutSwitchWithV(t *testing.T) {
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, makeFilterFixture(t))

	out, err := runTreeTUI(t, tempDir, 3000, []keyStep{
		kd("b", 600*time.Millisecond),
		kd("v", 600*time.Millisecond),
	})
	if err != nil {
		t.Fatalf("run board TUI: %v", err)
	}
	containsAll(t, out, []string{"A adaptive focus", "blocked by", "E focus + inspector", "Ready", "v layout A"})
}
