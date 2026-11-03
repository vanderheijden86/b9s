package ui

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanderheijden86/b9s/internal/datasource"
	"github.com/vanderheijden86/b9s/pkg/config"
	"github.com/vanderheijden86/b9s/pkg/model"
)

func forgetCommandModel(t *testing.T) Model {
	t.Helper()
	m, checkout := projectTableModel(t)
	m.appConfig.RecentProjects = append(m.appConfig.RecentProjects,
		config.RecentProject{Name: "offline", Database: "offline", Host: "127.0.0.1:1"},
		config.RecentProject{Name: "other", Database: "other", Host: "127.0.0.1:1"},
	)
	return m.WithConfig(m.appConfig, "b9s", checkout)
}

func submitCommand(t *testing.T, m Model, text string) Model {
	t.Helper()
	for _, key := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune(":")},
		{Type: tea.KeyRunes, Runes: []rune(text)},
		{Type: tea.KeyEnter},
	} {
		updated, _ := m.Update(key)
		m = updated.(Model)
	}
	return m
}

func TestForgetRecentProjectRemovesOfflineEntryAndPreservesComments(t *testing.T) {
	m := forgetCommandModel(t)
	if err := os.MkdirAll(filepath.Dir(config.ConfigPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	original := `# personal settings
ui:
  default_view: tree # keep this preference
`
	if err := os.WriteFile(config.ConfigPath(), []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := config.SaveRecentTo(config.ConfigPath(), m.appConfig.RecentProjects, false); err != nil {
		t.Fatal(err)
	}

	m = submitCommand(t, m, "forget 2")

	if len(m.appConfig.RecentProjects) != 2 {
		t.Fatalf("recent projects = %+v, status = %q, want offline forgotten", m.appConfig.RecentProjects, m.statusMsg)
	}
	for _, p := range m.allProjects {
		if p.Name == "offline" {
			t.Fatal("forgotten project still participates in header count refresh")
		}
	}
	if got := m.projectPicker.ProjectByFavoriteNum(2); got == nil || got.Name != "other" {
		t.Fatalf("key 2 = %+v, want other after forgetting offline", got)
	}
	saved, err := config.LoadFrom(config.ConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(saved.RecentProjects, m.appConfig.RecentProjects) {
		t.Fatalf("saved recent projects = %+v, want %+v", saved.RecentProjects, m.appConfig.RecentProjects)
	}
	data, err := os.ReadFile(config.ConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# personal settings", "default_view: tree # keep this preference"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("saved config lacks %q:\n%s", want, data)
		}
	}
	if m.statusIsError || !strings.Contains(m.statusMsg, "offline") {
		t.Errorf("status = %q, error = %v, want named success", m.statusMsg, m.statusIsError)
	}
}

func TestForgetRecentProjectRespectsLock(t *testing.T) {
	m := forgetCommandModel(t)
	m.appConfig.LockRecent = true
	if err := config.SaveRecentTo(config.ConfigPath(), m.appConfig.RecentProjects, true); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(config.ConfigPath())
	if err != nil {
		t.Fatal(err)
	}

	m = submitCommand(t, m, "forget 2")

	after, err := os.ReadFile(config.ConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) || len(m.appConfig.RecentProjects) != 3 {
		t.Fatal("forget changed a locked recent list")
	}
	if !m.statusIsError || !strings.Contains(m.statusMsg, "lock_recent") {
		t.Fatalf("status = %q, want lock_recent explanation", m.statusMsg)
	}
}

func TestForgetRecentProjectKeepsActiveProject(t *testing.T) {
	m := forgetCommandModel(t)

	m = submitCommand(t, m, "forget 1")

	if len(m.appConfig.RecentProjects) != 3 || !m.statusIsError || !strings.Contains(m.statusMsg, "Switch") {
		t.Fatalf("recent = %+v, status = %q, want switch-first explanation", m.appConfig.RecentProjects, m.statusMsg)
	}
}

func TestForgetRecentProjectKeepsActiveAlias(t *testing.T) {
	for _, kind := range []string{"symlink", "unclean path"} {
		t.Run(kind, func(t *testing.T) {
			m := forgetCommandModel(t)
			active := m.activeRecent()
			alias := active
			alias.Name = "active-alias"
			alias.Path = filepath.Dir(active.Path) + "/./" + filepath.Base(active.Path)
			if kind == "symlink" {
				alias.Path = filepath.Join(t.TempDir(), "checkout-alias")
				if err := os.Symlink(active.Path, alias.Path); err != nil {
					t.Fatal(err)
				}
			}
			m.appConfig.RecentProjects[0] = alias
			// A stale row must be guarded by identity even before header normalization.
			m.allProjects[0] = config.Project{
				Name: alias.Name, Path: alias.Path, Host: alias.Host, Database: alias.Database,
			}
			if err := config.SaveRecentTo(config.ConfigPath(), m.appConfig.RecentProjects, false); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(config.ConfigPath())
			if err != nil {
				t.Fatal(err)
			}

			m = submitCommand(t, m, "forget 1")

			after, err := os.ReadFile(config.ConfigPath())
			if err != nil {
				t.Fatal(err)
			}
			if string(after) != string(before) || len(m.appConfig.RecentProjects) != 3 {
				t.Fatal("forget changed the active project's alias or saved config")
			}
			if !m.statusIsError || !strings.Contains(m.statusMsg, "Switch") {
				t.Fatalf("status = %q, want switch-first explanation for active alias", m.statusMsg)
			}
		})
	}
}

func TestForgetRecentProjectUsesVisibleHeaderNumbers(t *testing.T) {
	m := forgetCommandModel(t)
	hidden := m.allProjects[1]
	m.projectReach = map[string]datasource.Reachability{projectKey(hidden): datasource.ReachDenied}
	m.projectPicker = NewProjectPicker(m.buildProjectEntries(), m.theme)

	m = submitCommand(t, m, "forget 2")

	if got := m.appConfig.RecentProjects; len(got) != 2 || got[1].Name != "offline" {
		t.Fatalf("recent = %+v, want hidden offline retained and visible other forgotten", got)
	}
}

func TestForgetRecentProjectSaveFailureKeepsHeaderAndConfig(t *testing.T) {
	m := forgetCommandModel(t)
	if err := os.MkdirAll(filepath.Dir(config.ConfigPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	invalid := "ui: ["
	if err := os.WriteFile(config.ConfigPath(), []byte(invalid), 0o600); err != nil {
		t.Fatal(err)
	}

	m = submitCommand(t, m, "forget 2")

	data, err := os.ReadFile(config.ConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != invalid || len(m.appConfig.RecentProjects) != 3 || len(m.allProjects) != 3 {
		t.Fatal("failed save changed recent projects or damaged config")
	}
	if !m.statusIsError || !strings.Contains(m.statusMsg, "save") {
		t.Fatalf("status = %q, want save error", m.statusMsg)
	}
}

func TestForgetRecentProjectRejectsInvalidNumbers(t *testing.T) {
	for _, text := range []string{"forget", "forget 0", "forget 10", "forget -1", "forget two", "forget 2 3", "forget 9"} {
		t.Run(text, func(t *testing.T) {
			m := submitCommand(t, forgetCommandModel(t), text)
			if len(m.appConfig.RecentProjects) != 3 || !m.statusIsError {
				t.Fatalf("%s: recent = %+v, status = %q, want error without changes", text, m.appConfig.RecentProjects, m.statusMsg)
			}
		})
	}
}

func pressKey(m Model, key tea.KeyType) Model {
	updated, _ := m.Update(tea.KeyMsg{Type: key})
	return updated.(Model)
}

func TestResolveCommand(t *testing.T) {
	tests := []struct {
		input string
		want  Command
	}{
		{input: "epic", want: Command{Kind: CommandTypeFilter, IssueType: model.TypeEpic}},
		{input: "epics", want: Command{Kind: CommandTypeFilter, IssueType: model.TypeEpic}},
		{input: "ep", want: Command{Kind: CommandTypeFilter, IssueType: model.TypeEpic}},
		{input: "ftr", want: Command{Kind: CommandTypeFilter, IssueType: model.TypeFeature}},
		{input: "tasks", want: Command{Kind: CommandTypeFilter, IssueType: model.TypeTask}},
		{input: "bug", want: Command{Kind: CommandTypeFilter, IssueType: model.TypeBug}},
		{input: "chores", want: Command{Kind: CommandTypeFilter, IssueType: model.TypeChore}},
		{input: "all", want: Command{Kind: CommandClearType}},
		{input: "proj", want: Command{Kind: CommandProjects}},
		{input: "  EPIC ", want: Command{Kind: CommandTypeFilter, IssueType: model.TypeEpic}},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ResolveCommand(tt.input)
			if err != nil {
				t.Fatalf("ResolveCommand(%q) error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("ResolveCommand(%q) = %+v, want %+v", tt.input, got, tt.want)
			}
		})
	}
}

// Type and issues commands take query terms, as k9s takes a filter after a
// resource name (:pods app=kindnet). The terms replace the shared query.
func TestCommandsTakeQueryTerms(t *testing.T) {
	got, err := ResolveCommand("epic status:open  label:ui")
	if err != nil || got != (Command{Kind: CommandTypeFilter, IssueType: model.TypeEpic, Query: "status:open label:ui"}) {
		t.Fatalf("got %+v, %v", got, err)
	}
	got, err = ResolveCommand("issues status:open")
	if err != nil || got != (Command{Kind: CommandClearType, Query: "status:open"}) {
		t.Fatalf("got %+v, %v", got, err)
	}
	if _, err := ResolveCommand("layout x"); err == nil || !strings.Contains(err.Error(), "takes no arguments") {
		t.Fatalf("layout x: err = %v", err)
	}
	if _, err := ResolveCommand("bug typ:x"); err == nil || !strings.Contains(err.Error(), `unknown query field "typ"`) {
		t.Fatalf("bug typ:x: err = %v", err)
	}
}

func TestCommandWithQueryTermsReplacesTheQuery(t *testing.T) {
	m := keybindingTestModel(t, nil)
	m.setQueryText("label:ui")
	m.queryState.Accept()
	m, _ = m.executeCommand(Command{Kind: CommandClearType, Query: "status:open"})
	if got := m.QueryText(); got != "status:open" {
		t.Fatalf("query = %q, want status:open", got)
	}
}

func TestResolveCommand_RejectsUnknownCommand(t *testing.T) {
	_, err := ResolveCommand("foo")
	if err == nil || !strings.Contains(err.Error(), "unknown command: foo") {
		t.Fatalf("error = %v, want unknown command: foo", err)
	}
}

func TestEveryStandardIssueTypeHasACommand(t *testing.T) {
	for _, issueType := range []model.IssueType{model.TypeBug, model.TypeFeature, model.TypeTask, model.TypeEpic, model.TypeChore} {
		got, err := ResolveCommand(string(issueType))
		if err != nil || got != (Command{Kind: CommandTypeFilter, IssueType: issueType}) {
			t.Errorf("ResolveCommand(%q) = %+v, %v; want a type filter for it", issueType, got, err)
		}
	}
}

func TestEveryCommandNameResolves(t *testing.T) {
	for _, name := range commandNames {
		if _, err := ResolveCommand(name); err != nil {
			t.Errorf("suggested command %q does not resolve: %v", name, err)
		}
	}
}

func TestColonOpensCommandPromptInEveryView(t *testing.T) {
	tests := []struct {
		name  string
		focus focus
		setup func(*Model)
	}{
		{name: "tree", focus: focusTree, setup: func(m *Model) { m.treeViewActive = true }},
		{name: "list", focus: focusList, setup: func(m *Model) { m.treeViewActive = false }},
		{name: "board", focus: focusBoard, setup: func(m *Model) { m.isBoardView = true }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newSearchFilterModel(t, "")
			m.focused = tt.focus
			tt.setup(&m)

			m = typeKeys(m, ":")

			if got := m.commandPrompt.Mode(); got != PromptEditing {
				t.Fatalf("prompt mode = %v, want editing", got)
			}
		})
	}
}

func TestColonIsQueryTextWhileQueryIsEditing(t *testing.T) {
	m := newSearchFilterModel(t, "")

	m = typeKeys(m, "/", "t", "y", "p", "e", ":")

	if got := m.commandPrompt.Mode(); got != PromptIdle {
		t.Errorf("prompt mode = %v, want idle", got)
	}
	if got := m.queryState.Text(); got != "type:" {
		t.Errorf("query text = %q, want type:", got)
	}
}

func TestCommandPromptSuggestsCommandByPrefix(t *testing.T) {
	m := newSearchFilterModel(t, "")

	m = typeKeys(m, ":", "e", "p")

	if got := m.commandPrompt.Suggestion(); got != "epic" {
		t.Errorf("suggestion = %q, want epic", got)
	}
}

func TestTabAcceptsCommandSuggestion(t *testing.T) {
	m := newSearchFilterModel(t, "")
	m = typeKeys(m, ":", "p", "r")

	m = pressKey(m, tea.KeyTab)

	if got := m.commandPrompt.Text(); got != "project" {
		t.Errorf("prompt text = %q, want project", got)
	}
}

func TestEpicCommandSetsTypeFilter(t *testing.T) {
	m := newSearchFilterModel(t, "")
	m = typeKeys(m, ":", "e", "p", "i", "c")

	m = pressKey(m, tea.KeyEnter)

	if got := m.queryState.Text(); got != "type:epic" {
		t.Errorf("query text = %q, want type:epic", got)
	}
	if got := m.commandPrompt.Mode(); got != PromptIdle {
		t.Errorf("prompt mode = %v, want idle after Enter", got)
	}
}

func TestTypeCommandReplacesTypeTokenAndKeepsOtherTokens(t *testing.T) {
	m := newSearchFilterModel(t, "")
	m.setQueryText("status:open type:bug !type:chore alpha")
	m = typeKeys(m, ":", "t", "a", "s", "k")

	m = pressKey(m, tea.KeyEnter)

	if got := m.queryState.Text(); got != "status:open alpha type:task" {
		t.Errorf("query text = %q, want status:open alpha type:task", got)
	}
}

func TestIssuesCommandClearsTypeFilter(t *testing.T) {
	m := newSearchFilterModel(t, "")
	m.setQueryText("status:open type:bug")
	m = typeKeys(m, ":", "a", "l", "l")

	m = pressKey(m, tea.KeyEnter)

	if got := m.queryState.Text(); got != "status:open" {
		t.Errorf("query text = %q, want status:open", got)
	}
}

func TestUnknownCommandShowsError(t *testing.T) {
	m := newSearchFilterModel(t, "")
	m = typeKeys(m, ":", "f", "o", "o")

	m = pressKey(m, tea.KeyEnter)

	if !m.statusIsError || !strings.Contains(m.statusMsg, "unknown command: foo") {
		t.Errorf("status = %q (error=%v), want unknown command: foo", m.statusMsg, m.statusIsError)
	}
}

func TestEscCancelsCommandPromptWithoutChangingQuery(t *testing.T) {
	m := newSearchFilterModel(t, "")
	m.setQueryText("status:open")
	m = typeKeys(m, ":", "e", "p")

	m = pressKey(m, tea.KeyEsc)

	if got := m.commandPrompt.Mode(); got != PromptIdle {
		t.Errorf("prompt mode = %v, want idle", got)
	}
	if got := m.queryState.Text(); got != "status:open" {
		t.Errorf("query text = %q, want status:open", got)
	}
}

func TestProjectCommandRequestsProjectTable(t *testing.T) {
	m := newSearchFilterModel(t, "")
	m = typeKeys(m, ":", "p", "r", "o", "j")

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if cmd == nil {
		t.Fatal("expected a command requesting the project table")
	}
	if _, ok := cmd().(OpenProjectTableMsg); !ok {
		t.Errorf("command message = %T, want OpenProjectTableMsg", cmd())
	}
}

func TestAcceptedTypeFilterStaysVisibleAsChip(t *testing.T) {
	m := newSearchFilterModel(t, "")
	m = typeKeys(m, ":", "e", "p", "i", "c")

	m = pressKey(m, tea.KeyEnter)

	if bar := stripANSI(m.renderUnifiedTitleBar(80)); !strings.Contains(bar, "type:epic") {
		t.Errorf("title bar = %q, want it to show type:epic", bar)
	}
}

func TestCommandPromptRendersTypedText(t *testing.T) {
	m := newSearchFilterModel(t, "")

	m = typeKeys(m, ":", "e", "p")

	if bar := stripANSI(m.renderUnifiedTitleBar(80)); !strings.Contains(bar, ":ep") || !strings.Contains(bar, "ic") {
		t.Errorf("title bar = %q, want :ep with suggestion ic", bar)
	}
}

func runMouseCommand(t *testing.T, m Model) (Model, tea.Msg) {
	t.Helper()
	m = typeKeys(m, ":", "m", "o", "u", "s", "e")
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal(":mouse returned no command, want a mouse mode change")
	}
	return updated.(Model), cmd()
}

func TestMouseCommandReleasesMouseForTextSelection(t *testing.T) {
	m := newSearchFilterModel(t, "")

	m, msg := runMouseCommand(t, m)

	if msg != tea.DisableMouse() {
		t.Errorf(":mouse sent %T, want tea.DisableMouse so the terminal selects text", msg)
	}
	if m.MouseCaptured() {
		t.Error("MouseCaptured() = true after :mouse, want false")
	}
	if !strings.Contains(m.statusMsg, "select") {
		t.Errorf("status = %q, want it to say text selection is on", m.statusMsg)
	}
}

func TestMouseCommandTwiceRestoresWheelScroll(t *testing.T) {
	m := newSearchFilterModel(t, "")
	m, _ = runMouseCommand(t, m)

	m, msg := runMouseCommand(t, m)

	if msg != tea.EnableMouseCellMotion() {
		t.Errorf("second :mouse sent %T, want tea.EnableMouseCellMotion", msg)
	}
	if !m.MouseCaptured() {
		t.Error("MouseCaptured() = false after second :mouse, want true")
	}
}
