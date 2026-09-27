package ui

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanderheijden86/beadwork/internal/attachref"
	"github.com/vanderheijden86/beadwork/internal/blobstore"
	"github.com/vanderheijden86/beadwork/internal/datasource"
	"github.com/vanderheijden86/beadwork/pkg/config"
	"github.com/vanderheijden86/beadwork/pkg/model"
)

// attachmentFixture builds one reference comment for content, ready to embed
// in a model.Issue's Comments. It returns the comment and the Ref, so a test
// can seed the same content at the matching blob store key.
func attachmentFixture(t *testing.T, commentID, name, mimeType string, content []byte, addedAt time.Time) (*model.Comment, attachref.Ref) {
	t.Helper()
	sum := sha256.Sum256(content)
	ref := attachref.Ref{
		SHA256: hex.EncodeToString(sum[:]),
		Size:   int64(len(content)),
		Type:   mimeType,
		Name:   name,
	}
	text, err := attachref.Format(ref)
	if err != nil {
		t.Fatalf("attachref.Format: %v", err)
	}
	return &model.Comment{
		ID:        commentID,
		Author:    "alice",
		Text:      text,
		CreatedAt: addedAt,
	}, ref
}

// attachmentTestModel builds a Model over one issue carrying attachment
// reference comments, wired to a project directory with a real .beads
// directory (NewCheckout requires one) and, when configured, an
// attachments.local_dir XDG config pointing at localDir. Returning localDir
// lets a test seed blobs at the exact key openProjectBlobStore will look
// under.
func attachmentTestModel(t *testing.T, comments []*model.Comment, configured bool) (Model, string) {
	t.Helper()
	projectDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(projectDir, ".beads"), 0o700); err != nil {
		t.Fatal(err)
	}
	localDir := filepath.Join(t.TempDir(), "blobs")

	xdgHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdgHome)
	if configured {
		cfgDir := filepath.Join(xdgHome, "b9s")
		if err := os.MkdirAll(cfgDir, 0o755); err != nil {
			t.Fatal(err)
		}
		yaml := fmt.Sprintf("attachments:\n  backend: local\n  local_dir: %q\n", localDir)
		if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(yaml), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	m := NewModel([]model.Issue{
		{ID: "bd-1", Title: "Has attachments", Status: model.StatusOpen, IssueType: model.TypeTask,
			CreatedAt: time.Now(), Comments: comments},
	}, "")
	m = m.WithConfig(config.Config{}, "proj", projectDir)
	m = m.WithSourceType(datasource.SourceTypeJSONLLocal)
	m.tree.SelectByID("bd-1")
	return m, localDir
}

// seedBlob stores content at the key openProjectBlobStore would resolve for
// ref, using the same Open() path production code uses, so a divergence in
// key layout fails this test rather than silently reading past it.
func seedBlob(t *testing.T, localDir, projectDir, projectName string, ref attachref.Ref, content []byte) {
	t.Helper()
	beadsDir, _ := projectBeadsDir(projectDir)
	srcInfo, err := blobstore.SourceInfoFromDataSource(
		datasource.DataSource{Type: datasource.SourceTypeJSONLLocal}, beadsDir, projectName)
	if err != nil {
		t.Fatalf("SourceInfoFromDataSource: %v", err)
	}
	cfg := &config.AttachmentsConfig{Backend: "local", LocalDir: localDir}
	handle, err := blobstore.Open(context.Background(), cfg, srcInfo)
	if err != nil {
		t.Fatalf("blobstore.Open: %v", err)
	}
	key, err := handle.Key(ref.SHA256)
	if err != nil {
		t.Fatalf("handle.Key: %v", err)
	}
	if err := handle.Store.Put(context.Background(), key, bytes.NewReader(content), int64(len(content)), ref.Type); err != nil {
		t.Fatalf("Store.Put: %v", err)
	}
}

func TestAttachmentList_ParsesReferenceComments(t *testing.T) {
	now := time.Now()
	comment, ref := attachmentFixture(t, "cmt-001", "notes.txt", "text/plain", []byte("hello world"), now)
	m, _ := attachmentTestModel(t, []*model.Comment{comment}, false)

	atts := m.AttachmentList()
	if len(atts) != 1 {
		t.Fatalf("AttachmentList() = %d attachments, want 1", len(atts))
	}
	if atts[0].Name != ref.Name || atts[0].SHA256 != ref.SHA256 || atts[0].Size != ref.Size {
		t.Fatalf("AttachmentList()[0] = %+v, want name=%s sha256=%s size=%d", atts[0], ref.Name, ref.SHA256, ref.Size)
	}
}

func TestAttachmentList_NoAttachmentsReturnsEmpty(t *testing.T) {
	m, _ := attachmentTestModel(t, nil, false)
	if atts := m.AttachmentList(); len(atts) != 0 {
		t.Fatalf("AttachmentList() = %v, want none", atts)
	}
}

func TestRKeyOpensAttachmentPickerOverSelectedIssue(t *testing.T) {
	now := time.Now()
	c1, _ := attachmentFixture(t, "cmt-001", "one.txt", "text/plain", []byte("one"), now)
	c2, _ := attachmentFixture(t, "cmt-002", "two.png", "image/png", []byte("two"), now.Add(time.Minute))
	m, _ := attachmentTestModel(t, []*model.Comment{c1, c2}, false)

	m, _ = pressBulkKey(t, m, runeKey("R"))

	if !m.ShowAttachmentPicker() {
		t.Fatal("R must open the attachment picker")
	}
	if got := m.AttachmentPickerCount(); got != 2 {
		t.Fatalf("AttachmentPickerCount() = %d, want 2", got)
	}
}

func TestRKeyOnIssueWithNoAttachmentsShowsStatusAndNoPicker(t *testing.T) {
	m, _ := attachmentTestModel(t, nil, false)

	m, _ = pressBulkKey(t, m, runeKey("R"))

	if m.ShowAttachmentPicker() {
		t.Fatal("R on an issue with no attachments must not open the picker")
	}
	if m.statusMsg == "" || m.statusIsError {
		t.Fatalf("expected a non-error status message, got %q (isError=%v)", m.statusMsg, m.statusIsError)
	}
}

func TestEscClosesAttachmentPickerWithoutOpening(t *testing.T) {
	now := time.Now()
	c1, _ := attachmentFixture(t, "cmt-001", "one.txt", "text/plain", []byte("one"), now)
	m, _ := attachmentTestModel(t, []*model.Comment{c1}, false)
	m, _ = pressBulkKey(t, m, runeKey("R"))

	m, cmd := pressBulkKey(t, m, tea.KeyMsg{Type: tea.KeyEsc})

	if m.ShowAttachmentPicker() {
		t.Fatal("esc must close the picker")
	}
	if cmd != nil {
		t.Fatal("esc must not dispatch an open command")
	}
	if m.LastAttachmentOpen() != nil {
		t.Fatal("esc must not record an attachment-open attempt")
	}
}

// runAttachmentOpenCmd opens the picker, presses enter on the selected
// attachment and runs the resulting tea.Cmd synchronously (the picker's
// enter case only returns the Cmd; nothing in Update executes it), feeding
// its result back into Update the same way the real event loop would.
func runAttachmentOpenCmd(t *testing.T, m Model) Model {
	t.Helper()
	m, _ = pressBulkKey(t, m, runeKey("R"))
	if !m.ShowAttachmentPicker() {
		t.Fatal("picker did not open; fixture must carry an attachment")
	}
	m, cmd := pressBulkKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter on a selected attachment must return a Cmd")
	}
	msg := cmd()
	updated, _ := m.Update(msg)
	return updated.(Model)
}

func TestEnterInTestModeRecordsOpenerWithoutRunningIt(t *testing.T) {
	t.Setenv("B9S_TEST_MODE", "1")
	now := time.Now()
	content := []byte("plain text content")
	comment, ref := attachmentFixture(t, "cmt-001", "notes.txt", "text/plain", content, now)
	m, localDir := attachmentTestModel(t, []*model.Comment{comment}, true)
	seedBlob(t, localDir, m.activeProjectPath, m.activeProjectName, ref, content)

	m = runAttachmentOpenCmd(t, m)

	rec := m.LastAttachmentOpen()
	if rec == nil {
		t.Fatal("expected a recorded attachment-open attempt")
	}
	if rec.IsError {
		t.Fatalf("unexpected error status: %q", rec.StatusMsg)
	}
	if len(rec.OpenerCmd) == 0 {
		t.Fatal("expected the would-be opener command to be recorded")
	}
	if rec.Path == "" {
		t.Fatal("expected the downloaded path to be recorded")
	}
	if _, err := os.Stat(rec.Path); err != nil {
		t.Fatalf("downloaded file must exist at recorded path: %v", err)
	}
}

func TestEnterRefusesToOpenContentSniffedAsHTML(t *testing.T) {
	t.Setenv("B9S_TEST_MODE", "1")
	now := time.Now()
	// The machine line claims text/plain; the actual bytes sniff as HTML.
	// attachmentOpenDownload must trust the sniff, not the claim.
	content := []byte("<!DOCTYPE html><html><body>hello</body></html>")
	comment, ref := attachmentFixture(t, "cmt-001", "fake.txt", "text/plain", content, now)
	m, localDir := attachmentTestModel(t, []*model.Comment{comment}, true)
	seedBlob(t, localDir, m.activeProjectPath, m.activeProjectName, ref, content)

	m = runAttachmentOpenCmd(t, m)

	rec := m.LastAttachmentOpen()
	if rec == nil {
		t.Fatal("expected a recorded attachment-open attempt")
	}
	if rec.IsError {
		t.Fatalf("a refusal is not an error status: %q", rec.StatusMsg)
	}
	if len(rec.OpenerCmd) != 0 {
		t.Fatalf("opener must not be recorded for a refused content type, got %v", rec.OpenerCmd)
	}
	if want := "opening refused"; !bytes.Contains([]byte(rec.StatusMsg), []byte(want)) {
		t.Fatalf("status message = %q, want it to contain %q", rec.StatusMsg, want)
	}
}

func TestEnterWithoutAttachmentsConfigShowsHint(t *testing.T) {
	now := time.Now()
	comment, _ := attachmentFixture(t, "cmt-001", "notes.txt", "text/plain", []byte("hi"), now)
	m, _ := attachmentTestModel(t, []*model.Comment{comment}, false)

	m = runAttachmentOpenCmd(t, m)

	rec := m.LastAttachmentOpen()
	if rec == nil {
		t.Fatal("expected a recorded attachment-open attempt")
	}
	if !rec.IsError {
		t.Fatalf("expected an error status, got %q", rec.StatusMsg)
	}
	if rec.StatusMsg != notConfiguredHint {
		t.Fatalf("status message = %q, want %q", rec.StatusMsg, notConfiguredHint)
	}
}
