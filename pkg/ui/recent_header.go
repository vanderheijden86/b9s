package ui

import "github.com/vanderheijden86/beadwork/pkg/config"

// headerProjects lists the header rows: the recent projects in stored order,
// preceded by the startup project when it is not among them, so the folder b9s
// opened in stays visible even when its entry could not be saved.
func headerProjects(recent []config.RecentProject, startupName, startupPath string) []config.Project {
	projects := make([]config.Project, 0, len(recent)+1)
	startupListed := startupPath == ""
	for _, r := range recent {
		projects = append(projects, config.Project{Name: r.Name, Path: r.Path})
		if r.Path != "" && r.Path == startupPath {
			startupListed = true
		}
	}
	if !startupListed {
		projects = append([]config.Project{{Name: startupName, Path: startupPath}}, projects...)
	}
	return projects
}
