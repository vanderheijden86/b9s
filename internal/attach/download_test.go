package attach

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vanderheijden86/beadwork/internal/attachref"
)

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

	tempPath, err := Download(context.Background(), h, att, destDir)

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

	_, err = Download(context.Background(), h, att, destDir)

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
