package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/vanderheijden86/b9s/internal/datasource"
	"github.com/vanderheijden86/b9s/pkg/config"
	"github.com/vanderheijden86/b9s/pkg/model"
)

// fakeOpener opens the directories in working and fails every other target
// with a server-down failure, recording what it was asked to open.
type fakeOpener struct {
	working map[string]int // dir -> issue count
	opened  []string
}

func (f *fakeOpener) open(target datasource.OpenTarget) (datasource.OpenedProject, *datasource.OpenFailure) {
	f.opened = append(f.opened, target.Dir)
	if n, ok := f.working[target.Dir]; ok {
		return datasource.OpenedProject{Issues: make([]model.Issue, n)}, nil
	}
	return datasource.OpenedProject{}, &datasource.OpenFailure{
		Reason: datasource.OpenServerDown, Project: target.Name, Dir: target.Dir, Server: "127.0.0.1:1", Database: target.Name,
	}
}

func configWithOpened(projects ...config.RecentProject) *config.Config {
	cfg := config.DefaultConfig()
	cfg.RecentProjects = projects
	return &cfg
}

func TestChooseStartupProject_OpensWorkingStartupFolder(t *testing.T) {
	opener := &fakeOpener{working: map[string]int{"/work/b9s": 3}}

	choice, err := chooseStartupProject("b9s", "/work/b9s", configWithOpened(), true, opener.open)

	if err != nil {
		t.Fatalf("err = %v, want the startup folder to open", err)
	}
	if choice.Dir != "/work/b9s" || len(choice.Opened.Issues) != 3 || choice.StartupFailure != nil {
		t.Errorf("choice = %+v, want b9s with 3 issues and no failure", choice)
	}
}

func TestChooseStartupProject_WithoutTerminalFailsInsteadOfFallingBack(t *testing.T) {
	opener := &fakeOpener{working: map[string]int{"/work/b9s": 3}}
	cfg := configWithOpened(config.RecentProject{Name: "b9s", Path: "/work/b9s", OpenedAt: time.Now()})

	_, err := chooseStartupProject("ghost", "/work/ghost", cfg, false, opener.open)

	if err == nil {
		t.Fatal("err = nil, want the startup failure without a terminal")
	}
	if len(opener.opened) != 1 {
		t.Errorf("opened %v, want only the startup folder", opener.opened)
	}
}

func TestChooseStartupProject_FallsBackToLastOpenedProject(t *testing.T) {
	opener := &fakeOpener{working: map[string]int{"/work/b9s": 3}}
	cfg := configWithOpened(config.RecentProject{Name: "b9s", Path: "/work/b9s", OpenedAt: time.Now()})

	choice, err := chooseStartupProject("ghost", "/work/ghost", cfg, true, opener.open)

	if err != nil {
		t.Fatalf("err = %v, want a fallback to b9s", err)
	}
	if choice.Name != "b9s" || choice.Dir != "/work/b9s" {
		t.Errorf("choice = %+v, want b9s", choice)
	}
	if choice.StartupFailure == nil || choice.StartupFailure.Project != "ghost" {
		t.Errorf("startup failure = %+v, want ghost's failure kept for the popup", choice.StartupFailure)
	}
}

func TestChooseStartupProject_WithoutOpenedProjectFails(t *testing.T) {
	opener := &fakeOpener{working: map[string]int{"/work/b9s": 3}}
	cfg := configWithOpened(config.RecentProject{Name: "b9s", Path: "/work/b9s"})

	if _, err := chooseStartupProject("ghost", "/work/ghost", cfg, true, opener.open); err == nil {
		t.Fatal("err = nil, want failure: no recent project has ever opened")
	}
}

func TestChooseStartupProject_FailingFallbackReportsBothProjects(t *testing.T) {
	opener := &fakeOpener{}
	cfg := configWithOpened(config.RecentProject{Name: "b9s", Path: "/work/b9s", OpenedAt: time.Now()})

	_, err := chooseStartupProject("ghost", "/work/ghost", cfg, true, opener.open)

	if err == nil {
		t.Fatal("err = nil, want failure when the fallback also fails")
	}
	for _, want := range []string{"ghost", "b9s"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %s", err, want)
		}
	}
}

func TestFormatOpenFailure_ShowsReasonAndNextSteps(t *testing.T) {
	f := &datasource.OpenFailure{Reason: datasource.OpenServerDown, Project: "ghost", Server: "127.0.0.1:1", Database: "ghost"}

	out := formatOpenFailure(f)

	for _, want := range []string{"b9s: cannot open ghost", "Reason", f.Message(), "Try", f.Try()[0], f.Try()[1]} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

func TestFormatOpenFailure_StaleTunnelHint(t *testing.T) {
	timeoutErr := fmt.Errorf("cannot connect to Dolt: %w", &net.OpError{
		Op: "read", Net: "tcp", Err: os.ErrDeadlineExceeded,
	})
	cases := []struct {
		name       string
		server     string
		err        error
		wantTunnel bool
	}{
		{"IPv4", "127.0.0.1:3306", timeoutErr, true},
		{"IPv4 loopback range", "127.0.0.2:13306", timeoutErr, true},
		{"localhost", "localhost:13306", timeoutErr, true},
		{"uppercase localhost", "LOCALHOST:3306", timeoutErr, true},
		{"IPv6", "[::1]:13306", timeoutErr, true},
		{"mapped IPv4", "[::ffff:127.0.0.1]:3306", timeoutErr, true},
		{"remote hostname", "db.example:3306", timeoutErr, false},
		{"private IPv4", "10.0.0.5:3306", timeoutErr, false},
		{"remote IPv6", "[2001:db8::1]:3306", timeoutErr, false},
		{"wildcard", "0.0.0.0:3306", timeoutErr, false},
		{"missing port", "localhost", timeoutErr, false},
		{"no error", "127.0.0.1:3306", nil, false},
		{"refused connection", "127.0.0.1:3306", &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNREFUSED}, false},
		{"context deadline", "127.0.0.1:3306", context.DeadlineExceeded, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &datasource.OpenFailure{
				Reason: datasource.OpenServerDown, Project: "ghost",
				Server: tc.server, Err: tc.err,
			}
			out := formatOpenFailure(f)
			_, hints, ok := strings.Cut(out, "  Try     ")
			if !ok {
				t.Fatalf("output has no Try lines:\n%s", out)
			}
			for _, want := range []string{
				"Check that " + tc.server + " is the Dolt server's address and that this computer can reach it",
				"Test the connection with bd: bd list",
			} {
				if !strings.Contains(hints, want) {
					t.Errorf("Try lines lack existing hint %q:\n%s", want, hints)
				}
			}
			if tc.wantTunnel {
				for _, want := range []string{
					"SSH tunnel on " + tc.server + " may be stale",
					"restart",
					"osenco",
					"launchctl kickstart -k gui/$UID/com.osenco.dolt-tunnel",
				} {
					if !strings.Contains(hints, want) {
						t.Errorf("Try lines lack %q:\n%s", want, hints)
					}
				}
			} else if strings.Contains(strings.ToLower(hints), "tunnel") {
				t.Errorf("Try lines unexpectedly mention a tunnel:\n%s", hints)
			}
		})
	}
}
