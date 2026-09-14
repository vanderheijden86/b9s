package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/vanderheijden86/beadwork/internal/datasource"
	"github.com/vanderheijden86/beadwork/pkg/config"
)

// projectOpener loads a project; datasource.OpenProject in production.
type projectOpener func(datasource.OpenTarget) (datasource.OpenedProject, *datasource.OpenFailure)

// startupChoice is the project b9s opens at startup.
type startupChoice struct {
	Name   string
	Dir    string
	Opened datasource.OpenedProject
	// StartupFailure is why the folder b9s was started in did not open, when
	// b9s fell back to the last project that opened successfully.
	StartupFailure *datasource.OpenFailure
}

// chooseStartupProject opens the startup folder, or, when that fails in an
// interactive terminal, the project that most recently opened successfully.
//
// Without a terminal b9s never opens a different project: a script that
// starts b9s in the wrong folder must fail, not quietly act on other data.
// Only a recent project with a local checkout can be the fallback, because a
// database without one needs the startup project's Dolt user to connect.
func chooseStartupProject(name, dir string, cfg *config.Config, interactive bool, open projectOpener) (startupChoice, error) {
	opened, failure := open(datasource.OpenTarget{Name: name, Dir: dir})
	if failure == nil {
		return startupChoice{Name: name, Dir: dir, Opened: opened}, nil
	}
	if !interactive {
		return startupChoice{}, startupError(formatOpenFailure(failure))
	}

	last, ok := cfg.LastOpened(config.RecentProject{Name: name, Path: dir})
	if !ok || last.Path == "" {
		return startupChoice{}, startupError(formatOpenFailure(failure) +
			"No recent project with a local checkout has opened successfully, so there is nothing to fall back to.\n")
	}
	fallback, fallbackFailure := open(datasource.OpenTarget{Name: last.Name, Dir: last.Path})
	if fallbackFailure != nil {
		return startupChoice{}, startupError(formatOpenFailure(failure) + formatOpenFailure(fallbackFailure))
	}
	return startupChoice{Name: last.Name, Dir: last.Path, Opened: fallback, StartupFailure: failure}, nil
}

func startupError(text string) error {
	return errors.New(strings.TrimRight(text, "\n"))
}

// formatOpenFailure renders a failure for stderr.
func formatOpenFailure(f *datasource.OpenFailure) string {
	project := f.Project
	if project == "" {
		project = f.Dir
	}
	var b strings.Builder
	fmt.Fprintf(&b, "b9s: cannot open %s\n", project)
	fmt.Fprintf(&b, "  Reason  %s\n", f.Message())
	if f.Err != nil {
		fmt.Fprintf(&b, "  Detail  %v\n", f.Err)
	}
	for i, step := range f.Try() {
		label := "        "
		if i == 0 {
			label = "Try     "
		}
		fmt.Fprintf(&b, "  %s%s\n", label, step)
	}
	return b.String()
}
