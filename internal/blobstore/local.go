package blobstore

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

// Local stores blobs as files under root, one file per key: a key's slashes
// become path separators. It backs solo and embedded use, where the bytes
// and the Dolt database live on the same machine.
type Local struct {
	root string
}

// NewLocal returns a Store rooted at root. root need not exist yet; Put
// creates whatever directories a key requires.
func NewLocal(root string) *Local {
	return &Local{root: filepath.Clean(root)}
}

// Put is idempotent for a key that already holds size bytes, so a retried
// upload after a crash writes nothing new. It writes through a temp file in
// the destination directory and renames into place, so a concurrent reader
// never observes a partially written blob.
func (l *Local) Put(ctx context.Context, key string, r io.Reader, size int64, mimeType string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	abs, existing, err := l.resolveForAccess(key)
	if err != nil {
		return err
	}
	if existing != nil {
		if !existing.Mode().IsRegular() {
			return fmt.Errorf("blob path %q exists and is not a regular file", key)
		}
		if existing.Size() == size {
			return nil
		}
	}

	dir := filepath.Dir(abs)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create blob directory: %w", err)
	}
	// MkdirAll may have just created the directory resolveForAccess checked
	// before it existed; check again so the containment check sees the same
	// tree the temp file and rename below are about to use.
	if err := rejectEscapingAncestor(l.root, dir); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, ".~blob-*")
	if err != nil {
		return fmt.Errorf("create temp blob: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	n, err := io.Copy(tmp, r)
	if err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write blob: %w", err)
	}
	if n != size {
		_ = tmp.Close()
		return fmt.Errorf("blob %q: wrote %d bytes, want %d", key, n, size)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temp blob: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp blob: %w", err)
	}
	if err := os.Rename(tmpPath, abs); err != nil {
		return fmt.Errorf("rename blob into place: %w", err)
	}
	cleanup = false
	return nil
}

// Open never follows a symlink planted at a blob's own path: blob_key is
// untrusted data that can arrive over a Dolt pull from another party, and
// following such a link would read whatever file it points to.
func (l *Local) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	abs, existing, err := l.resolveForAccess(key)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, ErrNotFound
	}
	if !existing.Mode().IsRegular() {
		return nil, fmt.Errorf("blob path %q is not a regular file", key)
	}
	f, err := os.Open(abs) //nolint:gosec // abs is resolved and containment-checked above
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return f, nil
}

// Stat reports the same Info a completed List would for this key, without
// walking the rest of the store.
func (l *Local) Stat(ctx context.Context, key string) (Info, error) {
	_, existing, err := l.resolveForAccess(key)
	if err != nil {
		return Info{}, err
	}
	if existing == nil {
		return Info{}, ErrNotFound
	}
	if !existing.Mode().IsRegular() {
		return Info{}, fmt.Errorf("blob path %q is not a regular file", key)
	}
	return Info{Key: key, Size: existing.Size(), LastModified: existing.ModTime()}, nil
}

// Delete is idempotent: a key already gone is not an error, because gc
// retries its deletes and two gc runs can race over the same orphan.
func (l *Local) Delete(ctx context.Context, key string) error {
	abs, existing, err := l.resolveForAccess(key)
	if err != nil {
		return err
	}
	if existing == nil {
		return nil
	}
	if err := os.Remove(abs); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete blob: %w", err)
	}
	return nil
}

// List walks every blob under prefix in the order filepath.WalkDir visits
// them. An empty prefix walks the whole store. gc is the only caller, and it
// relies on List never reporting a symlink as a blob: gc's Delete would then
// remove the link's target's directory entry, not a blob.
func (l *Local) List(ctx context.Context, prefix string, fn func(Info) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	start := l.root
	if trimmed := strings.TrimSuffix(prefix, "/"); trimmed != "" {
		clean, err := cleanRelPath(trimmed)
		if err != nil {
			return fmt.Errorf("list prefix %q: %w", prefix, err)
		}
		start = filepath.Join(l.root, clean)
		if err := ensureWithin(l.root, start); err != nil {
			return err
		}
	}
	if _, err := os.Stat(start); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat list prefix: %w", err)
	}
	return filepath.WalkDir(start, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || d.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(l.root, path)
		if err != nil {
			return err
		}
		return fn(Info{Key: filepath.ToSlash(rel), Size: info.Size(), LastModified: info.ModTime()})
	})
}

// URL returns a file:// link. It carries no expiry: a local file has no
// presigned access to revoke, so ttl and filename are unused here and exist
// only to satisfy Store for callers that do not branch on backend.
func (l *Local) URL(ctx context.Context, key string, ttl time.Duration, filename string) (string, error) {
	abs, existing, err := l.resolveForAccess(key)
	if err != nil {
		return "", err
	}
	if existing == nil {
		return "", ErrNotFound
	}
	return "file://" + abs, nil
}

// resolveForAccess validates key as a safe relative path, confirms no
// ancestor symlink resolves outside root, and rejects the leaf itself if it
// is a symlink, dangling or not. The third check is separate from the
// second because an ancestor check that only resolves what already exists
// cannot see a dangling symlink at the leaf: its target does not exist, so
// the containment walk treats the leaf as though it were simply absent.
// existing is nil when nothing occupies the path yet.
func (l *Local) resolveForAccess(key string) (abs string, existing fs.FileInfo, err error) {
	clean, err := cleanRelPath(key)
	if err != nil {
		return "", nil, err
	}
	abs = filepath.Join(l.root, clean)
	if err := ensureWithin(l.root, abs); err != nil {
		return "", nil, err
	}
	if err := rejectEscapingAncestor(l.root, abs); err != nil {
		return "", nil, err
	}
	fi, statErr := os.Lstat(abs)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			return abs, nil, nil
		}
		return "", nil, fmt.Errorf("lstat blob path: %w", statErr)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return "", nil, fmt.Errorf("blob path %q is a symlink, refusing", key)
	}
	return abs, fi, nil
}

// rejectEscapingAncestor fails if a symlink anywhere along abs's existing
// ancestors resolves outside root. A syntactic prefix match on abs is not
// enough: a symlinked directory placed under root can point outside it while
// still satisfying filepath.Rel against the unresolved path.
func rejectEscapingAncestor(root, abs string) error {
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		if os.IsNotExist(err) {
			// Nothing has been stored under root yet, so there is no
			// symlink an attacker could have planted there.
			return nil
		}
		return fmt.Errorf("resolve blob store root: %w", err)
	}
	resolved, err := evalExistingPrefix(abs)
	if err != nil {
		return fmt.Errorf("resolve blob path: %w", err)
	}
	if err := ensureWithin(realRoot, resolved); err != nil {
		return fmt.Errorf("blob path escapes blob store root: %w", err)
	}
	return nil
}

// evalExistingPrefix resolves symlinks along the longest existing ancestor
// of path and rejoins the remaining, not-yet-created suffix unresolved, so a
// path that does not exist yet (a new blob about to be written) can still be
// checked for an escaping symlink in one of its parent directories.
func evalExistingPrefix(path string) (string, error) {
	remainder := ""
	current := filepath.Clean(path)
	for {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			if remainder == "" {
				return resolved, nil
			}
			return filepath.Join(resolved, remainder), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return path, nil
		}
		if remainder == "" {
			remainder = filepath.Base(current)
		} else {
			remainder = filepath.Join(filepath.Base(current), remainder)
		}
		current = parent
	}
}

// cleanRelPath rejects a key that is not a clean relative path: an absolute
// path, an empty segment, a ".." segment, a backslash, a control character or
// whitespace would let a caller escape root, collide with an unrelated file,
// parse differently on Windows than on the platform that wrote it, or alias
// with a visually identical key that lacks the whitespace. Both Local and the
// S3 backend call this same check for every method that takes a key, since a
// blob_key is untrusted: it can arrive over a Dolt pull from another party,
// and Key() itself never produces whitespace, so trimming it here would only
// ever paper over a hostile or corrupted value.
func cleanRelPath(key string) (string, error) {
	if key == "" {
		return "", fmt.Errorf("blob key is empty")
	}
	if filepath.IsAbs(key) || strings.HasPrefix(key, "/") {
		return "", fmt.Errorf("blob key %q must be relative", key)
	}
	if strings.ContainsRune(key, '\\') {
		return "", fmt.Errorf("blob key %q must not contain a backslash", key)
	}
	for _, r := range key {
		if r < 0x20 || r == 0x7f {
			return "", fmt.Errorf("blob key %q contains a control character", key)
		}
		if unicode.IsSpace(r) {
			return "", fmt.Errorf("blob key %q contains whitespace", key)
		}
	}
	// strings.Split, not strings.FieldsFunc: FieldsFunc silently collapses a
	// run of separators, which would let "a//b" slip past the empty-segment
	// check below instead of being rejected by it.
	for _, part := range strings.Split(key, "/") {
		if part == "" || part == "." || part == ".." {
			return "", fmt.Errorf("blob key %q is unsafe", key)
		}
	}
	clean := filepath.Clean(filepath.FromSlash(key))
	if clean == "." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." {
		return "", fmt.Errorf("blob key %q is unsafe", key)
	}
	return clean, nil
}

// ensureWithin fails if path is not root or a descendant of it.
func ensureWithin(root, path string) error {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return fmt.Errorf("resolve blob path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("blob path escapes blob store root")
	}
	return nil
}
