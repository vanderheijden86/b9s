package datasource

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/vanderheijden86/b9s/pkg/model"
)

const releaseJSONLFixture = `{"id":"rel-planned","title":"Future","status":"open","issue_type":"milestone","priority":2,"due_at":"2100-01-01T00:00:00Z","dependencies":[{"issue_id":"rel-planned","depends_on_id":"epic-open","type":"blocks"},{"issue_id":"rel-planned","depends_on_id":"task-open","type":"related"}]}
{"id":"rel-overdue","title":"Late","status":"open","issue_type":"milestone","priority":2,"due_date":"2000-01-01T00:00:00Z","dependencies":[{"issue_id":"rel-overdue","depends_on_id":"epic-open","type":"blocks"}]}
{"id":"rel-ready","title":"Ready","status":"open","issue_type":"milestone","priority":2,"due_at":"2000-01-01T00:00:00Z","dependencies":[{"issue_id":"rel-ready","depends_on_id":"epic-closed","type":"blocks"}]}
{"id":"rel-released","title":"Shipped","status":"closed","issue_type":"milestone","priority":2,"dependencies":[{"issue_id":"rel-released","depends_on_id":"epic-open","type":"blocks"}]}
{"id":"epic-open","title":"Open epic","status":"open","issue_type":"epic","priority":2}
{"id":"epic-closed","title":"Closed epic","status":"closed","issue_type":"epic","priority":2}
{"id":"task-open","title":"Task","status":"open","issue_type":"task","priority":2}
`

func assertReleaseFixture(t *testing.T, project OpenedProject) {
	t.Helper()
	if len(project.Releases) != 4 {
		t.Fatalf("got %d releases, want 4: %+v", len(project.Releases), project.Releases)
	}
	issues := make(map[string]model.Issue)
	for _, issue := range project.Issues {
		issues[issue.ID] = issue
	}
	want := map[string]struct {
		state model.ReleaseState
		epic  string
	}{
		"rel-planned":  {model.ReleasePlanned, "epic-open"},
		"rel-overdue":  {model.ReleaseOverdue, "epic-open"},
		"rel-ready":    {model.ReleaseReadyToShip, "epic-closed"},
		"rel-released": {model.ReleaseReleased, "epic-open"},
	}
	seen := make(map[string]bool)
	for _, release := range project.Releases {
		expected, ok := want[release.Issue.ID]
		if !ok || seen[release.Issue.ID] {
			t.Fatalf("unexpected or duplicate release: %+v", release)
		}
		seen[release.Issue.ID] = true
		if release.State != expected.state {
			t.Errorf("%s state = %s, want %s", release.Issue.ID, release.State, expected.state)
		}
		if !reflect.DeepEqual(release.Issue, issues[release.Issue.ID]) {
			t.Errorf("%s milestone differs from the loaded issue", release.Issue.ID)
		}
		if len(release.Epics) != 1 || !reflect.DeepEqual(release.Epics[0], issues[expected.epic]) {
			t.Errorf("%s epics = %+v, want loaded %s", release.Issue.ID, release.Epics, expected.epic)
		}
	}
}

func TestOpenProjectReleasesJSONL(t *testing.T) {
	beadsDir := filepath.Join(t.TempDir(), ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(beadsDir, "issues.jsonl"), releaseJSONLFixture)
	project, failure := OpenProject(OpenTarget{Dir: filepath.Dir(beadsDir)})
	if failure != nil {
		t.Fatal(failure)
	}
	if project.Source.Type != SourceTypeJSONLLocal {
		t.Fatalf("source = %s, want JSONL", project.Source.Type)
	}
	assertReleaseFixture(t, project)
}

func releaseSQLFixture(dolt bool) []string {
	schema, due, target, kind := sqliteIssuesSchema, "due_date", "depends_on_id", "dependency_type"
	dependencies := `CREATE TABLE dependencies (issue_id TEXT, depends_on_id TEXT, dependency_type TEXT)`
	if dolt {
		schema, due, target, kind = fakeDoltIssuesSchema, "due_at", "depends_on_issue_id", "type"
		dependencies = `CREATE TABLE dependencies (issue_id TEXT, depends_on_issue_id TEXT, depends_on_wisp_id TEXT, depends_on_external TEXT, type TEXT)`
	}
	return []string{
		schema,
		`CREATE TABLE labels (issue_id TEXT, label TEXT)`,
		dependencies,
		`CREATE TABLE comments (id TEXT, issue_id TEXT, author TEXT, text TEXT, created_at DATETIME)`,
		`INSERT INTO issues (id, title, status, priority, issue_type, ` + due + `) VALUES
			('rel-planned', 'Future', 'open', 2, 'milestone', '2100-01-01 00:00:00'),
			('rel-overdue', 'Late', 'open', 2, 'milestone', '2000-01-01 00:00:00'),
			('rel-ready', 'Ready', 'open', 2, 'milestone', '2000-01-01 00:00:00'),
			('rel-released', 'Shipped', 'closed', 2, 'milestone', NULL),
			('epic-open', 'Open epic', 'open', 2, 'epic', NULL),
			('epic-closed', 'Closed epic', 'closed', 2, 'epic', NULL),
			('task-open', 'Task', 'open', 2, 'task', NULL)`,
		`INSERT INTO dependencies (issue_id, ` + target + `, ` + kind + `) VALUES
			('rel-planned', 'epic-open', 'blocks'),
			('rel-planned', 'task-open', 'related'),
			('rel-overdue', 'epic-open', 'blocks'),
			('rel-ready', 'epic-closed', 'blocks'),
			('rel-released', 'epic-open', 'blocks')`,
	}
}

func TestOpenProjectReleasesSQLite(t *testing.T) {
	path := openSQLiteFixture(t, releaseSQLFixture(false))
	project, failure := OpenProject(OpenTarget{Dir: releaseSQLiteProject(t, path)})
	if failure != nil {
		t.Fatal(failure)
	}
	if project.Source.Type != SourceTypeSQLite {
		t.Fatalf("source = %s, want SQLite", project.Source.Type)
	}
	assertReleaseFixture(t, project)
}

func releaseSQLiteProject(t *testing.T, path string) string {
	t.Helper()
	beadsDir := filepath.Join(t.TempDir(), ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(beadsDir, "beads.db"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	return filepath.Dir(beadsDir)
}

func TestOpenProjectReleasesCommentsUnavailable(t *testing.T) {
	statements := releaseSQLFixture(false)
	// Omitting only the comments table must preserve the source's release
	// scope and state rather than selecting the stale export.
	statements = append(statements[:3], statements[4:]...)
	path := openSQLiteFixture(t, statements)
	dir := releaseSQLiteProject(t, path)
	writeFile(t, filepath.Join(dir, ".beads", "issues.jsonl"),
		`{"id":"stale","title":"Stale","status":"open","issue_type":"milestone","priority":2}`+"\n")
	project, failure := OpenProject(OpenTarget{Dir: dir})
	if failure != nil {
		t.Fatal(failure)
	}
	if !errors.Is(project.CommentsErr, ErrCommentsUnavailable) {
		t.Fatalf("comments error = %v, want ErrCommentsUnavailable", project.CommentsErr)
	}
	assertReleaseFixture(t, project)
}

func TestOpenProjectReleasesEmbeddedDolt(t *testing.T) {
	dir, beadsDir, _ := newEmbeddedProject(t)
	bin := t.TempDir()
	writeFile(t, filepath.Join(beadsDir, "export.jsonl"), releaseJSONLFixture)
	writeFile(t, filepath.Join(bin, "bd"), `#!/bin/sh
if [ "$1" != "export" ]; then exit 2; fi
echo export >> "$BEADS_DIR/export-calls"
cat "$BEADS_DIR/export.jsonl"
`)
	if err := os.Chmod(filepath.Join(bin, "bd"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+"/usr/bin:/bin")
	project, failure := OpenProject(OpenTarget{Dir: dir})
	if failure != nil {
		t.Fatal(failure)
	}
	if project.Source.Type != SourceTypeDoltEmbedded {
		t.Fatalf("source = %s, want embedded Dolt", project.Source.Type)
	}
	assertReleaseFixture(t, project)
	calls, err := os.ReadFile(filepath.Join(beadsDir, "export-calls"))
	if err != nil || string(calls) != "export\n" {
		t.Fatalf("export calls = %q, %v, want one export", calls, err)
	}
}

func TestOpenProjectReleasesDoltReader(t *testing.T) {
	path := openSQLiteFixture(t, releaseSQLFixture(true))
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	reader := &DoltReader{db: db, addr: "local-fixture"}
	for _, mode := range []struct {
		name string
		load func() ([]model.Issue, error)
	}{
		{"whole", reader.LoadIssues},
		{"lean", reader.LoadLeanIssues},
	} {
		t.Run(mode.name, func(t *testing.T) {
			issues, err := mode.load()
			if err != nil {
				t.Fatal(err)
			}
			project := projectFromIssues(issues, DataSource{Type: SourceTypeDolt}, time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC))
			assertReleaseFixture(t, project)
		})
	}
}

func TestLoadDiscoveryOptionsSkipPreflightValidation(t *testing.T) {
	opts := loadDiscoveryOptions("/repo/.beads", "/repo")

	if opts.ValidateAfterDiscovery {
		t.Fatal("load discovery must not validate a source before LoadFromSource opens and queries it")
	}
	if opts.IncludeInvalid {
		t.Fatal("load discovery should not request invalid-source metadata")
	}
}

func TestLoadSmartLoadsAnUnvalidatedCandidate(t *testing.T) {
	beadsDir := filepath.Join(t.TempDir(), ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	issueData := []byte("{\"id\":\"test-1\",\"title\":\"Fast path\",\"status\":\"open\",\"priority\":2,\"issue_type\":\"task\"}\n")
	if err := os.WriteFile(filepath.Join(beadsDir, "issues.jsonl"), issueData, 0o644); err != nil {
		t.Fatal(err)
	}

	issues, err := loadSmart(beadsDir, "")
	if err != nil {
		t.Fatalf("loadSmart failed: %v", err)
	}
	if len(issues) != 1 || issues[0].ID != "test-1" {
		t.Fatalf("unexpected issues: %#v", issues)
	}
}

func TestLoadSmartIgnoresNewerAuxiliaryJSONL(t *testing.T) {
	repoDir := t.TempDir()
	beadsDir := filepath.Join(repoDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	issuesPath := filepath.Join(beadsDir, "issues.jsonl")
	issueData := []byte("{\"id\":\"test-1\",\"title\":\"Canonical issue\",\"status\":\"open\",\"priority\":2,\"issue_type\":\"task\"}\n")
	if err := os.WriteFile(issuesPath, issueData, 0o644); err != nil {
		t.Fatal(err)
	}

	interactionsPath := filepath.Join(beadsDir, "interactions.jsonl")
	interactionData := []byte("{\"id\":\"int-1\",\"kind\":\"field_change\",\"issue_id\":\"test-1\"}\n")
	if err := os.WriteFile(interactionsPath, interactionData, 0o644); err != nil {
		t.Fatal(err)
	}

	older := time.Unix(1_700_000_000, 0)
	newer := older.Add(time.Hour)
	if err := os.Chtimes(issuesPath, older, older); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(interactionsPath, newer, newer); err != nil {
		t.Fatal(err)
	}

	issues, err := loadSmart(beadsDir, repoDir)
	if err != nil {
		t.Fatalf("loadSmart failed: %v", err)
	}
	if len(issues) != 1 || issues[0].ID != "test-1" {
		t.Fatalf("unexpected issues: %#v", issues)
	}
}

// A checkout whose Dolt server refuses the connection must report that
// failure. Stale JSONL beside it is not the project, and reading it would
// present the database as reachable.
func TestLoadIssuesFromCanonicalSourceReportsDoltFailureInsteadOfJSONL(t *testing.T) {
	beadsDir := filepath.Join(t.TempDir(), ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	metadata := `{"database":"dolt","dolt_mode":"server","dolt_server_host":"127.0.0.1","dolt_server_port":1,"dolt_server_user":"reader","dolt_database":"shared"}`
	if err := os.WriteFile(filepath.Join(beadsDir, "metadata.json"), []byte(metadata), 0o644); err != nil {
		t.Fatal(err)
	}
	stale := `{"id":"old-1","title":"Stale export","status":"open","issue_type":"task","priority":2}` + "\n"
	if err := os.WriteFile(filepath.Join(beadsDir, "issues.jsonl"), []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}

	issues, err := LoadIssuesFromCanonicalSource(beadsDir)

	if err == nil {
		t.Fatalf("got %d issues and no error; the refused Dolt connection must be reported, not hidden by the JSONL export", len(issues))
	}
	if got := ClassifyConnError(err); got != ReachServerDown {
		t.Errorf("ClassifyConnError = %v, want server down", got)
	}
}

// TestLoadIssuesFromDirToleratesCommentsUnavailable guards the same
// regression as TestOpenProject_ToleratesCommentsUnavailableAndKeepsTheSource,
// through a different call path: LoadIssuesFromDir is what the TUI's
// FileChangedMsg reload calls for a project opened with a checkout
// (bd-t8j5.18). Before this fix, loadSmart treated any non-nil error from
// LoadFromSource - including a comments-only failure - as "this source
// failed", and silently served the stale auxiliary JSONL export instead of
// the SQLite database's own, otherwise-complete issues.
func TestLoadIssuesFromDirToleratesCommentsUnavailable(t *testing.T) {
	beadsDir := filepath.Join(t.TempDir(), ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(beadsDir, "beads.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		sqliteIssuesSchema,
		`INSERT INTO issues (id, title, status, priority, issue_type) VALUES ('bd-1', 'live', 'open', 2, 'task')`,
		// No comments table.
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("exec %q: %v", stmt, err)
		}
	}
	db.Close()
	stale := `{"id":"old-1","title":"Stale export","status":"open","issue_type":"task","priority":2}` + "\n"
	if err := os.WriteFile(filepath.Join(beadsDir, "issues.jsonl"), []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}

	issues, err := LoadIssuesFromDir(beadsDir)

	if !errors.Is(err, ErrCommentsUnavailable) {
		t.Fatalf("LoadIssuesFromDir err = %v, want it to wrap ErrCommentsUnavailable", err)
	}
	if len(issues) != 1 || issues[0].ID != "bd-1" {
		t.Fatalf("issues = %+v, want the live SQLite issue bd-1, not a fallback to the stale JSONL export", issues)
	}
}
