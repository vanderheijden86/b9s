package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanderheijden86/beadwork/internal/datasource"
	"github.com/vanderheijden86/beadwork/pkg/config"
	"github.com/vanderheijden86/beadwork/pkg/model"
)

func writeSwitchCheckout(t *testing.T, root, name string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Join(dir, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	issue := `{"id":"` + name + `-1","title":"` + name + ` work","status":"open","priority":2,"issue_type":"task"}`
	if err := os.WriteFile(filepath.Join(dir, ".beads", "issues.jsonl"), []byte(issue+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// switchModel shows alpha, with beta and a database-only project in the
// header. Opening never runs for real: tests deliver the result messages.
func switchModel(t *testing.T) (m Model, alpha, beta config.Project) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	alpha = config.Project{Name: "alpha", Path: writeSwitchCheckout(t, root, "alpha")}
	beta = config.Project{Name: "beta", Path: writeSwitchCheckout(t, root, "beta")}
	cfg := config.Config{RecentProjects: []config.RecentProject{
		{Name: alpha.Name, Path: alpha.Path},
		{Name: beta.Name, Path: beta.Path},
		{Name: "remote", Database: "remote_db", Host: "127.0.0.1:1"},
	}}
	issues := []model.Issue{{ID: "alpha-1", Title: "alpha work", Status: model.StatusOpen, IssueType: model.TypeTask, Priority: 2, CreatedAt: time.Now()}}
	m = NewModel(issues, filepath.Join(alpha.Path, ".beads", "issues.jsonl")).
		WithDoltFailure(&DoltFailure{Server: "127.0.0.1:1", Database: "alpha", User: "bd_startup"}).
		WithConfig(cfg, alpha.Name, alpha.Path)
	m.openProject = func(datasource.OpenTarget) (datasource.OpenedProject, *datasource.OpenFailure) {
		t.Fatal("opening must not run inside Update")
		return datasource.OpenedProject{}, nil
	}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	return updated.(Model), alpha, beta
}

func requestSwitch(t *testing.T, m Model, project config.Project) (Model, tea.Cmd) {
	t.Helper()
	updated, cmd := m.Update(SwitchProjectMsg{Project: project})
	return updated.(Model), cmd
}

func serverDown(project string) *datasource.OpenFailure {
	return &datasource.OpenFailure{Reason: datasource.OpenServerDown, Project: project, Server: "127.0.0.1:1", Database: project}
}

func TestSwitchKeepsCurrentProjectWhileOpening(t *testing.T) {
	m, _, beta := switchModel(t)

	m, cmd := requestSwitch(t, m, beta)

	if m.activeProjectName != "alpha" || len(m.issues) != 1 {
		t.Errorf("active = %q with %d issues, want alpha still shown while beta opens", m.activeProjectName, len(m.issues))
	}
	if m.projectSwitch.state != SwitchOpening {
		t.Errorf("switch state = %v, want opening", m.projectSwitch.state)
	}
	if cmd == nil {
		t.Error("a switch must start opening the project")
	}
}

func TestOpenedProjectReplacesCurrentProject(t *testing.T) {
	m, _, beta := switchModel(t)
	m, _ = requestSwitch(t, m, beta)

	updated, _ := m.Update(projectOpenedMsg{generation: m.projectSwitch.generation, project: beta})
	m = updated.(Model)

	if m.activeProjectName != "beta" {
		t.Errorf("active = %q, want beta", m.activeProjectName)
	}
	if m.projectSwitch.state != SwitchIdle {
		t.Errorf("switch state = %v, want idle", m.projectSwitch.state)
	}
}

func TestFailedOpenKeepsCurrentProjectAndShowsReason(t *testing.T) {
	m, _, beta := switchModel(t)
	m, _ = requestSwitch(t, m, beta)
	failure := serverDown("beta")

	updated, _ := m.Update(projectOpenFailedMsg{generation: m.projectSwitch.generation, project: beta, failure: failure})
	m = updated.(Model)

	if m.activeProjectName != "alpha" || len(m.issues) != 1 {
		t.Errorf("active = %q with %d issues, want alpha kept", m.activeProjectName, len(m.issues))
	}
	view := stripANSI(m.View())
	for _, want := range []string{"Cannot open beta", failure.Message(), failure.Try()[0], "Still showing alpha"} {
		if !strings.Contains(view, want) {
			t.Errorf("view lacks %q:\n%s", want, view)
		}
	}
}

func TestLateResultOfSupersededSwitchIsIgnored(t *testing.T) {
	m, _, beta := switchModel(t)
	m, _ = requestSwitch(t, m, beta)
	first := m.projectSwitch.generation
	m, _ = requestSwitch(t, m, config.Project{Name: "remote", Database: "remote_db", Host: "127.0.0.1:1"})

	updated, _ := m.Update(projectOpenedMsg{generation: first, project: beta})
	m = updated.(Model)

	if m.activeProjectName != "alpha" {
		t.Errorf("active = %q, want alpha: the beta result belongs to a superseded switch", m.activeProjectName)
	}
	if m.projectSwitch.state != SwitchOpening || m.projectSwitch.project.Name != "remote" {
		t.Errorf("switch = %+v, want still opening remote", m.projectSwitch)
	}
}

func TestEscCancelsOpeningProject(t *testing.T) {
	m, _, beta := switchModel(t)
	m, _ = requestSwitch(t, m, beta)
	generation := m.projectSwitch.generation

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated, _ = updated.(Model).Update(projectOpenedMsg{generation: generation, project: beta})
	m = updated.(Model)

	if m.projectSwitch.state != SwitchIdle {
		t.Errorf("switch state = %v, want idle after Esc", m.projectSwitch.state)
	}
	if m.activeProjectName != "alpha" {
		t.Errorf("active = %q, want alpha: a cancelled switch must not apply its late result", m.activeProjectName)
	}
}

func TestOpeningTimesOut(t *testing.T) {
	m, _, beta := switchModel(t)
	m, _ = requestSwitch(t, m, beta)

	updated, _ := m.Update(projectOpenDeadlineMsg{generation: m.projectSwitch.generation})
	m = updated.(Model)

	if !m.showOpenFailure || m.openFailure == nil || m.openFailure.Reason != datasource.OpenTimedOut {
		t.Fatalf("failure = %+v shown=%v, want a timed-out popup", m.openFailure, m.showOpenFailure)
	}
	if m.projectSwitch.state != SwitchIdle {
		t.Errorf("switch state = %v, want idle", m.projectSwitch.state)
	}
}

func TestRetryFromFailurePopupOpensProjectAgain(t *testing.T) {
	m, _, beta := switchModel(t)
	m, _ = requestSwitch(t, m, beta)
	updated, _ := m.Update(projectOpenFailedMsg{generation: m.projectSwitch.generation, project: beta, failure: serverDown("beta")})
	m = updated.(Model)

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	m = updated.(Model)

	if m.showOpenFailure {
		t.Error("r must close the failure popup")
	}
	if m.projectSwitch.state != SwitchOpening || m.projectSwitch.project.Name != "beta" || cmd == nil {
		t.Errorf("switch = %+v cmd=%v, want beta opening again", m.projectSwitch, cmd != nil)
	}
}

// A project without a checkout is read as the startup project's Dolt user,
// which is known from metadata even when the startup connection failed.
func TestProjectWithoutCheckoutOpensAsStartupDoltUser(t *testing.T) {
	m, _, _ := switchModel(t)

	m, _ = requestSwitch(t, m, config.Project{Name: "remote", Database: "remote_db", Host: "127.0.0.1:1"})

	target := m.projectSwitch.target
	if target.Dir != "" || target.Dolt == nil {
		t.Fatalf("target = %+v, want the database without a checkout", target)
	}
	if target.Dolt.User != "bd_startup" || target.Dolt.Database != "remote_db" {
		t.Errorf("dolt source = %+v, want remote_db as bd_startup", *target.Dolt)
	}
}

func TestOpenedProjectRecordsOpenedAt(t *testing.T) {
	m, _, beta := switchModel(t)
	m, _ = requestSwitch(t, m, beta)

	updated, _ := m.Update(projectOpenedMsg{generation: m.projectSwitch.generation, project: beta})
	m = updated.(Model)

	for _, p := range m.appConfig.RecentProjects {
		if p.Name == "beta" {
			if p.OpenedAt.IsZero() {
				t.Error("beta has no opened_at after it opened")
			}
			return
		}
	}
	t.Errorf("beta missing from recent projects %+v", m.appConfig.RecentProjects)
}

func TestStartupFailurePopupNamesFallbackProject(t *testing.T) {
	m, _, _ := switchModel(t)
	failure := &datasource.OpenFailure{Reason: datasource.OpenNotAProject, Project: "ghost", Dir: "/work/ghost"}

	m = m.WithStartupFailure(failure)

	view := stripANSI(m.View())
	for _, want := range []string{"Cannot open ghost", failure.Message(), "last project that opened successfully"} {
		if !strings.Contains(view, want) {
			t.Errorf("view lacks %q:\n%s", want, view)
		}
	}
}
