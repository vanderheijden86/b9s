package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/vanderheijden86/b9s/pkg/model"
)

func TestProjectTableShowsCountsAgentsAndFacts(t *testing.T) {
	tbl := NewProjectTable(Theme{Renderer: lipgloss.DefaultRenderer()})
	tbl.SetSize(120, 30)
	tbl.startLoading("127.0.0.1:3306")
	tbl.finishLoading([]string{"b9s", "osenco"}, nil, nil)
	if v := tbl.View(); !strings.Contains(v, "…") {
		t.Errorf("counts not read yet should show …:\n%s", v)
	}
	tbl.counts = map[string]projectCounts{
		"b9s":    {open: 12, inProgress: 3, closed7d: 9, lastChange: time.Now().Add(-5 * time.Minute)},
		"osenco": {open: 4, closed7d: 2},
	}
	tbl.agents = map[string]int{"b9s": 2}
	tbl.countsRead = true
	v := tbl.View()
	for _, want := range []string{"CLOSED/7d", "LAST CHANGE", "AGENTS", "● 2", "5m ago", "2 live agents in 1 projects", "11 issues closed in 7 days", "busiest: b9s (9)"} {
		if !strings.Contains(v, want) {
			t.Errorf("table lacks %q:\n%s", want, v)
		}
	}
}

func TestSummarizeProjectIssuesCountsTheWeekAndLastChange(t *testing.T) {
	now := time.Now()
	recent, old := now.Add(-time.Hour), now.Add(-10*24*time.Hour)
	c := summarizeProjectIssues([]model.Issue{
		{ID: "a", Status: model.StatusClosed, ClosedAt: &recent, UpdatedAt: recent},
		{ID: "b", Status: model.StatusClosed, ClosedAt: &old, UpdatedAt: old},
		{ID: "c", Status: model.StatusOpen, UpdatedAt: now.Add(-2 * time.Hour)},
	})
	if c.closed7d != 1 || !c.lastChange.Equal(recent) || c.open != 1 {
		t.Errorf("counts = %+v", c)
	}
}
