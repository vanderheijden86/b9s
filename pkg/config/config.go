// Package config handles loading and saving b9s configuration.
//
// Configuration follows the XDG Base Directory specification:
//   - Config:  ~/.config/b9s/config.yaml
//   - Data:    ~/.local/share/b9s/ (themes, plugins)
//   - State:   ~/.local/state/b9s/ (recent projects, view state cache)
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Project represents a registered project in the config.
type Project struct {
	Name string `yaml:"name"`
	Path string `yaml:"path"`
}

// UIConfig holds UI preference settings.
type UIConfig struct {
	DefaultView string     `yaml:"default_view,omitempty"` // list, tree, board, split
	SplitRatio  float64    `yaml:"split_ratio,omitempty"`  // Default split pane ratio (0.2-0.8)
	Headless    bool       `yaml:"headless,omitempty"`     // Compact header mode
	Sort        SortConfig `yaml:"sort,omitempty"`         // Tree sort applied at startup
}

// SortConfig is the tree sort b9s starts with. The sort popup overrides it for
// the rest of the session and never writes the override back, so every start
// returns to this sort.
type SortConfig struct {
	Field     string `yaml:"field,omitempty"`     // One of SortFieldNames
	Direction string `yaml:"direction,omitempty"` // asc or desc; empty takes the field's natural direction
}

// sortFieldNames must stay in step with ui.SortField; the ui package tests that
// every name maps to a field, because ui imports config and not the reverse.
var sortFieldNames = []string{"priority", "created", "updated", "title", "status", "type", "deps", "pagerank"}

// SortFieldNames returns the accepted ui.sort.field values.
func SortFieldNames() []string {
	return slices.Clone(sortFieldNames)
}

// UnmarshalYAML replaces the default sort as a whole: a file that names only a
// field must not inherit the default's direction, or `field: title` would sort Z-A.
func (s *SortConfig) UnmarshalYAML(node *yaml.Node) error {
	type plain SortConfig
	var decoded plain
	if err := node.Decode(&decoded); err != nil {
		return err
	}
	if decoded.Field != "" && !slices.Contains(sortFieldNames, decoded.Field) {
		return fmt.Errorf("invalid ui.sort.field %q: want one of %s", decoded.Field, strings.Join(sortFieldNames, ", "))
	}
	switch decoded.Direction {
	case "", "asc", "desc":
	default:
		return fmt.Errorf("invalid ui.sort.direction %q: want asc or desc", decoded.Direction)
	}
	*s = SortConfig(decoded)
	return nil
}

// DiscoveryConfig controls auto-discovery of projects.
type DiscoveryConfig struct {
	ScanPaths []string `yaml:"scan_paths,omitempty"` // Directories to scan for .beads/
	MaxDepth  int      `yaml:"max_depth,omitempty"`  // How deep to scan (default 3)
}

const (
	DefaultRefreshPollInterval = 500 * time.Millisecond
	minimumRefreshPollInterval = 100 * time.Millisecond
)

// RefreshInterval is a human-readable duration used in YAML configuration.
type RefreshInterval time.Duration

func (d *RefreshInterval) UnmarshalYAML(node *yaml.Node) error {
	parsed, err := time.ParseDuration(strings.TrimSpace(node.Value))
	if err != nil {
		return fmt.Errorf("invalid refresh poll interval %q: %w", node.Value, err)
	}
	if parsed < minimumRefreshPollInterval {
		return fmt.Errorf("refresh poll interval must be at least %s", minimumRefreshPollInterval)
	}
	*d = RefreshInterval(parsed)
	return nil
}

func (d RefreshInterval) MarshalYAML() (any, error) {
	return time.Duration(d).String(), nil
}

// RefreshConfig controls automatic data refresh behavior.
type RefreshConfig struct {
	PollInterval RefreshInterval `yaml:"poll_interval,omitempty"`
}

// ExperimentalConfig holds experimental feature flags.
type ExperimentalConfig struct {
	BackgroundMode *bool `yaml:"background_mode,omitempty"`
}

// Config is the top-level configuration for b9s.
type Config struct {
	Projects  []Project      `yaml:"projects,omitempty"`
	Favorites map[int]string `yaml:"favorites,omitempty"` // Number key (1-9) -> project name
	// RecentProjects is newest first; see TouchRecent for the ordering rules.
	RecentProjects []RecentProject    `yaml:"recent_projects,omitempty"`
	LockRecent     bool               `yaml:"lock_recent,omitempty"`
	UI             UIConfig           `yaml:"ui,omitempty"`
	Discovery      DiscoveryConfig    `yaml:"discovery,omitempty"`
	Refresh        RefreshConfig      `yaml:"refresh,omitempty"`
	Experimental   ExperimentalConfig `yaml:"experimental,omitempty"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Favorites: make(map[int]string),
		UI: UIConfig{
			DefaultView: "list",
			SplitRatio:  0.4,
			Sort:        SortConfig{Field: "created", Direction: "desc"},
		},
		Discovery: DiscoveryConfig{
			MaxDepth: 3,
		},
		Refresh: RefreshConfig{
			PollInterval: RefreshInterval(DefaultRefreshPollInterval),
		},
	}
}

// RefreshPollInterval returns the configured interval or the default when a
// Config value was assembled programmatically without refresh settings.
func (c Config) RefreshPollInterval() time.Duration {
	interval := time.Duration(c.Refresh.PollInterval)
	if interval == 0 {
		return DefaultRefreshPollInterval
	}
	return interval
}

// ConfigDir returns the XDG config directory for b9s.
func ConfigDir() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "b9s")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "b9s")
}

// DataDir returns the XDG data directory for b9s.
func DataDir() string {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, "b9s")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "share", "b9s")
}

// StateDir returns the XDG state directory for b9s.
func StateDir() string {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, "b9s")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "state", "b9s")
}

// ConfigPath returns the full path to config.yaml.
func ConfigPath() string {
	dir := ConfigDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "config.yaml")
}

// Load reads the config file from the XDG config directory.
// Returns DefaultConfig if the file doesn't exist.
func Load() (Config, error) {
	path := ConfigPath()
	if path == "" {
		return DefaultConfig(), nil
	}
	return LoadFrom(path)
}

// LoadFrom reads config from a specific path.
// Returns DefaultConfig if the file doesn't exist.
func LoadFrom(path string) (Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("reading config: %w", err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parsing config: %w", err)
	}

	// Ensure favorites map is initialized
	if cfg.Favorites == nil {
		cfg.Favorites = make(map[int]string)
	}

	// Expand ~ in project paths
	for i := range cfg.Projects {
		cfg.Projects[i].Path = expandHome(cfg.Projects[i].Path)
	}
	for i := range cfg.Discovery.ScanPaths {
		cfg.Discovery.ScanPaths[i] = expandHome(cfg.Discovery.ScanPaths[i])
	}
	for i := range cfg.RecentProjects {
		cfg.RecentProjects[i].Path = expandHome(cfg.RecentProjects[i].Path)
	}
	cfg.migrateFavorites()

	return cfg, nil
}

// Save writes the config to the XDG config directory.
func Save(cfg Config) error {
	path := ConfigPath()
	if path == "" {
		return fmt.Errorf("cannot determine config directory")
	}
	return SaveTo(cfg, path)
}

// SaveTo writes the config to a specific path.
func SaveTo(cfg Config, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	if existing, err := os.ReadFile(path); err == nil {
		cfg.RecentProjects = mergeRecent(cfg.RecentProjects, recentOnDisk(existing), cfg.LockRecent)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	return writeFileAtomic(path, data)
}

// writeFileAtomic writes data to a temp file beside path and renames it into
// place, so a reader never sees a half-written config.
func writeFileAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.yaml")
	if err != nil {
		return fmt.Errorf("creating temp config: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("writing config: %w", err)
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return fmt.Errorf("writing config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replacing config: %w", err)
	}
	return nil
}

// FindProject returns the project with the given name, or nil.
func (c Config) FindProject(name string) *Project {
	for i := range c.Projects {
		if strings.EqualFold(c.Projects[i].Name, name) {
			return &c.Projects[i]
		}
	}
	return nil
}

// FavoriteProject returns the project assigned to number key n (1-9), or nil.
func (c Config) FavoriteProject(n int) *Project {
	name, ok := c.Favorites[n]
	if !ok {
		return nil
	}
	return c.FindProject(name)
}

// SetFavorite assigns a project name to a number key (1-9).
func (c *Config) SetFavorite(n int, projectName string) {
	if c.Favorites == nil {
		c.Favorites = make(map[int]string)
	}
	if projectName == "" {
		delete(c.Favorites, n)
	} else {
		c.Favorites[n] = projectName
	}
}

// ProjectFavoriteNumber returns the favorite number (1-9) for a project name, or 0 if not favorited.
func (c Config) ProjectFavoriteNumber(name string) int {
	for n, pname := range c.Favorites {
		if strings.EqualFold(pname, name) {
			return n
		}
	}
	return 0
}

// ResolvedPath returns the project path with ~ expanded.
func (p Project) ResolvedPath() string {
	return expandHome(p.Path)
}

func expandHome(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[1:])
}
