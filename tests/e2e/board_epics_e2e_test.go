package main_test

import (
	"testing"
	"time"
)

// The board opens in the epic lanes design and v cycles through chips and
// groups in a real PTY. Each design renders text the others do not (the
// band's completion, the chip on the epic row, the design name), so the
// output proves every switch reached the renderer, not just the model.
func TestBoardEpicDesignsCycleWithV(t *testing.T) {
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, makeFilterFixture(t))

	out, err := runTreeTUI(t, tempDir, 3600, []keyStep{
		kd("b", 600*time.Millisecond),
		kd("v", 600*time.Millisecond),
		kd("v", 600*time.Millisecond),
	})
	if err != nil {
		t.Fatalf("run board TUI: %v", err)
	}
	containsAll(t, out, []string{
		"Epic lanes 1/3", "No epic", "1/3",
		"Epic chips 2/3", "epic · 1/3 done",
		"Epic groups 3/3",
	})
}

// Tab folds the selected epic's band in the lanes design: its children leave
// the board and the band header shows the folded marker.
func TestBoardEpicLanesTabFolds(t *testing.T) {
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, makeFilterFixture(t))

	out, err := runTreeTUI(t, tempDir, 2600, []keyStep{
		kd("b", 600*time.Millisecond),
		kd("\t", 600*time.Millisecond),
	})
	if err != nil {
		t.Fatalf("run board TUI: %v", err)
	}
	containsAll(t, out, []string{"▸ ◆"})
}
