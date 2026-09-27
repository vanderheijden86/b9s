package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPTYTimeoutsFailInsteadOfSkipping reads the E2E sources and fails when a
// branch that handles a context deadline calls Skip. A skip on a deadline turns
// a hung b9s into a green suite: only a positive detection that the PTY
// harness is missing may skip.
func TestPTYTimeoutsFailInsteadOfSkipping(t *testing.T) {
	files, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatal(err)
	}
	deadline := "Deadline" + "Exceeded"
	for _, name := range files {
		if name == "timeout_guard_test.go" {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(string(src), "\n")
		for i, line := range lines {
			if !strings.Contains(line, deadline) || !strings.Contains(line, "if ") {
				continue
			}
			for j := i + 1; j < len(lines) && j <= i+3; j++ {
				if strings.Contains(lines[j], "t.Skip") {
					t.Errorf("%s:%d: a deadline branch calls Skip; fail with the output instead", name, j+1)
				}
			}
		}
	}
}
