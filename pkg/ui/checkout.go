package ui

import (
	"os"
	"path/filepath"
)

// Checkout is a local Beads checkout that bd commands can run in. It is only
// built by NewCheckout, so a write can never be sent to bd for a project that
// was opened from its database without a checkout.
type Checkout struct {
	dir string
}

// NewCheckout returns the checkout at path when path contains a .beads
// directory.
func NewCheckout(path string) (Checkout, bool) {
	beadsDir, ok := projectBeadsDir(path)
	if !ok {
		return Checkout{}, false
	}
	info, err := os.Stat(beadsDir)
	if err != nil || !info.IsDir() {
		return Checkout{}, false
	}
	return Checkout{dir: path}, true
}

// Dir is the checkout's root directory, or "" for none.
func (c Checkout) Dir() string { return c.dir }

// projectBeadsDir is the .beads directory of the project checked out at path.
// It reports false for an empty path: filepath.Join would turn that into a
// relative ".beads", which resolves against the working directory and so reads
// the project b9s was started in. Every .beads path in this package goes through
// here, which TestProjectBeadsDirIsTheOnlyBeadsJoin enforces.
func projectBeadsDir(path string) (string, bool) {
	if path == "" {
		return "", false
	}
	return filepath.Join(path, ".beads"), true
}
