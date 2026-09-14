package config

import (
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"

	"github.com/vanderheijden86/beadwork/internal/datasource"
)

// MaxRecentProjects caps the recent list so every entry has a number key (1-9).
const MaxRecentProjects = 9

// RecentProject is a project the user has viewed, identified by the Dolt
// server address and database it reads from.
type RecentProject struct {
	Name     string `yaml:"name"`
	Database string `yaml:"database"`
	Host     string `yaml:"host"`
	Path     string `yaml:"path,omitempty"`
}

func (p RecentProject) sameProject(other RecentProject) bool {
	return p.Host == other.Host && p.Database == other.Database
}

// TouchRecent records p as viewed and reports whether the list changed.
//
// These are k9s's namespace-favourite rules: a project already in the list
// keeps its slot, so number keys stay stable while switching between recent
// projects; a new project is prepended and the oldest entry beyond
// MaxRecentProjects is dropped.
func (c *Config) TouchRecent(p RecentProject) bool {
	if c.LockRecent || containsRecent(c.RecentProjects, p) {
		return false
	}
	c.RecentProjects = trimRecent(append([]RecentProject{p}, c.RecentProjects...))
	return true
}

func containsRecent(list []RecentProject, p RecentProject) bool {
	for _, existing := range list {
		if existing.sameProject(p) {
			return true
		}
	}
	return false
}

func trimRecent(list []RecentProject) []RecentProject {
	if len(list) > MaxRecentProjects {
		return list[:MaxRecentProjects]
	}
	return list
}

// mergeRecent keeps the order of mine and appends entries only another b9s
// process has saved, so concurrent windows do not erase each other's history.
func mergeRecent(mine, onDisk []RecentProject, locked bool) []RecentProject {
	if locked {
		return mine
	}
	merged := append([]RecentProject(nil), mine...)
	for _, p := range onDisk {
		if !containsRecent(merged, p) {
			merged = append(merged, p)
		}
	}
	return trimRecent(merged)
}

// recentOnDisk reads only the recent list from an existing config file.
func recentOnDisk(data []byte) []RecentProject {
	var file struct {
		RecentProjects []RecentProject `yaml:"recent_projects"`
	}
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil
	}
	return file.RecentProjects
}

// migrateFavorites seeds an empty recent list from the numbered favorites of
// older configs, in slot order. Favorites that do not resolve to a Dolt server
// project are skipped: the recent list only holds server-backed projects.
func (c *Config) migrateFavorites() {
	if len(c.RecentProjects) > 0 || len(c.Favorites) == 0 {
		return
	}
	slots := make([]int, 0, len(c.Favorites))
	for n := range c.Favorites {
		slots = append(slots, n)
	}
	sort.Ints(slots)

	for _, n := range slots {
		project := c.FindProject(c.Favorites[n])
		if project == nil {
			continue
		}
		if recent, ok := recentFromCheckout(project.Name, project.ResolvedPath()); ok && !containsRecent(c.RecentProjects, recent) {
			c.RecentProjects = append(c.RecentProjects, recent)
		}
	}
	c.RecentProjects = trimRecent(c.RecentProjects)
}

// recentFromCheckout describes the Dolt server project checked out at path.
func recentFromCheckout(name, path string) (RecentProject, bool) {
	sources, err := datasource.DiscoverSources(datasource.DiscoveryOptions{
		BeadsDir:            filepath.Join(path, ".beads"),
		RepoPath:            path,
		SkipWorktreeSources: true,
	})
	if err != nil {
		return RecentProject{}, false
	}
	for _, source := range sources {
		if source.Type == datasource.SourceTypeDolt {
			return RecentProject{Name: name, Database: source.Database, Host: source.Path, Path: path}, true
		}
	}
	return RecentProject{}, false
}
