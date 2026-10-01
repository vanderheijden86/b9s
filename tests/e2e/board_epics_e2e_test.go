package main_test

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

func TestSepiaBoardRendersQuietPaperInPTY(t *testing.T) {
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, makeFilterFixture(t))
	out, err := runTreeTUIWithEnv(t, tempDir, 2200, []keyStep{kd("b", 600*time.Millisecond)},
		"COLORFGBG=0;15", "COLORTERM=truecolor", "NO_COLOR=", "XDG_CONFIG_HOME="+t.TempDir())
	if err != nil {
		t.Fatalf("run Sepia board TUI: %v", err)
	}
	frame := ansi.Strip(treeFinalFrame(out))
	for _, want := range []string{"BOARD", "Open Task A", "┃"} {
		if !strings.Contains(frame, want) {
			t.Fatalf("Quiet Paper frame lacks %q:\n%s", want, frame)
		}
	}
	if strings.ContainsAny(frame, "╭╮╰╯") {
		t.Fatalf("Sepia board still has rounded boxes:\n%s", frame)
	}
}

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

// f limits the board to the selected card's branch and names it in the bar
// above the board; f again shows the whole board.
func TestBoardBranchToggleWithF(t *testing.T) {
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, makeFilterFixture(t))

	out, err := runTreeTUI(t, tempDir, 3200, []keyStep{
		kd("b", 600*time.Millisecond),
		kd("f", 600*time.Millisecond),
		kd("f", 600*time.Millisecond),
	})
	if err != nil {
		t.Fatalf("run board TUI: %v", err)
	}
	containsAll(t, out, []string{"branch ", "f shows the whole board", "Whole board"})
}
