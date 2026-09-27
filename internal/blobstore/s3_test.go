package blobstore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithy "github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// TestS3StoreContract runs only against a disposable MinIO. It never touches a
// real bucket: B9S_TEST_S3_ENDPOINT must be a loopback URL.
func TestS3StoreContract(t *testing.T) {
	store := newGatedS3Store(t)
	runContract(t, store)

	t.Run("presigned URL serves the exact bytes with content-disposition", func(t *testing.T) {
		testS3PresignedDownload(t, store)
	})
}

// TestS3PutRejectsShortRead checks the requirement Local already enforces:
// a declared size larger than what r actually yields is a write error, and
// leaves no object behind for a caller to later Stat or Open successfully.
func TestS3PutRejectsShortRead(t *testing.T) {
	store := newGatedS3Store(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	key := "ws/db/sha256/ab/short-read"
	body := []byte("short")
	err := store.Put(ctx, key, bytes.NewReader(body), int64(len(body))+10, "text/plain")
	if err == nil {
		t.Fatal("Put with a declared size larger than the body succeeded, want error")
	}
	if _, statErr := store.Stat(ctx, key); !errors.Is(statErr, ErrNotFound) {
		t.Fatalf("Put on a short read left an object behind: Stat err = %v, want ErrNotFound", statErr)
	}
}

// TestS3PutRejectsLongRead is the other direction of the same check: a
// declared size smaller than what r actually yields must also fail, and
// must not silently store a truncated object.
func TestS3PutRejectsLongRead(t *testing.T) {
	store := newGatedS3Store(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	key := "ws/db/sha256/ab/long-read"
	body := []byte("more bytes than declared")
	err := store.Put(ctx, key, bytes.NewReader(body), int64(len(body))-5, "text/plain")
	if err == nil {
		t.Fatal("Put with a declared size smaller than the body succeeded, want error")
	}
	if _, statErr := store.Stat(ctx, key); !errors.Is(statErr, ErrNotFound) {
		t.Fatalf("Put on a long read left an object behind: Stat err = %v, want ErrNotFound", statErr)
	}
}

// newGatedS3Store returns a Store under a fresh sub-prefix of the shared
// beads-test bucket, or skips the test when no scratch MinIO is configured.
// A fresh prefix per call keeps concurrent test functions independent
// without ever emptying the bucket.
func newGatedS3Store(t *testing.T) Store {
	t.Helper()
	endpoint := os.Getenv("B9S_TEST_S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("set B9S_TEST_S3_ENDPOINT (see scripts/attachments-s3.sh)")
	}
	s, err := NewS3(context.Background(), S3Config{
		Endpoint:        endpoint,
		Region:          "us-east-1",
		Bucket:          "beads-test",
		PathStyle:       true,
		AccessKeyID:     "minioadmin",
		SecretAccessKey: "minioadmin",
		CreateBucket:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return prefixed(s, fmt.Sprintf("run-%d", time.Now().UnixNano()))
}

// testS3PresignedDownload fetches a URL over real HTTP and checks it serves
// the exact bytes under the original filename. runContract already deletes
// the key it exercises, so this uses its own, exercised only here.
func testS3PresignedDownload(t *testing.T, store Store) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	key := "ws/db/sha256/ef/presign-check"
	body := []byte("presigned attachment bytes")
	const filename = `weird "name".txt`
	if err := store.Put(ctx, key, bytes.NewReader(body), int64(len(body)), "text/plain"); err != nil {
		t.Fatal(err)
	}
	link, err := store.URL(ctx, key, time.Minute, filename)
	if err != nil {
		t.Fatal(err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("presigned GET status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("presigned GET body = %q, want %q", got, body)
	}
	disposition := resp.Header.Get("Content-Disposition")
	if !strings.Contains(disposition, "attachment") || !strings.Contains(disposition, "filename") {
		t.Fatalf("Content-Disposition = %q, want an attachment filename", disposition)
	}
}

// TestContentDispositionSanitizesFilename checks the header builder in
// isolation: a name carrying a quote, backslash, CR/LF or non-ASCII byte
// must never appear unescaped in the value, since that byte could otherwise
// inject a second header or break the quoted-string it sits in.
func TestContentDispositionSanitizesFilename(t *testing.T) {
	tests := []struct {
		name     string
		filename string
	}{
		{"plain name", "report.pdf"},
		{"double quote", `weird "name".txt`},
		{"backslash", `weird\name.txt`},
		{"embedded CR LF", "evil\r\nSet-Cookie: x=y"},
		{"non-ASCII", "résumé.pdf"},
		{"empty name", ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			disposition := contentDisposition(test.filename)
			if disposition == "" {
				t.Fatal("contentDisposition returned empty string")
			}
			if !strings.HasPrefix(disposition, "attachment") {
				t.Fatalf("contentDisposition(%q) = %q, want it to start with \"attachment\"", test.filename, disposition)
			}
			for _, bad := range []string{"\r", "\n"} {
				if strings.Contains(disposition, bad) {
					t.Fatalf("contentDisposition(%q) = %q, contains a raw CR/LF", test.filename, disposition)
				}
			}
			for _, b := range []byte(disposition) {
				if b >= 0x80 {
					t.Fatalf("contentDisposition(%q) = %q, contains a raw non-ASCII byte", test.filename, disposition)
				}
			}
		})
	}
}

// TestS3KeyRejectsUnsafeKeys checks that every method taking a key, plus
// List's prefix, refuses an unsafe key before it would ever reach the
// network: blob_key is untrusted, since it can arrive over a Dolt pull from
// another party, so this must hold even with no MinIO reachable at all.
func TestS3KeyRejectsUnsafeKeys(t *testing.T) {
	s := &S3{bucket: "unused"} // no client call is reached for a rejected key

	unsafe := []string{
		"",
		"/abs/path",
		"../escape",
		"a/../../b",
		"a/./b",
		"trailing/.",
		"back\\slash",
		"control\x00char",
		"a//b",
		" a/b",
		"a/b ",
		"a /b",
		"a/b\t",
	}

	ctx := context.Background()
	for _, key := range unsafe {
		t.Run(fmt.Sprintf("Put %q", key), func(t *testing.T) {
			if err := s.Put(ctx, key, bytes.NewReader(nil), 0, "text/plain"); err == nil {
				t.Fatalf("Put(%q) accepted an unsafe key", key)
			}
		})
		t.Run(fmt.Sprintf("Open %q", key), func(t *testing.T) {
			if _, err := s.Open(ctx, key); err == nil {
				t.Fatalf("Open(%q) accepted an unsafe key", key)
			}
		})
		t.Run(fmt.Sprintf("Stat %q", key), func(t *testing.T) {
			if _, err := s.Stat(ctx, key); err == nil {
				t.Fatalf("Stat(%q) accepted an unsafe key", key)
			}
		})
		t.Run(fmt.Sprintf("Delete %q", key), func(t *testing.T) {
			if err := s.Delete(ctx, key); err == nil {
				t.Fatalf("Delete(%q) accepted an unsafe key", key)
			}
		})
		t.Run(fmt.Sprintf("URL %q", key), func(t *testing.T) {
			if _, err := s.URL(ctx, key, time.Minute, "name.txt"); err == nil {
				t.Fatalf("URL(%q) accepted an unsafe key", key)
			}
		})
		if key == "" {
			continue // List treats an empty prefix as "the whole bucket", not unsafe.
		}
		t.Run(fmt.Sprintf("List prefix %q", key), func(t *testing.T) {
			err := s.List(ctx, key, func(Info) error {
				t.Fatalf("List(%q) invoked fn instead of rejecting the prefix", key)
				return nil
			})
			if err == nil {
				t.Fatalf("List(%q) accepted an unsafe prefix", key)
			}
		})
	}
}

// TestS3NewRefusesNonLoopbackEndpointWhenCreatingBucket checks that
// CreateBucket cannot be pointed at a real object store by a config
// mistake: it only ever provisions a bucket against a loopback endpoint.
func TestS3NewRefusesNonLoopbackEndpointWhenCreatingBucket(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
	}{
		{"empty endpoint", ""},
		{"public host", "https://s3.example.com"},
		{"public IP", "http://93.184.216.34:9000"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewS3(context.Background(), S3Config{
				Endpoint:        test.endpoint,
				Region:          "us-east-1",
				Bucket:          "beads-test",
				PathStyle:       true,
				AccessKeyID:     "minioadmin",
				SecretAccessKey: "minioadmin",
				CreateBucket:    true,
			})
			if err == nil {
				t.Fatalf("NewS3 with CreateBucket against %q succeeded, want a refusal", test.endpoint)
			}
		})
	}
}

// TestS3NewAllowsLoopbackEndpointWhenCreatingBucket checks the boundary the
// refusal above sits on: a loopback endpoint must still be allowed to reach
// client construction (the actual bucket creation needs a live MinIO and is
// exercised by the gated TestS3StoreContract).
func TestS3NewAllowsLoopbackEndpointWhenCreatingBucket(t *testing.T) {
	for _, host := range []string{"127.0.0.1", "localhost", "[::1]"} {
		t.Run(host, func(t *testing.T) {
			if err := requireLoopbackEndpoint("http://" + host + ":9000"); err != nil {
				t.Fatalf("requireLoopbackEndpoint(%q) = %v, want nil", host, err)
			}
		})
	}
}

// TestS3IsNotFoundErrClassifiesByType checks that not-found detection reads
// the error's type or, failing that, its HTTP status code, and never the
// text of its message: a service that phrases "not found" differently, or
// an unrelated error that happens to mention "not found", must not be
// confused for a missing object.
func TestS3IsNotFoundErrClassifiesByType(t *testing.T) {
	notFoundResponse := &smithyhttp.ResponseError{
		Response: &smithyhttp.Response{Response: &http.Response{StatusCode: http.StatusNotFound}},
	}
	serverErrorResponse := &smithyhttp.ResponseError{
		Response: &smithyhttp.Response{Response: &http.Response{StatusCode: http.StatusInternalServerError}},
	}

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"NoSuchKey", &types.NoSuchKey{}, true},
		{"NotFound", &types.NotFound{}, true},
		{"wrapped NoSuchKey", fmt.Errorf("get object: %w", &types.NoSuchKey{}), true},
		{"generic API error coded NotFound", &smithy.GenericAPIError{Code: "NotFound", Message: "nope"}, true},
		{"generic API error coded AccessDenied", &smithy.GenericAPIError{Code: "AccessDenied", Message: "nope"}, false},
		{"smithy response 404", notFoundResponse, true},
		{"smithy response 500", serverErrorResponse, false},
		{"unrelated error mentioning not found", errors.New("widget not found in cache"), false},
		{"context deadline", context.DeadlineExceeded, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := isNotFoundErr(test.err); got != test.want {
				t.Errorf("isNotFoundErr(%v) = %v, want %v", test.err, got, test.want)
			}
		})
	}
}
