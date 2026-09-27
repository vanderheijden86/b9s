package config

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Default attachment settings, applied whenever the corresponding YAML key
// is absent or left at zero.
const (
	DefaultAttachmentMaxBytes = 25 * 1024 * 1024 // 25 MiB
	DefaultAttachmentGCGrace  = 24 * time.Hour
	DefaultAttachmentURLTTL   = 15 * time.Minute
	// maxAttachmentURLTTL is the longest lifetime S3's SigV4 presign scheme
	// accepts; a longer value fails at request time on a real bucket, so
	// UnmarshalYAML rejects it at load time instead.
	maxAttachmentURLTTL = 7 * 24 * time.Hour
	// maxAttachmentMaxBytes caps attachments.max_bytes at the largest object
	// a single S3 PUT accepts; a file above it would need multipart upload,
	// which this package's S3 client does not implement.
	maxAttachmentMaxBytes = 5 * 1024 * 1024 * 1024 // 5 GiB
)

// Duration is a YAML duration that parses with time.ParseDuration and
// rejects a negative value. Unlike RefreshInterval it carries no minimum,
// since zero is meaningful here: it means "use this package's default"
// rather than "invalid".
type Duration time.Duration

func (d *Duration) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.ScalarNode {
		return fmt.Errorf("duration must be a YAML scalar, not %v", node.Kind)
	}
	trimmed := strings.TrimSpace(node.Value)
	if trimmed == "" {
		*d = 0
		return nil
	}
	parsed, err := time.ParseDuration(trimmed)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", node.Value, err)
	}
	if parsed < 0 {
		return fmt.Errorf("duration %q must not be negative", node.Value)
	}
	*d = Duration(parsed)
	return nil
}

func (d Duration) MarshalYAML() (any, error) {
	return time.Duration(d).String(), nil
}

// AttachmentsConfig configures the blob store b9s uses for issue
// attachments. See Config.Attachments for what a nil value means.
//
// Prefix lives here, not under S3, because it names the workspace segment of
// a blob key and applies identically to the local backend: two projects
// sharing one local_dir still need separate prefixes to avoid colliding on
// the same database name.
type AttachmentsConfig struct {
	Backend  string   `yaml:"backend"`
	MaxBytes int64    `yaml:"max_bytes,omitempty"`
	GCGrace  Duration `yaml:"gc_grace,omitempty"`
	LocalDir string   `yaml:"local_dir,omitempty"`
	Prefix   string   `yaml:"prefix,omitempty"`
	// LocalWithDoltServer opts into the local backend for a project whose
	// data source is a Dolt server. A Dolt server reached through an SSH
	// tunnel looks like loopback locally even when it is shared with other
	// operators, so a host-based check cannot tell a solo setup from a
	// shared one; this flag makes the operator say which it is.
	LocalWithDoltServer bool                `yaml:"local_with_dolt_server,omitempty"`
	S3                  S3AttachmentsConfig `yaml:"s3,omitempty"`
}

// S3AttachmentsConfig configures the S3-compatible backend. It carries no
// credential fields: validateAttachmentsKeys accepts only the fields below
// under attachments.s3, so a secret typed into the file is rejected by name
// rather than matched against a list of things to block. A secret can only
// come from the environment or CredentialCommand.
type S3AttachmentsConfig struct {
	Endpoint          string   `yaml:"endpoint,omitempty"`
	Region            string   `yaml:"region,omitempty"`
	Bucket            string   `yaml:"bucket,omitempty"`
	PathStyle         bool     `yaml:"path_style,omitempty"`
	URLTTL            Duration `yaml:"url_ttl,omitempty"`
	CredentialCommand string   `yaml:"credential_command,omitempty"`
}

// MaxBytesOrDefault returns the configured cap, or DefaultAttachmentMaxBytes
// when the file left it at zero.
func (a AttachmentsConfig) MaxBytesOrDefault() int64 {
	if a.MaxBytes == 0 {
		return DefaultAttachmentMaxBytes
	}
	return a.MaxBytes
}

// GCGraceOrDefault returns the configured grace period, or
// DefaultAttachmentGCGrace when the file left it unset.
func (a AttachmentsConfig) GCGraceOrDefault() time.Duration {
	if a.GCGrace == 0 {
		return DefaultAttachmentGCGrace
	}
	return time.Duration(a.GCGrace)
}

// URLTTLOrDefault returns the configured presigned URL lifetime, or
// DefaultAttachmentURLTTL when the file left it unset.
func (a AttachmentsConfig) URLTTLOrDefault() time.Duration {
	if a.S3.URLTTL == 0 {
		return DefaultAttachmentURLTTL
	}
	return time.Duration(a.S3.URLTTL)
}

// attachmentsAllowedKeys names every field YAML may set directly under
// attachments. A key not in this set is rejected by name: an allowlist,
// unlike a list of known secret names, cannot be defeated by a credential
// field the list's author did not think to name (aws_secret_access_key,
// secretAccessKey, session_token, ...).
var attachmentsAllowedKeys = map[string]bool{
	"backend":                true,
	"max_bytes":              true,
	"gc_grace":               true,
	"local_dir":              true,
	"prefix":                 true,
	"local_with_dolt_server": true,
	"s3":                     true,
}

// attachmentsS3AllowedKeys names every field YAML may set under
// attachments.s3. See attachmentsAllowedKeys.
var attachmentsS3AllowedKeys = map[string]bool{
	"endpoint":           true,
	"region":             true,
	"bucket":             true,
	"path_style":         true,
	"url_ttl":            true,
	"credential_command": true,
}

// UnmarshalYAML rejects a YAML alias or merge key anywhere under attachments,
// then an unknown field under attachments or attachments.s3, before decoding
// the rest of the section and validating Backend, MaxBytes, the S3 endpoint
// and URLTTL, so a bad config fails at Load rather than at first use.
func (a *AttachmentsConfig) UnmarshalYAML(node *yaml.Node) error {
	if err := rejectAttachmentAliasOrMerge(node); err != nil {
		return err
	}
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("attachments must be a mapping")
	}
	if err := rejectUnknownAttachmentKeys(node, attachmentsAllowedKeys, "attachments"); err != nil {
		return err
	}
	if s3Node := mappingValue(node, "s3"); s3Node != nil {
		if s3Node.Kind != yaml.MappingNode {
			return fmt.Errorf("attachments.s3 must be a mapping")
		}
		if err := rejectUnknownAttachmentKeys(s3Node, attachmentsS3AllowedKeys, "attachments.s3"); err != nil {
			return err
		}
	}

	type plain AttachmentsConfig
	var decoded plain
	if err := node.Decode(&decoded); err != nil {
		return err
	}
	switch decoded.Backend {
	case "s3", "local":
	default:
		return fmt.Errorf("invalid attachments.backend %q: want s3 or local", decoded.Backend)
	}
	if decoded.MaxBytes < 0 {
		return fmt.Errorf("attachments.max_bytes must not be negative")
	}
	if decoded.MaxBytes > maxAttachmentMaxBytes {
		return fmt.Errorf("attachments.max_bytes %d exceeds the 5 GiB S3 single PUT limit", decoded.MaxBytes)
	}
	if time.Duration(decoded.S3.URLTTL) > maxAttachmentURLTTL {
		return fmt.Errorf("attachments.s3.url_ttl %s exceeds the 7 day S3 presign limit", time.Duration(decoded.S3.URLTTL))
	}
	if decoded.S3.Endpoint != "" {
		if err := validateAttachmentEndpoint(decoded.S3.Endpoint); err != nil {
			return err
		}
	}
	*a = AttachmentsConfig(decoded)
	return nil
}

// validateAttachmentEndpoint rejects anything url.Parse cannot make sense of,
// a scheme other than http or https, an empty host, or embedded userinfo. The
// endpoint is never included in a returned error, quoted or otherwise: a
// malformed or userinfo-bearing endpoint can carry the credential it names in
// the very string that would be echoed back.
func validateAttachmentEndpoint(endpoint string) error {
	invalid := fmt.Errorf("attachments.s3.endpoint must be an http or https URL with a host and no embedded username or password; use B9S_ATTACHMENTS_S3_ACCESS_KEY_ID and B9S_ATTACHMENTS_S3_SECRET_ACCESS_KEY, or attachments.s3.credential_command")
	u, err := url.Parse(endpoint)
	if err != nil {
		return invalid
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return invalid
	}
	if u.Hostname() == "" {
		return invalid
	}
	if u.User != nil {
		return invalid
	}
	return nil
}

// mappingValue returns the value node for key in mapping node, or nil if
// node is not a mapping or has no such key.
func mappingValue(node *yaml.Node, key string) *yaml.Node {
	if node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

// rejectUnknownAttachmentKeys fails on the first key of mapping node that is
// not in allowed, naming it and path so the error points at the exact
// setting to remove.
func rejectUnknownAttachmentKeys(node *yaml.Node, allowed map[string]bool, path string) error {
	for i := 0; i+1 < len(node.Content); i += 2 {
		key := node.Content[i]
		if !allowed[key.Value] {
			return fmt.Errorf("%s has unknown key %q", path, key.Value)
		}
	}
	return nil
}

// rejectAttachmentAliasOrMerge walks every node under node (mapping keys and
// values, sequence items) and fails on a YAML alias or a merge key (<<). Both
// let a value under attachments come from outside the attachments section
// itself, which would defeat rejectUnknownAttachmentKeys: a merged-in map or
// an aliased scalar never appears as a literal key in the section being
// checked, so the allowlist walk above would not see it.
func rejectAttachmentAliasOrMerge(node *yaml.Node) error {
	if node.Kind == yaml.AliasNode {
		return fmt.Errorf("attachments config must not use a YAML alias (*%s)", node.Value)
	}
	if node.Tag == "!!merge" {
		return fmt.Errorf("attachments config must not use a YAML merge key (<<)")
	}
	for _, child := range node.Content {
		if err := rejectAttachmentAliasOrMerge(child); err != nil {
			return err
		}
	}
	return nil
}
