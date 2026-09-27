package testutil

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/vanderheijden86/b9s/pkg/model"
)

func TestAssertionHelpersAcceptValidIssueGraph(t *testing.T) {
	now := time.Now()
	issues := []model.Issue{
		{ID: "epic-1", Title: "Epic", Status: model.StatusOpen, IssueType: model.TypeEpic, CreatedAt: now, UpdatedAt: now},
		{
			ID: "task-1", Title: "Task", Status: model.StatusInProgress, IssueType: model.TypeTask, CreatedAt: now, UpdatedAt: now,
			Dependencies: []*model.Dependency{{IssueID: "task-1", DependsOnID: "epic-1", Type: model.DepParentChild}},
		},
		{ID: "bug-1", Title: "Bug", Status: model.StatusBlocked, IssueType: model.TypeBug, CreatedAt: now, UpdatedAt: now},
		{ID: "done-1", Title: "Done", Status: model.StatusClosed, IssueType: model.TypeTask, CreatedAt: now, UpdatedAt: now},
	}

	AssertIssueCount(t, issues, 4)
	AssertNoDuplicateIDs(t, issues)
	AssertAllValid(t, issues)
	AssertDependencyExists(t, issues, "task-1", "epic-1")
	AssertNoCycles(t, issues)
	AssertStatusCounts(t, issues, 1, 1, 1, 1)
	AssertJSONEqual(t, issues, append([]model.Issue(nil), issues...))
}

func TestAssertHasCycleAcceptsCycle(t *testing.T) {
	issues := []model.Issue{
		{ID: "a", Dependencies: []*model.Dependency{{IssueID: "a", DependsOnID: "b"}}},
		{ID: "b", Dependencies: []*model.Dependency{{IssueID: "b", DependsOnID: "a"}}},
	}

	AssertHasCycle(t, issues)
}

func TestGoldenFileHelpersRoundTripTextAndJSON(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "golden")
	t.Setenv("GENERATE_GOLDEN", "1")
	textGolden := NewGoldenFile(t, dir, "text.golden")
	textGolden.Assert("expected text")
	if textGolden.Path() != filepath.Join(dir, "text.golden") {
		t.Fatalf("unexpected golden path %q", textGolden.Path())
	}

	t.Setenv("GENERATE_GOLDEN", "")
	NewGoldenFile(t, dir, "text.golden").Assert("expected text")

	t.Setenv("GENERATE_GOLDEN", "1")
	NewGoldenFile(t, dir, "json.golden").AssertJSON(map[string]int{"count": 2})
	t.Setenv("GENERATE_GOLDEN", "")
	NewGoldenFile(t, dir, "json.golden").AssertJSON(map[string]int{"count": 2})
}

func TestIssueFileAndCollectionHelpers(t *testing.T) {
	issues := []model.Issue{
		{ID: "task-1", Title: "One", Status: model.StatusOpen, IssueType: model.TypeTask},
		{ID: "bug-2", Title: "Two", Status: model.StatusClosed, IssueType: model.TypeBug},
	}

	dir := TempBeadsDir(t)
	beadsPath := WriteBeadsFile(t, dir, issues)
	if _, err := os.Stat(beadsPath); err != nil {
		t.Fatalf("beads file not written: %v", err)
	}
	customPath := filepath.Join(t.TempDir(), "nested", "issues.jsonl")
	WriteIssuesFile(t, customPath, issues)
	if _, err := os.Stat(customPath); err != nil {
		t.Fatalf("custom issues file not written: %v", err)
	}

	issueMap := BuildIssueMap(issues)
	if issueMap["bug-2"] == nil || issueMap["bug-2"].Title != "Two" {
		t.Fatalf("unexpected issue map: %#v", issueMap)
	}
	if got := FindIssue(issues, "task-1"); got == nil || got.Title != "One" {
		t.Fatalf("unexpected found issue: %#v", got)
	}
	if got := FindIssue(issues, "missing"); got != nil {
		t.Fatalf("expected missing issue, got %#v", got)
	}
	if got := CountByStatus(issues); got[model.StatusOpen] != 1 || got[model.StatusClosed] != 1 {
		t.Fatalf("unexpected status counts: %#v", got)
	}
	if got := CountByType(issues); got[model.TypeTask] != 1 || got[model.TypeBug] != 1 {
		t.Fatalf("unexpected type counts: %#v", got)
	}
	if got := GetIDs(issues); !reflect.DeepEqual(got, []string{"task-1", "bug-2"}) {
		t.Fatalf("unexpected IDs: %#v", got)
	}
	if got := IssueID(7); got != "test-7" {
		t.Fatalf("unexpected generated issue ID %q", got)
	}
}
