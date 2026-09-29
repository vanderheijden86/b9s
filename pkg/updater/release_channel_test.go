package updater

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// The updater and install.sh read /releases/latest, which GitHub resolves to
// the newest release not marked as a prerelease. A release candidate stays
// out of both only while GoReleaser marks it so, and stays out of Homebrew,
// which has no prerelease channel, only while the tap upload is skipped.
func TestReleaseCandidatesStayOffTheStableChannels(t *testing.T) {
	config, err := os.ReadFile(filepath.Join("..", "..", ".goreleaser.yaml"))
	if err != nil {
		t.Fatalf("read GoReleaser config: %v", err)
	}
	for name, pattern := range map[string]string{
		"release.prerelease: auto": `(?m)^release:\n(?:[ \t]+.*\n|[ \t]*#.*\n)*?[ \t]+prerelease:[ \t]*auto\b`,
		"brews skip_upload: auto":  `(?m)^brews:\n(?:[ \t]+.*\n|[ \t]*#.*\n)*?[ \t]+skip_upload:[ \t]*auto\b`,
	} {
		if !regexp.MustCompile(pattern).Match(config) {
			t.Errorf(".goreleaser.yaml must set %s", name)
		}
	}
}
