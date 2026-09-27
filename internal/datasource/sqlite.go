package datasource

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/vanderheijden86/beadwork/pkg/debug"
	"github.com/vanderheijden86/beadwork/pkg/model"
)

// SQLiteReader provides read access to a beads SQLite database
type SQLiteReader struct {
	db   *sql.DB
	path string
}

// NewSQLiteReader opens a SQLite database for reading
func NewSQLiteReader(source DataSource) (*SQLiteReader, error) {
	if source.Type != SourceTypeSQLite {
		return nil, fmt.Errorf("source is not SQLite: %s", source.Type)
	}

	// Journal mode changes write database state and cannot be requested by a
	// read-only connection. The existing database journal mode is sufficient.
	dsn := fmt.Sprintf("file:%s?mode=ro&_busy_timeout=5000", source.Path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("cannot open database: %w", err)
	}

	// Set pragmas for read performance
	pragmas := []string{
		"PRAGMA cache_size = -64000",   // 64MB cache
		"PRAGMA mmap_size = 268435456", // 256MB mmap
		"PRAGMA temp_store = MEMORY",
	}
	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			// Non-fatal, just log
		}
	}

	return &SQLiteReader{
		db:   db,
		path: source.Path,
	}, nil
}

// Close closes the database connection
func (r *SQLiteReader) Close() error {
	if r.db != nil {
		return r.db.Close()
	}
	return nil
}

// LoadIssues reads all issues from the database
func (r *SQLiteReader) LoadIssues() ([]model.Issue, error) {
	return r.LoadIssuesFiltered(nil)
}

// LoadIssuesFiltered reads issues matching the filter function
func (r *SQLiteReader) LoadIssuesFiltered(filter func(*model.Issue) bool) ([]model.Issue, error) {
	// Query for all non-tombstone issues
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
		// Try simpler query if some columns don't exist
		return r.loadIssuesSimple(filter)
	}
	defer rows.Close()

	var issues []model.Issue
	var commentsErr error
	for rows.Next() {
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
			continue
		}

		// Map nullable fields
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

		// Parse labels JSON array
		if labelsJSON.Valid && labelsJSON.String != "" && labelsJSON.String != "null" {
			labels := parseJSONStringArray(labelsJSON.String)
			issue.Labels = labels
		}

		// Load dependencies for this issue
		issue.Dependencies = r.loadDependencies(issue.ID)

		// Load comments for this issue. A failure degrades this one issue's
		// Comments field rather than aborting the scan: commentsErr (the
		// first one seen) is still reported below, alongside the fully
		// populated issue slice, so a caller that must not mistake
		// "unknown" for "none" (internal/attach's CommentLoader) can fail
		// on it while one that tolerates a degraded view (the TUI) keeps
		// the issues.
		comments, cErr := r.loadComments(issue.ID)
		if cErr != nil {
			if commentsErr == nil {
				commentsErr = cErr
			}
			debug.Log("sqlite: comments unavailable for %s: %v", issue.ID, cErr)
		}
		issue.Comments = comments

		// Apply filter
		if filter != nil && !filter(&issue) {
			continue
		}

		issues = append(issues, issue)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating issues: %w", err)
	}

	applyCreators(r.db, issues)
	return issues, commentsErr
}

// loadIssuesSimple is a fallback for databases with fewer columns
func (r *SQLiteReader) loadIssuesSimple(filter func(*model.Issue) bool) ([]model.Issue, error) {
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

	applyCreators(r.db, issues)
	return issues, nil
}

// loadDependencies loads dependencies for an issue
func (r *SQLiteReader) loadDependencies(issueID string) []*model.Dependency {
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
	// Note: rows.Err() not checked here since loadDependencies is a
	// best-effort helper that returns nil on any error.
	return deps
}

// loadComments loads comments for an issue, reporting a query or scan
// failure wrapped in ErrCommentsUnavailable rather than returning nil: nil
// must mean "no comments," never "couldn't ask" (see ErrCommentsUnavailable).
func (r *SQLiteReader) loadComments(issueID string) ([]*model.Comment, error) {
	query := `SELECT id, author, text, created_at FROM comments WHERE issue_id = ? ORDER BY created_at`
	rows, err := r.db.Query(query, issueID)
	if err != nil {
		return nil, fmt.Errorf("%w: query comments for %s: %w", ErrCommentsUnavailable, issueID, err)
	}
	defer rows.Close()

	var comments []*model.Comment
	for rows.Next() {
		var comment model.Comment
		var createdAt sql.NullTime
		if err := rows.Scan(&comment.ID, &comment.Author, &comment.Text, &createdAt); err != nil {
			return nil, fmt.Errorf("%w: scan comment for %s: %w", ErrCommentsUnavailable, issueID, err)
		}
		if createdAt.Valid {
			comment.CreatedAt = createdAt.Time
		}
		comment.IssueID = issueID
		comments = append(comments, &comment)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%w: iterate comments for %s: %w", ErrCommentsUnavailable, issueID, err)
	}
	return comments, nil
}

// AllComments loads every comment in the database in one all-or-nothing
// query, keyed by issue ID, for a consumer that must see every attachment
// reference or none at all (b9s attach gc, bd-t8j5.16). Unlike loadComments
// as used from LoadIssuesFiltered, which degrades one issue's Comments field
// on failure so the issue list keeps rendering, a gc walking this map for
// live references must never see a partial one - that would make a real
// reference look garbage and delete a blob still in use.
func (r *SQLiteReader) AllComments() (map[string][]*model.Comment, error) {
	rows, err := r.db.Query(`SELECT id, issue_id, author, text, created_at FROM comments ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("%w: query comments: %w", ErrCommentsUnavailable, err)
	}
	defer rows.Close()

	result := make(map[string][]*model.Comment)
	for rows.Next() {
		var comment model.Comment
		var createdAt sql.NullTime
		if err := rows.Scan(&comment.ID, &comment.IssueID, &comment.Author, &comment.Text, &createdAt); err != nil {
			return nil, fmt.Errorf("%w: scan comment: %w", ErrCommentsUnavailable, err)
		}
		if createdAt.Valid {
			comment.CreatedAt = createdAt.Time
		}
		result[comment.IssueID] = append(result[comment.IssueID], &comment)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%w: iterate comments: %w", ErrCommentsUnavailable, err)
	}
	return result, nil
}

// CountIssues returns the count of non-tombstone issues
func (r *SQLiteReader) CountIssues() (int, error) {
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM issues WHERE (tombstone IS NULL OR tombstone = 0)").Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// GetIssueByID retrieves a single issue by ID
func (r *SQLiteReader) GetIssueByID(id string) (*model.Issue, error) {
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

// GetLastModified returns the most recent update time
func (r *SQLiteReader) GetLastModified() (time.Time, error) {
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

// parseJSONStringArray parses a JSON array of strings
func parseJSONStringArray(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" || s == "null" || s == "[]" {
		return nil
	}

	// Use proper JSON unmarshaling to handle edge cases like commas in labels
	var result []string
	if err := json.Unmarshal([]byte(s), &result); err != nil {
		// Fallback to simple parser for malformed JSON
		s = strings.TrimPrefix(s, "[")
		s = strings.TrimSuffix(s, "]")
		if s == "" {
			return nil
		}
		for _, item := range strings.Split(s, ",") {
			item = strings.TrimSpace(item)
			item = strings.Trim(item, `"`)
			if item != "" {
				result = append(result, item)
			}
		}
	}
	return result
}
