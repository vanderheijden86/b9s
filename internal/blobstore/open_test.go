package blobstore

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/vanderheijden86/beadwork/pkg/config"
)

func TestOpen_NilConfigIsNotConfigured(t *testing.T) {
	_, err := Open(context.Background(), nil, SourceInfo{})
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("Open(nil, ...) err = %v, want ErrNotConfigured", err)
	}
}

func TestOpen_LocalDefaultsToBeadsDirAttachments(t *testing.T) {
	beadsDir := filepath.Join(t.TempDir(), ".beads")
	cfg := &config.AttachmentsConfig{Backend: "local"}
	h, err := Open(context.Background(), cfg, SourceInfo{
		Kind:        SourceJSONL,
		BeadsDir:    beadsDir,
		ProjectName: "b9s",
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if h.MaxBytes != config.DefaultAttachmentMaxBytes {
		t.Errorf("MaxBytes = %d, want %d", h.MaxBytes, config.DefaultAttachmentMaxBytes)
	}
	if h.GCGrace != config.DefaultAttachmentGCGrace {
		t.Errorf("GCGrace = %s, want %s", h.GCGrace, config.DefaultAttachmentGCGrace)
	}
	if h.URLTTL != config.DefaultAttachmentURLTTL {
		t.Errorf("URLTTL = %s, want %s", h.URLTTL, config.DefaultAttachmentURLTTL)
	}
	local, ok := h.Store.(*Local)
	if !ok {
		t.Fatalf("Store = %T, want *Local", h.Store)
	}
	if local.root != filepath.Join(beadsDir, "attachments") {
		t.Errorf("local.root = %q, want %q", local.root, filepath.Join(beadsDir, "attachments"))
	}
}

func TestOpen_LocalHonorsExplicitDir(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.AttachmentsConfig{Backend: "local", LocalDir: dir}
	h, err := Open(context.Background(), cfg, SourceInfo{Kind: SourceSQLite, ProjectName: "b9s"})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	local := h.Store.(*Local)
	if local.root != filepath.Clean(dir) {
		t.Errorf("local.root = %q, want %q", local.root, dir)
	}
}

func TestOpen_LocalWithoutDirOrBeadsDirFails(t *testing.T) {
	cfg := &config.AttachmentsConfig{Backend: "local"}
	_, err := Open(context.Background(), cfg, SourceInfo{Kind: SourceSQLite, ProjectName: "b9s"})
	if err == nil {
		t.Fatal("Open succeeded, want an error: no local_dir and no beads dir")
	}
}

func TestOpen_LocalRefusedForDoltByDefault(t *testing.T) {
	cfg := &config.AttachmentsConfig{Backend: "local", LocalDir: t.TempDir()}
	// A Dolt server reached through an SSH tunnel looks like loopback from
	// here even when other operators share it, so this must be refused
	// regardless of host: only the explicit opt-in makes it safe.
	for _, host := range []string{"203.0.113.5", "127.0.0.1", "localhost", "::1"} {
		t.Run(host, func(t *testing.T) {
			_, err := Open(context.Background(), cfg, SourceInfo{
				Kind:     SourceDolt,
				Database: "b9s",
			})
			if err == nil || !strings.Contains(err.Error(), "local_with_dolt_server") {
				t.Fatalf("Open err = %v, want an error naming local_with_dolt_server", err)
			}
		})
	}
}

func TestOpen_LocalAllowedForDoltWithExplicitOptIn(t *testing.T) {
	cfg := &config.AttachmentsConfig{Backend: "local", LocalDir: t.TempDir(), LocalWithDoltServer: true}
	for _, host := range []string{"203.0.113.5", "127.0.0.1"} {
		t.Run(host, func(t *testing.T) {
			_, err := Open(context.Background(), cfg, SourceInfo{
				Kind:     SourceDolt,
				Database: "b9s",
			})
			if err != nil {
				t.Fatalf("Open: %v", err)
			}
		})
	}
}

func TestOpen_LocalAllowedForSQLiteAndJSONL(t *testing.T) {
	for _, kind := range []SourceKind{SourceSQLite, SourceJSONL} {
		t.Run(string(kind), func(t *testing.T) {
			cfg := &config.AttachmentsConfig{Backend: "local", LocalDir: t.TempDir()}
			if _, err := Open(context.Background(), cfg, SourceInfo{Kind: kind, ProjectName: "b9s"}); err != nil {
				t.Fatalf("Open: %v", err)
			}
		})
	}
}

func TestOpen_KeyUsesPrefixAndDatabaseSegment(t *testing.T) {
	const hash = "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"

	t.Run("dolt uses database name, default prefix falls back to it", func(t *testing.T) {
		cfg := &config.AttachmentsConfig{Backend: "local", LocalDir: t.TempDir(), LocalWithDoltServer: true}
		h, err := Open(context.Background(), cfg, SourceInfo{Kind: SourceDolt, Database: "b9s"})
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		got, err := h.Key(hash)
		if err != nil {
			t.Fatal(err)
		}
		want := "b9s/b9s/sha256/9f/" + hash
		if got != want {
			t.Errorf("Key() = %q, want %q", got, want)
		}
	})

	t.Run("explicit prefix wins over the default", func(t *testing.T) {
		cfg := &config.AttachmentsConfig{Backend: "local", LocalDir: t.TempDir(), LocalWithDoltServer: true}
		cfg.Prefix = "osenco"
		h, err := Open(context.Background(), cfg, SourceInfo{Kind: SourceDolt, Database: "b9s"})
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		got, err := h.Key(hash)
		if err != nil {
			t.Fatal(err)
		}
		want := "osenco/b9s/sha256/9f/" + hash
		if got != want {
			t.Errorf("Key() = %q, want %q", got, want)
		}
	})

	t.Run("sqlite and jsonl use the project name as the database segment", func(t *testing.T) {
		cfg := &config.AttachmentsConfig{Backend: "local", LocalDir: t.TempDir()}
		h, err := Open(context.Background(), cfg, SourceInfo{Kind: SourceSQLite, ProjectName: "b9s"})
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		got, err := h.Key(hash)
		if err != nil {
			t.Fatal(err)
		}
		want := "b9s/b9s/sha256/9f/" + hash
		if got != want {
			t.Errorf("Key() = %q, want %q", got, want)
		}
	})
}

func TestOpen_ValidatesKeyEarly(t *testing.T) {
	// A prefix segment containing a slash-unsafe character (here a space)
	// would only surface as a Key error on the first real attach; Open must
	// catch it immediately and name the bad segment.
	cfg := &config.AttachmentsConfig{Backend: "local", LocalDir: t.TempDir(), Prefix: "bad prefix"}
	_, err := Open(context.Background(), cfg, SourceInfo{Kind: SourceSQLite, ProjectName: "b9s"})
	if err == nil || !strings.Contains(err.Error(), "bad prefix") {
		t.Fatalf("Open err = %v, want an error naming the bad prefix segment", err)
	}
}

func TestHandle_ListPrefix(t *testing.T) {
	cfg := &config.AttachmentsConfig{Backend: "local", LocalDir: t.TempDir(), LocalWithDoltServer: true}
	h, err := Open(context.Background(), cfg, SourceInfo{Kind: SourceDolt, Database: "b9s"})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	want := "b9s/b9s/sha256"
	if got := h.ListPrefix(); got != want {
		t.Errorf("ListPrefix() = %q, want %q", got, want)
	}
}

func TestHandle_ListPrefixMatchesKeyForEveryPrefixSpelling(t *testing.T) {
	const hash = "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
	cases := []struct {
		name   string
		prefix string
	}{
		{"empty", ""},
		{"bare", "x"},
		{"trailing slash", "x/"},
		{"leading and trailing slash", "/x/"},
		{"two segments with trailing slash", "a/b/"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := &config.AttachmentsConfig{Backend: "local", LocalDir: t.TempDir(), Prefix: c.prefix}
			h, err := Open(context.Background(), cfg, SourceInfo{Kind: SourceSQLite, ProjectName: "b9s"})
			if err != nil {
				t.Fatalf("Open: %v", err)
			}
			key, err := h.Key(hash)
			if err != nil {
				t.Fatalf("Key: %v", err)
			}
			if err := h.Store.Put(context.Background(), key, strings.NewReader("data"), 4, "text/plain"); err != nil {
				t.Fatalf("Put: %v", err)
			}
			var got []string
			err = h.Store.List(context.Background(), h.ListPrefix(), func(info Info) error {
				got = append(got, info.Key)
				return nil
			})
			if err != nil {
				t.Fatalf("List(%q): %v", h.ListPrefix(), err)
			}
			if len(got) != 1 || got[0] != key {
				t.Fatalf("List(%q) = %v, want exactly [%q]", h.ListPrefix(), got, key)
			}
		})
	}
}

func TestOpen_UnknownBackend(t *testing.T) {
	// AttachmentsConfig.UnmarshalYAML rejects this before it reaches Open,
	// but a hand-built Config (as any caller not going through YAML would
	// have) must still be refused rather than silently picking a backend.
	cfg := &config.AttachmentsConfig{Backend: "ftp"}
	_, err := Open(context.Background(), cfg, SourceInfo{Kind: SourceSQLite, ProjectName: "b9s"})
	if err == nil || !strings.Contains(err.Error(), "ftp") {
		t.Fatalf("Open err = %v, want an error naming the unknown backend", err)
	}
}

func TestResolveS3Credentials_FromEnvironment(t *testing.T) {
	t.Setenv(envAccessKeyID, "AKIAEXAMPLE")
	t.Setenv(envSecretAccessKey, "shh-secret")
	id, secret, err := resolveS3Credentials(context.Background(), config.S3AttachmentsConfig{
		CredentialCommand: fakeCredentialScript(t, "should-not-run", "should-not-run"),
	})
	if err != nil {
		t.Fatalf("resolveS3Credentials: %v", err)
	}
	if id != "AKIAEXAMPLE" || secret != "shh-secret" {
		t.Fatalf("got (%q, %q), want env values (credential_command must not have run)", id, secret)
	}
}

func TestResolveS3Credentials_OnlyOneEnvVarSetIsAnError(t *testing.T) {
	t.Setenv(envAccessKeyID, "AKIAEXAMPLE")
	_, _, err := resolveS3Credentials(context.Background(), config.S3AttachmentsConfig{})
	if err == nil {
		t.Fatal("resolveS3Credentials succeeded with only one env var set")
	}
}

func TestResolveS3Credentials_MissingBothIsAnError(t *testing.T) {
	_, _, err := resolveS3Credentials(context.Background(), config.S3AttachmentsConfig{})
	if err == nil || !strings.Contains(err.Error(), envAccessKeyID) || !strings.Contains(err.Error(), envSecretAccessKey) {
		t.Fatalf("resolveS3Credentials err = %v, want it to name both env vars", err)
	}
}

func TestResolveS3Credentials_CredentialCommandSuccess(t *testing.T) {
	cfg := config.S3AttachmentsConfig{CredentialCommand: fakeCredentialScript(t, "AKIAFROMCOMMAND", "command-secret")}
	id, secret, err := resolveS3Credentials(context.Background(), cfg)
	if err != nil {
		t.Fatalf("resolveS3Credentials: %v", err)
	}
	if id != "AKIAFROMCOMMAND" || secret != "command-secret" {
		t.Fatalf("got (%q, %q)", id, secret)
	}
}

func TestResolveS3Credentials_CredentialCommandTrimsCRLFAndSpaces(t *testing.T) {
	script := writeScript(t, "#!/bin/sh\nprintf 'AKIAEXAMPLE \\r\\n  command-secret  \\r\\n'\n")
	id, secret, err := resolveS3Credentials(context.Background(), config.S3AttachmentsConfig{CredentialCommand: script})
	if err != nil {
		t.Fatalf("resolveS3Credentials: %v", err)
	}
	if id != "AKIAEXAMPLE" || secret != "command-secret" {
		t.Fatalf("got (%q, %q), want trimmed values with no CR or spaces", id, secret)
	}
}

func TestResolveS3Credentials_CredentialCommandWrongLineCount(t *testing.T) {
	script := writeScript(t, "#!/bin/sh\necho only-one-line\n")
	_, _, err := resolveS3Credentials(context.Background(), config.S3AttachmentsConfig{CredentialCommand: script})
	if err == nil || !strings.Contains(err.Error(), "two non-empty lines") {
		t.Fatalf("err = %v, want a complaint about the line count", err)
	}
}

func TestResolveS3Credentials_CredentialCommandNonZeroExit(t *testing.T) {
	script := writeScript(t, "#!/bin/sh\necho leaked-secret-in-stderr >&2\nexit 1\n")
	_, _, err := resolveS3Credentials(context.Background(), config.S3AttachmentsConfig{CredentialCommand: script})
	if err == nil {
		t.Fatal("resolveS3Credentials succeeded, want the exit-1 script to fail it")
	}
	if strings.Contains(err.Error(), "leaked-secret-in-stderr") {
		t.Fatalf("err = %v, stderr must never be echoed into the error", err)
	}
}

func TestResolveS3Credentials_CredentialCommandTimeout(t *testing.T) {
	old := credentialCommandTimeout
	credentialCommandTimeout = 50 * time.Millisecond
	defer func() { credentialCommandTimeout = old }()

	script := writeScript(t, "#!/bin/sh\nsleep 5\necho id\necho secret\n")
	start := time.Now()
	_, _, err := resolveS3Credentials(context.Background(), config.S3AttachmentsConfig{CredentialCommand: script})
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("err = %v, want a timeout error", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("resolveS3Credentials took %s, want it bounded by the shortened timeout", elapsed)
	}
}

func TestResolveS3Credentials_CredentialCommandKillsOrphanedGrandchild(t *testing.T) {
	old := credentialCommandTimeout
	credentialCommandTimeout = 500 * time.Millisecond
	defer func() { credentialCommandTimeout = old }()

	// The backgrounded "sleep 30" inherits the stdout pipe. A context that
	// only kills the "sh" process (not its process group) leaves that
	// grandchild holding the pipe open, so exec.Cmd.Wait blocks until it
	// exits on its own: this is the hang the fix must close.
	script := writeScript(t, "#!/bin/sh\nsleep 30 &\nwait\n")
	start := time.Now()
	_, _, err := resolveS3Credentials(context.Background(), config.S3AttachmentsConfig{CredentialCommand: script})
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("err = %v, want a timeout error", err)
	}
	if elapsed := time.Since(start); elapsed > credentialCommandTimeout+1500*time.Millisecond {
		t.Fatalf("resolveS3Credentials took %s, want it bounded by timeout + ~1.5s", elapsed)
	}
}

func TestResolveS3Credentials_CredentialCommandKillsSleepThenEcho(t *testing.T) {
	old := credentialCommandTimeout
	credentialCommandTimeout = 500 * time.Millisecond
	defer func() { credentialCommandTimeout = old }()

	script := writeScript(t, "#!/bin/sh\nsleep 30\necho x\n")
	start := time.Now()
	_, _, err := resolveS3Credentials(context.Background(), config.S3AttachmentsConfig{CredentialCommand: script})
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("err = %v, want a timeout error", err)
	}
	if elapsed := time.Since(start); elapsed > credentialCommandTimeout+1500*time.Millisecond {
		t.Fatalf("resolveS3Credentials took %s, want it bounded by timeout + ~1.5s", elapsed)
	}
}

// fakeCredentialScript writes an executable script printing id and secret on
// their own lines and returns its path, for use as a CredentialCommand.
func fakeCredentialScript(t *testing.T, id, secret string) string {
	t.Helper()
	return writeScript(t, "#!/bin/sh\necho "+id+"\necho "+secret+"\n")
}

func writeScript(t *testing.T, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("credential_command runs through sh, not available on windows")
	}
	path := filepath.Join(t.TempDir(), "credential-command.sh")
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}
