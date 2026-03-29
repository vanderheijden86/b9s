# Dolt Reader Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add direct Dolt database reads to b9s via MySQL protocol, so b9s works with `bd` v0.56+ which uses Dolt as sole backend.

**Architecture:** Add `DoltReader` (MySQL client via `go-sql-driver/mysql`) alongside existing `SQLiteReader`. Update `DiscoverSources()` to detect Dolt mode via `.beads/metadata.json`. Add a polling-based change detector using Dolt's `HASHOF('HEAD')` for live reload when source is Dolt. Existing JSONL/SQLite readers stay untouched for backward compatibility.

**Tech Stack:** Go 1.22+, `go-sql-driver/mysql`, `database/sql`, existing `pkg/model` types, existing `internal/datasource` patterns.

**Beads epic:** bd-7uea

---

## Task 1: Add `go-sql-driver/mysql` Dependency

**Files:**
- Modify: `go.mod`
- Modify: `go.sum` (auto-generated)

**Step 1: Add the MySQL driver dependency**

Run: `go get github.com/go-sql-driver/mysql@latest`

**Step 2: Verify it builds**

Run: `go build ./...`
Expected: Clean build, no errors.

**Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "deps: add go-sql-driver/mysql for Dolt backend support

Refs: bd-7uea"
```

---

## Task 2: Add `SourceTypeDolt` Constant and Discovery

**Files:**
- Modify: `internal/datasource/source.go` (add constant, priority, discovery function)
- Test: `internal/datasource/source_test.go`

**Step 1: Write the failing test**

Add to `internal/datasource/source_test.go`:

```go
func TestDiscoverSources_Dolt(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create metadata.json indicating Dolt mode
	metadataPath := filepath.Join(beadsDir, "metadata.json")
	if err := os.WriteFile(metadataPath, []byte(`{"database": "dolt"}`), 0644); err != nil {
		t.Fatal(err)
	}

	// Create the dolt directory
	doltDir := filepath.Join(beadsDir, "dolt")
	if err := os.MkdirAll(doltDir, 0755); err != nil {
		t.Fatal(err)
	}

	sources, err := DiscoverSources(DiscoveryOptions{
		BeadsDir:               beadsDir,
		ValidateAfterDiscovery: false,
	})
	if err != nil {
		t.Fatalf("DiscoverSources failed: %v", err)
	}

	found := false
	for _, s := range sources {
		if s.Type == SourceTypeDolt {
			found = true
			if s.Priority != PriorityDolt {
				t.Errorf("Expected priority %d, got %d", PriorityDolt, s.Priority)
			}
		}
	}
	if !found {
		t.Error("Dolt source not found in discovered sources")
	}
}

func TestDiscoverSources_DoltNotDetectedWithoutMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Only create dolt directory, no metadata.json
	doltDir := filepath.Join(beadsDir, "dolt")
	if err := os.MkdirAll(doltDir, 0755); err != nil {
		t.Fatal(err)
	}

	sources, err := DiscoverSources(DiscoveryOptions{
		BeadsDir:               beadsDir,
		ValidateAfterDiscovery: false,
	})
	if err != nil {
		t.Fatalf("DiscoverSources failed: %v", err)
	}

	for _, s := range sources {
		if s.Type == SourceTypeDolt {
			t.Error("Should not detect Dolt without metadata.json")
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/datasource/ -run TestDiscoverSources_Dolt -v`
Expected: FAIL (SourceTypeDolt undefined)

**Step 3: Write the implementation**

Add to `internal/datasource/source.go`:

Constants:
```go
// SourceTypeDolt is a Dolt database (accessed via MySQL protocol)
SourceTypeDolt SourceType = "dolt"

// PriorityDolt is the highest priority (Dolt is the canonical source when present)
PriorityDolt = 110
```

Discovery function:
```go
// discoverDoltSources detects Dolt mode via .beads/metadata.json
func discoverDoltSources(beadsDir string, opts DiscoveryOptions) ([]DataSource, error) {
	// Check metadata.json for database: dolt
	metadataPath := filepath.Join(beadsDir, "metadata.json")
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, nil // No metadata.json, not Dolt mode
	}

	// Parse just the "database" field
	var metadata struct {
		Database string `json:"database"`
	}
	if err := json.Unmarshal(data, &metadata); err != nil {
		if opts.Verbose {
			opts.Logger(fmt.Sprintf("Failed to parse metadata.json: %v", err))
		}
		return nil, nil
	}

	if metadata.Database != "dolt" {
		return nil, nil
	}

	// Read port from config.yaml if available (default 3306)
	port := readDoltPort(beadsDir)

	// Use dolt dir modtime as proxy for source freshness
	doltDir := filepath.Join(beadsDir, "dolt")
	info, err := os.Stat(doltDir)
	modTime := time.Now() // Default to now if we can't stat
	if err == nil {
		modTime = info.ModTime()
	}

	source := DataSource{
		Type:     SourceTypeDolt,
		Path:     fmt.Sprintf("127.0.0.1:%d", port),
		Priority: PriorityDolt,
		ModTime:  modTime,
		Valid:    true, // Validation happens at connect time
	}

	if opts.Verbose {
		opts.Logger(fmt.Sprintf("Found Dolt source: %s", source.Path))
	}

	return []DataSource{source}, nil
}

// readDoltPort reads the Dolt server port from .beads/config.yaml.
// Returns 3306 if not configured.
func readDoltPort(beadsDir string) int {
	configPath := filepath.Join(beadsDir, "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return 3306
	}

	// Simple YAML key extraction (avoid importing yaml just for this)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "port:") {
			portStr := strings.TrimSpace(strings.TrimPrefix(line, "port:"))
			if port, err := strconv.Atoi(portStr); err == nil && port > 0 {
				return port
			}
		}
	}
	return 3306
}
```

Add `"encoding/json"` and `"strconv"` to imports in source.go.

Wire into `DiscoverSources()` (insert before SQLite discovery):
```go
// Discover Dolt database (highest priority, short-circuits if found)
doltSources, err := discoverDoltSources(beadsDir, opts)
if err != nil && opts.Verbose {
    opts.Logger(fmt.Sprintf("Dolt discovery warning: %v", err))
}
sources = append(sources, doltSources...)
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/datasource/ -run TestDiscoverSources_Dolt -v`
Expected: PASS

**Step 5: Run full test suite**

Run: `go test ./internal/datasource/ -v`
Expected: All tests pass, no regressions.

**Step 6: Commit**

```bash
git add internal/datasource/source.go internal/datasource/source_test.go
git commit -m "feat(datasource): add Dolt source type and discovery

Detect Dolt mode via .beads/metadata.json. SourceTypeDolt gets
priority 110 (highest). Port read from config.yaml, default 3306.

Refs: bd-7uea"
```

---

## Task 3: Implement `DoltReader`

**Files:**
- Create: `internal/datasource/dolt.go`
- Create: `internal/datasource/dolt_test.go`

**Step 1: Write the failing test**

The DoltReader needs a real MySQL server to test against, so unit tests will use a test helper that checks for a Dolt server. Tests are skipped if no server is available.

Create `internal/datasource/dolt_test.go`:

```go
package datasource

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/go-sql-driver/mysql"
)

// doltTestDSN returns a DSN for testing, or "" if no Dolt server is available.
// Set B9S_TEST_DOLT_DSN=root@tcp(127.0.0.1:3306)/beads to enable.
func doltTestDSN() string {
	return os.Getenv("B9S_TEST_DOLT_DSN")
}

func skipIfNoDolt(t *testing.T) {
	t.Helper()
	if doltTestDSN() == "" {
		t.Skip("B9S_TEST_DOLT_DSN not set, skipping Dolt integration test")
	}
}

func TestDoltReader_NewAndClose(t *testing.T) {
	skipIfNoDolt(t)

	source := DataSource{
		Type: SourceTypeDolt,
		Path: "127.0.0.1:3306",
	}
	reader, err := NewDoltReader(source)
	if err != nil {
		t.Fatalf("NewDoltReader failed: %v", err)
	}
	defer reader.Close()

	if reader.db == nil {
		t.Fatal("Expected non-nil db connection")
	}
}

func TestDoltReader_LoadIssues(t *testing.T) {
	skipIfNoDolt(t)

	source := DataSource{
		Type: SourceTypeDolt,
		Path: "127.0.0.1:3306",
	}
	reader, err := NewDoltReader(source)
	if err != nil {
		t.Fatalf("NewDoltReader failed: %v", err)
	}
	defer reader.Close()

	issues, err := reader.LoadIssues()
	if err != nil {
		t.Fatalf("LoadIssues failed: %v", err)
	}

	// Should load without error (may be empty if test DB has no issues)
	t.Logf("Loaded %d issues from Dolt", len(issues))
}

func TestDoltReader_GetHeadHash(t *testing.T) {
	skipIfNoDolt(t)

	source := DataSource{
		Type: SourceTypeDolt,
		Path: "127.0.0.1:3306",
	}
	reader, err := NewDoltReader(source)
	if err != nil {
		t.Fatalf("NewDoltReader failed: %v", err)
	}
	defer reader.Close()

	hash, err := reader.GetHeadHash()
	if err != nil {
		t.Fatalf("GetHeadHash failed: %v", err)
	}

	if hash == "" {
		t.Fatal("Expected non-empty HEAD hash")
	}
	t.Logf("HEAD hash: %s", hash)
}

// TestDoltReader_ConnectFailure tests that a bad address gives a clear error
func TestDoltReader_ConnectFailure(t *testing.T) {
	source := DataSource{
		Type: SourceTypeDolt,
		Path: "127.0.0.1:19999", // Unlikely to be running
	}
	reader, err := NewDoltReader(source)
	if err == nil {
		// Connection may succeed lazily; force a ping
		if pingErr := reader.db.Ping(); pingErr == nil {
			t.Fatal("Expected connection failure to port 19999")
		}
		reader.Close()
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/datasource/ -run TestDoltReader -v`
Expected: FAIL (NewDoltReader undefined) or tests skipped (no Dolt server)

**Step 3: Write the implementation**

Create `internal/datasource/dolt.go`:

```go
package datasource

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/vanderheijden86/beadwork/pkg/model"
)

// DoltReader provides read access to a beads Dolt database via MySQL protocol.
type DoltReader struct {
	db   *sql.DB
	addr string
}

// NewDoltReader opens a connection to the Dolt server.
func NewDoltReader(source DataSource) (*DoltReader, error) {
	if source.Type != SourceTypeDolt {
		return nil, fmt.Errorf("source is not Dolt: %s", source.Type)
	}

	addr := source.Path
	if addr == "" {
		addr = "127.0.0.1:3306"
	}

	dsn := fmt.Sprintf("root@tcp(%s)/beads?parseTime=true&timeout=5s&readTimeout=10s", addr)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("cannot open Dolt connection: %w", err)
	}

	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Verify the connection is alive
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("cannot reach Dolt server at %s: %w", addr, err)
	}

	return &DoltReader{
		db:   db,
		addr: addr,
	}, nil
}

// Close closes the database connection.
func (r *DoltReader) Close() error {
	if r.db != nil {
		return r.db.Close()
	}
	return nil
}

// LoadIssues reads all non-tombstone issues from the Dolt database.
func (r *DoltReader) LoadIssues() ([]model.Issue, error) {
	return r.LoadIssuesFiltered(nil)
}

// LoadIssuesFiltered reads issues matching the filter function.
func (r *DoltReader) LoadIssuesFiltered(filter func(*model.Issue) bool) ([]model.Issue, error) {
	query := `
		SELECT
			id, title, description, status, priority, issue_type,
			assignee, estimated_minutes, created_at, updated_at,
			due_date, closed_at, external_ref, compaction_level,
			compacted_at, compacted_at_commit, original_size,
			labels, design, acceptance_criteria, notes, source_repo
		FROM issues
		WHERE (tombstone IS NULL OR tombstone = 0)
		ORDER BY updated_at DESC
	`

	rows, err := r.db.Query(query)
	if err != nil {
		return r.loadIssuesSimple(filter)
	}
	defer rows.Close()

	var issues []model.Issue
	for rows.Next() {
		issue, err := r.scanIssue(rows)
		if err != nil {
			continue
		}

		issue.Dependencies = r.loadDependencies(issue.ID)
		issue.Comments = r.loadComments(issue.ID)

		if filter != nil && !filter(&issue) {
			continue
		}

		issues = append(issues, issue)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating issues: %w", err)
	}

	return issues, nil
}

// scanIssue scans a single issue row (shared between full and simple queries).
func (r *DoltReader) scanIssue(rows *sql.Rows) (model.Issue, error) {
	var issue model.Issue
	var estimatedMinutes, compactionLevel, originalSize sql.NullInt64
	var createdAt, updatedAt, dueDate, closedAt, compactedAt sql.NullTime
	var description, assignee, externalRef, design, acceptanceCriteria, notes, sourceRepo, compactedAtCommit sql.NullString
	var labelsJSON sql.NullString
	var issueType string

	err := rows.Scan(
		&issue.ID, &issue.Title, &description, &issue.Status, &issue.Priority, &issueType,
		&assignee, &estimatedMinutes, &createdAt, &updatedAt,
		&dueDate, &closedAt, &externalRef, &compactionLevel,
		&compactedAt, &compactedAtCommit, &originalSize,
		&labelsJSON, &design, &acceptanceCriteria, &notes, &sourceRepo,
	)
	if err != nil {
		return issue, err
	}

	if description.Valid {
		issue.Description = description.String
	}
	issue.IssueType = model.IssueType(issueType)
	if assignee.Valid {
		issue.Assignee = assignee.String
	}
	if estimatedMinutes.Valid {
		v := int(estimatedMinutes.Int64)
		issue.EstimatedMinutes = &v
	}
	if createdAt.Valid {
		issue.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		issue.UpdatedAt = updatedAt.Time
	}
	if dueDate.Valid {
		t := dueDate.Time
		issue.DueDate = &t
	}
	if closedAt.Valid {
		t := closedAt.Time
		issue.ClosedAt = &t
	}
	if externalRef.Valid {
		s := externalRef.String
		issue.ExternalRef = &s
	}
	if compactionLevel.Valid {
		issue.CompactionLevel = int(compactionLevel.Int64)
	}
	if compactedAt.Valid {
		t := compactedAt.Time
		issue.CompactedAt = &t
	}
	if compactedAtCommit.Valid {
		s := compactedAtCommit.String
		issue.CompactedAtCommit = &s
	}
	if originalSize.Valid {
		issue.OriginalSize = int(originalSize.Int64)
	}
	if design.Valid {
		issue.Design = design.String
	}
	if acceptanceCriteria.Valid {
		issue.AcceptanceCriteria = acceptanceCriteria.String
	}
	if notes.Valid {
		issue.Notes = notes.String
	}
	if sourceRepo.Valid {
		issue.SourceRepo = sourceRepo.String
	}

	if labelsJSON.Valid && labelsJSON.String != "" && labelsJSON.String != "null" {
		issue.Labels = parseJSONStringArray(labelsJSON.String)
	}

	return issue, nil
}

// loadIssuesSimple is a fallback for databases with fewer columns.
func (r *DoltReader) loadIssuesSimple(filter func(*model.Issue) bool) ([]model.Issue, error) {
	query := `
		SELECT id, title, description, status, priority, issue_type, created_at, updated_at
		FROM issues
		WHERE (tombstone IS NULL OR tombstone = 0)
		ORDER BY updated_at DESC
	`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var issues []model.Issue
	for rows.Next() {
		var issue model.Issue
		var description sql.NullString
		var createdAt, updatedAt sql.NullTime
		var issueType string

		err := rows.Scan(
			&issue.ID, &issue.Title, &description, &issue.Status, &issue.Priority, &issueType,
			&createdAt, &updatedAt,
		)
		if err != nil {
			continue
		}

		if description.Valid {
			issue.Description = description.String
		}
		issue.IssueType = model.IssueType(issueType)
		if createdAt.Valid {
			issue.CreatedAt = createdAt.Time
		}
		if updatedAt.Valid {
			issue.UpdatedAt = updatedAt.Time
		}

		if filter != nil && !filter(&issue) {
			continue
		}

		issues = append(issues, issue)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating issues: %w", err)
	}

	return issues, nil
}

// loadDependencies loads dependencies for an issue.
func (r *DoltReader) loadDependencies(issueID string) []*model.Dependency {
	query := `SELECT depends_on_id, dependency_type FROM dependencies WHERE issue_id = ?`
	rows, err := r.db.Query(query, issueID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var deps []*model.Dependency
	for rows.Next() {
		var dep model.Dependency
		var depType string
		if err := rows.Scan(&dep.DependsOnID, &depType); err != nil {
			continue
		}
		dep.IssueID = issueID
		dep.Type = model.DependencyType(depType)
		deps = append(deps, &dep)
	}
	return deps
}

// loadComments loads comments for an issue.
func (r *DoltReader) loadComments(issueID string) []*model.Comment {
	query := `SELECT id, author, text, created_at FROM comments WHERE issue_id = ? ORDER BY created_at`
	rows, err := r.db.Query(query, issueID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var comments []*model.Comment
	for rows.Next() {
		var comment model.Comment
		var createdAt sql.NullTime
		if err := rows.Scan(&comment.ID, &comment.Author, &comment.Text, &createdAt); err != nil {
			continue
		}
		if createdAt.Valid {
			comment.CreatedAt = createdAt.Time
		}
		comment.IssueID = issueID
		comments = append(comments, &comment)
	}
	return comments
}

// GetHeadHash returns the current Dolt HEAD commit hash.
// Used by the polling watcher to detect changes.
func (r *DoltReader) GetHeadHash() (string, error) {
	var hash string
	err := r.db.QueryRow("SELECT HASHOF('HEAD')").Scan(&hash)
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD hash: %w", err)
	}
	return hash, nil
}

// GetLastModified returns the most recent update time across all issues.
func (r *DoltReader) GetLastModified() (time.Time, error) {
	var updatedAt sql.NullTime
	err := r.db.QueryRow("SELECT MAX(updated_at) FROM issues").Scan(&updatedAt)
	if err != nil {
		return time.Time{}, err
	}
	if !updatedAt.Valid {
		return time.Time{}, nil
	}
	return updatedAt.Time, nil
}

// CountIssues returns the count of non-tombstone issues.
func (r *DoltReader) CountIssues() (int, error) {
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM issues WHERE (tombstone IS NULL OR tombstone = 0)").Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}
```

Note: `parseJSONStringArray` is already in `sqlite.go` (same package), so it's reused directly.

**Step 4: Run tests**

Run: `go test ./internal/datasource/ -run TestDoltReader -v`
Expected: Tests pass (skipped if no Dolt server, ConnectFailure test runs regardless)

**Step 5: Verify build**

Run: `go build ./...`
Expected: Clean build.

**Step 6: Commit**

```bash
git add internal/datasource/dolt.go internal/datasource/dolt_test.go
git commit -m "feat(datasource): implement DoltReader via MySQL protocol

DoltReader connects to Dolt server, loads issues/deps/comments using
same SQL patterns as SQLiteReader. Includes GetHeadHash() for polling
watcher. Falls back to simple query if schema has fewer columns.

Refs: bd-7uea"
```

---

## Task 4: Wire DoltReader into `LoadFromSource`

**Files:**
- Modify: `internal/datasource/load.go`

**Step 1: Write the failing test**

Add to `internal/datasource/source_test.go`:

```go
func TestLoadFromSource_DoltType(t *testing.T) {
	// Verify that LoadFromSource dispatches to DoltReader for Dolt sources.
	// This will fail to connect (no server), but should attempt the right path.
	source := DataSource{
		Type:  SourceTypeDolt,
		Path:  "127.0.0.1:19999",
		Valid: true,
	}
	_, err := LoadFromSource(source)
	if err == nil {
		t.Fatal("Expected error connecting to non-existent Dolt server")
	}
	// Should mention "Dolt" or connection in the error
	if !strings.Contains(err.Error(), "Dolt") {
		t.Errorf("Expected error to mention Dolt, got: %v", err)
	}
}
```

Add `"strings"` to test file imports if not present.

**Step 2: Run test to verify it fails**

Run: `go test ./internal/datasource/ -run TestLoadFromSource_DoltType -v`
Expected: FAIL ("unknown source type: dolt")

**Step 3: Add the Dolt case to LoadFromSource**

In `internal/datasource/load.go`, add to the switch in `LoadFromSource`:

```go
case SourceTypeDolt:
    reader, err := NewDoltReader(source)
    if err != nil {
        return nil, fmt.Errorf("failed to open Dolt source %s: %w", source.Path, err)
    }
    defer reader.Close()
    return reader.LoadIssues()
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/datasource/ -run TestLoadFromSource_DoltType -v`
Expected: PASS (error now mentions "Dolt")

**Step 5: Run full test suite**

Run: `go test ./internal/datasource/ -v`
Expected: All tests pass.

**Step 6: Commit**

```bash
git add internal/datasource/load.go internal/datasource/source_test.go
git commit -m "feat(datasource): wire DoltReader into LoadFromSource dispatch

LoadFromSource now handles SourceTypeDolt, connecting via MySQL protocol
and loading issues through DoltReader.

Refs: bd-7uea"
```

---

## Task 5: Add Dolt Validation

**Files:**
- Modify: `internal/datasource/validate.go`
- Test: `internal/datasource/source_test.go`

**Step 1: Write the failing test**

```go
func TestValidateSource_Dolt_NoServer(t *testing.T) {
	source := DataSource{
		Type: SourceTypeDolt,
		Path: "127.0.0.1:19999",
	}
	err := ValidateSource(&source)
	if err == nil {
		t.Fatal("Expected validation to fail with no server")
	}
	if source.Valid {
		t.Fatal("Expected source.Valid to be false")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/datasource/ -run TestValidateSource_Dolt -v`
Expected: FAIL or unexpected behavior (current validate doesn't handle Dolt)

**Step 3: Add Dolt validation**

In `validate.go`, update `ValidateSourceWithOptions` to handle Dolt:

```go
case SourceTypeDolt:
    return validateDolt(source, opts)
```

Add the validation function:

```go
func validateDolt(source *DataSource, opts ValidationOptions) error {
	reader, err := NewDoltReader(*source)
	if err != nil {
		source.Valid = false
		source.ValidationError = fmt.Sprintf("cannot connect: %v", err)
		return err
	}
	defer reader.Close()

	if opts.CountIssues {
		count, err := reader.CountIssues()
		if err != nil {
			source.Valid = false
			source.ValidationError = fmt.Sprintf("cannot count issues: %v", err)
			return err
		}
		source.IssueCount = count
	}

	source.Valid = true
	return nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/datasource/ -run TestValidateSource_Dolt -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/datasource/validate.go internal/datasource/source_test.go
git commit -m "feat(datasource): add Dolt source validation

Validates Dolt source by attempting connection and optional issue count.
Sets source.Valid=false with clear error when server unreachable.

Refs: bd-7uea"
```

---

## Task 6: Add Dolt Polling Watcher

**Files:**
- Create: `internal/datasource/dolt_watcher.go`
- Create: `internal/datasource/dolt_watcher_test.go`

**Step 1: Write the failing test**

Create `internal/datasource/dolt_watcher_test.go`:

```go
package datasource

import (
	"testing"
	"time"
)

func TestDoltWatcher_NewAndStop(t *testing.T) {
	skipIfNoDolt(t)

	source := DataSource{
		Type: SourceTypeDolt,
		Path: "127.0.0.1:3306",
	}
	w, err := NewDoltWatcher(source, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("NewDoltWatcher failed: %v", err)
	}
	defer w.Stop()

	if w.Changed() == nil {
		t.Fatal("Expected non-nil Changed channel")
	}
}

func TestDoltWatcher_DetectsNoChangeOnPoll(t *testing.T) {
	skipIfNoDolt(t)

	source := DataSource{
		Type: SourceTypeDolt,
		Path: "127.0.0.1:3306",
	}
	w, err := NewDoltWatcher(source, 200*time.Millisecond)
	if err != nil {
		t.Fatalf("NewDoltWatcher failed: %v", err)
	}
	defer w.Stop()

	// Poll once to get baseline
	w.Start()

	// Wait for two poll cycles with no changes
	select {
	case <-w.Changed():
		t.Log("Got initial change notification (expected on first poll)")
	case <-time.After(1 * time.Second):
		// No change is also valid on first poll if hash was already set
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/datasource/ -run TestDoltWatcher -v`
Expected: FAIL (NewDoltWatcher undefined)

**Step 3: Write the implementation**

Create `internal/datasource/dolt_watcher.go`:

```go
package datasource

import (
	"context"
	"sync"
	"time"
)

// DoltWatcher polls a Dolt database for changes using HASHOF('HEAD').
type DoltWatcher struct {
	reader       *DoltReader
	pollInterval time.Duration
	lastHash     string
	onChange     func()

	ctx      context.Context
	cancel   context.CancelFunc
	changeCh chan struct{}
	started  bool
	mu       sync.RWMutex
}

// NewDoltWatcher creates a watcher that polls the Dolt server for changes.
func NewDoltWatcher(source DataSource, pollInterval time.Duration) (*DoltWatcher, error) {
	reader, err := NewDoltReader(source)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &DoltWatcher{
		reader:       reader,
		pollInterval: pollInterval,
		ctx:          ctx,
		cancel:       cancel,
		changeCh:     make(chan struct{}, 1),
	}, nil
}

// Start begins polling for changes.
func (w *DoltWatcher) Start() error {
	w.mu.Lock()
	if w.started {
		w.mu.Unlock()
		return nil
	}
	w.started = true
	w.mu.Unlock()

	// Get initial hash
	hash, err := w.reader.GetHeadHash()
	if err == nil {
		w.lastHash = hash
	}

	go w.poll()
	return nil
}

// Stop stops the watcher and closes the connection.
func (w *DoltWatcher) Stop() {
	w.cancel()
	w.reader.Close()
}

// Changed returns a channel that receives when the database changes.
func (w *DoltWatcher) Changed() <-chan struct{} {
	return w.changeCh
}

// SetOnChange sets a callback for when changes are detected.
func (w *DoltWatcher) SetOnChange(fn func()) {
	w.mu.Lock()
	w.onChange = fn
	w.mu.Unlock()
}

// IsPolling returns true (DoltWatcher always polls).
func (w *DoltWatcher) IsPolling() bool {
	return true
}

func (w *DoltWatcher) poll() {
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			hash, err := w.reader.GetHeadHash()
			if err != nil {
				continue // Transient error, retry next tick
			}

			if hash != w.lastHash && w.lastHash != "" {
				w.lastHash = hash
				w.notifyChange()
			} else if w.lastHash == "" {
				w.lastHash = hash
			}
		}
	}
}

func (w *DoltWatcher) notifyChange() {
	// Non-blocking send to channel
	select {
	case w.changeCh <- struct{}{}:
	default:
	}

	w.mu.RLock()
	fn := w.onChange
	w.mu.RUnlock()
	if fn != nil {
		fn()
	}
}
```

**Step 4: Run tests**

Run: `go test ./internal/datasource/ -run TestDoltWatcher -v`
Expected: PASS (or skipped if no Dolt server)

**Step 5: Verify build**

Run: `go build ./...`
Expected: Clean build.

**Step 6: Commit**

```bash
git add internal/datasource/dolt_watcher.go internal/datasource/dolt_watcher_test.go
git commit -m "feat(datasource): add DoltWatcher with HASHOF('HEAD') polling

Polls Dolt for changes by comparing HEAD commit hashes. Fires change
notification when hash changes. Configurable poll interval. Used by
TUI for live reload when source is Dolt.

Refs: bd-7uea"
```

---

## Task 7: Integrate DoltWatcher into TUI Model

**Files:**
- Modify: `pkg/ui/model.go` (watcher setup and FileChangedMsg handling)
- Modify: `cmd/b9s/main.go` (pass source type info to model)

This is the integration task that wires everything together. The key change: when the source is Dolt, create a `DoltWatcher` instead of a file `Watcher`. The `FileChangedMsg` and `WatchFileCmd` pattern stays the same.

**Step 1: Add source type to Model**

In `pkg/ui/model.go`, add to the Model struct:

```go
sourceType  datasource.SourceType // What backend we loaded from
```

Add a `WithSourceType` method:

```go
func (m Model) WithSourceType(st datasource.SourceType) Model {
	m.sourceType = st
	return m
}
```

**Step 2: Add DoltWatcher field to Model**

In `pkg/ui/model.go`, add to the Model struct:

```go
doltWatcher *datasource.DoltWatcher
```

**Step 3: Update watcher creation in NewModel/Init**

In `NewModel` or the init logic, after the existing file watcher setup, add:

```go
// If no file watcher path but we have a Dolt source, watcher creation
// is deferred until WithSourceType is called and Init runs.
```

In `Init()` (or wherever the initial watcher command is queued), add Dolt watcher support:

```go
if m.sourceType == datasource.SourceTypeDolt && m.doltWatcher == nil {
    // Detect Dolt source and create watcher
    beadsDir, _ := loader.GetBeadsDir("")
    sources, _ := datasource.DiscoverSources(datasource.DiscoveryOptions{
        BeadsDir: beadsDir,
    })
    for _, s := range sources {
        if s.Type == datasource.SourceTypeDolt {
            dw, err := datasource.NewDoltWatcher(s, 500*time.Millisecond)
            if err == nil {
                dw.Start()
                m.doltWatcher = dw
            }
            break
        }
    }
}
```

**Step 4: Create DoltWatchCmd**

Add alongside WatchFileCmd:

```go
func DoltWatchCmd(w *datasource.DoltWatcher) tea.Cmd {
	return func() tea.Msg {
		<-w.Changed()
		return FileChangedMsg{}
	}
}
```

**Step 5: Update Init to use DoltWatchCmd when appropriate**

In Init, where `WatchFileCmd` is queued:

```go
if m.doltWatcher != nil {
    cmds = append(cmds, DoltWatchCmd(m.doltWatcher))
} else if m.watcher != nil {
    cmds = append(cmds, WatchFileCmd(m.watcher))
}
```

**Step 6: Update FileChangedMsg handler to re-queue correct watcher**

In the FileChangedMsg handler, after reloading issues:

```go
if m.doltWatcher != nil {
    cmds = append(cmds, DoltWatchCmd(m.doltWatcher))
} else if m.watcher != nil {
    cmds = append(cmds, WatchFileCmd(m.watcher))
}
```

**Step 7: Update main.go to pass source type**

In `cmd/b9s/main.go`, after loading issues, detect source type:

```go
// Detect source type for watcher selection
beadsDir, _ := loader.GetBeadsDir("")
detectedSourceType := datasource.SourceTypeJSONLLocal // default
if sources, err := datasource.DiscoverSources(datasource.DiscoveryOptions{
    BeadsDir: beadsDir,
    ValidateAfterDiscovery: false,
}); err == nil {
    for _, s := range sources {
        if s.Type == datasource.SourceTypeDolt {
            detectedSourceType = datasource.SourceTypeDolt
            break
        }
    }
}

m := ui.NewModel(issues, beadsPath).
    WithSourceType(detectedSourceType).
    WithConfig(appCfg, projectName, projectPath)
```

**Step 8: Update Stop to close DoltWatcher**

In the Model's Stop/cleanup method:

```go
if m.doltWatcher != nil {
    m.doltWatcher.Stop()
}
```

**Step 9: Run full test suite**

Run: `go test ./... -v -timeout 120s`
Expected: All tests pass.

**Step 10: Commit**

```bash
git add pkg/ui/model.go cmd/b9s/main.go
git commit -m "feat(ui): integrate DoltWatcher for live reload from Dolt backend

When source is Dolt, use DoltWatcher (500ms polling via HASHOF('HEAD'))
instead of file watcher. FileChangedMsg flow unchanged. Source type
detected at startup and passed to Model.

Refs: bd-7uea"
```

---

## Task 8: Update FileChangedMsg Handler for Dolt Reload

**Files:**
- Modify: `pkg/ui/model.go` (FileChangedMsg handler to use `datasource.LoadIssues` instead of JSONL-specific loader)

Currently the FileChangedMsg handler reloads from JSONL file directly. For Dolt, it needs to go through the smart datasource path.

**Step 1: Update the reload logic**

In the FileChangedMsg handler, change from:

```go
loadedIssues, err := loader.LoadIssuesFromFileWithOptionsPooled(m.beadsPath, opts)
```

To:

```go
var loadedIssues []model.Issue
var err error
if m.sourceType == datasource.SourceTypeDolt {
    loadedIssues, err = datasource.LoadIssues("")
} else {
    loadedIssues, err = loader.LoadIssuesFromFileWithOptionsPooled(m.beadsPath, opts)
}
```

**Step 2: Run tests**

Run: `go test ./pkg/ui/ -v`
Expected: All tests pass.

**Step 3: Run full build**

Run: `go build ./...`
Expected: Clean build.

**Step 4: Commit**

```bash
git add pkg/ui/model.go
git commit -m "feat(ui): use smart datasource reload for Dolt on file change

FileChangedMsg handler now routes through datasource.LoadIssues() when
source is Dolt, instead of JSONL-specific loader.

Refs: bd-7uea"
```

---

## Summary

| Task | What | Files |
|------|------|-------|
| 1 | Add MySQL driver dependency | go.mod |
| 2 | SourceTypeDolt + discovery | source.go, source_test.go |
| 3 | DoltReader implementation | dolt.go, dolt_test.go |
| 4 | Wire into LoadFromSource | load.go, source_test.go |
| 5 | Dolt validation | validate.go, source_test.go |
| 6 | DoltWatcher (polling) | dolt_watcher.go, dolt_watcher_test.go |
| 7 | TUI integration | model.go, main.go |
| 8 | Reload path for Dolt | model.go |

Tasks 1-6 are in `internal/datasource/` (self-contained, testable without TUI). Tasks 7-8 wire into the UI layer. All tasks are independently committable.
