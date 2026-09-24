package ui

import (
	"strings"
	"testing"

	"github.com/vanderheijden86/beadwork/pkg/identity"
	"github.com/vanderheijden86/beadwork/pkg/model"
)

const assigneeIdentities = `[
  {"name": "andre", "kind": "human", "aliases": ["vanderheijden86", "vanderheijden86@gmail.com"]},
  {"name": "lanes", "kind": "agent", "aliases": ["ubuntu"]}
]`

func identityIssues() []model.Issue {
	return []model.Issue{
		{ID: "a-1", Title: "one", Status: model.StatusOpen, Assignee: "vanderheijden86"},
		{ID: "a-2", Title: "two", Status: model.StatusOpen, Assignee: "vanderheijden86@gmail.com"},
		{ID: "a-3", Title: "three", Status: model.StatusOpen, Assignee: "ubuntu"},
		{ID: "a-4", Title: "four", Status: model.StatusOpen, Assignee: "crew-7"},
	}
}

func modelWithIdentities(t *testing.T) Model {
	t.Helper()
	m := NewModel(identityIssues(), "")
	reg, err := identity.Parse(assigneeIdentities, "crew-7")
	if err != nil {
		t.Fatal(err)
	}
	m.setIdentities(reg)
	return m
}

func TestAssigneeFilter_CanonicalNameMatchesAliases(t *testing.T) {
	m := modelWithIdentities(t)
	m.assigneeFilter = "andre"
	var matched []string
	for _, issue := range m.issues {
		if m.matchesCurrentFilter(issue) {
			matched = append(matched, issue.ID)
		}
	}
	if len(matched) != 2 || matched[0] != "a-1" || matched[1] != "a-2" {
		t.Fatalf("matched = %v, want a-1 and a-2", matched)
	}
}

func TestTreeAssigneeFilter_CanonicalNameMatchesAliases(t *testing.T) {
	m := modelWithIdentities(t)
	issue := m.issues[1]
	m.tree.assigneeFilter = "andre"
	if !m.tree.passesScopeFilters(&issue) {
		t.Fatal("tree filter on andre must match vanderheijden86@gmail.com")
	}
}

func TestAssigneeEntries_MergeAliases(t *testing.T) {
	m := modelWithIdentities(t)
	m.rebuildAssigneeEntries()
	counts := map[string]int{}
	for _, e := range m.assigneeEntries {
		counts[e.Assignee] = e.Count
	}
	if counts["andre"] != 2 || counts["vanderheijden86"] != 0 {
		t.Fatalf("entries = %+v", m.assigneeEntries)
	}
}

func TestAssigneeEntries_GroupHumansAgentsPools(t *testing.T) {
	m := modelWithIdentities(t)
	m.rebuildAssigneeEntries()
	var order []string
	for _, e := range m.assigneeEntries {
		order = append(order, e.Assignee)
	}
	if len(order) != 3 || order[0] != "andre" || order[1] != "lanes" || order[2] != "crew-7" {
		t.Fatalf("order = %v, want human, agent, pool", order)
	}
}

func TestEditSuggestions_UseCanonicalNames(t *testing.T) {
	m := modelWithIdentities(t)
	got := m.collectEditSuggestions().Assignees
	if len(got) != 3 || got[0] != "andre" || got[1] != "crew-7" || got[2] != "lanes" {
		t.Fatalf("assignee suggestions = %v", got)
	}
}

func TestAssigneeBar_TagsAgentsAndPools(t *testing.T) {
	m := modelWithIdentities(t)
	m.width, m.height = 160, 40
	m.rebuildAssigneeEntries()
	bar := stripANSI(m.renderAssigneeBar())
	for _, want := range []string{"lanes (agent)", "crew-7 (pool)"} {
		if !strings.Contains(bar, want) {
			t.Errorf("assignee bar lacks %q:\n%s", want, bar)
		}
	}
	if strings.Contains(bar, "andre (human)") {
		t.Error("humans carry no tag")
	}
}
