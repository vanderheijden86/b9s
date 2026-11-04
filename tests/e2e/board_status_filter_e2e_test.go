package main_test

import (
	"strings"
	"testing"
	"time"
)

func TestBoardNavigationRoutingE2E(t *testing.T) {
	for _, tc := range []struct{ keys, want, absent string }{
		{"gg", "NAV FIRST CARD DETAIL", "NAV LAST CARD DETAIL"},
		{"0", "NAV FIRST CARD DETAIL", "NAV LAST CARD DETAIL"},
		{"G", "NAV LAST CARD DETAIL", "NAV FIRST CARD DETAIL"},
	} {
		t.Run(tc.keys, func(t *testing.T) {
			dir := t.TempDir()
			writeTreeFixture(t, dir, []treeFixtureIssue{
				{ID: "nav-first", Title: "First navigation card", Description: "NAV FIRST CARD DETAIL",
					Status: "open", Priority: 1, IssueType: "task", CreatedAt: "2026-01-01T00:00:00Z"},
				{ID: "nav-last", Title: "Last navigation card", Description: "NAV LAST CARD DETAIL",
					Status: "open", Priority: 2, IssueType: "task", CreatedAt: "2026-01-02T00:00:00Z"},
			})
			steps := []keyStep{kd("b", 200*time.Millisecond), kd("G", 200*time.Millisecond)}
			for _, key := range tc.keys {
				steps = append(steps, kd(string(key), 150*time.Millisecond))
			}
			// Detail exposes the selected card independently of the board layout.
			steps = append(steps, kd("\r", 150*time.Millisecond))
			out, err := runTreeTUI(t, dir, 2200, steps)
			if err != nil {
				t.Fatalf("TUI failed: %v\n%s", err, out)
			}
			frame := plainFinalFrame(out)
			if !strings.Contains(frame, tc.want) || strings.Contains(frame, tc.absent) {
				t.Fatalf("%s must select the expected card before Enter:\n%s", tc.keys, frame)
			}
		})
	}
}

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
