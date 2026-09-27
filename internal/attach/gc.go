package attach

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/vanderheijden86/beadwork/internal/attachref"
	"github.com/vanderheijden86/beadwork/internal/blobstore"
	"github.com/vanderheijden86/beadwork/pkg/model"
)

// ErrEmptyReferencesRefused is returned by GC when Apply is set, the
// referenced set came back empty while blobs exist, and AllowEmptyReferences
// is not set. An empty referenced set is far more likely a wrong database or
// a comments load that silently returned nothing than a project with
// genuinely zero live attachments, and the difference is exactly the one gc
// must never guess at.
var ErrEmptyReferencesRefused = errors.New("gc: no attachment references were found; pass --allow-empty-references to delete anyway")

// AllCommentsFunc loads every comment in the project's database, keyed by
// issue ID, as one all-or-nothing operation (datasource.LoadAllComments in
// production). GC treats any error from it as fatal and deletes nothing: a
// comments load is the only source of truth for what is still referenced,
// and a partial result would make a live attachment look unreferenced.
type AllCommentsFunc func() (map[string][]*model.Comment, error)

// GCOptions configures one GC run.
type GCOptions struct {
	// Grace is the minimum age an unreferenced blob must have reached to be
	// a candidate. The caller resolves the effective grace (the --grace flag
	// or blobstore.Handle.GCGrace) before calling GC; GC applies exactly the
	// value given here.
	Grace time.Duration
	// Apply deletes candidates. Without it, GC reports what it would delete
	// and changes nothing.
	Apply bool
	// AllowEmptyReferences lets Apply proceed when the referenced set is
	// empty. See ErrEmptyReferencesRefused.
	AllowEmptyReferences bool
	// Now returns the current time; defaults to time.Now. Tests fix it so
	// age and grace-window comparisons are deterministic.
	Now func() time.Time
}

// Candidate is one blob GC considers unreferenced and old enough to delete.
type Candidate struct {
	Key          string
	SHA256       string
	Size         int64
	LastModified time.Time
	Age          time.Duration
	// Deleted reports whether --apply actually removed this blob. A
	// candidate can end this run undeleted even with --apply: the re-check
	// before Delete skips one that was touched or removed since List saw it.
	Deleted bool
}

// Report is the outcome of one GC run.
type Report struct {
	TotalBlobs      int
	ReferencedCount int
	Candidates      []Candidate
	CandidateBytes  int64
	Deleted         int
	DeletedBytes    int64
	// Unrecognised lists a key under the project's prefix that does not
	// parse back as <prefix>/<database>/sha256/<xx>/<64 hex hash> via
	// h.Key. It is reported, never deleted: a key gc cannot rebuild itself
	// might belong to something other than an attachment blob.
	Unrecognised []string
	// EmptyReferences reports that the referenced set was empty while blobs
	// existed under the prefix, worth a warning independent of Apply.
	EmptyReferences bool
}

// GC lists every blob under h's project prefix (h.ListPrefix(), never a
// path assembled by hand) and reports which are unreferenced by any
// comment's attachref.Collect result and at least opts.Grace old. With
// opts.Apply it deletes those candidates, re-checking each immediately
// before Delete so a blob re-attached after List observed it survives.
func GC(ctx context.Context, h *blobstore.Handle, loadComments AllCommentsFunc, opts GCOptions) (Report, error) {
	now := opts.Now
	if now == nil {
		now = time.Now
	}

	allComments, err := loadComments()
	if err != nil {
		return Report{}, fmt.Errorf("gc: loading comments: %w", err)
	}

	referenced := make(map[string]bool)
	for _, comments := range allComments {
		for _, a := range attachref.Collect(comments) {
			referenced[a.SHA256] = true
		}
	}

	cutoff := now().Add(-opts.Grace)
	var report Report
	listErr := h.Store.List(ctx, h.ListPrefix(), func(info blobstore.Info) error {
		report.TotalBlobs++
		hash, ok := parseBlobKey(h, info.Key)
		if !ok {
			report.Unrecognised = append(report.Unrecognised, info.Key)
			return nil
		}
		if referenced[hash] {
			report.ReferencedCount++
			return nil
		}
		if info.LastModified.After(cutoff) {
			return nil
		}
		report.Candidates = append(report.Candidates, Candidate{
			Key:          info.Key,
			SHA256:       hash,
			Size:         info.Size,
			LastModified: info.LastModified,
			Age:          now().Sub(info.LastModified),
		})
		report.CandidateBytes += info.Size
		return nil
	})
	if listErr != nil {
		return Report{}, fmt.Errorf("gc: listing blobs: %w", listErr)
	}

	report.EmptyReferences = len(referenced) == 0 && report.TotalBlobs > 0

	if !opts.Apply {
		return report, nil
	}
	if report.EmptyReferences && !opts.AllowEmptyReferences {
		return report, ErrEmptyReferencesRefused
	}

	for i := range report.Candidates {
		c := &report.Candidates[i]
		// A blob re-attached after List saw it has a fresh LastModified,
		// because Add touches a blob that already exists, so it now falls
		// inside the grace window and must survive. ErrNotFound means a
		// concurrent gc run already removed it.
		info, statErr := h.Store.Stat(ctx, c.Key)
		if statErr != nil {
			if errors.Is(statErr, blobstore.ErrNotFound) {
				continue
			}
			return report, fmt.Errorf("gc: re-checking blob %s before delete: %w", c.Key, statErr)
		}
		if info.LastModified.After(cutoff) {
			continue
		}
		if err := h.Store.Delete(ctx, c.Key); err != nil {
			return report, fmt.Errorf("gc: deleting blob %s: %w", c.Key, err)
		}
		c.Deleted = true
		report.Deleted++
		report.DeletedBytes += c.Size
	}
	return report, nil
}

// parseBlobKey reports the sha256 hash a listed key names, only when
// rebuilding the key from that hash via h.Key reproduces key exactly. This
// is stricter than pattern-matching the key's shape: it proves the key is
// one Key itself could have produced for this project, rather than merely
// looking like one.
func parseBlobKey(h *blobstore.Handle, key string) (hash string, ok bool) {
	rest, found := strings.CutPrefix(key, h.ListPrefix()+"/")
	if !found {
		return "", false
	}
	i := strings.LastIndex(rest, "/")
	if i < 0 {
		return "", false
	}
	candidate := rest[i+1:]
	if !isSHA256(candidate) {
		return "", false
	}
	rebuilt, err := h.Key(candidate)
	if err != nil || rebuilt != key {
		return "", false
	}
	return candidate, true
}
