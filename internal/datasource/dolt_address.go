package datasource

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Defaults bd applies when nothing names a port. A local server without a
// configured port is one bd started itself, on a free port it records only in
// the port file; 3306 is kept as the last resort because b9s has always dialled
// it and a hand-run server usually listens there.
const (
	defaultLocalDoltPort  = 3306
	defaultRemoteDoltPort = 3307
	defaultSharedDoltPort = 3308
	doltPortFileName      = "dolt-server.port"
)

// resolveDoltAddress returns the host:port bd itself would dial for the
// project in beadsDir. It follows bd's precedence so that b9s and bd never
// read different servers:
//
//	host: BEADS_DOLT_SERVER_HOST > metadata dolt_server_host > config.yaml dolt.host > 127.0.0.1
//	port: BEADS_DOLT_SERVER_PORT > port file > listener.port of the Dolt data
//	      directory's config.yaml > config.yaml dolt.port > metadata dolt_server_port
//
// The port file belongs to a server bd runs on this machine, so it is ignored
// for a remote host, where the legacy BEADS_DOLT_PORT also applies. In shared
// server mode the port comes from the shared server directory, not the project.
func resolveDoltAddress(beadsDir string, meta beadsMetadata) string {
	yamlFiles := bdConfigFiles(beadsDir)

	host := os.Getenv("BEADS_DOLT_SERVER_HOST")
	if host == "" {
		host = meta.DoltServerHost
	}
	if host == "" {
		host = yamlValue(yamlFiles, "dolt", "host")
	}
	host = strings.Trim(strings.TrimSpace(host), "[]")
	if host == "" {
		host = "127.0.0.1"
	}
	remote := !isLocalDoltHost(host)

	portDir := beadsDir
	doltDir := doltDataDir(beadsDir)
	metaPort := meta.DoltServerPort
	shared := sharedServerMode(yamlFiles)
	if shared {
		if dir, ok := sharedServerDir(); ok {
			portDir = dir
			doltDir = filepath.Join(dir, "dolt")
			// bd reads the metadata port from the shared directory as well,
			// which never holds a project's metadata.json.
			metaPort = 0
		}
	}

	port, fromEnv := envPort("BEADS_DOLT_SERVER_PORT"), false
	if port > 0 {
		fromEnv = true
	}
	if port == 0 && !remote {
		port = readPortFile(filepath.Join(portDir, doltPortFileName))
	}
	if port == 0 {
		port = listenerPort(filepath.Join(doltDir, "config.yaml"))
	}
	if port == 0 {
		port, _ = strconv.Atoi(yamlValue(yamlFiles, "dolt", "port"))
	}
	if port <= 0 {
		port = metaPort
	}
	if remote && !fromEnv {
		if legacy := envPort("BEADS_DOLT_PORT"); legacy > 0 {
			port = legacy
		}
	}
	if port <= 0 {
		switch {
		case shared:
			port = defaultSharedDoltPort
		case remote:
			port = defaultRemoteDoltPort
		default:
			port = defaultLocalDoltPort
		}
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}

func isLocalDoltHost(host string) bool {
	switch strings.ToLower(host) {
	case "", "localhost", "127.0.0.1", "::1", "0.0.0.0":
		return true
	}
	return false
}

func envPort(name string) int {
	port, err := strconv.Atoi(strings.TrimSpace(os.Getenv(name)))
	if err != nil || port <= 0 {
		return 0
	}
	return port
}

func readPortFile(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	port, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || port <= 0 {
		return 0
	}
	return port
}

func doltDataDir(beadsDir string) string {
	if dir := os.Getenv("BEADS_DOLT_DATA_DIR"); dir != "" {
		return dir
	}
	return filepath.Join(beadsDir, "dolt")
}

func sharedServerMode(yamlFiles []string) bool {
	if v := os.Getenv("BEADS_DOLT_SHARED_SERVER"); v == "1" || strings.EqualFold(v, "true") {
		return true
	}
	v := yamlValue(yamlFiles, "dolt", "shared-server")
	return v == "1" || strings.EqualFold(v, "true")
}

func sharedServerDir() (string, bool) {
	if dir := os.Getenv("BEADS_SHARED_SERVER_DIR"); dir != "" {
		return dir, true
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}
	return filepath.Join(home, ".beads", "shared-server"), true
}

// bdConfigFiles lists the config.yaml files bd reads, highest priority first.
func bdConfigFiles(beadsDir string) []string {
	var files []string
	if dir := os.Getenv("BEADS_DIR"); dir != "" {
		files = append(files, filepath.Join(dir, "config.yaml"))
	}
	files = append(files, filepath.Join(beadsDir, "config.yaml"))
	if home, err := os.UserHomeDir(); err == nil {
		files = append(files, filepath.Join(home, ".config", "bd", "config.yaml"))
	}
	if dir, err := os.UserConfigDir(); err == nil {
		files = append(files, filepath.Join(dir, "bd", "config.yaml"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		files = append(files, filepath.Join(home, ".beads", "config.yaml"))
	}
	return files
}

// yamlValue returns section.key from the first file that sets it. bd reads
// its config through viper, which accepts both a nested section and a flat
// "section.key" entry.
func yamlValue(files []string, section, key string) string {
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var doc map[string]any
		if yaml.Unmarshal(data, &doc) != nil {
			continue
		}
		if nested, ok := doc[section].(map[string]any); ok {
			if v, ok := nested[key]; ok && v != nil {
				return scalarString(v)
			}
		}
		if v, ok := doc[section+"."+key]; ok && v != nil {
			return scalarString(v)
		}
	}
	return ""
}

func scalarString(v any) string {
	switch v := v.(type) {
	case string:
		return v
	case int:
		return strconv.Itoa(v)
	case bool:
		return strconv.FormatBool(v)
	}
	return ""
}

// listenerPort reads listener.port from a Dolt sql-server config.yaml.
func listenerPort(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	var cfg struct {
		Listener struct {
			Port int `yaml:"port"`
		} `yaml:"listener"`
	}
	if yaml.Unmarshal(data, &cfg) != nil || cfg.Listener.Port <= 0 {
		return 0
	}
	return cfg.Listener.Port
}
