package datasource

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/vanderheijden86/beadwork/pkg/model"
)

// DoltReader provides read access to a Dolt database via the MySQL protocol.
type DoltReader struct {
	db   *sql.DB
	addr string
}

// NewDoltReader opens a connection to a Dolt server using the MySQL protocol.
// Connection details come from the DataSource fields:
//   - Path: host:port address (default "127.0.0.1:3306")
//   - Database: database name (default "beads")
//   - User: MySQL user (default "root")
//
// The password is read from the BEADS_DOLT_PASSWORD environment variable.
// The connection is verified with a ping before returning.
func NewDoltReader(source DataSource) (*DoltReader, error) {
	if source.Type != SourceTypeDolt {
		return nil, fmt.Errorf("source is not Dolt: %s", source.Type)
	}

	addr := source.Path
	if addr == "" {
		addr = "127.0.0.1:3306"
	}
	dbName := source.Database
	if dbName == "" {
		dbName = "beads"
	}
	user := source.User
	if user == "" {
		user = "root"
	}
	password := os.Getenv("BEADS_DOLT_PASSWORD")

	var dsn string
	if password != "" {
		dsn = fmt.Sprintf("%s:%s@tcp(%s)/%s?parseTime=true&timeout=5s&readTimeout=10s", user, password, addr, dbName)
	} else {
		dsn = fmt.Sprintf("%s@tcp(%s)/%s?parseTime=true&timeout=5s&readTimeout=10s", user, addr, dbName)
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("cannot open Dolt connection: %w", err)
	}

	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("cannot connect to Dolt at %s: %w", source.Path, err)
	}

	return &DoltReader{
		db:   db,
		addr: source.Path,
	}, nil
}

// Close closes the underlying database connection.
func (r *DoltReader) Close() error {
	if r.db != nil {
		return r.db.Close()
	}
	return nil
}

// LoadIssues reads all issues from the Dolt database.
func (r *DoltReader) LoadIssues() ([]model.Issue, error) {
	return r.LoadIssuesFiltered(nil)
}

// LoadIssuesFiltered reads issues from the Dolt database, applying the optional
// filter function to each scanned issue. Passing nil loads all issues.
// Falls back to a minimal column set if the full-schema query fails.
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
		// Fall back to minimal schema if some columns are absent.
		return r.loadIssuesSimple(filter)
	}
	defer rows.Close()

	var issues []model.Issue
	for rows.Next() {
		issue, err := scanIssue(rows)
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

// loadIssuesSimple is a fallback for Dolt databases with fewer columns.
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

		if err := rows.Scan(
			&issue.ID, &issue.Title, &description, &issue.Status, &issue.Priority, &issueType,
			&createdAt, &updatedAt,
		); err != nil {
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

// scanIssue scans a full-schema row from the issues table into a model.Issue.
// The caller is responsible for calling rows.Next() before invoking this helper.
func scanIssue(rows *sql.Rows) (model.Issue, error) {
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
		return model.Issue{}, err
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

// loadDependencies loads the dependencies for a single issue. Returns nil on
// any error; this is a best-effort helper.
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

// loadComments loads the comments for a single issue. Returns nil on any error;
// this is a best-effort helper.
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

// GetHeadHash returns the Dolt commit hash for HEAD.
func (r *DoltReader) GetHeadHash() (string, error) {
	var hash string
	if err := r.db.QueryRow("SELECT HASHOF('HEAD')").Scan(&hash); err != nil {
		return "", fmt.Errorf("failed to get HEAD hash: %w", err)
	}
	return hash, nil
}

// GetLastModified returns the most recent updated_at timestamp across all issues.
func (r *DoltReader) GetLastModified() (time.Time, error) {
	var updatedAt sql.NullTime
	if err := r.db.QueryRow("SELECT MAX(updated_at) FROM issues").Scan(&updatedAt); err != nil {
		return time.Time{}, err
	}
	if !updatedAt.Valid {
		return time.Time{}, nil
	}
	return updatedAt.Time, nil
}

// GetIssueByID retrieves a single issue by ID.
func (r *DoltReader) GetIssueByID(id string) (*model.Issue, error) {
	issues, err := r.LoadIssuesFiltered(func(issue *model.Issue) bool {
		return issue.ID == id
	})
	if err != nil {
		return nil, err
	}
	if len(issues) == 0 {
		return nil, fmt.Errorf("issue not found: %s", id)
	}
	return &issues[0], nil
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
