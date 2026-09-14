package main

import (
	"strings"
	"testing"
	"time"

	"github.com/vanderheijden86/beadwork/internal/datasource"
	"github.com/vanderheijden86/beadwork/pkg/config"
	"github.com/vanderheijden86/beadwork/pkg/model"
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
