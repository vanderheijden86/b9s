package config

import (
	"os"
	"path/filepath"
	"strings"
)

// DiscoverProjects scans directories for .beads/ subdirectories and returns
// projects found. It merges discovered projects with existing registered
// projects, preferring the registered name when a path matches.
func DiscoverProjects(cfg Config) []Project {
	seen := make(map[string]bool)
	var result []Project

	// Start with registered projects
	for _, p := range cfg.Projects {
		resolved := p.ResolvedPath()
		seen[resolved] = true
		result = append(result, p)
	}

	// Scan discovery paths
	for _, scanPath := range cfg.Discovery.ScanPaths {
		maxDepth := cfg.Discovery.MaxDepth
		if maxDepth <= 0 {
			maxDepth = 3
		}
		found := scanForBeads(scanPath, maxDepth)
		for _, f := range found {
			if !seen[f] {
				seen[f] = true
				result = append(result, Project{
					Name: filepath.Base(f),
					Path: f,
				})
			}
		}
	}

	return result
}

// scanForBeads walks a directory tree up to maxDepth levels deep,
// looking for directories that contain a .beads/ subdirectory.
func scanForBeads(root string, maxDepth int) []string {
	root = expandHome(root)
	var results []string

	rootDepth := strings.Count(filepath.Clean(root), string(filepath.Separator))

	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return filepath.SkipDir
		}
		if !d.IsDir() {
			return nil
		}

		// Check depth
		currentDepth := strings.Count(filepath.Clean(path), string(filepath.Separator)) - rootDepth
		if currentDepth > maxDepth {
			return filepath.SkipDir
		}

		// Skip hidden directories (except .beads itself which we're looking for)
		name := d.Name()
		if strings.HasPrefix(name, ".") && name != ".beads" {
			return filepath.SkipDir
		}

		// Check if this directory contains .beads/
		beadsDir := filepath.Join(path, ".beads")
		if info, err := os.Stat(beadsDir); err == nil && info.IsDir() {
			results = append(results, path)
			return filepath.SkipDir // Don't recurse into projects
		}

		return nil
	})

	return results
}

// DetectCurrentProject attempts to find the current project by walking
// up from the current directory looking for .beads/.
func DetectCurrentProject() (string, bool) {
	dir, err := os.Getwd()
	if err != nil {
		return "", false
	}
	return findBeadsRoot(dir)
}

// findBeadsRoot walks up from dir looking for a .beads/ directory.
func findBeadsRoot(dir string) (string, bool) {
	home, _ := os.UserHomeDir()

	for {
		beadsDir := filepath.Join(dir, ".beads")
		if info, err := os.Stat(beadsDir); err == nil && info.IsDir() {
			return dir, true
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break // Reached filesystem root
		}
		// Don't go above home directory
		if home != "" && dir == home {
			break
		}
		dir = parent
	}
	return "", false
}
