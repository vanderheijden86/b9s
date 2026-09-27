package datasource

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// sqliteIssuesSchema is the CREATE TABLE LoadIssuesFiltered's full-schema
// query needs; every column it selects must exist or SQLiteReader silently
// falls back to loadIssuesSimple, which never loads comments at all (see
// TestSQLiteReader_LoadsCommentID).
const sqliteIssuesSchema = `CREATE TABLE issues (id TEXT, title TEXT, description TEXT, status TEXT, priority INT, issue_type TEXT,
	assignee TEXT, estimated_minutes INT, created_at DATETIME, updated_at DATETIME,
	due_date DATETIME, closed_at DATETIME, external_ref TEXT, compaction_level INT,
	compacted_at DATETIME, compacted_at_commit TEXT, original_size INT,
	labels TEXT, design TEXT, acceptance_criteria TEXT, notes TEXT, source_repo TEXT, tombstone INT)`

// openSQLiteFixture creates a beads.db at t.TempDir()/beads.db, running
// stmts (schema and seed data) against it, and returns its path.
func openSQLiteFixture(t *testing.T, stmts []string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "beads.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("exec %q: %v", stmt, err)
		}
	}
	return path
}

// TestSQLiteReader_LoadIssuesReportsCommentsQueryError guards against
// dolt.go/sqlite.go's original bug: a comments query that fails (here, the
// table does not exist at all) used to make loadComments return nil,
// indistinguishable from an issue that genuinely has no comments. `b9s
// attach list` read that nil as "nothing attached", and a future `b9s
// attach gc` would delete every blob it could no longer see referenced.
func TestSQLiteReader_LoadIssuesReportsCommentsQueryError(t *testing.T) {
	path := openSQLiteFixture(t, []string{
		sqliteIssuesSchema,
		`INSERT INTO issues (id, title, status, priority, issue_type) VALUES ('bd-1', 'one', 'open', 2, 'task')`,
		// No comments table: every comments query fails outright.
	})

	r, err := NewSQLiteReader(DataSource{Type: SourceTypeSQLite, Path: path})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	issues, loadErr := r.LoadIssues()
	if !errors.Is(loadErr, ErrCommentsUnavailable) {
		t.Fatalf("LoadIssues() err = %v, want it to wrap ErrCommentsUnavailable", loadErr)
	}
	// The issue itself must still come back: a comments failure degrades the
	// comments field, not the whole load, or the TUI's issue list (and
	// OpenProject's source selection) would fail on a comments hiccup alone.
	if len(issues) != 1 || issues[0].ID != "bd-1" {
		t.Fatalf("issues = %+v, want one issue bd-1 despite the comments error", issues)
	}
	if issues[0].Comments != nil {
		t.Fatalf("issues[0].Comments = %v, want nil (unknown, not empty) when the query failed", issues[0].Comments)
	}
}

// TestSQLiteReader_LoadIssuesReportsCommentsScanError covers the other half
// of the bug: a query that succeeds but whose row fails to scan (here, a
// NULL comment id, which database/sql refuses to convert into the
// non-nullable model.Comment.ID) must be reported too, not just a query
// failure.
func TestSQLiteReader_LoadIssuesReportsCommentsScanError(t *testing.T) {
	path := openSQLiteFixture(t, []string{
		sqliteIssuesSchema,
		`INSERT INTO issues (id, title, status, priority, issue_type) VALUES ('bd-1', 'one', 'open', 2, 'task')`,
		`CREATE TABLE comments (id TEXT, issue_id TEXT, author TEXT, text TEXT, created_at DATETIME)`,
		`INSERT INTO comments (id, issue_id, author, text, created_at) VALUES (NULL, 'bd-1', 'alice', 'note', '2026-09-25 00:00:00')`,
	})

	r, err := NewSQLiteReader(DataSource{Type: SourceTypeSQLite, Path: path})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	issues, loadErr := r.LoadIssues()
	if !errors.Is(loadErr, ErrCommentsUnavailable) {
		t.Fatalf("LoadIssues() err = %v, want it to wrap ErrCommentsUnavailable", loadErr)
	}
	if len(issues) != 1 || issues[0].ID != "bd-1" {
		t.Fatalf("issues = %+v, want one issue bd-1 despite the scan error", issues)
	}
}

// TestSQLiteReader_AllCommentsFailsClosedOnQueryError guards the
// gc-facing API (point 4): AllComments must return an error, never a
// partial or empty map, when the underlying query fails.
func TestSQLiteReader_AllCommentsFailsClosedOnQueryError(t *testing.T) {
	path := openSQLiteFixture(t, []string{sqliteIssuesSchema})

	r, err := NewSQLiteReader(DataSource{Type: SourceTypeSQLite, Path: path})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	comments, allErr := r.AllComments()
	if !errors.Is(allErr, ErrCommentsUnavailable) {
		t.Fatalf("AllComments() err = %v, want it to wrap ErrCommentsUnavailable", allErr)
	}
	if comments != nil {
		t.Fatalf("AllComments() = %v, want nil on failure, never a partial map", comments)
	}
}

// TestSQLiteReader_AllCommentsLoadsEveryIssuesComments is AllComments' happy
// path: every issue's comments come back keyed by issue ID in one call, the
// shape a future b9s attach gc needs to build its live-reference set.
func TestSQLiteReader_AllCommentsLoadsEveryIssuesComments(t *testing.T) {
	path := openSQLiteFixture(t, []string{
		sqliteIssuesSchema,
		`INSERT INTO issues (id, title, status, priority, issue_type) VALUES ('bd-1', 'one', 'open', 2, 'task')`,
		`INSERT INTO issues (id, title, status, priority, issue_type) VALUES ('bd-2', 'two', 'open', 2, 'task')`,
		`CREATE TABLE comments (id TEXT, issue_id TEXT, author TEXT, text TEXT, created_at DATETIME)`,
		`INSERT INTO comments (id, issue_id, author, text, created_at) VALUES ('cmt-1', 'bd-1', 'alice', 'note', '2026-09-25 00:00:00')`,
		`INSERT INTO comments (id, issue_id, author, text, created_at) VALUES ('cmt-2', 'bd-2', 'bob', 'note', '2026-09-25 00:00:01')`,
	})

	r, err := NewSQLiteReader(DataSource{Type: SourceTypeSQLite, Path: path})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	comments, allErr := r.AllComments()
	if allErr != nil {
		t.Fatalf("AllComments() err = %v, want nil", allErr)
	}
	if len(comments) != 2 || len(comments["bd-1"]) != 1 || len(comments["bd-2"]) != 1 {
		t.Fatalf("AllComments() = %+v, want one comment each for bd-1 and bd-2", comments)
	}
}

// TestOpenProject_ToleratesCommentsUnavailableAndKeepsTheSource is the
// regression this whole fix exists to protect: before LoadIssuesFiltered
// reported a comments failure as an error, OpenProject had no way to see it
// at all. Once it does, OpenProject's own loop must not treat "comments
// unavailable" the same as "source unreadable" - a naive `err != nil, try
// the next source` would make a live SQLite (or Dolt) connection with one
// broken table look identical to a database that cannot be reached, and
// silently fall back to a lower-priority, possibly stale, source instead of
// just showing a degraded comments view.
func TestOpenProject_ToleratesCommentsUnavailableAndKeepsTheSource(t *testing.T) {
	dir := t.TempDir()
	beadsDir := filepath.Join(dir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(beadsDir, "beads.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		sqliteIssuesSchema,
		`INSERT INTO issues (id, title, status, priority, issue_type) VALUES ('bd-1', 'one', 'open', 2, 'task')`,
		// No comments table.
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("exec %q: %v", stmt, err)
		}
	}
	db.Close()

	opened, failure := OpenProject(OpenTarget{Name: "proj", Dir: dir})
	if failure != nil {
		t.Fatalf("OpenProject failure = %v, want a comments-only failure to still open the source", failure)
	}
	if len(opened.Issues) != 1 || opened.Issues[0].ID != "bd-1" {
		t.Fatalf("opened.Issues = %+v, want one issue bd-1", opened.Issues)
	}
	if !errors.Is(opened.CommentsErr, ErrCommentsUnavailable) {
		t.Fatalf("opened.CommentsErr = %v, want it to wrap ErrCommentsUnavailable so the caller can show a note", opened.CommentsErr)
	}
	if opened.Source.Type != SourceTypeSQLite {
		t.Fatalf("opened.Source.Type = %v, want SourceTypeSQLite (must not have fallen back)", opened.Source.Type)
	}
}
