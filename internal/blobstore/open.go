package blobstore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/vanderheijden86/b9s/pkg/config"
)

// ErrNotConfigured is returned by Open when the project has no attachments
// section. Callers must treat it as a distinct, expected state: attachments
// are opt-in, since the local backend writes files under the project and
// the S3 backend needs a bucket a project has not necessarily reviewed.
var ErrNotConfigured = errors.New("attachments: not configured")

// SourceKind classifies the data source a project is being read through. It
// decides whether the local backend may be used and which segment names a
// project's blobs.
type SourceKind string

const (
	SourceDolt   SourceKind = "dolt"
	SourceSQLite SourceKind = "sqlite"
	SourceJSONL  SourceKind = "jsonl"
)

// SourceInfo describes the project a blob store is being opened for. It is
// defined here, rather than reused as an internal/datasource type, so that
// pkg/ui (which cannot import cmd/b9s) has a stable, minimal shape to build
// from without needing every field datasource.DataSource carries. A caller
// that already holds a datasource.DataSource gets one from
// SourceInfoFromDataSource.
type SourceInfo struct {
	Kind SourceKind
	// Database is the Dolt database name. Empty for SQLite and JSONL.
	Database string
	// BeadsDir is the project's .beads directory, used for the local
	// backend's default directory.
	BeadsDir string
	// ProjectName is the database segment for a source with no Dolt
	// database of its own (SQLite, JSONL).
	ProjectName string
}

// databaseSegment is the "database" a blob key is scoped under: the Dolt
// database name, or the project name for a source that has none.
func (s SourceInfo) databaseSegment() string {
	if s.Kind == SourceDolt {
		return s.Database
	}
	return s.ProjectName
}

// Handle is a blob store bound to one project: Key already carries the
// project's prefix and database segment, so a caller never assembles a blob
// key by hand and can never point one project's Key at another's blobs.
type Handle struct {
	Store    Store
	Key      func(hash string) (string, error)
	MaxBytes int64
	URLTTL   time.Duration
	GCGrace  time.Duration

	// prefix and database back ListPrefix; they are exactly the segments
	// Key already validated, so gc lists precisely the tree Key ever wrote
	// into rather than a hand-assembled path that could drift from it.
	prefix, database string
}

// ListPrefix returns the store prefix under which every blob Key produces
// for this project lives: gc lists this, never a path it assembles itself,
// so a gc run can never wander into another project's blobs.
func (h *Handle) ListPrefix() string {
	if h.prefix == "" {
		return h.database + "/sha256"
	}
	return h.prefix + "/" + h.database + "/sha256"
}

// dummyValidationHash is a syntactically valid sha256 hex string used only
// to exercise Key during Open, never to address a real blob.
const dummyValidationHash = "0000000000000000000000000000000000000000000000000000000000000000"

// credentialCommandTimeout bounds how long Open waits for
// attachments.s3.credential_command. It is a var, not a const, so a test can
// shrink it rather than waiting out the real timeout.
var credentialCommandTimeout = 10 * time.Second

const (
	envAccessKeyID     = "B9S_ATTACHMENTS_S3_ACCESS_KEY_ID"
	envSecretAccessKey = "B9S_ATTACHMENTS_S3_SECRET_ACCESS_KEY"
)

// Open selects and constructs the blob store for src, using cfg. cfg is nil
// exactly when the project has no attachments section, in which case Open
// returns ErrNotConfigured without touching the network or the filesystem.
func Open(ctx context.Context, cfg *config.AttachmentsConfig, src SourceInfo) (*Handle, error) {
	if cfg == nil {
		return nil, ErrNotConfigured
	}

	prefix := cfg.Prefix
	if prefix == "" {
		prefix = src.databaseSegment()
	}
	// Key trims and re-splits prefix on every call, so ListPrefix must be
	// built from these same normalised segments rather than the raw prefix:
	// otherwise "x/" and Key's "x" disagree, and ListPrefix's naive
	// concatenation produces a double slash that List then rejects.
	prefixSegs, err := NormalizePrefixSegments(prefix)
	if err != nil {
		return nil, fmt.Errorf("attachments: invalid prefix or database for this project: %w", err)
	}
	normalizedPrefix := strings.Join(prefixSegs, "/")
	database := src.databaseSegment()
	keyFunc := func(hash string) (string, error) {
		return Key(normalizedPrefix, database, "sha256", hash)
	}
	if _, err := keyFunc(dummyValidationHash); err != nil {
		return nil, fmt.Errorf("attachments: invalid prefix or database for this project: %w", err)
	}

	var store Store
	switch cfg.Backend {
	case "local":
		s, err := openLocal(cfg, src)
		if err != nil {
			return nil, err
		}
		store = s
	case "s3":
		s, err := openS3(ctx, cfg.S3)
		if err != nil {
			return nil, err
		}
		store = s
	default:
		// AttachmentsConfig.UnmarshalYAML already rejects any other value,
		// so this only fires for a Config assembled by hand rather than
		// loaded from YAML.
		return nil, fmt.Errorf("attachments: unknown backend %q", cfg.Backend)
	}

	return &Handle{
		Store:    store,
		Key:      keyFunc,
		MaxBytes: cfg.MaxBytesOrDefault(),
		URLTTL:   cfg.URLTTLOrDefault(),
		GCGrace:  cfg.GCGraceOrDefault(),
		prefix:   normalizedPrefix,
		database: database,
	}, nil
}

// openLocal refuses a Dolt source unless the operator has explicitly opted
// in with attachments.local_with_dolt_server. b9s's own shared Dolt server
// runs behind an SSH tunnel, so every project reaches it as 127.0.0.1: a
// host-based loopback check cannot tell that setup apart from Dolt genuinely
// running solo on this machine, and the wrong guess leaves files the local
// backend writes invisible to every other operator sharing that server.
func openLocal(cfg *config.AttachmentsConfig, src SourceInfo) (*Local, error) {
	if src.Kind == SourceDolt && !cfg.LocalWithDoltServer {
		return nil, fmt.Errorf(
			"attachments: local backend refused for a Dolt server source; set attachments.local_with_dolt_server: true if Dolt is not shared with other operators")
	}
	dir := cfg.LocalDir
	if dir == "" {
		if src.BeadsDir == "" {
			return nil, fmt.Errorf("attachments: local backend needs attachments.local_dir or a project .beads directory")
		}
		dir = filepath.Join(src.BeadsDir, "attachments")
	}
	return NewLocal(dir), nil
}

// openS3 never sets CreateBucket: that flag exists for disposable test
// backends (see NewS3), not for Open, which always runs against a bucket an
// operator has already provisioned.
func openS3(ctx context.Context, cfg config.S3AttachmentsConfig) (*S3, error) {
	accessKeyID, secretAccessKey, err := resolveS3Credentials(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return NewS3(ctx, S3Config{
		Endpoint:        cfg.Endpoint,
		Region:          cfg.Region,
		Bucket:          cfg.Bucket,
		PathStyle:       cfg.PathStyle,
		AccessKeyID:     accessKeyID,
		SecretAccessKey: secretAccessKey,
	})
}

// resolveS3Credentials prefers the environment over credential_command, so
// an operator can override a project's configured command for one shell
// without editing the file. A single env var set without its pair is
// treated as a mistake rather than silently falling through to the command.
func resolveS3Credentials(ctx context.Context, cfg config.S3AttachmentsConfig) (accessKeyID, secretAccessKey string, err error) {
	id, idSet := os.LookupEnv(envAccessKeyID)
	secret, secretSet := os.LookupEnv(envSecretAccessKey)
	idPresent := idSet && id != ""
	secretPresent := secretSet && secret != ""

	switch {
	case idPresent && secretPresent:
		return id, secret, nil
	case idPresent != secretPresent:
		return "", "", fmt.Errorf("attachments: set both %s and %s, not only one", envAccessKeyID, envSecretAccessKey)
	}

	if cfg.CredentialCommand != "" {
		return runCredentialCommand(ctx, cfg.CredentialCommand)
	}
	return "", "", fmt.Errorf("attachments: no S3 credentials; set %s and %s, or attachments.s3.credential_command", envAccessKeyID, envSecretAccessKey)
}

// runCredentialCommand runs command through sh -c with a bounded timeout and
// reads exactly two lines from stdout: the access key id, then the secret
// access key. Stderr is discarded rather than folded into the returned
// error, since a credential helper's diagnostic output can itself contain a
// secret, and that error can end up in a log.
//
// The command runs in its own process group, and cmd.Cancel kills the whole
// group rather than only the sh process exec.CommandContext started. sh's
// stdout pipe is inherited by every descendant it forks, including a
// backgrounded or foreground child sh is still waiting on, so killing sh
// alone leaves that child holding the pipe open and Cmd.Wait blocked on it
// until the child exits by itself. WaitDelay bounds that same wait for a
// process group member that ignores SIGKILL's stdio side effect (a rare
// case where a resource, not a signal, still holds a copy of the write end).
func runCredentialCommand(ctx context.Context, command string) (accessKeyID, secretAccessKey string, err error) {
	runCtx, cancel := context.WithTimeout(ctx, credentialCommandTimeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, "sh", "-c", command)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	// A grandchild that outlives WaitDelay still holding the pipe makes
	// Cmd.Wait return an error instead of hanging, which is exactly what
	// callers here already treat as failure: this bounds the wait rather
	// than needing to distinguish that case from any other command failure.
	cmd.WaitDelay = time.Second
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	runErr := cmd.Run()

	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		return "", "", fmt.Errorf("attachments.s3.credential_command timed out after %s", credentialCommandTimeout)
	}
	if runErr != nil {
		return "", "", fmt.Errorf("attachments.s3.credential_command failed: %w", runErr)
	}

	lines := strings.Split(strings.TrimRight(stdout.String(), "\n"), "\n")
	if len(lines) != 2 || strings.TrimSpace(lines[0]) == "" || strings.TrimSpace(lines[1]) == "" {
		return "", "", fmt.Errorf(
			"attachments.s3.credential_command must print exactly two non-empty lines (access key id, then secret access key), got %d",
			len(lines))
	}
	return strings.TrimSpace(lines[0]), strings.TrimSpace(lines[1]), nil
}
