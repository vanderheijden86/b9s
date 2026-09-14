package ui

import (
	"fmt"
	"strings"

	"github.com/vanderheijden86/beadwork/pkg/model"
)

// QueryMode is the finite interaction state of the shared query bar.
type QueryMode uint8

const (
	QueryIdle QueryMode = iota
	QueryEditing
)

// QueryField identifies a supported issue field in a structured query.
type QueryField string

const (
	QueryFieldText     QueryField = ""
	QueryFieldID       QueryField = "id"
	QueryFieldTitle    QueryField = "title"
	QueryFieldStatus   QueryField = "status"
	QueryFieldPriority QueryField = "priority"
	QueryFieldType     QueryField = "type"
	QueryFieldLabel    QueryField = "label"
	QueryFieldAssignee QueryField = "assignee"
	QueryFieldProject  QueryField = "project"
)

type issueQueryPredicate struct {
	field   QueryField
	value   string
	negated bool
}

// IssueQuery is a parsed, immutable query. Positive values within one field
// are alternatives; distinct fields and negated values all compose with AND.
type IssueQuery struct {
	raw        string
	predicates []issueQueryPredicate
}

// ParseIssueQuery parses plain ID/title text and field:value predicates.
func ParseIssueQuery(raw string) IssueQuery {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return IssueQuery{}
	}

	tokens := strings.Fields(raw)
	predicates := make([]issueQueryPredicate, 0, len(tokens))
	plainParts := make([]string, 0, len(tokens))
	for _, token := range tokens {
		negated := strings.HasPrefix(token, "!")
		if negated {
			token = strings.TrimPrefix(token, "!")
		}

		if separator := strings.IndexRune(token, ':'); separator > 0 {
			value := strings.ToLower(token[separator+1:])
			if value == "" {
				continue
			}
			predicates = append(predicates, issueQueryPredicate{
				field:   QueryField(strings.ToLower(token[:separator])),
				value:   value,
				negated: negated,
			})
			continue
		}

		if negated {
			predicates = append(predicates, issueQueryPredicate{
				field:   QueryFieldStatus,
				value:   strings.ToLower(token),
				negated: true,
			})
			continue
		}
		plainParts = append(plainParts, token)
	}

	if len(plainParts) > 0 {
		predicates = append(predicates, issueQueryPredicate{
			field: QueryFieldText,
			value: strings.ToLower(strings.Join(plainParts, " ")),
		})
	}

	return IssueQuery{raw: raw, predicates: predicates}
}

// Empty reports whether the query places no constraints on issues.
func (q IssueQuery) Empty() bool {
	return len(q.predicates) == 0
}

// Raw returns the normalized source text used to build the query.
func (q IssueQuery) Raw() string {
	return q.raw
}

// Matches reports whether an issue satisfies every field group in the query.
func (q IssueQuery) Matches(issue model.Issue) bool {
	if q.Empty() {
		return true
	}

	positiveByField := make(map[QueryField][]issueQueryPredicate)
	for _, predicate := range q.predicates {
		matched := predicateMatchesIssue(predicate, issue)
		if predicate.negated {
			if matched {
				return false
			}
			continue
		}
		positiveByField[predicate.field] = append(positiveByField[predicate.field], predicate)
	}

	for _, alternatives := range positiveByField {
		matched := false
		for _, predicate := range alternatives {
			if predicateMatchesIssue(predicate, issue) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func predicateMatchesIssue(predicate issueQueryPredicate, issue model.Issue) bool {
	value := predicate.value
	switch predicate.field {
	case QueryFieldText:
		return issueMatchesFuzzyText(issue, value)
	case QueryFieldID:
		return fuzzyTextMatch(issue.ID, value)
	case QueryFieldTitle:
		return fuzzyTextMatch(issue.Title, value)
	case QueryFieldStatus:
		return fuzzyTextMatch(string(issue.Status), value)
	case QueryFieldPriority:
		return fmt.Sprintf("%d", issue.Priority) == strings.TrimPrefix(value, "p")
	case QueryFieldType:
		return fuzzyTextMatch(string(issue.IssueType), value)
	case QueryFieldLabel:
		for _, label := range issue.Labels {
			if fuzzyTextMatch(label, value) {
				return true
			}
		}
		return false
	case QueryFieldAssignee:
		return fuzzyTextMatch(issue.Assignee, value)
	case QueryFieldProject:
		project := issue.SourceRepo
		if project == "" {
			project = ExtractRepoPrefix(issue.ID)
		}
		return fuzzyTextMatch(project, value)
	default:
		return false
	}
}

func fuzzyTextMatch(candidate, query string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return false
	}

	candidate = strings.ToLower(candidate)
	if strings.Contains(candidate, query) {
		return true
	}

	compactQuery := strings.Join(strings.Fields(query), "")
	// Long subsequences match ordinary text by chance and cease to express a
	// useful fuzzy relationship. Exact substrings remain unbounded.
	const maxFuzzySubsequenceLength = 16
	if len(compactQuery) > maxFuzzySubsequenceLength {
		return false
	}

	// A fuzzy match may span a short phrase, but never an entire prose field.
	// This keeps abbreviations useful without turning common letter sequences
	// into matches for nearly every issue description or note.
	const maxFuzzyWindowWords = 2
	terms := strings.Fields(candidate)
	for start := range terms {
		window := ""
		for end := start; end < len(terms) && end < start+maxFuzzyWindowWords; end++ {
			window += terms[end]
			if fuzzyScore(window, compactQuery) > 0 {
				return true
			}
		}
	}
	return false
}

func issueMatchesFuzzyText(issue model.Issue, query string) bool {
	for _, candidate := range issuePlainSearchValues(issue) {
		if fuzzyTextMatch(candidate, query) {
			return true
		}
	}
	return false
}

func issuePlainSearchValues(issue model.Issue) []string {
	values := []string{issue.ID, issue.Title}
	values = append(values, issue.Labels...)
	return values
}

// QueryState owns the editable text and its finite input lifecycle.
type QueryState struct {
	mode  QueryMode
	text  string
	query IssueQuery
}

func (s *QueryState) StartEditing() {
	s.mode = QueryEditing
}

func (s *QueryState) Accept() {
	s.mode = QueryIdle
}

func (s *QueryState) Clear() {
	s.mode = QueryIdle
	s.SetText("")
}

func (s *QueryState) SetText(text string) {
	s.text = text
	s.query = ParseIssueQuery(text)
}

func (s *QueryState) Append(runes ...rune) {
	s.SetText(s.text + string(runes))
}

func (s *QueryState) Backspace() {
	runes := []rune(s.text)
	if len(runes) == 0 {
		return
	}
	s.SetText(string(runes[:len(runes)-1]))
}

func (s QueryState) Mode() QueryMode {
	return s.mode
}

func (s QueryState) Text() string {
	return s.text
}

func (s QueryState) Empty() bool {
	return s.query.Empty()
}

func (s QueryState) Query() IssueQuery {
	return s.query
}

func (s QueryState) Matches(issue model.Issue) bool {
	return s.query.Matches(issue)
}

// WithInitialQuery applies the canonical issue query before the TUI starts.
func (m Model) WithInitialQuery(query string) Model {
	m.setQueryText(query)
	m.queryState.Accept()
	return m
}
