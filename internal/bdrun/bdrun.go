// Package bdrun resolves the bd CLI binary and runs it in a checkout
// directory. It is the one place that knows how to find and invoke bd, so
// every write path (the TUI's IssueWriter, the b9s attach CLI) resolves and
// runs it the same way.
package bdrun

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"time"
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

// waitDelay bounds how long Run waits for bd's process group to release
// stdout/stderr after a kill, the same role WaitDelay plays in
// internal/blobstore's credential_command runner: SIGKILL ends bd itself
// promptly, but a grandchild that inherited the pipes can hold them open
// past that, and Wait would otherwise block on it indefinitely.
const waitDelay = time.Second

// Run executes bdPath with args in dir, bound to ctx, and returns its
// combined stdout and stderr, trimmed of surrounding whitespace. args reach
// the process as a literal argv, never through a shell, so a value
// containing spaces or newlines (an attachment reference comment, for
// example) arrives at bd intact as a single argument.
//
// bd runs in its own process group (Setpgid), and ctx.Cancel (deadline or
// caller cancellation) kills the whole group, not only the bd process
// exec.CommandContext started: bd can itself fork a child (a hook, a git
// call) that inherits the same stdout/stderr pipes, and killing bd alone
// would leave that child holding them open, blocking Wait past the
// deadline that was supposed to bound this call.
func Run(ctx context.Context, bdPath, dir string, args ...string) (output string, err error) {
	cmd := exec.CommandContext(ctx, bdPath, args...)
	cmd.Dir = dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = waitDelay

	start := time.Now()
	out, runErr := cmd.CombinedOutput()
	elapsed := time.Since(start).Round(time.Millisecond)
	trimmed := strings.TrimSpace(string(out))

	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return trimmed, fmt.Errorf("bd timed out after %s", elapsed)
	}
	return trimmed, runErr
}
