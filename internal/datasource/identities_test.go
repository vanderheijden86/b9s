package datasource

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadIdentityConfig_ReadsBothKeys(t *testing.T) {
	db := openCreatorsDB(t,
		"CREATE TABLE config (`key` TEXT PRIMARY KEY, value TEXT NOT NULL)",
		"INSERT INTO config VALUES ('b9s.identities', '[{\"name\":\"a\",\"kind\":\"human\"}]'), ('claim.pools', 'crew'), ('other', 'x')",
	)
	cfg := readIdentityConfig(db)
	if cfg.Identities != `[{"name":"a","kind":"human"}]` || cfg.ClaimPools != "crew" {
		t.Fatalf("cfg = %+v", cfg)
	}
}

func TestReadIdentityConfig_MissingKeysAreEmpty(t *testing.T) {
	db := openCreatorsDB(t, "CREATE TABLE config (`key` TEXT PRIMARY KEY, value TEXT NOT NULL)")
	if cfg := readIdentityConfig(db); cfg.Identities != "" || cfg.ClaimPools != "" {
		t.Fatalf("cfg = %+v", cfg)
	}
}

func TestReadIdentityConfig_MissingTableIsEmpty(t *testing.T) {
	db := openCreatorsDB(t, `CREATE TABLE issues (id TEXT)`)
	if cfg := readIdentityConfig(db); cfg.Identities != "" || cfg.ClaimPools != "" {
		t.Fatalf("cfg = %+v", cfg)
	}
}

func TestLoadIdentityConfig_JSONLHasNone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "issues.jsonl")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadIdentityConfig(DataSource{Type: SourceTypeJSONLLocal, Path: path})
	if err != nil || cfg != (IdentityConfig{}) {
		t.Fatalf("cfg = %+v, err = %v", cfg, err)
	}
}

func TestLoadIdentityConfig_SQLiteSource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "beads.db")
	db := openSQLiteAt(t, path,
		"CREATE TABLE config (`key` TEXT PRIMARY KEY, value TEXT NOT NULL)",
		"INSERT INTO config VALUES ('claim.pools', 'fable-crew')",
	)
	db.Close()
	cfg, err := LoadIdentityConfig(DataSource{Type: SourceTypeSQLite, Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ClaimPools != "fable-crew" || cfg.SQLUser != "" {
		t.Fatalf("cfg = %+v", cfg)
	}
}
