package datasource

import (
	"regexp"
	"testing"

	"github.com/vanderheijden86/b9s/pkg/model"
)

func TestLeanIssueQuerySelectsNoTextColumn(t *testing.T) {
	for _, column := range []string{"description", "design", "acceptance_criteria", "notes"} {
		if regexp.MustCompile(`\b` + column + `\b`).MatchString(leanIssueQuery) {
			t.Errorf("lean query selects %s:\n%s", column, leanIssueQuery)
		}
	}
}

func TestIssueTextPreservesExtraDetailFields(t *testing.T) {
	db := openCreatorsDB(t,
		`CREATE TABLE issues (id TEXT, description TEXT, design TEXT, acceptance_criteria TEXT, notes TEXT, status TEXT, issue_type TEXT, close_reason TEXT, metadata TEXT)`,
		`INSERT INTO issues VALUES ('bd-39', '', '', '', '', 'closed', 'task', 'Finished after review', '{"branch":"feature/planning","nested":{"count":9007199254740993}}')`,
		`CREATE TABLE comments (id TEXT, issue_id TEXT, author TEXT, text TEXT, created_at DATETIME)`,
	)
	reader := &DoltReader{db: db}
	text, err := reader.LoadIssueText("bd-39")
	if err != nil {
		t.Fatal(err)
	}
	issue := model.Issue{ID: "bd-39", Abridged: true}
	text.ApplyTo(&issue)
	assertExtraDetailFields(t, issue)
	if issue.Abridged {
		t.Fatal("text load left issue abridged")
	}
	if _, err := db.Exec(`UPDATE issues SET status = 'open'`); err != nil {
		t.Fatal(err)
	}
	texts, err := reader.loadWholeIssueText([]string{"bd-39"})
	if err != nil {
		t.Fatal(err)
	}
	texts["bd-39"].ApplyTo(&issue)
	assertExtraDetailFields(t, issue)
}

// A closed issue arrives without its text and comments, and LoadIssueText
// supplies them; issues that are not closed, and epics, arrive whole.
func TestDoltIntegration_LoadLeanIssues(t *testing.T) {
	skipIfNoDoltIntegration(t)
	dbName, addr, cleanup := testDB(t)
	defer cleanup()

	db := openMutationDB(t, dbName, addr)
	defer db.Close()
	for _, stmt := range []string{
		`INSERT INTO issues (id, title, description, design, notes, status, priority, issue_type, closed_at)
			VALUES ('test-006', 'Old work', 'Closed text', 'Old design', 'Old notes', 'closed', 2, 'task', NOW())`,
		`INSERT INTO labels (issue_id, label) VALUES ('test-006', 'old')`,
		`INSERT INTO comments (id, issue_id, author, text) VALUES ('cmt-006', 'test-006', 'dana', 'Old remark')`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	reader := newTestDoltReader(t, dbName, addr)
	defer reader.Close()
	issues, err := reader.LoadLeanIssues()
	if err != nil {
		t.Fatalf("LoadLeanIssues: %v", err)
	}
	if len(issues) != 5 {
		t.Fatalf("got %d issues, want 5 (tombstone excluded)", len(issues))
	}
	byID := map[string]int{}
	for i := range issues {
		byID[issues[i].ID] = i
	}

	open := issues[byID["test-001"]]
	if open.Abridged || open.Description != "Users cannot log in with SSO" || open.Notes == "" || len(open.Comments) != 2 || len(open.Labels) != 2 {
		t.Fatalf("open issue should arrive whole: %+v", open)
	}
	epic := issues[byID["test-005"]]
	if epic.Abridged || epic.Design != "Use OAuth2 + PKCE" {
		t.Fatalf("epic should arrive whole: %+v", epic)
	}
	closed := issues[byID["test-006"]]
	if !closed.Abridged || closed.Description != "" || closed.Notes != "" || closed.Comments != nil {
		t.Fatalf("closed issue should arrive abridged: %+v", closed)
	}
	if closed.Title != "Old work" || closed.ClosedAt == nil || len(closed.Labels) != 1 {
		t.Fatalf("abridged issue lost a lean field: %+v", closed)
	}

	text, err := reader.LoadIssueText("test-006")
	if err != nil {
		t.Fatalf("LoadIssueText: %v", err)
	}
	if text.Description != "Closed text" || text.Design != "Old design" || text.Notes != "Old notes" {
		t.Fatalf("text = %+v", text)
	}
	if len(text.Comments) != 1 || text.Comments[0].ID != "cmt-006" || text.Comments[0].Text != "Old remark" {
		t.Fatalf("comments = %+v", text.Comments)
	}
}

// A schema without the lean columns reads whole rather than failing.
func TestDoltIntegration_LoadLeanIssuesFallsBackOnAnOlderSchema(t *testing.T) {
	skipIfNoDoltIntegration(t)
	dbName, addr, cleanup := testDBSimpleSchema(t)
	defer cleanup()

	reader := newTestDoltReader(t, dbName, addr)
	defer reader.Close()
	issues, err := reader.LoadLeanIssues()
	if err != nil {
		t.Fatalf("LoadLeanIssues() err = %v, want nil", err)
	}
	if len(issues) != 1 || issues[0].ID != "simple-001" || issues[0].Abridged {
		t.Fatalf("issues = %+v, want simple-001 read whole", issues)
	}
}

// Only a caller that loads Abridged text before using it asks for the lean
// read; OpenProject without Lean, as b9s web uses it, reads every issue whole.
func TestDoltIntegration_OnlyLeanOpenAbridges(t *testing.T) {
	skipIfNoDoltIntegration(t)
	dbName, addr, cleanup := testDB(t)
	defer cleanup()

	db := openMutationDB(t, dbName, addr)
	defer db.Close()
	if _, err := db.Exec(`INSERT INTO issues (id, title, description, status, priority, issue_type, closed_at)
		VALUES ('test-006', 'Old work', 'Closed text', 'closed', 2, 'task', NOW())`); err != nil {
		t.Fatalf("seed: %v", err)
	}
	source := DataSource{Type: SourceTypeDolt, Path: addr, Database: dbName, User: scratchServerUser()}

	find := func(opened OpenedProject) model.Issue {
		for _, issue := range opened.Issues {
			if issue.ID == "test-006" {
				return issue
			}
		}
		t.Fatal("test-006 missing")
		return model.Issue{}
	}

	whole, failure := OpenProject(OpenTarget{Name: dbName, Dolt: &source})
	if failure != nil {
		t.Fatalf("OpenProject: %v", failure)
	}
	if got := find(whole); got.Abridged || got.Description != "Closed text" {
		t.Fatalf("OpenProject abridged a closed issue: %+v", got)
	}

	lean, failure := OpenProjectLean(OpenTarget{Name: dbName, Dolt: &source})
	if failure != nil {
		t.Fatalf("OpenProjectLean: %v", failure)
	}
	if got := find(lean); !got.Abridged || got.Description != "" {
		t.Fatalf("OpenProjectLean read a closed issue whole: %+v", got)
	}
	text, err := LoadIssueText(source, "test-006")
	if err != nil || text.Description != "Closed text" {
		t.Fatalf("LoadIssueText = %+v, %v", text, err)
	}
}
