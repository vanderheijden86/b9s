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
	return []datasource.DataSource{databaseSource(project, m.startupDoltUser)}, nil
}

// databaseSource reads project straight from its Dolt database as user.
func databaseSource(project config.Project, user string) datasource.DataSource {
	return datasource.DataSource{
		Type:     datasource.SourceTypeDolt,
		Path:     project.Host,
		Database: project.Database,
		User:     user,
	}
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

// projectKey identifies project in the header count and reachability caches.
// A checkout keeps its path; projects without one all have an empty path, so
// they are told apart by server and database instead.
func projectKey(project config.Project) string {
	if path := project.ResolvedPath(); path != "" {
		return path
	}
	return "db:" + project.Host + "/" + project.Database
}

// allProjectsDBs lists the Dolt databases the 0 view combines: those found in
// checkouts' metadata, and those of projects opened without a checkout, read
// as the startup user.
func (m Model) allProjectsDBs() []datasource.DoltDBInfo {
	checkoutPaths := make(map[string]string, len(m.allProjects))
	var withoutCheckout []datasource.DoltDBInfo
	for _, p := range m.allProjects {
		if _, ok := projectBeadsDir(p.ResolvedPath()); ok {
			checkoutPaths[p.Name] = p.ResolvedPath()
			continue
		}
		if p.Database != "" {
			withoutCheckout = append(withoutCheckout, datasource.DoltDBInfo{
				Name:     p.Name,
				Host:     p.Host,
				User:     m.startupDoltUser,
				Database: p.Database,
			})
		}
	}
	return append(datasource.DiscoverDoltDBs(checkoutPaths), withoutCheckout...)
}
