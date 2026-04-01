package datasource

import (
	"fmt"

	"github.com/vanderheijden86/beadwork/pkg/debug"
	"github.com/vanderheijden86/beadwork/pkg/loader"
	"github.com/vanderheijden86/beadwork/pkg/model"
)

// LoadIssues performs smart multi-source detection and loading.
// It discovers all available sources (SQLite, JSONL), validates them, selects
// the freshest valid source, and loads issues from it. SQLite is preferred over
// JSONL when both exist at comparable freshness, since SQLite reflects the most
// recent state (including status changes from br operations).
//
// Falls back to legacy JSONL-only loading via loader.LoadIssues if smart
// detection finds no valid sources.
func LoadIssues(repoPath string) ([]model.Issue, error) {
	beadsDir, err := loader.GetBeadsDir(repoPath)
	if err != nil {
		debug.Log("load: GetBeadsDir(%q) failed: %v", repoPath, err)
		return nil, err
	}
	debug.Log("load: LoadIssues repoPath=%q beadsDir=%q", repoPath, beadsDir)

	issues, smartErr := loadSmart(beadsDir, repoPath)
	if smartErr == nil {
		debug.Log("load: loadSmart returned %d issues", len(issues))
		return issues, nil
	}

	// Fall back to legacy JSONL-only loading
	debug.Log("load: loadSmart failed (%v), falling back to JSONL", smartErr)
	issues, err = loader.LoadIssues(repoPath)
	debug.Log("load: JSONL fallback returned %d issues, err=%v", len(issues), err)
	return issues, err
}

// LoadIssuesFromDir performs smart source detection within a known beads directory.
// This is useful when the caller already knows the .beads path.
func LoadIssuesFromDir(beadsDir string) ([]model.Issue, error) {
	issues, smartErr := loadSmart(beadsDir, "")
	if smartErr == nil {
		return issues, nil
	}

	// Fall back to JSONL
	jsonlPath, err := loader.FindJSONLPath(beadsDir)
	if err != nil {
		return nil, err
	}
	return loader.LoadIssuesFromFile(jsonlPath)
}

// loadSmart discovers sources, validates, selects the best, and loads from it.
func loadSmart(beadsDir, repoPath string) ([]model.Issue, error) {
	sources, err := DiscoverSources(DiscoveryOptions{
		BeadsDir:               beadsDir,
		RepoPath:               repoPath,
		ValidateAfterDiscovery: true,
		IncludeInvalid:         false,
	})
	if err != nil {
		debug.Log("load: DiscoverSources failed: %v", err)
		return nil, err
	}
	debug.Log("load: discovered %d valid sources", len(sources))
	for i, s := range sources {
		debug.Log("load:   [%d] type=%s path=%s db=%s priority=%d", i, s.Type, s.Path, s.Database, s.Priority)
	}
	if len(sources) == 0 {
		return nil, fmt.Errorf("no valid sources discovered")
	}

	best, err := SelectBestSource(sources)
	if err != nil {
		debug.Log("load: SelectBestSource failed: %v", err)
		return nil, err
	}
	debug.Log("load: selected best source: type=%s path=%s db=%s", best.Type, best.Path, best.Database)

	return LoadFromSource(best)
}

// LoadFromSource loads issues from a specific DataSource, dispatching to the
// appropriate reader based on source type.
func LoadFromSource(source DataSource) ([]model.Issue, error) {
	debug.Log("load: LoadFromSource type=%s path=%s db=%s", source.Type, source.Path, source.Database)
	switch source.Type {
	case SourceTypeSQLite:
		debug.Log("load: opening SQLite reader: %s", source.Path)
		reader, err := NewSQLiteReader(source)
		if err != nil {
			return nil, fmt.Errorf("failed to open SQLite source %s: %w", source.Path, err)
		}
		defer reader.Close()
		issues, err := reader.LoadIssues()
		debug.Log("load: SQLite returned %d issues, err=%v", len(issues), err)
		return issues, err

	case SourceTypeDolt:
		debug.Log("load: opening Dolt reader: %s/%s", source.Path, source.Database)
		reader, err := NewDoltReader(source)
		if err != nil {
			return nil, fmt.Errorf("failed to open Dolt source %s: %w", source.Path, err)
		}
		defer reader.Close()
		issues, err := reader.LoadIssues()
		debug.Log("load: Dolt returned %d issues, err=%v", len(issues), err)
		return issues, err

	case SourceTypeJSONLLocal, SourceTypeJSONLWorktree:
		debug.Log("load: loading JSONL file: %s", source.Path)
		issues, err := loader.LoadIssuesFromFile(source.Path)
		debug.Log("load: JSONL returned %d issues, err=%v", len(issues), err)
		return issues, err

	default:
		return nil, fmt.Errorf("unknown source type: %s", source.Type)
	}
}
