package attach

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/vanderheijden86/beadwork/internal/attachref"
	"github.com/vanderheijden86/beadwork/internal/blobstore"
)

// Download fetches att's blob from h into a temp file created in dir,
// verifying its sha256 against att.SHA256 before returning. The caller owns
// the returned temp file: move or read it, then remove it. A hash mismatch
// removes the temp file here and returns an error, rather than handing the
// caller bytes it has not verified.
func Download(ctx context.Context, h *blobstore.Handle, att attachref.Attachment, dir string) (tempPath string, err error) {
	key, err := h.Key(att.SHA256)
	if err != nil {
		return "", err
	}
	rc, err := h.Store.Open(ctx, key)
	if err != nil {
		return "", err
	}
	defer rc.Close()

	tmp, err := os.CreateTemp(dir, ".b9s-attach-download-*")
	if err != nil {
		return "", err
	}
	tempPath = tmp.Name()
	defer tmp.Close()

	hasher := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, hasher), rc); err != nil {
		os.Remove(tempPath)
		return "", fmt.Errorf("downloading %s: %w", att.Name, err)
	}
	if sum := hex.EncodeToString(hasher.Sum(nil)); sum != att.SHA256 {
		os.Remove(tempPath)
		return "", fmt.Errorf("downloaded bytes for %s hash to %s, want %s", att.Name, sum, att.SHA256)
	}
	return tempPath, nil
}

// SafeJoin joins dir with name for a download destination. name comes from
// a comment and so is untrusted even though attachref already rejects a
// path separator in it; this is an independent check on the result rather
// than trust in attachref alone. It fails whenever filepath.Base would have
// to strip anything from name, since that means name was not already a bare
// file name.
func SafeJoin(dir, name string) (string, error) {
	base := filepath.Base(name)
	if base != name || base == "." || base == ".." {
		return "", fmt.Errorf("attachment name %q is not a safe file name", name)
	}
	return filepath.Join(dir, base), nil
}

// URL returns a presigned link for att, only for the S3 backend. The local
// backend's Store.URL returns a file:// path, which is not the link the
// `url` subcommand promises, so it is refused here rather than returned.
func URL(ctx context.Context, h *blobstore.Handle, att attachref.Attachment) (string, error) {
	if _, ok := h.Store.(*blobstore.S3); !ok {
		return "", errors.New("attachments: presigned URLs need the s3 backend; this project uses the local backend")
	}
	key, err := h.Key(att.SHA256)
	if err != nil {
		return "", err
	}
	return h.Store.URL(ctx, key, h.URLTTL, att.Name)
}
