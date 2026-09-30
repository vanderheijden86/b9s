package ui

import "github.com/vanderheijden86/b9s/pkg/config"

// headerProjects lists the header rows: the recent projects in stored order,
// preceded by the startup project when it is not among them, so the folder b9s
// opened in stays visible even when its entry could not be saved.
//
// The startup project is matched with RecentProject.SameProject, the identity
// TouchRecent stores under. An entry saved without a checkout (opened through
// :project) is the same project as the checkout on its database, so its row
// takes the checkout's name and path: the active-project checks compare those.
func headerProjects(recent []config.RecentProject, startup config.RecentProject) []config.Project {
	projects := make([]config.Project, 0, len(recent)+1)
	startupListed := startup.Path == ""
	for _, r := range recent {
		row := config.Project{Name: r.Name, Path: r.Path, Database: r.Database, Host: r.Host}
		if !startupListed && r.SameProject(startup) {
			startupListed = true
			row.Name, row.Path = startup.Name, startup.Path
		}
		projects = append(projects, row)
	}
	if !startupListed {
		projects = append([]config.Project{{Name: startup.Name, Path: startup.Path, Database: startup.Database, Host: startup.Host}}, projects...)
	}
	return projects
}
