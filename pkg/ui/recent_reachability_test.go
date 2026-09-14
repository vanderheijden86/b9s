package ui

import (
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
