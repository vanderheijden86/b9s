package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanderheijden86/beadwork/pkg/model"
)

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
