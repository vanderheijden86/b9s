// Package bdrun resolves the bd CLI binary and runs it in a checkout
// directory. It is the one place that knows how to find and invoke bd, so
// every write path (the TUI's IssueWriter, the b9s attach CLI) resolves and
// runs it the same way.
package bdrun

import (
	"os/exec"
	"strings"
)

// Resolve locates the bd binary on PATH. ok is false when bd is not
// installed, in which case path is empty.
func Resolve() (path string, ok bool) {
	p, err := exec.LookPath("bd")
	if err != nil {
		return "", false
	}
	return p, true
}

// Run executes bdPath with args in dir and returns its combined stdout and
// stderr, trimmed of surrounding whitespace. args reach the process as a
// literal argv, never through a shell, so a value containing spaces or
// newlines (an attachment reference comment, for example) arrives at bd
// intact as a single argument.
func Run(bdPath, dir string, args ...string) (output string, err error) {
	cmd := exec.Command(bdPath, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}
