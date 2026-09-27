package config

import (
	"fmt"
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
)

// Duration is a YAML duration that parses with time.ParseDuration and
// rejects a negative value. Unlike RefreshInterval it carries no minimum,
// since zero is meaningful here: it means "use this package's default"
// rather than "invalid".
type Duration time.Duration

func (d *Duration) UnmarshalYAML(node *yaml.Node) error {
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
type AttachmentsConfig struct {
	Backend  string              `yaml:"backend"`
	MaxBytes int64               `yaml:"max_bytes,omitempty"`
	GCGrace  Duration            `yaml:"gc_grace,omitempty"`
	LocalDir string              `yaml:"local_dir,omitempty"`
	S3       S3AttachmentsConfig `yaml:"s3,omitempty"`
}

// S3AttachmentsConfig configures the S3-compatible backend. It carries no
// credential fields: rejectAttachmentSecretKeys refuses a YAML key that
// looks like one anywhere in the attachments section, so a secret can only
// come from the environment or CredentialCommand.
type S3AttachmentsConfig struct {
	Endpoint          string   `yaml:"endpoint,omitempty"`
	Region            string   `yaml:"region,omitempty"`
	Bucket            string   `yaml:"bucket,omitempty"`
	Prefix            string   `yaml:"prefix,omitempty"`
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

// attachmentSecretKeys names the YAML keys that must never appear under
// attachments, because they hold a credential rather than settings.
var attachmentSecretKeys = map[string]bool{
	"access_key_id":     true,
	"secret_access_key": true,
	"secret":            true,
	"password":          true,
	"token":             true,
}

// UnmarshalYAML rejects a credential typed directly into the file before
// decoding the rest of the section, then validates Backend and MaxBytes so a
// bad config fails at Load rather than at first use.
func (a *AttachmentsConfig) UnmarshalYAML(node *yaml.Node) error {
	if err := rejectAttachmentSecretKeys(node); err != nil {
		return err
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
	*a = AttachmentsConfig(decoded)
	return nil
}

// rejectAttachmentSecretKeys walks node and every mapping nested under it
// (the s3 block especially) for a key that names a credential. A key typed
// into the file would put a secret in plain text on disk and in whatever
// backs it up, so attachments secrets come only from the environment or
// credential_command.
func rejectAttachmentSecretKeys(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		keyNode, valNode := node.Content[i], node.Content[i+1]
		normalized := strings.ToLower(strings.ReplaceAll(keyNode.Value, "-", "_"))
		if attachmentSecretKeys[normalized] {
			return fmt.Errorf(
				"attachments config must not set %q; use B9S_ATTACHMENTS_S3_ACCESS_KEY_ID and B9S_ATTACHMENTS_S3_SECRET_ACCESS_KEY, or attachments.s3.credential_command",
				keyNode.Value)
		}
		if err := rejectAttachmentSecretKeys(valNode); err != nil {
			return err
		}
	}
	return nil
}
