package ui

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	t.Setenv("TMPDIR", t.TempDir())
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
	if filepath.Base(rec.Path) != "notes.txt" {
		t.Fatalf("recorded path = %q, want it to end in the attachment name notes.txt (macOS `open` picks the opener from the extension)", rec.Path)
	}
}

func TestEnterRefusesToOpenContentSniffedAsHTML(t *testing.T) {
	t.Setenv("B9S_TEST_MODE", "1")
	t.Setenv("TMPDIR", t.TempDir())
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
	t.Setenv("TMPDIR", t.TempDir())
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

// TestFooterWritesHyperlinkRawAfterSanitizingLabel is review item 1: the
// footer must not run sanitizeTerminalLine over the OSC 8 escape codes
// (attachmentOpenWeb carries them in a separate field precisely so this
// cannot happen), or a B9S_WEB link collapses to its plain-text label.
func TestFooterWritesHyperlinkRawAfterSanitizingLabel(t *testing.T) {
	m, _ := attachmentTestModel(t, nil, false)
	m.width = 80
	const url = "https://example.com/presigned/diagram.png"
	m.statusMsg = "diagram.png:"
	m.statusHyperlink = osc8Hyperlink(url, url)

	footer := (&m).renderFooter()

	if !strings.Contains(footer, "\x1b]8;;"+url) {
		t.Fatalf("footer = %q, want it to contain the raw OSC 8 hyperlink escape for %s", footer, url)
	}
	if !strings.Contains(footer, url) {
		t.Fatalf("footer = %q, want it to contain the URL %s", footer, url)
	}
}

// TestEnterOnWebModeRefusesLocalBackendWithoutSettingHyperlink exercises
// attachmentOpenWeb end to end against a non-S3 handle (refused, since
// presigned links need S3) to confirm attachmentOpenResultMsg.hyperlink,
// not statusMsg, is where a link would land; the S3 success path itself
// needs a real or mocked S3 endpoint and is covered by the blobstore
// package's own gated integration tests.
func TestEnterOnWebModeRefusesLocalBackendWithoutSettingHyperlink(t *testing.T) {
	t.Setenv("B9S_WEB", "1")
	t.Setenv("TMPDIR", t.TempDir())
	now := time.Now()
	comment, ref := attachmentFixture(t, "cmt-001", "notes.txt", "text/plain", []byte("hi"), now)
	m, localDir := attachmentTestModel(t, []*model.Comment{comment}, true)
	seedBlob(t, localDir, m.activeProjectPath, m.activeProjectName, ref, []byte("hi"))

	m = runAttachmentOpenCmd(t, m)

	rec := m.LastAttachmentOpen()
	if rec == nil {
		t.Fatal("expected a recorded attachment-open attempt")
	}
	if !rec.IsError {
		t.Fatalf("expected the local backend to refuse a presigned link, got %q", rec.StatusMsg)
	}
	if m.statusHyperlink != "" {
		t.Fatalf("statusHyperlink = %q, want empty: the local backend never reaches attachmentOpenWeb's success path", m.statusHyperlink)
	}
}

// pngBytes, pdfBytes and plainTextBytes sniff (http.DetectContentType) to
// the exact base MIME types openAllowedContentClass maps to "image",
// "pdf" and "text": real signature bytes, not just a claimed type=, since
// attachmentOpenDownload trusts the sniff.
var pngBytes = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")

var pdfBytes = []byte("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n1 0 obj\n")

var plainTextBytes = []byte("just some plain text content")

func TestEnterOpenerAllowlist(t *testing.T) {
	tests := []struct {
		name    string
		attName string
		content []byte
		claimed string
		allowed bool
	}{
		{name: "png allowed", attName: "diagram.png", content: pngBytes, claimed: "image/png", allowed: true},
		{name: "pdf allowed", attName: "report.pdf", content: pdfBytes, claimed: "application/pdf", allowed: true},
		{name: "txt allowed", attName: "notes.txt", content: plainTextBytes, claimed: "text/plain", allowed: true},
		{name: "terminal plist refused", attName: "evil.terminal", content: []byte("<?xml version=\"1.0\"?><plist></plist>"), claimed: "text/plain", allowed: false},
		{name: "sh text refused", attName: "evil.sh", content: []byte("#!/bin/sh\necho hi\n"), claimed: "text/plain", allowed: false},
		{name: "jar zip refused", attName: "evil.jar", content: []byte("PK\x03\x04\x14\x00\x00\x00\x08\x00"), claimed: "application/java-archive", allowed: false},
		{name: "svg refused", attName: "evil.svg", content: []byte("<svg xmlns=\"http://www.w3.org/2000/svg\"></svg>"), claimed: "image/svg+xml", allowed: false},
		{name: "html refused", attName: "evil.html", content: []byte("<!DOCTYPE html><html><body>hi</body></html>"), claimed: "text/html", allowed: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("B9S_TEST_MODE", "1")
			t.Setenv("TMPDIR", t.TempDir())
			now := time.Now()
			comment, ref := attachmentFixture(t, "cmt-001", tc.attName, tc.claimed, tc.content, now)
			m, localDir := attachmentTestModel(t, []*model.Comment{comment}, true)
			seedBlob(t, localDir, m.activeProjectPath, m.activeProjectName, ref, tc.content)

			m = runAttachmentOpenCmd(t, m)

			rec := m.LastAttachmentOpen()
			if rec == nil {
				t.Fatal("expected a recorded attachment-open attempt")
			}
			if rec.IsError {
				t.Fatalf("unexpected error status: %q", rec.StatusMsg)
			}
			opened := len(rec.OpenerCmd) != 0
			if opened != tc.allowed {
				t.Fatalf("opener recorded = %v (cmd=%v, status=%q), want allowed=%v", opened, rec.OpenerCmd, rec.StatusMsg, tc.allowed)
			}
			if !tc.allowed {
				if !strings.Contains(rec.StatusMsg, "opening refused") {
					t.Fatalf("status message = %q, want it to contain %q", rec.StatusMsg, "opening refused")
				}
			}
		})
	}
}

// TestAttachTempDirIsReusedAcrossOpens is review item 4: attachTempDir is
// created once per run and reused, not once per open (which leaked one
// directory per R+enter). Each open still gets its own subdirectory of it,
// so two attachments sharing a file name never collide.
func TestAttachTempDirIsReusedAcrossOpens(t *testing.T) {
	t.Setenv("B9S_TEST_MODE", "1")
	t.Setenv("TMPDIR", t.TempDir())
	now := time.Now()
	c1, ref1 := attachmentFixture(t, "cmt-001", "one.txt", "text/plain", []byte("one"), now)
	c2, ref2 := attachmentFixture(t, "cmt-002", "two.txt", "text/plain", []byte("two"), now.Add(time.Minute))
	m, localDir := attachmentTestModel(t, []*model.Comment{c1, c2}, true)
	seedBlob(t, localDir, m.activeProjectPath, m.activeProjectName, ref1, []byte("one"))
	seedBlob(t, localDir, m.activeProjectPath, m.activeProjectName, ref2, []byte("two"))

	m = runAttachmentOpenCmd(t, m)
	firstDir := m.attachTemp.dir
	firstPath := m.LastAttachmentOpen().Path
	if firstDir == "" {
		t.Fatal("expected attachTemp.dir to be set after the first open")
	}

	// Select the second attachment and open it too.
	m, _ = pressBulkKey(t, m, runeKey("R"))
	m.attachmentPicker.MoveDown()
	m, cmd := pressBulkKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter on the second attachment must return a Cmd")
	}
	updated, _ := m.Update(cmd())
	m = updated.(Model)

	if m.attachTemp.dir != firstDir {
		t.Fatalf("attachTemp.dir changed across opens: first=%q second=%q, want the parent directory reused", firstDir, m.attachTemp.dir)
	}
	secondPath := m.LastAttachmentOpen().Path
	if secondPath == firstPath {
		t.Fatalf("both opens recorded the same path %q, want distinct subdirectories", secondPath)
	}
	if filepath.Dir(firstPath) == filepath.Dir(secondPath) {
		t.Fatalf("both downloads landed in the same subdirectory %q, want one per open", filepath.Dir(firstPath))
	}
	if !strings.HasPrefix(firstPath, firstDir) || !strings.HasPrefix(secondPath, firstDir) {
		t.Fatalf("expected both paths under the shared per-run dir %q, got %q and %q", firstDir, firstPath, secondPath)
	}
}

// TestStopRemovesAttachTempDir is review item 4: the per-run directory is
// removed once, at program exit (Model.Stop), not after each open, since
// the system opener may still be reading the file when Update moves on.
func TestStopRemovesAttachTempDir(t *testing.T) {
	t.Setenv("B9S_TEST_MODE", "1")
	t.Setenv("TMPDIR", t.TempDir())
	now := time.Now()
	comment, ref := attachmentFixture(t, "cmt-001", "notes.txt", "text/plain", []byte("hi"), now)
	m, localDir := attachmentTestModel(t, []*model.Comment{comment}, true)
	seedBlob(t, localDir, m.activeProjectPath, m.activeProjectName, ref, []byte("hi"))

	m = runAttachmentOpenCmd(t, m)
	dir := m.attachTemp.dir
	if dir == "" {
		t.Fatal("expected attachTemp.dir to be set after an open")
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("attach temp dir must exist before Stop: %v", err)
	}

	(&m).Stop()

	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("attach temp dir still exists after Stop: err=%v", err)
	}
}

// TestStopOnOriginalModelRemovesAttachTempDir is the regression case for the
// leak in the real program: Update always returns a new copy of Model, and
// cmd/b9s/main.go defers Stop on the ORIGINAL value it passed to
// tea.NewProgram, which is never reassigned to any of those copies. Before
// attachTemp became a shared pointer, that original copy's temp-dir field
// stayed empty forever, so Stop had nothing to remove and the directory
// leaked on every real exit. Only Stop reaching the same attachTempState
// every copy points at proves the fix.
func TestStopOnOriginalModelRemovesAttachTempDir(t *testing.T) {
	t.Setenv("B9S_TEST_MODE", "1")
	t.Setenv("TMPDIR", t.TempDir())
	now := time.Now()
	comment, ref := attachmentFixture(t, "cmt-001", "notes.txt", "text/plain", []byte("hi"), now)
	orig, localDir := attachmentTestModel(t, []*model.Comment{comment}, true)
	seedBlob(t, localDir, orig.activeProjectPath, orig.activeProjectName, ref, []byte("hi"))

	// runAttachmentOpenCmd takes its argument by value, so orig itself is
	// never touched by the R+Enter+Update sequence below: it stays exactly
	// the pre-run value that main.go's defer m.Stop() would operate on.
	updated := runAttachmentOpenCmd(t, orig)
	dir := updated.attachTemp.dir
	if dir == "" {
		t.Fatal("expected attachTemp.dir to be set after an open")
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("attach temp dir must exist before Stop: %v", err)
	}

	(&orig).Stop()

	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("attach temp dir still exists after orig.Stop(): err=%v", err)
	}
}

// TestRInLabelPickerGoesToInputNotAttachmentPicker is review item 5: R must
// not steal the letter from the label picker's fuzzy-search input.
func TestRInLabelPickerGoesToInputNotAttachmentPicker(t *testing.T) {
	now := time.Now()
	comment, _ := attachmentFixture(t, "cmt-001", "notes.txt", "text/plain", []byte("hi"), now)
	m, _ := attachmentTestModel(t, []*model.Comment{comment}, false)
	m.labelPicker.SetLabels([]string{"bug", "chore"}, map[string]int{"bug": 1, "chore": 1})
	m.showLabelPicker = true
	m.focused = focusLabelPicker

	m, _ = pressBulkKey(t, m, runeKey("R"))

	if m.ShowAttachmentPicker() {
		t.Fatal("R while the label picker is focused must not open the attachment picker")
	}
	if got := m.labelPicker.InputValue(); got != "R" {
		t.Fatalf("labelPicker.InputValue() = %q, want %q (R must reach the fuzzy-search input)", got, "R")
	}
}

// TestRWithSortPopupOpenDoesNotOpenAttachmentPicker is review item 5: R
// must not open the attachment picker while the tree's sort popup is
// taking keys, wherever focus happens to be.
func TestRWithSortPopupOpenDoesNotOpenAttachmentPicker(t *testing.T) {
	now := time.Now()
	comment, _ := attachmentFixture(t, "cmt-001", "notes.txt", "text/plain", []byte("hi"), now)
	m, _ := attachmentTestModel(t, []*model.Comment{comment}, false)
	m.tree.OpenSortPopup()

	m, _ = pressBulkKey(t, m, runeKey("R"))

	if m.ShowAttachmentPicker() {
		t.Fatal("R with the sort popup open must not open the attachment picker")
	}
}
