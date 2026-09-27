package datasource

import (
	"database/sql"
	"errors"
	"testing"
)

// queryComments opens rows shaped like every reader's comments query (id,
// issue_id, author, text, created_at) against a fixture built by
// openSQLiteFixture, so scanAllComments can be tested directly against a
// real *sql.Rows without a mock library or a live Dolt server.
func queryComments(t *testing.T, path string) *sql.Rows {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	rows, err := db.Query(`SELECT id, issue_id, author, text, created_at FROM comments ORDER BY created_at`)
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func TestScanAllComments_GroupsByIssueID(t *testing.T) {
	path := openSQLiteFixture(t, []string{
		`CREATE TABLE comments (id TEXT, issue_id TEXT, author TEXT, text TEXT, created_at DATETIME)`,
		`INSERT INTO comments (id, issue_id, author, text, created_at) VALUES ('cmt-1', 'bd-1', 'alice', 'note', '2026-09-25 00:00:00')`,
		`INSERT INTO comments (id, issue_id, author, text, created_at) VALUES ('cmt-2', 'bd-2', 'bob', 'note', '2026-09-25 00:00:01')`,
	})

	result, err := scanAllComments(queryComments(t, path))
	if err != nil {
		t.Fatalf("scanAllComments() err = %v, want nil", err)
	}
	if len(result) != 2 || len(result["bd-1"]) != 1 || len(result["bd-2"]) != 1 {
		t.Fatalf("scanAllComments() = %+v, want one comment each for bd-1 and bd-2", result)
	}
}

func TestScanAllComments_ScanFailureReportsErrCommentsUnavailable(t *testing.T) {
	path := openSQLiteFixture(t, []string{
		`CREATE TABLE comments (id TEXT, issue_id TEXT, author TEXT, text TEXT, created_at DATETIME)`,
		`INSERT INTO comments (id, issue_id, author, text, created_at) VALUES (NULL, 'bd-1', 'alice', 'note', '2026-09-25 00:00:00')`,
	})

	result, err := scanAllComments(queryComments(t, path))
	if !errors.Is(err, ErrCommentsUnavailable) {
		t.Fatalf("scanAllComments() err = %v, want it to wrap ErrCommentsUnavailable", err)
	}
	if result != nil {
		t.Fatalf("scanAllComments() = %v, want nil on a scan failure, never a partial map", result)
	}
}
