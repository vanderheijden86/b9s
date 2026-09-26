package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vanderheijden86/beadwork/internal/control"
	"github.com/vanderheijden86/beadwork/pkg/debug"
	"github.com/vanderheijden86/beadwork/pkg/ui"
)

const ctlUsage = `usage: b9s ctl [--pane %N] branch <issue-id>

Steers a running b9s. Inside tmux it picks the b9s in the caller's tmux
window; outside tmux it picks the only running b9s. --pane names one.

  branch <issue-id>   select the issue and show only its top-level branch
`

// runCtl implements `b9s ctl` and returns the process exit code.
func runCtl(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("ctl", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	pane := fs.String("pane", "", "tmux pane id of the b9s to steer")
	if err := fs.Parse(args); err != nil {
		fmt.Fprint(stderr, ctlUsage)
		return 2
	}
	rest := fs.Args()
	if len(rest) == 0 {
		fmt.Fprint(stderr, ctlUsage)
		return 2
	}
	var req control.Request
	switch rest[0] {
	case string(control.VerbBranch):
		if len(rest) != 2 {
			fmt.Fprint(stderr, ctlUsage)
			return 2
		}
		req = control.Request{Verb: control.VerbBranch, ID: rest[1]}
	default:
		fmt.Fprintf(stderr, "b9s ctl: unknown command %q\n%s", rest[0], ctlUsage)
		return 2
	}

	dir, err := control.DefaultDir()
	if err != nil {
		fmt.Fprintf(stderr, "b9s ctl: %v\n", err)
		return 1
	}
	instances, err := control.List(dir)
	if err != nil {
		fmt.Fprintf(stderr, "b9s ctl: %v\n", err)
		return 1
	}
	sel := control.Selector{Pane: *pane, CallerPane: os.Getenv("TMUX_PANE")}
	var windows map[string]string
	if sel.CallerPane != "" && sel.Pane == "" {
		windows = control.TmuxWindows()
	}
	inst, err := control.Pick(instances, windows, sel)
	if err != nil {
		fmt.Fprintf(stderr, "b9s ctl: %v\n", err)
		return 1
	}
	if err := control.Send(inst, req); err != nil {
		fmt.Fprintf(stderr, "b9s ctl: %v\n", err)
		return 1
	}
	return 0
}

// startControl lets `b9s ctl` steer p. b9s runs without it when the socket
// cannot be opened, since steering is an extra, not a requirement.
func startControl(p *tea.Program) *control.Server {
	dir, err := control.DefaultDir()
	if err != nil {
		debug.Log("control: %v", err)
		return nil
	}
	cwd, _ := os.Getwd()
	inst := control.Instance{PID: os.Getpid(), Cwd: cwd, TmuxPane: os.Getenv("TMUX_PANE")}
	srv, err := control.Serve(dir, inst, func(r control.Request) error {
		switch r.Verb {
		case control.VerbBranch:
			p.Send(ui.ShowBranchMsg{ID: r.ID})
		}
		return nil
	})
	if err != nil {
		debug.Log("control: %v", err)
		return nil
	}
	return srv
}
