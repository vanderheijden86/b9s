package datasource

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverDoltDBs_SkipsProjectWithoutPath(t *testing.T) {
	startup := t.TempDir()
	beadsDir := filepath.Join(startup, ".beads")
	if err := os.Mkdir(beadsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	metadata := `{"dolt_mode":"server","dolt_server_host":"127.0.0.1","dolt_server_port":3306,"dolt_server_user":"reader","dolt_database":"startup_db"}`
	if err := os.WriteFile(filepath.Join(beadsDir, "metadata.json"), []byte(metadata), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(startup)

	dbs := DiscoverDoltDBs(map[string]string{"remote": ""})

	if len(dbs) != 0 {
		t.Errorf("DiscoverDoltDBs for a project without a path = %+v; it read the working directory", dbs)
	}
}
