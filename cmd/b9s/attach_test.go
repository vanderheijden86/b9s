package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/vanderheijden86/b9s/internal/attachref"
	"github.com/vanderheijden86/b9s/internal/blobstore"
	"github.com/vanderheijden86/b9s/pkg/config"
	"github.com/vanderheijden86/b9s/pkg/model"
)

// sha256Hex returns content's sha256 as lowercase hex, the same form
// attachref requires in a machine line.
func sha256Hex(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// attachTestProject sets up a bare JSONL project with one issue ("bd-1", no
// comments), a local-backend attachments config, and a fake bd on PATH that
// records every invocation to recordPath. It never touches the shared Dolt
// server: the data source is a plain issues.jsonl file the test controls.
func attachTestProject(t *testing.T, bdExitCode int) (beadsDir, recordPath, cwdPath string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake bd script needs a POSIX shell")
	}

	dir := t.TempDir()
	beadsDir = filepath.Join(dir, ".beads")
	if err := os.Mkdir(beadsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BEADS_DIR", beadsDir)

	issue := model.Issue{ID: "bd-1", Title: "Test issue", Status: "open", IssueType: "task"}
	line, err := json.Marshal(issue)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(beadsDir, "issues.jsonl"), append(line, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}

	xdgConfig := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdgConfig)
	if err := os.MkdirAll(filepath.Join(xdgConfig, "b9s"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfgYAML := "attachments:\n  backend: local\n"
	if err := os.WriteFile(filepath.Join(xdgConfig, "b9s", "config.yaml"), []byte(cfgYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	recordPath = filepath.Join(dir, "bd-calls")
	cwdPath = filepath.Join(dir, "bd-cwd")
	binDir := t.TempDir()
	// Args are separated by RS (\036) within one call and calls by GS (\035),
	// since a reference comment's text itself contains a real newline and so
	// cannot be used as either separator.
	script := "#!/bin/sh\n" +
		"{\n" +
		"  for a in \"$@\"; do printf '%s\\036' \"$a\"; done\n" +
		"  printf '\\035'\n" +
		"} >> " + shellQuote(recordPath) + "\n" +
		"pwd >> " + shellQuote(cwdPath) + "\n" +
		"exit " + strconv.Itoa(bdExitCode) + "\n"
	if err := os.WriteFile(filepath.Join(binDir, "bd"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)

	return beadsDir, recordPath, cwdPath
}

// shellQuote wraps s in single quotes for embedding in a generated sh
// script, escaping any single quote s itself contains.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// readBdCalls parses recordPath into one []string per bd invocation, in
// order.
func readBdCalls(t *testing.T, recordPath string) [][]string {
	t.Helper()
	data, err := os.ReadFile(recordPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	var calls [][]string
	for _, rawCall := range strings.Split(string(data), "\035") {
		if rawCall == "" {
			continue
		}
		args := strings.Split(rawCall, "\036")
		// The script's trailing printf leaves one empty element after the
		// last separator.
		if len(args) > 0 && args[len(args)-1] == "" {
			args = args[:len(args)-1]
		}
		calls = append(calls, args)
	}
	return calls
}

// seedAttachment stores content in the same local blob store the CLI opens
// for this project, and returns the Ref describing it, so a list/get/url
// test can start from an attachment that already exists without going
// through `b9s attach` first.
func seedAttachment(t *testing.T, beadsDir, projectName string, content []byte, name, mimeType string) attachref.Ref {
	t.Helper()
	cfg := &config.AttachmentsConfig{Backend: "local"}
	h, err := blobstore.Open(context.Background(), cfg, blobstore.SourceInfo{
		Kind:        blobstore.SourceJSONL,
		BeadsDir:    beadsDir,
		ProjectName: projectName,
	})
	if err != nil {
		t.Fatal(err)
	}
	ref := attachref.Ref{Size: int64(len(content)), Type: mimeType, Name: name}
	sum := sha256Hex(content)
	ref.SHA256 = sum
	key, err := h.Key(sum)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Store.Put(context.Background(), key, bytes.NewReader(content), ref.Size, ref.Type); err != nil {
		t.Fatal(err)
	}
	return ref
}

// seedComment appends an attach reference comment for ref to bd-1's
// issues.jsonl entry, standing in for what a real `bd comments add` would
// have written.
func seedComment(t *testing.T, beadsDir string, ref attachref.Ref) {
	t.Helper()
	text, err := attachref.Format(ref)
	if err != nil {
		t.Fatal(err)
	}
	issue := model.Issue{
		ID: "bd-1", Title: "Test issue", Status: "open", IssueType: "task",
		Comments: []*model.Comment{{ID: "1", IssueID: "bd-1", Author: "tester", Text: text}},
	}
	line, err := json.Marshal(issue)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(beadsDir, "issues.jsonl"), append(line, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAttachAdd_UploadsFileAndWritesReferenceComment(t *testing.T) {
	beadsDir, recordPath, _ := attachTestProject(t, 0)
	srcDir := t.TempDir()
	path := filepath.Join(srcDir, "notes.txt")
	if err := os.WriteFile(path, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runAttach([]string{"bd-1", path}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	calls := readBdCalls(t, recordPath)
	if len(calls) != 1 || len(calls[0]) != 4 || calls[0][0] != "comments" || calls[0][1] != "add" || calls[0][2] != "bd-1" {
		t.Fatalf("bd calls = %v", calls)
	}
	refs, _ := attachref.Parse(calls[0][3])
	if len(refs) != 1 || refs[0].Name != "notes.txt" {
		t.Fatalf("parsed refs = %+v", refs)
	}

	// The blob must be in the store the CLI itself would open, independent of
	// the JSONL fixture never having been updated with the comment.
	projectName := filepath.Base(filepath.Dir(beadsDir))
	h, err := blobstore.Open(context.Background(), &config.AttachmentsConfig{Backend: "local"}, blobstore.SourceInfo{
		Kind: blobstore.SourceJSONL, BeadsDir: beadsDir, ProjectName: projectName,
	})
	if err != nil {
		t.Fatal(err)
	}
	key, err := h.Key(refs[0].SHA256)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.Store.Stat(context.Background(), key); err != nil {
		t.Errorf("blob missing from store after add: %v", err)
	}
}

func TestAttachAdd_CrashOrder_BlobSurvivesAFailedBdCall(t *testing.T) {
	beadsDir, recordPath, _ := attachTestProject(t, 1)
	srcDir := t.TempDir()
	path := filepath.Join(srcDir, "notes.txt")
	if err := os.WriteFile(path, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runAttach([]string{"bd-1", path}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("exit = 0, want a failure reported when bd exits 1")
	}
	if len(readBdCalls(t, recordPath)) != 1 {
		t.Fatalf("bd calls = %v, want exactly one attempt", readBdCalls(t, recordPath))
	}

	projectName := filepath.Base(filepath.Dir(beadsDir))
	h, err := blobstore.Open(context.Background(), &config.AttachmentsConfig{Backend: "local"}, blobstore.SourceInfo{
		Kind: blobstore.SourceJSONL, BeadsDir: beadsDir, ProjectName: projectName,
	})
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256Hex([]byte("hello world"))
	key, err := h.Key(sum)
	if err != nil {
		t.Fatal(err)
	}
	// Put happens before the bd call that then fails, so the blob survives:
	// a retried add for the same bytes finds it already present.
	if _, err := h.Store.Stat(context.Background(), key); err != nil {
		t.Errorf("blob missing after a failed bd comments add: %v", err)
	}
}

// TestAttachAdd_RejectsFIFO is the CLI-level guard for the regular-file
// check internal/attach.Add now applies (bd-t8j5.10 review): a named pipe
// must be rejected with a clear error, not left to block os.Open forever.
func TestAttachAdd_RejectsFIFO(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("named pipes need syscall.Mkfifo, POSIX-only")
	}
	_, recordPath, _ := attachTestProject(t, 0)
	srcDir := t.TempDir()
	fifoPath := filepath.Join(srcDir, "pipe")
	if err := syscall.Mkfifo(fifoPath, 0o600); err != nil {
		t.Fatalf("Mkfifo: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := runAttach([]string{"bd-1", fifoPath}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("exit = 0, want a failure reported for a FIFO path")
	}
	if !strings.Contains(stderr.String(), "not a regular file") {
		t.Fatalf("stderr = %q, want it to say the path is not a regular file", stderr.String())
	}
	if len(readBdCalls(t, recordPath)) != 0 {
		t.Fatalf("bd calls = %v, want none: a FIFO must never reach bd", readBdCalls(t, recordPath))
	}
}

func TestAttachAdd_MultiFilePartialFailure(t *testing.T) {
	beadsDir, recordPath, _ := attachTestProject(t, 0)
	srcDir := t.TempDir()
	good1 := filepath.Join(srcDir, "good1.txt")
	// attachref rejects a name ending in a dot (Windows cannot use one), so
	// this file fails validation before it ever reaches bd.
	bad := filepath.Join(srcDir, "bad.")
	good2 := filepath.Join(srcDir, "good2.txt")
	for path, content := range map[string]string{good1: "one", bad: "two", good2: "three"} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	var stdout, stderr bytes.Buffer
	code := runAttach([]string{"bd-1", good1, bad, good2}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("exit = 0, want a non-zero exit: one of the three files is invalid")
	}
	if !strings.Contains(stdout.String(), "good1.txt") || !strings.Contains(stdout.String(), "good2.txt") {
		t.Errorf("stdout = %q, want it to report both good files attached", stdout.String())
	}
	if !strings.Contains(stderr.String(), bad) {
		t.Errorf("stderr = %q, want it to report the bad file's failure", stderr.String())
	}
	calls := readBdCalls(t, recordPath)
	if len(calls) != 2 {
		t.Fatalf("bd calls = %v, want one per good file (2), not one per argument", calls)
	}
	_ = beadsDir
}

func TestAttach_BdRunsInProjectDirectory(t *testing.T) {
	beadsDir, _, cwdPath := attachTestProject(t, 0)
	projectDir := filepath.Dir(beadsDir)
	srcDir := t.TempDir()
	path := filepath.Join(srcDir, "notes.txt")
	if err := os.WriteFile(path, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Run from a directory other than the project's, so a cwd match in
	// bd-cwd can only come from b9s explicitly passing projectDir to bdrun,
	// never from inheriting the test process's own working directory.
	t.Chdir(t.TempDir())

	var stdout, stderr bytes.Buffer
	code := runAttach([]string{"bd-1", path}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	got, err := os.ReadFile(cwdPath)
	if err != nil {
		t.Fatal(err)
	}
	if gotDir := strings.TrimSpace(string(got)); gotDir != projectDir {
		t.Errorf("bd ran in %q, want %q (the project directory)", gotDir, projectDir)
	}
}

func TestAttachDetach_WritesADetachComment(t *testing.T) {
	beadsDir, recordPath, _ := attachTestProject(t, 0)
	ref := seedAttachment(t, beadsDir, filepath.Base(filepath.Dir(beadsDir)), []byte("hello world"), "notes.txt", "text/plain")
	seedComment(t, beadsDir, ref)

	var stdout, stderr bytes.Buffer
	code := runAttach([]string{"--detach", "bd-1", ref.SHA256}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	calls := readBdCalls(t, recordPath)
	if len(calls) != 1 || calls[0][0] != "comments" || calls[0][1] != "add" || calls[0][2] != "bd-1" {
		t.Fatalf("bd calls = %v", calls)
	}
	_, detaches := attachref.Parse(calls[0][3])
	if len(detaches) != 1 || detaches[0] != ref.SHA256 {
		t.Errorf("detaches = %v, want [%s]", detaches, ref.SHA256)
	}
}

// TestAttachDetach_FlagRecognizedAfterPositionals covers item 6: --detach
// must be recognized wherever it appears among the arguments, not only
// before the positionals. Before parseFlags was wired into add/detach,
// "--detach" here was consumed as a second file argument to add instead.
func TestAttachDetach_FlagRecognizedAfterPositionals(t *testing.T) {
	beadsDir, recordPath, _ := attachTestProject(t, 0)
	ref := seedAttachment(t, beadsDir, filepath.Base(filepath.Dir(beadsDir)), []byte("hello world"), "notes.txt", "text/plain")
	seedComment(t, beadsDir, ref)

	var stdout, stderr bytes.Buffer
	code := runAttach([]string{"bd-1", ref.SHA256, "--detach"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	calls := readBdCalls(t, recordPath)
	if len(calls) != 1 || calls[0][0] != "comments" || calls[0][1] != "add" || calls[0][2] != "bd-1" {
		t.Fatalf("bd calls = %v", calls)
	}
	_, detaches := attachref.Parse(calls[0][3])
	if len(detaches) != 1 || detaches[0] != ref.SHA256 {
		t.Errorf("detaches = %v, want [%s]: --detach after positionals must not be treated as a file", detaches, ref.SHA256)
	}
}

func TestAttachList_HumanAndJSON(t *testing.T) {
	beadsDir, _, _ := attachTestProject(t, 0)
	ref := seedAttachment(t, beadsDir, filepath.Base(filepath.Dir(beadsDir)), []byte("hello world"), "notes.txt", "text/plain")
	seedComment(t, beadsDir, ref)

	var stdout, stderr bytes.Buffer
	if code := runAttach([]string{"list", "bd-1"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "notes.txt") {
		t.Errorf("list output = %q, want it to mention notes.txt", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := runAttach([]string{"list", "bd-1", "--json"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	var got []struct {
		SHA256 string `json:"sha256"`
		Name   string `json:"name"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal(%q): %v", stdout.String(), err)
	}
	if len(got) != 1 || got[0].SHA256 != ref.SHA256 || got[0].Name != "notes.txt" {
		t.Fatalf("json list = %+v", got)
	}
}

func TestAttachGet_DownloadsAndVerifies(t *testing.T) {
	beadsDir, _, _ := attachTestProject(t, 0)
	ref := seedAttachment(t, beadsDir, filepath.Base(filepath.Dir(beadsDir)), []byte("hello world"), "notes.txt", "text/plain")
	seedComment(t, beadsDir, ref)

	destDir := t.TempDir()
	destPath := filepath.Join(destDir, "downloaded.txt")

	var stdout, stderr bytes.Buffer
	code := runAttach([]string{"get", "bd-1", ref.SHA256, "-o", destPath}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	got, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello world" {
		t.Errorf("downloaded content = %q, want %q", got, "hello world")
	}
}

func TestAttachGet_ByHashPrefix(t *testing.T) {
	beadsDir, _, _ := attachTestProject(t, 0)
	ref := seedAttachment(t, beadsDir, filepath.Base(filepath.Dir(beadsDir)), []byte("hello world"), "notes.txt", "text/plain")
	seedComment(t, beadsDir, ref)

	destDir := t.TempDir()
	destPath := filepath.Join(destDir, "downloaded.txt")

	var stdout, stderr bytes.Buffer
	code := runAttach([]string{"get", "bd-1", ref.SHA256[:12], "-o", destPath}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	got, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello world" {
		t.Errorf("downloaded content = %q, want %q", got, "hello world")
	}
}

func TestAttachGet_DefaultOutputPathWritesAttachmentName(t *testing.T) {
	beadsDir, _, _ := attachTestProject(t, 0)
	ref := seedAttachment(t, beadsDir, filepath.Base(filepath.Dir(beadsDir)), []byte("hello world"), "notes.txt", "text/plain")
	seedComment(t, beadsDir, ref)

	t.Chdir(t.TempDir())

	var stdout, stderr bytes.Buffer
	code := runAttach([]string{"get", "bd-1", ref.SHA256}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	got, err := os.ReadFile("notes.txt")
	if err != nil {
		t.Fatalf("expected notes.txt in the current directory: %v", err)
	}
	if string(got) != "hello world" {
		t.Errorf("content = %q, want %q", got, "hello world")
	}
}

func TestAttachGet_ByNamePrefixAndStdout(t *testing.T) {
	beadsDir, _, _ := attachTestProject(t, 0)
	ref := seedAttachment(t, beadsDir, filepath.Base(filepath.Dir(beadsDir)), []byte("hello world"), "notes.txt", "text/plain")
	seedComment(t, beadsDir, ref)

	var stdout, stderr bytes.Buffer
	code := runAttach([]string{"get", "bd-1", "notes.txt", "-o", "-"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	if stdout.String() != "hello world" {
		t.Errorf("stdout = %q, want %q", stdout.String(), "hello world")
	}
}

func TestAttachGet_RefusesToOverwriteExistingFile(t *testing.T) {
	beadsDir, _, _ := attachTestProject(t, 0)
	ref := seedAttachment(t, beadsDir, filepath.Base(filepath.Dir(beadsDir)), []byte("hello world"), "notes.txt", "text/plain")
	seedComment(t, beadsDir, ref)

	destDir := t.TempDir()
	destPath := filepath.Join(destDir, "downloaded.txt")
	if err := os.WriteFile(destPath, []byte("original content"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runAttach([]string{"get", "bd-1", ref.SHA256, "-o", destPath}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("exit = 0, want a non-zero exit: get must refuse to overwrite an existing file")
	}
	if stderr.String() == "" {
		t.Error("stderr is empty, want a clear message about the existing file")
	}
	got, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "original content" {
		t.Errorf("destPath content = %q, want it unchanged", got)
	}
}

func TestAttachGet_NeverFollowsSymlinkAtDestination(t *testing.T) {
	beadsDir, _, _ := attachTestProject(t, 0)
	ref := seedAttachment(t, beadsDir, filepath.Base(filepath.Dir(beadsDir)), []byte("hello world"), "notes.txt", "text/plain")
	seedComment(t, beadsDir, ref)

	destDir := t.TempDir()
	realTarget := filepath.Join(destDir, "real-target.txt")
	if err := os.WriteFile(realTarget, []byte("original content"), 0o644); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(destDir, "link.txt")
	if err := os.Symlink(realTarget, linkPath); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := runAttach([]string{"get", "bd-1", ref.SHA256, "-o", linkPath}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("exit = 0, want a non-zero exit: get must refuse to overwrite through a symlink")
	}
	got, err := os.ReadFile(realTarget)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "original content" {
		t.Errorf("symlink target content = %q, want it unchanged", got)
	}
}

func TestAttachGet_ForceReplacesExistingFile(t *testing.T) {
	beadsDir, _, _ := attachTestProject(t, 0)
	ref := seedAttachment(t, beadsDir, filepath.Base(filepath.Dir(beadsDir)), []byte("hello world"), "notes.txt", "text/plain")
	seedComment(t, beadsDir, ref)

	destDir := t.TempDir()
	destPath := filepath.Join(destDir, "downloaded.txt")
	if err := os.WriteFile(destPath, []byte("stale content"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runAttach([]string{"get", "bd-1", ref.SHA256, "-o", destPath, "--force"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	got, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello world" {
		t.Errorf("destPath content = %q, want %q", got, "hello world")
	}
}

func TestAttachGet_DefaultOutputPathAlsoRefusesOverwrite(t *testing.T) {
	beadsDir, _, _ := attachTestProject(t, 0)
	ref := seedAttachment(t, beadsDir, filepath.Base(filepath.Dir(beadsDir)), []byte("hello world"), "notes.txt", "text/plain")
	seedComment(t, beadsDir, ref)

	workDir := t.TempDir()
	t.Chdir(workDir)
	if err := os.WriteFile("notes.txt", []byte("original content"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runAttach([]string{"get", "bd-1", ref.SHA256}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("exit = 0, want a non-zero exit: the default output path must refuse to overwrite too")
	}
	got, err := os.ReadFile(filepath.Join(workDir, "notes.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "original content" {
		t.Errorf("notes.txt content = %q, want it unchanged", got)
	}
}

func TestAttachGet_ODirectoryWritesSafeNameInside(t *testing.T) {
	beadsDir, _, _ := attachTestProject(t, 0)
	ref := seedAttachment(t, beadsDir, filepath.Base(filepath.Dir(beadsDir)), []byte("hello world"), "notes.txt", "text/plain")
	seedComment(t, beadsDir, ref)

	destDir := t.TempDir()

	var stdout, stderr bytes.Buffer
	code := runAttach([]string{"get", "bd-1", ref.SHA256, "-o", destDir}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	got, err := os.ReadFile(filepath.Join(destDir, "notes.txt"))
	if err != nil {
		t.Fatalf("expected notes.txt inside %s: %v", destDir, err)
	}
	if string(got) != "hello world" {
		t.Errorf("content = %q, want %q", got, "hello world")
	}
}

func TestAttachGet_WritesWithNormalCreatePermissions(t *testing.T) {
	beadsDir, _, _ := attachTestProject(t, 0)
	ref := seedAttachment(t, beadsDir, filepath.Base(filepath.Dir(beadsDir)), []byte("hello world"), "notes.txt", "text/plain")
	seedComment(t, beadsDir, ref)

	destDir := t.TempDir()
	destPath := filepath.Join(destDir, "downloaded.txt")

	var stdout, stderr bytes.Buffer
	code := runAttach([]string{"get", "bd-1", ref.SHA256, "-o", destPath}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	got, err := os.Stat(destPath)
	if err != nil {
		t.Fatal(err)
	}
	// A plain 0666 create in the same directory, masked by the same umask
	// and any default ACL, is what "the permissions of a normal create"
	// means; this reference file shows the value independent of any
	// production code that might compute (or fail to compute) it.
	refPath := filepath.Join(destDir, "reference")
	refFile, err := os.OpenFile(refPath, os.O_CREATE|os.O_WRONLY, 0o666)
	if err != nil {
		t.Fatal(err)
	}
	refFile.Close()
	want, err := os.Stat(refPath)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := got.Mode().Perm(), want.Mode().Perm(); got != want {
		t.Errorf("mode = %o, want %o (a plain 0666 create masked by umask, not os.CreateTemp's 0600)", got, want)
	}
}

// TestAttachGet_TempRemoveFailureAfterLinkDoesNotFailTheCommand covers item
// 4: a Link that already placed the bytes at destPath must not be reported
// as a command failure just because the follow-up cleanup of tempPath could
// not remove it. tempPath is put in a directory with its write bit removed,
// so unlink(2) fails on it while link(2) (which does not modify tempPath's
// directory) still succeeds.
func TestAttachGet_TempRemoveFailureAfterLinkDoesNotFailTheCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory permission bits don't block unlink the same way on windows")
	}

	tempDir := t.TempDir()
	tempPath := filepath.Join(tempDir, "temp-download")
	if err := os.WriteFile(tempPath, []byte("hello world"), 0o600); err != nil {
		t.Fatal(err)
	}
	destDir := t.TempDir()
	destPath := filepath.Join(destDir, "notes.txt")

	if err := os.Chmod(tempDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(tempDir, 0o755) })

	if err := placeByExclusiveCreate(tempPath, destPath); err != nil {
		t.Fatalf("placeByExclusiveCreate: %v, want nil: a Link that succeeded must not fail on a later Remove error", err)
	}
	got, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello world" {
		t.Errorf("destPath content = %q, want %q", got, "hello world")
	}
}

func TestAttachURL_RefusedForLocalBackend(t *testing.T) {
	beadsDir, _, _ := attachTestProject(t, 0)
	ref := seedAttachment(t, beadsDir, filepath.Base(filepath.Dir(beadsDir)), []byte("hello world"), "notes.txt", "text/plain")
	seedComment(t, beadsDir, ref)

	var stdout, stderr bytes.Buffer
	code := runAttach([]string{"url", "bd-1", ref.SHA256}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("exit = 0, want the local backend refused")
	}
	if strings.Contains(stderr.String(), "file://") {
		t.Errorf("stderr = %q, must not suggest a file:// link", stderr.String())
	}
}

// TestAttachURL_DoubleDashEndsFlagScanning covers item 6: url must go
// through parseFlags too, so "--" ends flag scanning there exactly as it
// does for the other subcommands. Before this, url counted args directly
// and required exactly two, so "--" itself consumed one of the two slots.
func TestAttachURL_DoubleDashEndsFlagScanning(t *testing.T) {
	beadsDir, _, _ := attachTestProject(t, 0)
	ref := seedAttachment(t, beadsDir, filepath.Base(filepath.Dir(beadsDir)), []byte("hello world"), "notes.txt", "text/plain")
	seedComment(t, beadsDir, ref)

	var stdout, stderr bytes.Buffer
	code := runAttach([]string{"url", "--", "bd-1", ref.SHA256}, &stdout, &stderr)

	if code == 2 {
		t.Fatalf("exit = 2 (usage/argument error), stderr %q: -- must end flag scanning, leaving exactly two positionals", stderr.String())
	}
}

func TestAttachNotConfigured_ExitsWithGuidance(t *testing.T) {
	beadsDir, _, _ := attachTestProject(t, 0)
	// Overwrite the seeded config with an empty one: no attachments section.
	xdgConfig := os.Getenv("XDG_CONFIG_HOME")
	if err := os.WriteFile(filepath.Join(xdgConfig, "b9s", "config.yaml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = beadsDir

	var stdout, stderr bytes.Buffer
	code := runAttach([]string{"list", "bd-1"}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "attachments") {
		t.Errorf("stderr = %q, want it to mention attachments configuration", stderr.String())
	}
}

func TestAttachNotConfigured_NeverResolvesBdOrProject(t *testing.T) {
	// No bd on PATH, no .beads directory, and no attachments config: the
	// missing-configuration check must win before any of the three is ever
	// touched, so this must exit 2 with guidance rather than fail on a
	// missing bd binary or a missing project.
	t.Setenv("BEADS_DIR", filepath.Join(t.TempDir(), "does-not-exist"))
	t.Setenv("PATH", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	var stdout, stderr bytes.Buffer
	code := runAttach([]string{"list", "bd-1"}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("exit = %d, stderr %q, want 2", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "attachments") {
		t.Errorf("stderr = %q, want it to mention attachments configuration", stderr.String())
	}
}

func TestParseFlags_InlineEqualsSyntax(t *testing.T) {
	values, _, rest, err := parseFlags([]string{"get", "bd-1", "-o=out.txt"}, nil, []string{"-o"})

	if err != nil {
		t.Fatalf("parseFlags: %v", err)
	}
	if values["-o"] != "out.txt" {
		t.Errorf("values[-o] = %q, want out.txt", values["-o"])
	}
	if want := []string{"get", "bd-1"}; !slicesEqual(rest, want) {
		t.Errorf("rest = %v, want %v", rest, want)
	}
}

func TestParseFlags_DoubleDashEndsFlagScanning(t *testing.T) {
	_, _, rest, err := parseFlags([]string{"bd-1", "--", "--json"}, []string{"--json"}, nil)

	if err != nil {
		t.Fatalf("parseFlags: %v", err)
	}
	if want := []string{"bd-1", "--json"}; !slicesEqual(rest, want) {
		t.Errorf("rest = %v, want %v: everything after -- is positional", rest, want)
	}
}

func TestParseFlags_RepeatedFlagIsAnError(t *testing.T) {
	_, _, _, err := parseFlags([]string{"-o", "a.txt", "-o", "b.txt"}, nil, []string{"-o"})

	if err == nil {
		t.Fatal("parseFlags: err = nil, want an error for a repeated flag")
	}
}

func TestParseFlags_ValueLookingLikeAFlagIsAnError(t *testing.T) {
	_, _, _, err := parseFlags([]string{"-o", "--force"}, []string{"--force"}, []string{"-o"})

	if err == nil {
		t.Fatal("parseFlags: err = nil, want an error: -o's value looks like another flag")
	}
}

func TestParseFlags_DashAloneIsAllowedAsOValue(t *testing.T) {
	values, _, rest, err := parseFlags([]string{"get", "bd-1", "-o", "-"}, nil, []string{"-o"})

	if err != nil {
		t.Fatalf("parseFlags: %v", err)
	}
	if values["-o"] != "-" {
		t.Errorf("values[-o] = %q, want \"-\"", values["-o"])
	}
	if want := []string{"get", "bd-1"}; !slicesEqual(rest, want) {
		t.Errorf("rest = %v, want %v", rest, want)
	}
}

func TestParseFlags_UnknownFlagIsAnErrorNotAPositional(t *testing.T) {
	_, _, _, err := parseFlags([]string{"bd-1", "--bogus"}, []string{"--json"}, nil)

	if err == nil {
		t.Fatal("parseFlags: err = nil, want an error for an unknown flag")
	}
}

func TestParseFlags_EmptyInlineValueIsAnError(t *testing.T) {
	_, _, _, err := parseFlags([]string{"-o="}, nil, []string{"-o"})

	if err == nil {
		t.Fatal("parseFlags: err = nil, want an error for -o= with an empty value")
	}
}

func TestParseFlags_BoolFlagAfterPositionals(t *testing.T) {
	_, bools, rest, err := parseFlags([]string{"bd-1", "--json"}, []string{"--json"}, nil)

	if err != nil {
		t.Fatalf("parseFlags: %v", err)
	}
	if !bools["--json"] {
		t.Errorf("bools[--json] = false, want true")
	}
	if want := []string{"bd-1"}; !slicesEqual(rest, want) {
		t.Errorf("rest = %v, want %v", rest, want)
	}
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestAttachUsage_NoArgsPrintsUsageAndExits2(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runAttach(nil, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "usage: b9s attach") {
		t.Errorf("stderr = %q, want usage text", stderr.String())
	}
}
