package ui

import (
	"strings"
	"testing"

	"github.com/vanderheijden86/beadwork/pkg/model"
)

func TestIssueQueryPlainTextMatchesIDAndTitle(t *testing.T) {
	issue := model.Issue{ID: "bd-sxgr", Title: "Live refresh support"}

	for _, raw := range []string{"sxgr", "live refresh"} {
		if query := ParseIssueQuery(raw); !query.Matches(issue) {
			t.Errorf("query %q did not match issue ID or title", raw)
		}
	}
	if query := ParseIssueQuery("unrelated"); query.Matches(issue) {
		t.Error("unrelated plain query matched issue")
	}
}

func TestIssueQueryRejectsLongSparseSubsequence(t *testing.T) {
	const raw = "assetdeasdfjsladkfjsadfjsadf"
	issue := model.Issue{
		ID:          "issue-1",
		Title:       "Unrelated task",
		Description: strings.Join(strings.Split(raw, ""), " unrelated "),
	}

	if query := ParseIssueQuery(raw); query.Matches(issue) {
		t.Error("long sparse subsequence matched an unrelated issue")
	}
}

func TestIssueQueryPlainTextFuzzyMatchesPrimaryFields(t *testing.T) {
	issue := model.Issue{
		ID:     "bd-gewa",
		Title:  "Clean global search",
		Labels: []string{"lane-attempt=1"},
	}

	tests := map[string]string{
		"ID":    "bdgwa",
		"title": "clngbl",
		"label": "lna1",
	}

	for name, raw := range tests {
		t.Run(name, func(t *testing.T) {
			if query := ParseIssueQuery(raw); !query.Matches(issue) {
				t.Errorf("plain query %q did not fuzzy-match %s", raw, name)
			}
		})
	}
}

func TestIssueQueryPlainTextDistinguishesRelevantTermsFromScatteredLetters(t *testing.T) {
	tests := []struct {
		name  string
		raw   string
		issue model.Issue
		want  bool
	}{
		{
			name:  "exact title term",
			raw:   "prevent",
			issue: model.Issue{Title: "Prevent fuzzy search from matching unrelated issues"},
			want:  true,
		},
		{
			name:  "fuzzy title abbreviation",
			raw:   "prvnt",
			issue: model.Issue{Title: "Prevent fuzzy search from matching unrelated issues"},
			want:  true,
		},
		{
			name:  "fuzzy label abbreviation",
			raw:   "lna1",
			issue: model.Issue{Labels: []string{"lane-attempt=1"}},
			want:  true,
		},
		{
			name:  "letters scattered across title words",
			raw:   "prevent",
			issue: model.Issue{Title: "Publish release verification events"},
			want:  false,
		},
		{
			name:  "letters scattered across release notes",
			raw:   "prevent",
			issue: model.Issue{Notes: "The previous release completed successfully and the current version remains available"},
			want:  false,
		},
		{
			name:  "unrelated task",
			raw:   "prevent",
			issue: model.Issue{Title: "Make dependency blocking visible in the TUI"},
			want:  false,
		},
		{
			name:  "no hit",
			raw:   "zzzzzz",
			issue: model.Issue{Title: "Prevent fuzzy search from matching unrelated issues"},
			want:  false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ParseIssueQuery(test.raw).Matches(test.issue); got != test.want {
				t.Errorf("query %q match = %t, want %t for issue %+v", test.raw, got, test.want, test.issue)
			}
		})
	}
}

func TestIssueQueryPlainTextSearchesOnlyPrimaryFields(t *testing.T) {
	externalRef := "https://github.com/example/project/issues/482"
	tests := []struct {
		name  string
		raw   string
		issue model.Issue
		want  bool
	}{
		{name: "id", raw: "ljw", issue: model.Issue{ID: "bd-lj5w"}, want: true},
		{name: "title", raw: "prvnt", issue: model.Issue{ID: "bd-neutral", Title: "Prevent irrelevant fuzzy matches"}, want: true},
		{name: "label", raw: "lna1", issue: model.Issue{ID: "bd-neutral", Labels: []string{"lane-attempt=1"}}, want: true},
		{name: "description", raw: "descneedle", issue: model.Issue{ID: "bd-neutral", Description: "descneedle"}, want: false},
		{name: "design", raw: "designneedle", issue: model.Issue{ID: "bd-neutral", Design: "designneedle"}, want: false},
		{name: "acceptance criteria", raw: "acceptneedle", issue: model.Issue{ID: "bd-neutral", AcceptanceCriteria: "acceptneedle"}, want: false},
		{name: "notes", raw: "ljw", issue: model.Issue{ID: "bd-neutral", Notes: "Blocked by bd-lj5w"}, want: false},
		{name: "status", raw: "inprg", issue: model.Issue{ID: "bd-neutral", Status: model.StatusInProgress}, want: false},
		{name: "priority", raw: "p1", issue: model.Issue{ID: "bd-neutral", Priority: 1}, want: false},
		{name: "type", raw: "bg", issue: model.Issue{ID: "bd-neutral", IssueType: model.TypeBug}, want: false},
		{name: "assignee", raw: "andre", issue: model.Issue{ID: "bd-neutral", Assignee: "andre"}, want: false},
		{name: "project", raw: "spectroscope", issue: model.Issue{ID: "bd-neutral", SourceRepo: "spectroscope"}, want: false},
		{name: "external reference", raw: "ghb482", issue: model.Issue{ID: "bd-neutral", ExternalRef: &externalRef}, want: false},
		{name: "comment author", raw: "operator", issue: model.Issue{ID: "bd-neutral", Comments: []*model.Comment{{Author: "operator"}}}, want: false},
		{name: "comment text", raw: "commentneedle", issue: model.Issue{ID: "bd-neutral", Comments: []*model.Comment{{Text: "commentneedle"}}}, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ParseIssueQuery(test.raw).Matches(test.issue); got != test.want {
				t.Errorf("plain query %q match = %t, want %t", test.raw, got, test.want)
			}
		})
	}
}

func TestIssueQueryFieldValuesMatchFuzzily(t *testing.T) {
	issue := model.Issue{
		Status:     model.StatusInProgress,
		IssueType:  model.TypeFeature,
		Labels:     []string{"lane-attempt=1"},
		Assignee:   "andre",
		SourceRepo: "spectroscope",
	}

	for _, raw := range []string{
		"label:lane-a",
		"label:lna1",
		"status:inprg",
		"type:ftr",
		"assignee:adr",
		"project:spctrscp",
	} {
		if query := ParseIssueQuery(raw); !query.Matches(issue) {
			t.Errorf("field query %q did not fuzzy-match issue", raw)
		}
	}
}

func TestIssueQueryEmptyFieldValueMatchesEveryIssue(t *testing.T) {
	query := ParseIssueQuery("label:")
	issues := []model.Issue{
		{ID: "with-label", Labels: []string{"lane-attempt=1"}},
		{ID: "without-label"},
	}

	for _, issue := range issues {
		if !query.Matches(issue) {
			t.Errorf("empty label predicate excluded %q", issue.ID)
		}
	}
}

func TestIssueQueryPartialLabelShowsEveryMatchingIssue(t *testing.T) {
	query := ParseIssueQuery("label:l")
	issues := []model.Issue{
		{ID: "lane", Labels: []string{"lane-attempt=1"}},
		{ID: "loser", Labels: []string{"loser"}},
		{ID: "lover", Labels: []string{"lover"}},
		{ID: "other", Labels: []string{"backend"}},
	}

	for _, issue := range issues[:3] {
		if !query.Matches(issue) {
			t.Errorf("partial label query excluded %q", issue.ID)
		}
	}
	if query.Matches(issues[3]) {
		t.Errorf("partial label query included unrelated %q", issues[3].ID)
	}
}

func TestModelPartialLabelQueryNarrowsResultsIncrementally(t *testing.T) {
	issues := []model.Issue{
		{ID: "lane", Labels: []string{"lane-attempt=1"}},
		{ID: "loser", Labels: []string{"loser"}},
		{ID: "lover", Labels: []string{"lover"}},
		{ID: "other", Labels: []string{"backend"}},
	}
	m := NewModel(issues, "")

	m.setQueryText("label:")
	if got := len(m.list.Items()); got != 4 {
		t.Fatalf("empty label facet shows %d issues, want 4", got)
	}

	m.setQueryText("label:l")
	if got := len(m.list.Items()); got != 3 {
		t.Fatalf("partial label facet shows %d issues, want 3", got)
	}
}

func TestQueryTabCompletesPlainLabelAndSearchTerms(t *testing.T) {
	issues := []model.Issue{
		{ID: "issue-1", Title: "Deploy dispatcher", Labels: []string{"lane-attempt=1"}},
	}

	for _, test := range []struct {
		name string
		text string
		want string
	}{
		{name: "label", text: "lan", want: "lane-attempt=1"},
		{name: "structured label", text: "label:lan", want: "label:lane-attempt=1"},
		{name: "search term", text: "dep", want: "Deploy"},
	} {
		t.Run(test.name, func(t *testing.T) {
			m := NewModel(issues, "")
			m.queryState.StartEditing()
			m.setQueryText(test.text)

			m.completeQuery()

			if got := m.queryState.Text(); got != test.want {
				t.Fatalf("completion for %q = %q, want %q", test.text, got, test.want)
			}
		})
	}
}

func TestQueryTabDoesNotCompleteHiddenIssueContent(t *testing.T) {
	m := NewModel([]model.Issue{
		{ID: "issue-1", Title: "Neutral title", Notes: "laneinternal"},
	}, "")
	m.queryState.StartEditing()
	m.setQueryText("lanei")

	m.completeQuery()

	if got := m.queryState.Text(); got != "lanei" {
		t.Fatalf("completion exposed hidden issue content: got %q, want %q", got, "lanei")
	}
}

func TestIssueQueryUsesORWithinFacetAndANDBetweenFacets(t *testing.T) {
	query := ParseIssueQuery("status:open status:blocked type:epic")

	openEpic := model.Issue{Status: model.StatusOpen, IssueType: model.TypeEpic}
	blockedEpic := model.Issue{Status: model.StatusBlocked, IssueType: model.TypeEpic}
	openTask := model.Issue{Status: model.StatusOpen, IssueType: model.TypeTask}

	if !query.Matches(openEpic) || !query.Matches(blockedEpic) {
		t.Error("same-facet status values should use OR semantics")
	}
	if query.Matches(openTask) {
		t.Error("different facets should use AND semantics")
	}
}

func TestIssueQueryMatchesSupportedFields(t *testing.T) {
	issue := model.Issue{
		ID:         "bd-7rt1.3",
		Title:      "Add filter palette",
		Status:     model.StatusOpen,
		Priority:   2,
		IssueType:  model.TypeFeature,
		Labels:     []string{"ui", "search"},
		Assignee:   "andre",
		SourceRepo: "b9s",
	}

	for _, raw := range []string{
		"id:7rt1",
		"title:palette",
		"status:open",
		"priority:2",
		"type:feature",
		"label:search",
		"assignee:andre",
		"project:b9s",
		"!status:closed",
	} {
		if query := ParseIssueQuery(raw); !query.Matches(issue) {
			t.Errorf("supported query %q did not match issue", raw)
		}
	}
}

func TestQueryStateHasFiniteEditingLifecycle(t *testing.T) {
	var state QueryState
	if state.Mode() != QueryIdle {
		t.Fatalf("initial mode = %v, want idle", state.Mode())
	}

	state.StartEditing()
	state.SetText("id:sxgr")
	if state.Mode() != QueryEditing {
		t.Fatalf("editing mode = %v, want editing", state.Mode())
	}
	state.Accept()
	if state.Mode() != QueryIdle || state.Text() != "id:sxgr" {
		t.Fatalf("accepted state = (%v, %q), want idle with retained query", state.Mode(), state.Text())
	}
	state.Clear()
	if !state.Empty() || state.Mode() != QueryIdle {
		t.Fatalf("cleared state = (%v, %q), want empty idle", state.Mode(), state.Text())
	}
}

func TestModelQueryFiltersTreeListAndBoard(t *testing.T) {
	issues := []model.Issue{
		{ID: "bd-target", Title: "Target epic", Status: model.StatusOpen, IssueType: model.TypeEpic},
		{ID: "bd-other", Title: "Other task", Status: model.StatusOpen, IssueType: model.TypeTask},
	}
	m := NewModel(issues, "")
	m.tree.Build(issues)

	m.setQueryText("id:target")

	if got := len(m.list.Items()); got != 1 {
		t.Errorf("list items = %d, want 1", got)
	}
	if got := len(m.board.allIssues); got != 1 {
		t.Errorf("board issues = %d, want 1", got)
	}
	if got := m.tree.NodeCount(); got != 1 {
		t.Errorf("tree nodes = %d, want 1", got)
	}
}

func TestModelQueryFiltersAllViewsByRepresentativeCases(t *testing.T) {
	issues := []model.Issue{
		{ID: "prevent-fuzzy", Title: "Prevent fuzzy search from matching unrelated issues", Labels: []string{"search"}},
		{ID: "publish-release", Title: "Publish release verification events", Notes: "The previous release completed successfully and the current version remains available"},
		{ID: "lane-task", Title: "Dispatch work", Labels: []string{"lane-attempt=1"}},
		{ID: "dependency-task", Title: "Make dependency blocking visible in the TUI", Labels: []string{"backend"}},
	}

	tests := []struct {
		name string
		raw  string
		want int
	}{
		{name: "exact term", raw: "prevent", want: 1},
		{name: "fuzzy abbreviation", raw: "prvnt", want: 1},
		{name: "fuzzy label", raw: "lna1", want: 1},
		{name: "structured partial label", raw: "label:l", want: 1},
		{name: "incomplete predicate", raw: "label:", want: len(issues)},
		{name: "no hit", raw: "zzzzzz", want: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			m := NewModel(issues, "")
			m.tree.Build(issues)
			m.setQueryText(test.raw)

			if got := len(m.list.Items()); got != test.want {
				t.Errorf("list query %q shows %d issues, want %d", test.raw, got, test.want)
			}
			if got := len(m.board.allIssues); got != test.want {
				t.Errorf("board query %q shows %d issues, want %d", test.raw, got, test.want)
			}
			if got := m.tree.NodeCount(); got != test.want {
				t.Errorf("tree query %q shows %d issues (%v), want %d", test.raw, got, treeVisibleIDs(&m.tree), test.want)
			}
		})
	}
}

func TestWithInitialQueryFiltersOnStartup(t *testing.T) {
	issues := []model.Issue{
		{ID: "backend-open", Title: "Open backend task", Status: model.StatusOpen, Labels: []string{"backend"}},
		{ID: "backend-closed", Title: "Closed backend task", Status: model.StatusClosed, Labels: []string{"backend"}},
		{ID: "frontend-open", Title: "Open frontend task", Status: model.StatusOpen, Labels: []string{"frontend"}},
	}

	m := NewModel(issues, "").WithInitialQuery("status:open label:backend")

	visible := m.FilteredIssues()
	if len(visible) != 1 || visible[0].ID != "backend-open" {
		t.Fatalf("startup query shows %v, want only backend-open", issueIDs(visible))
	}
	if got := m.queryState.Text(); got != "status:open label:backend" {
		t.Fatalf("startup query text = %q, want it retained", got)
	}
	if got := m.queryState.Mode(); got != QueryIdle {
		t.Fatalf("startup query mode = %v, want idle", got)
	}
}

func TestTreeQueryRetainsOnlyAncestorsNeededForContext(t *testing.T) {
	issues := []model.Issue{
		{ID: "epic", Title: "Search improvements", IssueType: model.TypeEpic},
		{
			ID:        "matching-child",
			Title:     "Prevent unrelated fuzzy matches",
			IssueType: model.TypeTask,
			Dependencies: []*model.Dependency{
				{IssueID: "matching-child", DependsOnID: "epic", Type: model.DepParentChild},
			},
		},
		{ID: "unrelated-root", Title: "Publish release verification events", IssueType: model.TypeTask},
	}
	m := NewModel(issues, "")
	m.tree.Build(issues)

	m.setQueryText("prevent")

	if got := treeVisibleIDs(&m.tree); len(got) != 2 || got[0] != "epic" || got[1] != "matching-child" {
		t.Errorf("visible tree IDs = %v, want context ancestor and matching child", got)
	}
	if got := len(m.list.Items()); got != 1 {
		t.Errorf("list shows %d issues, want only the matching child", got)
	}
	if got := len(m.board.allIssues); got != 1 {
		t.Errorf("board shows %d issues, want only the matching child", got)
	}
}
