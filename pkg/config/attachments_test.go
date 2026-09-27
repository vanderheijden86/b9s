package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeConfigAndLoad(t *testing.T, content string) (Config, error) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return LoadFrom(path)
}

func TestAttachments_AbsentSectionIsNotConfigured(t *testing.T) {
	cfg, err := writeConfigAndLoad(t, "ui:\n  default_view: tree\n")
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if cfg.Attachments != nil {
		t.Fatalf("Attachments = %+v, want nil for a config with no attachments section", cfg.Attachments)
	}
}

func TestAttachments_Defaults(t *testing.T) {
	cfg, err := writeConfigAndLoad(t, "attachments:\n  backend: local\n")
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if cfg.Attachments == nil {
		t.Fatal("Attachments = nil, want a configured section")
	}
	if got := cfg.Attachments.MaxBytesOrDefault(); got != DefaultAttachmentMaxBytes {
		t.Errorf("MaxBytesOrDefault() = %d, want %d", got, DefaultAttachmentMaxBytes)
	}
	if got := cfg.Attachments.GCGraceOrDefault(); got != DefaultAttachmentGCGrace {
		t.Errorf("GCGraceOrDefault() = %s, want %s", got, DefaultAttachmentGCGrace)
	}
	if got := cfg.Attachments.URLTTLOrDefault(); got != DefaultAttachmentURLTTL {
		t.Errorf("URLTTLOrDefault() = %s, want %s", got, DefaultAttachmentURLTTL)
	}
}

func TestAttachments_FullyPopulated(t *testing.T) {
	content := `
attachments:
  backend: s3
  max_bytes: 1048576
  gc_grace: 48h
  local_dir: /tmp/should-be-ignored-for-s3
  s3:
    endpoint: https://nbg1.your-objectstorage.com
    region: nbg1
    bucket: osenco-beads-attachments
    prefix: osenco
    path_style: true
    url_ttl: 5m
    credential_command: "print-creds.sh"
`
	cfg, err := writeConfigAndLoad(t, content)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	a := cfg.Attachments
	if a == nil {
		t.Fatal("Attachments = nil")
	}
	if a.Backend != "s3" {
		t.Errorf("Backend = %q, want s3", a.Backend)
	}
	if a.MaxBytesOrDefault() != 1048576 {
		t.Errorf("MaxBytesOrDefault() = %d, want 1048576", a.MaxBytesOrDefault())
	}
	if a.GCGraceOrDefault() != 48*time.Hour {
		t.Errorf("GCGraceOrDefault() = %s, want 48h", a.GCGraceOrDefault())
	}
	if a.URLTTLOrDefault() != 5*time.Minute {
		t.Errorf("URLTTLOrDefault() = %s, want 5m", a.URLTTLOrDefault())
	}
	if a.S3.Endpoint != "https://nbg1.your-objectstorage.com" {
		t.Errorf("S3.Endpoint = %q", a.S3.Endpoint)
	}
	if a.S3.Region != "nbg1" || a.S3.Bucket != "osenco-beads-attachments" || a.S3.Prefix != "osenco" {
		t.Errorf("S3 = %+v", a.S3)
	}
	if !a.S3.PathStyle {
		t.Error("S3.PathStyle = false, want true")
	}
	if a.S3.CredentialCommand != "print-creds.sh" {
		t.Errorf("S3.CredentialCommand = %q", a.S3.CredentialCommand)
	}
}

func TestAttachments_InvalidBackend(t *testing.T) {
	_, err := writeConfigAndLoad(t, "attachments:\n  backend: ftp\n")
	if err == nil || !strings.Contains(err.Error(), "backend") {
		t.Fatalf("LoadFrom error = %v, want an error naming the backend", err)
	}
}

func TestAttachments_MissingBackend(t *testing.T) {
	_, err := writeConfigAndLoad(t, "attachments:\n  max_bytes: 100\n")
	if err == nil || !strings.Contains(err.Error(), "backend") {
		t.Fatalf("LoadFrom error = %v, want an error naming the backend", err)
	}
}

func TestAttachments_NegativeMaxBytes(t *testing.T) {
	_, err := writeConfigAndLoad(t, "attachments:\n  backend: local\n  max_bytes: -1\n")
	if err == nil || !strings.Contains(err.Error(), "max_bytes") {
		t.Fatalf("LoadFrom error = %v, want an error naming max_bytes", err)
	}
}

func TestAttachments_BadDuration(t *testing.T) {
	_, err := writeConfigAndLoad(t, "attachments:\n  backend: local\n  gc_grace: nope\n")
	if err == nil || !strings.Contains(err.Error(), "duration") {
		t.Fatalf("LoadFrom error = %v, want an error naming a bad duration", err)
	}
}

func TestAttachments_NegativeDuration(t *testing.T) {
	_, err := writeConfigAndLoad(t, "attachments:\n  backend: local\n  s3:\n    url_ttl: -5m\n")
	if err == nil || !strings.Contains(err.Error(), "negative") {
		t.Fatalf("LoadFrom error = %v, want an error rejecting a negative duration", err)
	}
}

func TestAttachments_RejectsSecretKeys(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{"top-level access_key_id", "attachments:\n  backend: s3\n  access_key_id: AKIA...\n"},
		{"top-level secret", "attachments:\n  backend: s3\n  secret: shh\n"},
		{"nested secret_access_key", "attachments:\n  backend: s3\n  s3:\n    secret_access_key: shh\n"},
		{"nested password", "attachments:\n  backend: s3\n  s3:\n    password: shh\n"},
		{"nested token, hyphenated", "attachments:\n  backend: s3\n  s3:\n    access-key-id: AKIA...\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := writeConfigAndLoad(t, c.content)
			if err == nil {
				t.Fatal("LoadFrom succeeded, want a rejected secret key")
			}
			if !strings.Contains(err.Error(), "B9S_ATTACHMENTS_S3_ACCESS_KEY_ID") {
				t.Errorf("error = %v, want it to name the env var alternative", err)
			}
		})
	}
}
