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

	"github.com/vanderheijden86/beadwork/pkg/debug"
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

// waitDelay bounds how long Run waits for bd's stdout/stderr pipes to close
// after bd's own process exits, whether that exit is an ordinary one or a
// kill: a descendant that inherited the pipes (a hook, a backgrounded job)
// can otherwise hold them open indefinitely even though bd itself is done.
const waitDelay = time.Second

// ErrTimeout wraps the error Run returns when ctx's deadline elapses before
// bd exits, so a caller can tell a timeout apart from every other bd failure
// with errors.Is rather than matching Run's message text.
var ErrTimeout = errors.New("bd timed out")

// ErrCancelled wraps the error Run returns when ctx is cancelled (not timed
// out) while bd is running or before it starts, so a caller can tell a
// deliberate cancellation apart from a timeout or an ordinary bd failure
// with errors.Is.
var ErrCancelled = errors.New("bd cancelled")

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
//
// A non-nil error names its cause: ErrTimeout for a deadline, ErrCancelled
// for a caller cancellation, and bd's own error otherwise. Whether bd
// succeeded is judged from its own exit, never from ctx.Err() alone: ctx can
// already be past its deadline by the time CombinedOutput returns even
// though bd exited cleanly, if a descendant it forked kept a pipe open past
// that deadline. That same lingering-descendant case surfaces from Wait as
// exec.ErrWaitDelay rather than nil, regardless of whether ctx has a
// deadline at all, and Run reports it as success: nothing about the run
// itself failed.
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

	if runErr == nil {
		return trimmed, nil
	}

	if errors.Is(runErr, exec.ErrWaitDelay) {
		debug.Log("bdrun: bd exited successfully but a lingering child held its output pipes open past WaitDelay (dir=%s, args=%v)", dir, args)
		return trimmed, nil
	}

	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return trimmed, fmt.Errorf("%w after %s", ErrTimeout, elapsed)
	case errors.Is(ctx.Err(), context.Canceled):
		return trimmed, fmt.Errorf("%w: %w", ErrCancelled, runErr)
	default:
		return trimmed, runErr
	}
}
