package updater

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sigstore/sigstore-go/pkg/root"
)

// The fixtures are the real provenance bundle GitHub issued for the v1.2.0
// release and a snapshot of the Sigstore public-good trusted root, so these
// tests exercise real signatures without network access.
const v120DarwinArm64Digest = "0e097c35aa381965dacb2d8deb9d1c4c842e2f2008d38d1b1fce178734fb7946"

func provenanceFixtures(t *testing.T) (string, root.TrustedMaterial) {
	t.Helper()
	dir := filepath.Join("testdata", "provenance")
	trusted, err := root.NewTrustedRootFromPath(filepath.Join(dir, "trusted_root.json"))
	if err != nil {
		t.Fatalf("load trusted root: %v", err)
	}
	return filepath.Join(dir, "v1.2.0.sigstore.json"), trusted
}

func mustDigest(t *testing.T, s string) []byte {
	t.Helper()
	d, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("decode digest: %v", err)
	}
	return d
}

func TestVerifyProvenanceAcceptsGenuineReleaseArchive(t *testing.T) {
	bundlePath, trusted := provenanceFixtures(t)
	if err := verifyProvenance(bundlePath, mustDigest(t, v120DarwinArm64Digest), "v1.2.0", trusted); err != nil {
		t.Fatalf("genuine v1.2.0 archive rejected: %v", err)
	}
}

func TestVerifyProvenanceRejectsArchiveTheBundleDoesNotName(t *testing.T) {
	bundlePath, trusted := provenanceFixtures(t)
	tampered := strings.Repeat("ab", 32)
	if err := verifyProvenance(bundlePath, mustDigest(t, tampered), "v1.2.0", trusted); err == nil {
		t.Fatal("archive with a digest outside the attestation was accepted")
	}
}

func TestVerifyProvenanceRejectsBundleSignedForAnotherTag(t *testing.T) {
	bundlePath, trusted := provenanceFixtures(t)
	if err := verifyProvenance(bundlePath, mustDigest(t, v120DarwinArm64Digest), "v1.3.0", trusted); err == nil {
		t.Fatal("v1.2.0 attestation accepted for tag v1.3.0")
	}
}

func TestVerifyProvenanceRejectsUnreadableBundle(t *testing.T) {
	_, trusted := provenanceFixtures(t)
	if err := verifyProvenance(filepath.Join("testdata", "provenance", "missing.json"), mustDigest(t, v120DarwinArm64Digest), "v1.2.0", trusted); err == nil {
		t.Fatal("missing bundle accepted")
	}
}

func TestReleaseWithoutAttestationIsRefused(t *testing.T) {
	release := &Release{TagName: "v9.9.9", Assets: []Asset{{Name: "checksums.txt"}}}
	if _, err := requireAttestationAsset(release); err == nil {
		t.Fatal("release without an attestation bundle was accepted")
	}
	release.Assets = append(release.Assets, Asset{Name: attestationAssetName})
	if asset, err := requireAttestationAsset(release); err != nil || asset.Name != attestationAssetName {
		t.Fatalf("attestation asset not found: %v", err)
	}
}

// The updater refuses any release without the bundle, so a workflow that
// stops publishing it under attestationAssetName breaks every future update.
func TestReleaseWorkflowPublishesProvenanceBundleAndSBOMs(t *testing.T) {
	workflow, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "release.yml"))
	if err != nil {
		t.Fatalf("read release workflow: %v", err)
	}
	config, err := os.ReadFile(filepath.Join("..", "..", ".goreleaser.yaml"))
	if err != nil {
		t.Fatalf("read GoReleaser config: %v", err)
	}
	for _, required := range []string{
		"anchore/sbom-action/download-syft@3ad7283483fc7af8ff2b4ea19663c2d5ca935e26 # v0.24.2",
		"id: attest",
		"steps.attest.outputs.bundle-path",
		"dist/*.sbom.json",
		"gh release upload",
		attestationAssetName,
	} {
		if !strings.Contains(string(workflow), required) {
			t.Errorf("release workflow missing %q", required)
		}
	}
	if !strings.Contains(string(config), "sboms:") || !strings.Contains(string(config), "artifacts: archive") {
		t.Error("GoReleaser does not emit an SBOM per archive")
	}
}
