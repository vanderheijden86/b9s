package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/go-sql-driver/mysql"

	"github.com/vanderheijden86/beadwork/pkg/config"
)

// projectTableModel is a sized model whose startup project connects to the
// shared server as bd_b9s, with b9s already the first recent project.
func projectTableModel(t *testing.T) (Model, string) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	checkout := t.TempDir()
	if err := os.Mkdir(filepath.Join(checkout, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{RecentProjects: []config.RecentProject{
		{Name: "b9s", Database: "b9s", Host: "127.0.0.1:3306", Path: checkout},
	}}
	m := NewModel(nil, "").
		WithDoltFailure(&DoltFailure{Server: "127.0.0.1:3306", Database: "b9s", User: "bd_b9s"}).
		WithConfig(cfg, "b9s", checkout)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	return updated.(Model), checkout
}

func loadedProjectTable(t *testing.T, databases ...string) (Model, string) {
	t.Helper()
	m, checkout := projectTableModel(t)
	updated, _ := m.Update(OpenProjectTableMsg{})
	updated, _ = updated.(Model).Update(projectTableLoadedMsg{databases: databases})
	return updated.(Model), checkout
}

func TestOpenProjectTableStartsLoading(t *testing.T) {
	m, _ := projectTableModel(t)

	updated, cmd := m.Update(OpenProjectTableMsg{})
	m = updated.(Model)

	if !m.showProjectTable {
		t.Fatal("OpenProjectTableMsg did not show the project table")
	}
	if got := m.projectTable.State(); got != ProjectTableLoading {
		t.Errorf("table state = %v, want loading", got)
	}
	if cmd == nil {
		t.Error("opening the table must start loading the database list")
	}
}

func TestProjectTableListsDatabasesAndMarksRecentOnes(t *testing.T) {
	m, _ := loadedProjectTable(t, "alpha", "b9s")

	if got := m.projectTable.State(); got != ProjectTableLoaded {
		t.Fatalf("table state = %v, want loaded", got)
	}
	view := stripANSI(m.View())
	for _, want := range []string{"alpha", "b9s", "<1>"} {
		if !strings.Contains(view, want) {
			t.Errorf("project table view lacks %q:\n%s", want, view)
		}
	}
}

func TestProjectTableShowsWhyLoadingFailed(t *testing.T) {
	m, _ := projectTableModel(t)
	updated, _ := m.Update(OpenProjectTableMsg{})

	updated, _ = updated.(Model).Update(projectTableLoadedMsg{err: &mysql.MySQLError{Number: 1045, Message: "Access denied"}})
	m = updated.(Model)

	if got := m.projectTable.State(); got != ProjectTableFailed {
		t.Fatalf("table state = %v, want failed", got)
	}
	if view := stripANSI(m.View()); !strings.Contains(view, "access denied") {
		t.Errorf("failed project table does not say why:\n%s", view)
	}
}

func TestProjectTableEnterSwitchesToSelectedProject(t *testing.T) {
	m, _ := loadedProjectTable(t, "alpha", "b9s")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	if m.showProjectTable {
		t.Error("Enter must close the project table")
	}
	if cmd == nil {
		t.Fatal("Enter produced no command")
	}
	switchMsg, ok := cmd().(SwitchProjectMsg)
	if !ok {
		t.Fatalf("Enter produced %T, want SwitchProjectMsg", cmd())
	}
	want := config.Project{Name: "alpha", Database: "alpha", Host: "127.0.0.1:3306"}
	if switchMsg.Project != want {
		t.Errorf("switch project = %+v, want %+v", switchMsg.Project, want)
	}
	if first := m.appConfig.RecentProjects[0]; first.Database == "alpha" {
		t.Errorf("alpha is already a recent project before it opened: %+v", m.appConfig.RecentProjects)
	}
}

// A project enters the recent list only once it has opened, so a database
// that fails to open never takes a number key.
func TestProjectTableProjectBecomesRecentOnceItOpens(t *testing.T) {
	m, _ := loadedProjectTable(t, "alpha", "b9s")
	// A closed port: applying the switch starts a Dolt watcher, which must not
	// reach a real server from a unit test.
	alpha := config.Project{Name: "alpha", Database: "alpha", Host: "127.0.0.1:1"}
	updated, _ := m.Update(SwitchProjectMsg{Project: alpha})
	m = updated.(Model)

	updated, _ = m.Update(projectOpenedMsg{generation: m.projectSwitch.generation, project: alpha})
	m = updated.(Model)

	if first := m.appConfig.RecentProjects[0]; first.Database != "alpha" {
		t.Errorf("first recent project = %+v, want alpha prepended", first)
	}
	saved, err := os.ReadFile(config.ConfigPath())
	if err != nil || !strings.Contains(string(saved), "alpha") {
		t.Errorf("recent list not saved with alpha (err=%v):\n%s", err, saved)
	}
}

func TestProjectTableEnterOnRecentProjectKeepsItsCheckout(t *testing.T) {
	m, checkout := loadedProjectTable(t, "alpha", "b9s")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if cmd == nil {
		t.Fatal("Enter produced no command")
	}
	switchMsg, ok := cmd().(SwitchProjectMsg)
	if !ok {
		t.Fatalf("Enter produced %T, want SwitchProjectMsg", cmd())
	}
	if switchMsg.Project.Path != checkout {
		t.Errorf("switch path = %q, want the recent project's checkout %q", switchMsg.Project.Path, checkout)
	}
}

func TestProjectTableEscCloses(t *testing.T) {
	m, _ := loadedProjectTable(t, "alpha")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)

	if m.showProjectTable {
		t.Error("Esc must close the project table")
	}
}

func TestStartupDoltHostSurvivesFailedConnectionAndSwitch(t *testing.T) {
	m, _ := projectTableModel(t)
	if m.startupDoltHost != "127.0.0.1:3306" {
		t.Fatalf("startup host = %q, want the failed connection's server", m.startupDoltHost)
	}

	updated, _ := m.Update(SwitchProjectMsg{Project: config.Project{Name: "other", Database: "other", Host: "127.0.0.1:1"}})
	m = updated.(Model)

	if m.startupDoltHost != "127.0.0.1:3306" {
		t.Errorf("startup host after a switch = %q, want it unchanged", m.startupDoltHost)
	}
}
