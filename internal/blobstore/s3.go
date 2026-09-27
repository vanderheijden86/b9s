package blobstore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithy "github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// fallbackAttachmentDownloadName is used for Content-Disposition when a
// filename cannot be encoded as a media type parameter at all (never
// expected for the token and RFC 2231 forms this package builds, but the
// header still has to be well-formed rather than empty).
const fallbackAttachmentDownloadName = "attachment"

// S3Config configures an S3-compatible backend. It fits both a real AWS
// bucket and any S3-compatible object store (MinIO, Hetzner Object Storage)
// reachable through Endpoint.
type S3Config struct {
	Endpoint        string
	Region          string
	Bucket          string
	PathStyle       bool
	AccessKeyID     string
	SecretAccessKey string
	// CreateBucket makes NewS3 create Bucket if it does not exist. Restricted
	// to a loopback Endpoint so a misconfigured production endpoint cannot
	// silently provision a bucket instead of failing.
	CreateBucket bool
}

// S3 stores blobs in a bucket on an S3-compatible object store. blob_key is
// untrusted (it can arrive over a Dolt pull from another party), so every
// method validates it the same way Local does before it reaches the network.
type S3 struct {
	client  *s3.Client
	presign *s3.PresignClient
	bucket  string
}

// NewS3 builds a Store backed by cfg.Bucket. It refuses to create the bucket
// against anything but a loopback endpoint, since CreateBucket exists for
// disposable test backends, not for provisioning production infrastructure.
func NewS3(ctx context.Context, cfg S3Config) (*S3, error) {
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("s3 blob store: bucket is required")
	}
	if cfg.CreateBucket {
		if err := requireLoopbackEndpoint(cfg.Endpoint); err != nil {
			return nil, err
		}
	}

	var opts []func(*config.LoadOptions) error
	if cfg.Region != "" {
		opts = append(opts, config.WithRegion(cfg.Region))
	}
	if cfg.AccessKeyID != "" || cfg.SecretAccessKey != "" {
		opts = append(opts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		))
	}
	awsCfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("s3 blob store: load aws config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
		o.UsePathStyle = cfg.PathStyle
	})
	st := &S3{client: client, presign: s3.NewPresignClient(client), bucket: cfg.Bucket}

	if cfg.CreateBucket {
		if err := st.ensureBucket(ctx); err != nil {
			return nil, err
		}
	}
	return st, nil
}

// requireLoopbackEndpoint rejects any endpoint that is not explicitly
// loopback, so CreateBucket cannot be pointed at a real object store by a
// config mistake.
func requireLoopbackEndpoint(endpoint string) error {
	if endpoint == "" {
		return fmt.Errorf("s3 blob store: CreateBucket requires a loopback Endpoint, got none")
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("s3 blob store: parse endpoint %q: %w", endpoint, err)
	}
	host := u.Hostname()
	if host == "localhost" {
		return nil
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return nil
	}
	return fmt.Errorf("s3 blob store: CreateBucket refuses non-loopback endpoint %q", endpoint)
}

func (s *S3) ensureBucket(ctx context.Context) error {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: &s.bucket})
	if err == nil {
		return nil
	}
	if !isNotFoundErr(err) {
		return fmt.Errorf("s3 blob store: head bucket %q: %w", s.bucket, err)
	}
	if _, err := s.client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: &s.bucket}); err != nil {
		var owned *types.BucketAlreadyOwnedByYou
		if errors.As(err, &owned) {
			return nil
		}
		return fmt.Errorf("s3 blob store: create bucket %q: %w", s.bucket, err)
	}
	return nil
}

// Put is idempotent for a key that already holds size bytes: HeadObject
// reports the same size and Put returns without touching the network again.
// The declared size is enforced against what r actually yields before any
// PutObject call, so a short or long read never reaches the bucket as a
// wrongly sized object.
func (s *S3) Put(ctx context.Context, key string, r io.Reader, size int64, mimeType string) error {
	k, err := s3Key(key)
	if err != nil {
		return err
	}
	existing, err := s.headInfo(ctx, k)
	if err == nil {
		if existing.Size == size {
			return nil
		}
	} else if !errors.Is(err, ErrNotFound) {
		return err
	}

	// A buffer, not a streamed body: it lets PutObject seek for its SHA256
	// checksum, and the size cap this backend is used under (25 MiB) makes
	// buffering cheap enough that streaming is not worth the complexity.
	buf, err := io.ReadAll(io.LimitReader(r, size+1))
	if err != nil {
		return fmt.Errorf("s3 blob store: read blob %q: %w", key, err)
	}
	if int64(len(buf)) != size {
		return fmt.Errorf("s3 blob store: blob %q: read %d bytes, want %d", key, len(buf), size)
	}

	if _, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:            &s.bucket,
		Key:               &k,
		Body:              bytes.NewReader(buf),
		ContentType:       aws.String(mimeType),
		ContentLength:     aws.Int64(size),
		ChecksumAlgorithm: types.ChecksumAlgorithmSha256,
	}); err != nil {
		return fmt.Errorf("s3 blob store: put blob %q: %w", key, err)
	}
	return nil
}

// Open maps a missing key to ErrNotFound so callers do not need to know this
// backend's own not-found error types.
func (s *S3) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	k, err := s3Key(key)
	if err != nil {
		return nil, err
	}
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: &s.bucket, Key: &k})
	if err != nil {
		if isNotFoundErr(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("s3 blob store: get blob %q: %w", key, err)
	}
	return out.Body, nil
}

// Stat reports the same Info a completed List would for this key, without
// listing the rest of the bucket.
func (s *S3) Stat(ctx context.Context, key string) (Info, error) {
	k, err := s3Key(key)
	if err != nil {
		return Info{}, err
	}
	return s.headInfo(ctx, k)
}

func (s *S3) headInfo(ctx context.Context, k string) (Info, error) {
	out, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: &s.bucket, Key: &k})
	if err != nil {
		if isNotFoundErr(err) {
			return Info{}, ErrNotFound
		}
		return Info{}, fmt.Errorf("s3 blob store: head blob %q: %w", k, err)
	}
	return Info{Key: k, Size: aws.ToInt64(out.ContentLength), LastModified: aws.ToTime(out.LastModified)}, nil
}

// Delete is idempotent: DeleteObject on a key that is already gone still
// reports success, which is what lets gc retry a delete without ceremony.
func (s *S3) Delete(ctx context.Context, key string) error {
	k, err := s3Key(key)
	if err != nil {
		return err
	}
	if _, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: &s.bucket, Key: &k}); err != nil {
		return fmt.Errorf("s3 blob store: delete blob %q: %w", key, err)
	}
	return nil
}

// List yields every blob under prefix in the order ListObjectsV2 pages them.
// An empty prefix lists the whole bucket. It stops and returns fn's own
// error unchanged on the first failure, so a caller can use a sentinel to
// stop early without List wrapping it into something errors.Is can't see.
func (s *S3) List(ctx context.Context, prefix string, fn func(Info) error) error {
	p := ""
	if trimmed := strings.TrimSuffix(prefix, "/"); trimmed != "" {
		clean, err := s3Key(trimmed)
		if err != nil {
			return fmt.Errorf("s3 blob store: list prefix %q: %w", prefix, err)
		}
		p = clean + "/"
	}

	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: &s.bucket,
		Prefix: &p,
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("s3 blob store: list blobs under %q: %w", prefix, err)
		}
		for _, obj := range page.Contents {
			info := Info{
				Key:          aws.ToString(obj.Key),
				Size:         aws.ToInt64(obj.Size),
				LastModified: aws.ToTime(obj.LastModified),
			}
			if err := fn(info); err != nil {
				return err
			}
		}
	}
	return nil
}

// URL presigns a GetObject request valid for ttl. ResponseContentDisposition
// carries filename so a browser downloads the attachment under its original
// name rather than its content-hash key.
func (s *S3) URL(ctx context.Context, key string, ttl time.Duration, filename string) (string, error) {
	k, err := s3Key(key)
	if err != nil {
		return "", err
	}
	disposition := contentDisposition(filename)
	req, err := s.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket:                     &s.bucket,
		Key:                        &k,
		ResponseContentDisposition: &disposition,
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", fmt.Errorf("s3 blob store: presign blob %q: %w", key, err)
	}
	return req.URL, nil
}

// contentDisposition builds an "attachment" Content-Disposition value for
// name. mime.FormatMediaType is the sanitizer: a value containing a quote,
// backslash, control character (including CR/LF) or non-ASCII byte cannot
// be written into the header literally, so FormatMediaType either quotes
// and escapes it or switches to the RFC 2231 filename*=utf-8”... form,
// which percent-encodes every byte that would otherwise be unsafe.
func contentDisposition(name string) string {
	if name == "" {
		name = fallbackAttachmentDownloadName
	}
	if disposition := mime.FormatMediaType("attachment", map[string]string{"filename": name}); disposition != "" {
		return disposition
	}
	if disposition := mime.FormatMediaType("attachment", map[string]string{"filename": fallbackAttachmentDownloadName}); disposition != "" {
		return disposition
	}
	return fallbackAttachmentDownloadName
}

// s3Key runs key through the same clean-relative-path check Local uses, so
// an untrusted blob_key is rejected identically by both backends, then
// renders it back to the forward slashes S3 keys always use regardless of
// the host OS.
func s3Key(key string) (string, error) {
	clean, err := cleanRelPath(key)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(clean), nil
}

// isNotFoundErr classifies a not-found condition by error type or, failing
// that, by the HTTP status code the service actually returned, never by
// matching text in an error message.
func isNotFoundErr(err error) bool {
	var noSuchKey *types.NoSuchKey
	if errors.As(err, &noSuchKey) {
		return true
	}
	var notFound *types.NotFound
	if errors.As(err, &notFound) {
		return true
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) && apiErr.ErrorCode() == "NotFound" {
		return true
	}
	var respErr *smithyhttp.ResponseError
	if errors.As(err, &respErr) && respErr.HTTPStatusCode() == http.StatusNotFound {
		return true
	}
	return false
}
