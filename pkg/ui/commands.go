package ui

import (
	"fmt"
	"strings"

	"github.com/vanderheijden86/beadwork/pkg/model"
)

// CommandKind identifies what a ':' command does.
type CommandKind uint8

const (
	CommandTypeFilter CommandKind = iota + 1
	CommandClearType
	CommandProjects
	CommandMouse
)

// Command is a resolved ':' command.
type Command struct {
	Kind      CommandKind
	IssueType model.IssueType
}

func typeFilterCommand(t model.IssueType) Command {
	return Command{Kind: CommandTypeFilter, IssueType: t}
}

// commandNames are the canonical spellings offered as suggestions. The alias
// table below must resolve every one of them.
var commandNames = []string{"bug", "chore", "epic", "feature", "issues", "mouse", "project", "task"}

// commandAliases is a closed table in code rather than config: the set of
// entity views is fixed by the Beads schema, and tests can check it is total.
var commandAliases = map[string]Command{
	"epic": typeFilterCommand(model.TypeEpic), "epics": typeFilterCommand(model.TypeEpic), "ep": typeFilterCommand(model.TypeEpic),
	"feature": typeFilterCommand(model.TypeFeature), "features": typeFilterCommand(model.TypeFeature),
	"feat": typeFilterCommand(model.TypeFeature), "ftr": typeFilterCommand(model.TypeFeature),
	"task": typeFilterCommand(model.TypeTask), "tasks": typeFilterCommand(model.TypeTask),
	"bug": typeFilterCommand(model.TypeBug), "bugs": typeFilterCommand(model.TypeBug),
	"chore": typeFilterCommand(model.TypeChore), "chores": typeFilterCommand(model.TypeChore),
	"issues": {Kind: CommandClearType}, "all": {Kind: CommandClearType},
	"mouse":   {Kind: CommandMouse},
	"project": {Kind: CommandProjects}, "projects": {Kind: CommandProjects}, "proj": {Kind: CommandProjects},
}

// ResolveCommand maps prompt text to a command.
func ResolveCommand(text string) (Command, error) {
	name := strings.ToLower(strings.TrimSpace(text))
	if command, ok := commandAliases[name]; ok {
		return command, nil
	}
	return Command{}, fmt.Errorf("unknown command: %s", name)
}

// commandSuggestion returns the first canonical command name that extends
// text, or "" when none does.
func commandSuggestion(text string) string {
	prefix := strings.ToLower(strings.TrimSpace(text))
	if prefix == "" {
		return ""
	}
	for _, name := range commandNames {
		if strings.HasPrefix(name, prefix) {
			return name
		}
	}
	return ""
}

// withTypeFilter removes every type predicate, negated ones included, from a
// query and appends type:issueType when issueType is not empty. Other tokens
// keep their order.
func withTypeFilter(query string, issueType model.IssueType) string {
	tokens := strings.Fields(query)
	kept := make([]string, 0, len(tokens)+1)
	for _, token := range tokens {
		field := strings.TrimPrefix(token, "!")
		if separator := strings.IndexRune(field, ':'); separator > 0 && QueryField(strings.ToLower(field[:separator])) == QueryFieldType {
			continue
		}
		kept = append(kept, token)
	}
	if issueType != "" {
		kept = append(kept, string(QueryFieldType)+":"+string(issueType))
	}
	return strings.Join(kept, " ")
}
