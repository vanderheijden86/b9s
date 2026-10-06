package ui

import (
	"fmt"
	"strings"

	"github.com/vanderheijden86/b9s/pkg/model"
)

// CommandKind identifies what a ':' command does.
type CommandKind uint8

const (
	CommandTypeFilter CommandKind = iota + 1
	CommandClearType
	CommandProjects
	CommandMouse
	CommandLayout
	CommandWrap
	CommandBranch
)

// Command is a resolved ':' command.
type Command struct {
	Kind      CommandKind
	IssueType model.IssueType
	// Arg is the issue id a branch command names.
	Arg string
	// Query holds the query terms after a type or issues command. When set,
	// they replace the shared query instead of editing its type term.
	Query string
}

func typeFilterCommand(t model.IssueType) Command {
	return Command{Kind: CommandTypeFilter, IssueType: t}
}

// commandNames are the canonical spellings offered as suggestions. The alias
// table below must resolve every one of them.
var commandNames = []string{"branch", "bug", "chore", "epic", "feature", "issues", "layout", "mouse", "project", "task", "wrap"}

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
	"layout":  {Kind: CommandLayout},
	"wrap":    {Kind: CommandWrap},
	"project": {Kind: CommandProjects}, "projects": {Kind: CommandProjects}, "proj": {Kind: CommandProjects},
}

// ResolveCommand maps prompt text to a command. A type or issues command may
// carry query terms after its name, as k9s takes a filter after a resource
// (:pods app=kindnet); the other commands take no arguments, except branch.
func ResolveCommand(text string) (Command, error) {
	fields := strings.Fields(text)
	if len(fields) > 0 && strings.EqualFold(fields[0], "branch") {
		if len(fields) > 2 {
			return Command{}, fmt.Errorf("branch takes one issue id")
		}
		command := Command{Kind: CommandBranch}
		if len(fields) == 2 {
			command.Arg = fields[1]
		}
		return command, nil
	}
	if len(fields) == 0 {
		return Command{}, fmt.Errorf("unknown command: ")
	}
	name := strings.ToLower(fields[0])
	command, ok := commandAliases[name]
	if !ok {
		return Command{}, fmt.Errorf("unknown command: %s", name)
	}
	if len(fields) > 1 {
		if command.Kind != CommandTypeFilter && command.Kind != CommandClearType {
			return Command{}, fmt.Errorf("%s takes no arguments", name)
		}
		command.Query = strings.Join(fields[1:], " ")
		if field, bad := unknownQueryField(command.Query); bad {
			return Command{}, fmt.Errorf("unknown query field %q, want one of %s", field, strings.Join(queryFieldNames(), ", "))
		}
	}
	return command, nil
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
