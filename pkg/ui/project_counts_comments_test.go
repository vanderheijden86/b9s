package ui

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/vanderheijden86/beadwork/pkg/config"
)

// sqliteFullIssuesSchema mirrors every column SQLiteReader.LoadIssuesFiltered
// selects, so the full-schema query succeeds and any comments failure comes
// from the missing comments table alone, not from the schema fallback.
const sqliteFullIssuesSchema = `CREATE TABLE issues (id TEXT, title TEXT, description TEXT, status TEXT, priority INT, issue_type TEXT,
	assignee TEXT, estimated_minutes INT, created_at DATETIME, updated_at DATETIME,
	due_date DATETIME, closed_at DATETIME, external_ref TEXT, compaction_level INT,
	compacted_at DATETIME, compacted_at_commit TEXT, original_size INT,
	labels TEXT, design TEXT, acceptance_criteria TEXT, notes TEXT, source_repo TEXT, tombstone INT)`

// writeCheckoutWithCommentsFailure builds a checkout whose SQLite database
// has one open issue and no comments table, so loading it succeeds with an
// error wrapping datasource.ErrCommentsUnavailable rather than failing
// outright.
func writeCheckoutWithCommentsFailure(t *testing.T, root, name string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	beadsDir := filepath.Join(dir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", filepath.Join(beadsDir, "beads.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, stmt := range []string{
		sqliteFullIssuesSchema,
		`INSERT INTO issues (id, title, status, priority, issue_type) VALUES ('` + name + `-1', '` + name + ` work', 'open', 2, 'task')`,
		// No comments table.
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("exec %q: %v", stmt, err)
		}
	}
	return dir
}

// TestProjectCountsKeepsCountsOnCommentsOnlyError guards bd-t8j5.18: a
// comments-only failure must still count as reachable with real numbers, not
// be treated the same as a source that could not be read at all - the R
// picker and the project header would otherwise show a live project as
// completely unreachable over one broken table.
func TestProjectCountsKeepsCountsOnCommentsOnlyError(t *testing.T) {
	root := t.TempDir()
	dir := writeCheckoutWithCommentsFailure(t, root, "degraded")
	project := config.Project{Name: "degraded", Path: dir}

	msg := loadProjectCountsCmd([]config.Project{project}, "")()

	loaded, ok := msg.(projectCountsLoadedMsg)
	if !ok {
		t.Fatalf("loadProjectCountsCmd produced %T, want projectCountsLoadedMsg", msg)
	}
	key := projectKey(project)
	counts, found := loaded.counts[key]
	if !found {
		t.Fatalf("counts missing for %q; a comments-only error must not drop the project's counts", key)
	}
	if counts.open != 1 {
		t.Errorf("counts = %+v, want open=1 for the one seeded issue", counts)
	}
}
