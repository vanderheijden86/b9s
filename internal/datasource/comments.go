package datasource

import (
	"errors"
	"fmt"

	"github.com/vanderheijden86/b9s/pkg/loader"
	"github.com/vanderheijden86/b9s/pkg/model"
)

// ErrCommentsUnavailable wraps a query or scan failure while loading
// comments. It is distinct from every other load error a reader returns,
// because a comments failure alone must never look like "this issue has no
// comments": internal/attach folds comments into the attachment list, and a
// silent empty result there would make `b9s attach list` report nothing
// attached, `b9s attach detach` report nothing to detach, and a future
// `b9s attach gc` (bd-t8j5.16) delete every blob it can no longer see a
// reference to.
//
// LoadIssuesFiltered and LoadIssues return this error, wrapped, alongside a
// fully populated issue slice when every other column loaded but comments
// did not: unlike a genuine connection or schema failure, one failing
// comments query must not make OpenProject discard a live Dolt connection
// and fall back to a stale JSONL export, and must not stop the TUI from
// showing the issue list it already has. A caller that needs comments to be
// trustworthy (the CLI's attach subcommands) fails on any non-nil error, as
// they already do; a caller that tolerates a degraded comments view (the
// TUI) checks errors.Is(err, ErrCommentsUnavailable) and keeps the issues.
var ErrCommentsUnavailable = errors.New("comments unavailable")

// AllCommentsLoader is implemented by a reader that can load every comment
// in the database as one all-or-nothing operation, keyed by issue ID. It
// exists for a consumer that must see every attachment reference or none at
// all (b9s attach gc, bd-t8j5.16): unlike LoadIssuesFiltered's per-load
// batch, which degrades to an empty comments map on failure so the issue
// list keeps rendering, AllComments has no degraded mode. A partial map
// would make a live reference look garbage, and gc would delete a blob still
// in use.
type AllCommentsLoader interface {
	AllComments() (map[string][]*model.Comment, error)
}

// LoadAllComments loads every comment in source's database as one
// all-or-nothing map keyed by issue ID, mirroring LoadFromSource's dispatch
// by source type. SQLite and Dolt implement AllCommentsLoader directly; a
// JSONL source has no separate comments query to fail, so its map is built
// from the issues LoadIssuesFromFile already returns. A JSONL load error
// stops the caller exactly as a SQLite or Dolt query error would.
func LoadAllComments(source DataSource) (map[string][]*model.Comment, error) {
	switch source.Type {
	case SourceTypeSQLite:
		reader, err := NewSQLiteReader(source)
		if err != nil {
			return nil, fmt.Errorf("failed to open SQLite source %s: %w", source.Path, err)
		}
		defer reader.Close()
		return reader.AllComments()

	case SourceTypeDolt:
		reader, err := NewDoltReader(source)
		if err != nil {
			return nil, fmt.Errorf("failed to open Dolt source %s: %w", source.Path, err)
		}
		defer reader.Close()
		return reader.AllComments()

	case SourceTypeJSONLLocal, SourceTypeJSONLWorktree:
		issues, err := loader.LoadIssuesFromFile(source.Path)
		if err != nil {
			return nil, err
		}
		out := make(map[string][]*model.Comment, len(issues))
		for i := range issues {
			if len(issues[i].Comments) > 0 {
				out[issues[i].ID] = issues[i].Comments
			}
		}
		return out, nil

	default:
		return nil, fmt.Errorf("unknown source type: %s", source.Type)
	}
}
