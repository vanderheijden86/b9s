package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/vanderheijden86/b9s/internal/datasource"
	"github.com/vanderheijden86/b9s/pkg/model"
)

func decisionTree(t *testing.T, width int, withGraph bool) *TreeModel {
	t.Helper()
	g := fixtureMemoryGraph()
	tree := NewTreeModel(newTreeTestTheme())
	if withGraph {
		tree.SetDecisionLookup(func() (datasource.MemoryGraph, bool) { return g, true })
	}
	tree.SetSize(width, 20)
	tree.Build(g.Issues())
	tree.ExpandAll()
	return &tree
}

func TestDecisionCellCountsNativeMemoryLinks(t *testing.T) {
	g := fixtureMemoryGraph()
	cases := map[string]string{
		"sample-e.1":  "2 Links",
		"sample-e.2":  "1 Link",
		"sample-lone": "0 Links",
		"sample-e":    "1 Link",
	}
	for id, want := range cases {
		if got := decisionCell(g, id); got != want {
			t.Errorf("decisionCell(%s) = %q, want %q", id, got, want)
		}
	}
}

func TestTreeShowsDecisionsColumnOnlyForAGraphWorkspace(t *testing.T) {
	with := stripANSI(decisionTree(t, 140, true).View())
	if !strings.Contains(with, "MEMORY LINKS") || !strings.Contains(with, "2 Links") || !strings.Contains(with, "1 Link") {
		t.Fatalf("graph workspace tree lacks the DECISIONS column:\n%s", with)
	}
	without := stripANSI(decisionTree(t, 140, false).View())
	if strings.Contains(without, "MEMORY LINKS") {
		t.Fatalf("a project without Memory Links must not spend width on DECISIONS:\n%s", without)
	}
}

func TestDecisionsColumnKeepsRowsAlignedAndYieldsOnNarrowScreens(t *testing.T) {
	tree := decisionTree(t, 140, true)
	lines := strings.Split(stripANSI(tree.View()), "\n")
	header := stripANSI(tree.RenderHeader())
	col := lipgloss.Width(header[:strings.Index(header, "MEMORY LINKS")])
	for _, l := range lines {
		if i := strings.Index(l, "1 Link"); i >= 0 && lipgloss.Width(l[:i]) != col {
			t.Fatalf("cell at %d, header at %d:\n%s\n%s", i, col, header, l)
		}
	}
	if narrow := stripANSI(decisionTree(t, 90, true).RenderHeader()); strings.Contains(narrow, "MEMORY LINKS") {
		t.Fatalf("auto layout keeps the title width on a narrow screen: %s", narrow)
	}
	tree.SetColumnPreference(TreeColumnDecisions, ColumnHide)
	if strings.Contains(stripANSI(tree.RenderHeader()), "MEMORY LINKS") {
		t.Fatal("Hide must remove the column")
	}
}

func TestTreeRereadsDecisionsOnEveryBuild(t *testing.T) {
	g := fixtureMemoryGraph()
	served := false
	tree := NewTreeModel(newTreeTestTheme())
	tree.SetDecisionLookup(func() (datasource.MemoryGraph, bool) { return g, served })
	tree.SetSize(140, 20)
	tree.Build(g.Issues())
	if strings.Contains(stripANSI(tree.RenderHeader()), "MEMORY LINKS") {
		t.Fatal("no graph read yet")
	}
	served = true
	tree.Build([]model.Issue(g.Issues()))
	if !strings.Contains(stripANSI(tree.RenderHeader()), "MEMORY LINKS") {
		t.Fatal("a reload that brings a graph must show the column")
	}
}
