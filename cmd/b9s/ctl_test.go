package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/vanderheijden86/beadwork/internal/control"
)

func TestCtlBranchSendsTheIDToTheRunningInstance(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "b9sctl")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	t.Setenv("B9S_CONTROL_DIR", dir)
	t.Setenv("TMUX_PANE", "")

	got := make(chan control.Request, 1)
	srv, err := control.Serve(dir, control.Instance{PID: os.Getpid()}, func(r control.Request) error {
		got <- r
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	var stdout, stderr bytes.Buffer
	if code := runCtl([]string{"branch", "bd-42"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	if r := <-got; r.Verb != control.VerbBranch || r.ID != "bd-42" {
		t.Fatalf("instance got %+v", r)
	}
}

func TestCtlReportsUsageAndMissingInstances(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "b9sctl")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	t.Setenv("B9S_CONTROL_DIR", dir)
	t.Setenv("TMUX_PANE", "")

	tests := []struct {
		args []string
		want string
	}{
		{args: nil, want: "usage: b9s ctl"},
		{args: []string{"branch"}, want: "usage: b9s ctl"},
		{args: []string{"type", "q"}, want: "unknown command"},
		{args: []string{"branch", "bd-1"}, want: "no b9s is running"},
	}
	for _, tt := range tests {
		var stdout, stderr bytes.Buffer
		if code := runCtl(tt.args, &stdout, &stderr); code == 0 || !strings.Contains(stderr.String(), tt.want) {
			t.Errorf("runCtl(%v) = %d, stderr %q; want failure with %q", tt.args, code, stderr.String(), tt.want)
		}
	}
}
