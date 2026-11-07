// recent_projects_e2e_test.go - End-to-end proof that the header lists the
// startup project plus the persisted recent projects in k9s order, and that
// :project opens the startup server's project table (bd-6e8h).
package main_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b[()][A-Z0-9]`)

func stripTerminalCodes(out []byte) string {
	return ansiEscape.ReplaceAllString(string(out), "")
}

// recentFixture creates a JSONL project whose directory name is the project
// name b9s shows in the header.
func recentFixture(t *testing.T, root, name string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	writeTreeFixture(t, dir, []treeFixtureIssue{
		{ID: name + "-1", Title: name + " work", Status: "open", Priority: 2, IssueType: "task",
			CreatedAt: time.Now().Format(time.RFC3339)},
	})
	return dir
}

func runRecentTUI(t *testing.T, dir string, autoCloseMs int, keys ...keyStep) string {
	t.Helper()
	out, err := runTreeTUI(t, dir, autoCloseMs, keys)
	if err != nil {
		t.Fatalf("TUI run in %s failed: %v\noutput:\n%s", dir, err, out)
	}
	return stripTerminalCodes(out)
}

// savedRecentNames reads the recent list the binary wrote to the isolated
// config home.
func savedRecentNames(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "b9s", "config.yaml"))
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	var cfg struct {
		RecentProjects []struct {
			Name string `yaml:"name"`
		} `yaml:"recent_projects"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parse saved config: %v\n%s", err, data)
	}
	names := make([]string, 0, len(cfg.RecentProjects))
	for _, p := range cfg.RecentProjects {
		names = append(names, p.Name)
	}
	return names
}

func assertHeaderSlot(t *testing.T, screen string, slot int, name string) {
	t.Helper()
	pattern := regexp.MustCompile(`<` + strconv.Itoa(slot) + `>\s+` + regexp.QuoteMeta(name) + `\b`)
	if !pattern.MatchString(screen) {
		t.Errorf("header does not show %s on key %d\nscreen:\n%s", name, slot, screen)
	}
}

func typeCommand(command string) []keyStep {
	steps := []keyStep{kd(":", 200*time.Millisecond)}
	for _, r := range command {
		steps = append(steps, kd(string(r), 60*time.Millisecond))
	}
	return append(steps, kd("\r", 150*time.Millisecond))
}

func TestNewStartupProjectTakesKeyOneE2E(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	alpha := recentFixture(t, root, "alpha")
	beta := recentFixture(t, root, "beta")

	runRecentTUI(t, alpha, 1200)
	screen := runRecentTUI(t, beta, 1200)

	if got, want := savedRecentNames(t), []string{"beta", "alpha"}; !slices.Equal(got, want) {
		t.Errorf("saved recent projects = %v, want %v", got, want)
	}
	assertHeaderSlot(t, screen, 1, "beta")
	assertHeaderSlot(t, screen, 2, "alpha")
}

func TestRevisitedProjectKeepsItsKeyE2E(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	alpha := recentFixture(t, root, "alpha")
	beta := recentFixture(t, root, "beta")
	runRecentTUI(t, alpha, 1200)
	runRecentTUI(t, beta, 1200)

	screen := runRecentTUI(t, alpha, 1200)

	if got, want := savedRecentNames(t), []string{"beta", "alpha"}; !slices.Equal(got, want) {
		t.Errorf("saved recent projects = %v, want %v (a known project keeps its slot)", got, want)
	}
	assertHeaderSlot(t, screen, 2, "alpha")
}

func TestProjectCommandWithoutServerSaysWhyE2E(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	alpha := recentFixture(t, t.TempDir(), "alpha")

	screen := runRecentTUI(t, alpha, 2500, typeCommand("proj")...)

	if !strings.Contains(screen, "Cannot list databases") {
		t.Errorf(":proj on a project without a Dolt server did not say why\nscreen:\n%s", screen)
	}
}

func TestForgetRecentProjectE2E(t *testing.T) {
	for _, locked := range []bool{false, true} {
		t.Run(fmt.Sprintf("lock_recent=%t", locked), func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			root := t.TempDir()
			alpha := recentFixture(t, root, "alpha")
			beta := recentFixture(t, root, "beta")
			configDir := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "b9s")
			if err := os.MkdirAll(configDir, 0o755); err != nil {
				t.Fatal(err)
			}
			settings := fmt.Sprintf(`# personal settings
ui:
  default_view: tree # keep trees
lock_recent: %t
recent_projects:
  - name: alpha
    path: %q
  - name: offline
    database: offline
    host: 127.0.0.1:1
  - name: beta
    path: %q
`, locked, alpha, beta)
			configPath := filepath.Join(configDir, "config.yaml")
			if err := os.WriteFile(configPath, []byte(settings), 0o600); err != nil {
				t.Fatal(err)
			}

			out, err := runTreeTUI(t, alpha, 3500, typeCommand("forget 2"))
			if err != nil {
				t.Fatalf("forget TUI action failed: %v\n%s", err, out)
			}
			frame := stripTerminalCodes([]byte(treeFinalFrame(out)))
			t.Logf("Final TUI frame:\n%s", frame)
			wantNames := []string{"alpha", "beta"}
			wantStatus := "Forgot recent project offline"
			if locked {
				wantNames = []string{"alpha", "offline", "beta"}
				wantStatus = "Recent projects are locked"
				assertHeaderSlot(t, frame, 2, "offline")
			} else {
				assertHeaderSlot(t, frame, 2, "beta")
				if regexp.MustCompile(`<\d>\s+offline\b`).MatchString(frame) {
					t.Errorf("forgotten project remains in final header:\n%s", frame)
				}
			}
			if !strings.Contains(frame, wantStatus) {
				t.Errorf("final frame lacks status %q:\n%s", wantStatus, frame)
			}
			if got := savedRecentNames(t); !slices.Equal(got, wantNames) {
				t.Errorf("saved recent projects = %v, want %v", got, wantNames)
			}
			saved, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatal(err)
			}
			for _, comment := range []string{"# personal settings", "default_view: tree # keep trees"} {
				if !strings.Contains(string(saved), comment) {
					t.Errorf("config lost %q:\n%s", comment, saved)
				}
			}
		})
	}
}

// TestProjectCommandListsServerDatabasesE2E needs a live server. Picking a
// database not yet in the recent list is covered by unit tests, because a
// scoped project user can usually read only its own database.
func TestProjectCommandListsServerDatabasesE2E(t *testing.T) {
	host := os.Getenv("B9S_TEST_DOLT_HOST")
	database := os.Getenv("B9S_TEST_DOLT_CATALOG_DB")
	if host == "" || database == "" || os.Getenv("BEADS_DOLT_PASSWORD") == "" {
		t.Skip("set B9S_TEST_DOLT_HOST, B9S_TEST_DOLT_CATALOG_DB and BEADS_DOLT_PASSWORD to list a live server's databases")
	}
	port := 3306
	if v := os.Getenv("B9S_TEST_DOLT_PORT"); v != "" {
		p, err := strconv.Atoi(v)
		if err != nil {
			t.Fatalf("B9S_TEST_DOLT_PORT = %q: %v", v, err)
		}
		port = p
	}
	user := os.Getenv("B9S_TEST_DOLT_USER")
	if user == "" {
		user = "root"
	}

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := filepath.Join(t.TempDir(), database)
	if err := os.MkdirAll(filepath.Join(dir, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	metadata, err := json.Marshal(map[string]any{
		"dolt_mode":        "server",
		"dolt_server_host": host,
		"dolt_server_port": port,
		"dolt_server_user": user,
		"dolt_database":    database,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".beads", "metadata.json"), metadata, 0o644); err != nil {
		t.Fatal(err)
	}

	screen := runRecentTUI(t, dir, 5000, append([]keyStep{kd("", 1500*time.Millisecond)}, typeCommand("proj")...)...)

	if want := "Projects (" + host + ":" + strconv.Itoa(port) + ")"; !strings.Contains(screen, want) {
		t.Errorf("project table title %q missing\nscreen:\n%s", want, screen)
	}
	row := regexp.MustCompile(`▸ ` + regexp.QuoteMeta(database) + `\s+<1>`)
	if !row.MatchString(screen) {
		t.Errorf("project table does not list %s as recent project <1>\nscreen:\n%s", database, screen)
	}
}
