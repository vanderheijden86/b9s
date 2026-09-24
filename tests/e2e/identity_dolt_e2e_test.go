package main_test

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// Tests here create databases, so they run only against the disposable local
// Dolt server in B9S_TEST_DOLT_SCRATCH_ADDR, never the shared server behind
// the tunnel on 3306. internal/datasource's
// TestDatabaseCreatingTestsRequireScratchServer checks this file too.
const scratchAddrEnv = "B9S_TEST_DOLT_SCRATCH_ADDR"

func requireScratchServer(t *testing.T) string {
	t.Helper()
	addr := os.Getenv(scratchAddrEnv)
	if addr == "" {
		t.Skipf("%s is not set: set it to a disposable local Dolt server to run tests that write", scratchAddrEnv)
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("refusing %s: want host:port: %v", addr, err)
	}
	if port == "3306" {
		t.Fatalf("refusing %s: port 3306 is the tunnel to the shared Dolt server", addr)
	}
	if ip := net.ParseIP(host); host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		t.Fatalf("refusing %s: not a local address", addr)
	}
	return addr
}

// createIdentityProject makes a Dolt database holding issues under several
// aliases and a b9s.identities config, and a project directory pointing at it.
func createIdentityProject(t *testing.T) string {
	t.Helper()
	addr := requireScratchServer(t)
	dbName := fmt.Sprintf("b9s_e2e_ids_%d", time.Now().UnixNano())

	db, err := sql.Open("mysql", fmt.Sprintf("root@tcp(%s)/?multiStatements=true", addr))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec("DROP DATABASE IF EXISTS `" + dbName + "`")
		db.Close()
	})
	stmts := []string{
		"CREATE DATABASE `" + dbName + "`",
		"USE `" + dbName + "`",
		`CREATE TABLE issues (
			id VARCHAR(255) PRIMARY KEY, title VARCHAR(500) NOT NULL,
			description TEXT NOT NULL DEFAULT '', status VARCHAR(32) NOT NULL DEFAULT 'open',
			priority INT NOT NULL DEFAULT 2, issue_type VARCHAR(32) NOT NULL DEFAULT 'task',
			assignee VARCHAR(255), estimated_minutes INT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			due_at DATETIME, closed_at DATETIME, external_ref VARCHAR(255),
			compaction_level INT DEFAULT 0, compacted_at DATETIME, compacted_at_commit VARCHAR(64),
			original_size INT, design TEXT NOT NULL DEFAULT '', acceptance_criteria TEXT NOT NULL DEFAULT '',
			notes TEXT NOT NULL DEFAULT '', source_repo VARCHAR(512) DEFAULT '',
			created_by VARCHAR(255) DEFAULT '', owner VARCHAR(255) DEFAULT '', defer_until DATETIME)`,
		`CREATE TABLE labels (issue_id VARCHAR(255) NOT NULL, label VARCHAR(255) NOT NULL, PRIMARY KEY (issue_id, label))`,
		`CREATE TABLE dependencies (id CHAR(36) NOT NULL PRIMARY KEY, issue_id VARCHAR(255) NOT NULL,
			type VARCHAR(32) NOT NULL DEFAULT 'blocks', created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			created_by VARCHAR(255) NOT NULL DEFAULT '', metadata JSON, thread_id VARCHAR(255),
			depends_on_issue_id VARCHAR(255), depends_on_wisp_id VARCHAR(255), depends_on_external VARCHAR(255))`,
		`CREATE TABLE comments (id CHAR(36) NOT NULL PRIMARY KEY, issue_id VARCHAR(255) NOT NULL,
			author VARCHAR(255) NOT NULL, text TEXT NOT NULL, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		"CREATE TABLE config (`key` VARCHAR(255) PRIMARY KEY, value TEXT NOT NULL)",
		`INSERT INTO issues (id, title, assignee, created_by) VALUES
			('ids-1', 'First by login', 'e2e-login', 'e2e-login'),
			('ids-2', 'Second by email', 'e2e@example.invalid', 'e2e-login'),
			('ids-3', 'Third by agent', 'e2e-bot', 'e2e-bot')`,
		`INSERT INTO config VALUES
			('b9s.identities', '[{"name":"e2e-person","kind":"human","aliases":["e2e-login","e2e@example.invalid"]},{"name":"e2e-agents","kind":"agent","aliases":["e2e-bot"]}]')`,
		"CALL DOLT_COMMIT('-Am', 'fixture')",
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", strings.Fields(stmt)[0], err)
		}
	}

	host, portStr, _ := net.SplitHostPort(addr)
	var port int
	fmt.Sscanf(portStr, "%d", &port)
	meta, _ := json.Marshal(map[string]any{
		"dolt_mode": "server", "dolt_server_host": host, "dolt_server_port": port,
		"dolt_server_user": "root", "dolt_database": dbName,
	})
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".beads", "metadata.json"), meta, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestAssigneePickerMergesAliasesE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PTY e2e test in -short mode")
	}
	dir := createIdentityProject(t)

	out, err := runTreeTUIWithEnv(t, dir, 2500, []keyStep{kd("A", 1200*time.Millisecond)}, "BEADS_DOLT_PASSWORD=")
	if err != nil {
		t.Fatalf("run TUI: %v", err)
	}
	containsAll(t, out, []string{"First by login", "e2e-person", "e2e-agents (agent)"})
	containsNone(t, out, []string{"e2e@example.invalid"})
}
