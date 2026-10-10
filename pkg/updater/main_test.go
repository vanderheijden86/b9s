package updater

import (
	"os"
	"testing"

	"github.com/vanderheijden86/b9s/pkg/version"
)

// A test binary carries no release version, so version.Version resolves to
// "dev", which compareVersions ranks above every release. The update checks
// under test need a release to compare against.
func TestMain(m *testing.M) {
	version.Version = "v1.0.0"
	os.Exit(m.Run())
}
