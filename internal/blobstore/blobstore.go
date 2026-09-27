// Package blobstore holds attachment bytes. Rows in the attachments table name a
// blob by key only; which backend and bucket hold it is configuration, so a
// project can change backends by copying objects, with no data migration.
package blobstore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
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

// NormalizePrefixSegments splits prefix on "/", trims a leading or trailing
// slash, and validates each segment against safeSegment. Key and Open's
// ListPrefix both call this rather than each trimming prefix on their own, so
// "x", "x/" and "/x/" always normalise to the identical "x": a prefix Key
// accepted with a trailing slash previously produced a ListPrefix with a
// double slash that Local.List's cleanRelPath then rejected outright.
func NormalizePrefixSegments(prefix string) ([]string, error) {
	if prefix == "" {
		return nil, nil
	}
	segs := strings.Split(strings.Trim(prefix, "/"), "/")
	for _, seg := range segs {
		if !safeSegment.MatchString(seg) {
			return nil, fmt.Errorf("prefix segment %q is not safe", seg)
		}
	}
	return segs, nil
}

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
	segs, err := NormalizePrefixSegments(prefix)
	if err != nil {
		return "", err
	}
	parts := append(segs, database, algo, hash[:2], hash)
	return strings.Join(parts, "/"), nil
}

// isLoopbackHost reports whether host names only the local machine: a bare
// "localhost" or a loopback IP literal, never a hostname that merely
// resolves there today. The S3 backend's CreateBucket guard uses this to
// stop a config mistake from creating a bucket against a remote endpoint.
func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}
