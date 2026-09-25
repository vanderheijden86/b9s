package main_test

import (
	"testing"
	"time"
)

// o and i on the board write their status terms into the query bar, where
// they combine.
func TestBoardStatusKeysFillQueryBarE2E(t *testing.T) {
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, makeFilterFixture(t))

	out, err := runTreeTUI(t, tempDir, 3000, []keyStep{
		kd("b", 600*time.Millisecond),
		kd("o", 600*time.Millisecond),
		kd("i", 600*time.Millisecond),
	})
	if err != nil {
		t.Fatalf("run board TUI: %v", err)
	}
	containsAll(t, out, []string{"status:open status:in_progress"})
}
