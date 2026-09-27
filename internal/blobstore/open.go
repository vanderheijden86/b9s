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
	"time"

	"github.com/vanderheijden86/beadwork/pkg/config"
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
// defined here, rather than accepted as an internal/datasource type,
// because pkg/config already imports internal/datasource (for the recent
// projects list); blobstore importing datasource too would close a cycle
// back through config, which blobstore also needs for AttachmentsConfig. A
// caller that already holds a datasource.DataSource converts it to a
// SourceInfo itself.
type SourceInfo struct {
	Kind SourceKind
	// DoltHost is the Dolt server's host, with no port. Set only for
	// SourceDolt; Open refuses the local backend unless this is loopback.
	DoltHost string
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
}

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

	prefix := cfg.S3.Prefix
	if prefix == "" {
		prefix = src.databaseSegment()
	}
	database := src.databaseSegment()
	keyFunc := func(hash string) (string, error) {
		return Key(prefix, database, "sha256", hash)
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
	}, nil
}

// openLocal refuses a Dolt server that is not on loopback: files the local
// backend writes live only on this machine, so any other reader of that
// server (another operator's b9s, a CI job) would see the reference comment
// but never find the bytes.
func openLocal(cfg *config.AttachmentsConfig, src SourceInfo) (*Local, error) {
	if src.Kind == SourceDolt && !isLoopbackHost(src.DoltHost) {
		return nil, fmt.Errorf(
			"attachments: local backend refused for Dolt server %q, which is not on loopback; other readers of that server cannot see local files",
			src.DoltHost)
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
func runCredentialCommand(ctx context.Context, command string) (accessKeyID, secretAccessKey string, err error) {
	runCtx, cancel := context.WithTimeout(ctx, credentialCommandTimeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, "sh", "-c", command)
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
	if len(lines) != 2 || lines[0] == "" || lines[1] == "" {
		return "", "", fmt.Errorf(
			"attachments.s3.credential_command must print exactly two non-empty lines (access key id, then secret access key), got %d",
			len(lines))
	}
	return lines[0], lines[1], nil
}
