package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/vanderheijden86/b9s/internal/datasource"
)

// MaxRecentProjects caps the recent list so every entry has a number key (1-9).
const MaxRecentProjects = 9

const recentProjectsKey = "recent_projects"

// RecentProject is a project the user has viewed. A local project is identified
// by its resolved checkout path and host. A project without a checkout is
// identified by its server address and database.
type RecentProject struct {
	Name     string `yaml:"name"`
	Database string `yaml:"database,omitempty"`
	Host     string `yaml:"host,omitempty"`
	Path     string `yaml:"path,omitempty"`
	// OpenedAt is when this project's issues last loaded. It is never set for
	// a project that failed to open, so it names a project known to work.
	OpenedAt time.Time `yaml:"opened_at,omitempty"`
}

// SameProject uses checkout identity when both entries have paths. Catalog
// entries without a checkout match by server and database.
func (p RecentProject) SameProject(other RecentProject) bool {
	if p.Path != "" && other.Path != "" {
		return p.Host == other.Host && resolvedRecentPath(p.Path) == resolvedRecentPath(other.Path)
	}
	if p.Database != "" && other.Database != "" {
		return p.Host == other.Host && p.Database == other.Database
	}
	return false
}

func resolvedRecentPath(path string) string {
	path = expandHome(path)
	if absolute, err := filepath.Abs(path); err == nil {
		path = absolute
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	// Unavailable checkouts still need a stable identity in the saved list.
	return filepath.Clean(path)
}

// TouchRecent records p as viewed and reports whether the list changed.
//
// These are k9s's namespace-favourite rules: a project already in the list
// keeps its slot, so number keys stay stable while switching between recent
// projects; a new project is prepended and the oldest entry beyond
// MaxRecentProjects is dropped. A local checkout refreshes its metadata in
// that slot and absorbs stale entries for the same checkout and host.
func (c *Config) TouchRecent(p RecentProject) bool {
	if c.LockRecent {
		return false
	}
	if i := indexRecent(c.RecentProjects, p); i >= 0 {
		existing := c.RecentProjects[i]
		if p.Path == "" {
			return false
		}
		if p.OpenedAt.IsZero() {
			p.OpenedAt = existing.OpenedAt
		}
		changed := existing != p
		c.RecentProjects[i] = p
		for j := i + 1; j < len(c.RecentProjects); {
			if c.RecentProjects[j].SameProject(p) {
				c.RecentProjects = append(c.RecentProjects[:j], c.RecentProjects[j+1:]...)
				changed = true
			} else {
				j++
			}
		}
		return changed
	}
	c.RecentProjects = trimRecent(append([]RecentProject{p}, c.RecentProjects...))
	return true
}

// ForgetRecent removes p by project identity without changing the other slots'
// relative order. A locked list keeps its membership.
func (c *Config) ForgetRecent(p RecentProject) bool {
	if c.LockRecent || !containsRecent(c.RecentProjects, p) {
		return false
	}
	c.RecentProjects = withoutRecent(c.RecentProjects, []RecentProject{p})
	return true
}

func withoutRecent(list, forgotten []RecentProject) []RecentProject {
	kept := make([]RecentProject, 0, len(list))
	for _, p := range list {
		if !containsRecent(forgotten, p) {
			kept = append(kept, p)
		}
	}
	return kept
}

func containsRecent(list []RecentProject, p RecentProject) bool {
	for _, existing := range list {
		if existing.SameProject(p) {
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
// For a project both lists hold, the later OpenedAt wins: another window may
// have opened it more recently than this one.
func mergeRecent(mine, onDisk []RecentProject, locked bool) []RecentProject {
	merged := append([]RecentProject(nil), mine...)
	for _, p := range onDisk {
		i := indexRecent(merged, p)
		switch {
		case i >= 0:
			if p.OpenedAt.After(merged[i].OpenedAt) {
				merged[i].OpenedAt = p.OpenedAt
			}
		case !locked:
			merged = append(merged, p)
		}
	}
	return trimRecent(merged)
}

func indexRecent(list []RecentProject, p RecentProject) int {
	for i, existing := range list {
		if existing.SameProject(p) {
			return i
		}
	}
	return -1
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
// server backend takes precedence over local exports when naming its source.
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
		case datasource.SourceTypeJSONLLocal, datasource.SourceTypeSQLite, datasource.SourceTypeDoltEmbedded:
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
// Forgotten projects are excluded from both lists before merging, unless locked,
// so the on-disk copy cannot restore an entry the user explicitly forgot.
func SaveRecentTo(path string, recent []RecentProject, locked bool, forgotten ...RecentProject) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	if !locked && len(forgotten) > 0 {
		recent = withoutRecent(recent, forgotten)
	}

	var doc yaml.Node
	existing, err := os.ReadFile(path)
	switch {
	case err == nil:
		if err := yaml.Unmarshal(existing, &doc); err != nil {
			return fmt.Errorf("parsing config: %w", err)
		}
		onDisk := recentOnDisk(existing)
		if !locked && len(forgotten) > 0 {
			onDisk = withoutRecent(onDisk, forgotten)
		}
		recent = mergeRecent(recent, onDisk, locked)
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

// MarkOpened records that p's issues loaded at the given time. It changes only
// an entry already in the list, so it never moves a project to a new slot.
func (c *Config) MarkOpened(p RecentProject, at time.Time) bool {
	for i := range c.RecentProjects {
		if c.RecentProjects[i].SameProject(p) {
			c.RecentProjects[i].OpenedAt = at
			return true
		}
	}
	return false
}

// LastOpened returns the project that most recently opened successfully,
// skipping except. Entries that never opened are not candidates.
func (c *Config) LastOpened(except RecentProject) (RecentProject, bool) {
	var last RecentProject
	found := false
	for _, p := range c.RecentProjects {
		if p.OpenedAt.IsZero() || p.SameProject(except) {
			continue
		}
		if !found || p.OpenedAt.After(last.OpenedAt) {
			last, found = p, true
		}
	}
	return last, found
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
