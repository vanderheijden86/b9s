package attach

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vanderheijden86/beadwork/internal/attachref"
	"github.com/vanderheijden86/beadwork/internal/blobstore"
)

// fakeStore is a minimal blobstore.Store whose Open is the only method
// Download calls; the rest exist only to satisfy the interface.
type fakeStore struct {
	open func(ctx context.Context, key string) (io.ReadCloser, error)
}

func (f *fakeStore) Put(context.Context, string, io.Reader, int64, string) error { return nil }
func (f *fakeStore) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	return f.open(ctx, key)
}
func (f *fakeStore) Stat(context.Context, string) (blobstore.Info, error) {
	return blobstore.Info{}, nil
}
func (f *fakeStore) Delete(context.Context, string) error { return nil }
func (f *fakeStore) List(context.Context, string, func(blobstore.Info) error) error {
	return nil
}
func (f *fakeStore) URL(context.Context, string, time.Duration, string) (string, error) {
	return "", nil
}

func fakeHandle(store blobstore.Store) *blobstore.Handle {
	return &blobstore.Handle{
		Store: store,
		Key:   func(hash string) (string, error) { return hash, nil },
	}
}

// countingReader tallies every byte Read returns, so a test can assert a
// bound on how much the reader beneath it was ever asked to give up.
type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

func TestDownload_FetchesAndVerifiesAStoredBlob(t *testing.T) {
	h := localHandle(t, 1<<20)
	srcDir := t.TempDir()
	path := writeTempFile(t, srcDir, "notes.txt", []byte("hello world"))
	bd := &fakeBd{}
	results := Add(context.Background(), h, bd, "bd-1", []string{path})
	if results[0].Err != nil {
		t.Fatal(results[0].Err)
	}
	att := attachref.Attachment{Ref: results[0].Ref}
	destDir := t.TempDir()

	tempPath, err := Download(context.Background(), h, att, destDir, DownloadModeNormal)

	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	defer os.Remove(tempPath)
	got, err := os.ReadFile(tempPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello world" {
		t.Errorf("downloaded content = %q, want %q", got, "hello world")
	}
	if filepath.Dir(tempPath) != destDir {
		t.Errorf("temp file dir = %s, want %s (same filesystem as the destination)", filepath.Dir(tempPath), destDir)
	}
}

func TestDownload_RejectsAndRemovesOnHashMismatch(t *testing.T) {
	h := localHandle(t, 1<<20)
	srcDir := t.TempDir()
	path := writeTempFile(t, srcDir, "notes.txt", []byte("hello world"))
	bd := &fakeBd{}
	results := Add(context.Background(), h, bd, "bd-1", []string{path})
	if results[0].Err != nil {
		t.Fatal(results[0].Err)
	}
	// Simulate corruption: claim a sha256 that does not match the stored bytes.
	att := attachref.Attachment{Ref: attachref.Ref{
		SHA256: "0000000000000000000000000000000000000000000000000000000000000000"[:64],
		Size:   results[0].Ref.Size,
		Type:   results[0].Ref.Type,
		Name:   results[0].Ref.Name,
	}}
	key, err := h.Key(results[0].Ref.SHA256)
	if err != nil {
		t.Fatal(err)
	}
	wrongKey, err := h.Key(att.SHA256)
	if err != nil {
		t.Fatal(err)
	}
	// Put the real bytes under the wrong key, so Download reads bytes whose
	// hash does not match the sha256 it was asked to verify.
	rc, err := h.Store.Open(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Store.Put(context.Background(), wrongKey, rc, results[0].Ref.Size, results[0].Ref.Type); err != nil {
		t.Fatal(err)
	}
	rc.Close()
	destDir := t.TempDir()

	_, err = Download(context.Background(), h, att, destDir, DownloadModeNormal)

	if err == nil {
		t.Fatal("Download: err = nil, want a hash-mismatch error")
	}
	entries, err := os.ReadDir(destDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("destDir entries = %v, want none: a failed download must not leave a temp file behind", entries)
	}
}

func TestDownload_RejectsSizeMismatchBeforeHash(t *testing.T) {
	content := []byte("hello world")
	store := &fakeStore{open: func(context.Context, string) (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(content)), nil
	}}
	h := fakeHandle(store)
	att := attachref.Attachment{Ref: attachref.Ref{
		SHA256: sha256Hex(content),
		Size:   int64(len(content)) + 1, // claims one more byte than the store holds
		Type:   "text/plain",
		Name:   "notes.txt",
	}}
	destDir := t.TempDir()

	_, err := Download(context.Background(), h, att, destDir, DownloadModeNormal)

	// The hash of the bytes actually returned matches att.SHA256, so a check
	// that ran the hash comparison first would let this through; the size
	// mismatch must be caught before that comparison ever runs.
	if err == nil {
		t.Fatal("Download: err = nil, want a size-mismatch error")
	}
	if strings.Contains(err.Error(), "hash to") {
		t.Errorf("error = %v, want the size mismatch reported, not a hash comparison", err)
	}
	if !strings.Contains(err.Error(), "size") {
		t.Errorf("error = %v, want it to name the size mismatch", err)
	}
	entries, rdErr := os.ReadDir(destDir)
	if rdErr != nil {
		t.Fatal(rdErr)
	}
	if len(entries) != 0 {
		t.Errorf("destDir entries = %v, want none: a failed download must not leave a temp file behind", entries)
	}
}

func TestDownload_ReadsAtMostSizePlusOneBytes(t *testing.T) {
	content := bytes.Repeat([]byte("x"), 1<<20) // far more than the claimed size
	cr := &countingReader{r: bytes.NewReader(content)}
	store := &fakeStore{open: func(context.Context, string) (io.ReadCloser, error) {
		return io.NopCloser(cr), nil
	}}
	h := fakeHandle(store)
	att := attachref.Attachment{Ref: attachref.Ref{
		SHA256: sha256Hex(content[:10]), // deliberately wrong; only the read bound matters here
		Size:   10,
		Type:   "text/plain",
		Name:   "notes.txt",
	}}
	destDir := t.TempDir()

	Download(context.Background(), h, att, destDir, DownloadModeNormal)

	if cr.n > att.Size+1 {
		t.Errorf("bytes read from the store = %d, want at most %d (att.Size+1)", cr.n, att.Size+1)
	}
}

func TestSafeJoin_AcceptsABareName(t *testing.T) {
	dir := t.TempDir()

	got, err := SafeJoin(dir, "notes.txt")

	if err != nil {
		t.Fatalf("SafeJoin: %v", err)
	}
	if got != filepath.Join(dir, "notes.txt") {
		t.Errorf("SafeJoin = %s", got)
	}
}

func TestSafeJoin_RejectsAPathSeparator(t *testing.T) {
	dir := t.TempDir()

	_, err := SafeJoin(dir, "sub/notes.txt")

	if err == nil {
		t.Fatal("SafeJoin: err = nil, want a rejection of an embedded path separator")
	}
}

func TestSafeJoin_RejectsDotAndDotDot(t *testing.T) {
	dir := t.TempDir()

	for _, name := range []string{".", ".."} {
		if _, err := SafeJoin(dir, name); err == nil {
			t.Errorf("SafeJoin(%q): err = nil, want a rejection", name)
		}
	}
}

func TestURL_RefusedForTheLocalBackend(t *testing.T) {
	h := localHandle(t, 1<<20)
	att := attachref.Attachment{Ref: attachref.Ref{SHA256: testSHA, Size: 1, Type: "text/plain", Name: "notes.txt"}}

	_, err := URL(context.Background(), h, att)

	if err == nil {
		t.Fatal("URL: err = nil, want the local backend refused")
	}
	if strings.Contains(err.Error(), "file://") {
		t.Errorf("URL error = %v, must not suggest a file:// link", err)
	}
}
