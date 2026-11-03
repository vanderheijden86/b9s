package updater

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
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
	// The release embeds the committed pkg/web/dist. The hook stops a
	// release whose bundle was built from other sources than the tag holds.
	if !strings.Contains(configText, "TestEmbeddedBundleIsCurrent") {
		t.Fatal("GoReleaser has no before-hook that checks the embedded web bundle")
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

// A tracked symlink with an absolute target publishes the author's workstation
// layout and resolves to nothing on any other checkout.
func TestNoTrackedSymlinkHasAbsoluteTarget(t *testing.T) {
	repoRoot := filepath.Join("..", "..")
	out, err := exec.Command("git", "-C", repoRoot, "ls-files", "-s").Output()
	if err != nil {
		t.Skipf("git ls-files unavailable: %v", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 || fields[0] != "120000" {
			continue
		}
		target, err := exec.Command("git", "-C", repoRoot, "cat-file", "blob", fields[1]).Output()
		if err != nil {
			t.Fatalf("read symlink blob for %s: %v", fields[3], err)
		}
		if filepath.IsAbs(string(target)) {
			t.Errorf("tracked symlink %s points to absolute path %q", fields[3], target)
		}
	}
}

// Generated exports and mockups copy the author's home directory into the
// published tree; vendored code is third-party and not under review here.
func TestNoTrackedFileContainsTheAuthorsHomeDirectory(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil || len(home) < 2 {
		t.Skip("no usable home directory")
	}
	repoRoot := filepath.Join("..", "..")
	out, err := exec.Command("git", "-C", repoRoot, "grep", "-lIF", home+string(filepath.Separator), "--", ".", ":!vendor").Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return
		}
		t.Skipf("git grep unavailable: %v", err)
	}
	t.Fatalf("tracked files contain the home directory %s:\n%s", home, out)
}

// homePathPattern matches a macOS or Linux home directory with its user name.
// The check above sees only the home of the machine running it, so on CI it
// cannot see the author's; this one sees every user's.
const homePathERE = `(/Users|/home)/([A-Za-z0-9._-]+)/`

var homePathPattern = regexp.MustCompile(homePathERE)

// placeholderHomeUsers are the user names the docs and tests use in example
// paths; they name no real machine.
var placeholderHomeUsers = map[string]bool{"me": true, "user": true, "dev": true, "runner": true}

// leakedHomePaths returns the home paths in text whose user name is not a
// placeholder.
func leakedHomePaths(text string) []string {
	var leaks []string
	for _, m := range homePathPattern.FindAllStringSubmatch(text, -1) {
		if !placeholderHomeUsers[m[2]] {
			leaks = append(leaks, m[0])
		}
	}
	return leaks
}

func TestLeakedHomePathsKeepsPlaceholders(t *testing.T) {
	// Joined at run time, so this file does not trip the repository check below.
	users, home := "/Users/", "/home/"
	text := "rig_root: " + users + "alice/gascity/rigs/b9s-alpha\npath: " + users + "me/src/b9s\n" + home + "user/file.txt " + home + "bob/x"
	got := leakedHomePaths(text)
	want := []string{users + "alice/", home + "bob/"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("leakedHomePaths = %v, want %v", got, want)
	}
}

// Beads exports under .beads are issue data, not published docs or code.
func TestNoTrackedFileContainsAnyUsersHomeDirectory(t *testing.T) {
	repoRoot := filepath.Join("..", "..")
	out, err := exec.Command("git", "-C", repoRoot, "grep", "-nIE", homePathERE, "--", ".", ":!vendor", ":!.beads").Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return
		}
		t.Skipf("git grep unavailable: %v", err)
	}
	var bad []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if len(leakedHomePaths(line)) > 0 {
			bad = append(bad, line)
		}
	}
	if len(bad) > 0 {
		t.Fatalf("tracked files contain a user's home directory:\n%s", strings.Join(bad, "\n"))
	}
}

// Compiled executables in the source tree go stale against the code and carry
// their build machine's paths and vulnerable dependencies.
func TestNoTrackedCompiledExecutables(t *testing.T) {
	repoRoot := filepath.Join("..", "..")
	out, err := exec.Command("git", "-C", repoRoot, "ls-files", "-s", "--", ".", ":!vendor").Output()
	if err != nil {
		t.Skipf("git ls-files unavailable: %v", err)
	}
	magics := [][]byte{
		{0x7f, 'E', 'L', 'F'},
		{0xcf, 0xfa, 0xed, 0xfe}, {0xce, 0xfa, 0xed, 0xfe},
		{0xca, 0xfe, 0xba, 0xbe},
		{'M', 'Z'},
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 || fields[0] == "120000" {
			continue
		}
		f, err := os.Open(filepath.Join(repoRoot, fields[3]))
		if err != nil {
			continue
		}
		head := make([]byte, 4)
		n, _ := f.Read(head)
		f.Close()
		for _, magic := range magics {
			if n >= len(magic) && string(head[:len(magic)]) == string(magic) {
				t.Errorf("tracked file %s is a compiled executable", fields[3])
			}
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
		// go install resolves the module path, so a stale one installs nothing.
		if strings.Contains(installer, "vanderheijden86/beadwork") {
			t.Fatal("installer still points at the previous module path")
		}
	}
	if !strings.Contains(string(installPowerShell), `$BIN_NAME = "b9s"`) {
		t.Fatal("install.ps1 still installs the previous binary name")
	}
}

func TestInstallerGoRequirement(t *testing.T) {
	mod, err := os.ReadFile(filepath.Join("..", "..", "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	match := regexp.MustCompile(`(?m)^go (\d+)\.(\d+)\.\d+$`).FindStringSubmatch(string(mod))
	if match == nil {
		t.Fatal("go.mod must declare a Go version")
	}
	minimum := match[1] + "." + match[2]
	var major, minor int
	if _, err := fmt.Sscanf(minimum, "%d.%d", &major, &minor); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name    string
		version string
		accept  bool
	}{
		{"old_advertised_minimum", "1.21.0", false},
		{"below_module_minimum", fmt.Sprintf("%d.%d.9", major, minor-1), false},
		{"module_minimum", minimum + ".0", true},
		{"minimum_without_patch", minimum, true},
		{"later_patch", minimum + ".5", true},
		{"later_minor", fmt.Sprintf("%d.%d.0", major, minor+1), true},
		{"later_major", fmt.Sprintf("%d.0.0", major+1), true},
		{"missing", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := runInstallerShell(t, tc.version, "ensure_go")
			if tc.accept {
				if err != nil {
					t.Fatalf("Go %s must be accepted: %v\n%s", tc.version, err, out)
				}
				if out != tc.version {
					t.Fatalf("accepted version = %q, want %q", out, tc.version)
				}
				return
			}
			if err == nil {
				t.Fatalf("unsupported Go %q was accepted, require Go %s or later", tc.version, minimum)
			}
			if tc.version != "" && !strings.Contains(out, "Go "+minimum+" or later is required.") {
				t.Fatalf("version warning must name go.mod minimum Go %s:\n%s", minimum, out)
			}
		})
	}

	for _, tc := range []struct {
		name    string
		version string
		accept  bool
	}{
		{"unsupported", fmt.Sprintf("%d.%d.9", major, minor-1), false},
		{"missing", "", false},
		{"supported", minimum + ".0", true},
	} {
		t.Run("source_fallback_"+tc.name, func(t *testing.T) {
			out, err := runInstallerShell(t, tc.version, "try_go_install")
			if tc.accept {
				if err != nil {
					t.Fatalf("supported source fallback failed: %v\n%s", err, out)
				}
				for _, marker := range []string{"FETCH_CALLED", "BUILD_CALLED", "INSTALL_CALLED"} {
					if !strings.Contains(out, marker) {
						t.Errorf("source fallback did not reach %s:\n%s", marker, out)
					}
				}
				return
			}
			if err == nil {
				t.Errorf("unsupported source fallback succeeded:\n%s", out)
			}
			for _, marker := range []string{"FETCH_CALLED", "BUILD_CALLED", "INSTALL_CALLED"} {
				if strings.Contains(out, marker) {
					t.Errorf("unsupported source fallback reached %s:\n%s", marker, out)
				}
			}
			want := "Go " + minimum + " or later is required for building from source."
			if !strings.Contains(out, want) {
				t.Errorf("source fallback error must contain %q:\n%s", want, out)
			}
		})
	}
}

func runInstallerShell(t *testing.T, version, action string) (string, error) {
	t.Helper()
	installer, err := filepath.Abs(filepath.Join("..", "..", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	workspace := t.TempDir()
	if err := os.Mkdir(filepath.Join(workspace, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Shell functions isolate tool discovery and side effects from the host.
	script := `
source "$1"
go() {
    case "$1" in
        version) printf 'go version go%s linux/amd64\n' "$TEST_GO_VERSION" ;;
        build) printf 'BUILD_CALLED\n' ;;
        *) return 1 ;;
    esac
}
command() {
    if [[ "$1" == "-v" && "$2" == "go" && -z "$TEST_GO_VERSION" ]]; then
        return 1
    fi
    builtin command "$@"
}
brew() { return 1; }
uname() { printf 'Linux\n'; }
is_tty() { return 1; }
make_tmp_dir() { printf '%s\n' "$TEST_WORKSPACE"; }
git() { printf 'FETCH_CALLED\n' >&3; }
ensure_install_dir() { :; }
mv() { printf 'INSTALL_CALLED\n'; }
INSTALL_DIR="$TEST_WORKSPACE"
exec 3>&1
"$2"
`
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", "-c", script, "installer-test", installer, action)
	cmd.Env = append(os.Environ(), "TEST_GO_VERSION="+version, "TEST_WORKSPACE="+workspace)
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("installer shell exceeded deadline: %v\n%s", ctx.Err(), out)
	}
	return string(out), err
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
