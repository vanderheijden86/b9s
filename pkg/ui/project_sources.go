package ui

import (
	"errors"

	"github.com/vanderheijden86/beadwork/internal/datasource"
	"github.com/vanderheijden86/beadwork/pkg/config"
)

// errNoCheckout marks a project that can only be read from its database.
var errNoCheckout = errors.New("project has no local checkout")

// projectSources lists the data sources project can be loaded from. A checkout
// is discovered from its .beads directory. A project without one is read from
// its database as the startup project's Dolt user, the only credential b9s
// holds for it.
func (m Model) projectSources(project config.Project) ([]datasource.DataSource, error) {
	if beadsDir, ok := projectBeadsDir(project.ResolvedPath()); ok {
		return datasource.DiscoverSources(datasource.DiscoveryOptions{
			BeadsDir:               beadsDir,
			ValidateAfterDiscovery: false,
		})
	}
	if project.Database == "" {
		return nil, nil
	}
	return []datasource.DataSource{{
		Type:     datasource.SourceTypeDolt,
		Path:     project.Host,
		Database: project.Database,
		User:     m.startupDoltUser,
	}}, nil
}

// activeProject is the active header project, including the database of a
// project opened without a checkout, which the active name and path alone do
// not carry.
func (m Model) activeProject() config.Project {
	for _, p := range m.allProjects {
		if p.Name == m.activeProjectName && p.ResolvedPath() == m.activeProjectPath {
			return p
		}
	}
	return config.Project{Name: m.activeProjectName, Path: m.activeProjectPath}
}
