package ui

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanderheijden86/b9s/internal/datasource"
	"github.com/vanderheijden86/b9s/pkg/config"
	"github.com/vanderheijden86/b9s/pkg/model"
)

func TestProjectSwitchTreePersistence(t *testing.T) {
	for _, saved := range []string{"bookmarked", "empty-bookmarks", "missing"} {
		t.Run(saved, func(t *testing.T) {
			m, alpha, beta := switchModel(t)
			defer func() { m.Stop() }()
			m.tree.Build(nil)
			m.tree.Build(refreshExpansionIssues())
			expandToLeaf(t, &m.tree)
			m.tree.ToggleBookmark()
			alphaState := filepath.Join(alpha.Path, ".beads", treeStateFileName)
			before, err := os.ReadFile(alphaState)
			if err != nil {
				t.Fatal(err)
			}

			betaState := filepath.Join(beta.Path, ".beads", treeStateFileName)
			if saved != "missing" {
				state := DefaultTreeState()
				state.Expanded["epic"] = false
				if saved == "bookmarked" {
					state.Bookmarks = []string{"epic"}
				}
				data, err := json.Marshal(state)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(betaState, data, 0o644); err != nil {
					t.Fatal(err)
				}
			}

			m, _ = requestSwitch(t, m, beta)
			updated, _ := m.Update(projectOpenedMsg{generation: m.projectSwitch.generation, project: beta})
			m = updated.(Model)
			m.tree.Build(refreshExpansionIssues())
			wantRows := 1
			if saved == "missing" {
				wantRows = 2
			}
			if got := m.tree.NodeCount(); got != wantRows {
				t.Errorf("beta has %d rows, want %d from its own expansion state", got, wantRows)
			}
			wantBookmarks := 0
			if saved == "bookmarked" {
				wantBookmarks = 1
				if !m.tree.bookmarks["epic"] {
					t.Error("beta's saved bookmark was not loaded")
				}
			}
			if len(m.tree.bookmarks) != wantBookmarks {
				t.Errorf("beta bookmarks = %v, want %d bookmarks", m.tree.bookmarks, wantBookmarks)
			}
			m.tree.SelectByID("epic")
			m.tree.ToggleBookmark()
			after, err := os.ReadFile(alphaState)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Error("saving beta overwrote alpha's state")
			}
			if _, err := os.ReadFile(betaState); err != nil {
				t.Errorf("beta state was not saved: %v", err)
			}
			savedBeta := NewModel(refreshExpansionIssues(), "").WithConfig(config.Config{}, beta.Name, beta.Path)
			defer savedBeta.Stop()
			if savedBeta.tree.bookmarks["epic"] != (saved != "bookmarked") || savedBeta.tree.bookmarks["subtask"] {
				t.Errorf("reopened beta bookmarks = %v", savedBeta.tree.bookmarks)
			}

			m, _ = requestSwitch(t, m, alpha)
			updated, _ = m.Update(projectOpenedMsg{generation: m.projectSwitch.generation, project: alpha})
			m = updated.(Model)
			m.tree.Build(refreshExpansionIssues())
			if m.tree.NodeCount() != 4 || !m.tree.bookmarks["subtask"] || len(m.tree.bookmarks) != 1 {
				t.Errorf("return to alpha: rows=%d bookmarks=%v, want 4 rows and subtask bookmark", m.tree.NodeCount(), m.tree.bookmarks)
			}
		})
	}
}

func TestDoltOnlyStartupTreePersistence(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := writeEmbeddedCheckout(t, t.TempDir(), "dolt-only")
	m := NewModel(refreshExpansionIssues(), "").WithConfig(config.Config{}, "dolt-only", dir)
	defer m.Stop()
	expandToLeaf(t, &m.tree)
	m.tree.ToggleBookmark()
	if _, err := os.Stat(filepath.Join(dir, ".beads", "issues.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("fixture must have no issues.jsonl: %v", err)
	}
	if _, err := os.ReadFile(filepath.Join(dir, ".beads", treeStateFileName)); err != nil {
		t.Errorf("Dolt-only state was not saved: %v", err)
	}

	reopened := NewModel(refreshExpansionIssues(), "").WithConfig(config.Config{}, "dolt-only", dir)
	defer reopened.Stop()
	if reopened.tree.NodeCount() != 4 || !reopened.tree.bookmarks["subtask"] {
		t.Errorf("reopened Dolt-only project: rows=%d bookmarks=%v, want 4 rows and subtask bookmark", reopened.tree.NodeCount(), reopened.tree.bookmarks)
	}
}

func TestDoltOnlySwitchTreePersistence(t *testing.T) {
	m, alpha, _ := switchModel(t)
	defer func() { m.Stop() }()
	m.tree.ToggleBookmark()
	alphaState := filepath.Join(alpha.Path, ".beads", treeStateFileName)
	before, err := os.ReadFile(alphaState)
	if err != nil {
		t.Fatal(err)
	}
	dir := writeEmbeddedCheckout(t, t.TempDir(), "dolt-only")
	project := config.Project{Name: "dolt-only", Path: dir}
	m, _ = requestSwitch(t, m, project)
	updated, _ := m.Update(projectOpenedMsg{generation: m.projectSwitch.generation, project: project})
	m = updated.(Model)
	m.tree.Build(refreshExpansionIssues())
	expandToLeaf(t, &m.tree)
	m.tree.ToggleBookmark()
	if _, err := os.ReadFile(filepath.Join(dir, ".beads", treeStateFileName)); err != nil {
		t.Errorf("Dolt-only project state was not saved: %v", err)
	}
	after, err := os.ReadFile(alphaState)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Error("saving Dolt-only project overwrote alpha's state")
	}
}

func TestStartupTreePersistenceUsesResolvedDirectory(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	projectPath := writeSwitchCheckout(t, root, "checkout")
	resolvedDir := filepath.Join(root, "custom-beads")
	newModel := func() Model {
		return NewModel(refreshExpansionIssues(), "").
			WithConfig(config.Config{}, "checkout", projectPath).
			WithBeadsDir(resolvedDir)
	}
	m := newModel()
	defer m.Stop()
	expandToLeaf(t, &m.tree)
	m.tree.ToggleBookmark()
	if _, err := os.ReadFile(TreeStatePath(resolvedDir)); err != nil {
		t.Fatalf("resolved directory has no tree state: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectPath, ".beads", treeStateFileName)); !os.IsNotExist(err) {
		t.Fatalf("startup wrote tree state outside the resolved directory: %v", err)
	}
	reopened := newModel()
	defer reopened.Stop()
	if reopened.tree.NodeCount() != 4 || !reopened.tree.bookmarks["subtask"] {
		t.Errorf("resolved directory was not restored: rows=%d bookmarks=%v", reopened.tree.NodeCount(), reopened.tree.bookmarks)
	}
}

func TestProjectSwitchWithoutCheckoutDisablesTreePersistence(t *testing.T) {
	m, alpha, _ := switchModel(t)
	defer func() { m.Stop() }()
	m.tree.ToggleBookmark()
	m.tree.ToggleFlatMode()
	statePath := filepath.Join(alpha.Path, ".beads", treeStateFileName)
	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	remote := config.Project{Name: "remote", Database: "remote_db", Host: "127.0.0.1:1"}
	m, _ = requestSwitch(t, m, remote)
	updated, _ := m.Update(projectOpenedMsg{generation: m.projectSwitch.generation, project: remote})
	m = updated.(Model)
	m.tree.Build(refreshExpansionIssues())
	if m.tree.beadsDir != "" || len(m.tree.bookmarks) != 0 || m.tree.IsFlatMode() {
		t.Errorf("checkout-free project inherited persistence: dir=%q bookmarks=%v list=%v", m.tree.beadsDir, m.tree.bookmarks, m.tree.IsFlatMode())
	}
	m.tree.ToggleBookmark()
	after, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Error("checkout-free project overwrote alpha's state")
	}
}

func TestDatabaseOnlyProjectSwitchClearsTreeState(t *testing.T) {
	m, _, _ := switchModel(t)
	defer func() { m.Stop() }()
	for i, name := range []string{"remote-a", "remote-b"} {
		project := config.Project{Name: name, Database: name, Host: "127.0.0.1:1"}
		m, _ = requestSwitch(t, m, project)
		updated, _ := m.Update(projectOpenedMsg{generation: m.projectSwitch.generation, project: project})
		m = updated.(Model)
		m.tree.Build(refreshExpansionIssues())
		if i == 0 {
			expandToLeaf(t, &m.tree)
			m.tree.SelectByID("epic")
			m.tree.ToggleBookmark()
			m.tree.ToggleFlatMode()
		}
	}
	if len(m.tree.bookmarks) != 0 {
		t.Errorf("remote-b inherited remote-a bookmarks: %v", m.tree.bookmarks)
	}
	if m.tree.IsFlatMode() {
		t.Error("remote-b inherited remote-a list layout")
	}
	if m.tree.NodeCount() != 2 {
		t.Errorf("remote-b has %d visible rows, want default expansion with 2 rows", m.tree.NodeCount())
	}
}

func TestDatabaseOnlyTreeStateSurvivesRefreshAndFailedSwitch(t *testing.T) {
	for _, event := range []string{"refresh", "failed-switch"} {
		t.Run(event, func(t *testing.T) {
			m, _, _ := switchModel(t)
			defer func() { m.Stop() }()
			project := config.Project{Name: "remote-a", Database: "remote-a", Host: "127.0.0.1:1"}
			m, _ = requestSwitch(t, m, project)
			updated, _ := m.Update(projectOpenedMsg{generation: m.projectSwitch.generation, project: project})
			m = updated.(Model)
			m.tree.Build(refreshExpansionIssues())
			expandToLeaf(t, &m.tree)
			m.tree.SelectByID("epic")
			m.tree.ToggleBookmark()
			m.tree.ToggleFlatMode()

			if event == "refresh" {
				m.tree.Build(refreshExpansionIssues())
			} else {
				target := config.Project{Name: "remote-b", Database: "remote-b", Host: "127.0.0.1:1"}
				m, _ = requestSwitch(t, m, target)
				updated, _ = m.Update(projectOpenFailedMsg{generation: m.projectSwitch.generation, project: target, failure: serverDown(target.Name)})
				m = updated.(Model)
			}
			if m.activeProjectName != project.Name || m.tree.beadsDir != "" {
				t.Errorf("active project changed: name=%q dir=%q", m.activeProjectName, m.tree.beadsDir)
			}
			if len(m.tree.bookmarks) != 1 || !m.tree.bookmarks["epic"] || !m.tree.IsFlatMode() {
				t.Errorf("project state was lost: bookmarks=%v list=%v", m.tree.bookmarks, m.tree.IsFlatMode())
			}
			if !m.tree.issueMap["task"].Expanded || !m.tree.issueMap["subtask"].Expanded {
				t.Error("project expansion was lost")
			}
		})
	}
}

func TestFailedProjectSwitchKeepsTreePersistence(t *testing.T) {
	m, alpha, beta := switchModel(t)
	defer m.Stop()
	m, _ = requestSwitch(t, m, beta)
	updated, _ := m.Update(projectOpenFailedMsg{generation: m.projectSwitch.generation, project: beta, failure: serverDown(beta.Name)})
	m = updated.(Model)
	m.tree.ToggleBookmark()
	if _, err := os.ReadFile(filepath.Join(alpha.Path, ".beads", treeStateFileName)); err != nil {
		t.Errorf("failed switch lost alpha's persistence: %v", err)
	}
	if _, err := os.Stat(filepath.Join(beta.Path, ".beads", treeStateFileName)); !os.IsNotExist(err) {
		t.Errorf("failed switch wrote beta's state: %v", err)
	}
}

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

// TestSwitchingProjectsClearsCommentsLoadErr guards bd-t8j5.18: a comments
// failure recorded against one Dolt project must not linger and be shown
// against the next project switched to, the same leak doltFailure was
// already cleared for.
func TestSwitchingProjectsClearsCommentsLoadErr(t *testing.T) {
	m, _, beta := switchModel(t)
	m.commentsLoadErr = errors.New("alpha: comments unavailable")
	m, _ = requestSwitch(t, m, beta)

	updated, _ := m.Update(projectOpenedMsg{generation: m.projectSwitch.generation, project: beta})
	m = updated.(Model)

	if m.commentsLoadErr != nil {
		t.Errorf("commentsLoadErr = %v after switching to beta, want nil: alpha's error must not leak into beta", m.commentsLoadErr)
	}
}

// TestExitingAllProjectsModeClearsCommentsLoadErr guards bd-t8j5.18: a
// comments failure recorded while browsing all projects must not linger and
// be shown against the single project exitAllProjectsMode returns to.
func TestExitingAllProjectsModeClearsCommentsLoadErr(t *testing.T) {
	m, _, _ := switchModel(t)
	m.allProjectsMode = true
	m.commentsLoadErr = errors.New("all-projects: comments unavailable")

	m.exitAllProjectsMode()

	if m.commentsLoadErr != nil {
		t.Errorf("commentsLoadErr = %v after exiting all-projects mode, want nil", m.commentsLoadErr)
	}
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
