package datasource

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/vanderheijden86/b9s/pkg/debug"
	"github.com/vanderheijden86/b9s/pkg/model"
)

// leanIssueQuery reads every column the tree, board, filters and sorting need
// and none of the long text columns. On a real project the text of closed
// issues is most of a snapshot's bytes, and nobody reads it until an issue is
// opened (docs/adr/0060-load-a-lean-snapshot-and-text-on-demand.md).
const leanIssueQuery = `
	SELECT
		id, title, status, priority, issue_type,
		assignee, estimated_minutes, created_at, updated_at,
		due_at, closed_at, external_ref, compaction_level,
		compacted_at, compacted_at_commit, original_size,
		source_repo, defer_until
	FROM issues
	WHERE status != 'tombstone'`

const leanIssueOrder = `
	ORDER BY updated_at DESC`

// wholeIssueCondition selects the issues whose text and comments arrive with
// the snapshot: the ones a user is likely to open, and epics, whose
// description the board's lane rail shows.
const wholeIssueCondition = `(i.status NOT IN ('closed', 'tombstone') OR i.issue_type = 'epic')`

const wholeIssueTextQuery = `
	SELECT i.id, i.description, i.design, i.acceptance_criteria, i.notes
	FROM issues i
	WHERE ` + wholeIssueCondition

const wholeIssueCommentsQuery = `
	SELECT c.id, c.issue_id, c.author, c.text, c.created_at
	FROM comments c JOIN issues i ON i.id = c.issue_id
	WHERE ` + wholeIssueCondition

const wholeIssueCommentsOrder = `
	ORDER BY c.created_at`

// IssueText is the part of an issue a lean snapshot leaves out.
type IssueText struct {
	Description        string
	Design             string
	AcceptanceCriteria string
	Notes              string
	CloseReason        string
	Metadata           json.RawMessage
	Comments           []*model.Comment
}

// ApplyTo fills issue with t and marks it whole.
func (t IssueText) ApplyTo(issue *model.Issue) {
	issue.Description = t.Description
	issue.Design = t.Design
	issue.AcceptanceCriteria = t.AcceptanceCriteria
	issue.Notes = t.Notes
	issue.CloseReason = t.CloseReason
	issue.Metadata = append(json.RawMessage(nil), t.Metadata...)
	issue.Comments = t.Comments
	issue.Abridged = false
}

// LoadLeanIssues reads every issue's lean columns, labels and dependencies,
// and the text and comments of issues that are not closed and of epics. The
// other issues come back Abridged; LoadIssueText supplies their text. A schema
// without the lean columns falls back to the full read.
func (r *DoltReader) LoadLeanIssues() ([]model.Issue, error) {
	return r.readLeanIssues(nil)
}

// LoadFirstPage reads what the tree shows before any closed issue is needed:
// every issue that is not closed, and every epic, each exactly as
// LoadLeanIssues returns it. Closed issues other than epics are left out, so
// a caller that shows this must follow it with a full read.
func (r *DoltReader) LoadFirstPage() ([]model.Issue, error) {
	return r.readLean(leanScope{firstPage: true})
}

// readLeanIssues reads the lean snapshot of the issues in ids, or of every
// issue when ids is nil. A restricted read never falls back to the full read:
// it runs only after a lean read succeeded against the same schema.
func (r *DoltReader) readLeanIssues(ids []string) ([]model.Issue, error) {
	return r.readLean(leanScope{ids: ids})
}

// leanScope names the issues one lean read covers: those in ids, the first
// page, or every issue when neither is set.
type leanScope struct {
	ids       []string
	firstPage bool
}

// firstPageCondition matches the issues whose text a lean read carries
// (wholeIssueCondition), on the issues table without an alias.
const firstPageCondition = `(status NOT IN ('closed', 'tombstone') OR issue_type = 'epic')`

// issueFilter restricts a query on the issues table.
func (s leanScope) issueFilter(column string) (string, []any) {
	if s.firstPage {
		return " AND " + firstPageCondition, nil
	}
	return idFilter(column, s.ids)
}

// relationFilter restricts a query on a table keyed by issue id.
func (s leanScope) relationFilter(column string) (string, []any) {
	if s.firstPage {
		return " AND " + column + " IN (SELECT id FROM issues WHERE " + firstPageCondition + ")", nil
	}
	return idFilter(column, s.ids)
}

// textIDs restricts the text and comment queries, which already cover only
// the first page's issues.
func (s leanScope) textIDs() []string {
	if s.firstPage {
		return nil
	}
	return s.ids
}

// leanReadQueries is how many queries one lean read runs at once. Each costs
// a round trip through the tunnel, so they run in parallel rather than in
// turn, and the reader's pool must hold a connection for each.
const leanReadQueries = 6

func (r *DoltReader) readLean(scope leanScope) ([]model.Issue, error) {
	var (
		wg          sync.WaitGroup
		issues      []model.Issue
		issuesErr   error
		texts       map[string]IssueText
		textErr     error
		allLabels   map[string][]string
		allDeps     map[string][]*model.Dependency
		depsErr     error
		comments    map[string][]*model.Comment
		commentsErr error
		creators    map[string]issueCreator
	)
	run := func(f func()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			f()
		}()
	}
	inIssues, args := scope.issueFilter("id")
	run(func() { issues, issuesErr = r.queryLeanIssues(inIssues, args) })
	run(func() { texts, textErr = r.loadWholeIssueText(scope.textIDs()) })
	inRelation, relationArgs := scope.relationFilter("issue_id")
	run(func() { allLabels = r.labelsWhere(inRelation, relationArgs...) })
	run(func() { allDeps, depsErr = r.dependenciesWhere(inRelation, relationArgs...) })
	run(func() {
		inComments, commentArgs := idFilter("c.issue_id", scope.textIDs())
		commentRows, err := r.db.Query(wholeIssueCommentsQuery+inComments+wholeIssueCommentsOrder, commentArgs...)
		if err != nil {
			commentsErr = fmt.Errorf("%w: query comments: %w", ErrCommentsUnavailable, err)
			return
		}
		comments, commentsErr = scanAllComments(commentRows)
	})
	run(func() { creators = loadCreatorsWhere(r.db, inIssues, args...) })
	wg.Wait()

	if issuesErr != nil {
		if scope.ids == nil && doltMissingColumnError(issuesErr) {
			debug.Log("dolt: lean query FAILED (%v), falling back to the full read", issuesErr)
			return r.LoadIssuesFiltered(nil)
		}
		return nil, fmt.Errorf("query issues: %w", issuesErr)
	}
	if textErr != nil {
		return nil, textErr
	}
	if depsErr != nil {
		return nil, depsErr
	}
	if commentsErr != nil {
		debug.Log("dolt: comments unavailable, issues will report no comments: %v", commentsErr)
	}
	for i := range issues {
		issues[i].Labels = allLabels[issues[i].ID]
		issues[i].Dependencies = allDeps[issues[i].ID]
		if c, ok := creators[issues[i].ID]; ok {
			issues[i].CreatedBy, issues[i].Owner = c.createdBy, c.owner
		}
		text, whole := texts[issues[i].ID]
		if !whole {
			issues[i].Abridged = true
			continue
		}
		text.Comments = comments[issues[i].ID]
		text.ApplyTo(&issues[i])
	}
	debug.Log("dolt: lean RESULT: %d issues, %d whole, firstPage=%t", len(issues), len(texts), scope.firstPage)
	return issues, commentsErr
}

func (r *DoltReader) queryLeanIssues(and string, args []any) ([]model.Issue, error) {
	rows, err := r.db.Query(leanIssueQuery+and+leanIssueOrder, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var issues []model.Issue
	for rows.Next() {
		issue, err := scanLeanIssue(rows)
		if err != nil {
			debug.Log("dolt: lean scan error on row: %v", err)
			continue
		}
		issues = append(issues, issue)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating issues: %w", err)
	}
	return issues, nil
}

// idFilter returns an AND clause that restricts column to ids, with its
// arguments, or nothing when ids is nil.
func idFilter(column string, ids []string) (string, []any) {
	if ids == nil {
		return "", nil
	}
	if len(ids) == 0 {
		return " AND 1 = 0", nil
	}
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	return " AND " + column + " IN (?" + strings.Repeat(", ?", len(ids)-1) + ")", args
}

func (r *DoltReader) loadWholeIssueText(ids []string) (map[string]IssueText, error) {
	in, args := idFilter("i.id", ids)
	rows, err := r.db.Query(wholeIssueTextQuery+in, args...)
	if err != nil {
		return nil, fmt.Errorf("query issue text: %w", err)
	}
	defer rows.Close()
	texts := map[string]IssueText{}
	for rows.Next() {
		var id string
		var text IssueText
		var description, design, acceptance, notes sql.NullString
		if err := rows.Scan(&id, &description, &design, &acceptance, &notes); err != nil {
			return nil, fmt.Errorf("scan issue text: %w", err)
		}
		text.Description, text.Design = description.String, design.String
		text.AcceptanceCriteria, text.Notes = acceptance.String, notes.String
		texts[id] = text
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	in, args = idFilter("id", ids)
	details, err := loadIssueDetailsWhere(r.db, " AND "+firstPageCondition+in, args...)
	if err != nil {
		return nil, err
	}
	for id, text := range texts {
		text.CloseReason, text.Metadata = details[id].CloseReason, details[id].Metadata
		texts[id] = text
	}
	return texts, nil
}

// LoadIssueText reads the text columns and comments of one issue.
func (r *DoltReader) LoadIssueText(id string) (IssueText, error) {
	var text IssueText
	var description, design, acceptance, notes sql.NullString
	err := r.db.QueryRow(
		`SELECT description, design, acceptance_criteria, notes FROM issues WHERE id = ?`, id,
	).Scan(&description, &design, &acceptance, &notes)
	if err != nil {
		return IssueText{}, fmt.Errorf("read text of %s: %w", id, err)
	}
	text.Description, text.Design = description.String, design.String
	text.AcceptanceCriteria, text.Notes = acceptance.String, notes.String
	details, err := loadIssueDetailsWhere(r.db, " AND id = ?", id)
	if err != nil {
		return IssueText{}, err
	}
	text.CloseReason, text.Metadata = details[id].CloseReason, details[id].Metadata

	rows, err := r.db.Query(
		`SELECT id, issue_id, author, text, created_at FROM comments WHERE issue_id = ? ORDER BY created_at`, id)
	if err != nil {
		return IssueText{}, fmt.Errorf("%w: read comments of %s: %w", ErrCommentsUnavailable, id, err)
	}
	comments, err := scanAllComments(rows)
	if err != nil {
		return IssueText{}, err
	}
	text.Comments = comments[id]
	return text, nil
}

func scanLeanIssue(rows *sql.Rows) (model.Issue, error) {
	var issue model.Issue
	var estimatedMinutes, compactionLevel, originalSize sql.NullInt64
	var createdAt, updatedAt, dueAt, closedAt, compactedAt, deferUntil sql.NullTime
	var assignee, externalRef, sourceRepo, compactedAtCommit sql.NullString
	var issueType string
	err := rows.Scan(
		&issue.ID, &issue.Title, &issue.Status, &issue.Priority, &issueType,
		&assignee, &estimatedMinutes, &createdAt, &updatedAt,
		&dueAt, &closedAt, &externalRef, &compactionLevel,
		&compactedAt, &compactedAtCommit, &originalSize,
		&sourceRepo, &deferUntil,
	)
	if err != nil {
		return model.Issue{}, err
	}
	issue.IssueType = model.IssueType(issueType)
	issue.Assignee = assignee.String
	issue.SourceRepo = sourceRepo.String
	if estimatedMinutes.Valid {
		v := int(estimatedMinutes.Int64)
		issue.EstimatedMinutes = &v
	}
	issue.CreatedAt = createdAt.Time
	issue.UpdatedAt = updatedAt.Time
	issue.DueDate = nullTimePtr(dueAt)
	issue.ClosedAt = nullTimePtr(closedAt)
	issue.DeferUntil = nullTimePtr(deferUntil)
	issue.CompactedAt = nullTimePtr(compactedAt)
	if externalRef.Valid {
		s := externalRef.String
		issue.ExternalRef = &s
	}
	if compactedAtCommit.Valid {
		s := compactedAtCommit.String
		issue.CompactedAtCommit = &s
	}
	issue.CompactionLevel = int(compactionLevel.Int64)
	issue.OriginalSize = int(originalSize.Int64)
	return issue, nil
}

func nullTimePtr(t sql.NullTime) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}
