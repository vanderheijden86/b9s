package datasource

import (
	"os"
	"path/filepath"
	"testing"
)

// isolateDoltAddressEnv clears every input bd reads for a server address and
// gives the test its own home directory, so the developer's shell and global
// bd config cannot leak into a case.
func isolateDoltAddressEnv(t *testing.T) (home string) {
	t.Helper()
	for _, name := range []string{
		"BEADS_DOLT_SERVER_PORT", "BEADS_DOLT_PORT", "BEADS_DOLT_SERVER_HOST",
		"BEADS_DOLT_SHARED_SERVER", "BEADS_SHARED_SERVER_DIR", "BEADS_DOLT_DATA_DIR", "BEADS_DIR",
	} {
		t.Setenv(name, "")
	}
	home = t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	return home
}

func writeFileAll(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestResolveDoltAddressFollowsBDPrecedence(t *testing.T) {
	tests := []struct {
		name  string
		meta  beadsMetadata
		env   map[string]string
		files map[string]string // relative to .beads; "~/" is the home directory
		want  string
	}{
		{name: "nothing configured", want: "127.0.0.1:3306"},
		{name: "metadata port", meta: beadsMetadata{DoltServerPort: 4000}, want: "127.0.0.1:4000"},
		{
			name:  "port file of a server bd started beats metadata",
			meta:  beadsMetadata{DoltServerPort: 4000},
			files: map[string]string{"dolt-server.port": "51234\n"},
			want:  "127.0.0.1:51234",
		},
		{
			name:  "environment beats the port file",
			env:   map[string]string{"BEADS_DOLT_SERVER_PORT": "5000"},
			files: map[string]string{"dolt-server.port": "51234"},
			want:  "127.0.0.1:5000",
		},
		{
			name:  "project config.yaml beats metadata",
			meta:  beadsMetadata{DoltServerPort: 4000},
			files: map[string]string{"config.yaml": "dolt:\n  port: 4100\n"},
			want:  "127.0.0.1:4100",
		},
		{
			name:  "flat dotted key in config.yaml",
			files: map[string]string{"config.yaml": "dolt.port: 4200\n"},
			want:  "127.0.0.1:4200",
		},
		{
			name: "project config.yaml beats global config",
			files: map[string]string{
				"config.yaml":              "dolt:\n  port: 4100\n",
				"~/.config/bd/config.yaml": "dolt:\n  port: 4300\n",
			},
			want: "127.0.0.1:4100",
		},
		{
			name:  "global config applies without a project value",
			files: map[string]string{"~/.config/bd/config.yaml": "dolt:\n  port: 4300\n"},
			want:  "127.0.0.1:4300",
		},
		{
			name:  "listener.port of the Dolt data directory beats config.yaml",
			files: map[string]string{"dolt/config.yaml": "listener:\n  port: 4400\n", "config.yaml": "dolt:\n  port: 4100\n"},
			want:  "127.0.0.1:4400",
		},
		{
			name:  "port file beats listener.port",
			files: map[string]string{"dolt/config.yaml": "listener:\n  port: 4400\n", "dolt-server.port": "51234"},
			want:  "127.0.0.1:51234",
		},
		{
			name:  "unreadable port file is skipped",
			meta:  beadsMetadata{DoltServerPort: 4000},
			files: map[string]string{"dolt-server.port": "not a port"},
			want:  "127.0.0.1:4000",
		},
		{
			name:  "remote host ignores the local port file and defaults to 3307",
			meta:  beadsMetadata{DoltServerHost: "db.example"},
			files: map[string]string{"dolt-server.port": "51234"},
			want:  "db.example:3307",
		},
		{
			name:  "remote host keeps a configured port",
			meta:  beadsMetadata{DoltServerHost: "db.example", DoltServerPort: 3306},
			files: map[string]string{"dolt-server.port": "51234"},
			want:  "db.example:3306",
		},
		{
			name: "remote host honours the legacy port variable",
			meta: beadsMetadata{DoltServerHost: "db.example", DoltServerPort: 3306},
			env:  map[string]string{"BEADS_DOLT_PORT": "3310"},
			want: "db.example:3310",
		},
		{
			name: "host from the environment",
			meta: beadsMetadata{DoltServerHost: "db.example"},
			env:  map[string]string{"BEADS_DOLT_SERVER_HOST": "127.0.0.1"},
			want: "127.0.0.1:3306",
		},
		{
			name:  "host from config.yaml when metadata has none",
			files: map[string]string{"config.yaml": "dolt:\n  host: db.example\n"},
			want:  "db.example:3307",
		},
		{
			name:  "IPv6 loopback",
			meta:  beadsMetadata{DoltServerHost: "::1"},
			files: map[string]string{"dolt-server.port": "51234"},
			want:  "[::1]:51234",
		},
		{
			name: "shared server reads the shared port file, not the project one",
			env:  map[string]string{"BEADS_DOLT_SHARED_SERVER": "1"},
			files: map[string]string{
				"dolt-server.port":                        "51234",
				"~/.beads/shared-server/dolt-server.port": "3399",
			},
			want: "127.0.0.1:3399",
		},
		{
			name:  "shared server defaults to 3308",
			files: map[string]string{"config.yaml": "dolt:\n  shared-server: true\n"},
			want:  "127.0.0.1:3308",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := isolateDoltAddressEnv(t)
			for name, value := range tt.env {
				t.Setenv(name, value)
			}
			beadsDir := filepath.Join(t.TempDir(), ".beads")
			for rel, content := range tt.files {
				path := filepath.Join(beadsDir, rel)
				if len(rel) > 2 && rel[:2] == "~/" {
					path = filepath.Join(home, rel[2:])
				}
				writeFileAll(t, path, content)
			}

			if got := resolveDoltAddress(beadsDir, tt.meta); got != tt.want {
				t.Errorf("resolveDoltAddress = %s, want %s", got, tt.want)
			}
		})
	}
}

// A project that bd set up with its own server has no port in metadata.json:
// bd starts the server on a free port and records it in the port file.
func TestDiscoverDoltSourcesDialsThePortBDRecorded(t *testing.T) {
	isolateDoltAddressEnv(t)
	beadsDir := filepath.Join(t.TempDir(), ".beads")
	writeFileAll(t, filepath.Join(beadsDir, "metadata.json"), `{"backend":"dolt","dolt_mode":"server","dolt_database":"proj"}`)
	writeFileAll(t, filepath.Join(beadsDir, "dolt-server.port"), "51234")

	sources, err := discoverDoltSources(beadsDir, DiscoveryOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 1 || sources[0].Path != "127.0.0.1:51234" {
		t.Fatalf("sources = %+v, want one Dolt source at 127.0.0.1:51234", sources)
	}
}
