package datasource

import (
	"errors"

	"github.com/vanderheijden86/beadwork/pkg/model"
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
