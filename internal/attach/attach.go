// Package attach implements the core of `b9s attach` (ADR 0024): hashing and
// uploading a file to the configured blob store, then writing the Beads
// comment that references it. It has no bubbletea dependency, so both the
// CLI (cmd/b9s/attach.go) and the TUI (pkg/ui, add-from-the-TUI) can share
// it.
package attach

import (
	"context"

	"github.com/vanderheijden86/b9s/internal/attachref"
	"github.com/vanderheijden86/b9s/pkg/model"
)

// Attachment is one attachment folded from an issue's comments. It is an
// alias for attachref.Attachment, so a caller that only imports this package
// (the CLI, and later the TUI) never needs to know attachref exists.
type Attachment = attachref.Attachment

// BdRunner runs `bd <args...>` bound to one project's checkout and returns
// its combined output. internal/bdrun.Run, bound to a resolved path and
// directory, satisfies it in production. ctx bounds the run (bd-t8j5.20): a
// hung bd must not block Add or Detach, and every caller has a ctx of its
// own to pass through (attach.Add's, the CLI's per-command deadline,
// IssueWriter's per-call one).
type BdRunner interface {
	Run(ctx context.Context, args ...string) (output string, err error)
}

// RunnerFunc adapts a plain function to BdRunner.
type RunnerFunc func(ctx context.Context, args ...string) (string, error)

// Run calls f.
func (f RunnerFunc) Run(ctx context.Context, args ...string) (string, error) { return f(ctx, args...) }

// CommentLoader loads an issue's current comments, in the same order and
// from the same data source b9s reads (Dolt, SQLite or JSONL). Every
// subcommand that lists, detaches, gets or presigns an attachment folds the
// result through attachref.Collect.
type CommentLoader interface {
	Comments(issueID string) ([]*model.Comment, error)
}

// CommentLoaderFunc adapts a plain function to CommentLoader.
type CommentLoaderFunc func(issueID string) ([]*model.Comment, error)

// Comments calls f.
func (f CommentLoaderFunc) Comments(issueID string) ([]*model.Comment, error) { return f(issueID) }
