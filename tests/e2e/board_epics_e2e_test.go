package main_test

import (
	"testing"
	"time"
)

// The board opens in the epic rail design and v switches to epic rows and
// back in a real PTY. Each design renders text the other does not (the rail's
// EPIC column, the design name), so the output proves every switch reached
// the renderer, not just the model.
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
		"Epic rail 1/2", "EPIC", "No epic", "╭",
		"Epic rows 2/2",
	})
}

// Tab folds the selected epic's lane: its cards leave the board and the lane
// shows the folded marker.
func TestBoardEpicTabFolds(t *testing.T) {
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

// The closed column stays hidden until c shows it.
func TestBoardClosedColumnToggleWithC(t *testing.T) {
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, makeFilterFixture(t))

	out, err := runTreeTUI(t, tempDir, 2600, []keyStep{
		kd("b", 600*time.Millisecond),
		kd("c", 600*time.Millisecond),
	})
	if err != nil {
		t.Fatalf("run board TUI: %v", err)
	}
	containsAll(t, out, []string{"hidden · c", "Closed column shown", "CLOSED"})
}
