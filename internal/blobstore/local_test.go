package blobstore

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalStoreContract(t *testing.T) {
	runContract(t, NewLocal(t.TempDir()))
}

// Put's idempotent short-circuit only fires on an exact size match; a blob
// path already occupied by content of a different size is a real write, not
// a retry, so Put must overwrite it rather than silently keeping the old
// bytes.
func TestLocalOverwritesDifferentSizeBlob(t *testing.T) {
	root := t.TempDir()
	key := "ws/db/sha256/ab/abcdef0123"
	dir := filepath.Join(root, "ws", "db", "sha256", "ab")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	stale := []byte("stale")
	if err := os.WriteFile(filepath.Join(dir, "abcdef0123"), stale, 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewLocal(root)
	body := []byte("hello attachment")
	if err := s.Put(context.Background(), key, bytes.NewReader(body), int64(len(body)), "text/plain"); err != nil {
		t.Fatalf("Put over a different-size blob: %v", err)
	}

	rc, err := s.Open(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(rc)
	rc.Close()
	if !bytes.Equal(got, body) {
		t.Fatalf("Open after overwrite read %q, want %q", got, body)
	}
}

// A symlink planted at the first path segment a key resolves to must not let
// Put escape root, even though the deeper segments under it do not exist yet
// and so cannot be caught by a plain "does this path exist" check.
func TestLocalPutRejectsSymlinkedFirstSegment(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()

	if err := os.Symlink(outside, filepath.Join(root, "ws")); err != nil {
		t.Fatal(err)
	}

	s := NewLocal(root)
	body := []byte("attacker-controlled")
	err := s.Put(context.Background(), "ws/db/sha256/ab/abcdef0123", bytes.NewReader(body), int64(len(body)), "text/plain")
	if err == nil {
		t.Fatal("Put through a symlinked first segment succeeded, want error")
	}

	entries, readErr := os.ReadDir(outside)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("Put wrote into the symlink target: %v", entries)
	}
}

// A dangling symlink at a blob's own path is not "not yet created": Put must
// refuse it rather than silently replacing it, and Open must refuse it
// rather than reporting ErrNotFound as if nothing were there.
func TestLocalRejectsDanglingSymlinkAtBlobPath(t *testing.T) {
	root := t.TempDir()
	key := "ws/db/sha256/ab/abcdef0123"
	dir := filepath.Join(root, "ws", "db", "sha256", "ab")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "abcdef0123")
	if err := os.Symlink(filepath.Join(dir, "does-not-exist"), link); err != nil {
		t.Fatal(err)
	}

	s := NewLocal(root)
	body := []byte("hello")
	if err := s.Put(context.Background(), key, bytes.NewReader(body), int64(len(body)), "text/plain"); err == nil {
		t.Fatal("Put through a dangling symlink succeeded, want error")
	}
	if _, err := s.Open(context.Background(), key); err == nil || errors.Is(err, ErrNotFound) {
		t.Fatalf("Open on a dangling symlink: err = %v, want a non-ErrNotFound error", err)
	}

	if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("the dangling symlink was replaced instead of rejected: %v, %v", fi, err)
	}
}

// TestCleanRelPathRejectsUnsafeKeys checks the check every blobstore backend
// shares: none of these forms may reach the filesystem or an S3 key, since
// a blob_key can arrive over a Dolt pull from another party.
func TestCleanRelPathRejectsUnsafeKeys(t *testing.T) {
	unsafe := []string{
		"",
		"   ",
		"/abs/path",
		"../escape",
		"a/../../b",
		"a/./b",
		"a/",
		"a//b",
		"trailing/.",
		"back\\slash",
		"control\x00char",
		"bell\x07char",
		"delete\x7fchar",
		" a/b",
		"a/b ",
		"a /b",
		"a/b\t",
	}
	for _, key := range unsafe {
		if _, err := cleanRelPath(key); err == nil {
			t.Errorf("cleanRelPath(%q) accepted an unsafe key", key)
		}
	}
}

func TestCleanRelPathAcceptsSafeKeys(t *testing.T) {
	safe := map[string]string{
		"ws/db/sha256/ab/abcdef0123": "ws/db/sha256/ab/abcdef0123",
		"a":                          "a",
	}
	for key, want := range safe {
		got, err := cleanRelPath(key)
		if err != nil {
			t.Errorf("cleanRelPath(%q) = %v, want nil error", key, err)
			continue
		}
		if filepath.ToSlash(got) != want {
			t.Errorf("cleanRelPath(%q) = %q, want %q", key, got, want)
		}
	}
}
