package blobstore

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"
)

// s3TestServerImageTag matches a pinned, versioned image reference such as
// "image=adobe/s3mock:3.11.0". It requires a dotted version after the colon,
// so an edit that drops the pin to ":latest" or a bare image name fails this
// check instead of floating silently.
var s3TestServerImageTag = regexp.MustCompile(`\bimage=\S+:\d+\.\d+(\.\d+)?\S*\b`)

// TestAttachmentsS3ScriptPinsImageTag checks that the disposable S3 server
// used by the gated S3 blobstore tests names a specific version, not
// "latest" or an untagged image: a floating tag changes what a test run
// exercises with no corresponding diff to review.
func TestAttachmentsS3ScriptPinsImageTag(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// internal/blobstore -> repo root -> scripts/attachments-s3.sh
	root := filepath.Dir(filepath.Dir(filepath.Dir(file)))
	script, err := os.ReadFile(filepath.Join(root, "scripts", "attachments-s3.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if !s3TestServerImageTag.Match(script) {
		t.Fatalf("scripts/attachments-s3.sh does not pin a versioned image tag:\n%s", script)
	}
}
