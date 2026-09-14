package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"

	"github.com/vanderheijden86/beadwork/internal/datasource"
)

// MaxRecentProjects caps the recent list so every entry has a number key (1-9).
const MaxRecentProjects = 9

const recentProjectsKey = "recent_projects"

// RecentProject is a project the user has viewed. A Dolt server project is
// identified by its server address and database; a JSONL or SQLite project
// has neither and is identified by its checkout path.
type RecentProject struct {
	Name     string `yaml:"name"`
	Database string `yaml:"database,omitempty"`
	Host     string `yaml:"host,omitempty"`
	Path     string `yaml:"path,omitempty"`
}

// sameProject compares server identity only when both entries carry one. An
// entry written by hand may lack the database of a Dolt checkout, and must
// still match that checkout by path rather than gain a duplicate.
func (p RecentProject) sameProject(other RecentProject) bool {
	if p.Database != "" && other.Database != "" {
		return p.Host == other.Host && p.Database == other.Database
	}
	return p.Path != "" && p.Path == other.Path
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
// older configs, in slot order. Favorites whose path no longer holds a Beads
// checkout are skipped.
func (c *Config) migrateFavorites(legacy legacyConfig) {
	if len(c.RecentProjects) > 0 || len(legacy.Favorites) == 0 {
		return
	}
	slots := make([]int, 0, len(legacy.Favorites))
	for n := range legacy.Favorites {
		slots = append(slots, n)
	}
	sort.Ints(slots)

	for _, n := range slots {
		project := legacy.findProject(legacy.Favorites[n])
		if project == nil {
			continue
		}
		if recent, ok := RecentFromCheckout(project.Name, project.ResolvedPath()); ok && !containsRecent(c.RecentProjects, recent) {
			c.RecentProjects = append(c.RecentProjects, recent)
		}
	}
	c.RecentProjects = trimRecent(c.RecentProjects)
}

// RecentFromCheckout describes the Beads project checked out at path. A Dolt
// server backend takes precedence, because the database, not the folder, is
// what another checkout of the same project would share.
func RecentFromCheckout(name, path string) (RecentProject, bool) {
	// An empty path would resolve .beads against the working directory and
	// describe whichever project b9s was started in.
	if path == "" {
		return RecentProject{}, false
	}
	sources, err := datasource.DiscoverSources(datasource.DiscoveryOptions{
		BeadsDir:            filepath.Join(path, ".beads"),
		RepoPath:            path,
		SkipWorktreeSources: true,
	})
	if err != nil {
		return RecentProject{}, false
	}
	hasLocalSource := false
	for _, source := range sources {
		switch source.Type {
		case datasource.SourceTypeDolt:
			return RecentProject{Name: name, Database: source.Database, Host: source.Path, Path: path}, true
		case datasource.SourceTypeJSONLLocal, datasource.SourceTypeSQLite:
			hasLocalSource = true
		}
	}
	if !hasLocalSource {
		return RecentProject{}, false
	}
	return RecentProject{Name: name, Path: path}, true
}

// SaveRecentTo writes recent as the config file's recent_projects and leaves
// every other key, and the user's comments, as they are. Opening a project
// saves this list, so it must not rewrite a hand-edited file with defaults.
func SaveRecentTo(path string, recent []RecentProject, locked bool) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	var doc yaml.Node
	existing, err := os.ReadFile(path)
	switch {
	case err == nil:
		if err := yaml.Unmarshal(existing, &doc); err != nil {
			return fmt.Errorf("parsing config: %w", err)
		}
		recent = mergeRecent(recent, recentOnDisk(existing), locked)
	case os.IsNotExist(err):
	default:
		return fmt.Errorf("reading config: %w", err)
	}

	if doc.Kind == 0 {
		doc.Kind = yaml.DocumentNode
	}
	if len(doc.Content) == 0 {
		doc.Content = []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return fmt.Errorf("parsing config: top level is not a mapping")
	}

	var value yaml.Node
	if err := value.Encode(recent); err != nil {
		return fmt.Errorf("encoding recent projects: %w", err)
	}
	setMappingValue(root, recentProjectsKey, &value)

	data, err := yaml.Marshal(&doc)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	return writeFileAtomic(path, data)
}

func setMappingValue(mapping *yaml.Node, key string, value *yaml.Node) {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content[i+1] = value
			return
		}
	}
	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		value)
}
