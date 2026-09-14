// tree_search_filter_e2e_test.go - End-to-end proof that free-text search keeps
// an active label filter instead of dropping it (bd-oe1y).
package main_test

import (
	"strings"
	"testing"
	"time"
)

// makeLabelSearchFixture builds a fixture where the "bug" label is the most used
// one, so it is reachable as label number 1 in the picker (labels are numbered by
// count descending). Only one "bug" issue also matches the query "alpha".
func makeLabelSearchFixture(t *testing.T) []treeFixtureIssue {
	t.Helper()
	now := time.Now()
	return []treeFixtureIssue{
		{ID: "bug-alpha", Title: "Alpha bug work", Status: "open", Priority: 1, IssueType: "bug",
			CreatedAt: now.Format(time.RFC3339), Labels: []string{"bug"}},
		{ID: "bug-beta", Title: "Beta bug work", Status: "open", Priority: 1, IssueType: "bug",
			CreatedAt: now.Add(time.Second).Format(time.RFC3339), Labels: []string{"bug"}},
		{ID: "bug-gamma", Title: "Gamma bug work", Status: "open", Priority: 1, IssueType: "bug",
			CreatedAt: now.Add(2 * time.Second).Format(time.RFC3339), Labels: []string{"bug"}},
		{ID: "feat-alpha", Title: "Alpha feature work", Status: "open", Priority: 2, IssueType: "feature",
			CreatedAt: now.Add(3 * time.Second).Format(time.RFC3339), Labels: []string{"feature"}},
		{ID: "feat-beta", Title: "Beta feature work", Status: "open", Priority: 2, IssueType: "feature",
			CreatedAt: now.Add(4 * time.Second).Format(time.RFC3339), Labels: []string{"feature"}},
	}
}

func TestSearchFieldHiddenUntilSlashE2E(t *testing.T) {
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, makeLabelSearchFixture(t))

	idleOut, err := runTreeTUI(t, tempDir, 1200, nil)
	if err != nil {
		t.Fatalf("idle TUI run failed: %v\noutput:\n%s", err, idleOut)
	}
	if strings.Contains(string(idleOut), "/█") {
		t.Fatalf("idle TUI rendered the search field\noutput:\n%s", idleOut)
	}

	editingOut, err := runTreeTUI(t, tempDir, 1500, []keyStep{kd("/", 150*time.Millisecond)})
	if err != nil {
		t.Fatalf("editing TUI run failed: %v\noutput:\n%s", err, editingOut)
	}
	if !strings.Contains(string(editingOut), "/█") {
		t.Fatalf("slash did not reveal the search field\noutput:\n%s", editingOut)
	}
}

func TestSearchTabCompletesPlainLabelE2E(t *testing.T) {
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, []treeFixtureIssue{
		{ID: "dispatch-1", Title: "Dispatch", Status: "open", Priority: 1, IssueType: "task", CreatedAt: time.Now().Format(time.RFC3339), Labels: []string{"lane-attempt=1"}},
	})

	out, err := runTreeTUI(t, tempDir, 2200, []keyStep{
		kd("/", 150*time.Millisecond),
		kd("l", 80*time.Millisecond),
		kd("a", 80*time.Millisecond),
		kd("n", 80*time.Millisecond),
		kd("\t", 100*time.Millisecond),
	})
	if err != nil {
		t.Fatalf("completion TUI run failed: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(string(out), "/ lane-attempt=1█") {
		t.Fatalf("Tab did not complete the plain label\noutput:\n%s", out)
	}
}

func TestEmptyLabelFacetKeepsResultsE2E(t *testing.T) {
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, makeLabelSearchFixture(t))

	out, err := runTreeTUI(t, tempDir, 2200, []keyStep{
		kd("/", 150*time.Millisecond),
		kd("l", 60*time.Millisecond),
		kd("a", 60*time.Millisecond),
		kd("b", 60*time.Millisecond),
		kd("e", 60*time.Millisecond),
		kd("l", 60*time.Millisecond),
		kd(":", 60*time.Millisecond),
		kd("\r", 100*time.Millisecond),
		kd("K", 100*time.Millisecond),
	})
	if err != nil {
		t.Fatalf("empty-facet TUI run failed: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(string(out), "Close issue?") {
		t.Fatalf("empty label facet removed every selectable issue\noutput:\n%s", out)
	}
}

// TestTreeSearchKeepsLabelFilterE2E drives the real TUI through a composed
// label filter and free-text query.
func TestTreeSearchKeepsLabelFilterE2E(t *testing.T) {
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, makeLabelSearchFixture(t))

	out, err := runTreeTUI(t, tempDir, 4000, []keyStep{
		kd("L", 150*time.Millisecond), // switch picker to label mode
		kd("1", 150*time.Millisecond), // filter to the "bug" label
		kd("/", 150*time.Millisecond), // enter tree search
		kd("a", 80*time.Millisecond),
		kd("l", 80*time.Millisecond),
		kd("p", 80*time.Millisecond),
		kd("h", 80*time.Millisecond),
		kd("a", 80*time.Millisecond),
	})
	if err != nil {
		t.Fatalf("TUI run failed: %v\noutput:\n%s", err, out)
	}

	s := string(out)
	if !strings.Contains(s, "/ alpha█") {
		t.Errorf("shared search field never showed the full query %q\noutput:\n%s", "alpha", s)
	}
	if !strings.Contains(s, "╭") || !strings.Contains(s, "╰") {
		t.Errorf("expected a bordered search field\noutput:\n%s", s)
	}
	if strings.Contains(s, "WHERE / FILTER") || strings.Contains(s, "ORDER BY") {
		t.Errorf("search field contains query-plan clutter\noutput:\n%s", s)
	}
}

// TestGlobalFuzzyLabelSearchE2E verifies that plain text can find an issue by
// an abbreviated label even when the title and ID do not contain the query.
func TestGlobalFuzzyLabelSearchE2E(t *testing.T) {
	tempDir := t.TempDir()
	now := time.Now()
	writeTreeFixture(t, tempDir, []treeFixtureIssue{
		{ID: "dispatch-1", Title: "Dispatch first lane", Status: "open", Priority: 1, IssueType: "task",
			CreatedAt: now.Format(time.RFC3339), Labels: []string{"lane-attempt=1"}},
		{ID: "queue-1", Title: "Queue maintenance", Status: "open", Priority: 2, IssueType: "task",
			CreatedAt: now.Add(time.Second).Format(time.RFC3339), Labels: []string{"queue"}},
	})

	out, err := runTreeTUI(t, tempDir, 3000, []keyStep{
		kd("/", 150*time.Millisecond),
		kd("l", 80*time.Millisecond),
		kd("n", 80*time.Millisecond),
		kd("a", 80*time.Millisecond),
		kd("1", 80*time.Millisecond),
	})
	if err != nil {
		t.Fatalf("TUI run failed: %v\noutput:\n%s", err, out)
	}

	s := string(out)
	if !strings.Contains(s, "/ lna1█") {
		t.Errorf("search field never showed fuzzy label query\noutput:\n%s", s)
	}
	if strings.Contains(s, "No issues to display.") {
		t.Errorf("fuzzy label query produced an empty result set\noutput:\n%s", s)
	}
}

func TestGlobalFuzzySearchRejectsLongSparseSubsequenceE2E(t *testing.T) {
	const raw = "assetdeasdfjsladkfjsadfjsadf"
	tempDir := t.TempDir()
	now := time.Now()
	sparseTitle := strings.Join(strings.Split(raw, ""), " unrelated ")
	writeTreeFixture(t, tempDir, []treeFixtureIssue{
		{ID: "unrelated-1", Title: sparseTitle, Status: "open", Priority: 1, IssueType: "task", CreatedAt: now.Format(time.RFC3339)},
		{ID: "unrelated-2", Title: sparseTitle, Status: "open", Priority: 2, IssueType: "task", CreatedAt: now.Add(time.Second).Format(time.RFC3339)},
	})

	steps := []keyStep{kd("/", 150*time.Millisecond)}
	for _, char := range raw {
		steps = append(steps, kd(string(char), 20*time.Millisecond))
	}
	out, err := runTreeTUI(t, tempDir, 3000, steps)
	if err != nil {
		t.Fatalf("TUI run failed: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(string(out), "No issues to display.") {
		t.Fatalf("long sparse query retained unrelated issues\noutput:\n%s", out)
	}
}

func TestGlobalSearchShowsOnlyRelevantPreventResultE2E(t *testing.T) {
	tempDir := t.TempDir()
	now := time.Now()
	writeTreeFixture(t, tempDir, []treeFixtureIssue{
		{ID: "prevent-fuzzy", Title: "Prevent fuzzy search from matching unrelated issues", Status: "closed", Priority: 1, IssueType: "bug", CreatedAt: now.Format(time.RFC3339)},
		{ID: "publish-release", Title: "Publish release verification events", Status: "closed", Priority: 1, IssueType: "task", CreatedAt: now.Add(time.Second).Format(time.RFC3339)},
		{ID: "dependency-task", Title: "Make dependency blocking visible in the TUI", Status: "open", Priority: 1, IssueType: "epic", CreatedAt: now.Add(2 * time.Second).Format(time.RFC3339)},
	})

	out, err := runTreeTUI(t, tempDir, 2500, []keyStep{
		kd("/", 150*time.Millisecond),
		kd("prevent", 200*time.Millisecond),
	})
	if err != nil {
		t.Fatalf("TUI run failed: %v\noutput:\n%s", err, out)
	}

	s := string(out)
	queryStart := strings.LastIndex(s, "/ prevent")
	if queryStart < 0 {
		t.Fatalf("final query was not rendered\noutput:\n%s", s)
	}
	finalFrame := s[queryStart:]
	if !strings.Contains(finalFrame, "Prevent fuzzy search from matching unrelated issues") {
		t.Fatalf("matching prevent issue is absent\noutput:\n%s", s)
	}
	for _, unrelated := range []string{"Publish release verification events", "Make dependency blocking visible in the TUI"} {
		if strings.Contains(finalFrame, unrelated) {
			t.Errorf("query retained unrelated issue %q\noutput:\n%s", unrelated, s)
		}
	}
}

// makeBranchSearchFixture is one epic with two features, where only the task
// under the first feature mentions "zephyr", plus a standalone task (bd-xkxb).
func makeBranchSearchFixture() []treeFixtureIssue {
	now := time.Now()
	at := func(offset int) string { return now.Add(time.Duration(offset) * time.Second).Format(time.RFC3339) }
	parent := func(id, of string) []*treeFixtureDep {
		return []*treeFixtureDep{{IssueID: id, DependsOnID: of, Type: "parent-child"}}
	}
	return []treeFixtureIssue{
		{ID: "epic-1", Title: "Quokka acceptance testing", Status: "open", Priority: 1, IssueType: "epic", CreatedAt: at(0)},
		{ID: "feat-1", Title: "Verify first implementation", Status: "open", Priority: 1, IssueType: "feature", CreatedAt: at(1), Dependencies: parent("feat-1", "epic-1")},
		{ID: "task-1", Title: "Connect over the zephyr tunnel", Status: "open", Priority: 1, IssueType: "task", CreatedAt: at(2), Dependencies: parent("task-1", "feat-1")},
		{ID: "feat-2", Title: "Deploy to the cloud", Status: "open", Priority: 1, IssueType: "feature", CreatedAt: at(3), Dependencies: parent("feat-2", "epic-1")},
		{ID: "solo-1", Title: "Rotate release keys", Status: "open", Priority: 2, IssueType: "task", CreatedAt: at(4)},
	}
}

func finalSearchFrame(t *testing.T, out []byte, query string) string {
	t.Helper()
	s := string(out)
	start := strings.LastIndex(s, "/ "+query)
	if start < 0 {
		t.Fatalf("final query %q was not rendered\noutput:\n%s", query, s)
	}
	return s[start:]
}

func TestSearchHitOnTaskKeepsItsBranchE2E(t *testing.T) {
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, makeBranchSearchFixture())

	out, err := runTreeTUI(t, tempDir, 2500, []keyStep{
		kd("/", 150*time.Millisecond),
		kd("zephyr", 200*time.Millisecond),
	})
	if err != nil {
		t.Fatalf("TUI run failed: %v\noutput:\n%s", err, out)
	}

	frame := finalSearchFrame(t, out, "zephyr")
	for _, want := range []string{"Quokka acceptance testing", "Verify first implementation", "Connect over the zephyr tunnel"} {
		if !strings.Contains(frame, want) {
			t.Errorf("branch row %q is missing\noutput:\n%s", want, out)
		}
	}
	for _, hidden := range []string{"Deploy to the cloud", "Rotate release keys"} {
		if strings.Contains(frame, hidden) {
			t.Errorf("row %q outside the hit's branch is visible\noutput:\n%s", hidden, out)
		}
	}
}

func TestSearchHitOnEpicShowsItsSubtreeE2E(t *testing.T) {
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, makeBranchSearchFixture())

	out, err := runTreeTUI(t, tempDir, 2500, []keyStep{
		kd("/", 150*time.Millisecond),
		kd("quokka", 200*time.Millisecond),
	})
	if err != nil {
		t.Fatalf("TUI run failed: %v\noutput:\n%s", err, out)
	}

	frame := finalSearchFrame(t, out, "quokka")
	for _, want := range []string{"Quokka acceptance testing", "Verify first implementation", "Connect over the zephyr tunnel", "Deploy to the cloud"} {
		if !strings.Contains(frame, want) {
			t.Errorf("subtree row %q is missing\noutput:\n%s", want, out)
		}
	}
	if strings.Contains(frame, "Rotate release keys") {
		t.Errorf("unrelated standalone task is visible\noutput:\n%s", out)
	}
}

func TestGlobalSearchIgnoresBlockerMentionsForShortIssuePathE2E(t *testing.T) {
	tempDir := t.TempDir()
	now := time.Now()
	writeTreeFixture(t, tempDir, []treeFixtureIssue{
		{ID: "bd-lj5w", Title: "Publish v0.8.1 search patch release", Status: "closed", Priority: 1, IssueType: "task", CreatedAt: now.Format(time.RFC3339)},
		{ID: "bd-butn", Title: "Make dependency blocking visible in the TUI", Status: "open", Priority: 1, IssueType: "epic", CreatedAt: now.Add(time.Second).Format(time.RFC3339), Notes: "Blocked by bd-lj5w"},
		{ID: "bd-ej4c", Title: "Match k9s search interaction", Status: "closed", Priority: 2, IssueType: "epic", CreatedAt: now.Add(2 * time.Second).Format(time.RFC3339), Notes: "Related release: bd-lj5w"},
	})

	out, err := runTreeTUI(t, tempDir, 2500, []keyStep{
		kd("/", 150*time.Millisecond),
		kd("ljw", 200*time.Millisecond),
	})
	if err != nil {
		t.Fatalf("TUI run failed: %v\noutput:\n%s", err, out)
	}

	s := string(out)
	queryStart := strings.LastIndex(s, "/ ljw")
	if queryStart < 0 {
		t.Fatalf("final query was not rendered\noutput:\n%s", s)
	}
	finalFrame := s[queryStart:]
	if !strings.Contains(finalFrame, "Publish v0.8.1 search patch release") {
		t.Fatalf("matching issue path is absent\noutput:\n%s", s)
	}
	for _, unrelated := range []string{"Make dependency blocking visible in the TUI", "Match k9s search interaction"} {
		if strings.Contains(finalFrame, unrelated) {
			t.Errorf("query retained issue mentioning the path only in notes %q\noutput:\n%s", unrelated, s)
		}
	}
}
