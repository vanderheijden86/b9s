package datasource

import (
	"database/sql"
	"fmt"

	"github.com/vanderheijden86/b9s/pkg/model"
)

// scanAllComments scans every row of a "SELECT id, issue_id, author, text,
// created_at FROM comments" query into a map keyed by issue ID, closing rows
// itself. DoltReader.loadAllComments and SQLiteReader.AllComments differ only
// in how the query is opened (MySQL protocol vs. a local file), not in how a
// row becomes a model.Comment, so both call this rather than each keeping
// their own copy of the scan loop. A scan or iteration failure returns a nil
// map and an error wrapping ErrCommentsUnavailable instead of whatever rows
// it already scanned, so a caller never mistakes a partial batch for a
// complete one.
func scanAllComments(rows *sql.Rows) (map[string][]*model.Comment, error) {
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
