package main_test

import (
	"testing"
	"time"
)

// z on the board folds the focused column into a rail and names the key
// that unfolds it.
func TestBoardFoldColumnE2E(t *testing.T) {
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, makeFilterFixture(t))

	out, err := runTreeTUI(t, tempDir, 3000, []keyStep{
		kd("b", 600*time.Millisecond),
		kd("\x1b[C", 400*time.Millisecond), // Right: off the epic column onto the open column
		kd("z", 600*time.Millisecond),
	})
	if err != nil {
		t.Fatalf("run board TUI: %v", err)
	}
	containsAll(t, out, []string{"Z unfolds"})
}
