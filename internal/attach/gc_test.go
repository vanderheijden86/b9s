package attach

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/vanderheijden86/beadwork/internal/attachref"
	"github.com/vanderheijden86/beadwork/internal/blobstore"
	"github.com/vanderheijden86/beadwork/pkg/config"
	"github.com/vanderheijden86/beadwork/pkg/model"
)

// gcHandle mirrors localHandleWithDir but lets a test pick the project name,
// so a sibling-database test can build two handles that share one
// local_dir.
func gcHandle(t *testing.T, dir, projectName string) *blobstore.Handle {
	t.Helper()
	cfg := &config.AttachmentsConfig{Backend: "local", LocalDir: dir, MaxBytes: 1 << 20}
	h, err := blobstore.Open(context.Background(), cfg, blobstore.SourceInfo{
		Kind:        blobstore.SourceJSONL,
		ProjectName: projectName,
	})
	if err != nil {
		t.Fatalf("blobstore.Open: %v", err)
	}
	return h
}

// refFor builds the Ref a blob of content would hash to, named name.
func refFor(content []byte, name string) attachref.Ref {
	return attachref.Ref{SHA256: sha256Hex(content), Size: int64(len(content)), Type: "text/plain", Name: name}
}

// putBlob stores content under h's key for ref, bypassing Add, and backdates
// the file's mtime by age so a test can place a blob at a known age without
// waiting for real time to pass.
func putBlob(t *testing.T, h *blobstore.Handle, dir string, ref attachref.Ref, content []byte, age time.Duration) {
	t.Helper()
	key := mustKey(t, h, ref)
	if err := h.Store.Put(context.Background(), key, bytes.NewReader(content), ref.Size, ref.Type); err != nil {
		t.Fatal(err)
	}
	backdateBlob(t, dir, key, age)
}

// backdateBlob sets the file backing key age in the past. Local stores one
// file per key with slashes as path separators, the same layout Local.Put
// itself writes.
func backdateBlob(t *testing.T, dir, key string, age time.Duration) {
	t.Helper()
	when := time.Now().Add(-age)
	path := filepath.Join(dir, filepath.FromSlash(key))
	if err := os.Chtimes(path, when, when); err != nil {
		t.Fatal(err)
	}
}

func mustKey(t *testing.T, h *blobstore.Handle, ref attachref.Ref) string {
	t.Helper()
	key, err := h.Key(ref.SHA256)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func detachComment(t *testing.T, id string, at time.Time, ref attachref.Ref) *model.Comment {
	t.Helper()
	text, err := attachref.FormatDetach(ref)
	if err != nil {
		t.Fatal(err)
	}
	return &model.Comment{ID: id, Author: "tester", Text: text, CreatedAt: at}
}

func noComments() (map[string][]*model.Comment, error) { return nil, nil }

func TestGC_KeepsAReferencedBlob(t *testing.T) {
	dir := t.TempDir()
	h := gcHandle(t, dir, "b9s")
	content := []byte("keep me")
	ref := refFor(content, "keep.txt")
	putBlob(t, h, dir, ref, content, 48*time.Hour)
	key := mustKey(t, h, ref)

	comments := map[string][]*model.Comment{
		"bd-1": {attachComment(t, "cmt-1", "bd-1", time.Now().Add(-48*time.Hour), ref)},
	}
	loadComments := func() (map[string][]*model.Comment, error) { return comments, nil }

	report, err := GC(context.Background(), h, loadComments, GCOptions{Grace: time.Hour, Apply: true})
	if err != nil {
		t.Fatalf("GC err = %v", err)
	}
	if report.TotalBlobs != 1 || report.ReferencedCount != 1 || len(report.Candidates) != 0 {
		t.Fatalf("report = %+v, want the blob referenced and no candidates", report)
	}
	if _, statErr := h.Store.Stat(context.Background(), key); statErr != nil {
		t.Errorf("referenced blob was removed: %v", statErr)
	}
}

func TestGC_DetachedBlobOlderThanGrace_DeletedWithApplyKeptInDryRun(t *testing.T) {
	dir := t.TempDir()
	h := gcHandle(t, dir, "b9s")
	content := []byte("detached old")
	ref := refFor(content, "old.txt")
	putBlob(t, h, dir, ref, content, 48*time.Hour)
	key := mustKey(t, h, ref)

	// keepRef stays referenced throughout, so the referenced set is never
	// empty and this test exercises only the grace mechanic, not the
	// --allow-empty-references guard (covered separately).
	keepContent := []byte("kept elsewhere")
	keepRef := refFor(keepContent, "keep.txt")
	putBlob(t, h, dir, keepRef, keepContent, 48*time.Hour)

	comments := map[string][]*model.Comment{
		"bd-1": {
			attachComment(t, "cmt-1", "bd-1", time.Now().Add(-48*time.Hour), ref),
			detachComment(t, "cmt-2", time.Now().Add(-47*time.Hour), ref),
		},
		"bd-2": {
			attachComment(t, "cmt-3", "bd-2", time.Now().Add(-48*time.Hour), keepRef),
		},
	}
	loadComments := func() (map[string][]*model.Comment, error) { return comments, nil }

	dryReport, err := GC(context.Background(), h, loadComments, GCOptions{Grace: time.Hour})
	if err != nil {
		t.Fatalf("dry run GC err = %v", err)
	}
	if len(dryReport.Candidates) != 1 || dryReport.Candidates[0].Deleted {
		t.Fatalf("dry run report = %+v, want one undeleted candidate", dryReport)
	}
	if _, statErr := h.Store.Stat(context.Background(), key); statErr != nil {
		t.Fatalf("dry run deleted the blob: %v", statErr)
	}

	applyReport, err := GC(context.Background(), h, loadComments, GCOptions{Grace: time.Hour, Apply: true})
	if err != nil {
		t.Fatalf("apply GC err = %v", err)
	}
	if applyReport.Deleted != 1 || len(applyReport.Candidates) != 1 || !applyReport.Candidates[0].Deleted {
		t.Fatalf("apply report = %+v, want the candidate deleted", applyReport)
	}
	if _, statErr := h.Store.Stat(context.Background(), key); !errors.Is(statErr, blobstore.ErrNotFound) {
		t.Fatalf("blob still present after apply: err = %v", statErr)
	}
}

func TestGC_KeepsAYoungUnreferencedBlob(t *testing.T) {
	dir := t.TempDir()
	h := gcHandle(t, dir, "b9s")
	content := []byte("young orphan")
	ref := refFor(content, "young.txt")
	putBlob(t, h, dir, ref, content, time.Minute)
	key := mustKey(t, h, ref)

	report, err := GC(context.Background(), h, noComments, GCOptions{Grace: 24 * time.Hour, Apply: true, AllowEmptyReferences: true})
	if err != nil {
		t.Fatalf("GC err = %v", err)
	}
	if report.TotalBlobs != 1 || len(report.Candidates) != 0 {
		t.Fatalf("report = %+v, want the young blob kept, not a candidate", report)
	}
	if _, statErr := h.Store.Stat(context.Background(), key); statErr != nil {
		t.Errorf("young blob was removed: %v", statErr)
	}
}

func TestGC_KeepsABlobDetachedOnOneIssueButAttachedOnAnother(t *testing.T) {
	dir := t.TempDir()
	h := gcHandle(t, dir, "b9s")
	content := []byte("shared")
	ref := refFor(content, "shared.txt")
	putBlob(t, h, dir, ref, content, 48*time.Hour)
	key := mustKey(t, h, ref)

	comments := map[string][]*model.Comment{
		"bd-1": {
			attachComment(t, "cmt-1", "bd-1", time.Now().Add(-48*time.Hour), ref),
			detachComment(t, "cmt-2", time.Now().Add(-47*time.Hour), ref),
		},
		"bd-2": {
			attachComment(t, "cmt-3", "bd-2", time.Now().Add(-46*time.Hour), ref),
		},
	}
	loadComments := func() (map[string][]*model.Comment, error) { return comments, nil }

	report, err := GC(context.Background(), h, loadComments, GCOptions{Grace: time.Hour, Apply: true})
	if err != nil {
		t.Fatalf("GC err = %v", err)
	}
	if report.ReferencedCount != 1 || len(report.Candidates) != 0 {
		t.Fatalf("report = %+v, want the blob referenced via bd-2 despite being detached on bd-1", report)
	}
	if _, statErr := h.Store.Stat(context.Background(), key); statErr != nil {
		t.Errorf("blob still referenced elsewhere was removed: %v", statErr)
	}
}

func TestGC_CommentLoadErrorDeletesNothing(t *testing.T) {
	dir := t.TempDir()
	h := gcHandle(t, dir, "b9s")
	content := []byte("orphan")
	ref := refFor(content, "orphan.txt")
	putBlob(t, h, dir, ref, content, 48*time.Hour)
	key := mustKey(t, h, ref)

	loadErr := errors.New("comments unavailable")
	loadComments := func() (map[string][]*model.Comment, error) { return nil, loadErr }

	_, err := GC(context.Background(), h, loadComments, GCOptions{Grace: time.Hour, Apply: true, AllowEmptyReferences: true})
	if err == nil {
		t.Fatal("GC err = nil, want the comments load failure reported")
	}
	if !errors.Is(err, loadErr) {
		t.Errorf("GC err = %v, want it to wrap %v", err, loadErr)
	}
	if _, statErr := h.Store.Stat(context.Background(), key); statErr != nil {
		t.Errorf("blob was removed despite a comments load failure: %v", statErr)
	}
}

func TestGC_NeverListsASiblingDatabasesBlobs(t *testing.T) {
	dir := t.TempDir()
	h := gcHandle(t, dir, "b9s")
	sibling := gcHandle(t, dir, "other-project")

	content := []byte("sibling blob")
	ref := refFor(content, "sibling.txt")
	putBlob(t, sibling, dir, ref, content, 48*time.Hour)
	siblingKey := mustKey(t, sibling, ref)

	report, err := GC(context.Background(), h, noComments, GCOptions{Grace: time.Hour, Apply: true, AllowEmptyReferences: true})
	if err != nil {
		t.Fatalf("GC err = %v", err)
	}
	if report.TotalBlobs != 0 || len(report.Candidates) != 0 {
		t.Fatalf("report = %+v, want the sibling database's blob never seen", report)
	}
	if _, statErr := sibling.Store.Stat(context.Background(), siblingKey); statErr != nil {
		t.Errorf("sibling blob was removed: %v", statErr)
	}
}

func TestGC_KeepsAnUnrecognisedKey(t *testing.T) {
	dir := t.TempDir()
	h := gcHandle(t, dir, "b9s")
	garbageKey := h.ListPrefix() + "/not-a-valid-blob-key"
	content := []byte("garbage")
	if err := h.Store.Put(context.Background(), garbageKey, bytes.NewReader(content), int64(len(content)), "text/plain"); err != nil {
		t.Fatal(err)
	}
	backdateBlob(t, dir, garbageKey, 48*time.Hour)

	report, err := GC(context.Background(), h, noComments, GCOptions{Grace: time.Hour, Apply: true, AllowEmptyReferences: true})
	if err != nil {
		t.Fatalf("GC err = %v", err)
	}
	if len(report.Unrecognised) != 1 || report.Unrecognised[0] != garbageKey {
		t.Fatalf("report.Unrecognised = %v, want [%q]", report.Unrecognised, garbageKey)
	}
	if len(report.Candidates) != 0 {
		t.Fatalf("candidates = %+v, want none: an unrecognised key must never become a candidate", report.Candidates)
	}
	if _, statErr := h.Store.Stat(context.Background(), garbageKey); statErr != nil {
		t.Errorf("unrecognised key was deleted: %v", statErr)
	}
}

func TestGC_AllowEmptyReferencesEnforced(t *testing.T) {
	dir := t.TempDir()
	h := gcHandle(t, dir, "b9s")
	content := []byte("no references at all")
	ref := refFor(content, "orphan.txt")
	putBlob(t, h, dir, ref, content, 48*time.Hour)
	key := mustKey(t, h, ref)

	dryReport, err := GC(context.Background(), h, noComments, GCOptions{Grace: time.Hour})
	if err != nil {
		t.Fatalf("dry run GC err = %v, want nil: only --apply requires the guard", err)
	}
	if !dryReport.EmptyReferences || len(dryReport.Candidates) != 1 {
		t.Fatalf("dry run report = %+v, want EmptyReferences true and one candidate reported", dryReport)
	}

	refused, err := GC(context.Background(), h, noComments, GCOptions{Grace: time.Hour, Apply: true})
	if !errors.Is(err, ErrEmptyReferencesRefused) {
		t.Fatalf("apply GC err = %v, want ErrEmptyReferencesRefused", err)
	}
	if !refused.EmptyReferences {
		t.Errorf("report.EmptyReferences = false, want true")
	}
	if _, statErr := h.Store.Stat(context.Background(), key); statErr != nil {
		t.Errorf("blob was removed despite the refusal: %v", statErr)
	}

	allowed, err := GC(context.Background(), h, noComments, GCOptions{Grace: time.Hour, Apply: true, AllowEmptyReferences: true})
	if err != nil {
		t.Fatalf("apply GC err = %v, want nil with AllowEmptyReferences", err)
	}
	if allowed.Deleted != 1 {
		t.Fatalf("report.Deleted = %d, want 1", allowed.Deleted)
	}
	if _, statErr := h.Store.Stat(context.Background(), key); !errors.Is(statErr, blobstore.ErrNotFound) {
		t.Errorf("blob still present after AllowEmptyReferences apply: err = %v", statErr)
	}
}

// raceStore wraps a Store to simulate the exact interleaving race 6b guards
// against: between gc's List call and its re-check immediately before
// Delete, something else (a concurrent attach.Add) re-references the blob
// and calls Touch. The first Stat call for key fires that simulated Touch
// before returning, standing in for a race that would otherwise need two
// real goroutines and a synchronization point neither gc nor Add exposes.
type raceStore struct {
	blobstore.Store
	key   string
	fired bool
}

func (r *raceStore) Stat(ctx context.Context, key string) (blobstore.Info, error) {
	if key == r.key && !r.fired {
		r.fired = true
		if err := r.Store.Touch(ctx, key); err != nil {
			return blobstore.Info{}, err
		}
	}
	return r.Store.Stat(ctx, key)
}

func TestGC_Race6b_SkipsABlobReattachedBetweenListAndDelete(t *testing.T) {
	dir := t.TempDir()
	h := gcHandle(t, dir, "b9s")
	content := []byte("raced")
	ref := refFor(content, "raced.txt")
	putBlob(t, h, dir, ref, content, 48*time.Hour)
	key := mustKey(t, h, ref)

	raced := *h
	raced.Store = &raceStore{Store: h.Store, key: key}

	report, err := GC(context.Background(), &raced, noComments, GCOptions{Grace: time.Hour, Apply: true, AllowEmptyReferences: true})
	if err != nil {
		t.Fatalf("GC err = %v", err)
	}
	if len(report.Candidates) != 1 || report.Candidates[0].Deleted {
		t.Fatalf("report = %+v, want the candidate listed but not deleted (re-attached mid-run)", report)
	}
	if report.Deleted != 0 {
		t.Errorf("report.Deleted = %d, want 0", report.Deleted)
	}
	if _, statErr := h.Store.Stat(context.Background(), key); statErr != nil {
		t.Errorf("blob was removed despite being re-attached mid-run: %v", statErr)
	}
}
