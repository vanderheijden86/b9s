package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanderheijden86/beadwork/pkg/config"
)

// testCheckout returns a checkout in a fresh temp directory.
func testCheckout(t *testing.T) Checkout {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	checkout, ok := NewCheckout(dir)
	if !ok {
		t.Fatalf("NewCheckout(%q) rejected a directory with .beads", dir)
	}
	return checkout
}

func TestNewCheckout_RequiresBeadsDirectory(t *testing.T) {
	withoutBeads := t.TempDir()
	if _, ok := NewCheckout(withoutBeads); ok {
		t.Errorf("NewCheckout(%q) accepted a directory without .beads", withoutBeads)
	}
	if _, ok := NewCheckout(""); ok {
		t.Error("NewCheckout accepted an empty path")
	}

	withBeads := t.TempDir()
	if err := os.Mkdir(filepath.Join(withBeads, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	checkout, ok := NewCheckout(withBeads)
	if !ok || checkout.Dir() != withBeads {
		t.Errorf("NewCheckout(%q) = %q, %v; want the directory, true", withBeads, checkout.Dir(), ok)
	}
}

func TestIssueWriterWithoutCheckoutRefusesEveryWrite(t *testing.T) {
	w := &IssueWriter{bdPath: "/nonexistent/bd", available: true}
	writes := map[string]tea.Cmd{
		"update": w.UpdateIssue("x-1", map[string]string{"status": "open"}),
		"create": w.CreateIssue(map[string]string{"title": "New"}),
		"close":  w.CloseIssue("x-1", ""),
		"delete": w.DeleteIssue("x-1"),
		"defer":  w.DeferIssue("x-1", ""),
	}
	for name, cmd := range writes {
		t.Run(name, func(t *testing.T) {
			result, ok := cmd().(BdResultMsg)
			if !ok {
				t.Fatalf("%s produced %T, want BdResultMsg", name, cmd())
			}
			if result.Success || result.Error == nil || !strings.Contains(result.Error.Error(), "read-only: no local checkout") {
				t.Errorf("%s result = %+v, want a read-only refusal", name, result)
			}
		})
	}
}

// A project without a checkout is read as the startup project's Dolt user,
// which is known from metadata even when the startup connection failed.
func TestSwitchWithoutCheckoutKeepsStartupDoltUserAcrossSwitches(t *testing.T) {
	startup := t.TempDir()
	if err := os.Mkdir(filepath.Join(startup, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	m := NewModel(nil, "").
		WithDoltFailure(&DoltFailure{Server: "127.0.0.1:1", Database: "startup", User: "bd_startup", Error: "refused"}).
		WithConfig(config.Config{}, "startup", startup)

	for _, database := range []string{"first_db", "second_db"} {
		updated, _ := m.Update(SwitchProjectMsg{Project: config.Project{Name: database, Database: database, Host: "127.0.0.1:1"}})
		m = updated.(Model)

		if target := m.projectSwitch.target; target.Dolt == nil || target.Dolt.User != "bd_startup" {
			t.Errorf("switch to %s opens %+v, want the database as the startup user bd_startup", database, target)
		}
	}
}

func TestSwitchToProjectWithoutCheckoutIsReadOnly(t *testing.T) {
	// An opened project is saved to the recent list.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	startup := t.TempDir()
	if err := os.Mkdir(filepath.Join(startup, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	m := NewModel(nil, "").WithConfig(config.Config{}, "startup", startup)
	if m.issueWriter.checkout.Dir() != startup {
		t.Fatalf("startup checkout = %q, want %q", m.issueWriter.checkout.Dir(), startup)
	}

	remote := config.Project{Name: "remote", Database: "remote_db", Host: "127.0.0.1:1"}
	updated, _ := m.Update(SwitchProjectMsg{Project: remote})
	updated, _ = updated.(Model).Update(projectOpenedMsg{generation: updated.(Model).projectSwitch.generation, project: remote})
	m = updated.(Model)

	if m.activeProjectName != "remote" {
		t.Errorf("active project = %q, want remote", m.activeProjectName)
	}
	if strings.Contains(m.statusMsg, "No beads found") {
		t.Errorf("status = %q; a project without a checkout must still open from its database", m.statusMsg)
	}
	if dir := m.issueWriter.checkout.Dir(); dir != "" {
		t.Errorf("checkout = %q after switching to a project without one, want none", dir)
	}
}
