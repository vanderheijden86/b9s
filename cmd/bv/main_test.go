package main

import (
	"testing"
	"time"

	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
	"github.com/Dicklesworthstone/beads_viewer/pkg/recipe"
)

func TestFilterByRepo_CaseInsensitiveAndFlexibleSeparators(t *testing.T) {
	issues := []model.Issue{
		{ID: "api-AUTH-1", SourceRepo: "services/api"},
		{ID: "web:UI-2", SourceRepo: "apps/web"},
		{ID: "lib_UTIL_3", SourceRepo: "libs/util"},
		{ID: "misc-4", SourceRepo: "misc"},
	}

	tests := []struct {
		filter   string
		expected int
	}{
		{"API", 1},      // case-insensitive, matches api-
		{"web", 1},      // flexible with ':' separator
		{"lib", 1},      // flexible with '_' separator
		{"missing", 0},  // no match
		{"misc-", 1},    // exact prefix
		{"services", 1}, // matches SourceRepo when ID lacks prefix
	}

	for _, tt := range tests {
		got := filterByRepo(issues, tt.filter)
		if len(got) != tt.expected {
			t.Errorf("filterByRepo(%q) = %d issues, want %d", tt.filter, len(got), tt.expected)
		}
	}
}

func TestApplyRecipeFilters_ActionableAndHasBlockers(t *testing.T) {
	now := time.Now()
	a := model.Issue{ID: "A", Title: "Root", Status: model.StatusOpen, Priority: 2, CreatedAt: now}
	b := model.Issue{
		ID:     "B",
		Title:  "Blocked by A",
		Status: model.StatusOpen,
		Dependencies: []*model.Dependency{
			{DependsOnID: "A", Type: model.DepBlocks},
		},
		CreatedAt: now.Add(-time.Hour),
	}
	issues := []model.Issue{a, b}

	r := &recipe.Recipe{
		Filters: recipe.FilterConfig{
			Actionable: ptrBool(true),
		},
	}
	actionable := applyRecipeFilters(issues, r)
	if len(actionable) != 1 || actionable[0].ID != "A" {
		t.Fatalf("expected only A actionable, got %#v", actionable)
	}

	r.Filters.Actionable = nil
	r.Filters.HasBlockers = ptrBool(true)
	blocked := applyRecipeFilters(issues, r)
	if len(blocked) != 1 || blocked[0].ID != "B" {
		t.Fatalf("expected only B when HasBlockers=true, got %#v", blocked)
	}
}

func TestApplyRecipeFilters_TitleAndPrefix(t *testing.T) {
	issues := []model.Issue{
		{ID: "UI-1", Title: "Add login button"},
		{ID: "API-2", Title: "Login endpoint"},
	}
	r := &recipe.Recipe{
		Filters: recipe.FilterConfig{
			TitleContains: "login",
			IDPrefix:      "API",
		},
	}
	got := applyRecipeFilters(issues, r)
	if len(got) != 1 || got[0].ID != "API-2" {
		t.Fatalf("expected API-2 only, got %#v", got)
	}
}

func TestApplyRecipeSort_DefaultsAndFields(t *testing.T) {
	now := time.Now()
	issues := []model.Issue{
		{ID: "A", Title: "zzz", Priority: 2, CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-30 * time.Minute)},
		{ID: "B", Title: "aaa", Priority: 0, CreatedAt: now, UpdatedAt: now},
	}

	// Priority default ascending
	r := &recipe.Recipe{Sort: recipe.SortConfig{Field: "priority"}}
	sorted := applyRecipeSort(append([]model.Issue{}, issues...), r)
	if sorted[0].ID != "B" {
		t.Fatalf("priority sort expected B first, got %s", sorted[0].ID)
	}

	// Created default descending (newest first)
	r.Sort = recipe.SortConfig{Field: "created"}
	sorted = applyRecipeSort(append([]model.Issue{}, issues...), r)
	if sorted[0].ID != "B" {
		t.Fatalf("created sort expected newest (B) first, got %s", sorted[0].ID)
	}

	// Title ascending explicit desc
	r.Sort = recipe.SortConfig{Field: "title", Direction: "desc"}
	sorted = applyRecipeSort(append([]model.Issue{}, issues...), r)
	if sorted[0].ID != "A" {
		t.Fatalf("title desc expected A (zzz) first, got %s", sorted[0].ID)
	}
}

func TestFormatCycle(t *testing.T) {
	if got := formatCycle(nil); got != "(empty)" {
		t.Fatalf("expected (empty), got %q", got)
	}
	c := []string{"X", "Y", "Z"}
	want := "X → Y → Z → X"
	if got := formatCycle(c); got != want {
		t.Fatalf("formatCycle mismatch: got %q want %q", got, want)
	}
}

func ptrBool(b bool) *bool { return &b }
