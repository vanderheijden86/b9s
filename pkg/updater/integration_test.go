package updater

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================================================
// Full update flow integration tests
// ============================================================================

func TestFullUpdateFlow_WithMockServer(t *testing.T) {
	// Create a mock GitHub API and download server
	binaryContent := []byte("#!/bin/sh\necho 'mock b9s v99.0.0'")

	// Create tar.gz archive
	var archiveBuf bytes.Buffer
	gzw := gzip.NewWriter(&archiveBuf)
	tw := tar.NewWriter(gzw)

	hdr := &tar.Header{
		Name: "b9s",
		Mode: 0o755,
		Size: int64(len(binaryContent)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("write tar header: %v", err)
	}
	if _, err := tw.Write(binaryContent); err != nil {
		t.Fatalf("write tar content: %v", err)
	}
	tw.Close()
	gzw.Close()
	archiveBytes := archiveBuf.Bytes()

	// Calculate checksum
	h := sha256.New()
	h.Write(archiveBytes)
	archiveHash := hex.EncodeToString(h.Sum(nil))
	assetName := getAssetName("v99.0.0")
	checksumContent := fmt.Sprintf("%s  %s\n", archiveHash, assetName)

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "releases/latest"):
			release := Release{
				TagName: "v99.0.0",
				HTMLURL: "http://example.com/release",
				Assets: []Asset{
					{
						Name:               assetName,
						BrowserDownloadURL: "http://localhost/archive",
						Size:               int64(len(archiveBytes)),
					},
					{
						Name:               "checksums.txt",
						BrowserDownloadURL: "http://localhost/checksums",
						Size:               int64(len(checksumContent)),
					},
				},
			}
			json.NewEncoder(w).Encode(release)

		case strings.Contains(r.URL.Path, "archive"):
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(archiveBytes)))
			w.Write(archiveBytes)

		case strings.Contains(r.URL.Path, "checksums"):
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(checksumContent)))
			w.Write([]byte(checksumContent))

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	// Test that the mocked release can be parsed
	client := server.Client()
	tag, url, err := checkForUpdates(client, server.URL+"/releases/latest")
	if err != nil {
		t.Fatalf("checkForUpdates failed: %v", err)
	}

	if tag != "v99.0.0" {
		t.Errorf("expected tag v99.0.0, got %s", tag)
	}
	if url != "http://example.com/release" {
		t.Errorf("expected release URL, got %s", url)
	}
}

// TestCheckUpdateAvailable_NoUpdateNeeded verifies no update when remote is older
func TestCheckUpdateAvailable_NoUpdateNeeded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		release := Release{
			TagName: "v0.0.1", // Lower than any real version
			HTMLURL: "http://example.com/release",
		}
		json.NewEncoder(w).Encode(release)
	}))
	defer server.Close()

	client := server.Client()
	tag, _, err := checkForUpdates(client, server.URL)
	if err != nil {
		t.Fatalf("checkForUpdates failed: %v", err)
	}

	// Should be empty (no update) since v0.0.1 < current version
	if tag != "" {
		t.Errorf("expected no update (empty tag), got %s", tag)
	}
}

// ============================================================================
// Edge cases for extraction
// ============================================================================

func TestExtractBinary_NoBinaryInArchive(t *testing.T) {
	// Create archive without the b9s binary
	var archiveBuf bytes.Buffer
	gzw := gzip.NewWriter(&archiveBuf)
	tw := tar.NewWriter(gzw)

	hdr := &tar.Header{
		Name: "other_file.txt",
		Mode: 0o644,
		Size: 5,
	}
	tw.WriteHeader(hdr)
	tw.Write([]byte("hello"))
	tw.Close()
	gzw.Close()

	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "archive.tar.gz")
	destPath := filepath.Join(tmpDir, "b9s")

	if err := os.WriteFile(archivePath, archiveBuf.Bytes(), 0o644); err != nil {
		t.Fatalf("write archive: %v", err)
	}

	err := extractBinary(archivePath, destPath)
	if err == nil {
		t.Fatal("expected error when binary not found in archive")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestExtractBinary_NestedPath(t *testing.T) {
	// Create archive with a nested b9s binary
	content := []byte("nested binary")
	var archiveBuf bytes.Buffer
	gzw := gzip.NewWriter(&archiveBuf)
	tw := tar.NewWriter(gzw)

	hdr := &tar.Header{
		Name: "some/nested/path/b9s",
		Mode: 0o755,
		Size: int64(len(content)),
	}
	tw.WriteHeader(hdr)
	tw.Write(content)
	tw.Close()
	gzw.Close()

	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "archive.tar.gz")
	destPath := filepath.Join(tmpDir, "b9s")

	if err := os.WriteFile(archivePath, archiveBuf.Bytes(), 0o644); err != nil {
		t.Fatalf("write archive: %v", err)
	}

	// Should find b9s even in a nested path
	err := extractBinary(archivePath, destPath)
	if err != nil {
		t.Fatalf("extractBinary failed: %v", err)
	}

	got, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("read extracted: %v", err)
	}

	if !bytes.Equal(got, content) {
		t.Errorf("content mismatch: got %q, want %q", got, content)
	}
}

func TestValidateBinaryArchiveEntryRejectsOversizedBinary(t *testing.T) {
	header := &tar.Header{Name: "b9s", Typeflag: tar.TypeReg, Size: maxExtractedBinarySize + 1}
	if err := validateBinaryArchiveEntry(header); err == nil {
		t.Fatal("expected oversized release binary to be rejected")
	}
}

func TestValidateBinaryArchiveEntryRejectsSymlink(t *testing.T) {
	header := &tar.Header{Name: "b9s", Typeflag: tar.TypeSymlink, Linkname: "/tmp/payload"}
	if err := validateBinaryArchiveEntry(header); err == nil {
		t.Fatal("expected symlink release entry to be rejected")
	}
}

func TestReleaseConfigurationPinsActionsAndEmitsProvenance(t *testing.T) {
	workflow, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "release.yml"))
	if err != nil {
		t.Fatalf("read release workflow: %v", err)
	}
	ciWorkflow, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatalf("read CI workflow: %v", err)
	}
	config, err := os.ReadFile(filepath.Join("..", "..", ".goreleaser.yaml"))
	if err != nil {
		t.Fatalf("read GoReleaser config: %v", err)
	}

	workflowText := string(workflow) + string(ciWorkflow)
	for _, mutableRef := range []string{"actions/checkout@v4", "actions/setup-go@v6", "goreleaser/goreleaser-action@v6"} {
		if strings.Contains(workflowText, mutableRef) {
			t.Fatalf("release workflow still uses mutable action ref %q", mutableRef)
		}
	}
	for _, currentPinnedRef := range []string{
		"actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1",
		"actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0",
		"goreleaser/goreleaser-action@f06c13b6b1a9625abc9e6e439d9c05a8f2190e94 # v7.2.3",
		"actions/attest-build-provenance@4d101475d8b20a2381f78447822ac1eab6504dd8 # v4.2.2",
	} {
		if !strings.Contains(workflowText, currentPinnedRef) {
			t.Fatalf("workflow missing current pinned action %q", currentPinnedRef)
		}
	}
	for _, required := range []string{"actions/attest-build-provenance@", "attestations: write", "id-token: write", "subject-path:"} {
		if !strings.Contains(workflowText, required) {
			t.Fatalf("release workflow missing %q", required)
		}
	}

	configText := string(config)
	if !strings.Contains(configText, "-trimpath") {
		t.Fatal("GoReleaser build does not remove local build paths")
	}
	if !strings.Contains(configText, "https://github.com/vanderheijden86/b9s") {
		t.Fatal("GoReleaser metadata still points at the old repository identity")
	}
}

func TestDebugExecutablesAreIgnored(t *testing.T) {
	ignore, err := os.ReadFile(filepath.Join("..", "..", ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	text := string(ignore)
	for _, artifact := range []string{"/bv_profile\n", "/bv_test\n"} {
		if !strings.Contains(text, artifact) {
			t.Fatalf(".gitignore does not prevent regenerated artifact %q", strings.TrimSpace(artifact))
		}
	}
}

func TestInstallersUseCurrentIdentityAndRequireChecksums(t *testing.T) {
	installShell, err := os.ReadFile(filepath.Join("..", "..", "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}
	installPowerShell, err := os.ReadFile(filepath.Join("..", "..", "install.ps1"))
	if err != nil {
		t.Fatalf("read install.ps1: %v", err)
	}

	shellText := string(installShell)
	for _, required := range []string{`REPO_OWNER="vanderheijden86"`, `BIN_NAME="b9s"`, "checksums.txt", "verify_checksum"} {
		if !strings.Contains(shellText, required) {
			t.Fatalf("install.sh missing %q", required)
		}
	}
	for _, installer := range []string{shellText, string(installPowerShell)} {
		if strings.Contains(installer, "Dicklesworthstone") {
			t.Fatal("installer still points at the previous repository owner")
		}
	}
	if !strings.Contains(string(installPowerShell), `$BIN_NAME = "b9s"`) {
		t.Fatal("install.ps1 still installs the previous binary name")
	}
}

// TestDownloadFile_404_ReturnsError verifies HTTP error handling
func TestDownloadFile_404_ReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "download.bin")

	err := downloadFile(server.URL, destPath, 0)
	if err == nil {
		t.Fatal("expected error for 404 response")
	}

	if !strings.Contains(err.Error(), "404") && !strings.Contains(err.Error(), "Not Found") {
		t.Errorf("expected 404 error, got: %v", err)
	}
}

// TestParseChecksums_EmptyLinesIgnored verifies empty line handling
func TestParseChecksums_EmptyLinesIgnored(t *testing.T) {
	tmpDir := t.TempDir()
	checksumPath := filepath.Join(tmpDir, "checksums.txt")

	content := `abc123  file1.tar.gz

def456  file2.tar.gz

ghi789  file3.tar.gz`

	if err := os.WriteFile(checksumPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write checksums: %v", err)
	}

	checksums, err := parseChecksums(checksumPath)
	if err != nil {
		t.Fatalf("parseChecksums failed: %v", err)
	}

	if len(checksums) != 3 {
		t.Errorf("expected 3 checksums (ignoring empty lines), got %d", len(checksums))
	}
}

// TestGetAssetName_HandlesVersionWithoutV verifies version without v prefix
func TestGetAssetName_HandlesVersionWithoutV(t *testing.T) {
	name := getAssetName("2.0.0")
	if !strings.HasPrefix(name, "b9s_2.0.0_") {
		t.Errorf("expected asset name to start with b9s_2.0.0_, got %s", name)
	}
}

// TestRelease_FindPlatformAsset_NoAssets verifies empty asset list handling
func TestRelease_FindPlatformAsset_NoAssets(t *testing.T) {
	rel := Release{
		TagName: "v1.0.0",
		Assets:  []Asset{},
	}

	asset := rel.FindPlatformAsset()
	if asset != nil {
		t.Errorf("expected nil for empty assets, got %+v", asset)
	}
}
