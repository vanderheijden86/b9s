package main

import (
	"bytes"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// sqliteIssuesSchemaForAttach mirrors internal/datasource's sqliteIssuesSchema:
// every column LoadIssuesFiltered's full-schema query selects must exist, or
// the reader silently falls back to a simpler query that never loads
// comments at all, and this test would pass for the wrong reason.
const sqliteIssuesSchemaForAttach = `CREATE TABLE issues (id TEXT, title TEXT, description TEXT, status TEXT, priority INT, issue_type TEXT,
	assignee TEXT, estimated_minutes INT, created_at DATETIME, updated_at DATETIME,
	due_date DATETIME, closed_at DATETIME, external_ref TEXT, compaction_level INT,
	compacted_at DATETIME, compacted_at_commit TEXT, original_size INT,
	labels TEXT, design TEXT, acceptance_criteria TEXT, notes TEXT, source_repo TEXT, tombstone INT)`

// attachTestProjectSQLiteBrokenComments builds a project backed by a SQLite
// beads.db with one issue and no comments table: every comments query on it
// fails outright, the fault this test exists to guard (bd-t8j5.18). It
// mirrors attachTestProject's JSONL fixture (fake bd on PATH, local-backend
// attachments config), swapping only the data source.
func attachTestProjectSQLiteBrokenComments(t *testing.T, bdExitCode int) (beadsDir, recordPath string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake bd script needs a POSIX shell")
	}

	dir := t.TempDir()
	beadsDir = filepath.Join(dir, ".beads")
	if err := os.Mkdir(beadsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BEADS_DIR", beadsDir)

	dbPath := filepath.Join(beadsDir, "beads.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		sqliteIssuesSchemaForAttach,
		`INSERT INTO issues (id, title, status, priority, issue_type) VALUES ('bd-1', 'Test issue', 'open', 2, 'task')`,
		// No comments table: every comments query fails outright.
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("exec %q: %v", stmt, err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	xdgConfig := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdgConfig)
	if err := os.MkdirAll(filepath.Join(xdgConfig, "b9s"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfgYAML := "attachments:\n  backend: local\n"
	if err := os.WriteFile(filepath.Join(xdgConfig, "b9s", "config.yaml"), []byte(cfgYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	recordPath = filepath.Join(dir, "bd-calls")
	binDir := t.TempDir()
	script := "#!/bin/sh\n" +
		"{\n" +
		"  for a in \"$@\"; do printf '%s\\036' \"$a\"; done\n" +
		"  printf '\\035'\n" +
		"} >> " + shellQuote(recordPath) + "\n" +
		"exit " + strconv.Itoa(bdExitCode) + "\n"
	if err := os.WriteFile(filepath.Join(binDir, "bd"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)

	return beadsDir, recordPath
}

// TestAttachList_FailsClosedWhenCommentsCannotLoad is the CLI-facing half of
// bd-t8j5.18: before the datasource fix, a broken comments table made
// loadComments return nil, and `b9s attach list` read that nil as "this
// issue has no attachments" - the exact state a future `b9s attach gc`
// would read as "delete every blob". It must instead fail with a non-zero
// exit and a message that names the real cause, never the empty-list message.
func TestAttachList_FailsClosedWhenCommentsCannotLoad(t *testing.T) {
	attachTestProjectSQLiteBrokenComments(t, 0)

	var stdout, stderr bytes.Buffer
	code := runAttach([]string{"list", "bd-1"}, &stdout, &stderr)

	if code == 0 {
		t.Fatalf("exit code = 0, want non-zero when comments cannot load; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if strings.Contains(stdout.String(), "no attachments") {
		t.Fatalf("stdout = %q, must never report \"no attachments\" when the comments query failed", stdout.String())
	}
	if !strings.Contains(stderr.String(), "comments") {
		t.Fatalf("stderr = %q, want it to name the comments failure", stderr.String())
	}
}

// TestAttachDetach_FailsClosedWhenCommentsCannotLoad mirrors the list case
// for --detach: it must not report "not currently attached" when the truth
// is that comments could not be read at all.
func TestAttachDetach_FailsClosedWhenCommentsCannotLoad(t *testing.T) {
	attachTestProjectSQLiteBrokenComments(t, 0)

	var stdout, stderr bytes.Buffer
	code := runAttach([]string{"--detach", "bd-1", strings.Repeat("a", 64)}, &stdout, &stderr)

	if code == 0 {
		t.Fatalf("exit code = 0, want non-zero when comments cannot load; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if strings.Contains(stderr.String(), "not currently attached") {
		t.Fatalf("stderr = %q, must never report \"not currently attached\" when the comments query failed", stderr.String())
	}
	if !strings.Contains(stderr.String(), "comments") {
		t.Fatalf("stderr = %q, want it to name the comments failure", stderr.String())
	}
}
