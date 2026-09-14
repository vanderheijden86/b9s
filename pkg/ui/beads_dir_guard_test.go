package ui

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestProjectBeadsDirIsTheOnlyBeadsJoin fails when non-test code in this package
// joins a path with ".beads" directly. A project opened without a checkout has
// an empty path, and a direct join silently reads the startup project's data
// under the other project's name. projectBeadsDir refuses the empty path.
func TestProjectBeadsDirIsTheOnlyBeadsJoin(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	// Arguments may themselves be calls, such as p.ResolvedPath(), so the match
	// runs to ".beads" anywhere later on the line.
	directJoin := regexp.MustCompile(`filepath\.Join\(.*"\.beads"`)
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") || file == "checkout.go" {
			continue
		}
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			if directJoin.MatchString(line) {
				t.Errorf("%s:%d joins .beads directly; use projectBeadsDir", file, i+1)
			}
		}
	}
}
