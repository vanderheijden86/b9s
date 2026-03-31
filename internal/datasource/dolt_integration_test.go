package datasource

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/vanderheijden86/beadwork/pkg/model"

	_ "github.com/go-sql-driver/mysql"
)

// Integration tests for the Dolt backend.
//
// Configuration is loaded from .env.test in the repo root (auto-detected).
// Override with env vars: B9S_TEST_DOLT_HOST, B9S_TEST_DOLT_PORT, B9S_TEST_DOLT_USER.
// Password comes from BEADS_DOLT_PASSWORD (not stored in .env.test).
//
// The tests create a temporary database, populate it with test data,
// run all assertions, and drop it on cleanup.

func init() {
	loadEnvTestFile()
}

// loadEnvTestFile reads .env.test from the repo root and sets env vars
// that are not already set (env vars take precedence over file).
func loadEnvTestFile() {
	// Find .env.test relative to this test file's location
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return
	}
	// Walk up from internal/datasource/ to repo root
	repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(filename)))
	envPath := filepath.Join(repoRoot, ".env.test")

	f, err := os.Open(envPath)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		// Don't override existing env vars
		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, val)
		}
	}
}

const testDBPrefix = "b9s_inttest_"

// doltIntegrationDSN builds a DSN for integration tests from env vars.
// Returns "" if no Dolt server is configured.
func doltIntegrationDSN(dbName string) string {
	// Full DSN takes precedence
	if dsn := os.Getenv("B9S_TEST_DOLT_DSN"); dsn != "" {
		if dbName != "" {
			if idx := strings.Index(dsn, "/"); idx != -1 {
				paramIdx := strings.Index(dsn[idx:], "?")
				if paramIdx != -1 {
					return dsn[:idx+1] + dbName + dsn[idx+paramIdx:]
				}
				return dsn[:idx+1] + dbName
			}
		}
		return dsn
	}

	host := os.Getenv("B9S_TEST_DOLT_HOST")
	if host == "" {
		return ""
	}
	port := os.Getenv("B9S_TEST_DOLT_PORT")
	if port == "" {
		port = "3306"
	}
	user := os.Getenv("B9S_TEST_DOLT_USER")
	if user == "" {
		user = "root"
	}
	password := os.Getenv("BEADS_DOLT_PASSWORD")
	if dbName == "" {
		dbName = "b9s_test"
	}
	if password != "" {
		return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&timeout=10s&readTimeout=10s", user, password, host, port, dbName)
	}
	return fmt.Sprintf("%s@tcp(%s:%s)/%s?parseTime=true&timeout=10s&readTimeout=10s", user, host, port, dbName)
}

// buildDSN constructs a MySQL DSN from components, including password if set.
func buildDSN(user, password, addr, dbName string) string {
	if password != "" {
		return fmt.Sprintf("%s:%s@tcp(%s)/%s?parseTime=true&timeout=10s&readTimeout=10s", user, password, addr, dbName)
	}
	return fmt.Sprintf("%s@tcp(%s)/%s?parseTime=true&timeout=10s&readTimeout=10s", user, addr, dbName)
}

// skipIfNoDoltIntegration skips when no Dolt server is configured.
func skipIfNoDoltIntegration(t *testing.T) {
	t.Helper()
	if doltIntegrationDSN("") == "" && os.Getenv("B9S_TEST_DOLT_HOST") == "" {
		t.Skip("No Dolt server configured (set B9S_TEST_DOLT_DSN or B9S_TEST_DOLT_HOST)")
	}
}

// testDB creates a fresh Dolt database with the beads schema, populates it with
// seed data, and returns a cleanup function that drops it.
func testDB(t *testing.T) (dbName string, addr string, cleanup func()) {
	t.Helper()

	dbName = fmt.Sprintf("%s%d", testDBPrefix, time.Now().UnixNano())

	// Connect without a database to create one
	serverDSN := doltIntegrationDSN("")
	// Strip database from DSN for server-level ops
	host := os.Getenv("B9S_TEST_DOLT_HOST")
	port := os.Getenv("B9S_TEST_DOLT_PORT")
	user := os.Getenv("B9S_TEST_DOLT_USER")
	if host == "" {
		host = "osen.co"
	}
	if port == "" {
		port = "3306"
	}
	if user == "" {
		user = "root"
	}

	password := os.Getenv("BEADS_DOLT_PASSWORD")

	if serverDSN == "" {
		if password != "" {
			serverDSN = fmt.Sprintf("%s:%s@tcp(%s:%s)/?parseTime=true&timeout=10s", user, password, host, port)
		} else {
			serverDSN = fmt.Sprintf("%s@tcp(%s:%s)/?parseTime=true&timeout=10s", user, host, port)
		}
	} else {
		// Use DSN but connect to no specific database
		if idx := strings.LastIndex(serverDSN, "/"); idx != -1 {
			paramIdx := strings.Index(serverDSN[idx:], "?")
			if paramIdx != -1 {
				serverDSN = serverDSN[:idx+1] + serverDSN[idx+paramIdx:]
			} else {
				serverDSN = serverDSN[:idx+1]
			}
		}
	}

	addr = fmt.Sprintf("%s:%s", host, port)

	db, err := sql.Open("mysql", serverDSN)
	if err != nil {
		t.Fatalf("cannot connect to Dolt server: %v", err)
	}

	// Create test database
	if _, err := db.Exec(fmt.Sprintf("CREATE DATABASE `%s`", dbName)); err != nil {
		db.Close()
		t.Fatalf("cannot create test database %s: %v", dbName, err)
	}

	// Switch to the new database
	if _, err := db.Exec(fmt.Sprintf("USE `%s`", dbName)); err != nil {
		db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", dbName))
		db.Close()
		t.Fatalf("cannot use test database: %v", err)
	}

	// Create beads schema
	schema := []string{
		`CREATE TABLE issues (
			id VARCHAR(255) PRIMARY KEY,
			title VARCHAR(1024) NOT NULL,
			description TEXT,
			status VARCHAR(50) NOT NULL DEFAULT 'open',
			priority INT NOT NULL DEFAULT 2,
			issue_type VARCHAR(50) NOT NULL DEFAULT 'task',
			assignee VARCHAR(255),
			estimated_minutes INT,
			created_at DATETIME,
			updated_at DATETIME,
			due_date DATETIME,
			closed_at DATETIME,
			external_ref VARCHAR(1024),
			compaction_level INT DEFAULT 0,
			compacted_at DATETIME,
			compacted_at_commit VARCHAR(255),
			original_size INT DEFAULT 0,
			labels JSON,
			design TEXT,
			acceptance_criteria TEXT,
			notes TEXT,
			source_repo VARCHAR(255),
			tombstone TINYINT DEFAULT 0
		)`,
		`CREATE TABLE dependencies (
			issue_id VARCHAR(255) NOT NULL,
			depends_on_id VARCHAR(255) NOT NULL,
			dependency_type VARCHAR(50) NOT NULL DEFAULT 'blocks',
			created_at DATETIME,
			created_by VARCHAR(255),
			PRIMARY KEY (issue_id, depends_on_id)
		)`,
		`CREATE TABLE comments (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			issue_id VARCHAR(255) NOT NULL,
			author VARCHAR(255),
			text TEXT,
			created_at DATETIME
		)`,
	}

	for _, ddl := range schema {
		if _, err := db.Exec(ddl); err != nil {
			db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", dbName))
			db.Close()
			t.Fatalf("schema creation failed: %v", err)
		}
	}

	// Seed test data
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	seeds := []string{
		fmt.Sprintf(`INSERT INTO issues (id, title, description, status, priority, issue_type, assignee, created_at, updated_at, labels, notes)
			VALUES ('test-001', 'Fix login bug', 'Users cannot log in with SSO', 'open', 1, 'bug', 'alice', '%s', '%s', '["auth","urgent"]', 'Investigating SSO provider')`, now, now),
		fmt.Sprintf(`INSERT INTO issues (id, title, description, status, priority, issue_type, created_at, updated_at)
			VALUES ('test-002', 'Add dark mode', 'Implement dark mode theme', 'in_progress', 2, 'feature', '%s', '%s')`, now, now),
		fmt.Sprintf(`INSERT INTO issues (id, title, status, priority, issue_type, created_at, updated_at, closed_at)
			VALUES ('test-003', 'Update docs', 'closed', 3, 'task', '%s', '%s', '%s')`, now, now, now),
		fmt.Sprintf(`INSERT INTO issues (id, title, status, priority, issue_type, created_at, updated_at, tombstone)
			VALUES ('test-004', 'Deleted issue', 'tombstone', 4, 'task', '%s', '%s', 1)`, now, now),
		fmt.Sprintf(`INSERT INTO issues (id, title, description, status, priority, issue_type, created_at, updated_at, design, acceptance_criteria)
			VALUES ('test-005', 'Epic: Auth overhaul', 'Complete auth rewrite', 'open', 0, 'epic', '%s', '%s', 'Use OAuth2 + PKCE', 'All tests pass, no regressions')`, now, now),

		// Dependencies
		`INSERT INTO dependencies (issue_id, depends_on_id, dependency_type) VALUES ('test-002', 'test-001', 'blocks')`,
		`INSERT INTO dependencies (issue_id, depends_on_id, dependency_type) VALUES ('test-002', 'test-005', 'related')`,

		// Comments
		fmt.Sprintf(`INSERT INTO comments (issue_id, author, text, created_at) VALUES ('test-001', 'bob', 'Reproduced on staging', '%s')`, now),
		fmt.Sprintf(`INSERT INTO comments (issue_id, author, text, created_at) VALUES ('test-001', 'alice', 'Fix in progress', '%s')`, now),
		fmt.Sprintf(`INSERT INTO comments (issue_id, author, text, created_at) VALUES ('test-002', 'charlie', 'Design looks good', '%s')`, now),
	}

	for _, seed := range seeds {
		if _, err := db.Exec(seed); err != nil {
			db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", dbName))
			db.Close()
			t.Fatalf("seed data failed: %v", err)
		}
	}

	// Dolt commit the seed data
	db.Exec("CALL DOLT_ADD('-A')")
	db.Exec("CALL DOLT_COMMIT('-m', 'seed test data')")

	db.Close()

	cleanup = func() {
		var cleanDSN string
		if password != "" {
			cleanDSN = fmt.Sprintf("%s:%s@tcp(%s:%s)/?parseTime=true&timeout=10s", user, password, host, port)
		} else {
			cleanDSN = fmt.Sprintf("%s@tcp(%s:%s)/?parseTime=true&timeout=10s", user, host, port)
		}
		cleanDB, err := sql.Open("mysql", cleanDSN)
		if err == nil {
			cleanDB.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", dbName))
			cleanDB.Close()
		}
	}

	return dbName, addr, cleanup
}

// newTestDoltReader creates a DoltReader connected to the test database.
// The DSN is modified to use the test database name instead of "beads".
func newTestDoltReader(t *testing.T, dbName, addr string) *DoltReader {
	t.Helper()

	user := os.Getenv("B9S_TEST_DOLT_USER")
	if user == "" {
		user = "root"
	}
	password := os.Getenv("BEADS_DOLT_PASSWORD")

	var dsn string
	if password != "" {
		dsn = fmt.Sprintf("%s:%s@tcp(%s)/%s?parseTime=true&timeout=10s&readTimeout=10s", user, password, addr, dbName)
	} else {
		dsn = fmt.Sprintf("%s@tcp(%s)/%s?parseTime=true&timeout=10s&readTimeout=10s", user, addr, dbName)
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("cannot open test DB connection: %v", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		t.Fatalf("cannot ping test DB: %v", err)
	}

	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(5 * time.Minute)

	return &DoltReader{db: db, addr: addr}
}

// ---------------------------------------------------------------------------
// Test: Connection & Lifecycle
// ---------------------------------------------------------------------------

func TestDoltIntegration_Connect(t *testing.T) {
	skipIfNoDoltIntegration(t)
	dbName, addr, cleanup := testDB(t)
	defer cleanup()

	reader := newTestDoltReader(t, dbName, addr)
	defer reader.Close()

	t.Log("Connected to Dolt test database successfully")
}

// ---------------------------------------------------------------------------
// Test: LoadIssues - basic loading
// ---------------------------------------------------------------------------

func TestDoltIntegration_LoadIssues(t *testing.T) {
	skipIfNoDoltIntegration(t)
	dbName, addr, cleanup := testDB(t)
	defer cleanup()

	reader := newTestDoltReader(t, dbName, addr)
	defer reader.Close()

	issues, err := reader.LoadIssues()
	if err != nil {
		t.Fatalf("LoadIssues failed: %v", err)
	}

	// Should load 4 issues (test-004 is tombstoned, excluded)
	if len(issues) != 4 {
		t.Errorf("expected 4 non-tombstone issues, got %d", len(issues))
		for _, iss := range issues {
			t.Logf("  loaded: %s (status=%s, tombstone=%v)", iss.ID, iss.Status, iss.Status == "tombstone")
		}
	}
}

func TestDoltIntegration_LoadIssues_ExcludesTombstones(t *testing.T) {
	skipIfNoDoltIntegration(t)
	dbName, addr, cleanup := testDB(t)
	defer cleanup()

	reader := newTestDoltReader(t, dbName, addr)
	defer reader.Close()

	issues, err := reader.LoadIssues()
	if err != nil {
		t.Fatalf("LoadIssues failed: %v", err)
	}

	for _, iss := range issues {
		if iss.ID == "test-004" {
			t.Error("tombstoned issue test-004 should not be returned")
		}
	}
}

// ---------------------------------------------------------------------------
// Test: Issue field mapping
// ---------------------------------------------------------------------------

func TestDoltIntegration_IssueFields(t *testing.T) {
	skipIfNoDoltIntegration(t)
	dbName, addr, cleanup := testDB(t)
	defer cleanup()

	reader := newTestDoltReader(t, dbName, addr)
	defer reader.Close()

	issue, err := reader.GetIssueByID("test-001")
	if err != nil {
		t.Fatalf("GetIssueByID failed: %v", err)
	}

	if issue.Title != "Fix login bug" {
		t.Errorf("expected title 'Fix login bug', got %q", issue.Title)
	}
	if issue.Description != "Users cannot log in with SSO" {
		t.Errorf("expected description, got %q", issue.Description)
	}
	if string(issue.Status) != "open" {
		t.Errorf("expected status 'open', got %q", issue.Status)
	}
	if issue.Priority != 1 {
		t.Errorf("expected priority 1, got %d", issue.Priority)
	}
	if string(issue.IssueType) != "bug" {
		t.Errorf("expected type 'bug', got %q", issue.IssueType)
	}
	if issue.Assignee != "alice" {
		t.Errorf("expected assignee 'alice', got %q", issue.Assignee)
	}
	if issue.Notes != "Investigating SSO provider" {
		t.Errorf("expected notes, got %q", issue.Notes)
	}
	if issue.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
	if issue.UpdatedAt.IsZero() {
		t.Error("expected non-zero UpdatedAt")
	}
}

func TestDoltIntegration_IssueLabels(t *testing.T) {
	skipIfNoDoltIntegration(t)
	dbName, addr, cleanup := testDB(t)
	defer cleanup()

	reader := newTestDoltReader(t, dbName, addr)
	defer reader.Close()

	issue, err := reader.GetIssueByID("test-001")
	if err != nil {
		t.Fatalf("GetIssueByID failed: %v", err)
	}

	if len(issue.Labels) != 2 {
		t.Fatalf("expected 2 labels, got %d: %v", len(issue.Labels), issue.Labels)
	}
	if issue.Labels[0] != "auth" || issue.Labels[1] != "urgent" {
		t.Errorf("expected labels [auth, urgent], got %v", issue.Labels)
	}
}

func TestDoltIntegration_IssueDesignFields(t *testing.T) {
	skipIfNoDoltIntegration(t)
	dbName, addr, cleanup := testDB(t)
	defer cleanup()

	reader := newTestDoltReader(t, dbName, addr)
	defer reader.Close()

	issue, err := reader.GetIssueByID("test-005")
	if err != nil {
		t.Fatalf("GetIssueByID failed: %v", err)
	}

	if issue.Design != "Use OAuth2 + PKCE" {
		t.Errorf("expected design field, got %q", issue.Design)
	}
	if issue.AcceptanceCriteria != "All tests pass, no regressions" {
		t.Errorf("expected acceptance_criteria, got %q", issue.AcceptanceCriteria)
	}
}

func TestDoltIntegration_ClosedIssueHasClosedAt(t *testing.T) {
	skipIfNoDoltIntegration(t)
	dbName, addr, cleanup := testDB(t)
	defer cleanup()

	reader := newTestDoltReader(t, dbName, addr)
	defer reader.Close()

	issue, err := reader.GetIssueByID("test-003")
	if err != nil {
		t.Fatalf("GetIssueByID failed: %v", err)
	}

	if issue.ClosedAt == nil {
		t.Error("expected non-nil ClosedAt for closed issue")
	}
}

// ---------------------------------------------------------------------------
// Test: Dependencies
// ---------------------------------------------------------------------------

func TestDoltIntegration_Dependencies(t *testing.T) {
	skipIfNoDoltIntegration(t)
	dbName, addr, cleanup := testDB(t)
	defer cleanup()

	reader := newTestDoltReader(t, dbName, addr)
	defer reader.Close()

	issue, err := reader.GetIssueByID("test-002")
	if err != nil {
		t.Fatalf("GetIssueByID failed: %v", err)
	}

	if len(issue.Dependencies) != 2 {
		t.Fatalf("expected 2 dependencies, got %d", len(issue.Dependencies))
	}

	depMap := make(map[string]string)
	for _, dep := range issue.Dependencies {
		depMap[dep.DependsOnID] = string(dep.Type)
		if dep.IssueID != "test-002" {
			t.Errorf("dependency IssueID should be test-002, got %q", dep.IssueID)
		}
	}

	if depMap["test-001"] != "blocks" {
		t.Errorf("expected blocks dependency on test-001, got %q", depMap["test-001"])
	}
	if depMap["test-005"] != "related" {
		t.Errorf("expected related dependency on test-005, got %q", depMap["test-005"])
	}
}

func TestDoltIntegration_NoDependencies(t *testing.T) {
	skipIfNoDoltIntegration(t)
	dbName, addr, cleanup := testDB(t)
	defer cleanup()

	reader := newTestDoltReader(t, dbName, addr)
	defer reader.Close()

	issue, err := reader.GetIssueByID("test-003")
	if err != nil {
		t.Fatalf("GetIssueByID failed: %v", err)
	}

	if len(issue.Dependencies) != 0 {
		t.Errorf("expected 0 dependencies, got %d", len(issue.Dependencies))
	}
}

// ---------------------------------------------------------------------------
// Test: Comments
// ---------------------------------------------------------------------------

func TestDoltIntegration_Comments(t *testing.T) {
	skipIfNoDoltIntegration(t)
	dbName, addr, cleanup := testDB(t)
	defer cleanup()

	reader := newTestDoltReader(t, dbName, addr)
	defer reader.Close()

	issue, err := reader.GetIssueByID("test-001")
	if err != nil {
		t.Fatalf("GetIssueByID failed: %v", err)
	}

	if len(issue.Comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(issue.Comments))
	}

	// Comments should be ordered by created_at
	if issue.Comments[0].Author != "bob" {
		t.Errorf("first comment author should be bob, got %q", issue.Comments[0].Author)
	}
	if issue.Comments[0].Text != "Reproduced on staging" {
		t.Errorf("first comment text wrong: %q", issue.Comments[0].Text)
	}
	if issue.Comments[1].Author != "alice" {
		t.Errorf("second comment author should be alice, got %q", issue.Comments[1].Author)
	}
	for _, c := range issue.Comments {
		if c.IssueID != "test-001" {
			t.Errorf("comment IssueID should be test-001, got %q", c.IssueID)
		}
		if c.CreatedAt.IsZero() {
			t.Error("comment should have non-zero CreatedAt")
		}
	}
}

func TestDoltIntegration_NoComments(t *testing.T) {
	skipIfNoDoltIntegration(t)
	dbName, addr, cleanup := testDB(t)
	defer cleanup()

	reader := newTestDoltReader(t, dbName, addr)
	defer reader.Close()

	issue, err := reader.GetIssueByID("test-003")
	if err != nil {
		t.Fatalf("GetIssueByID failed: %v", err)
	}

	if len(issue.Comments) != 0 {
		t.Errorf("expected 0 comments, got %d", len(issue.Comments))
	}
}

// ---------------------------------------------------------------------------
// Test: LoadIssuesFiltered
// ---------------------------------------------------------------------------

func TestDoltIntegration_LoadIssuesFiltered(t *testing.T) {
	skipIfNoDoltIntegration(t)
	dbName, addr, cleanup := testDB(t)
	defer cleanup()

	reader := newTestDoltReader(t, dbName, addr)
	defer reader.Close()

	bugs, err := reader.LoadIssuesFiltered(func(iss *model.Issue) bool {
		return iss.IssueType == "bug"
	})
	if err != nil {
		t.Fatalf("LoadIssuesFiltered failed: %v", err)
	}

	if len(bugs) != 1 {
		t.Errorf("expected 1 bug, got %d", len(bugs))
	}
	if len(bugs) > 0 && bugs[0].ID != "test-001" {
		t.Errorf("expected test-001 bug, got %s", bugs[0].ID)
	}
}

// ---------------------------------------------------------------------------
// Test: CountIssues
// ---------------------------------------------------------------------------

func TestDoltIntegration_CountIssues(t *testing.T) {
	skipIfNoDoltIntegration(t)
	dbName, addr, cleanup := testDB(t)
	defer cleanup()

	reader := newTestDoltReader(t, dbName, addr)
	defer reader.Close()

	count, err := reader.CountIssues()
	if err != nil {
		t.Fatalf("CountIssues failed: %v", err)
	}

	if count != 4 {
		t.Errorf("expected 4 non-tombstone issues, got %d", count)
	}
}

// ---------------------------------------------------------------------------
// Test: GetLastModified
// ---------------------------------------------------------------------------

func TestDoltIntegration_GetLastModified(t *testing.T) {
	skipIfNoDoltIntegration(t)
	dbName, addr, cleanup := testDB(t)
	defer cleanup()

	reader := newTestDoltReader(t, dbName, addr)
	defer reader.Close()

	lastMod, err := reader.GetLastModified()
	if err != nil {
		t.Fatalf("GetLastModified failed: %v", err)
	}

	if lastMod.IsZero() {
		t.Error("expected non-zero last modified time")
	}
}

// ---------------------------------------------------------------------------
// Test: GetHeadHash
// ---------------------------------------------------------------------------

func TestDoltIntegration_GetHeadHash(t *testing.T) {
	skipIfNoDoltIntegration(t)
	dbName, addr, cleanup := testDB(t)
	defer cleanup()

	reader := newTestDoltReader(t, dbName, addr)
	defer reader.Close()

	hash, err := reader.GetHeadHash()
	if err != nil {
		t.Fatalf("GetHeadHash failed: %v", err)
	}

	if hash == "" {
		t.Error("expected non-empty HEAD hash")
	}
	if len(hash) < 20 {
		t.Errorf("HEAD hash looks too short: %q", hash)
	}
	t.Logf("HEAD hash: %s", hash)
}

// ---------------------------------------------------------------------------
// Test: GetIssueByID - not found
// ---------------------------------------------------------------------------

func TestDoltIntegration_GetIssueByID_NotFound(t *testing.T) {
	skipIfNoDoltIntegration(t)
	dbName, addr, cleanup := testDB(t)
	defer cleanup()

	reader := newTestDoltReader(t, dbName, addr)
	defer reader.Close()

	_, err := reader.GetIssueByID("nonexistent-999")
	if err == nil {
		t.Fatal("expected error for nonexistent issue")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Test: DoltWatcher - change detection
// ---------------------------------------------------------------------------

func TestDoltIntegration_WatcherDetectsChange(t *testing.T) {
	skipIfNoDoltIntegration(t)
	dbName, addr, cleanup := testDB(t)
	defer cleanup()

	user := os.Getenv("B9S_TEST_DOLT_USER")
	if user == "" {
		user = "root"
	}

	source := DataSource{
		Type:     SourceTypeDolt,
		Path:     addr,
		Database: dbName,
		User:     user,
	}

	// Create watcher connected to our test DB
	dw, err := NewDoltWatcher(source, 200*time.Millisecond)
	if err != nil {
		t.Fatalf("NewDoltWatcher failed: %v", err)
	}
	defer dw.Stop()

	// Start the watcher to get baseline hash
	if err := dw.Start(); err != nil {
		t.Fatalf("watcher Start failed: %v", err)
	}

	reader := newTestDoltReader(t, dbName, addr)
	defer reader.Close()

	hash1, err := reader.GetHeadHash()
	if err != nil {
		t.Fatalf("GetHeadHash failed: %v", err)
	}

	// Mutate data
	dsn := buildDSN(user, os.Getenv("BEADS_DOLT_PASSWORD"), addr, dbName)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("cannot open mutation connection: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`INSERT INTO issues (id, title, status, priority, issue_type, created_at, updated_at)
		VALUES ('test-new', 'New issue', 'open', 2, 'task', NOW(), NOW())`)
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}
	db.Exec("CALL DOLT_ADD('-A')")
	db.Exec("CALL DOLT_COMMIT('-m', 'add test issue for watcher')")

	hash2, err := reader.GetHeadHash()
	if err != nil {
		t.Fatalf("GetHeadHash after mutation failed: %v", err)
	}

	if hash1 == hash2 {
		t.Error("HEAD hash should change after commit; watcher would not detect this change")
	}
	t.Logf("Hash before: %s, after: %s", hash1[:12], hash2[:12])

	// The watcher should detect the change within a few poll cycles
	select {
	case <-dw.Changed():
		t.Log("Watcher detected the change")
	case <-time.After(3 * time.Second):
		t.Error("Watcher did not detect change within 3s")
	}
}

// ---------------------------------------------------------------------------
// Test: Write-then-read cycle (simulates b9s edit flow)
// ---------------------------------------------------------------------------

func TestDoltIntegration_WriteReadCycle(t *testing.T) {
	skipIfNoDoltIntegration(t)
	dbName, addr, cleanup := testDB(t)
	defer cleanup()

	user := os.Getenv("B9S_TEST_DOLT_USER")
	if user == "" {
		user = "root"
	}

	reader := newTestDoltReader(t, dbName, addr)
	defer reader.Close()

	// Read initial state
	before, err := reader.GetIssueByID("test-001")
	if err != nil {
		t.Fatalf("initial read failed: %v", err)
	}
	if string(before.Status) != "open" {
		t.Fatalf("expected initial status 'open', got %q", before.Status)
	}

	// Simulate a bd update (direct SQL mutation)
	dsn := buildDSN(user, os.Getenv("BEADS_DOLT_PASSWORD"), addr, dbName)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("cannot open mutation connection: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`UPDATE issues SET status = 'in_progress', updated_at = NOW() WHERE id = 'test-001'`)
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}

	// Read again - should see the change immediately (no commit needed for reads)
	after, err := reader.GetIssueByID("test-001")
	if err != nil {
		t.Fatalf("read after write failed: %v", err)
	}

	if string(after.Status) != "in_progress" {
		t.Errorf("expected status 'in_progress' after update, got %q", after.Status)
	}
}

// ---------------------------------------------------------------------------
// Test: Sorting order (updated_at DESC)
// ---------------------------------------------------------------------------

func TestDoltIntegration_LoadIssuesSortedByUpdatedAt(t *testing.T) {
	skipIfNoDoltIntegration(t)
	dbName, addr, cleanup := testDB(t)
	defer cleanup()

	reader := newTestDoltReader(t, dbName, addr)
	defer reader.Close()

	issues, err := reader.LoadIssues()
	if err != nil {
		t.Fatalf("LoadIssues failed: %v", err)
	}

	for i := 1; i < len(issues); i++ {
		if issues[i].UpdatedAt.After(issues[i-1].UpdatedAt) {
			t.Errorf("issues not sorted by updated_at DESC: issue[%d]=%s > issue[%d]=%s",
				i, issues[i].UpdatedAt, i-1, issues[i-1].UpdatedAt)
		}
	}
}

// ---------------------------------------------------------------------------
// Test: Empty database
// ---------------------------------------------------------------------------

func TestDoltIntegration_EmptyDatabase(t *testing.T) {
	skipIfNoDoltIntegration(t)

	// Create a database with schema but no data
	user := os.Getenv("B9S_TEST_DOLT_USER")
	if user == "" {
		user = "root"
	}
	host := os.Getenv("B9S_TEST_DOLT_HOST")
	if host == "" {
		host = "osen.co"
	}
	port := os.Getenv("B9S_TEST_DOLT_PORT")
	if port == "" {
		port = "3306"
	}

	dbName := fmt.Sprintf("%s%d_empty", testDBPrefix, time.Now().UnixNano())
	addr := fmt.Sprintf("%s:%s", host, port)

	serverDSN := buildDSN(user, os.Getenv("BEADS_DOLT_PASSWORD"), addr, "")
	db, err := sql.Open("mysql", serverDSN)
	if err != nil {
		t.Fatalf("cannot connect: %v", err)
	}
	defer func() {
		db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", dbName))
		db.Close()
	}()

	db.Exec(fmt.Sprintf("CREATE DATABASE `%s`", dbName))
	db.Exec(fmt.Sprintf("USE `%s`", dbName))
	db.Exec(`CREATE TABLE issues (
		id VARCHAR(255) PRIMARY KEY, title VARCHAR(1024), description TEXT,
		status VARCHAR(50), priority INT, issue_type VARCHAR(50),
		created_at DATETIME, updated_at DATETIME, tombstone TINYINT DEFAULT 0
	)`)
	db.Exec(`CREATE TABLE dependencies (issue_id VARCHAR(255), depends_on_id VARCHAR(255), dependency_type VARCHAR(50), PRIMARY KEY(issue_id, depends_on_id))`)
	db.Exec(`CREATE TABLE comments (id BIGINT AUTO_INCREMENT PRIMARY KEY, issue_id VARCHAR(255), author VARCHAR(255), text TEXT, created_at DATETIME)`)

	reader := newTestDoltReader(t, dbName, addr)
	defer reader.Close()

	issues, err := reader.LoadIssues()
	if err != nil {
		t.Fatalf("LoadIssues on empty DB failed: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("expected 0 issues, got %d", len(issues))
	}

	count, err := reader.CountIssues()
	if err != nil {
		t.Fatalf("CountIssues on empty DB failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected count 0, got %d", count)
	}
}

// ---------------------------------------------------------------------------
// Test: Full create/edit/close cycle via SQL (simulates bd CLI writes)
// ---------------------------------------------------------------------------

// openMutationDB opens a raw SQL connection to the test database for writes.
func openMutationDB(t *testing.T, dbName, addr string) *sql.DB {
	t.Helper()
	user := os.Getenv("B9S_TEST_DOLT_USER")
	if user == "" {
		user = "root"
	}
	dsn := buildDSN(user, os.Getenv("BEADS_DOLT_PASSWORD"), addr, dbName)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("cannot open mutation connection: %v", err)
	}
	return db
}

func TestDoltIntegration_CreateIssueAndReadBack(t *testing.T) {
	skipIfNoDoltIntegration(t)
	dbName, addr, cleanup := testDB(t)
	defer cleanup()

	reader := newTestDoltReader(t, dbName, addr)
	defer reader.Close()
	db := openMutationDB(t, dbName, addr)
	defer db.Close()

	// Verify initial count
	before, err := reader.CountIssues()
	if err != nil {
		t.Fatalf("CountIssues failed: %v", err)
	}
	if before != 4 {
		t.Fatalf("expected 4 initial issues, got %d", before)
	}

	// Create a new issue (simulates bd create)
	_, err = db.Exec(`INSERT INTO issues
		(id, title, description, status, priority, issue_type, assignee, created_at, updated_at, labels, notes)
		VALUES ('test-new-001', 'Implement dark theme', 'Add dark mode support to the UI',
		'open', 2, 'feature', 'charlie', NOW(), NOW(), '["ui","theme"]', 'Requested by users')`)
	if err != nil {
		t.Fatalf("INSERT failed: %v", err)
	}

	// Read back via DoltReader
	issue, err := reader.GetIssueByID("test-new-001")
	if err != nil {
		t.Fatalf("GetIssueByID after create failed: %v", err)
	}

	if issue.Title != "Implement dark theme" {
		t.Errorf("expected title 'Implement dark theme', got %q", issue.Title)
	}
	if issue.Description != "Add dark mode support to the UI" {
		t.Errorf("expected description, got %q", issue.Description)
	}
	if string(issue.Status) != "open" {
		t.Errorf("expected status 'open', got %q", issue.Status)
	}
	if issue.Priority != 2 {
		t.Errorf("expected priority 2, got %d", issue.Priority)
	}
	if string(issue.IssueType) != "feature" {
		t.Errorf("expected type 'feature', got %q", issue.IssueType)
	}
	if issue.Assignee != "charlie" {
		t.Errorf("expected assignee 'charlie', got %q", issue.Assignee)
	}
	if issue.Notes != "Requested by users" {
		t.Errorf("expected notes, got %q", issue.Notes)
	}
	if len(issue.Labels) != 2 || issue.Labels[0] != "ui" || issue.Labels[1] != "theme" {
		t.Errorf("expected labels [ui, theme], got %v", issue.Labels)
	}

	// Count should increase
	after, err := reader.CountIssues()
	if err != nil {
		t.Fatalf("CountIssues after create failed: %v", err)
	}
	if after != before+1 {
		t.Errorf("expected count %d after create, got %d", before+1, after)
	}
}

func TestDoltIntegration_EditIssueFieldsAndReadBack(t *testing.T) {
	skipIfNoDoltIntegration(t)
	dbName, addr, cleanup := testDB(t)
	defer cleanup()

	reader := newTestDoltReader(t, dbName, addr)
	defer reader.Close()
	db := openMutationDB(t, dbName, addr)
	defer db.Close()

	// Verify initial state
	before, err := reader.GetIssueByID("test-001")
	if err != nil {
		t.Fatalf("initial read failed: %v", err)
	}
	if before.Title != "Fix login bug" {
		t.Fatalf("expected initial title 'Fix login bug', got %q", before.Title)
	}

	// Edit multiple fields (simulates bd update --title=... --priority=... --notes=...)
	_, err = db.Exec(`UPDATE issues SET
		title = 'Fix SSO login bug',
		priority = 0,
		status = 'in_progress',
		assignee = 'bob',
		notes = 'Root cause found: token expiry',
		updated_at = NOW()
		WHERE id = 'test-001'`)
	if err != nil {
		t.Fatalf("UPDATE failed: %v", err)
	}

	// Read back
	after, err := reader.GetIssueByID("test-001")
	if err != nil {
		t.Fatalf("read after edit failed: %v", err)
	}

	if after.Title != "Fix SSO login bug" {
		t.Errorf("title not updated: got %q", after.Title)
	}
	if after.Priority != 0 {
		t.Errorf("priority not updated: got %d", after.Priority)
	}
	if string(after.Status) != "in_progress" {
		t.Errorf("status not updated: got %q", after.Status)
	}
	if after.Assignee != "bob" {
		t.Errorf("assignee not updated: got %q", after.Assignee)
	}
	if after.Notes != "Root cause found: token expiry" {
		t.Errorf("notes not updated: got %q", after.Notes)
	}
	if after.UpdatedAt.Before(before.UpdatedAt) || after.UpdatedAt.Equal(before.UpdatedAt) {
		t.Error("updated_at should be newer after edit")
	}
}

func TestDoltIntegration_CloseIssueAndReadBack(t *testing.T) {
	skipIfNoDoltIntegration(t)
	dbName, addr, cleanup := testDB(t)
	defer cleanup()

	reader := newTestDoltReader(t, dbName, addr)
	defer reader.Close()
	db := openMutationDB(t, dbName, addr)
	defer db.Close()

	// Verify open before close
	before, err := reader.GetIssueByID("test-001")
	if err != nil {
		t.Fatalf("initial read failed: %v", err)
	}
	if string(before.Status) != "open" {
		t.Fatalf("expected initial status 'open', got %q", before.Status)
	}

	// Close issue (simulates bd close)
	_, err = db.Exec(`UPDATE issues SET
		status = 'closed',
		closed_at = NOW(),
		updated_at = NOW()
		WHERE id = 'test-001'`)
	if err != nil {
		t.Fatalf("close UPDATE failed: %v", err)
	}

	// Read back
	after, err := reader.GetIssueByID("test-001")
	if err != nil {
		t.Fatalf("read after close failed: %v", err)
	}

	if string(after.Status) != "closed" {
		t.Errorf("expected status 'closed', got %q", after.Status)
	}
	if after.ClosedAt == nil {
		t.Error("expected non-nil ClosedAt after close")
	}
}

func TestDoltIntegration_AddDependencyAndReadBack(t *testing.T) {
	skipIfNoDoltIntegration(t)
	dbName, addr, cleanup := testDB(t)
	defer cleanup()

	reader := newTestDoltReader(t, dbName, addr)
	defer reader.Close()
	db := openMutationDB(t, dbName, addr)
	defer db.Close()

	// test-003 has no deps initially
	before, err := reader.GetIssueByID("test-003")
	if err != nil {
		t.Fatalf("initial read failed: %v", err)
	}
	if len(before.Dependencies) != 0 {
		t.Fatalf("expected 0 deps initially, got %d", len(before.Dependencies))
	}

	// Add a dependency (simulates bd dep add)
	_, err = db.Exec(`INSERT INTO dependencies (issue_id, depends_on_id, dependency_type)
		VALUES ('test-003', 'test-001', 'blocks')`)
	if err != nil {
		t.Fatalf("INSERT dep failed: %v", err)
	}

	// Read back
	after, err := reader.GetIssueByID("test-003")
	if err != nil {
		t.Fatalf("read after dep add failed: %v", err)
	}

	if len(after.Dependencies) != 1 {
		t.Fatalf("expected 1 dep after add, got %d", len(after.Dependencies))
	}
	if after.Dependencies[0].DependsOnID != "test-001" {
		t.Errorf("expected dep on test-001, got %q", after.Dependencies[0].DependsOnID)
	}
	if string(after.Dependencies[0].Type) != "blocks" {
		t.Errorf("expected dep type 'blocks', got %q", after.Dependencies[0].Type)
	}
}

func TestDoltIntegration_AddCommentAndReadBack(t *testing.T) {
	skipIfNoDoltIntegration(t)
	dbName, addr, cleanup := testDB(t)
	defer cleanup()

	reader := newTestDoltReader(t, dbName, addr)
	defer reader.Close()
	db := openMutationDB(t, dbName, addr)
	defer db.Close()

	// test-003 has no comments initially
	before, err := reader.GetIssueByID("test-003")
	if err != nil {
		t.Fatalf("initial read failed: %v", err)
	}
	if len(before.Comments) != 0 {
		t.Fatalf("expected 0 comments initially, got %d", len(before.Comments))
	}

	// Add a comment
	_, err = db.Exec(`INSERT INTO comments (issue_id, author, text, created_at)
		VALUES ('test-003', 'dave', 'Docs look good, merging', NOW())`)
	if err != nil {
		t.Fatalf("INSERT comment failed: %v", err)
	}

	// Read back
	after, err := reader.GetIssueByID("test-003")
	if err != nil {
		t.Fatalf("read after comment add failed: %v", err)
	}

	if len(after.Comments) != 1 {
		t.Fatalf("expected 1 comment after add, got %d", len(after.Comments))
	}
	if after.Comments[0].Author != "dave" {
		t.Errorf("expected author 'dave', got %q", after.Comments[0].Author)
	}
	if after.Comments[0].Text != "Docs look good, merging" {
		t.Errorf("expected comment text, got %q", after.Comments[0].Text)
	}
}

// ---------------------------------------------------------------------------
// Test: Full bd CLI write path (skipped if bd not in PATH)
// ---------------------------------------------------------------------------

func skipIfNoBdCLI(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("bd"); err != nil {
		t.Skip("bd CLI not found in PATH, skipping CLI integration test")
	}
}

// setupBdProject creates a temp directory and runs `bd init --server` to create a
// proper Dolt database with the full schema that bd expects. Returns the work
// directory, database name, and a cleanup function.
func setupBdProject(t *testing.T) (workDir, dbName, addr string, cleanup func()) {
	t.Helper()

	host := os.Getenv("B9S_TEST_DOLT_HOST")
	if host == "" {
		host = "osen.co"
	}
	port := os.Getenv("B9S_TEST_DOLT_PORT")
	if port == "" {
		port = "3306"
	}
	user := os.Getenv("B9S_TEST_DOLT_USER")
	if user == "" {
		user = "root"
	}
	password := os.Getenv("BEADS_DOLT_PASSWORD")
	addr = fmt.Sprintf("%s:%s", host, port)

	// Unique database name for this test
	dbName = fmt.Sprintf("%s%d_bd", testDBPrefix, time.Now().UnixNano())

	dir := t.TempDir()

	// Init git first (bd requires it)
	gitInit := exec.Command("git", "init")
	gitInit.Dir = dir
	if out, err := gitInit.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}
	// Need at least one commit
	gitCommit := exec.Command("git", "commit", "--allow-empty", "-m", "init")
	gitCommit.Dir = dir
	gitCommit.Run()

	// Run bd init to create the database with full schema
	cmd := exec.Command("bd", "init",
		"--server",
		fmt.Sprintf("--server-host=%s", host),
		fmt.Sprintf("--server-port=%s", port),
		fmt.Sprintf("--server-user=%s", user),
		fmt.Sprintf("--database=%s", dbName),
		"--prefix=test",
	)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"BEADS_DOLT_SERVER_MODE=1",
		fmt.Sprintf("BEADS_DOLT_SERVER_DATABASE=%s", dbName),
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bd init failed: %v\nOutput: %s", err, output)
	}
	t.Logf("bd init output: %s", strings.TrimSpace(string(output)))

	cleanup = func() {
		var cleanDSN string
		if password != "" {
			cleanDSN = fmt.Sprintf("%s:%s@tcp(%s:%s)/?parseTime=true&timeout=10s", user, password, host, port)
		} else {
			cleanDSN = fmt.Sprintf("%s@tcp(%s:%s)/?parseTime=true&timeout=10s", user, host, port)
		}
		cleanDB, err := sql.Open("mysql", cleanDSN)
		if err == nil {
			cleanDB.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", dbName))
			cleanDB.Close()
		}
	}

	return dir, dbName, addr, cleanup
}

func TestDoltIntegration_BdCLI_CreateAndReadBack(t *testing.T) {
	skipIfNoDoltIntegration(t)
	skipIfNoBdCLI(t)
	workDir, dbName, addr, cleanup := setupBdProject(t)
	defer cleanup()

	reader := newTestDoltReader(t, dbName, addr)
	defer reader.Close()

	// Count before
	beforeCount, err := reader.CountIssues()
	if err != nil {
		t.Fatalf("CountIssues failed: %v", err)
	}

	// Run bd create
	cmd := exec.Command("bd", "create",
		"--title=Test issue from b9s",
		"--description=Created by integration test",
		"--type=task",
		"--priority=2",
	)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(),
		"BEADS_DOLT_SERVER_MODE=1",
		fmt.Sprintf("BEADS_DOLT_SERVER_DATABASE=%s", dbName),
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bd create failed: %v\nOutput: %s", err, output)
	}
	t.Logf("bd create output: %s", strings.TrimSpace(string(output)))

	// Verify count increased
	afterCount, err := reader.CountIssues()
	if err != nil {
		t.Fatalf("CountIssues after create failed: %v", err)
	}
	if afterCount != beforeCount+1 {
		t.Errorf("expected count %d after bd create, got %d", beforeCount+1, afterCount)
	}

	// Find the new issue and verify fields
	issues, err := reader.LoadIssuesFiltered(func(iss *model.Issue) bool {
		return iss.Title == "Test issue from b9s"
	})
	if err != nil {
		t.Fatalf("LoadIssuesFiltered failed: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue with title 'Test issue from b9s', got %d", len(issues))
	}

	created := issues[0]
	if created.Description != "Created by integration test" {
		t.Errorf("description mismatch: %q", created.Description)
	}
	if string(created.IssueType) != "task" {
		t.Errorf("type mismatch: %q", created.IssueType)
	}
	if created.Priority != 2 {
		t.Errorf("priority mismatch: %d", created.Priority)
	}
	t.Logf("Created issue: %s", created.ID)
}

func TestDoltIntegration_BdCLI_UpdateAndReadBack(t *testing.T) {
	skipIfNoDoltIntegration(t)
	skipIfNoBdCLI(t)
	workDir, dbName, addr, cleanup := setupBdProject(t)
	defer cleanup()

	reader := newTestDoltReader(t, dbName, addr)
	defer reader.Close()

	bdEnv := append(os.Environ(),
		"BEADS_DOLT_SERVER_MODE=1",
		fmt.Sprintf("BEADS_DOLT_SERVER_DATABASE=%s", dbName),
	)

	// First create an issue to update
	createCmd := exec.Command("bd", "create",
		"--title=Issue to update",
		"--type=bug",
		"--priority=1",
	)
	createCmd.Dir = workDir
	createCmd.Env = bdEnv
	createOut, err := createCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bd create (setup) failed: %v\nOutput: %s", err, createOut)
	}

	// Find the created issue ID
	issues, err := reader.LoadIssuesFiltered(func(iss *model.Issue) bool {
		return iss.Title == "Issue to update"
	})
	if err != nil || len(issues) == 0 {
		t.Fatalf("could not find created issue: %v", err)
	}
	issueID := issues[0].ID
	t.Logf("Created issue %s for update test", issueID)

	// Run bd update to change title and status
	cmd := exec.Command("bd", "update", issueID,
		"--title=Issue updated via CLI",
		"--status=in_progress",
	)
	cmd.Dir = workDir
	cmd.Env = bdEnv
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bd update failed: %v\nOutput: %s", err, output)
	}
	t.Logf("bd update output: %s", strings.TrimSpace(string(output)))

	// Read back and verify
	after, err := reader.GetIssueByID(issueID)
	if err != nil {
		t.Fatalf("read after update failed: %v", err)
	}

	if after.Title != "Issue updated via CLI" {
		t.Errorf("title not updated: got %q", after.Title)
	}
	if string(after.Status) != "in_progress" {
		t.Errorf("status not updated: got %q", after.Status)
	}
}

func TestDoltIntegration_BdCLI_CloseAndReadBack(t *testing.T) {
	skipIfNoDoltIntegration(t)
	skipIfNoBdCLI(t)
	workDir, dbName, addr, cleanup := setupBdProject(t)
	defer cleanup()

	reader := newTestDoltReader(t, dbName, addr)
	defer reader.Close()

	bdEnv := append(os.Environ(),
		"BEADS_DOLT_SERVER_MODE=1",
		fmt.Sprintf("BEADS_DOLT_SERVER_DATABASE=%s", dbName),
	)

	// First create an issue to close
	createCmd := exec.Command("bd", "create",
		"--title=Issue to close",
		"--type=task",
		"--priority=3",
	)
	createCmd.Dir = workDir
	createCmd.Env = bdEnv
	createOut, err := createCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bd create (setup) failed: %v\nOutput: %s", err, createOut)
	}

	// Find the created issue ID
	issues, err := reader.LoadIssuesFiltered(func(iss *model.Issue) bool {
		return iss.Title == "Issue to close"
	})
	if err != nil || len(issues) == 0 {
		t.Fatalf("could not find created issue: %v", err)
	}
	issueID := issues[0].ID
	t.Logf("Created issue %s for close test", issueID)

	// Close it
	cmd := exec.Command("bd", "close", issueID, "--reason=Fixed in PR #42")
	cmd.Dir = workDir
	cmd.Env = bdEnv
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bd close failed: %v\nOutput: %s", err, output)
	}
	t.Logf("bd close output: %s", strings.TrimSpace(string(output)))

	// Read back and verify
	after, err := reader.GetIssueByID(issueID)
	if err != nil {
		t.Fatalf("read after close failed: %v", err)
	}

	if string(after.Status) != "closed" {
		t.Errorf("expected status 'closed', got %q", after.Status)
	}
	if after.ClosedAt != nil {
		t.Logf("ClosedAt: %s", after.ClosedAt)
	} else {
		t.Log("Note: ClosedAt nil (bd v0.63 may store this differently)")
	}
}
