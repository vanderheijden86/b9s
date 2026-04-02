package datasource

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/vanderheijden86/beadwork/pkg/debug"
	"github.com/vanderheijden86/beadwork/pkg/model"
)

// MultiDoltReader loads issues from multiple Dolt databases on the same server.
// Each database gets its own DoltReader (separate connection).
type MultiDoltReader struct {
	readers  map[string]*DoltReader // database name -> reader
	dbNames  []string              // ordered list of database names
	mu       sync.RWMutex
}

// DoltDBInfo holds connection info for one Dolt database, extracted from a project.
type DoltDBInfo struct {
	Name     string // database name (e.g., "LP_Team")
	Host     string // e.g., "osen.co"
	Port     int    // e.g., 3306
	User     string // e.g., "root"
	Database string // e.g., "LP_Team"
}

// DiscoverDoltDBs scans all project paths and returns Dolt database info for each
// project that has a Dolt server backend. Non-Dolt projects are skipped.
func DiscoverDoltDBs(projectPaths map[string]string) []DoltDBInfo {
	var dbs []DoltDBInfo
	for name, projPath := range projectPaths {
		beadsDir := filepath.Join(projPath, ".beads")
		sources, err := discoverDoltSources(beadsDir, DiscoveryOptions{})
		if err != nil || len(sources) == 0 {
			continue
		}
		s := sources[0]
		dbs = append(dbs, DoltDBInfo{
			Name:     name,
			Host:     s.Path, // already "host:port"
			User:     s.User,
			Database: s.Database,
		})
	}
	return dbs
}

// NewMultiDoltReader creates readers for each Dolt database.
// Databases that fail to connect are logged and skipped.
func NewMultiDoltReader(dbs []DoltDBInfo) (*MultiDoltReader, error) {
	readers := make(map[string]*DoltReader, len(dbs))
	var dbNames []string

	for _, db := range dbs {
		source := DataSource{
			Type:     SourceTypeDolt,
			Path:     db.Host,
			Database: db.Database,
			User:     db.User,
		}
		reader, err := NewDoltReader(source)
		if err != nil {
			debug.Log("multi-dolt: skip %s/%s: %v", db.Host, db.Database, err)
			continue
		}
		readers[db.Database] = reader
		dbNames = append(dbNames, db.Database)
		debug.Log("multi-dolt: connected to %s/%s", db.Host, db.Database)
	}

	if len(readers) == 0 {
		return nil, fmt.Errorf("no Dolt databases reachable")
	}

	return &MultiDoltReader{
		readers: readers,
		dbNames: dbNames,
	}, nil
}

// LoadAllIssues loads issues from all connected databases concurrently.
// Returns a merged slice. Each issue's SourceRepo is set to the database name
// for identification. Returns partial results if some databases fail.
func (m *MultiDoltReader) LoadAllIssues() ([]model.Issue, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	type result struct {
		db     string
		issues []model.Issue
		err    error
	}

	ch := make(chan result, len(m.readers))
	for db, reader := range m.readers {
		go func(db string, r *DoltReader) {
			issues, err := r.LoadIssues()
			ch <- result{db: db, issues: issues, err: err}
		}(db, reader)
	}

	var all []model.Issue
	var errors []string
	for range m.readers {
		res := <-ch
		if res.err != nil {
			debug.Log("multi-dolt: load %s failed: %v", res.db, res.err)
			errors = append(errors, fmt.Sprintf("%s: %v", res.db, res.err))
			continue
		}
		// Tag each issue with its source database
		for i := range res.issues {
			if res.issues[i].SourceRepo == "" {
				res.issues[i].SourceRepo = res.db
			}
		}
		all = append(all, res.issues...)
		debug.Log("multi-dolt: loaded %d issues from %s", len(res.issues), res.db)
	}

	debug.Log("multi-dolt: total %d issues from %d databases (%d errors)",
		len(all), len(m.readers)-len(errors), len(errors))

	return all, nil
}

// GetHeadHashes returns the HEAD hash for each database.
func (m *MultiDoltReader) GetHeadHashes() map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	hashes := make(map[string]string, len(m.readers))
	for db, reader := range m.readers {
		hash, err := reader.GetHeadHash()
		if err != nil {
			continue
		}
		hashes[db] = hash
	}
	return hashes
}

// DBNames returns the list of connected database names.
func (m *MultiDoltReader) DBNames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.dbNames
}

// Close closes all readers.
func (m *MultiDoltReader) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, reader := range m.readers {
		reader.Close()
	}
	m.readers = nil
}
