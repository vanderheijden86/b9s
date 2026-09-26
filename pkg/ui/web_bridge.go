package ui

import (
	"github.com/vanderheijden86/beadwork/internal/datasource"
	"github.com/vanderheijden86/beadwork/pkg/config"
	"github.com/vanderheijden86/beadwork/pkg/model"
)

// These wrappers expose TUI rules to b9s web, so the browser applies the same
// actor, project list and status classification as the terminal.

// ResolveActor is the bd actor for writes run in projectDir, before identity
// resolution.
func ResolveActor(projectDir string) string {
	return defaultActorLookup.actor(projectDir)
}

// HeaderProjects is the project list the TUI header numbers 1-9.
func HeaderProjects(recent []config.RecentProject, activeName, activePath string) []config.Project {
	return headerProjects(recent, activeName, activePath)
}

// IsClosedLike reports whether status counts as closed in filters and counts.
func IsClosedLike(status model.Status) bool {
	return isClosedLikeStatus(status)
}

// ProjectKey identifies a project across the header, its counts and its
// reachability.
func ProjectKey(project config.Project) string {
	return projectKey(project)
}

// ProjectCounts is the header's issue summary of one project.
type ProjectCounts struct {
	Open, InProgress, Ready, Blocked int
}

// ProbeProjects loads every project in parallel, as the header does, and
// returns counts and reachability keyed by ProjectKey.
func ProbeProjects(projects []config.Project, startupUser string) (map[string]ProjectCounts, map[string]datasource.Reachability) {
	counts, reach := probeProjects(projects, startupUser)
	out := make(map[string]ProjectCounts, len(counts))
	for key, c := range counts {
		out[key] = ProjectCounts{Open: c.open, InProgress: c.inProgress, Ready: c.ready, Blocked: c.blocked}
	}
	return out, reach
}

// ProjectOpenTarget is what the header opens for project: its checkout, or its
// database read as the startup user when it has none.
func ProjectOpenTarget(project config.Project, startupUser string) datasource.OpenTarget {
	return openTargetFor(project, startupUser)
}

// AllProjectsDBs lists the databases the all-projects view combines.
func AllProjectsDBs(projects []config.Project, startupUser string) []datasource.DoltDBInfo {
	return allProjectsDBsFor(projects, startupUser)
}
