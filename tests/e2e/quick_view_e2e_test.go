package main_test

import (
	"strings"
	"testing"
	"time"
)

func TestQuickViewE2E(t *testing.T) {
	const pause = 200 * time.Millisecond
	detail := []keyStep{kd("\r", pause), kd("\x0f", pause), kd("quick-child.1", pause), kd("\r", pause)}
	readOnly := append([]keyStep{}, detail...)
	for _, key := range "eSKIy1:b" {
		readOnly = append(readOnly, kd(string(key), 20*time.Millisecond))
	}
	for _, tc := range []struct {
		name string
		keys []keyStep
		want []string
		deny []string
	}{
		{"row-preview", []keyStep{kd("\x0f", pause)}, []string{"Quick view", "Original issue"}, []string{"Edit Issue"}},
		{"flat-list-preview", []keyStep{kd("t", pause), kd("\x0f", pause)}, []string{"Quick view", "Original issue"}, []string{"Edit Issue"}},
		{"related-preview", detail, []string{"Quick view", "FIRST PREVIEW BODY"}, []string{"Edit Issue"}},
		{"sibling-cycle", append(append([]keyStep{}, detail...), kd("n", pause)), []string{"Quick view", "SECOND PREVIEW BODY"}, nil},
		{"read-only", readOnly, []string{"Quick view", "FIRST PREVIEW BODY"}, []string{"Edit Issue", "Set Status"}},
		{"restore-actions", append(append([]keyStep{}, detail...), kd("\x1b", pause), kd("e", pause)), []string{"Edit Issue: quick-origin"}, []string{"Edit Issue: quick-child"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeTreeFixture(t, dir, []treeFixtureIssue{
				{ID: "quick-origin", Title: "Original issue", Status: "open", Priority: 0, IssueType: "task",
					CreatedAt: "2026-11-10T00:00:00Z", Description: "See quick-child.1 and quick-child.2.",
					Dependencies: []*treeFixtureDep{{IssueID: "quick-origin", DependsOnID: "quick-child.1", Type: "related"}}},
				{ID: "quick-parent", Title: "Preview parent", Status: "open", Priority: 2, IssueType: "epic", CreatedAt: "2026-11-09T00:00:00Z"},
				{ID: "quick-child.1", Title: "First preview", Status: "closed", Priority: 2, IssueType: "task", Description: "FIRST PREVIEW BODY",
					CreatedAt:    "2026-11-09T00:00:00Z",
					Dependencies: []*treeFixtureDep{{IssueID: "quick-child.1", DependsOnID: "quick-parent", Type: "parent-child"}}},
				{ID: "quick-child.2", Title: "Second preview", Status: "open", Priority: 2, IssueType: "task", Description: "SECOND PREVIEW BODY",
					CreatedAt:    "2026-11-09T00:00:00Z",
					Dependencies: []*treeFixtureDep{{IssueID: "quick-child.2", DependsOnID: "quick-parent", Type: "parent-child"}}},
			})
			// A literal query selects the original while hiding both preview targets.
			keys := []keyStep{kd("/", pause)}
			for _, key := range "id:quick-origin" {
				keys = append(keys, kd(string(key), 20*time.Millisecond))
			}
			keys = append(keys, kd("\r", pause))
			keys = append(keys, tc.keys...)
			out, err := runTreeTUI(t, dir, 3500, keys)
			if err != nil {
				t.Fatalf("TUI failed: %v\n%s", err, out)
			}
			frame := plainFinalFrame(out)
			t.Logf("Final terminal frame (%s):\n%s", tc.name, frame)
			for _, want := range tc.want {
				if !strings.Contains(frame, want) {
					t.Errorf("missing %q:\n%s", want, frame)
				}
			}
			for _, deny := range tc.deny {
				if strings.Contains(frame, deny) {
					t.Errorf("unexpected %q:\n%s", deny, frame)
				}
			}
		})
	}
}
