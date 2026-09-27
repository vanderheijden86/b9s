package attach

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/vanderheijden86/beadwork/internal/attachref"
	"github.com/vanderheijden86/beadwork/internal/bdrun"
	"github.com/vanderheijden86/beadwork/internal/blobstore"
	"github.com/vanderheijden86/beadwork/pkg/config"
)

// errCommandFailed stands in for whatever error a real bd invocation
// returns on a non-zero exit; only its presence matters to these tests.
var errCommandFailed = errors.New("bd exited 1")

// fakeBd records every argv it was called with, and fails with err when set.
// output stands in for what a real bd invocation prints (its combined
// stdout/stderr), which a caller's error must surface rather than discard.
type fakeBd struct {
	calls  [][]string
	err    error
	output string
}

func (f *fakeBd) Run(_ context.Context, args ...string) (string, error) {
	f.calls = append(f.calls, append([]string(nil), args...))
	return f.output, f.err
}

// sha256Hex returns content's sha256 as lowercase hex, the same form
// attachref requires in a machine line.
func sha256Hex(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func localHandle(t *testing.T, maxBytes int64) *blobstore.Handle {
	t.Helper()
	cfg := &config.AttachmentsConfig{Backend: "local", LocalDir: t.TempDir(), MaxBytes: maxBytes}
	h, err := blobstore.Open(context.Background(), cfg, blobstore.SourceInfo{
		Kind:        blobstore.SourceJSONL,
		ProjectName: "b9s",
	})
	if err != nil {
		t.Fatalf("blobstore.Open: %v", err)
	}
	return h
}

// localHandleWithDir mirrors localHandle but also returns the local
// backend's root directory, so a test can reach into the store's files
// directly (backdating a blob's mtime, for example) without Local exposing
// that path itself.
func localHandleWithDir(t *testing.T, maxBytes int64) (*blobstore.Handle, string) {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.AttachmentsConfig{Backend: "local", LocalDir: dir, MaxBytes: maxBytes}
	h, err := blobstore.Open(context.Background(), cfg, blobstore.SourceInfo{
		Kind:        blobstore.SourceJSONL,
		ProjectName: "b9s",
	})
	if err != nil {
		t.Fatalf("blobstore.Open: %v", err)
	}
	return h, dir
}

func writeTempFile(t *testing.T, dir, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAdd_UploadsAndWritesReferenceComment(t *testing.T) {
	h := localHandle(t, 1<<20)
	srcDir := t.TempDir()
	path := writeTempFile(t, srcDir, "notes.txt", []byte("hello world"))
	bd := &fakeBd{}

	results := Add(context.Background(), h, bd, "bd-1", []string{path})

	if len(results) != 1 || results[0].Err != nil {
		t.Fatalf("Add results = %+v", results)
	}
	r := results[0]
	if r.Ref.Name != "notes.txt" || r.Ref.Type != "text/plain" || r.Ref.Size != int64(len("hello world")) {
		t.Errorf("ref = %+v", r.Ref)
	}
	if r.Skipped {
		t.Errorf("Skipped = true on first upload, want false")
	}

	key, err := h.Key(r.Ref.SHA256)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.Store.Stat(context.Background(), key); err != nil {
		t.Errorf("blob missing from store after Add: %v", err)
	}

	if len(bd.calls) != 1 {
		t.Fatalf("bd calls = %d, want 1", len(bd.calls))
	}
	call := bd.calls[0]
	if len(call) != 4 || call[0] != "comments" || call[1] != "add" || call[2] != "bd-1" {
		t.Fatalf("bd call = %v", call)
	}
	// The comment text must arrive as a single argv element: Parse must
	// read back exactly the ref that was uploaded.
	refs, _ := attachref.Parse(call[3])
	if len(refs) != 1 || refs[0].SHA256 != r.Ref.SHA256 || refs[0].Name != "notes.txt" {
		t.Errorf("Parse(%q) refs = %+v", call[3], refs)
	}
}

func TestAdd_MimeSniff_TxtAndMd(t *testing.T) {
	h := localHandle(t, 1<<20)
	srcDir := t.TempDir()
	txt := writeTempFile(t, srcDir, "readme.txt", []byte("plain text content"))
	md := writeTempFile(t, srcDir, "readme.md", []byte("# Heading\n\nSome *markdown*.\n"))
	bd := &fakeBd{}

	results := Add(context.Background(), h, bd, "bd-1", []string{txt, md})

	if len(results) != 2 || results[0].Err != nil || results[1].Err != nil {
		t.Fatalf("Add results = %+v", results)
	}
	if got := results[0].Ref.Type; got != "text/plain" {
		t.Errorf("readme.txt type = %q, want text/plain", got)
	}
	if got := results[1].Ref.Type; got != "text/markdown" {
		t.Errorf("readme.md type = %q, want text/markdown", got)
	}
}

func TestAdd_RejectsFileOverMaxBytes(t *testing.T) {
	h := localHandle(t, 4)
	srcDir := t.TempDir()
	path := writeTempFile(t, srcDir, "big.bin", []byte("way too big"))
	bd := &fakeBd{}

	results := Add(context.Background(), h, bd, "bd-1", []string{path})

	if len(results) != 1 || results[0].Err == nil {
		t.Fatalf("Add results = %+v, want an over-limit error", results)
	}
	if len(bd.calls) != 0 {
		t.Errorf("bd calls = %v, want none: an oversized file must never reach bd", bd.calls)
	}
}

func TestAdd_RejectsInvalidName(t *testing.T) {
	h := localHandle(t, 1<<20)
	srcDir := t.TempDir()
	// attachref rejects a name ending in a dot (Windows cannot use one).
	path := writeTempFile(t, srcDir, "bad.", []byte("x"))
	bd := &fakeBd{}

	results := Add(context.Background(), h, bd, "bd-1", []string{path})

	if len(results) != 1 || results[0].Err == nil {
		t.Fatalf("Add results = %+v, want a name-validation error", results)
	}
	if len(bd.calls) != 0 {
		t.Errorf("bd calls = %v, want none: an invalid name must never reach bd", bd.calls)
	}
}

func TestAdd_ProcessesFilesIndependently(t *testing.T) {
	h := localHandle(t, 1<<20)
	srcDir := t.TempDir()
	good1 := writeTempFile(t, srcDir, "good1.txt", []byte("one"))
	bad := writeTempFile(t, srcDir, "bad.", []byte("x"))
	good2 := writeTempFile(t, srcDir, "good2.txt", []byte("two"))
	bd := &fakeBd{}

	results := Add(context.Background(), h, bd, "bd-1", []string{good1, bad, good2})

	if len(results) != 3 {
		t.Fatalf("Add results = %+v", results)
	}
	if results[0].Err != nil || results[2].Err != nil {
		t.Errorf("good files failed: %+v / %+v", results[0], results[2])
	}
	if results[1].Err == nil {
		t.Errorf("bad file succeeded, want an error")
	}
	if len(bd.calls) != 2 {
		t.Errorf("bd calls = %d, want 2 (one per good file)", len(bd.calls))
	}
}

func TestAdd_SkipsPutWhenBlobAlreadyExists(t *testing.T) {
	h := localHandle(t, 1<<20)
	srcDir := t.TempDir()
	path1 := writeTempFile(t, srcDir, "a.txt", []byte("same bytes"))
	path2 := writeTempFile(t, srcDir, "b.txt", []byte("same bytes"))
	bd := &fakeBd{}

	results := Add(context.Background(), h, bd, "bd-1", []string{path1, path2})

	if len(results) != 2 || results[0].Err != nil || results[1].Err != nil {
		t.Fatalf("Add results = %+v", results)
	}
	if results[0].Skipped {
		t.Errorf("first upload Skipped = true, want false")
	}
	if !results[1].Skipped {
		t.Errorf("second upload of identical bytes Skipped = false, want true")
	}
	if results[0].Ref.SHA256 != results[1].Ref.SHA256 {
		t.Errorf("identical content hashed to different sums: %s vs %s", results[0].Ref.SHA256, results[1].Ref.SHA256)
	}
}

// TestAdd_ReattachOfExistingBlobRefreshesLastModified is the test for race
// 6a (bd-t8j5.16): attaching bytes already present in the store must refresh
// the blob's LastModified, not only skip the Put. Otherwise a concurrent gc
// run measures the grace window from the blob's original upload rather than
// this renewed reference, and can delete a blob a comment was just written
// to attach.
func TestAdd_ReattachOfExistingBlobRefreshesLastModified(t *testing.T) {
	h, dir := localHandleWithDir(t, 1<<20)
	srcDir := t.TempDir()
	content := []byte("same bytes")
	path := writeTempFile(t, srcDir, "a.txt", content)
	bd := &fakeBd{}

	results := Add(context.Background(), h, bd, "bd-1", []string{path})
	if len(results) != 1 || results[0].Err != nil {
		t.Fatalf("first Add results = %+v", results)
	}
	key, err := h.Key(results[0].Ref.SHA256)
	if err != nil {
		t.Fatal(err)
	}

	old := time.Now().Add(-48 * time.Hour)
	blobPath := filepath.Join(dir, filepath.FromSlash(key))
	if err := os.Chtimes(blobPath, old, old); err != nil {
		t.Fatal(err)
	}
	before, err := h.Store.Stat(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	if !before.LastModified.Before(time.Now().Add(-time.Hour)) {
		t.Fatalf("backdated LastModified = %v, want well in the past", before.LastModified)
	}

	path2 := writeTempFile(t, srcDir, "b.txt", content)
	results2 := Add(context.Background(), h, bd, "bd-2", []string{path2})
	if len(results2) != 1 || results2[0].Err != nil {
		t.Fatalf("second Add results = %+v", results2)
	}
	if !results2[0].Skipped {
		t.Errorf("second Add Skipped = false, want true (identical bytes already stored)")
	}

	after, err := h.Store.Stat(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	if !after.LastModified.After(before.LastModified) {
		t.Fatalf("LastModified after re-attach = %v, want it to have advanced past %v", after.LastModified, before.LastModified)
	}
}

func TestAdd_BdFailureErrorIncludesBdOutput(t *testing.T) {
	h := localHandle(t, 1<<20)
	srcDir := t.TempDir()
	path := writeTempFile(t, srcDir, "notes.txt", []byte("hello world"))
	bd := &fakeBd{err: errCommandFailed, output: "bd: issue bd-1 not found"}

	results := Add(context.Background(), h, bd, "bd-1", []string{path})

	if len(results) != 1 || results[0].Err == nil {
		t.Fatalf("Add results = %+v, want the bd failure reported", results)
	}
	if !strings.Contains(results[0].Err.Error(), "bd: issue bd-1 not found") {
		t.Errorf("error = %v, want it to include bd's output", results[0].Err)
	}
}

func TestAdd_MaxBytesAtInt64MaxDoesNotOverflow(t *testing.T) {
	h := localHandle(t, math.MaxInt64)
	srcDir := t.TempDir()
	content := []byte("hello world")
	path := writeTempFile(t, srcDir, "notes.txt", content)
	bd := &fakeBd{}

	results := Add(context.Background(), h, bd, "bd-1", []string{path})

	if len(results) != 1 || results[0].Err != nil {
		t.Fatalf("Add results = %+v", results)
	}
	// MaxBytes+1 overflows to a negative int64 when MaxBytes is
	// math.MaxInt64; io.LimitReader treats a negative limit as "read
	// nothing", which would silently upload an empty blob instead of the
	// real file.
	if results[0].Ref.Size != int64(len(content)) {
		t.Fatalf("Ref.Size = %d, want %d: MaxBytes+1 must not overflow into a truncated read", results[0].Ref.Size, len(content))
	}
	if want := sha256Hex(content); results[0].Ref.SHA256 != want {
		t.Errorf("Ref.SHA256 = %s, want %s", results[0].Ref.SHA256, want)
	}
}

// TestAdd_RejectsFIFO guards against os.Open blocking forever on a named
// pipe with no writer: Add must reject it via the os.Stat regular-file check
// before ever calling os.Open, rather than hang past ctx's deadline (which
// does not bound a blocking local open).
func TestAdd_RejectsFIFO(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("named pipes need syscall.Mkfifo, POSIX-only")
	}
	h := localHandle(t, 1<<20)
	srcDir := t.TempDir()
	fifoPath := filepath.Join(srcDir, "pipe")
	if err := syscall.Mkfifo(fifoPath, 0o600); err != nil {
		t.Fatalf("Mkfifo: %v", err)
	}
	bd := &fakeBd{}

	done := make(chan []AddResult, 1)
	go func() { done <- Add(context.Background(), h, bd, "bd-1", []string{fifoPath}) }()

	select {
	case results := <-done:
		if len(results) != 1 || results[0].Err == nil {
			t.Fatalf("Add results = %+v, want a rejection error for a FIFO", results)
		}
		if !strings.Contains(results[0].Err.Error(), "not a regular file") {
			t.Errorf("error = %v, want it to say the path is not a regular file", results[0].Err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Add blocked on a FIFO instead of rejecting it up front")
	}
	if len(bd.calls) != 0 {
		t.Errorf("bd calls = %v, want none: a FIFO must never reach bd", bd.calls)
	}
}

// TestAdd_RejectsDirectory guards the same os.Stat check for a directory
// path, which os.Open would otherwise accept (a directory FD opens fine; the
// failure only surfaces later, on read).
func TestAdd_RejectsDirectory(t *testing.T) {
	h := localHandle(t, 1<<20)
	srcDir := t.TempDir()
	subdir := filepath.Join(srcDir, "subdir")
	if err := os.Mkdir(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	bd := &fakeBd{}

	results := Add(context.Background(), h, bd, "bd-1", []string{subdir})

	if len(results) != 1 || results[0].Err == nil {
		t.Fatalf("Add results = %+v, want a rejection error for a directory", results)
	}
	if !strings.Contains(results[0].Err.Error(), "not a regular file") {
		t.Errorf("error = %v, want it to say the path is not a regular file", results[0].Err)
	}
	if len(bd.calls) != 0 {
		t.Errorf("bd calls = %v, want none: a directory must never reach bd", bd.calls)
	}
}

func TestAdd_CrashOrder_BlobSurvivesAFailedComment(t *testing.T) {
	h := localHandle(t, 1<<20)
	srcDir := t.TempDir()
	path := writeTempFile(t, srcDir, "notes.txt", []byte("hello world"))
	bd := &fakeBd{err: errCommandFailed}

	results := Add(context.Background(), h, bd, "bd-1", []string{path})

	if len(results) != 1 || results[0].Err == nil {
		t.Fatalf("Add results = %+v, want the bd failure reported", results)
	}
	// Put happened before the failing bd call, so the blob is not lost: a
	// retried Add for the same bytes would find it already present.
	key, err := h.Key(results[0].Ref.SHA256)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.Store.Stat(context.Background(), key); err != nil {
		t.Errorf("blob missing after a failed bd comments add: %v", err)
	}
}

// TestAdd_HungBd_ReturnsTimeoutAndKeepsTheBlobWithoutAComment exercises Add
// through a real bdrun.Run, not fakeBd, so bd's own hang and bdrun's timeout
// bound are both on the path under test (bd-t8j5.20). Put lands before the
// bd call, so a timed-out comment must leave the blob in the store with no
// comment written, the same crash-order guarantee
// TestAdd_CrashOrder_BlobSurvivesAFailedComment checks for a plain failure.
func TestAdd_HungBd_ReturnsTimeoutAndKeepsTheBlobWithoutAComment(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake bd script needs a POSIX shell")
	}
	h := localHandle(t, 1<<20)
	srcDir := t.TempDir()
	path := writeTempFile(t, srcDir, "notes.txt", []byte("hello world"))

	binDir := t.TempDir()
	script := "#!/bin/sh\nsleep 30\n"
	if err := os.WriteFile(filepath.Join(binDir, "bd"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	bdPath := filepath.Join(binDir, "bd")
	bd := RunnerFunc(func(ctx context.Context, args ...string) (string, error) {
		return bdrun.Run(ctx, bdPath, t.TempDir(), args...)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	results := Add(ctx, h, bd, "bd-1", []string{path})
	elapsed := time.Since(start)

	if elapsed > 5*time.Second {
		t.Fatalf("Add took %v, want it bounded by the 200ms deadline plus WaitDelay, not the 30s sleep", elapsed)
	}
	if len(results) != 1 || results[0].Err == nil {
		t.Fatalf("Add results = %+v, want the timeout reported", results)
	}
	if !strings.Contains(results[0].Err.Error(), "timed out") {
		t.Errorf("error = %v, want it to name the timeout clearly", results[0].Err)
	}

	// Put ran before the hung bd call, so the blob is not lost: a retried
	// Add for the same bytes would find it already present.
	key, err := h.Key(results[0].Ref.SHA256)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.Store.Stat(context.Background(), key); err != nil {
		t.Errorf("blob missing after a hung bd comments add: %v", err)
	}
}
