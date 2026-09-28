// Package bdrun resolves the bd CLI binary and runs it in a checkout
// directory. It is the one place that knows how to find and invoke bd, so
// every write path (the TUI's IssueWriter, the b9s attach CLI) resolves and
// runs it the same way.
package bdrun

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/vanderheijden86/b9s/pkg/debug"
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
	cmd := command(ctx, bdPath, dir, nil, args)
	start := time.Now()
	out, runErr := cmd.CombinedOutput()
	trimmed := strings.TrimSpace(string(out))
	return trimmed, classify(ctx, cmd, runErr, time.Since(start), dir, args)
}

// Output runs bd like Run, but returns stdout and stderr apart, for a caller
// that parses stdout: bd prints hints and warnings on stderr, which would
// corrupt machine-readable output. env entries are appended to the inherited
// environment, so they override it.
func Output(ctx context.Context, bdPath, dir string, env []string, args ...string) (stdout []byte, stderr string, err error) {
	cmd := command(ctx, bdPath, dir, env, args)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	start := time.Now()
	runErr := cmd.Run()
	return outBuf.Bytes(), strings.TrimSpace(errBuf.String()), classify(ctx, cmd, runErr, time.Since(start), dir, args)
}

func command(ctx context.Context, bdPath, dir string, env, args []string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, bdPath, args...)
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = waitDelay
	return cmd
}

// classify turns the error from running cmd into Run's documented result.
func classify(ctx context.Context, cmd *exec.Cmd, runErr error, elapsed time.Duration, dir string, args []string) error {
	elapsed = elapsed.Round(time.Millisecond)
	if runErr == nil {
		return nil
	}

	if errors.Is(runErr, exec.ErrWaitDelay) {
		debug.Log("bdrun: bd exited successfully but a lingering child held its output pipes open past WaitDelay (dir=%s, args=%v)", dir, args)
		return nil
	}

	// cmd.ProcessState is set as soon as the bd process itself exits, and is
	// authoritative over ctx.Err(): watchCtx and the pipe-copying goroutines
	// can still be unwinding a lingering descendant well after bd is done, so
	// ctx can already read as DeadlineExceeded even though bd finished
	// cleanly or failed on its own. Only a process a signal killed, or one
	// that never started, falls through to the ctx-based classification
	// below.
	if state := cmd.ProcessState; state != nil {
		if state.Success() {
			return nil
		}
		if state.Exited() {
			return runErr
		}
	}

	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return fmt.Errorf("%w after %s", ErrTimeout, elapsed)
	case errors.Is(ctx.Err(), context.Canceled):
		return fmt.Errorf("%w: %w (%v)", ErrCancelled, ctx.Err(), runErr)
	default:
		return runErr
	}
}
