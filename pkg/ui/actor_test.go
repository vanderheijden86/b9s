package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vanderheijden86/beadwork/pkg/identity"
)

func stubActorLookup(env map[string]string, gitUser string) actorLookup {
	return actorLookup{
		getenv:      func(k string) string { return env[k] },
		gitUserName: func(string) string { return gitUser },
	}
}

func TestActorLookup_BeadsActorWins(t *testing.T) {
	l := stubActorLookup(map[string]string{"BEADS_ACTOR": "lane-7", "BD_ACTOR": "old", "USER": "ubuntu"}, "Andre")
	if got := l.actor("/p"); got != "lane-7" {
		t.Fatalf("actor = %q", got)
	}
}

func TestActorLookup_BDActorBeforeGit(t *testing.T) {
	l := stubActorLookup(map[string]string{"BD_ACTOR": "old", "USER": "ubuntu"}, "Andre")
	if got := l.actor("/p"); got != "old" {
		t.Fatalf("actor = %q", got)
	}
}

func TestActorLookup_GitUserBeforeUSER(t *testing.T) {
	l := stubActorLookup(map[string]string{"USER": "ubuntu"}, "Andre")
	if got := l.actor("/p"); got != "Andre" {
		t.Fatalf("actor = %q", got)
	}
}

func TestActorLookup_USERLast(t *testing.T) {
	l := stubActorLookup(map[string]string{"USER": "ubuntu"}, "")
	if got := l.actor("/p"); got != "ubuntu" {
		t.Fatalf("actor = %q", got)
	}
}

func TestActorLookup_NothingIsEmpty(t *testing.T) {
	if got := stubActorLookup(nil, "").actor("/p"); got != "" {
		t.Fatalf("actor = %q, want empty rather than bd's \"unknown\"", got)
	}
}

func TestActorLookup_GitRunsInProjectDir(t *testing.T) {
	var dir string
	l := actorLookup{getenv: func(string) string { return "" }, gitUserName: func(d string) string { dir = d; return "x" }}
	l.actor("/projects/b9s")
	if dir != "/projects/b9s" {
		t.Fatalf("git ran in %q", dir)
	}
}

func TestCreateModalFor_PrefillsAssignee(t *testing.T) {
	modal := NewCreateModalFor(DefaultTheme(lipgloss.DefaultRenderer()), "andre")
	if *modal.assignee != "andre" {
		t.Fatalf("assignee = %q", *modal.assignee)
	}
}

func TestCreateModalFor_PassesActor(t *testing.T) {
	modal := NewCreateModalFor(DefaultTheme(lipgloss.DefaultRenderer()), "andre")
	*modal.title = "t"
	*modal.assignee = "someone-else"
	args := modal.BuildCreateArgs()
	if args["actor"] != "andre" {
		t.Fatalf("actor arg = %q; the creator must not follow the assignee", args["actor"])
	}
}

func TestCreateModal_NoActorWithoutOne(t *testing.T) {
	modal := NewCreateModal(DefaultTheme(lipgloss.DefaultRenderer()))
	*modal.title = "t"
	if _, ok := modal.BuildCreateArgs()["actor"]; ok {
		t.Fatal("no --actor without a known actor")
	}
}

func TestCreateModalFor_HeaderShowsCreator(t *testing.T) {
	modal := NewCreateModalFor(DefaultTheme(lipgloss.DefaultRenderer()), "andre")
	modal.SetSize(100, 40)
	if view := stripANSI(modal.View()); !strings.Contains(view, "as @andre") {
		t.Fatalf("create header should name the creator:\n%s", view)
	}
}

func TestCtrlN_PrefillsResolvedActor(t *testing.T) {
	m := NewModel(nil, "")
	m.activeProjectPath = "/projects/b9s"
	m.actorLookup = stubActorLookup(nil, "vanderheijden86")
	m.identities, _ = identity.Parse(`[{"name":"andre","kind":"human","aliases":["vanderheijden86"]}]`, "")
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	got := next.(Model)
	if !got.showEditModal || *got.editModal.assignee != "andre" {
		t.Fatalf("assignee = %q, want the resolved actor", *got.editModal.assignee)
	}
}
