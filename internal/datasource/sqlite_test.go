package datasource

import (
	"errors"
	"testing"
)

// sqliteSimpleSchema is the 8-column shape loadIssuesSimple's fallback query
// selects. A database with only these columns makes the full-schema query in
// LoadIssuesFiltered fail with SQLite's "no such column" error, the one
// signal that legitimately means "try the older, narrower schema."
const sqliteSimpleSchema = `CREATE TABLE issues (id TEXT, title TEXT, description TEXT, status TEXT, priority INT, issue_type TEXT,
	created_at DATETIME, updated_at DATETIME, tombstone INT)`

// TestSQLiteReader_SimpleSchemaFallbackLoadsComments guards the "fails open"
// bug in loadIssuesSimple (bd-t8j5.18 review): before this fix, the fallback
// path never loaded comments at all and always returned a nil error, so an
// issue with a real comment silently showed none.
func TestSQLiteReader_SimpleSchemaFallbackLoadsComments(t *testing.T) {
	path := openSQLiteFixture(t, []string{
		sqliteSimpleSchema,
		`INSERT INTO issues (id, title, status, priority, issue_type) VALUES ('bd-1', 'one', 'open', 2, 'task')`,
		`CREATE TABLE comments (id TEXT, issue_id TEXT, author TEXT, text TEXT, created_at DATETIME)`,
		`INSERT INTO comments (id, issue_id, author, text, created_at) VALUES ('cmt-1', 'bd-1', 'alice', 'note', '2026-09-25 00:00:00')`,
	})

	r, err := NewSQLiteReader(DataSource{Type: SourceTypeSQLite, Path: path})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	issues, loadErr := r.LoadIssues()
	if loadErr != nil {
		t.Fatalf("LoadIssues() err = %v, want nil", loadErr)
	}
	if len(issues) != 1 || len(issues[0].Comments) != 1 || issues[0].Comments[0].ID != "cmt-1" {
		t.Fatalf("issues = %+v, want bd-1 with comment cmt-1", issues)
	}
}

// TestSQLiteReader_SimpleSchemaFallbackReportsCommentsUnavailable is the
// other half: the fallback must report a comments failure, not swallow it
// behind a nil error the way it used to.
func TestSQLiteReader_SimpleSchemaFallbackReportsCommentsUnavailable(t *testing.T) {
	path := openSQLiteFixture(t, []string{
		sqliteSimpleSchema,
		`INSERT INTO issues (id, title, status, priority, issue_type) VALUES ('bd-1', 'one', 'open', 2, 'task')`,
		// No comments table.
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
		t.Fatalf("issues = %+v, want bd-1 despite the comments error", issues)
	}
}

// TestSQLiteReader_NonColumnQueryErrorPropagatesWithoutFallback guards the
// other bug the review found: LoadIssuesFiltered used to treat every full
// query error as a reason to fall back to the simple schema, so a transient
// failure (here, the connection is already closed) silently downgraded to a
// partial load instead of being reported.
func TestSQLiteReader_NonColumnQueryErrorPropagatesWithoutFallback(t *testing.T) {
	path := openSQLiteFixture(t, []string{
		sqliteIssuesSchema,
		`INSERT INTO issues (id, title, status, priority, issue_type) VALUES ('bd-1', 'one', 'open', 2, 'task')`,
	})

	r, err := NewSQLiteReader(DataSource{Type: SourceTypeSQLite, Path: path})
	if err != nil {
		t.Fatal(err)
	}
	r.Close() // Every subsequent query now fails with "sql: database is closed", not a column error.

	issues, loadErr := r.LoadIssues()
	if loadErr == nil {
		t.Fatal("LoadIssues() err = nil, want the closed-connection error reported unchanged")
	}
	if errors.Is(loadErr, ErrCommentsUnavailable) {
		t.Fatalf("LoadIssues() err = %v, want the raw query error, not a comments fallback result", loadErr)
	}
	if issues != nil {
		t.Fatalf("issues = %+v, want nil: a non-column error must not trigger the simple-schema fallback", issues)
	}
}

// TestSQLiteMissingColumnError_OnlyMatchesNoSuchColumn is the discriminator
// LoadIssuesFiltered's fallback gate relies on: it must fire for SQLite's own
// "no such column" wording and nothing else, or a transient failure (a locked
// or closed database) would still quietly downgrade to the simple schema.
func TestSQLiteMissingColumnError_OnlyMatchesNoSuchColumn(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "missing column", err: errors.New("SQL logic error: no such column: tombstone (1)"), want: true},
		{name: "missing table", err: errors.New("SQL logic error: no such table: comments"), want: false},
		{name: "closed database", err: errors.New("sql: database is closed"), want: false},
		{name: "locked database", err: errors.New("SQL logic error: database is locked"), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sqliteMissingColumnError(tt.err); got != tt.want {
				t.Errorf("sqliteMissingColumnError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
