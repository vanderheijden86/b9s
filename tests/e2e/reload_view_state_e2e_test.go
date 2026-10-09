package main_test

import (
	"database/sql"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// A write by another process reloads the open project. These tests check that
// the reload keeps what the user is looking at: the folds of the tree and the
// issue in the detail pane. Both run against the scratch Dolt server, because
// the reload under test is the one a Dolt commit triggers.

// reloadSettle covers the 500ms Dolt poll, the reload itself and the
// identities load that follows it.
const reloadSettle = 2500 * time.Millisecond

// touchIssue changes one issue and commits, which is what `bd update` does.
func touchIssue(t *testing.T, db *sql.DB, id string) func() {
	return func() {
		stmts := []string{
			fmt.Sprintf("UPDATE issues SET notes = CONCAT(notes, 'x'), updated_at = NOW() WHERE id = '%s'", id),
			"CALL DOLT_COMMIT('-Am', 'touch')",
		}
		for _, stmt := range stmts {
			if _, err := db.Exec(stmt); err != nil {
				t.Errorf("touch %s: %v", id, err)
			}
		}
	}
}

func TestReloadKeepsTreeFoldsUnderStatusFilterE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PTY e2e test in -short mode")
	}
	dir, db := createDoltProject(t, "fold",
		`INSERT INTO issues (id, title, issue_type) VALUES
			('fold-1', 'Epic Alpha', 'epic'),
			('fold-2', 'Epic Beta', 'epic'),
			('fold-3', 'Child Alpha One', 'task'),
			('fold-4', 'Child Alpha Two', 'task'),
			('fold-5', 'Child Beta One', 'task'),
			('fold-6', 'Loose Task', 'task')`,
		`INSERT INTO dependencies (id, issue_id, depends_on_issue_id, type) VALUES
			(UUID(), 'fold-3', 'fold-1', 'parent-child'),
			(UUID(), 'fold-4', 'fold-1', 'parent-child'),
			(UUID(), 'fold-5', 'fold-2', 'parent-child')`,
	)

	var mark atomic.Int64
	out, err := runTreeTUIWithEnv(t, dir, 6500, []keyStep{
		kd("o", 1000*time.Millisecond), // status filter: open
		k("Z"),                         // collapse all
		ka(500*time.Millisecond, touchIssue(t, db, "fold-6"), &mark),
		kd("", reloadSettle),
	}, "BEADS_DOLT_PASSWORD=")
	if err != nil {
		t.Fatalf("run TUI: %v", err)
	}
	if mark.Load() == 0 {
		t.Fatal("the write never ran")
	}
	before, after := out[:mark.Load()], out[mark.Load():]
	containsAll(t, before, []string{"Child Alpha One"})
	for _, child := range []string{"Child Alpha One", "Child Alpha Two", "Child Beta One"} {
		if strings.Contains(string(after), child) {
			t.Errorf("reload reopened a collapsed branch: %q drawn after the write", child)
		}
	}
}

func TestReloadKeepsDetailOnSameIssueE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PTY e2e test in -short mode")
	}
	// Closed issues sort ahead of the open ones in the full list, so an issue's
	// row number differs between the full and the filtered list.
	dir, db := createDoltProject(t, "detail",
		`INSERT INTO issues (id, title, description, status, priority, created_at) VALUES
			('det-1', 'Open First', 'BODY-OPEN-FIRST', 'open', 2, '2026-01-01 00:00:00'),
			('det-2', 'Open Second', 'BODY-OPEN-SECOND', 'open', 2, '2026-01-02 00:00:00'),
			('det-3', 'Open Third', 'BODY-OPEN-THIRD', 'open', 2, '2026-01-03 00:00:00'),
			('det-4', 'Closed One', 'BODY-CLOSED-ONE', 'closed', 0, '2026-02-01 00:00:00'),
			('det-5', 'Closed Two', 'BODY-CLOSED-TWO', 'closed', 0, '2026-02-02 00:00:00'),
			('det-6', 'Closed Three', 'BODY-CLOSED-THREE', 'closed', 0, '2026-02-03 00:00:00'),
			('det-7', 'Closed Four', 'BODY-CLOSED-FOUR', 'closed', 0, '2026-02-04 00:00:00')`,
	)
	bodies := []string{
		"BODY-OPEN-FIRST", "BODY-OPEN-SECOND", "BODY-OPEN-THIRD",
		"BODY-CLOSED-ONE", "BODY-CLOSED-TWO", "BODY-CLOSED-THREE", "BODY-CLOSED-FOUR",
	}

	var mark atomic.Int64
	out, err := runTreeTUIWithEnv(t, dir, 6500, []keyStep{
		kd("o", 1000*time.Millisecond), // status filter: open
		k("j"),
		k("j"),
		k("\r"), // open the detail view
		ka(500*time.Millisecond, touchIssue(t, db, "det-1"), &mark),
		kd("", reloadSettle),
	}, "BEADS_DOLT_PASSWORD=")
	if err != nil {
		t.Fatalf("run TUI: %v", err)
	}
	if mark.Load() == 0 {
		t.Fatal("the write never ran")
	}
	before, after := string(out[:mark.Load()]), string(out[mark.Load():])

	shown, at := "", -1
	for _, b := range bodies {
		if i := strings.LastIndex(before, b); i > at {
			shown, at = b, i
		}
	}
	if shown == "" {
		t.Fatalf("no issue body drawn before the write; output:\n%s", truncateOutput(before, 4000))
	}
	for _, b := range bodies {
		if b != shown && strings.Contains(after, b) {
			t.Errorf("detail view switched issue on reload: showed %s, then drew %s", shown, b)
		}
	}
}
