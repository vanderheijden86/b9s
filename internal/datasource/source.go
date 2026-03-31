// Package datasource provides intelligent multi-source data detection and selection
// for beadwork. It discovers, validates, and selects the freshest valid source
// from SQLite databases, worktree JSONL files, and local JSONL files.
package datasource

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// SourceType identifies the type of data source
type SourceType string

const (
	// SourceTypeSQLite is a SQLite database (beads.db)
	SourceTypeSQLite SourceType = "sqlite"
	// SourceTypeJSONLWorktree is a JSONL file from a git worktree
	SourceTypeJSONLWorktree SourceType = "jsonl_worktree"
	// SourceTypeJSONLLocal is a local JSONL file
	SourceTypeJSONLLocal SourceType = "jsonl_local"
	// SourceTypeDolt is a Dolt MySQL-compatible database
	SourceTypeDolt SourceType = "dolt"
)

// Priority values for source types (higher = more authoritative)
const (
	PriorityDolt          = 110
	PrioritySQLite        = 100
	PriorityJSONLWorktree = 80
	PriorityJSONLLocal    = 50
)

// DataSource represents a potential source of beads data
type DataSource struct {
	// Type identifies the source type
	Type SourceType `json:"type"`
	// Path is the absolute path to the source file (or host:port for Dolt)
	Path string `json:"path"`
	// Priority determines preference when timestamps are equal (higher = preferred)
	Priority int `json:"priority"`
	// ModTime is the last modification time of the source
	ModTime time.Time `json:"mod_time"`
	// Valid indicates whether the source passed validation
	Valid bool `json:"valid"`
	// ValidationError describes why validation failed (if Valid is false)
	ValidationError string `json:"validation_error,omitempty"`
	// IssueCount is the number of issues in the source (set during validation)
	IssueCount int `json:"issue_count"`
	// Size is the file size in bytes
	Size int64 `json:"size"`
	// Database is the Dolt database name (only used for SourceTypeDolt)
	Database string `json:"database,omitempty"`
	// User is the Dolt connection user (only used for SourceTypeDolt)
	User string `json:"user,omitempty"`
}

// String returns a human-readable description of the source
func (s DataSource) String() string {
	status := "valid"
	if !s.Valid {
		status = fmt.Sprintf("invalid: %s", s.ValidationError)
	}
	return fmt.Sprintf("%s (%s, priority=%d, mod=%s, issues=%d, %s)",
		s.Path, s.Type, s.Priority, s.ModTime.Format(time.RFC3339), s.IssueCount, status)
}

// DiscoveryOptions configures source discovery behavior
type DiscoveryOptions struct {
	// BeadsDir is the .beads directory path (optional, auto-detected if empty)
	BeadsDir string
	// RepoPath is the repository root path (optional, uses cwd if empty)
	RepoPath string
	// ValidateAfterDiscovery runs validation on each discovered source
	ValidateAfterDiscovery bool
	// IncludeInvalid includes sources that failed validation in results
	IncludeInvalid bool
	// Verbose enables detailed logging during discovery
	Verbose bool
	// Logger receives log messages when Verbose is true
	Logger func(msg string)
}

// DiscoverSources finds all potential data sources in the beads directory
func DiscoverSources(opts DiscoveryOptions) ([]DataSource, error) {
	if opts.Logger == nil {
		opts.Logger = func(string) {}
	}

	// Determine beads directory
	beadsDir := opts.BeadsDir
	if beadsDir == "" {
		// Check BEADS_DIR environment variable
		if envDir := os.Getenv("BEADS_DIR"); envDir != "" {
			beadsDir = envDir
		} else {
			// Use repo path or current directory
			repoPath := opts.RepoPath
			if repoPath == "" {
				var err error
				repoPath, err = os.Getwd()
				if err != nil {
					return nil, fmt.Errorf("failed to get current directory: %w", err)
				}
			}
			beadsDir = filepath.Join(repoPath, ".beads")
		}
	}

	if opts.Verbose {
		opts.Logger(fmt.Sprintf("Discovering sources in: %s", beadsDir))
	}

	var sources []DataSource

	// Discover Dolt database (highest priority)
	doltSources, err := discoverDoltSources(beadsDir, opts)
	if err != nil && opts.Verbose {
		opts.Logger(fmt.Sprintf("Dolt discovery warning: %v", err))
	}
	sources = append(sources, doltSources...)

	// Discover SQLite database
	sqliteSources, err := discoverSQLiteSources(beadsDir, opts)
	if err != nil && opts.Verbose {
		opts.Logger(fmt.Sprintf("SQLite discovery warning: %v", err))
	}
	sources = append(sources, sqliteSources...)

	// Discover local JSONL files
	localSources, err := discoverLocalJSONLSources(beadsDir, opts)
	if err != nil && opts.Verbose {
		opts.Logger(fmt.Sprintf("Local JSONL discovery warning: %v", err))
	}
	sources = append(sources, localSources...)

	// Discover worktree JSONL files
	worktreeSources, err := discoverWorktreeSources(opts.RepoPath, opts)
	if err != nil && opts.Verbose {
		opts.Logger(fmt.Sprintf("Worktree discovery warning: %v", err))
	}
	sources = append(sources, worktreeSources...)

	// Validate sources if requested
	if opts.ValidateAfterDiscovery {
		for i := range sources {
			if err := ValidateSource(&sources[i]); err != nil && opts.Verbose {
				opts.Logger(fmt.Sprintf("Validation failed for %s: %v", sources[i].Path, err))
			}
		}
	}

	// Filter out invalid sources if not including them
	if opts.ValidateAfterDiscovery && !opts.IncludeInvalid {
		var validSources []DataSource
		for _, s := range sources {
			if s.Valid {
				validSources = append(validSources, s)
			}
		}
		sources = validSources
	}

	// Sort by priority and mod time
	sort.Slice(sources, func(i, j int) bool {
		if sources[i].ModTime.Equal(sources[j].ModTime) {
			return sources[i].Priority > sources[j].Priority
		}
		return sources[i].ModTime.After(sources[j].ModTime)
	})

	if opts.Verbose {
		opts.Logger(fmt.Sprintf("Discovered %d sources", len(sources)))
	}

	return sources, nil
}

// doltConfig holds Dolt connection settings parsed from .beads/config.yaml.
type doltConfig struct {
	Host     string
	Port     int
	User     string
	Database string
}

// parseDoltConfig reads Dolt settings from config.yaml using simple line parsing.
func parseDoltConfig(beadsDir string) doltConfig {
	cfg := doltConfig{
		Host:     "127.0.0.1",
		Port:     3306,
		User:     "root",
		Database: "beads",
	}

	configPath := filepath.Join(beadsDir, "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return cfg
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Try "dolt.key: value" format (canonical)
		// Also "dolt-port: value" for backward compat
		parseVal := func(prefixes []string) (string, bool) {
			for _, prefix := range prefixes {
				if strings.HasPrefix(line, prefix) {
					val := strings.TrimSpace(strings.TrimPrefix(line, prefix))
					val = strings.Trim(val, `"'`)
					return val, true
				}
			}
			return "", false
		}

		if val, ok := parseVal([]string{"dolt.host:", "dolt-host:"}); ok && val != "" {
			cfg.Host = val
		}
		if val, ok := parseVal([]string{"dolt.port:", "dolt-port:"}); ok {
			if p, err := strconv.Atoi(val); err == nil && p > 0 {
				cfg.Port = p
			}
		}
		if val, ok := parseVal([]string{"dolt.user:", "dolt-user:"}); ok && val != "" {
			cfg.User = val
		}
		if val, ok := parseVal([]string{"dolt.database:", "dolt-database:"}); ok && val != "" {
			cfg.Database = val
		}
	}

	return cfg
}

// discoverDoltSources detects a Dolt backend via .beads/metadata.json or
// .beads/config.yaml dolt.host setting. Returns a DataSource with connection
// details parsed from config.yaml.
func discoverDoltSources(beadsDir string, opts DiscoveryOptions) ([]DataSource, error) {
	isDolt := false

	// Detection method 1: metadata.json with "database": "dolt"
	metadataPath := filepath.Join(beadsDir, "metadata.json")
	if metadataBytes, err := os.ReadFile(metadataPath); err == nil {
		var metadata struct {
			Database string `json:"database"`
		}
		if json.Unmarshal(metadataBytes, &metadata) == nil && metadata.Database == "dolt" {
			isDolt = true
		}
	}

	// Detection method 2: config.yaml has dolt.host set
	if !isDolt {
		cfg := parseDoltConfig(beadsDir)
		if cfg.Host != "127.0.0.1" {
			isDolt = true
		}
	}

	if !isDolt {
		return nil, nil
	}

	cfg := parseDoltConfig(beadsDir)

	// Use dolt directory modtime as a freshness proxy if it exists;
	// otherwise use current time (remote Dolt server without local dolt dir).
	modTime := time.Now()
	doltDir := filepath.Join(beadsDir, "dolt")
	if info, err := os.Stat(doltDir); err == nil {
		modTime = info.ModTime()
	}

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	source := DataSource{
		Type:     SourceTypeDolt,
		Path:     addr,
		Priority: PriorityDolt,
		ModTime:  modTime,
		Database: cfg.Database,
		User:     cfg.User,
	}

	if opts.Verbose {
		opts.Logger(fmt.Sprintf("Found Dolt: %s/%s (user=%s, mod=%s)", addr, cfg.Database, cfg.User, modTime.Format(time.RFC3339)))
	}

	return []DataSource{source}, nil
}

// discoverSQLiteSources finds SQLite databases in the beads directory
func discoverSQLiteSources(beadsDir string, opts DiscoveryOptions) ([]DataSource, error) {
	var sources []DataSource

	// Look for beads.db
	dbPath := filepath.Join(beadsDir, "beads.db")
	info, err := os.Stat(dbPath)
	if err == nil {
		sources = append(sources, DataSource{
			Type:     SourceTypeSQLite,
			Path:     dbPath,
			Priority: PrioritySQLite,
			ModTime:  info.ModTime(),
			Size:     info.Size(),
		})
		if opts.Verbose {
			opts.Logger(fmt.Sprintf("Found SQLite: %s (mod=%s)", dbPath, info.ModTime().Format(time.RFC3339)))
		}
	}

	return sources, nil
}

// discoverLocalJSONLSources finds JSONL files in the beads directory
func discoverLocalJSONLSources(beadsDir string, opts DiscoveryOptions) ([]DataSource, error) {
	var sources []DataSource

	entries, err := os.ReadDir(beadsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read beads directory: %w", err)
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()

		// Must be a .jsonl file
		if !strings.HasSuffix(name, ".jsonl") {
			continue
		}

		// Skip backups, merge artifacts, and deletion manifests
		if strings.Contains(name, ".backup") ||
			strings.Contains(name, ".orig") ||
			strings.Contains(name, ".merge") ||
			name == "deletions.jsonl" ||
			strings.HasPrefix(name, "beads.left") ||
			strings.HasPrefix(name, "beads.right") {
			continue
		}

		path := filepath.Join(beadsDir, name)
		info, err := e.Info()
		if err != nil {
			continue
		}

		sources = append(sources, DataSource{
			Type:     SourceTypeJSONLLocal,
			Path:     path,
			Priority: PriorityJSONLLocal,
			ModTime:  info.ModTime(),
			Size:     info.Size(),
		})

		if opts.Verbose {
			opts.Logger(fmt.Sprintf("Found local JSONL: %s (mod=%s)", path, info.ModTime().Format(time.RFC3339)))
		}
	}

	return sources, nil
}

// discoverWorktreeSources finds JSONL files in git worktree beads directories
func discoverWorktreeSources(repoPath string, opts DiscoveryOptions) ([]DataSource, error) {
	if repoPath == "" {
		var err error
		repoPath, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get current directory: %w", err)
		}
	}

	// Find git directory
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		// Not a git repository
		return nil, nil
	}
	gitDir := strings.TrimSpace(string(out))
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(repoPath, gitDir)
	}

	// Look for beads-worktrees directory
	worktreesDir := filepath.Join(gitDir, "beads-worktrees")
	if _, err := os.Stat(worktreesDir); err != nil {
		// No worktrees directory
		return nil, nil
	}

	var sources []DataSource

	// Enumerate worktree directories
	entries, err := os.ReadDir(worktreesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read worktrees directory: %w", err)
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		wtDir := filepath.Join(worktreesDir, e.Name())

		// Look for issues.jsonl in this worktree
		jsonlPath := filepath.Join(wtDir, "issues.jsonl")
		info, err := os.Stat(jsonlPath)
		if err != nil {
			continue
		}

		sources = append(sources, DataSource{
			Type:     SourceTypeJSONLWorktree,
			Path:     jsonlPath,
			Priority: PriorityJSONLWorktree,
			ModTime:  info.ModTime(),
			Size:     info.Size(),
		})

		if opts.Verbose {
			opts.Logger(fmt.Sprintf("Found worktree JSONL: %s (mod=%s)", jsonlPath, info.ModTime().Format(time.RFC3339)))
		}
	}

	return sources, nil
}
