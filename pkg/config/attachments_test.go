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
	content := "attachments:\n" +
		"  backend: s3\n" +
		"  max_bytes: 1048576\n" +
		"  gc_grace: 48h\n" +
		"  local_dir: /tmp/should-be-ignored-for-s3\n" +
		"  prefix: osenco\n" +
		"  s3:\n" +
		"    endpoint: https://nbg1.your-objectstorage.com\n" +
		"    region: nbg1\n" +
		"    bucket: osenco-beads-attachments\n" +
		"    path_style: true\n" +
		"    url_ttl: 5m\n" +
		"    credential_command: \"print-creds.sh\"\n"
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
	if a.Prefix != "osenco" {
		t.Errorf("Prefix = %q, want osenco", a.Prefix)
	}
	if a.S3.Endpoint != "https://nbg1.your-objectstorage.com" {
		t.Errorf("S3.Endpoint = %q", a.S3.Endpoint)
	}
	if a.S3.Region != "nbg1" || a.S3.Bucket != "osenco-beads-attachments" {
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

func TestAttachments_MaxBytesAboveFiveGiBIsAnError(t *testing.T) {
	_, err := writeConfigAndLoad(t, "attachments:\n  backend: local\n  max_bytes: 9223372036854775807\n")
	if err == nil || !strings.Contains(err.Error(), "max_bytes") || !strings.Contains(err.Error(), "5 GiB") {
		t.Fatalf("LoadFrom error = %v, want an error naming max_bytes and the 5 GiB limit", err)
	}
}

func TestAttachments_MaxBytesAtFiveGiBIsAllowed(t *testing.T) {
	cfg, err := writeConfigAndLoad(t, "attachments:\n  backend: local\n  max_bytes: 5368709120\n")
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if got := cfg.Attachments.MaxBytesOrDefault(); got != 5368709120 {
		t.Errorf("MaxBytesOrDefault() = %d, want 5368709120", got)
	}
}

func TestAttachments_BadDuration(t *testing.T) {
	_, err := writeConfigAndLoad(t, "attachments:\n  backend: local\n  gc_grace: nope\n")
	if err == nil || !strings.Contains(err.Error(), "duration") {
		t.Fatalf("LoadFrom error = %v, want an error naming a bad duration", err)
	}
}

func TestAttachments_DurationRejectsNonScalarNode(t *testing.T) {
	_, err := writeConfigAndLoad(t, "attachments:\n  backend: local\n  gc_grace: [1h, 2h]\n")
	if err == nil || !strings.Contains(err.Error(), "scalar") {
		t.Fatalf("LoadFrom error = %v, want an error naming a non-scalar duration node", err)
	}
}

func TestAttachments_NegativeDuration(t *testing.T) {
	_, err := writeConfigAndLoad(t, "attachments:\n  backend: local\n  s3:\n    url_ttl: -5m\n")
	if err == nil || !strings.Contains(err.Error(), "negative") {
		t.Fatalf("LoadFrom error = %v, want an error rejecting a negative duration", err)
	}
}

func TestAttachments_URLTTLAboveSevenDaysIsAnError(t *testing.T) {
	_, err := writeConfigAndLoad(t, "attachments:\n  backend: s3\n  s3:\n    url_ttl: 169h\n")
	if err == nil || !strings.Contains(err.Error(), "url_ttl") || !strings.Contains(err.Error(), "7 day") {
		t.Fatalf("LoadFrom error = %v, want an error naming url_ttl and the 7 day S3 presign limit", err)
	}
}

func TestAttachments_URLTTLAtSevenDaysIsAllowed(t *testing.T) {
	cfg, err := writeConfigAndLoad(t, "attachments:\n  backend: s3\n  s3:\n    url_ttl: 168h\n")
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if got := cfg.Attachments.URLTTLOrDefault(); got != 168*time.Hour {
		t.Errorf("URLTTLOrDefault() = %s, want 168h", got)
	}
}

func TestAttachments_RejectsUnknownKeys(t *testing.T) {
	cases := []struct {
		name      string
		content   string
		wantInErr string
	}{
		{
			"top-level unknown key",
			"attachments:\n  backend: s3\n  aws_secret_access_key: AKIA...\n",
			"aws_secret_access_key",
		},
		{
			"nested unknown key, camelCase",
			"attachments:\n  backend: s3\n  s3:\n    secretAccessKey: shh\n",
			"secretAccessKey",
		},
		{
			"nested unknown key",
			"attachments:\n  backend: s3\n  s3:\n    session_token: shh\n",
			"session_token",
		},
		{
			"unknown key whose value is a sequence",
			"attachments:\n  backend: s3\n  s3:\n    endpoint: https://x\n    bucket: b\n    extra:\n      - secret_access_key\n      - shh\n",
			"extra",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := writeConfigAndLoad(t, c.content)
			if err == nil {
				t.Fatal("LoadFrom succeeded, want a rejected unknown key")
			}
			if !strings.Contains(err.Error(), c.wantInErr) {
				t.Errorf("error = %v, want it to name %q", err, c.wantInErr)
			}
		})
	}
}

func TestAttachments_RejectsAliasAndMergeNodes(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{
			"s3 value is an alias",
			"shared: &c\n  bucket: evil\nattachments:\n  backend: s3\n  s3: *c\n",
		},
		{
			"merge key under s3",
			"shared: &c\n  bucket: evil\nattachments:\n  backend: s3\n  s3:\n    <<: *c\n    endpoint: https://x\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := writeConfigAndLoad(t, c.content)
			if err == nil {
				t.Fatal("LoadFrom succeeded, want a rejected alias or merge node")
			}
		})
	}
}

func TestAttachments_EndpointWithUserinfoIsRejected(t *testing.T) {
	_, err := writeConfigAndLoad(t, "attachments:\n  backend: s3\n  s3:\n    endpoint: https://AKIA:S@host\n")
	if err == nil {
		t.Fatal("LoadFrom succeeded, want an error rejecting userinfo in the endpoint")
	}
	if strings.Contains(err.Error(), "AKIA:S@host") || strings.Contains(err.Error(), "AKIA:S") {
		t.Fatalf("error = %v, want the endpoint never echoed back", err)
	}
}

func TestAttachments_InvalidEndpointsAreRejected(t *testing.T) {
	cases := []struct {
		name     string
		endpoint string
	}{
		{"userinfo without scheme", "AKIA:SECRET@host"},
		{"userinfo with unparsable path", "https://AKIA:SECRET@host/%zz"},
		{"non-http scheme", "ftp://host"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			content := "attachments:\n  backend: s3\n  s3:\n    endpoint: \"" + c.endpoint + "\"\n"
			_, err := writeConfigAndLoad(t, content)
			if err == nil {
				t.Fatalf("LoadFrom succeeded for endpoint %q, want it rejected", c.endpoint)
			}
			if strings.Contains(err.Error(), c.endpoint) {
				t.Fatalf("error = %v, want the endpoint never echoed back", err)
			}
		})
	}
}

func TestAttachments_ValidEndpointIsAccepted(t *testing.T) {
	cfg, err := writeConfigAndLoad(t, "attachments:\n  backend: s3\n  s3:\n    endpoint: https://nbg1.your-objectstorage.com\n")
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if cfg.Attachments.S3.Endpoint != "https://nbg1.your-objectstorage.com" {
		t.Errorf("S3.Endpoint = %q", cfg.Attachments.S3.Endpoint)
	}
}
