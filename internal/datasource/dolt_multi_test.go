package datasource

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

// fakeDoltIssuesSchema mirrors the columns DoltReader.LoadIssuesFiltered's
// full query selects. Its SQL is plain enough that any database/sql driver
// runs it, so a DoltReader backed by an on-disk SQLite file (not a real Dolt
// server) exercises the production scan and merge code directly.
const fakeDoltIssuesSchema = `CREATE TABLE issues (id TEXT, title TEXT, description TEXT, status TEXT, priority INT, issue_type TEXT,
	assignee TEXT, estimated_minutes INT, created_at DATETIME, updated_at DATETIME,
	due_at DATETIME, closed_at DATETIME, external_ref TEXT, compaction_level INT,
	compacted_at DATETIME, compacted_at_commit TEXT, original_size INT,
	design TEXT, acceptance_criteria TEXT, notes TEXT, source_repo TEXT, defer_until DATETIME)`

// newFakeDoltDB builds a *DoltReader over an on-disk SQLite file seeded with
// one issue, standing in for one database on a Dolt server. withComments
// controls whether its comments table exists, so a test can produce a
// comments-only failure on demand.
func newFakeDoltDB(t *testing.T, name string, withComments bool) *DoltReader {
	t.Helper()
	path := filepath.Join(t.TempDir(), name+".db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	stmts := []string{
		fakeDoltIssuesSchema,
		`INSERT INTO issues (id, title, status, priority, issue_type) VALUES ('` + name + `-1', 'one', 'open', 2, 'task')`,
		`CREATE TABLE labels (issue_id TEXT, label TEXT)`,
		`CREATE TABLE dependencies (issue_id TEXT, depends_on_issue_id TEXT, depends_on_wisp_id TEXT, depends_on_external TEXT, type TEXT)`,
	}
	if withComments {
		stmts = append(stmts, `CREATE TABLE comments (id TEXT, issue_id TEXT, author TEXT, text TEXT, created_at DATETIME)`)
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("exec %q: %v", stmt, err)
		}
	}
	return &DoltReader{db: db, addr: name}
}

// TestMultiDoltReader_LoadAllIssuesKeepsIssuesOnCommentsOnlyError guards
// bd-t8j5.18: before this fix, a database whose comments query failed lost
// its issues entirely from the merged all-projects view, the same
// silent-drop bug LoadIssuesFiltered's own ErrCommentsUnavailable contract
// already fixed for a single project.
func TestMultiDoltReader_LoadAllIssuesKeepsIssuesOnCommentsOnlyError(t *testing.T) {
	healthy := newFakeDoltDB(t, "healthy", true)
	degraded := newFakeDoltDB(t, "degraded", false)
	m := &MultiDoltReader{
		readers: map[string]*DoltReader{"healthy": healthy, "degraded": degraded},
		dbNames: []string{"healthy", "degraded"},
	}

	issues, err := m.LoadAllIssues()

	if !errors.Is(err, ErrCommentsUnavailable) {
		t.Fatalf("LoadAllIssues() err = %v, want it to wrap ErrCommentsUnavailable", err)
	}
	if len(issues) != 2 {
		t.Fatalf("issues = %+v, want both databases' issues kept despite one comments failure", issues)
	}
}

// TestMultiDoltReader_LoadAllIssuesDropsGenuineFailure keeps the existing
// "drop a database that truly failed" behaviour: a comments-only error must
// not widen into tolerating every error.
func TestMultiDoltReader_LoadAllIssuesDropsGenuineFailure(t *testing.T) {
	healthy := newFakeDoltDB(t, "healthy", true)
	broken := newFakeDoltDB(t, "broken", true)
	broken.Close() // Every query on this reader now fails outright.
	m := &MultiDoltReader{
		readers: map[string]*DoltReader{"healthy": healthy, "broken": broken},
		dbNames: []string{"healthy", "broken"},
	}

	issues, err := m.LoadAllIssues()

	if errors.Is(err, ErrCommentsUnavailable) {
		t.Fatalf("LoadAllIssues() err = %v, want it to not report a genuine failure as ErrCommentsUnavailable", err)
	}
	if len(issues) != 1 || issues[0].SourceRepo != "healthy" {
		t.Fatalf("issues = %+v, want only healthy's issue; broken's database must still be dropped", issues)
	}
}
