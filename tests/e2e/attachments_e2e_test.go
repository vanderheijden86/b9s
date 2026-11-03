package main_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vanderheijden86/b9s/internal/attachref"
)

func TestAttachmentsStockBDLocalBackendE2E(t *testing.T) {
	skipIfNoScript(t)
	bd := os.Getenv("B9S_TEST_STOCK_BD")
	if bd == "" {
		var err error
		bd, err = exec.LookPath("bd")
		if err != nil {
			t.Skip("stock bd is not installed")
		}
	}
	bd, err := filepath.Abs(bd)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	bin := filepath.Join(dir, "bin")
	cfgDir := filepath.Join(home, ".config", "b9s")
	for _, path := range []string{bin, cfgDir} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(bd, filepath.Join(bin, "bd")); err != nil {
		t.Fatal(err)
	}
	// No inherited connection, routing or b9s setting may direct this test
	// away from its private embedded project and local blob directory.
	var env []string
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(name, "BEADS_") || strings.HasPrefix(name, "BD_") ||
			strings.HasPrefix(name, "B9S_") || strings.HasPrefix(name, "OMNIGENT_BEADS_") {
			env = append(env, name+"=")
		}
	}
	env = append(env,
		"HOME="+home, "PATH="+bin+":/usr/bin:/bin",
		"XDG_CONFIG_HOME="+filepath.Join(home, ".config"),
		"XDG_CACHE_HOME="+filepath.Join(home, ".cache"),
		"XDG_DATA_HOME="+filepath.Join(home, ".local", "share"),
		"XDG_STATE_HOME="+filepath.Join(home, ".local", "state"),
		"OMNIGENT_BEADS_CONNECTION="+filepath.Join(dir, "no-lane-connection"),
		"BEADS_DOLT_AUTO_START=0", "BD_NON_INTERACTIVE=1",
		"BEADS_ACTOR=attachment-e2e", "BD_ACTOR=attachment-e2e",
		"B9S_TEST_MODE=1", "B9S_NO_BROWSER=1",
		"GIT_AUTHOR_NAME=e2e", "GIT_AUTHOR_EMAIL=e2e@example.invalid",
		"GIT_COMMITTER_NAME=e2e", "GIT_COMMITTER_EMAIL=e2e@example.invalid",
	)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	run := func(binary string, args ...string) []byte {
		t.Helper()
		commandCtx, stop := context.WithTimeout(ctx, 30*time.Second)
		defer stop()
		cmd := exec.CommandContext(commandCtx, binary, args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), env...)
		cmd.WaitDelay = time.Second
		out, err := cmd.CombinedOutput()
		if err != nil || commandCtx.Err() != nil {
			t.Fatalf("%s %v: %v (deadline: %v)\n%s", binary, args, err, commandCtx.Err(), out)
		}
		return out
	}
	t.Logf("stock bd: %s", strings.TrimSpace(string(run(bd, "version"))))
	run("git", "init", "-q")
	run(bd, "init", "--prefix", "att", "--quiet", "--skip-hooks")
	meta, err := os.ReadFile(filepath.Join(dir, ".beads", "metadata.json"))
	if err != nil {
		t.Fatal(err)
	}
	var metadata struct {
		Mode string `json:"dolt_mode"`
	}
	if err := json.Unmarshal(meta, &metadata); err != nil || metadata.Mode != "embedded" {
		t.Fatalf("refusing writes: bd did not create an embedded project: %s (%v)", meta, err)
	}
	env = append(env, "BEADS_DIR="+filepath.Join(dir, ".beads"))
	issueID := strings.TrimSpace(string(run(bd, "create", "Attachment PTY issue", "--silent")))
	if !strings.HasPrefix(issueID, "att-") || strings.ContainsAny(issueID, " \n\t") {
		t.Fatalf("unexpected created issue ID: %q", issueID)
	}
	settings := "attachments:\n  backend: local\n  prefix: pty\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(settings), 0o600); err != nil {
		t.Fatal(err)
	}
	name := "terminal-note.txt"
	content := []byte("Attachment from a real terminal and stock bd.\n")
	if err := os.WriteFile(filepath.Join(dir, name), content, 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := runTreeTUIWithEnv(t, dir, 6500, []keyStep{
		kd("I", 200*time.Millisecond),
		kd(name, 200*time.Millisecond),
		kd("\x13", 200*time.Millisecond),
		kd("d", 1200*time.Millisecond),
		kd("R", 300*time.Millisecond),
		kd("\r", 300*time.Millisecond),
	}, env...)
	if err != nil {
		t.Fatalf("attachment PTY: %v\n%s", err, truncateOutput(string(out), 6000))
	}
	screen := stripTerminalCodes(out)
	for _, want := range []string{"Attach Files", "attached", "Attachments", name, "test mode: opener not run"} {
		if !strings.Contains(screen, want) {
			t.Fatalf("PTY lacks %q:\n%s", want, truncateOutput(screen, 9000))
		}
	}
	export := run(bd, "export")
	// JSONL escapes the comment's newlines, so read its text through JSON.
	var issue struct {
		Comments []struct {
			Text string `json:"text"`
		} `json:"comments"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(export), &issue); err != nil {
		t.Fatalf("stock bd export: %v\n%s", err, export)
	}
	var refs []attachref.Ref
	for _, comment := range issue.Comments {
		parsed, _ := attachref.Parse(comment.Text)
		refs = append(refs, parsed...)
	}
	sum := sha256.Sum256(content)
	hash := hex.EncodeToString(sum[:])
	if len(refs) != 1 || refs[0].Name != name || refs[0].SHA256 != hash || refs[0].Size != int64(len(content)) {
		t.Fatalf("stock bd did not persist the attachment reference: %+v", refs)
	}
	list := run(buildBvBinary(t), "attach", "list", issueID, "--json")
	var listed []attachref.Attachment
	if err := json.Unmarshal(list, &listed); err != nil || len(listed) != 1 || listed[0].SHA256 != hash {
		t.Fatalf("attachment list: %s (%v)", list, err)
	}
	download := filepath.Join(dir, "download.txt")
	run(buildBvBinary(t), "attach", "get", issueID, name, "-o", download)
	got, err := os.ReadFile(download)
	if err != nil || !bytes.Equal(got, content) {
		t.Fatalf("download does not match original bytes: %q (%v)", got, err)
	}
	t.Logf("PTY attached, listed and opened %s. Stock bd persisted one SHA256 reference %s. Download bytes match.", name, hash)
}
