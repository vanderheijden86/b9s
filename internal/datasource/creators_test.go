package datasource

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/vanderheijden86/beadwork/pkg/model"
	_ "modernc.org/sqlite"
)

func openCreatorsDB(t *testing.T, schema string, inserts ...string) *sql.DB {
	t.Helper()
	return openSQLiteAt(t, filepath.Join(t.TempDir(), "c.db"), append([]string{schema}, inserts...)...)
}

func openSQLiteAt(t *testing.T, path string, stmts ...string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("exec %q: %v", stmt, err)
		}
	}
	return db
}

func TestApplyCreators_FillsCreatedByAndOwner(t *testing.T) {
	db := openCreatorsDB(t,
		`CREATE TABLE issues (id TEXT, created_by TEXT, owner TEXT)`,
		`INSERT INTO issues VALUES ('bd-1', 'vanderheijden86', 'v@example.com'), ('bd-2', NULL, NULL)`,
	)
	issues := []model.Issue{{ID: "bd-1", Assignee: "BrownBear"}, {ID: "bd-2"}}

	applyCreators(db, issues)

	if issues[0].CreatedBy != "vanderheijden86" || issues[0].Owner != "v@example.com" {
		t.Errorf("bd-1 creator = %q/%q", issues[0].CreatedBy, issues[0].Owner)
	}
	if issues[0].Assignee != "BrownBear" {
		t.Errorf("bd-1 assignee changed to %q", issues[0].Assignee)
	}
	if issues[1].CreatedBy != "" || issues[1].Owner != "" {
		t.Errorf("bd-2 should have no creator, got %q/%q", issues[1].CreatedBy, issues[1].Owner)
	}
}

func TestApplyCreators_SchemaWithoutOwner(t *testing.T) {
	db := openCreatorsDB(t,
		`CREATE TABLE issues (id TEXT, created_by TEXT)`,
		`INSERT INTO issues VALUES ('bd-1', 'jemanuel')`,
	)
	issues := []model.Issue{{ID: "bd-1"}}

	applyCreators(db, issues)

	if issues[0].CreatedBy != "jemanuel" {
		t.Errorf("CreatedBy = %q, want jemanuel", issues[0].CreatedBy)
	}
}

func TestApplyCreators_SchemaWithoutCreatorColumns(t *testing.T) {
	db := openCreatorsDB(t, `CREATE TABLE issues (id TEXT)`, `INSERT INTO issues VALUES ('bd-1')`)
	issues := []model.Issue{{ID: "bd-1", Title: "kept"}}

	applyCreators(db, issues)

	if issues[0].CreatedBy != "" || issues[0].Title != "kept" {
		t.Errorf("issue changed: %+v", issues[0])
	}
}

func TestSQLiteReader_LoadsCreator(t *testing.T) {
	path := filepath.Join(t.TempDir(), "beads.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`CREATE TABLE issues (id TEXT, title TEXT, description TEXT, status TEXT, priority INT, issue_type TEXT,
			created_at DATETIME, updated_at DATETIME, tombstone INT, created_by TEXT, owner TEXT)`,
		`INSERT INTO issues (id, title, status, priority, issue_type, created_by, owner)
			VALUES ('bd-1', 'one', 'open', 2, 'task', 'ubuntu', 'u@example.com')`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("exec: %v", err)
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
	if len(issues) != 1 || issues[0].CreatedBy != "ubuntu" || issues[0].Owner != "u@example.com" {
		t.Fatalf("issues = %+v", issues)
	}
}
