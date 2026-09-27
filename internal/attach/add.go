package attach

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"mime"
	"net/http"
	"os"
	"path/filepath"

	"github.com/vanderheijden86/beadwork/internal/attachref"
	"github.com/vanderheijden86/beadwork/internal/blobstore"
)

// dummySHA256 is a syntactically valid hash used only to isolate a name
// validation error from attachref.Format's other checks; see checkName.
const dummySHA256 = "0000000000000000000000000000000000000000000000000000000000000000"

// AddResult is the outcome of attaching one file.
type AddResult struct {
	Path string
	Ref  attachref.Ref
	// Skipped reports that the blob already existed in the store, so Put
	// was not called again.
	Skipped bool
	Err     error
}

// Add uploads each path in order and writes one `bd comments add` per file.
// A failure on one file is recorded in its own result; it never stops or
// undoes the files processed before or after it.
func Add(ctx context.Context, h *blobstore.Handle, bd BdRunner, issueID string, paths []string) []AddResult {
	results := make([]AddResult, 0, len(paths))
	for _, path := range paths {
		results = append(results, addOne(ctx, h, bd, issueID, path))
	}
	return results
}

func addOne(ctx context.Context, h *blobstore.Handle, bd BdRunner, issueID, path string) AddResult {
	name := filepath.Base(path)
	if err := checkName(name); err != nil {
		return AddResult{Path: path, Err: fmt.Errorf("%s: %w", path, err)}
	}

	ref, tempPath, err := hashAndSniff(h, path, name)
	if err != nil {
		return AddResult{Path: path, Err: err}
	}
	defer os.Remove(tempPath)

	// Put lands before the comment: a crash between the two leaves an
	// orphan blob for gc to reclaim, never a comment whose bytes are
	// missing.
	skipped, err := putIfAbsent(ctx, h, ref, tempPath)
	if err != nil {
		return AddResult{Path: path, Err: fmt.Errorf("%s: %w", path, err)}
	}

	text, err := attachref.Format(ref)
	if err != nil {
		return AddResult{Path: path, Ref: ref, Skipped: skipped, Err: fmt.Errorf("%s: %w", path, err)}
	}
	if output, err := bd.Run(ctx, "comments", "add", issueID, text); err != nil {
		return AddResult{Path: path, Ref: ref, Skipped: skipped, Err: fmt.Errorf("%s: bd comments add: %s: %w", path, output, err)}
	}
	return AddResult{Path: path, Ref: ref, Skipped: skipped}
}

// checkName isolates a name-only validation error from attachref.Format's
// other checks (sha256, size, type), by pairing name with values Format
// always accepts. A name Format rejects is reported before any byte of the
// file is read.
func checkName(name string) error {
	_, err := attachref.Format(attachref.Ref{
		SHA256: dummySHA256,
		Size:   0,
		Type:   "application/octet-stream",
		Name:   name,
	})
	return err
}

// hashAndSniff streams path into a temp file while hashing it, stopping as
// soon as it exceeds h.MaxBytes so an oversized file is never read in full.
// On success the temp file is left on disk for the caller to Put and
// remove; on any error it removes the temp file itself.
func hashAndSniff(h *blobstore.Handle, path, name string) (attachref.Ref, string, error) {
	// os.Stat follows symlinks, so a symlink to a regular file is accepted
	// and a symlink to a FIFO or directory is rejected the same as the real
	// thing. Checked before os.Open because os.Open on a FIFO with no writer
	// blocks indefinitely, and ctx does not bound that call: a network
	// timeout does nothing for a local open that never returns.
	info, err := os.Stat(path)
	if err != nil {
		return attachref.Ref{}, "", fmt.Errorf("%s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return attachref.Ref{}, "", fmt.Errorf("%s: not a regular file (%s)", path, fileKindDescription(info.Mode()))
	}

	src, err := os.Open(path)
	if err != nil {
		return attachref.Ref{}, "", fmt.Errorf("%s: %w", path, err)
	}
	defer src.Close()

	tmp, err := os.CreateTemp("", "b9s-attach-*")
	if err != nil {
		return attachref.Ref{}, "", err
	}
	tempPath := tmp.Name()
	defer tmp.Close()

	// h.MaxBytes+1 gives the reader one byte of headroom to detect an
	// over-limit file; pkg/config caps a loaded MaxBytes well below
	// math.MaxInt64, but a Handle built directly (as tests do) is not bound
	// by that check, and MaxBytes+1 at math.MaxInt64 would overflow to a
	// negative limit, which io.LimitReader silently reads as "nothing".
	limit := h.MaxBytes
	if limit < math.MaxInt64 {
		limit++
	}
	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(tmp, hasher), io.LimitReader(src, limit))
	if err != nil {
		os.Remove(tempPath)
		return attachref.Ref{}, "", fmt.Errorf("reading %s: %w", path, err)
	}
	if written > h.MaxBytes {
		os.Remove(tempPath)
		return attachref.Ref{}, "", fmt.Errorf("%s is %d bytes, over the %d byte limit", path, written, h.MaxBytes)
	}

	header := make([]byte, 512)
	n, err := tmp.ReadAt(header, 0)
	if err != nil && err != io.EOF {
		os.Remove(tempPath)
		return attachref.Ref{}, "", fmt.Errorf("%s: %w", path, err)
	}

	sum := hex.EncodeToString(hasher.Sum(nil))
	ref := attachref.Ref{SHA256: sum, Size: written, Type: sniffMIME(header[:n], name), Name: name}
	return ref, tempPath, nil
}

// fileKindDescription names the non-regular file kind for the error
// hashAndSniff returns, so "not a regular file" is a directory, named pipe or
// device rather than a bare rejection the caller has to guess at.
func fileKindDescription(mode os.FileMode) string {
	switch {
	case mode&os.ModeDir != 0:
		return "a directory"
	case mode&os.ModeNamedPipe != 0:
		return "a named pipe"
	case mode&os.ModeSocket != 0:
		return "a socket"
	case mode&os.ModeDevice != 0:
		return "a device"
	case mode&os.ModeSymlink != 0:
		return "a symlink" // os.Stat resolves symlinks, so this should be unreachable
	default:
		return mode.String()
	}
}

// sniffMIME sniffs content from header (http.DetectContentType), then falls
// back to the file extension only when the sniff is too generic to be
// useful (application/octet-stream or text/plain, which DetectContentType
// returns for most text formats regardless of extension). Either way the
// result is reduced to a bare type/subtype: attachref.Format rejects MIME
// parameters such as "; charset=utf-8".
func sniffMIME(header []byte, name string) string {
	mt := bareMediaType(http.DetectContentType(header))
	if mt == "application/octet-stream" || mt == "text/plain" {
		if ext := filepath.Ext(name); ext != "" {
			if extType := mime.TypeByExtension(ext); extType != "" {
				if bare := bareMediaType(extType); bare != "" {
					mt = bare
				}
			}
		}
	}
	return mt
}

func bareMediaType(t string) string {
	bare, _, err := mime.ParseMediaType(t)
	if err != nil {
		return "application/octet-stream"
	}
	return bare
}

// putIfAbsent uploads the blob at tempPath under ref's key unless the store
// already holds it, so a retried Add after a crash never re-uploads bytes
// that already made it to the store.
func putIfAbsent(ctx context.Context, h *blobstore.Handle, ref attachref.Ref, tempPath string) (skipped bool, err error) {
	key, err := h.Key(ref.SHA256)
	if err != nil {
		return false, err
	}
	if _, statErr := h.Store.Stat(ctx, key); statErr == nil {
		return true, nil
	} else if !errors.Is(statErr, blobstore.ErrNotFound) {
		return false, statErr
	}

	f, err := os.Open(tempPath)
	if err != nil {
		return false, err
	}
	defer f.Close()
	if err := h.Store.Put(ctx, key, f, ref.Size, ref.Type); err != nil {
		return false, err
	}
	return false, nil
}
