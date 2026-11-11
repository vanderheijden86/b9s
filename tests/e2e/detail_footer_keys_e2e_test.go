package main_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDetailCommentOrderE2E(t *testing.T) {
	for _, oldestFirst := range []bool{false, true} {
		name := "newest-first"
		if oldestFirst {
			name = "oldest-first"
		}
		t.Run(name, func(t *testing.T) {
			dir := detailFooterFixture(t)
			fixture := `{"id":"detail-1","title":"Comment order","status":"open","priority":2,"issue_type":"task","comments":[{"id":"1","text":"OLD-MESSAGE","created_at":"2026-01-01T00:00:00Z"},{"id":"2","text":"NEW-MESSAGE","created_at":"2026-01-02T00:00:00Z"}]}`
			if err := os.WriteFile(filepath.Join(dir, ".beads", "beads.jsonl"), []byte(fixture+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			keys := []keyStep{kd("\r", 300*time.Millisecond)}
			if oldestFirst {
				keys = append(keys, kd("s", 400*time.Millisecond))
			}
			out, err := runTreeTUIWithEnv(t, dir, 2500, keys, "B9S_TEST_MODE=1")
			if err != nil {
				t.Fatalf("TUI failed: %v\n%s", err, out)
			}
			frame := treeFinalFrame(out)
			old, recent := strings.Index(frame, "OLD-MESSAGE"), strings.Index(frame, "NEW-MESSAGE")
			if old < 0 || recent < 0 || (old < recent) != oldestFirst {
				t.Fatalf("wrong comment order (%s):\n%s", name, frame)
			}
		})
	}
}

func detailFooterFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeTreeFixture(t, dir, []treeFixtureIssue{{
		ID: "detail-1", Title: "Detail issue", Status: "open", Priority: 2, IssueType: "task",
		CreatedAt: time.Now().Format(time.RFC3339), Description: "Some description.",
	}})
	return dir
}

func TestDetailEKeyOpensEditModalE2E(t *testing.T) {
	dir := detailFooterFixture(t)

	out, err := runTreeTUIWithEnv(t, dir, 3000, []keyStep{
		kd("\r", 300*time.Millisecond), // open the detail
		kd("e", 400*time.Millisecond),
	}, "B9S_TEST_MODE=1")
	if err != nil {
		t.Fatalf("TUI run failed: %v\n%s", err, out)
	}
	frame := treeFinalFrame(out)
	if !strings.Contains(frame, "Edit Issue: detail-1") {
		t.Fatalf("expected e on the detail screen to open the edit modal:\n%s", frame)
	}
}

func TestDetailIgnoresRemovedEditorKeyE2E(t *testing.T) {
	dir := detailFooterFixture(t)

	out, err := runTreeTUIWithEnv(t, dir, 3000, []keyStep{
		kd("\r", 300*time.Millisecond), // open the detail
		kd("O", 400*time.Millisecond),
	}, "B9S_TEST_MODE=1", "TMPDIR="+t.TempDir())
	if err != nil {
		t.Fatalf("TUI run failed: %v\n%s", err, out)
	}
	frame := treeFinalFrame(out)
	if strings.Contains(frame, "Would open") || !strings.Contains(frame, "Detail issue") {
		t.Fatalf("O must leave the detail visible without opening an editor:\n%s", frame)
	}
}

func TestDetailChildrenBeyondNineE2E(t *testing.T) {
	for _, tc := range []struct {
		name string
		keys string
		want string
	}{
		{"tenth", "9n", "BODY-CHILD-10"},
		{"last", "9" + strings.Repeat("n", 15), "BODY-CHILD-24"},
		{"return-to-parent", "9n\x7f", "BODY-PARENT"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			now := time.Now()
			issues := []treeFixtureIssue{{
				ID: "many-epic", Title: "Many children", Status: "open", Priority: 1,
				IssueType: "epic", CreatedAt: now.Format(time.RFC3339), Description: "BODY-PARENT",
			}}
			for n := 1; n <= 24; n++ {
				id := fmt.Sprintf("many-child-%02d", n)
				issues = append(issues, treeFixtureIssue{
					ID: id, Title: fmt.Sprintf("Child %02d", n), Status: "open", Priority: 2,
					IssueType: "task", CreatedAt: now.Add(-time.Duration(n) * time.Second).Format(time.RFC3339),
					Description:  fmt.Sprintf("BODY-CHILD-%02d", n),
					Dependencies: []*treeFixtureDep{{IssueID: id, DependsOnID: "many-epic", Type: "parent-child"}},
				})
			}
			writeTreeFixture(t, dir, issues)
			keys := []keyStep{kd("\r", 200*time.Millisecond)}
			for _, key := range tc.keys {
				keys = append(keys, kd(string(key), 50*time.Millisecond))
			}
			out, err := runTreeTUI(t, dir, 2500, keys)
			if err != nil {
				t.Fatalf("child navigation TUI failed: %v\n%s", err, out)
			}
			frame := treeFinalFrame(out)
			if !strings.Contains(frame, tc.want) {
				t.Fatalf("expected %q in the selected detail:\n%s", tc.want, frame)
			}
			t.Logf("terminal final frame:\n%s", frame)
		})
	}
}
