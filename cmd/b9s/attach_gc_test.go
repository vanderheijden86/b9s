package main

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vanderheijden86/b9s/internal/attachref"
	"github.com/vanderheijden86/b9s/internal/blobstore"
	"github.com/vanderheijden86/b9s/pkg/config"
)

func TestAttachGC_DryRun_ReportsUnreferencedBlobWithoutDeleting(t *testing.T) {
	beadsDir, _, _ := attachTestProject(t, 0)
	projectName := filepath.Base(filepath.Dir(beadsDir))
	seedReferencedComment(t, beadsDir)
	orphan := seedAttachment(t, beadsDir, projectName, []byte("orphan"), "orphan.txt", "text/plain")

	var stdout, stderr bytes.Buffer
	code := runAttach([]string{"gc", "--grace", "0s"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), orphan.SHA256) {
		t.Errorf("stdout = %q, want the orphan blob's hash reported", stdout.String())
	}
	if !strings.Contains(stdout.String(), "dry run") {
		t.Errorf("stdout = %q, want a dry-run hint that --apply deletes", stdout.String())
	}
	assertBlobExists(t, beadsDir, projectName, orphan)
}

func TestAttachGC_Apply_DeletesUnreferencedBlobPastGrace(t *testing.T) {
	beadsDir, _, _ := attachTestProject(t, 0)
	projectName := filepath.Base(filepath.Dir(beadsDir))
	seedReferencedComment(t, beadsDir)
	orphan := seedAttachment(t, beadsDir, projectName, []byte("orphan"), "orphan.txt", "text/plain")

	var stdout, stderr bytes.Buffer
	code := runAttach([]string{"gc", "--apply", "--grace", "0s"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "deleted: 1") {
		t.Errorf("stdout = %q, want the summary to report one deletion", stdout.String())
	}
	assertBlobGone(t, beadsDir, projectName, orphan)
}

func TestAttachGC_KeepsAReferencedBlob(t *testing.T) {
	beadsDir, _, _ := attachTestProject(t, 0)
	projectName := filepath.Base(filepath.Dir(beadsDir))
	kept := seedAttachment(t, beadsDir, projectName, []byte("keep"), "keep.txt", "text/plain")
	seedComment(t, beadsDir, kept)

	var stdout, stderr bytes.Buffer
	code := runAttach([]string{"gc", "--apply", "--grace", "0s"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	if strings.Contains(stdout.String(), kept.SHA256) {
		t.Errorf("stdout = %q, a referenced blob must never be listed as a candidate", stdout.String())
	}
	if !strings.Contains(stdout.String(), "referenced: 1") {
		t.Errorf("stdout = %q, want the summary to count the blob as referenced", stdout.String())
	}
	assertBlobExists(t, beadsDir, projectName, kept)
}

func TestAttachGC_JSONReportsCandidateAndDeletion(t *testing.T) {
	beadsDir, _, _ := attachTestProject(t, 0)
	projectName := filepath.Base(filepath.Dir(beadsDir))
	seedReferencedComment(t, beadsDir)
	orphan := seedAttachment(t, beadsDir, projectName, []byte("orphan"), "orphan.txt", "text/plain")

	var stdout, stderr bytes.Buffer
	code := runAttach([]string{"gc", "--apply", "--grace", "0s", "--json"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	var report jsonGCReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decoding --json output: %v\noutput: %s", err, stdout.String())
	}
	if !report.Applied || report.Deleted != 1 || report.DeletedBytes != orphan.Size {
		t.Fatalf("report = %+v, want Applied and one deletion of %d bytes", report, orphan.Size)
	}
	if len(report.Candidates) != 1 || report.Candidates[0].SHA256 != orphan.SHA256 || !report.Candidates[0].Deleted {
		t.Fatalf("report.Candidates = %+v, want one deleted candidate for %s", report.Candidates, orphan.SHA256)
	}
	assertBlobGone(t, beadsDir, projectName, orphan)
}

func TestAttachGC_AllowEmptyReferencesRequiredForApply(t *testing.T) {
	beadsDir, _, _ := attachTestProject(t, 0)
	projectName := filepath.Base(filepath.Dir(beadsDir))
	orphan := seedAttachment(t, beadsDir, projectName, []byte("orphan"), "orphan.txt", "text/plain")

	var stdout, stderr bytes.Buffer
	code := runAttach([]string{"gc", "--apply", "--grace", "0s"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("exit = 0, want a refusal: the referenced set is empty and --allow-empty-references was not given")
	}
	assertBlobExists(t, beadsDir, projectName, orphan)

	stdout.Reset()
	stderr.Reset()
	code = runAttach([]string{"gc", "--apply", "--grace", "0s", "--allow-empty-references"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q, want success with --allow-empty-references", code, stderr.String())
	}
	assertBlobGone(t, beadsDir, projectName, orphan)
}

// seedReferencedComment attaches an unrelated, never-uploaded ref to bd-1 so
// a test's own blob is unreferenced while the project's referenced set as a
// whole is non-empty, keeping --allow-empty-references out of scope for
// tests that are not specifically about that guard.
func seedReferencedComment(t *testing.T, beadsDir string) attachref.Ref {
	t.Helper()
	ref := attachref.Ref{SHA256: sha256Hex([]byte("kept elsewhere")), Size: 14, Type: "text/plain", Name: "elsewhere.txt"}
	seedComment(t, beadsDir, ref)
	return ref
}

func assertBlobExists(t *testing.T, beadsDir, projectName string, ref attachref.Ref) {
	t.Helper()
	h, key := openTestBlobKey(t, beadsDir, projectName, ref)
	if _, err := h.Store.Stat(context.Background(), key); err != nil {
		t.Errorf("blob %s missing: %v", ref.SHA256, err)
	}
}

func assertBlobGone(t *testing.T, beadsDir, projectName string, ref attachref.Ref) {
	t.Helper()
	h, key := openTestBlobKey(t, beadsDir, projectName, ref)
	if _, err := h.Store.Stat(context.Background(), key); err == nil {
		t.Errorf("blob %s still present, want it deleted", ref.SHA256)
	}
}

func openTestBlobKey(t *testing.T, beadsDir, projectName string, ref attachref.Ref) (*blobstore.Handle, string) {
	t.Helper()
	h, err := blobstore.Open(context.Background(), &config.AttachmentsConfig{Backend: "local"}, blobstore.SourceInfo{
		Kind: blobstore.SourceJSONL, BeadsDir: beadsDir, ProjectName: projectName,
	})
	if err != nil {
		t.Fatal(err)
	}
	key, err := h.Key(ref.SHA256)
	if err != nil {
		t.Fatal(err)
	}
	return h, key
}
