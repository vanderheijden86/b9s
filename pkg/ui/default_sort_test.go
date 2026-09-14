package ui_test

import (
	"strings"
	"testing"
	"time"

	"github.com/vanderheijden86/beadwork/pkg/config"
	"github.com/vanderheijden86/beadwork/pkg/model"
	"github.com/vanderheijden86/beadwork/pkg/ui"
)

func defaultSortIssues() []model.Issue {
	return []model.Issue{
		{ID: "a", Title: "Alpha", Status: model.StatusOpen, IssueType: model.TypeTask, CreatedAt: time.Now()},
	}
}

func modelWithSort(t *testing.T, sort config.SortConfig) ui.Model {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.UI.Sort = sort
	return ui.NewModel(defaultSortIssues(), "").WithConfig(cfg, "p", t.TempDir())
}

func TestWithConfigAppliesConfiguredTreeSort(t *testing.T) {
	m := modelWithSort(t, config.SortConfig{Field: "updated", Direction: "asc"})

	if m.TreeSortField() != ui.SortFieldUpdated || m.TreeSortDirection() != ui.SortAscending {
		t.Fatalf("sort = %v %v, want Updated ascending", m.TreeSortField(), m.TreeSortDirection())
	}
}

func TestWithConfigUsesFieldDirectionWhenDirectionOmitted(t *testing.T) {
	m := modelWithSort(t, config.SortConfig{Field: "title"})

	if m.TreeSortField() != ui.SortFieldTitle || m.TreeSortDirection() != ui.SortAscending {
		t.Fatalf("sort = %v %v, want Title ascending", m.TreeSortField(), m.TreeSortDirection())
	}
}

func TestWithConfigWithoutSortShowsNewestCreatedFirst(t *testing.T) {
	m := ui.NewModel(defaultSortIssues(), "").WithConfig(config.Config{}, "p", t.TempDir())

	if m.TreeSortField() != ui.SortFieldCreated || m.TreeSortDirection() != ui.SortDescending {
		t.Fatalf("sort = %v %v, want Created descending", m.TreeSortField(), m.TreeSortDirection())
	}
}

// The config package validates sort names without importing ui, so the two
// vocabularies are kept in step by this test rather than by the compiler.
func TestConfigSortFieldNamesMatchTreeSortFields(t *testing.T) {
	names := config.SortFieldNames()
	if len(names) != int(ui.NumSortFields) {
		t.Fatalf("config has %d sort field names, ui has %d sort fields", len(names), ui.NumSortFields)
	}
	for f := ui.SortField(0); f < ui.NumSortFields; f++ {
		name := strings.ToLower(f.String())
		m := modelWithSort(t, config.SortConfig{Field: name})
		if m.TreeSortField() != f {
			t.Errorf("config field %q applied %v, want %v", name, m.TreeSortField(), f)
		}
	}
}
