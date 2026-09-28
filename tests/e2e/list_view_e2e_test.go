// list_view_e2e_test.go - End-to-end proof of the flat list view: t sorts
// every issue across the hierarchy, the choice survives a restart, and a
// type: query lists only that type without parent rows (bd-9faq, ADR 0027).
package main_test

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// makeListViewFixture nests the newest issue two levels deep. With the
// default Created-descending sort the tree shows it last, or not at all while
// its parent is folded; the flat list must show it first.
//
//	Dingo root bug        (bug,  1h ago)
//	Aardvark epic         (epic, 3h ago)
//	  Bison task          (task, 2h ago)
//	    Cheetah task      (task, now)
func makeListViewFixture() []treeFixtureIssue {
	now := time.Now()
	at := func(offset time.Duration) string { return now.Add(offset).Format(time.RFC3339) }
	parent := func(id, of string) []*treeFixtureDep {
		return []*treeFixtureDep{{IssueID: id, DependsOnID: of, Type: "parent-child"}}
	}
	return []treeFixtureIssue{
		{ID: "lv-epic", Title: "Aardvark epic", Status: "open", Priority: 2, IssueType: "epic", CreatedAt: at(-3 * time.Hour)},
		{ID: "lv-task", Title: "Bison task", Status: "open", Priority: 2, IssueType: "task", CreatedAt: at(-2 * time.Hour), Dependencies: parent("lv-task", "lv-epic")},
		{ID: "lv-deep", Title: "Cheetah task", Status: "open", Priority: 2, IssueType: "task", CreatedAt: at(0), Dependencies: parent("lv-deep", "lv-task")},
		{ID: "lv-bug", Title: "Dingo root bug", Status: "open", Priority: 1, IssueType: "bug", CreatedAt: at(-time.Hour)},
	}
}

// finalListFrame returns the output from the last render of the list header.
func finalListFrame(t *testing.T, out []byte) string {
	t.Helper()
	s := stripTerminalCodes(out)
	start := strings.LastIndex(s, "[LIST]")
	if start < 0 {
		t.Fatalf("the list header [LIST] was never rendered\noutput:\n%s", s)
	}
	return s[start:]
}

func assertRowOrder(t *testing.T, frame string, titles ...string) {
	t.Helper()
	last := -1
	for _, title := range titles {
		pos := strings.Index(frame, title)
		if pos <= last {
			t.Fatalf("expected %q after position %d, got %d\nframe:\n%s", title, last, pos, frame)
		}
		last = pos
	}
}

func TestListViewSortsAcrossHierarchyAndPersistsE2E(t *testing.T) {
	dir := t.TempDir()
	writeTreeFixture(t, dir, makeListViewFixture())
	pass := 0

	out, err := runTreeTUI(t, dir, 2000, []keyStep{kd("t", 300*time.Millisecond)})
	if err != nil {
		t.Fatalf("TUI run with t failed: %v\noutput:\n%s", err, out)
	}
	assertRowOrder(t, finalListFrame(t, out), "Cheetah task", "Dingo root bug", "Bison task", "Aardvark epic")
	pass++

	out, err = runTreeTUI(t, dir, 1500, nil)
	if err != nil {
		t.Fatalf("restart failed: %v\noutput:\n%s", err, out)
	}
	assertRowOrder(t, finalListFrame(t, out), "Cheetah task", "Dingo root bug")
	pass++

	out, err = runTreeTUI(t, dir, 1500, []keyStep{kd("t", 300*time.Millisecond)})
	if err != nil {
		t.Fatalf("TUI run with the second t failed: %v\noutput:\n%s", err, out)
	}
	out, err = runTreeTUI(t, dir, 1500, nil)
	if err != nil {
		t.Fatalf("second restart failed: %v\noutput:\n%s", err, out)
	}
	if strings.Contains(stripTerminalCodes(out), "[LIST]") {
		t.Fatalf("t again must restore the tree after a restart\noutput:\n%s", stripTerminalCodes(out))
	}
	pass++
	fmt.Printf("=== LIST VIEW PERSIST DONE pass=%d fail=0 ===\n", pass)
}

func TestTypeQueryShowsOnlyThatTypeE2E(t *testing.T) {
	cases := []struct {
		name string
		keys []keyStep
	}{
		{name: "slash query", keys: []keyStep{
			kd("/", 200*time.Millisecond),
			kd("type:task", 150*time.Millisecond),
			kd("\r", 150*time.Millisecond),
		}},
		{name: "entity command", keys: []keyStep{
			kd(":", 200*time.Millisecond),
			kd("task", 150*time.Millisecond),
			kd("\r", 150*time.Millisecond),
		}},
	}
	pass := 0
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeTreeFixture(t, dir, makeListViewFixture())

			out, err := runTreeTUI(t, dir, 2200, tc.keys)
			if err != nil {
				t.Fatalf("TUI run failed: %v\noutput:\n%s", err, out)
			}
			frame := finalListFrame(t, out)
			assertRowOrder(t, frame, "Cheetah task", "Bison task")
			for _, hidden := range []string{"Aardvark epic", "Dingo root bug"} {
				if strings.Contains(frame, hidden) {
					t.Errorf("type:task shows %q\nframe:\n%s", hidden, frame)
				}
			}
			if !t.Failed() {
				pass++
			}
		})
	}
	fmt.Printf("=== TYPE QUERY LIST DONE pass=%d fail=%d ===\n", pass, len(cases)-pass)
}
