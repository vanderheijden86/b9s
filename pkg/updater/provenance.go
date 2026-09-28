package updater

import (
	"fmt"

	"github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/verify"
)

// The release workflow uploads the Sigstore bundle that GitHub issues for the
// build provenance attestation. The bundle, not the GitHub attestations API,
// is the updater's source: anonymous API calls share a limit of 60 an hour per
// address, and a release download does not.
const attestationAssetName = "attestation.sigstore.json"

const (
	githubActionsIssuer    = "https://token.actions.githubusercontent.com"
	releaseWorkflowIDFmt   = "https://github.com/" + repoOwner + "/" + repoName + "/.github/workflows/release.yml@refs/tags/%s"
	slsaProvenancePredType = "https://slsa.dev/provenance/v1"
)

// fetchTrustedMaterial reads the Sigstore public-good trusted root through TUF,
// which starts from the root embedded in sigstore-go and caches under
// ~/.sigstore. Tests replace it with a fixed snapshot.
var fetchTrustedMaterial = func() (root.TrustedMaterial, error) {
	return root.FetchTrustedRoot()
}

func requireAttestationAsset(release *Release) (*Asset, error) {
	for i := range release.Assets {
		if release.Assets[i].Name == attestationAssetName {
			return &release.Assets[i], nil
		}
	}
	return nil, fmt.Errorf("release %s has no %s; refusing an update without build provenance", release.TagName, attestationAssetName)
}

// verifyProvenance accepts the archive only when the bundle is a SLSA
// provenance statement naming the archive's SHA-256, signed through GitHub
// Actions by this repository's release workflow running for tag. The checksum
// file alone proves nothing here: whoever can replace an archive in a release
// can replace checksums.txt beside it.
func verifyProvenance(bundlePath string, archiveSHA256 []byte, tag string, trusted root.TrustedMaterial) error {
	b, err := bundle.LoadJSONFromPath(bundlePath)
	if err != nil {
		return fmt.Errorf("read provenance bundle: %w", err)
	}
	verifier, err := verify.NewVerifier(trusted,
		verify.WithSignedCertificateTimestamps(1),
		verify.WithTransparencyLog(1),
		verify.WithObserverTimestamps(1),
	)
	if err != nil {
		return fmt.Errorf("build provenance verifier: %w", err)
	}
	identity, err := verify.NewShortCertificateIdentity(githubActionsIssuer, "", fmt.Sprintf(releaseWorkflowIDFmt, tag), "")
	if err != nil {
		return fmt.Errorf("build signer identity: %w", err)
	}
	result, err := verifier.Verify(b, verify.NewPolicy(
		verify.WithArtifactDigest("sha256", archiveSHA256),
		verify.WithCertificateIdentity(identity),
	))
	if err != nil {
		return fmt.Errorf("provenance does not cover this archive: %w", err)
	}
	if result.Statement == nil || result.Statement.GetPredicateType() != slsaProvenancePredType {
		return fmt.Errorf("provenance bundle is not a SLSA provenance statement")
	}
	return nil
}
