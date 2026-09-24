package datasource

import (
	"database/sql"
	"fmt"

	"github.com/vanderheijden86/beadwork/pkg/debug"
	"github.com/vanderheijden86/beadwork/pkg/identity"
)

// IdentityConfig is the raw identity configuration a project database holds.
// It is read with SQL rather than through bd, which costs seconds per call.
type IdentityConfig struct {
	Identities string // b9s.identities, a JSON list
	ClaimPools string // claim.pools, comma-separated
	// SQLUser is the Dolt login b9s connected as. It names a workspace
	// credential shared by every agent in it, so it is shown, never used as
	// the creator (ADR 0014).
	SQLUser string
}

// readIdentityConfig returns empty values when the config table or a key is
// missing: a project without identity configuration shows raw names.
func readIdentityConfig(db *sql.DB) IdentityConfig {
	var cfg IdentityConfig
	rows, err := db.Query("SELECT `key`, value FROM config WHERE `key` IN (?, ?)",
		identity.ConfigKey, identity.PoolsConfigKey)
	if err != nil {
		debug.Log("identities: config query failed: %v", err)
		return cfg
	}
	defer rows.Close()
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			continue
		}
		switch key {
		case identity.ConfigKey:
			cfg.Identities = value
		case identity.PoolsConfigKey:
			cfg.ClaimPools = value
		}
	}
	return cfg
}

// IdentityConfig reads the identity keys and the connected SQL user.
func (r *DoltReader) IdentityConfig() IdentityConfig {
	cfg := readIdentityConfig(r.db)
	if err := r.db.QueryRow("SELECT CURRENT_USER()").Scan(&cfg.SQLUser); err != nil {
		debug.Log("identities: CURRENT_USER failed: %v", err)
	}
	return cfg
}

// LoadIdentityConfig reads the identity configuration of a source. JSONL
// sources have no config table and return an empty configuration.
func LoadIdentityConfig(source DataSource) (IdentityConfig, error) {
	switch source.Type {
	case SourceTypeDolt:
		reader, err := NewDoltReader(source)
		if err != nil {
			return IdentityConfig{}, fmt.Errorf("failed to open Dolt source %s: %w", source.Path, err)
		}
		defer reader.Close()
		return reader.IdentityConfig(), nil
	case SourceTypeSQLite:
		reader, err := NewSQLiteReader(source)
		if err != nil {
			return IdentityConfig{}, fmt.Errorf("failed to open SQLite source %s: %w", source.Path, err)
		}
		defer reader.Close()
		return readIdentityConfig(reader.db), nil
	default:
		return IdentityConfig{}, nil
	}
}
