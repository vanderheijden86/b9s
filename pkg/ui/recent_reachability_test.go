package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanderheijden86/beadwork/internal/datasource"
	"github.com/vanderheijden86/beadwork/pkg/config"
)

func TestProjectKeyDistinguishesProjectsWithoutCheckout(t *testing.T) {
	first := projectKey(config.Project{Name: "first", Database: "first_db", Host: "127.0.0.1:3306"})
	second := projectKey(config.Project{Name: "second", Database: "second_db", Host: "127.0.0.1:3306"})

	if first == "" || second == "" || first == second {
		t.Errorf("keys = %q and %q; projects without a checkout must not share a count cache key", first, second)
	}
	if got := projectKey(config.Project{Name: "local", Path: "/work/local"}); got != "/work/local" {
		t.Errorf("checkout key = %q, want its path", got)
	}
}

func TestProjectCountsRecordServerDownForUnreachableDatabase(t *testing.T) {
	remote := config.Project{Name: "remote", Database: "remote_db", Host: "127.0.0.1:1"}

	msg := loadProjectCountsCmd([]config.Project{remote}, "reader")()

	loaded, ok := msg.(projectCountsLoadedMsg)
	if !ok {
		t.Fatalf("loadProjectCountsCmd produced %T, want projectCountsLoadedMsg", msg)
	}
	if got := loaded.reach[projectKey(remote)]; got != datasource.ReachServerDown {
		t.Errorf("reachability = %v, want server down for a closed port", got)
	}
	if counts, found := loaded.counts[projectKey(remote)]; found {
		t.Errorf("unreachable project has counts %+v", counts)
	}
}

func TestHeaderMarksUnreachableProject(t *testing.T) {
	remote := config.RecentProject{Name: "remote", Database: "remote_db", Host: "127.0.0.1:1"}
	m := NewModel(nil, "").WithConfig(config.Config{RecentProjects: []config.RecentProject{remote}}, "startup", t.TempDir())
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = updated.(Model)
	remoteKey := projectKey(config.Project{Name: remote.Name, Database: remote.Database, Host: remote.Host})

	updated, _ = m.Update(projectCountsLoadedMsg{
		counts: map[string]projectCounts{},
		reach:  map[string]datasource.Reachability{remoteKey: datasource.ReachServerDown},
	})
	m = updated.(Model)

	found := false
	for _, entry := range m.buildProjectEntries() {
		if entry.Project.Name == "remote" {
			found = true
			if entry.Reachability != datasource.ReachServerDown {
				t.Errorf("remote entry reachability = %v, want server down", entry.Reachability)
			}
		}
	}
	if !found {
		t.Fatal("remote project missing from header entries")
	}
	if view := m.projectPicker.View(); !strings.Contains(view, "✗") {
		t.Errorf("header does not mark the unreachable project:\n%s", view)
	}
}

func TestAllProjectsDBsIncludeProjectWithoutCheckout(t *testing.T) {
	remote := config.RecentProject{Name: "remote", Database: "remote_db", Host: "10.0.0.5:3306"}
	m := NewModel(nil, "").
		WithDoltFailure(&DoltFailure{User: "bd_startup"}).
		WithConfig(config.Config{RecentProjects: []config.RecentProject{remote}}, "startup", t.TempDir())

	dbs := m.allProjectsDBs()

	want := datasource.DoltDBInfo{Name: "remote", Host: "10.0.0.5:3306", User: "bd_startup", Database: "remote_db"}
	for _, db := range dbs {
		if db == want {
			return
		}
	}
	t.Errorf("all-projects databases = %+v, want to include %+v", dbs, want)
}

// writeCheckoutWithStaleExport returns a checkout whose metadata names a Dolt
// server that refuses every connection, beside an issues.jsonl export.
func writeCheckoutWithStaleExport(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	beadsDir := filepath.Join(dir, ".beads")
	if err := os.Mkdir(beadsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	metadata := `{"database":"dolt","dolt_mode":"server","dolt_server_host":"127.0.0.1","dolt_server_port":1,"dolt_server_user":"reader","dolt_database":"shared"}`
	if err := os.WriteFile(filepath.Join(beadsDir, "metadata.json"), []byte(metadata), 0o644); err != nil {
		t.Fatal(err)
	}
	stale := `{"id":"old-1","title":"Stale export","status":"open","issue_type":"task","priority":2}` + "\n"
	if err := os.WriteFile(filepath.Join(beadsDir, "issues.jsonl"), []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestProjectCountsReportCheckoutServerFailureInsteadOfExport(t *testing.T) {
	checkout := config.Project{Name: "checkout", Path: writeCheckoutWithStaleExport(t)}

	msg := loadProjectCountsCmd([]config.Project{checkout}, "reader")()

	loaded, ok := msg.(projectCountsLoadedMsg)
	if !ok {
		t.Fatalf("loadProjectCountsCmd produced %T, want projectCountsLoadedMsg", msg)
	}
	if got := loaded.reach[projectKey(checkout)]; got != datasource.ReachServerDown {
		t.Errorf("reachability = %v, want server down; the JSONL export must not stand in for the database", got)
	}
	if counts, found := loaded.counts[projectKey(checkout)]; found {
		t.Errorf("checkout with a refused database has counts %+v from its export", counts)
	}
}

// deniedHeaderModel is a header of startup, a project the startup user may
// not read, and a readable one, after the counts have loaded.
func deniedHeaderModel(t *testing.T) Model {
	t.Helper()
	denied := config.RecentProject{Name: "denied", Database: "denied_db", Host: "127.0.0.1:3306"}
	other := config.RecentProject{Name: "other", Database: "other_db", Host: "127.0.0.1:3306"}
	m := NewModel(nil, "").
		WithDoltSource(datasource.DataSource{Type: datasource.SourceTypeDolt, Path: "127.0.0.1:3306", User: "reader"}).
		WithConfig(config.Config{RecentProjects: []config.RecentProject{denied, other}}, "startup", t.TempDir())
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = updated.(Model)

	updated, _ = m.Update(projectCountsLoadedMsg{
		counts: map[string]projectCounts{},
		reach: map[string]datasource.Reachability{
			projectKey(config.Project{Name: "denied", Database: "denied_db", Host: "127.0.0.1:3306"}): datasource.ReachDenied,
			projectKey(config.Project{Name: "other", Database: "other_db", Host: "127.0.0.1:3306"}):   datasource.ReachReachable,
		},
	})
	return updated.(Model)
}

func TestHeaderHidesProjectTheStartupUserCannotRead(t *testing.T) {
	m := deniedHeaderModel(t)

	var names []string
	for _, entry := range m.buildProjectEntries() {
		names = append(names, entry.Project.Name)
	}
	if strings.Join(names, ",") != "startup,other" {
		t.Errorf("header entries = %v, want the denied project hidden", names)
	}
	if view := m.projectPicker.View(); strings.Contains(view, "denied") {
		t.Errorf("header still shows a project the startup user cannot read:\n%s", view)
	}

	// Number keys follow the visible rows, so the readable project moves up.
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	if cmd == nil {
		t.Fatal("key 2 produced no command")
	}
	switchMsg, ok := cmd().(SwitchProjectMsg)
	if !ok {
		t.Fatalf("key 2 produced %T, want SwitchProjectMsg", cmd())
	}
	if switchMsg.Project.Name != "other" {
		t.Errorf("key 2 opens %q, want the readable project shown as <2>", switchMsg.Project.Name)
	}

	slots := m.projectTableSlots()
	if slots["other_db"] != 2 {
		t.Errorf(":project slot for other_db = %d, want 2", slots["other_db"])
	}
	if slot, found := slots["denied_db"]; found {
		t.Errorf(":project marks the hidden project as recent <%d>", slot)
	}
}

func TestHeaderKeepsActiveProjectWhoseDatabaseIsDenied(t *testing.T) {
	startupPath := t.TempDir()
	m := NewModel(nil, "").WithConfig(config.Config{}, "startup", startupPath)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = updated.(Model)

	updated, _ = m.Update(projectCountsLoadedMsg{
		counts: map[string]projectCounts{},
		reach:  map[string]datasource.Reachability{projectKey(config.Project{Name: "startup", Path: startupPath}): datasource.ReachDenied},
	})
	m = updated.(Model)

	entries := m.buildProjectEntries()
	if len(entries) != 1 || entries[0].Project.Name != "startup" {
		t.Errorf("header entries = %+v, want the active project kept even when its database is denied", entries)
	}
	if m.activeProjectSlot() != 1 {
		t.Errorf("active project slot = %d, want 1", m.activeProjectSlot())
	}
}
