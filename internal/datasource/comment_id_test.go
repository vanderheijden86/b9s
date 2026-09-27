package datasource

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// TestSQLiteReader_LoadsCommentID guards attachref.Collect's tie-break: it
// orders same-timestamp comments by ID, so a reader that drops the ID would
// silently break attachment ordering without any parse error to notice.
func TestSQLiteReader_LoadsCommentID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "beads.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		// LoadIssues only calls loadComments from its full-schema query path
		// (sqlite.go loadIssuesSimple, used when a column here is missing,
		// never loads comments at all), so every column that query selects
		// must exist for this fixture to exercise the code under test.
		`CREATE TABLE issues (id TEXT, title TEXT, description TEXT, status TEXT, priority INT, issue_type TEXT,
			assignee TEXT, estimated_minutes INT, created_at DATETIME, updated_at DATETIME,
			due_date DATETIME, closed_at DATETIME, external_ref TEXT, compaction_level INT,
			compacted_at DATETIME, compacted_at_commit TEXT, original_size INT,
			labels TEXT, design TEXT, acceptance_criteria TEXT, notes TEXT, source_repo TEXT, tombstone INT)`,
		`INSERT INTO issues (id, title, status, priority, issue_type) VALUES ('bd-1', 'one', 'open', 2, 'task')`,
		`CREATE TABLE comments (id TEXT, issue_id TEXT, author TEXT, text TEXT, created_at DATETIME)`,
		`INSERT INTO comments (id, issue_id, author, text, created_at) VALUES ('cmt-001', 'bd-1', 'alice', 'note', '2026-09-25 00:00:00')`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("exec %q: %v", stmt, err)
		}
	}
	db.Close()

	r, err := NewSQLiteReader(DataSource{Type: SourceTypeSQLite, Path: path})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	issues, err := r.LoadIssues()
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 1 || len(issues[0].Comments) != 1 {
		t.Fatalf("issues = %+v", issues)
	}
	if got := issues[0].Comments[0].ID; got != "cmt-001" {
		t.Errorf("comment ID = %q, want cmt-001", got)
	}
}
