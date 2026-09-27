// Package blobstore holds attachment bytes. Rows in the attachments table name a
// blob by key only; which backend and bucket hold it is configuration, so a
// project can change backends by copying objects, with no data migration.
package blobstore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
)

// ErrNotFound is returned by Open and Stat when no blob has the key.
var ErrNotFound = errors.New("blob not found")

// Info describes a stored blob.
type Info struct {
	Key          string
	Size         int64
	LastModified time.Time
}

// Store is the byte layer under bd attachment. Implementations must make Put
// idempotent for a key that already holds the same bytes: keys are content
// hashes, so a retry after a crash writes nothing new.
type Store interface {
	Put(ctx context.Context, key string, r io.Reader, size int64, mimeType string) error
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	Stat(ctx context.Context, key string) (Info, error)
	Delete(ctx context.Context, key string) error
	// List yields every blob under prefix. gc is the only caller.
	List(ctx context.Context, prefix string, fn func(Info) error) error
	// URL returns a link a browser can open for ttl. Local stores return file://.
	URL(ctx context.Context, key string, ttl time.Duration, filename string) (string, error)
}

var (
	safeSegment = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	hexHash     = regexp.MustCompile(`^[0-9a-f]{7,128}$`)
)

// Key builds <prefix>/<database>/<algo>/<first two hex>/<hash>. prefix is the
// workspace and may be empty or contain slashes; every segment must be safe so
// a crafted database name cannot escape the workspace prefix.
func Key(prefix, database, algo, hash string) (string, error) {
	if algo != "sha256" {
		return "", fmt.Errorf("unsupported hash algorithm %q", algo)
	}
	if !hexHash.MatchString(hash) {
		return "", fmt.Errorf("content hash %q is not lower-case hex", hash)
	}
	if !safeSegment.MatchString(database) {
		return "", fmt.Errorf("database name %q is not a safe key segment", database)
	}
	parts := []string{}
	if prefix != "" {
		for _, seg := range strings.Split(strings.Trim(prefix, "/"), "/") {
			if !safeSegment.MatchString(seg) {
				return "", fmt.Errorf("prefix segment %q is not safe", seg)
			}
			parts = append(parts, seg)
		}
	}
	parts = append(parts, database, algo, hash[:2], hash)
	return strings.Join(parts, "/"), nil
}
